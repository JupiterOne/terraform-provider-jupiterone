package jupiterone

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

var FrameworkTypes = []string{
	string(client.ComplianceFrameworkTypeBenchmark),
	string(client.ComplianceFrameworkTypeStandard),
	string(client.ComplianceFrameworkTypeQuestionnaire),
}

type ComplianceFrameworkModel struct {
	Id            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Version       types.String `tfsdk:"version"`
	FrameworkType types.String `tfsdk:"framework_type"`
	WebLink       types.String `tfsdk:"web_link"`
	ScopeFilters  []string     `tfsdk:"scope_filters"`
}

// BuildScopeFilters builds the data model that is accepted by the J1 API
// for its `JSON` types
func (c *ComplianceFrameworkModel) BuildScopeFilters() ([]map[string]interface{}, diag.Diagnostics) {
	var diag diag.Diagnostics
	scopeFilters := make([]map[string]interface{}, len(c.ScopeFilters))
	for i, f := range c.ScopeFilters {
		err := json.Unmarshal([]byte(f), &scopeFilters[i])
		if err != nil {
			diag.AddError(fmt.Sprintf("Could not marshal scope filter at index %d", i), err.Error())
		}
	}
	return scopeFilters, diag
}

var _ resource.Resource = &ComplianceFrameworkResource{}
var _ resource.ResourceWithConfigure = &ComplianceFrameworkResource{}
var _ resource.ResourceWithImportState = &ComplianceFrameworkResource{}

type ComplianceFrameworkResource struct {
	version string
	qlient  graphql.Client
}

func NewFrameworkResource() resource.Resource {
	return &ComplianceFrameworkResource{}
}

// Metadata implements resource.Resource
func (*ComplianceFrameworkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_framework"
}

// Configure implements resource.ResourceWithConfigure
func (r *ComplianceFrameworkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	p, ok := req.ProviderData.(*JupiterOneProvider)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected JupiterOneProvider, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.version = p.version
	r.qlient = p.Qlient
}

// Schema implements resource.Resource
func (*ComplianceFrameworkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A custom compliance standard, benchmark, or questionnaire.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The framework's name",
			},
			"version": schema.StringAttribute{
				Required:    true,
				Description: "Version of the framework itself (not an J1 API version)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"framework_type": schema.StringAttribute{
				Required:    true,
				Description: "Whether this is a standard, benchmark, or questionnaire",
				Validators: []validator.String{
					stringvalidator.OneOf(FrameworkTypes...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"web_link": schema.StringAttribute{
				Optional:    true,
				Description: "A URL for referencing additional information about the framework",
				// TODO: basic URL pattern matching
				//Validators: []validator.String{
				//},
			},
			"scope_filters": schema.ListAttribute{
				Description: "JSON encoded filters for scoping the framework.",
				Optional:    true,
				ElementType: types.StringType,
				Validators: []validator.List{
					jsonValidator{},
				},
				PlanModifiers: []planmodifier.List{
					jsonIgnoreDiffPlanModifierList(),
				},
			},
		},
	}
}

// ImportState implements resource.ResourceWithImportState
func (*ComplianceFrameworkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Create implements resource.Resource
func (r *ComplianceFrameworkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *ComplianceFrameworkModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	scopeFilters, diag := data.BuildScopeFilters()
	resp.Diagnostics.Append(diag...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := client.CreateComplianceFramework(ctx, r.qlient, client.CreateComplianceFrameworkInput{
		Name:          data.Name.ValueString(),
		Version:       data.Version.ValueString(),
		FrameworkType: client.ComplianceFrameworkType(data.FrameworkType.ValueString()),
		WebLink:       data.WebLink.ValueString(),
		ScopeFilters:  scopeFilters,
	})

	if err != nil {
		resp.Diagnostics.AddError("failed to create framework", err.Error())
		return
	}

	data.Id = types.StringValue(created.CreateComplianceFramework.Id)

	tflog.Trace(ctx, "Created framework",
		map[string]interface{}{"name": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete implements resource.Resource
func (r *ComplianceFrameworkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ComplianceFrameworkModel

	// Read Terraform ste into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.DeleteComplianceFramework(ctx, r.qlient, client.DeleteComplianceFrameworkInput{Id: data.Id.ValueString()}); err != nil {
		resp.Diagnostics.AddError("failed to delete framework", err.Error())
	}
}

// Read implements resource.Resource
func (r *ComplianceFrameworkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ComplianceFrameworkModel

	// Read Terraform ste into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var f client.GetComplianceFrameworkByIdComplianceFramework
	if r, err := client.GetComplianceFrameworkById(ctx, r.qlient, data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to find framework", err.Error())
		return
	} else {
		f = r.ComplianceFramework
	}

	data.Name = types.StringValue(f.Name)
	data.FrameworkType = types.StringValue(string(f.FrameworkType))
	if f.WebLink != "" || !data.WebLink.IsNull() {
		data.WebLink = types.StringValue(f.WebLink)
	}

	newScopeFilters := []string{}

	for _, f := range f.ScopeFilters {
		b, err := json.Marshal(f)
		if err != nil {
			resp.Diagnostics.AddError("failed to json encode scope filter", err.Error())
		}
		newScopeFilters = append(newScopeFilters, string(b))
	}

	if resp.Diagnostics.HasError() {
		return
	}

	data.ScopeFilters = newScopeFilters

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update implements resource.Resource
func (r *ComplianceFrameworkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *ComplianceFrameworkModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	scopeFilters, diag := data.BuildScopeFilters()
	resp.Diagnostics.Append(diag...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := client.UpdateComplianceFramework(ctx, r.qlient, client.UpdateComplianceFrameworkInput{
		Id: data.Id.ValueString(),
		Updates: client.UpdateComplianceFrameworkFields{
			Name:         data.Name.ValueString(),
			WebLink:      data.WebLink.ValueString(),
			ScopeFilters: scopeFilters,
		},
	})

	if err != nil {
		resp.Diagnostics.AddError("failed to update framework", err.Error())
		return
	}

	tflog.Trace(ctx, "Update framework",
		map[string]interface{}{"name": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

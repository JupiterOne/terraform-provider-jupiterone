package jupiterone

import (
	"context"
	"fmt"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

type ComplianceFrameworkItemModel struct {
	Id              types.String `tfsdk:"id"`
	FrameworkId     types.String `tfsdk:"framework_id"`
	GroupId         types.String `tfsdk:"group_id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	DisplayCategory types.String `tfsdk:"display_category"`
	WebLink         types.String `tfsdk:"web_link"`
	Ref             types.String `tfsdk:"ref"`
}

var _ resource.Resource = &ComplianceFrameworkItemResource{}
var _ resource.ResourceWithConfigure = &ComplianceFrameworkItemResource{}
var _ resource.ResourceWithImportState = &ComplianceFrameworkItemResource{}

type ComplianceFrameworkItemResource struct {
	version string
	qlient  graphql.Client
}

func NewFrameworkItemResource() resource.Resource {
	return &ComplianceFrameworkItemResource{}
}

// Metadata implements resource.Resource
func (*ComplianceFrameworkItemResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_frameworkitem"
}

// Configure implements resource.ResourceWithConfigure
func (r *ComplianceFrameworkItemResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (*ComplianceFrameworkItemResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A FrameworkItem (Requirement in the web UI) is a control that is part of a framework",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"framework_id": schema.StringAttribute{
				Required:    true,
				Description: "The internal ID of the framework this item belongs to",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"group_id": schema.StringAttribute{
				Required:    true,
				Description: "The internal ID of the framework group this item belongs to",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The FrameworkItem's display name",
			},
			"ref": schema.StringAttribute{
				Required:    true,
				Description: "A unique identifier that can be used to refer to this item",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description of the item",
			},
			"display_category": schema.StringAttribute{
				Optional: true,
			},
			"web_link": schema.StringAttribute{
				Optional:    true,
				Description: "A URL for referencing additional information about the item",
				// TODO: basic URL pattern matching
				//Validators: []validator.String{
				//},
			},
		},
	}
}

// ImportState implements resource.ResourceWithImportState
func (*ComplianceFrameworkItemResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Create implements resource.Resource
func (r *ComplianceFrameworkItemResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *ComplianceFrameworkItemModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	created, err := client.CreateComplianceFrameworkItem(ctx, r.qlient, client.CreateComplianceFrameworkItemInput{
		Name:            data.Name.ValueString(),
		Description:     data.Description.ValueString(),
		DisplayCategory: data.DisplayCategory.ValueString(),
		Ref:             data.Ref.ValueString(),
		FrameworkId:     data.FrameworkId.ValueString(),
		GroupId:         data.GroupId.ValueString(),
		WebLink:         data.WebLink.ValueString(),
	})

	if err != nil {
		resp.Diagnostics.AddError("failed to create framework item", err.Error())
		return
	}

	data.Id = types.StringValue(created.CreateComplianceFrameworkItem.Id)

	tflog.Trace(ctx, "Created framework item (requirement)",
		map[string]interface{}{"name": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete implements resource.Resource
func (r *ComplianceFrameworkItemResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ComplianceFrameworkItemModel

	// Read Terraform ste into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.DeleteComplianceFrameworkItem(ctx, r.qlient, data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to delete framework item", err.Error())
	}
}

// Read implements resource.Resource
func (r *ComplianceFrameworkItemResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ComplianceFrameworkItemModel

	// Read Terraform ste into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var i client.GetComplianceFrameworkItemByIdComplianceFrameworkItem
	if r, err := client.GetComplianceFrameworkItemById(ctx, r.qlient, data.Id.ValueString()); err != nil {
		if strings.Contains(err.Error(), "Could not find") {
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("failed to find framework item", err.Error())
		}
		return
	} else {
		i = r.ComplianceFrameworkItem
	}

	data.Name = types.StringValue(i.Name)
	data.GroupId = types.StringValue(i.GroupId)
	data.FrameworkId = types.StringValue(i.FrameworkId)
	if i.Description != "" || !data.Description.IsNull() {
		data.Description = types.StringValue(i.Description)
	}
	if i.DisplayCategory != "" || !data.DisplayCategory.IsNull() {
		data.DisplayCategory = types.StringValue(i.DisplayCategory)
	}
	if i.WebLink != "" || !data.WebLink.IsNull() {
		data.WebLink = types.StringValue(i.WebLink)
	}
	if i.Ref != "" || !data.Ref.IsNull() {
		data.Ref = types.StringValue(i.Ref)
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update implements resource.Resource
func (r *ComplianceFrameworkItemResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *ComplianceFrameworkItemModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := client.UpdateComplianceFrameworkItem(ctx, r.qlient, client.UpdateComplianceFrameworkItemInput{
		Id: data.Id.ValueString(),
		Updates: client.UpdateComplianceFrameworkItemFields{
			Name:            data.Name.ValueString(),
			Description:     data.Description.ValueString(),
			DisplayCategory: data.DisplayCategory.ValueString(),
			GroupId:         data.GroupId.ValueString(),
			Ref:             data.Ref.ValueString(),
			WebLink:         data.WebLink.ValueString(),
		},
	})

	if err != nil {
		resp.Diagnostics.AddError("failed to update framework item", err.Error())
		return
	}

	tflog.Trace(ctx, "Update framework item",
		map[string]interface{}{"name": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

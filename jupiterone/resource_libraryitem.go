package jupiterone

import (
	"context"
	"fmt"

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

type ComplianceLibraryItemModel struct {
	Id              types.String `tfsdk:"id"`
	PolicyItemId    types.String `tfsdk:"policy_item_id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	DisplayCategory types.String `tfsdk:"display_category"`
	WebLink         types.String `tfsdk:"web_link"`
	Ref             types.String `tfsdk:"ref"`
}

var _ resource.Resource = &ComplianceLibraryItemResource{}
var _ resource.ResourceWithConfigure = &ComplianceLibraryItemResource{}
var _ resource.ResourceWithImportState = &ComplianceLibraryItemResource{}

type ComplianceLibraryItemResource struct {
	version string
	qlient  graphql.Client
}

func NewLibraryItemResource() resource.Resource {
	return &ComplianceLibraryItemResource{}
}

// Metadata implements resource.Resource
func (*ComplianceLibraryItemResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_libraryitem"
}

// Configure implements resource.ResourceWithConfigure
func (r *ComplianceLibraryItemResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (*ComplianceLibraryItemResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `A Library Item (Control in the web UI) is a control that is independent of a framework, but can be associated with framework items (requirements).

Refer to the resource_framework docs for example usage`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The Library Item's display name",
			},
			"ref": schema.StringAttribute{
				Required:    true,
				Description: "A unique identifier that can be used to refer to this Library Item for linking to framework items (requirements)",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description of the Library Item",
			},
			"display_category": schema.StringAttribute{
				Optional: true,
			},
			"web_link": schema.StringAttribute{
				Optional:    true,
				Description: "A URL for referencing additional information about the LibraryItem",
				// TODO: basic URL pattern matching
				//Validators: []validator.String{
				//},
			},
			"policy_item_id": schema.StringAttribute{
				Optional:    true,
				Description: "The internal ID of the policy item this control is related to, if any",
			},
		},
	}
}

// ImportState implements resource.ResourceWithImportState
func (*ComplianceLibraryItemResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Create implements resource.Resource
func (r *ComplianceLibraryItemResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *ComplianceLibraryItemModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	created, err := client.CreateComplianceLibraryItem(ctx, r.qlient, client.CreateComplianceLibraryItemInput{
		Name:            data.Name.ValueString(),
		Description:     data.Description.ValueString(),
		DisplayCategory: data.DisplayCategory.ValueString(),
		Ref:             data.Ref.ValueString(),
		WebLink:         data.WebLink.ValueString(),
	})

	if err != nil {
		resp.Diagnostics.AddError("failed to create library item", err.Error())
		return
	}

	data.Id = types.StringValue(created.CreateComplianceLibraryItem.Id)

	tflog.Trace(ctx, "Created framework item (LibraryItem)",
		map[string]interface{}{"name": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete implements resource.Resource
func (r *ComplianceLibraryItemResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ComplianceLibraryItemModel

	// Read Terraform ste into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.DeleteComplianceLibraryItem(ctx, r.qlient, data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to delete library item", err.Error())
	}
}

// Read implements resource.Resource
func (r *ComplianceLibraryItemResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ComplianceLibraryItemModel

	// Read Terraform ste into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var i client.GetComplianceLibraryItemByIdComplianceLibraryItem
	if r, err := client.GetComplianceLibraryItemById(ctx, r.qlient, data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to find library item", err.Error())
		return
	} else {
		i = r.ComplianceLibraryItem
	}

	data.Name = types.StringValue(i.Name)
	if i.PolicyItemId != "" || !data.PolicyItemId.IsNull() {
		data.Description = types.StringValue(i.PolicyItemId)
	}
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
func (r *ComplianceLibraryItemResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *ComplianceLibraryItemModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := client.UpdateComplianceLibraryItem(ctx, r.qlient, client.UpdateComplianceLibraryItemInput{
		Id: data.Id.ValueString(),
		Updates: client.UpdateComplianceLibraryItemFields{
			Name:            data.Name.ValueString(),
			PolicyItemId:    data.PolicyItemId.ValueString(),
			Description:     data.Description.ValueString(),
			DisplayCategory: data.DisplayCategory.ValueString(),
			Ref:             data.Ref.ValueString(),
			WebLink:         data.WebLink.ValueString(),
		},
	})

	if err != nil {
		resp.Diagnostics.AddError("failed to update library item", err.Error())
		return
	}

	tflog.Trace(ctx, "Update library item",
		map[string]interface{}{"name": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

package jupiterone

import (
	"context"
	"errors"
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

type ComplianceGroupModel struct {
	Id              types.String `tfsdk:"id"`
	FrameworkId     types.String `tfsdk:"framework_id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	DisplayCategory types.String `tfsdk:"display_category"`
	WebLink         types.String `tfsdk:"web_link"`
}

var _ resource.Resource = &ComplianceGroupResource{}
var _ resource.ResourceWithConfigure = &ComplianceGroupResource{}
var _ resource.ResourceWithImportState = &ComplianceGroupResource{}

type ComplianceGroupResource struct {
	version string
	qlient  graphql.Client
}

func NewGroupResource() resource.Resource {
	return &ComplianceGroupResource{}
}

// Metadata implements resource.Resource
func (*ComplianceGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

// Configure implements resource.ResourceWithConfigure
func (r *ComplianceGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (*ComplianceGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `A compliance group is a child of a framework. Referred to as "Section" in the web UI.

Refer to the resource_framework docs for example usage`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"framework_id": schema.StringAttribute{
				Required:    true,
				Description: "The internal ID of the framework this group is a part of",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The group's name",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "A brief description of the group",
			},
			"display_category": schema.StringAttribute{
				Optional: true,
			},
			"web_link": schema.StringAttribute{
				Optional:    true,
				Description: "A URL for referencing additional information about the group",
				// TODO: basic URL pattern matching
				//Validators: []validator.String{
				//},
			},
		},
	}
}

// ImportState implements resource.ResourceWithImportState
func (*ComplianceGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Create implements resource.Resource
func (r *ComplianceGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *ComplianceGroupModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	created, err := client.CreateComplianceGroup(ctx, r.qlient, client.CreateComplianceGroupInput{
		FrameworkId:     data.FrameworkId.ValueString(),
		Name:            data.Name.ValueString(),
		Description:     data.Description.ValueString(),
		DisplayCategory: data.DisplayCategory.ValueString(),
		WebLink:         data.WebLink.ValueString(),
	})

	if err != nil {
		resp.Diagnostics.AddError("failed to create group", err.Error())
		return
	}

	data.Id = types.StringValue(created.CreateComplianceGroup.Id)

	tflog.Trace(ctx, "Created group",
		map[string]interface{}{"name": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete implements resource.Resource
func (r *ComplianceGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ComplianceGroupModel

	// Read Terraform ste into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.DeleteComplianceGroup(ctx, r.qlient, data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to delete group", err.Error())
	}
}

// Read implements resource.Resource
func (r *ComplianceGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ComplianceGroupModel

	// Read Terraform ste into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	group, err := getGroup(ctx, r.qlient, data.FrameworkId.ValueString(), data.Id.ValueString())

	if err != nil {
		resp.Diagnostics.AddError("failed to find group", err.Error())
		return
	}

	data.Name = types.StringValue(group.Name)
	data.FrameworkId = types.StringValue(group.FrameworkId)
	if group.Description != "" || !data.Description.IsNull() {
		data.Description = types.StringValue(group.Description)
	}
	if group.DisplayCategory != "" || !data.DisplayCategory.IsNull() {
		data.DisplayCategory = types.StringValue(group.DisplayCategory)
	}
	if group.WebLink != "" || !data.WebLink.IsNull() {
		data.WebLink = types.StringValue(group.WebLink)
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// getGroup is workaround for there not being a single getFrameworkGroup API call.
//
// TODO: Remove this if there ever is an easier way to fetch group metadata
// for a single group
func getGroup(ctx context.Context, qlient graphql.Client, frameworkId, groupId string) (*client.ComplianceGroup, error) {
	r, err := client.GetComplianceGroups(ctx, qlient, frameworkId)
	if err != nil {
		return nil, err
	}
	var group *client.ComplianceGroup
	for _, g := range r.ComplianceFramework.Groups {
		if g.Id == groupId {
			group = &g
			break
		}
	}
	if group != nil {
		return group, nil
	}
	return nil, errors.New("child group not found in framework")
}

// Update implements resource.Resource
func (r *ComplianceGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *ComplianceGroupModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := client.UpdateComplianceGroup(ctx, r.qlient, client.UpdateComplianceGroupInput{
		Id: data.Id.ValueString(),
		Updates: client.UpdateComplianceGroupFields{
			FrameworkId:     data.FrameworkId.ValueString(),
			Name:            data.Name.ValueString(),
			Description:     data.Description.ValueString(),
			DisplayCategory: data.DisplayCategory.ValueString(),
			WebLink:         data.WebLink.ValueString(),
		},
	})

	if err != nil {
		resp.Diagnostics.AddError("failed to update group", err.Error())
		return
	}

	tflog.Trace(ctx, "Update group",
		map[string]interface{}{"name": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

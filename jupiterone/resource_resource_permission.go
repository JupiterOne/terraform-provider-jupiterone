package jupiterone

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

type ResourcePermissionResource struct {
	version string
	qlient  graphql.Client
}

type ResourcePermissionModel struct {
	ID           types.String `json:"id,omitempty" tfsdk:"id"`
	SubjectType  types.String `json:"subjectType" tfsdk:"subject_type"`
	SubjectId    types.String `json:"subjectId" tfsdk:"subject_id"`
	ResourceArea types.String `json:"resourceArea" tfsdk:"resource_area"`
	ResourceType types.String `json:"resourceType" tfsdk:"resource_type"`
	ResourceId   types.String `json:"resourceId" tfsdk:"resource_id"`
	CanRead      types.Bool   `json:"canRead" tfsdk:"can_read"`
	CanCreate    types.Bool   `json:"canCreate" tfsdk:"can_create"`
	CanUpdate    types.Bool   `json:"canUpdate" tfsdk:"can_update"`
	CanDelete    types.Bool   `json:"canDelete" tfsdk:"can_delete"`
}

func NewResourcePermissionResource() resource.Resource {
	return &ResourcePermissionResource{}
}

func (*ResourcePermissionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource_permission"
}

func (r *ResourcePermissionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (*ResourcePermissionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "JupiterOne Resource Based Permission",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"subject_type": schema.StringAttribute{
				Required:    true,
				Description: "The type of the subject that the resource permissions will be applied to (e.g. group).",
			},
			"subject_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the subject that the resource permissions will be applied to (e.g. group ID).",
			},
			"resource_area": schema.StringAttribute{
				Required:    true,
				Description: "The resource area that these permissions will be applied to (e.g. rule).",
			},
			"resource_type": schema.StringAttribute{
				Required:    true,
				Description: "The resource type that these permissions will be applied to (e.g. rule, resource_group, *).",
			},
			"resource_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the resource that these permissions will be applied to (e.g. rule ID, resource group ID, *).",
			},
			"can_read": schema.BoolAttribute{
				Required:    true,
				Description: "Whether the subject can read the resource.",
			},
			"can_create": schema.BoolAttribute{
				Required:    true,
				Description: "Whether the subject can create the resource.",
			},
			"can_update": schema.BoolAttribute{
				Required:    true,
				Description: "Whether the subject can update the resource.",
			},
			"can_delete": schema.BoolAttribute{
				Required:    true,
				Description: "Whether the subject can delete the resource.",
			},
		},
	}
}

func (r *ResourcePermissionModel) BuildSetResourcePermissionInput() (client.SetResourcePermissionInput, error) {
	permissionResource := client.SetResourcePermissionInput{
		SubjectType:  r.SubjectType.ValueString(),
		SubjectId:    r.SubjectId.ValueString(),
		ResourceArea: r.ResourceArea.ValueString(),
		ResourceType: r.ResourceType.ValueString(),
		ResourceId:   r.ResourceId.ValueString(),
		CanRead:      r.CanRead.ValueBool(),
		CanCreate:    r.CanCreate.ValueBool(),
		CanUpdate:    r.CanUpdate.ValueBool(),
		CanDelete:    r.CanDelete.ValueBool(),
	}

	return permissionResource, nil
}

func (r *ResourcePermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ResourcePermissionModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	permissionResource, err := data.BuildSetResourcePermissionInput()

	if err != nil {
		resp.Diagnostics.AddError("failed to build resource permission from configuration", err.Error())
		return
	}

	created, err := client.SetResourcePermission(ctx, r.qlient, permissionResource)

	if err != nil {
		resp.Diagnostics.AddError("failed to create resource permission", err.Error())
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("%s-%s-%s-%s-%s",
		data.SubjectType.ValueString(),
		data.SubjectId.ValueString(),
		data.ResourceArea.ValueString(),
		data.ResourceType.ValueString(),
		data.ResourceId.ValueString()))

	tflog.Trace(ctx, "Set resource permission",
		map[string]interface{}{"resourceArea": created.SetResourcePermission.ResourceArea})

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ResourcePermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ResourcePermissionModel
	var state ResourcePermissionModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	// Read current state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// A change in any of these fields should trigger a deletion of the old resource and a creation of a new one
	shouldCreateNewPermissionSetFields := []struct {
		newVal, oldVal attr.Value
		name           string
	}{
		{data.SubjectId, state.SubjectId, "subject_id"},
		{data.SubjectType, state.SubjectType, "subject_type"},
		{data.ResourceArea, state.ResourceArea, "resource_area"},
		{data.ResourceType, state.ResourceType, "resource_type"},
		{data.ResourceId, state.ResourceId, "resource_id"},
	}

	for _, field := range shouldCreateNewPermissionSetFields {
		if !field.newVal.Equal(field.oldVal) {

			tflog.Trace(ctx, "Resource permission fields changed, deleting old permission")

			_, err := client.DeleteResourcePermission(ctx, r.qlient, client.DeleteResourcePermissionInput{
				SubjectId:    state.SubjectId.ValueString(),
				SubjectType:  state.SubjectType.ValueString(),
				ResourceArea: state.ResourceArea.ValueString(),
				ResourceType: state.ResourceType.ValueString(),
				ResourceId:   state.ResourceId.ValueString()})

			if err != nil {
				resp.Diagnostics.AddError("Failed to delete resource permission", err.Error())
				return
			}

			break
		}
	}

	// Can now continue with creating or updating the resource permission
	permissionResource, err := data.BuildSetResourcePermissionInput()

	if err != nil {
		resp.Diagnostics.AddError("failed to build resource permission from configuration", err.Error())
		return
	}

	updated, err := client.SetResourcePermission(ctx, r.qlient, permissionResource)

	if err != nil {
		resp.Diagnostics.AddError("failed to update resource permission", err.Error())
		return
	}

	tflog.Trace(ctx, "Set resource permission",
		map[string]interface{}{"resourceArea": updated.SetResourcePermission.ResourceArea})

	data.ID = types.StringValue(fmt.Sprintf("%s-%s-%s-%s-%s",
		data.SubjectType.ValueString(),
		data.SubjectId.ValueString(),
		data.ResourceArea.ValueString(),
		data.ResourceType.ValueString(),
		data.ResourceId.ValueString()))

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ResourcePermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ResourcePermissionModel

	// Read Terraform state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var deleteInput client.DeleteResourcePermissionInput
	deleteInput.SubjectType = data.SubjectType.ValueString()
	deleteInput.SubjectId = data.SubjectId.ValueString()
	deleteInput.ResourceArea = data.ResourceArea.ValueString()
	deleteInput.ResourceType = data.ResourceType.ValueString()
	deleteInput.ResourceId = data.ResourceId.ValueString()

	_, err := client.DeleteResourcePermission(ctx, r.qlient, deleteInput)

	if err != nil {
		resp.Diagnostics.AddError("failed to delete resource permission", err.Error())
		return
	}
}

func (r *ResourcePermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ResourcePermissionModel

	// Read Terraform state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	const maxResults = 10
	// Check if this resource exists
	resourcePermission, err := client.GetResourcePermissions(ctx, r.qlient, client.GetResourcePermissionsFilter{
		SubjectId:    data.SubjectId.ValueString(),
		SubjectType:  data.SubjectType.ValueString(),
		ResourceArea: data.ResourceArea.ValueString(),
		ResourceType: data.ResourceType.ValueString(),
		ResourceId:   data.ResourceId.ValueString(),
	}, "", maxResults)

	if err != nil {
		resp.Diagnostics.AddError("failed to get resource permission", err.Error())
		return
	}

	// If the resource no longer exists (we may have deleted it as part of the Update action), remove it from the state
	if len(resourcePermission.GetGetResourcePermissions()) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}
}

func (r *ResourcePermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

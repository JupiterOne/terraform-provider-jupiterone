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

type ResourceGroupResource struct {
	version string
	qlient  graphql.Client
}

type ResourceGroupModel struct {
	Id   types.String `json:"id,omitempty" tfsdk:"id"`
	Name types.String `json:"name" tfsdk:"name"`
}

func NewResourceGroupResource() resource.Resource {
	return &ResourceGroupResource{}
}

func (*ResourceGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource_group"
}

func (r *ResourceGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.

	if req.ProviderData == nil {
		tflog.Error(ctx, "ProviderData is nil in Configure")
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

func (*ResourceGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "JupiterOne Resource Group",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the resource group.",
			},
		},
	}
}

func (r *ResourceGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ResourceGroupModel

	tflog.Debug(ctx, "!!!! attempting to create resource group")

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "!!!! SOS", map[string]interface{}{"name": data.Name.ValueString(), "id": data.Id.ValueString()})

	created, err := client.CreateResourceGroup(ctx, r.qlient, client.CreateIamResourceGroupInput{
		Name: data.Name.ValueString(),
	})
	tflog.Debug(ctx, "!!!! after created")

	if err != nil {
		resp.Diagnostics.AddError("failed to create resource group", err.Error())
		return
	}

	data.Id = types.StringValue(created.CreateResourceGroup.Id)
	data.Name = types.StringValue(created.CreateResourceGroup.Name)

	tflog.Trace(ctx, "Created resource group",
		map[string]interface{}{"name": created.CreateResourceGroup.Name})

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ResourceGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ResourceGroupModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := client.UpdateResourceGroup(ctx, r.qlient, client.UpdateIamResourceGroupInput{
		Id:   data.Id.ValueString(),
		Name: data.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to update resource group", err.Error())
		return
	}

	tflog.Trace(ctx, "Updated resource group", map[string]interface{}{"name": updated.UpdateResourceGroup.Name})

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ResourceGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ResourceGroupModel

	// Read Terraform state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := client.DeleteResourceGroup(ctx, r.qlient, client.DeleteIamResourceGroupInput{Id: data.Id.ValueString()})

	if err != nil {
		resp.Diagnostics.AddError("failed to delete resource group", err.Error())
		return
	}
}

func (r *ResourceGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ResourceGroupModel

	// Read Terraform state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resourceGroup, err := client.GetResourceGroup(ctx, r.qlient, data.Id.ValueString())

	if err != nil {
		if strings.Contains(err.Error(), "Item not found") {
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("failed to get resource group", err.Error())
		}
		return
	}

	data.Name = types.StringValue(resourceGroup.ResourceGroup.Name)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ResourceGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

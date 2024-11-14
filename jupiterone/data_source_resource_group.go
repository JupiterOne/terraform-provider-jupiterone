package jupiterone

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

// NewResourceGroupDataSource is a helper function to simplify the provider implementation.
func NewResourceGroupDataSource() datasource.DataSource {
	return &resourceGroupDataSource{}
}

// resourceGroupDataSource is the data source implementation.
type resourceGroupDataSource struct {
	version string
	qlient  graphql.Client
}

// Metadata implements resource.Resource
func (*resourceGroupDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource_group"
}

// Schema implements resource.Resource
func (*resourceGroupDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A saved JupiterOne Resource Group.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the resource group.",
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *resourceGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ResourceGroupModel

	// Read Terraform state into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resourceGroupsData, err := client.GetResourceGroups(ctx, d.qlient)
	if err != nil {
		resp.Diagnostics.AddError("failed to get resource groups", err.Error())
		return
	}

	// Grab one group that has the same name as the provided name
	if len(resourceGroupsData.ResourceGroups) == 0 {
		resp.Diagnostics.AddError("failed to get resource group", "no resource group found with the given name")
		return
	}

	var group *client.GetResourceGroupsResourceGroupsIamResourceGroup

	for _, groupData := range resourceGroupsData.ResourceGroups {
		if groupData.Name == data.Name.ValueString() {
			group = &groupData
			break
		}
	}

	if group == nil {
		resp.Diagnostics.AddError("failed to get resource group", "no group found with the exact given name")
		return
	}

	data.Id = types.StringValue(group.Id)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Configure implements resource.ResourceWithConfigure
func (r *resourceGroupDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

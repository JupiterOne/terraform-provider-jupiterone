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

type IntegrationExternalIdModel struct {
	Id types.String `json:"id,omitempty" tfsdk:"id"`
}

// NewIntegrationExternalIdDataSource is a helper function to simplify the provider implementation.
func NewIntegrationExternalIdDataSource() datasource.DataSource {
	return &integrationExternalIdDataSource{}
}

// integrationExternalIdDataSource is the data source implementation.
type integrationExternalIdDataSource struct {
	version string
	qlient  graphql.Client
}

// Metadata implements resource.Resource
func (*integrationExternalIdDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_external_id"
}

// Schema implements resource.Resource
func (*integrationExternalIdDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An external ID to use in integrations like aws.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *integrationExternalIdDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data IntegrationExternalIdModel

	// Read Terraform state into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	response, err := client.GetExternalId(ctx, d.qlient)
	if err != nil {
		resp.Diagnostics.AddError("failed to execute query", err.Error())
		return
	}

	data.Id = types.StringValue(response.GenerateExternalId)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Configure implements resource.ResourceWithConfigure
func (r *integrationExternalIdDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

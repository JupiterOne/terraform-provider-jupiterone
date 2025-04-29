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

// Ensure provider defined types fully satisfy framework interfaces
var _ datasource.DataSource = &customIntegrationDefinitionDataSource{}

func NewCustomIntegrationDefinitionDataSource() datasource.DataSource {
	return &customIntegrationDefinitionDataSource{}
}

// customIntegrationDefinitionDataSource defines the data source implementation.
type customIntegrationDefinitionDataSource struct {
	version string
	qlient  graphql.Client
}

// customIntegrationDefinitionDataSourceModel describes the data source data model.
type customIntegrationDefinitionDataSourceModel struct {
	ID                   types.String   `tfsdk:"id"`
	IntegrationType      types.String   `tfsdk:"integration_type"`
	Name                 types.String   `tfsdk:"name"`
	Icon                 types.String   `tfsdk:"icon"`
	DocsWebLink          types.String   `tfsdk:"docs_web_link"`
	Description          types.String   `tfsdk:"description"`
	IntegrationCategory  []types.String `tfsdk:"integration_category"`
	CustomDefinitionType types.String   `tfsdk:"custom_definition_type"`
}

func (d *customIntegrationDefinitionDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_integration_definition"
}

func (d *customIntegrationDefinitionDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a JupiterOne custom integration definition.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the custom integration definition.",
				Computed:    true,
			},
			"integration_type": schema.StringAttribute{
				Description: "The type of the integration.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the integration.",
				Computed:    true,
			},
			"icon": schema.StringAttribute{
				Description: "The icon URL for the integration.",
				Computed:    true,
			},
			"docs_web_link": schema.StringAttribute{
				Description: "The documentation web link for the integration.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "The description of the integration.",
				Computed:    true,
			},
			"integration_category": schema.ListAttribute{
				Description: "The categories of the integration.",
				ElementType: types.StringType,
				Computed:    true,
			},
			"custom_definition_type": schema.StringAttribute{
				Description: "The custom definition type of the integration.",
				Computed:    true,
			},
		},
	}
}

func (d *customIntegrationDefinitionDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.version = p.version
	d.qlient = p.Qlient
}

func (d *customIntegrationDefinitionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data customIntegrationDefinitionDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := client.GetCustomIntegrationDefinition(ctx, d.qlient, data.IntegrationType.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read custom integration definition, got error: %s", err))
		return
	}

	def := result.CustomIntegrationDefinition

	// Map response body to model
	data.ID = types.StringValue(def.Id)
	data.Name = types.StringValue(def.Name)
	data.Icon = types.StringValue(def.Icon)
	data.DocsWebLink = types.StringValue(def.DocsWebLink)
	data.Description = types.StringValue(def.Description)

	// Convert []string to []types.String for IntegrationCategory
	integrationCategory := make([]types.String, len(def.IntegrationCategory))
	for i, category := range def.IntegrationCategory {
		integrationCategory[i] = types.StringValue(category)
	}
	data.IntegrationCategory = integrationCategory

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

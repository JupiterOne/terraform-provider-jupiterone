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

// NewCollectorDataSource is a helper function to simplify the provider implementation.
func NewCollectorDataSource() datasource.DataSource {
	return &collectorDataSource{}
}

// collectorDataSource is the data source implementation.
type collectorDataSource struct {
	version string
	qlient  graphql.Client
}

// Metadata implements datasource.DataSource
func (*collectorDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_collector"
}

// Schema implements datasource.DataSource
func (*collectorDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lookup a JupiterOne Collector by id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The ID of the collector.",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "The name of the collector.",
			},
			"account_id":                 schema.StringAttribute{Computed: true},
			"created_at":                 schema.Int64Attribute{Computed: true},
			"updated_at":                 schema.Int64Attribute{Computed: true},
			"collector_pool_id":          schema.StringAttribute{Computed: true},
			"state":                      schema.StringAttribute{Computed: true},
			"integration_instance_count": schema.Int64Attribute{Computed: true},
			"last_heartbeat_at":          schema.Int64Attribute{Computed: true},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *collectorDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data CollectorModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.IsNull() || data.Id.ValueString() == "" {
		resp.Diagnostics.AddError("missing id", "collector data source requires 'id' to be set")
		return
	}

	out, err := client.GetCollector(ctx, d.qlient, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to get collector", err.Error())
		return
	}

	c := out.Collector
	data.Id = types.StringValue(c.Id)
	data.AccountId = types.StringValue(c.AccountId)
	data.Name = types.StringValue(c.Name)
	data.CreatedAt = types.Int64Value(c.CreatedAt)
	data.UpdatedAt = types.Int64Value(c.UpdatedAt)
	data.CollectorPoolId = types.StringValue(c.CollectorPoolId)
	data.State = types.StringValue(c.State)
	data.IntegrationInstanceCount = types.Int64Value(int64(c.IntegrationInstanceCount))
	data.LastHeartbeatAt = types.Int64Value(c.LastHeartbeatAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Configure implements datasource.DataSourceWithConfigure
func (d *collectorDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

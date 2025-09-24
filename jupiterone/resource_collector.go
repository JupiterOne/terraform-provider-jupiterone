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

type CollectorResource struct {
	version string
	qlient  graphql.Client
}

// CollectorModel is the terraform HCL representation of a collector.
type CollectorModel struct {
	Id                       types.String `json:"id,omitempty" tfsdk:"id"`
	Name                     types.String `json:"name" tfsdk:"name"`
	AccountId                types.String `json:"accountId,omitempty" tfsdk:"account_id"`
	CreatedAt                types.Int64  `json:"createdAt,omitempty" tfsdk:"created_at"`
	UpdatedAt                types.Int64  `json:"updatedAt,omitempty" tfsdk:"updated_at"`
	CollectorPoolId          types.String `json:"collectorPoolId,omitempty" tfsdk:"collector_pool_id"`
	State                    types.String `json:"state,omitempty" tfsdk:"state"`
	IntegrationInstanceCount types.Int64  `json:"integrationInstanceCount,omitempty" tfsdk:"integration_instance_count"`
	LastHeartbeatAt          types.Int64  `json:"lastHeartbeatAt,omitempty" tfsdk:"last_heartbeat_at"`
}

func NewCollectorResource() resource.Resource { return &CollectorResource{} }

// Metadata implements resource.Resource
func (*CollectorResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_collector"
}

// Schema implements resource.Resource
func (*CollectorResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A JupiterOne Collector.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
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

// Configure implements resource.ResourceWithConfigure
func (r *CollectorResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Create implements resource.Resource
func (r *CollectorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CollectorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := client.CreateCollector(ctx, r.qlient, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to create collector", err.Error())
		return
	}

	c := created.CreateCollector.Collector
	data.Id = types.StringValue(c.Id)
	data.AccountId = types.StringValue(c.AccountId)
	data.Name = types.StringValue(c.Name)
	data.CreatedAt = types.Int64Value(c.CreatedAt)
	data.UpdatedAt = types.Int64Value(c.UpdatedAt)
	data.CollectorPoolId = types.StringValue(c.CollectorPoolId)
	data.State = types.StringValue(c.State)
	data.IntegrationInstanceCount = types.Int64Value(int64(c.IntegrationInstanceCount))
	data.LastHeartbeatAt = types.Int64Value(c.LastHeartbeatAt)

	tflog.Trace(ctx, "Created collector", map[string]interface{}{"id": data.Id})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete implements resource.Resource
func (r *CollectorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CollectorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.DeleteCollector(ctx, r.qlient, data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to delete collector", err.Error())
	}
}

// Read implements resource.Resource
func (r *CollectorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CollectorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	out, err := client.GetCollector(ctx, r.qlient, data.Id.ValueString())
	if err != nil {
		// If API indicates not found, remove from state
		if err.Error() != "" { // simplistic; provider typically inspects error text
			// Best effort: if not found string present, drop from state
		}
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

// ImportState implements resource.ResourceWithImportState
func (*CollectorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Update implements resource.Resource
func (r *CollectorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data CollectorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := client.UpdateCollector(ctx, r.qlient, data.Id.ValueString(), data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to update collector", err.Error())
		return
	}

	c := updated.UpdateCollector
	data.Id = types.StringValue(c.Id)
	data.AccountId = types.StringValue(c.AccountId)
	data.Name = types.StringValue(c.Name)
	data.CreatedAt = types.Int64Value(c.CreatedAt)
	data.UpdatedAt = types.Int64Value(c.UpdatedAt)
	data.CollectorPoolId = types.StringValue(c.CollectorPoolId)
	data.State = types.StringValue(c.State)
	data.IntegrationInstanceCount = types.Int64Value(int64(c.IntegrationInstanceCount))
	data.LastHeartbeatAt = types.Int64Value(c.LastHeartbeatAt)

	tflog.Trace(ctx, "Updated collector", map[string]interface{}{"id": data.Id})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

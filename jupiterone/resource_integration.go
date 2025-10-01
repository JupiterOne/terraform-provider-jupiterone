package jupiterone

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ resource.Resource = &IntegrationResource{}
var _ resource.ResourceWithConfigure = &IntegrationResource{}
var _ resource.ResourceWithImportState = &IntegrationResource{}

type IntegrationResource struct {
	qlient graphql.Client
}

type IngestionSourceOverride struct {
	IngestionSourceId string `tfsdk:"ingestion_source_id"`
	Enabled           bool   `tfsdk:"enabled"`
}

type IntegrationModel struct {
	Id                            types.String               `tfsdk:"id"`
	Name                          types.String               `tfsdk:"name"`
	PollingInterval               types.String               `tfsdk:"polling_interval"`
	IntegrationDefinitionId       types.String               `tfsdk:"integration_definition_id"`
	Description                   types.String               `tfsdk:"description"`
	Config                        types.String               `tfsdk:"config"`
	SourceIntegrationInstanceId   types.String               `tfsdk:"source_integration_instance_id"`
	CollectorPoolId               types.String               `tfsdk:"collector_pool_id"`
	PollingIntervalCronExpression types.String               `tfsdk:"polling_interval_cron_expression"`
	IngestionSourcesOverrides     *[]IngestionSourceOverride `tfsdk:"ingestion_sources_overrides"`
	ResourceGroupId               types.String               `tfsdk:"resource_group_id"`
}

func NewIntegrationResource() resource.Resource {
	return &IntegrationResource{}
}

func (r *IntegrationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration"
}

func (r *IntegrationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.qlient = p.Qlient
}

func (r *IntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *IntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(data.Config.ValueString()), &config); err != nil {
		resp.Diagnostics.AddError("Failed to unmarshal config", err.Error())
		return
	}

	input := client.CreateIntegrationInstanceInput{
		Name:                    data.Name.ValueString(),
		PollingInterval:         client.IntegrationPollingInterval(data.PollingInterval.ValueString()),
		IntegrationDefinitionId: data.IntegrationDefinitionId.ValueString(),
		Description:             data.Description.ValueString(),
		Config:                  config,
	}

	if !data.SourceIntegrationInstanceId.IsNull() {
		input.SourceIntegrationInstanceId = data.SourceIntegrationInstanceId.ValueString()
	}

	if !data.CollectorPoolId.IsNull() {
		input.CollectorPoolId = data.CollectorPoolId.ValueString()
	}

	if !data.PollingIntervalCronExpression.IsNull() {
		var cronExpression client.IntegrationPollingIntervalCronExpressionInput
		if err := json.Unmarshal([]byte(data.PollingIntervalCronExpression.ValueString()), &cronExpression); err != nil {
			resp.Diagnostics.AddError("Failed to unmarshal polling interval cron expression", err.Error())
			return
		}
		input.PollingIntervalCronExpression = cronExpression
	}

	if data.IngestionSourcesOverrides != nil {
		converted := make([]client.IngestionSourcesOverridesInput, len(*data.IngestionSourcesOverrides))
		for i, v := range *data.IngestionSourcesOverrides {
			converted[i] = client.IngestionSourcesOverridesInput{
				IngestionSourceId: v.IngestionSourceId,
				Enabled:           v.Enabled,
			}
		}
		input.IngestionSourcesOverrides = converted
	}

	if !data.ResourceGroupId.IsNull() {
		input.ResourceGroupId = data.ResourceGroupId.ValueString()
	}

	created, err := client.CreateIntegrationInstance(ctx, r.qlient, input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create integration instance", err.Error())
		return
	}

	data.Id = types.StringValue(created.CreateIntegrationInstance.Id)

	tflog.Trace(ctx, "Created integration instance", map[string]interface{}{"id": data.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data IntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	response, err := client.GetIntegrationInstance(ctx, r.qlient, data.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "Integration instance not found") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read integration instance", err.Error())
		return
	}

	data.Name = types.StringValue(response.IntegrationInstance.Name)
	data.PollingInterval = types.StringValue(string(response.IntegrationInstance.PollingInterval))
	data.IntegrationDefinitionId = types.StringValue(response.IntegrationInstance.IntegrationDefinitionId)
	data.Description = types.StringValue(response.IntegrationInstance.Description)
	data.ResourceGroupId = types.StringValue(response.IntegrationInstance.ResourceGroupId)

	configJSON, err := json.Marshal(response.IntegrationInstance.Config)
	if err != nil {
		resp.Diagnostics.AddError("Failed to marshal config", err.Error())
		return
	}

	// Only update config if it doesn't contain masked values
	// If it does, preserve the existing state value to avoid false diffs
	if !strings.Contains(string(configJSON), "***masked***") {
		data.Config = types.StringValue(string(configJSON))
	} else {
		resp.Diagnostics.AddWarning("Config contains masked values", "Config contains masked values, so it will not be updated in terraform state to avoid false diffs.")
	}

	if response.IntegrationInstance.SourceIntegrationInstanceId != "" {
		data.SourceIntegrationInstanceId = types.StringValue(response.IntegrationInstance.SourceIntegrationInstanceId)
	}

	if response.IntegrationInstance.CollectorPoolId != "" {
		data.CollectorPoolId = types.StringValue(response.IntegrationInstance.CollectorPoolId)
	}

	cronExpression := response.IntegrationInstance.PollingIntervalCronExpression
	if cronExpression.Hour != 0 || cronExpression.DayOfWeek != 0 {
		cronExpressionJSON, err := json.Marshal(cronExpression)
		if err != nil {
			resp.Diagnostics.AddError("Failed to marshal polling interval cron expression", err.Error())
			return
		}
		data.PollingIntervalCronExpression = types.StringValue(string(cronExpressionJSON))
	}

	if len(response.IntegrationInstance.IngestionSourcesOverrides) > 0 {
		converted := make([]IngestionSourceOverride, len(response.IntegrationInstance.IngestionSourcesOverrides))
		for i, v := range response.IntegrationInstance.IngestionSourcesOverrides {
			converted[i] = IngestionSourceOverride{
				IngestionSourceId: v.IngestionSourceId,
				Enabled:           v.Enabled,
			}
		}
		data.IngestionSourcesOverrides = &converted
	} else {
		data.IngestionSourcesOverrides = nil
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *IntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(data.Config.ValueString()), &config); err != nil {
		resp.Diagnostics.AddError("Failed to unmarshal config", err.Error())
		return
	}

	// Cannot pass externalId to the API, so remove it from the config
	delete(config, "externalId")

	input := client.UpdateIntegrationInstanceInput{
		Name:            data.Name.ValueString(),
		PollingInterval: client.IntegrationPollingInterval(data.PollingInterval.ValueString()),
		Description:     data.Description.ValueString(),
		Config:          config,
	}

	if !data.SourceIntegrationInstanceId.IsNull() {
		input.SourceIntegrationInstanceId = data.SourceIntegrationInstanceId.ValueString()
	}

	if !data.CollectorPoolId.IsNull() {
		input.CollectorPoolId = data.CollectorPoolId.ValueString()
	}

	if !data.PollingIntervalCronExpression.IsNull() {
		var cronExpression client.IntegrationPollingIntervalCronExpressionInput
		if err := json.Unmarshal([]byte(data.PollingIntervalCronExpression.ValueString()), &cronExpression); err != nil {
			resp.Diagnostics.AddError("Failed to unmarshal polling interval cron expression", err.Error())
			return
		}
		input.PollingIntervalCronExpression = cronExpression
	}

	if data.IngestionSourcesOverrides != nil && len(*data.IngestionSourcesOverrides) > 0 {
		converted := make([]client.IngestionSourcesOverridesInput, len(*data.IngestionSourcesOverrides))
		for i, v := range *data.IngestionSourcesOverrides {
			converted[i] = client.IngestionSourcesOverridesInput{
				IngestionSourceId: v.IngestionSourceId,
				Enabled:           v.Enabled,
			}
		}
		input.IngestionSourcesOverrides = converted
	} else {
		input.IngestionSourcesOverrides = []client.IngestionSourcesOverridesInput{}
	}

	if !data.ResourceGroupId.IsNull() {
		input.ResourceGroupId = data.ResourceGroupId.ValueString()
	}

	_, err := client.UpdateIntegrationInstance(ctx, r.qlient, data.Id.ValueString(), input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update integration instance", err.Error())
		return
	}

	tflog.Trace(ctx, "Updated integration instance", map[string]interface{}{"id": data.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *IntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := client.DeleteIntegrationInstance(ctx, r.qlient, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete integration instance", err.Error())
		return
	}

	tflog.Trace(ctx, "Deleted integration instance", map[string]interface{}{"id": data.Id.ValueString()})
}

func (r *IntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *IntegrationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A JupiterOne integration instance.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique identifier of the integration instance.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the integration instance.",
			},
			"polling_interval": schema.StringAttribute{
				Required:    true,
				Description: "The polling interval for the integration instance.",
			},
			"integration_definition_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the integration definition. This cannot be changed after creation.",
				PlanModifiers: []planmodifier.String{
					integrationDefinitionIDCannotBeChangedModifier(),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "The description of the integration instance.",
			},
			"config": schema.StringAttribute{
				Required:    true,
				Description: "The configuration for the integration instance as a JSON string.",
			},
			"source_integration_instance_id": schema.StringAttribute{
				Optional:    true,
				Description: "The ID of the source integration instance.",
			},
			"collector_pool_id": schema.StringAttribute{
				Optional:    true,
				Description: "The ID of the collector pool.",
			},
			"polling_interval_cron_expression": schema.StringAttribute{
				Optional:    true,
				Description: "The cron expression for the polling interval as a JSON string.",
			},
			"ingestion_sources_overrides": schema.ListAttribute{
				Optional:    true,
				Description: "Overrides for ingestion sources.",
				ElementType: types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"ingestion_source_id": types.StringType,
						"enabled":             types.BoolType,
					},
				},
			},
			"resource_group_id": schema.StringAttribute{
				Optional:    true,
				Description: "The ID of the resource group to which the integration instance belongs.",
			},
		},
	}
}

func integrationDefinitionIDCannotBeChangedModifier() planmodifier.String {
	return stringplanmodifier.RequiresReplace()
}

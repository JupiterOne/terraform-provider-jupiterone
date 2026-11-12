package jupiterone

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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

// integrationConfigMaskedValue is what the API returns in place of a secret
// config value. It is never the secret itself, so it is never compared as one.
const integrationConfigMaskedValue = "***masked***"

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

	resp.Diagnostics.Append(applyIntegrationInstance(&data, response.IntegrationInstance)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *IntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// After an import the planned config can be the masked copy held in state
	// (see integrationMaskedConfigModifier). The masked marker must never be
	// written back to the API as if it were the secret, so the values to send
	// come from the configuration.
	configToSend := data.Config
	if strings.Contains(data.Config.ValueString(), integrationConfigMaskedValue) {
		var configured types.String
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("config"), &configured)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if !configured.IsNull() && !configured.IsUnknown() {
			configToSend = configured
		}
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(configToSend.ValueString()), &config); err != nil {
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
				Required:  true,
				Sensitive: true,
				Description: "The configuration for the integration instance as a JSON string. " +
					"It is marked sensitive, so Terraform redacts it in plan and apply output; it is still stored in the state file, so protect the state. " +
					"The API returns secret values as `***masked***`. After `terraform import`, state holds those masked values, " +
					"and a key that is masked in state plans no change whatever the configuration sets it to. " +
					"A change to a secret alone is therefore not detected on an imported integration: " +
					"it is sent the next time another attribute or a non-secret config value changes.",
				PlanModifiers: []planmodifier.String{
					integrationMaskedConfigPlanModifier(),
				},
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

// applyIntegrationInstance copies what the API returned for an integration
// instance onto the model read from state. It is written so that a refresh,
// or the read that follows an import, leaves nothing for the next plan to
// change when the configuration matches the instance.
func applyIntegrationInstance(data *IntegrationModel, instance client.GetIntegrationInstanceIntegrationInstance) diag.Diagnostics {
	var diags diag.Diagnostics

	data.Name = types.StringValue(instance.Name)
	data.PollingInterval = types.StringValue(string(instance.PollingInterval))
	data.IntegrationDefinitionId = types.StringValue(instance.IntegrationDefinitionId)
	data.Description = optionalStringFromAPI(instance.Description, data.Description)
	data.ResourceGroupId = optionalStringFromAPI(instance.ResourceGroupId, data.ResourceGroupId)

	configJSON, err := json.Marshal(instance.Config)
	if err != nil {
		diags.AddError("Failed to marshal config", err.Error())
		return diags
	}

	switch {
	case !strings.Contains(string(configJSON), integrationConfigMaskedValue):
		data.Config = types.StringValue(string(configJSON))
	case data.Config.IsNull() || data.Config.IsUnknown():
		// Nothing in state to keep, which is the read that follows an import.
		// Take the config as the API returns it, masked values included;
		// integrationMaskedConfigModifier keeps those from planning a change.
		data.Config = types.StringValue(string(configJSON))
	default:
		// Keep the value already in state to avoid false diffs.
		diags.AddWarning("Config contains masked values", "Config contains masked values, so it will not be updated in terraform state to avoid false diffs.")
	}

	if instance.SourceIntegrationInstanceId != "" {
		data.SourceIntegrationInstanceId = types.StringValue(instance.SourceIntegrationInstanceId)
	}

	if instance.CollectorPoolId != "" {
		data.CollectorPoolId = types.StringValue(instance.CollectorPoolId)
	}

	cronExpression, cronDiags := cronExpressionFromAPI(instance.PollingIntervalCronExpression.Hour, instance.PollingIntervalCronExpression.DayOfWeek, data.PollingIntervalCronExpression)
	diags.Append(cronDiags...)
	if diags.HasError() {
		return diags
	}
	data.PollingIntervalCronExpression = cronExpression

	data.IngestionSourcesOverrides = ingestionSourcesOverridesFromAPI(data.IngestionSourcesOverrides, instance.IngestionSourcesOverrides)

	return diags
}

// optionalStringFromAPI maps an optional string the API returns as "" when it
// has no value. An attribute the configuration omits is null, so an empty
// string from the API stays null unless state already holds an explicit "".
func optionalStringFromAPI(fromAPI string, inState types.String) types.String {
	if fromAPI == "" && (inState.IsNull() || inState.IsUnknown()) {
		return types.StringNull()
	}
	return types.StringValue(fromAPI)
}

// cronExpressionFromAPI maps the cron the API returns onto the JSON string in
// state.
//
// The API's cron fields are plain integers, so hour 0 on day 0 cannot be told
// apart from "no cron". A cron already in state is therefore always compared
// with the API, midnight on Sunday included, and the string in state is kept
// when it means the same thing, so key order or spacing never plans a change.
// With no cron in state, a 0/0 from the API stays null.
func cronExpressionFromAPI(hour, dayOfWeek int, inState types.String) (types.String, diag.Diagnostics) {
	var diags diag.Diagnostics

	if !inState.IsNull() && !inState.IsUnknown() {
		var current client.IntegrationPollingIntervalCronExpressionInput
		if err := json.Unmarshal([]byte(inState.ValueString()), &current); err == nil &&
			current.Hour == hour && current.DayOfWeek == dayOfWeek {
			return inState, diags
		}
	} else if hour == 0 && dayOfWeek == 0 {
		return types.StringNull(), diags
	}

	cronExpressionJSON, err := json.Marshal(client.IntegrationPollingIntervalCronExpressionInput{Hour: hour, DayOfWeek: dayOfWeek})
	if err != nil {
		diags.AddError("Failed to marshal polling interval cron expression", err.Error())
		return inState, diags
	}
	return types.StringValue(string(cronExpressionJSON)), diags
}

// ingestionSourcesOverridesFromAPI returns the overrides the API holds, in the
// order they have in state, followed by any that state does not hold yet,
// sorted by ingestion source id. The attribute is a list, so without this the
// same overrides returned in a different order would plan a change.
func ingestionSourcesOverridesFromAPI(inState *[]IngestionSourceOverride, fromAPI []client.GetIntegrationInstanceIntegrationInstanceIngestionSourcesOverrides) *[]IngestionSourceOverride {
	if len(fromAPI) == 0 {
		return nil
	}

	remaining := make([]IngestionSourceOverride, len(fromAPI))
	for i, v := range fromAPI {
		remaining[i] = IngestionSourceOverride{
			IngestionSourceId: v.IngestionSourceId,
			Enabled:           v.Enabled,
		}
	}

	ordered := make([]IngestionSourceOverride, 0, len(fromAPI))
	if inState != nil {
		for _, known := range *inState {
			for i, candidate := range remaining {
				if candidate.IngestionSourceId == known.IngestionSourceId {
					ordered = append(ordered, candidate)
					remaining = append(remaining[:i], remaining[i+1:]...)
					break
				}
			}
		}
	}

	sort.SliceStable(remaining, func(i, j int) bool {
		return remaining[i].IngestionSourceId < remaining[j].IngestionSourceId
	})
	ordered = append(ordered, remaining...)

	return &ordered
}

var _ planmodifier.String = integrationMaskedConfigModifier{}

// integrationMaskedConfigPlanModifier keeps a config that only differs from
// state in values state holds as masked from planning a change.
func integrationMaskedConfigPlanModifier() planmodifier.String {
	return integrationMaskedConfigModifier{}
}

type integrationMaskedConfigModifier struct{}

func (integrationMaskedConfigModifier) Description(context.Context) string {
	return "Plans no change to config when it matches state, treating a value state holds as " + integrationConfigMaskedValue + " as unchanged."
}

func (m integrationMaskedConfigModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (integrationMaskedConfigModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() ||
		req.PlanValue.IsNull() || req.PlanValue.IsUnknown() ||
		req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}

	// Only a state that holds masked values is handled here. Anything else
	// plans exactly as it did before this modifier existed.
	if !strings.Contains(req.StateValue.ValueString(), integrationConfigMaskedValue) {
		return
	}

	var inState, planned interface{}
	if err := json.Unmarshal([]byte(req.StateValue.ValueString()), &inState); err != nil {
		return
	}
	if err := json.Unmarshal([]byte(req.PlanValue.ValueString()), &planned); err != nil {
		return
	}

	if equalIgnoringMasked(inState, planned) {
		resp.PlanValue = req.StateValue
	}
}

// equalIgnoringMasked reports whether planned matches inState, where a value
// inState holds as the masked marker matches anything. Keys must be the same
// on both sides: a key added or removed is a change.
func equalIgnoringMasked(inState, planned interface{}) bool {
	if masked, ok := inState.(string); ok && masked == integrationConfigMaskedValue {
		return true
	}

	switch stateValue := inState.(type) {
	case map[string]interface{}:
		plannedValue, ok := planned.(map[string]interface{})
		if !ok || len(plannedValue) != len(stateValue) {
			return false
		}
		for key, value := range stateValue {
			other, present := plannedValue[key]
			if !present || !equalIgnoringMasked(value, other) {
				return false
			}
		}
		return true
	case []interface{}:
		plannedValue, ok := planned.([]interface{})
		if !ok || len(plannedValue) != len(stateValue) {
			return false
		}
		for i := range stateValue {
			if !equalIgnoringMasked(stateValue[i], plannedValue[i]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(inState, planned)
	}
}

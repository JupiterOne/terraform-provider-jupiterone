package jupiterone

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
var _ resource.ResourceWithValidateConfig = &IntegrationResource{}

// integrationConfigMaskedValue is what the API returns in place of a secret
// config value. It is never the secret itself, so it is never compared as one.
const integrationConfigMaskedValue = "***masked***"

// integrationConfigSentHashKey is the private state key that holds a hash of
// the config last sent to the API by Create or Update.
//
// The API masks more than secrets: for some definitions every config value
// comes back masked. State then says nothing about what the instance holds,
// so the hash is what a later plan compares the configuration with. An
// imported integration has no hash until its first apply.
const integrationConfigSentHashKey = "config_sent_sha256"

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

	if containsMaskedValue(config) {
		resp.Diagnostics.AddAttributeError(path.Root("config"), integrationMaskedValueInConfigSummary, integrationMaskedValueInConfigDetail)
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

	resp.Diagnostics.Append(setIntegrationConfigSentHash(ctx, resp.Private, data.Config.ValueString())...)
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

	// ValidateConfig already refuses the marker in a known configuration. This
	// covers a config only known at apply time: the API stores the marker as
	// the literal value for fields it masks by default.
	if containsMaskedValue(config) {
		resp.Diagnostics.AddAttributeError(path.Root("config"), integrationMaskedValueInConfigSummary, integrationMaskedValueInConfigDetail)
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

	resp.Diagnostics.Append(setIntegrationConfigSentHash(ctx, resp.Private, configToSend.ValueString())...)
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

const integrationMaskedValueInConfigSummary = "Config contains the masked marker"
const integrationMaskedValueInConfigDetail = "The config contains the value " + integrationConfigMaskedValue + ", which is what the API returns in place of a value it masks. " +
	"It is not the value itself, and for some fields the API would store it as the literal value. " +
	"Replace each " + integrationConfigMaskedValue + " with the real value."

// ValidateConfig refuses a configuration that contains the masked marker.
// State holds the marker after an import, so a config copied from
// `terraform state pull` carries it, and the API does not put the real value
// back for every field: for a field it masks by default, it stores the marker.
func (r *IntegrationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var configured types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("config"), &configured)...)
	if resp.Diagnostics.HasError() || configured.IsNull() || configured.IsUnknown() {
		return
	}

	var config interface{}
	if err := json.Unmarshal([]byte(configured.ValueString()), &config); err != nil {
		// Not valid JSON is reported by Create and Update.
		return
	}
	if containsMaskedValue(config) {
		resp.Diagnostics.AddAttributeError(path.Root("config"), integrationMaskedValueInConfigSummary, integrationMaskedValueInConfigDetail)
	}
}

// containsMaskedValue reports whether any value in a decoded config is the
// masked marker.
func containsMaskedValue(value interface{}) bool {
	switch v := value.(type) {
	case string:
		return v == integrationConfigMaskedValue
	case map[string]interface{}:
		for _, nested := range v {
			if containsMaskedValue(nested) {
				return true
			}
		}
	case []interface{}:
		for _, nested := range v {
			if containsMaskedValue(nested) {
				return true
			}
		}
	}
	return false
}

// integrationConfigHash is a hash of a config JSON string that does not
// depend on key order or spacing.
func integrationConfigHash(configJSON string) (string, error) {
	var decoded interface{}
	if err := json.Unmarshal([]byte(configJSON), &decoded); err != nil {
		return "", err
	}
	// encoding/json writes map keys in sorted order.
	canonical, err := json.Marshal(decoded)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

// privateStateWriter is the part of the framework's private state that Create
// and Update write to.
type privateStateWriter interface {
	SetKey(ctx context.Context, key string, value []byte) diag.Diagnostics
}

// setIntegrationConfigSentHash records the config that was just sent to the
// API, so the next plan can tell whether the configuration has changed even
// when state only holds masked values.
func setIntegrationConfigSentHash(ctx context.Context, private privateStateWriter, sentConfigJSON string) diag.Diagnostics {
	var diags diag.Diagnostics
	hash, err := integrationConfigHash(sentConfigJSON)
	if err != nil {
		diags.AddError("Failed to hash config", err.Error())
		return diags
	}
	// Private state values must be JSON.
	encoded, err := json.Marshal(hash)
	if err != nil {
		diags.AddError("Failed to encode config hash", err.Error())
		return diags
	}
	diags.Append(private.SetKey(ctx, integrationConfigSentHashKey, encoded)...)
	return diags
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
					"The API returns masked values as `***masked***`; for some integration definitions that is every value. " +
					"After `terraform import`, state holds those masked values, and until the integration is next applied a change to a masked value cannot be detected: " +
					"the plan shows a warning saying so. Once Terraform has applied the integration, it remembers what it sent and plans any later change to the config. " +
					"The config must hold the real values: `***masked***` itself is refused.",
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
				Description: "Overrides for ingestion sources. The order does not matter: the same overrides in a different order plan no change.",
				ElementType: types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"ingestion_source_id": types.StringType,
						"enabled":             types.BoolType,
					},
				},
				PlanModifiers: []planmodifier.List{
					ingestionSourcesOverridesOrderPlanModifier(),
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
		// integrationMaskedConfigModifier decides what that plans.
		data.Config = types.StringValue(string(configJSON))
	default:
		// Keep the value already in state to avoid false diffs.
		diags.AddWarning("Config contains masked values", "Config contains masked values, so it will not be updated in terraform state to avoid false diffs.")
	}

	data.SourceIntegrationInstanceId = optionalStringFromAPI(instance.SourceIntegrationInstanceId, data.SourceIntegrationInstanceId)
	data.CollectorPoolId = optionalStringFromAPI(instance.CollectorPoolId, data.CollectorPoolId)

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

// integrationMaskedConfigPlanModifier decides what a config plans when state
// holds masked values. See planMaskedConfig.
func integrationMaskedConfigPlanModifier() planmodifier.String {
	return integrationMaskedConfigModifier{}
}

type integrationMaskedConfigModifier struct{}

func (integrationMaskedConfigModifier) Description(context.Context) string {
	return "When state holds " + integrationConfigMaskedValue + " values, plans a change to config only when it differs from the config Terraform last sent."
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

	sentHash := ""
	if req.Private != nil {
		stored, diags := req.Private.GetKey(ctx, integrationConfigSentHashKey)
		resp.Diagnostics.Append(diags...)
		if len(stored) > 0 {
			// A value that does not decode is treated as no hash.
			_ = json.Unmarshal(stored, &sentHash)
		}
	}

	keepState, undetectable := planMaskedConfig(req.StateValue.ValueString(), req.ConfigValue.ValueString(), sentHash)
	if keepState {
		resp.PlanValue = req.StateValue
	}
	if undetectable {
		resp.Diagnostics.AddAttributeWarning(req.Path,
			"Changes to masked config values cannot be detected yet",
			"State holds "+integrationConfigMaskedValue+" in place of one or more config values, and Terraform has not applied this integration since it was imported, "+
				"so it cannot tell whether the configured values differ from what the integration holds. No change is planned for them. "+
				"After the next apply that updates this integration, Terraform remembers the config it sent and plans any later change.")
	}
}

// planMaskedConfig decides what a config plans when state holds masked
// values. keepState means plan no change to config; undetectable means a
// change to a masked value may exist and cannot be seen.
//
//   - With a hash of the config Terraform last sent, state is not consulted at
//     all: the config plans no change only when it hashes to the same value.
//     This is what makes a change to a masked value plan an update, including
//     on a definition where the API masks every value.
//   - With no hash, which is an integration imported and not yet applied, the
//     only evidence is state. A config that differs from state in a value that
//     is not masked plans a change. One that differs only under masked values
//     plans none, and the caller warns.
func planMaskedConfig(stateJSON, configJSON, sentHash string) (keepState, undetectable bool) {
	if sentHash != "" {
		configHash, err := integrationConfigHash(configJSON)
		if err != nil {
			return false, false
		}
		return configHash == sentHash, false
	}

	var inState, configured interface{}
	if err := json.Unmarshal([]byte(stateJSON), &inState); err != nil {
		return false, false
	}
	if err := json.Unmarshal([]byte(configJSON), &configured); err != nil {
		return false, false
	}
	if equalIgnoringMasked(inState, configured) {
		return true, true
	}
	return false, false
}

var _ planmodifier.List = ingestionSourcesOverridesOrderModifier{}

// ingestionSourcesOverridesOrderPlanModifier plans no change to the overrides
// when the configuration and state hold the same overrides in a different
// order.
func ingestionSourcesOverridesOrderPlanModifier() planmodifier.List {
	return ingestionSourcesOverridesOrderModifier{}
}

type ingestionSourcesOverridesOrderModifier struct{}

func (ingestionSourcesOverridesOrderModifier) Description(context.Context) string {
	return "Plans no change when the configuration and state hold the same overrides in a different order."
}

func (m ingestionSourcesOverridesOrderModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

// The attribute is a list, and the API returns overrides in its own order.
// After an import state has no order of its own to follow, so without this a
// configuration that lists the same overrides differently plans an update.
func (ingestionSourcesOverridesOrderModifier) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() ||
		req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}

	var configured, inState []IngestionSourceOverride
	if diags := req.ConfigValue.ElementsAs(ctx, &configured, false); diags.HasError() {
		// An element that is not known yet: leave the plan as it is.
		return
	}
	if diags := req.StateValue.ElementsAs(ctx, &inState, false); diags.HasError() {
		return
	}

	if sameIngestionSourceOverrides(configured, inState) {
		resp.PlanValue = req.StateValue
	}
}

// sameIngestionSourceOverrides reports whether two lists hold the same
// {ingestion source id, enabled} pairs, whatever their order. A pair that
// appears twice must appear twice on both sides.
func sameIngestionSourceOverrides(a, b []IngestionSourceOverride) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[IngestionSourceOverride]int, len(a))
	for _, override := range a {
		counts[override]++
	}
	for _, override := range b {
		if counts[override] == 0 {
			return false
		}
		counts[override]--
	}
	return true
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

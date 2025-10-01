package jupiterone

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

const MIN_RULE_NAME_LENGTH = 1
const MAX_RULE_NAME_LENGTH = 255
const MIN_JSON_LENGTH = 2

var PollingIntervals = []string{
	string(client.SchedulerPollingIntervalDisabled),
	string(client.SchedulerPollingIntervalThirtyMinutes),
	string(client.SchedulerPollingIntervalOneHour),
	string(client.SchedulerPollingIntervalFourHours),
	string(client.SchedulerPollingIntervalEightHours),
	string(client.SchedulerPollingIntervalTwelveHours),
	string(client.SchedulerPollingIntervalOneDay),
	string(client.SchedulerPollingIntervalOneWeek),
}

// Ensure provider defined types fully satisfy framework interfaces
var _ resource.Resource = &QuestionRuleResource{}
var _ resource.ResourceWithConfigure = &QuestionRuleResource{}
var _ resource.ResourceWithImportState = &QuestionRuleResource{}
var _ resource.ResourceWithConfigValidators = &QuestionRuleResource{}
var _ resource.ResourceWithModifyPlan = &QuestionRuleResource{}

type J1QueryInputModel struct {
	// Query tests must be cleaned of carriage returns before being sent to
	// the server.
	Query           string `json:"query" tfsdk:"query"`
	Version         string `json:"version" tfsdk:"version"`
	Name            string `json:"name" tfsdk:"name"`
	IncludedDeleted bool   `json:"include_deleted" tfsdk:"include_deleted"`
}

type QuestionRuleResource struct {
	version string
	qlient  graphql.Client
}

type RuleQuestion struct {
	Queries []*J1QueryInputModel `json:"queries" tfsdk:"queries"`
}

type RuleOperation struct {
	When    types.String `json:"when" tfsdk:"when"`
	Actions []string     `json:"actions" tfsdk:"actions"`
}

type RuleLabel struct {
	LabelName  types.String `json:"label_name" tfsdk:"label_name"`
	LabelValue types.String `json:"label_value" tfsdk:"label_value"`
}

// newOperationsWithoutId removes any "id" fields before saving into state.
func newOperationsWithoutId(ops []client.RuleOperationOutput) ([]RuleOperation, error) {
	l := make([]RuleOperation, 0, len(ops))
	for _, o := range ops {

		op := RuleOperation{
			Actions: make([]string, 0, len(o.Actions)),
		}

		if o.When != nil {
			w, err := json.Marshal(o.When)
			if err != nil {
				return nil, err
			}
			op.When = types.StringValue(string(w))
		}

		for _, action := range o.Actions {
			if actionMap, ok := action.(map[string]interface{}); ok {
				delete(actionMap, "id")
			}

			a, err := json.Marshal(action)
			if err != nil {
				return nil, err
			}
			op.Actions = append(op.Actions, string(a))
		}

		l = append(l, op)
	}
	return l, nil
}

// tagsToStringSlice converts a types.List to []string
func tagsToStringSlice(tagsList types.List) []string {
	if tagsList.IsNull() || tagsList.IsUnknown() {
		return []string{}
	}

	tags := make([]string, len(tagsList.Elements()))
	for i, elem := range tagsList.Elements() {
		if stringVal, ok := elem.(types.String); ok {
			tags[i] = stringVal.ValueString()
		}
	}
	return tags
}

// outputsToStringSlice converts a types.List to []string
// Returns nil if the list is null (field not set) to preserve existing value
// Returns empty slice if the list is empty (explicitly clearing the field)
func outputsToStringSlice(outputsList types.List) []string {
	// Need to explicitly return nil to prevent perpetual diffs and allow the data to be sent as the value - output: null to the graphql api
	if outputsList.IsNull() || outputsList.IsUnknown() {
		return nil
	}

	outputs := make([]string, len(outputsList.Elements()))
	for i, elem := range outputsList.Elements() {
		if stringVal, ok := elem.(types.String); ok {
			outputs[i] = stringVal.ValueString()
		}
	}
	return outputs
}

func labelsToClientLabels(labelsList types.List) []client.RuleInstanceLabelInput {
	// Need to return an empty slice instead of nil to prevent perpetual diffs and allow the data to be sent as an empty list to the graphql api
	if labelsList.IsNull() || labelsList.IsUnknown() {
		return []client.RuleInstanceLabelInput{}
	}

	elements := labelsList.Elements()
	// Need to return an empty slice instead of nil to prevent perpetual diffs and allow the data to be sent as an empty list to the graphql api
	if len(elements) == 0 {
		return []client.RuleInstanceLabelInput{}
	}

	labels := make([]client.RuleInstanceLabelInput, 0, len(elements))

	for _, elem := range elements {
		if objVal, ok := elem.(types.Object); ok {
			attrs := objVal.Attributes()
			label := client.RuleInstanceLabelInput{}

			if labelName, ok := attrs["label_name"].(types.String); ok {
				label.LabelName = labelName.ValueString()
			}
			if labelValue, ok := attrs["label_value"].(types.String); ok {
				label.LabelValue = labelValue.ValueString()
			}

			labels = append(labels, label)
		}
	}

	return labels
}

// convertLabelsToTerraformList converts API labels to a Terraform types.List.
// Returns an error if there were diagnostic errors during conversion.
func convertLabelsToTerraformList(labels any) (types.List, error) {
	// Use reflection to handle different label types from generated client
	labelsSlice := reflect.ValueOf(labels)

	if labelsSlice.Len() > 0 {
		labelObjects := make([]attr.Value, labelsSlice.Len())
		for i := 0; i < labelsSlice.Len(); i++ {
			label := labelsSlice.Index(i)
			labelName := label.FieldByName("LabelName").String()
			labelValue := label.FieldByName("LabelValue").String()

			labelMap := map[string]attr.Value{
				"label_name":  types.StringValue(labelName),
				"label_value": types.StringValue(labelValue),
			}
			labelObj, diags := types.ObjectValue(map[string]attr.Type{
				"label_name":  types.StringType,
				"label_value": types.StringType,
			}, labelMap)
			if diags.HasError() {
				return types.List{}, fmt.Errorf("failed to create label object: %v", diags.Errors())
			}
			labelObjects[i] = labelObj
		}
		labelsListValue, diags := types.ListValue(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"label_name":  types.StringType,
				"label_value": types.StringType,
			},
		}, labelObjects)
		if diags.HasError() {
			return types.List{}, fmt.Errorf("failed to create labels list: %v", diags.Errors())
		}
		return labelsListValue, nil
	}

	// Set to empty list instead of nil
	emptyLabelsList, diags := types.ListValue(types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"label_name":  types.StringType,
			"label_value": types.StringType,
		},
	}, []attr.Value{})
	if diags.HasError() {
		return types.List{}, fmt.Errorf("failed to create empty labels list: %v", diags.Errors())
	}
	return emptyLabelsList, nil
}

// ensureLabelsInitialized sets the Labels field to an empty list if it's unknown or null.
// This prevents perpetual diffs when the API returns an empty labels array.
func ensureLabelsInitialized(labels types.List) (types.List, diag.Diagnostics) {
	if labels.IsUnknown() || labels.IsNull() {
		emptyLabelsList, diags := types.ListValue(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"label_name":  types.StringType,
				"label_value": types.StringType,
			},
		}, []attr.Value{})
		return emptyLabelsList, diags
	}
	return labels, nil
}

// RuleModel represents the terraform representation of the rule
type RuleModel struct {
	Id              types.String      `json:"id,omitempty" tfsdk:"id"`
	Name            types.String      `json:"name" tfsdk:"name"`
	Description     types.String      `json:"description" tfsdk:"description"`
	Version         types.Int64       `json:"version,omitempty" tfsdk:"version"`
	SpecVersion     types.Int64       `json:"specVersion,omitempty" tfsdk:"spec_version"`
	PollingInterval types.String      `json:"polling_interval,omitempty" tfsdk:"polling_interval"`
	Templates       map[string]string `json:"templates" tfsdk:"templates"`
	Question        []*RuleQuestion   `json:"question,omitempty" tfsdk:"question"`
	QuestionId      types.String      `json:"questionId,omitempty" tfsdk:"question_id"`
	// Operations TODO: breaking change for new version to do more in the
	// HCL and/or make better use of things like jsonencode
	Operations            []RuleOperation `json:"operations" tfsdk:"operations"`
	Outputs               types.List      `json:"outputs" tfsdk:"outputs"`
	Tags                  types.List      `json:"tags" tfsdk:"tags"`
	NotifyOnFailure       types.Bool      `json:"notify_on_failure" tfsdk:"notify_on_failure"`
	TriggerOnNewOnly      types.Bool      `json:"trigger_on_new_only" tfsdk:"trigger_on_new_only"`
	IgnorePreviousResults types.Bool      `json:"ignore_previous_results" tfsdk:"ignore_previous_results"`
	Labels                types.List      `json:"labels" tfsdk:"labels"`
	ResourceGroupId       types.String    `json:"resource_group_id,omitempty" tfsdk:"resource_group_id"`
}

func NewQuestionRuleResource() resource.Resource {
	return &QuestionRuleResource{}
}

// Metadata implements resource.Resource
func (*QuestionRuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rule"
}

// Configure implements resource.ResourceWithConfigure
func (r *QuestionRuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Schema implements resource.ResourceWithConfigure
func (*QuestionRuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A saved JupiterOne question based alert",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Unique id that identifies the rule",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"version": schema.Int64Attribute{
				Description: "Computed current version of the rule. Incremented each time the rule is updated.",
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					// This works with ModifyPlan() prevent planned changes
					// to this computed value
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the rule, which is unique to each account.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(MIN_RULE_NAME_LENGTH, MAX_RULE_NAME_LENGTH),
				},
			},
			"description": schema.StringAttribute{
				Description: "Description of the rule",
				Required:    true,
			},
			"spec_version": schema.Int64Attribute{
				Description: "Rule evaluation specification version in the case of breaking changes.",
				Default:     int64default.StaticInt64(1),
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"polling_interval": schema.StringAttribute{
				Description: "Frequency of automated rule evaluation. Defaults to ONE_DAY.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(string(client.SchedulerPollingIntervalOneDay)),
				Validators: []validator.String{
					stringvalidator.OneOf(PollingIntervals...),
				},
			},
			"templates": schema.MapAttribute{
				Description: "Optional key/value pairs of template name to template",
				ElementType: types.StringType,
				Optional:    true,
			},
			"question_id": schema.StringAttribute{
				Description: "Specifies the ID of a question to be used in rule evaluation.",
				Optional:    true,
			},
			"operations": schema.ListNestedAttribute{
				Description: "Actions that are executed when a corresponding condition is met.",
				Required:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"when": schema.StringAttribute{
							Optional: true,
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(MIN_JSON_LENGTH),
							},
							PlanModifiers: []planmodifier.String{
								jsonIgnoreDiffPlanModifier(),
							},
						},
						"actions": schema.ListAttribute{
							Required:    true,
							ElementType: types.StringType,
							Validators:  []validator.List{},
							PlanModifiers: []planmodifier.List{
								jsonIgnoreDiffPlanModifierList(),
							},
						},
					},
				},
			},
			"outputs": schema.ListAttribute{
				Description: "Names of properties that can be used throughout the rule evaluation process and will be included in each record of a rule evaluation. (e.g. queries.query0.total)",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Default:     listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{})),
			},
			"tags": schema.ListAttribute{
				Description: "Comma separated list of tags to apply to the rule.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Default:     listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{})),
			},
			"notify_on_failure": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"trigger_on_new_only": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"ignore_previous_results": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"labels": schema.ListNestedAttribute{
				Description: "Comma separated list of labelName/labelValue pairs to apply to the rule.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.List{
					useEmptyListForNullWithType(map[string]attr.Type{
						"label_name":  types.StringType,
						"label_value": types.StringType,
					}),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"label_name": schema.StringAttribute{
							Required:    true,
							Description: "Name of the label",
						},
						"label_value": schema.StringAttribute{
							Required:    true,
							Description: "Value of the label, which is represented in TF as a string but will be treated as string, number or boolean in the application",
						},
					},
				},
			},
			"resource_group_id": schema.StringAttribute{
				Optional:    true,
				Description: "Specifies the ID of a resource group for the rule to be added to",
			},
		},
		// TODO: Deprecate the use of blocks following new framework guidance:
		// https://developer.hashicorp.com/terraform/plugin/framework/handling-data/blocks
		Blocks: map[string]schema.Block{
			"question": schema.ListNestedBlock{
				Description: "Contains properties related to queries used in the rule evaluation.",
				NestedObject: schema.NestedBlockObject{
					Blocks: map[string]schema.Block{
						"queries": schema.ListNestedBlock{
							Description: "Contains properties related to queries used in the rule evaluation.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Optional: true,
										Validators: []validator.String{
											stringvalidator.LengthBetween(MIN_RULE_NAME_LENGTH, MAX_RULE_NAME_LENGTH),
										},
									},
									"query": schema.StringAttribute{
										Required: true,
										Validators: []validator.String{
											stringvalidator.LengthAtLeast(1),
										},
									},
									"version": schema.StringAttribute{
										Required: true,
										Validators: []validator.String{
											stringvalidator.LengthBetween(MIN_RULE_NAME_LENGTH, MAX_RULE_NAME_LENGTH),
										},
									},
									"include_deleted": schema.BoolAttribute{
										Optional: true,
										Computed: true,
										Default:  booldefault.StaticBool(false),
									},
								},
							},
							Validators: []validator.List{
								listvalidator.SizeAtLeast(1),
							},
						},
					},
				},
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
			},
		},
	}
}

// ConfigValidators implements resource.ResourceWithConfigValidators
func (*QuestionRuleResource) ConfigValidators(context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.Conflicting(
			path.MatchRoot("question"),
			path.MatchRoot("question_id"),
		),
		resourcevalidator.AtLeastOneOf(
			path.MatchRoot("question"),
			path.MatchRoot("question_id"),
		),
		resourcevalidator.Conflicting(
			path.MatchRoot("trigger_on_new_only"),
			path.MatchRoot("ignore_previous_results"),
		),
	}
}

// ModifyPlan is a workaround for unexpected behavior in the framework around
// the `computed: true` `version` field to make sure that it is only part of
// the plan if there is some other change in the resource.
//
// Based on the implementation of the Time resource:
// https://github.com/hashicorp/terraform-provider-time/blob/main/internal/provider/resource_time_rotating.go#L189-L234
//
// This may be a bug in the framework, if so, this can be removed when fixed:
// https://github.com/hashicorp/terraform-plugin-framework/issues/628
func (*QuestionRuleResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Plan does not need to be modified when the resource is being destroyed.
	if req.Plan.Raw.IsNull() {
		return
	}

	// Plan only needs modifying if the resource already exists as the purpose of
	// the plan modifier is to show updated attribute values on CLI.
	if req.State.Raw.IsNull() {
		return
	}

	var plan, state *RuleModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if !reflect.DeepEqual(plan, state) {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("version"),
			types.Int64Unknown())...)
	}
}

type Versioner interface {
	GetVersion() int
}

type IdVersioner interface {
	Versioner
	GetId() string
}

// Create implements resource.ResourceWithConfigure
func (r *QuestionRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *RuleModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var c IdVersioner
	if len(data.Question) > 0 {
		rule, err := data.BuildCreateInlineQuestionRuleInstanceInput()
		if err != nil {
			resp.Diagnostics.AddError("failed to build rule from configuration", err.Error())
			return
		}

		created, err := client.CreateInlineQuestionRuleInstance(ctx, r.qlient, rule)
		if err != nil {
			resp.Diagnostics.AddError("failed to create rule", err.Error())
			return
		}
		c = &created.CreateQuestionRuleInstance
	} else {
		rule, err := data.BuildCreateReferencedQuestionRuleInstanceInput()
		if err != nil {
			resp.Diagnostics.AddError("failed to build rule from configuration", err.Error())
			return
		}

		created, err := client.CreateReferencedQuestionRuleInstance(ctx, r.qlient, rule)
		if err != nil {
			resp.Diagnostics.AddError("failed to create rule", err.Error())
			return
		}
		c = &created.CreateQuestionRuleInstance
	}

	data.Id = types.StringValue(c.GetId())
	data.Version = types.Int64Value(int64(c.GetVersion()))

	var diags diag.Diagnostics
	data.Labels, diags = ensureLabelsInitialized(data.Labels)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	tflog.Trace(ctx, "Created rule",
		map[string]interface{}{"title": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete implements resource.ResourceWithConfigure
func (r *QuestionRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RuleModel

	// Read Terraform ste into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.DeleteRuleInstance(ctx, r.qlient, data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to delete rule", err.Error())
	}
}

// Read implements resource.ResourceWithConfigure
func (r *QuestionRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var oldData RuleModel

	// Read Terraform state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &oldData)...)

	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := client.GetQuestionRuleInstance(ctx, r.qlient, oldData.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("failed to get rule", err.Error())
		}
		return
	}
	rule := getResp.QuestionRuleInstance

	outputs := rule.Outputs
	if outputs == nil {
		outputs = []string{}
	}

	// Convert outputs to types.List
	outputValues := make([]attr.Value, len(outputs))
	for i, output := range outputs {
		outputValues[i] = types.StringValue(output)
	}
	outputsListValue, diags := types.ListValue(types.StringType, outputValues)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Convert tags to types.List
	tags := rule.Tags
	tagValues := make([]attr.Value, len(tags))
	for i, tag := range tags {
		tagValues[i] = types.StringValue(tag)
	}
	tagsListValue, diags := types.ListValue(types.StringType, tagValues)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Log a debug message with the rule labels
	tflog.Debug(ctx, "Rule tags", map[string]interface{}{"tags": rule.Tags})

	data := RuleModel{
		Id:                    types.StringValue(rule.Id),
		Name:                  types.StringValue(rule.Name),
		Description:           types.StringValue(rule.Description),
		Version:               types.Int64Value(int64(rule.Version)),
		SpecVersion:           types.Int64Value(int64(rule.SpecVersion)),
		PollingInterval:       types.StringValue(string(rule.PollingInterval)),
		Outputs:               outputsListValue,
		Tags:                  tagsListValue,
		NotifyOnFailure:       types.BoolValue(rule.NotifyOnFailure),
		TriggerOnNewOnly:      types.BoolValue(rule.TriggerActionsOnNewEntitiesOnly),
		IgnorePreviousResults: types.BoolValue(rule.IgnorePreviousResults),
	}

	// FIXME: handling of these JSON fields (map[string]interface{}) is not DRY
	templates, err := json.Marshal(rule.Templates)
	if err != nil {
		resp.Diagnostics.AddError("error marshaling templates from response", err.Error())
	}
	err = json.Unmarshal(templates, &data.Templates)
	if err != nil {
		resp.Diagnostics.AddError("error unmarshaling templates from response", err.Error())
	}

	if rule.ResourceGroupId != "" {
		data.ResourceGroupId = types.StringValue(rule.ResourceGroupId)
	} else {
		data.ResourceGroupId = types.StringNull()
	}

	if rule.QuestionId != "" {
		data.QuestionId = types.StringValue(rule.QuestionId)
	} else {
		data.QuestionId = types.StringNull()
	}
	if queries := rule.Question.Queries; len(queries) > 0 {
		queryModels := make([]*J1QueryInputModel, len(queries))
		for i, query := range queries {
			queryModels[i] = &J1QueryInputModel{
				Name:            query.Name,
				Query:           query.Query,
				Version:         query.Version,
				IncludedDeleted: query.IncludeDeleted,
			}
		}
		data.Question = []*RuleQuestion{
			{
				Queries: queryModels,
			},
		}
	} else {
		data.Question = nil
	}

	data.Operations, err = newOperationsWithoutId(rule.Operations)
	if err != nil {
		resp.Diagnostics.AddError("error unmarshaling templates from response", err.Error())
	}

	// Convert labels to types.List
	labelsListValue, err := convertLabelsToTerraformList(rule.Labels)
	if err != nil {
		resp.Diagnostics.AddError("failed to convert labels", err.Error())
		return
	}
	data.Labels = labelsListValue

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ImportState implements resource.ResourceWithImportState
func (*QuestionRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Update implements resource.ResourceWithConfigure
func (r *QuestionRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RuleModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// The UpdateRule operation needs the most current version of the rule to update it.
	// We fetch it from the state if it is not specified by the user.
	if data.Version.IsUnknown() {
		var state RuleModel
		resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
		if resp.Diagnostics.HasError() {
			return
		}
		data.Version = state.Version
	}

	var update Versioner
	if len(data.Question) > 0 {
		rule, err := data.BuildUpdateInlineQuestionRuleInstanceInput()
		if err != nil {
			resp.Diagnostics.AddError("failed to build rule from configuration", err.Error())
			return
		}

		updated, err := client.UpdateInlineQuestionRuleInstance(ctx, r.qlient, rule)
		if err != nil {
			resp.Diagnostics.AddError("failed to update inline question rule", err.Error())
			return
		}
		update = &updated.UpdateInlineQuestionRuleInstance
	} else {
		rule, err := data.BuildUpdateReferencedQuestionRuleInstanceInput()
		if err != nil {
			resp.Diagnostics.AddError("failed to build rule from configuration", err.Error())
			return
		}

		updated, err := client.UpdateReferencedQuestionRuleInstance(ctx, r.qlient, rule)
		if err != nil {
			resp.Diagnostics.AddError("failed to update referenced question rule", err.Error())
			return
		}
		update = &updated.UpdateReferencedQuestionRuleInstance
	}

	data.Version = types.Int64Value(int64(update.GetVersion()))

	var diags diag.Diagnostics
	data.Labels, diags = ensureLabelsInitialized(data.Labels)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	tflog.Trace(ctx, "Updated rule",
		map[string]interface{}{"title": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r *RuleModel) buildOperations() ([]client.RuleOperationInput, error) {
	ops := make([]client.RuleOperationInput, 0, len(r.Operations))
	for _, o := range r.Operations {
		op := client.RuleOperationInput{}
		if !o.When.IsNull() {
			err := json.Unmarshal([]byte(o.When.ValueString()), &op.When)
			if err != nil {
				return nil, err
			}
		}

		for _, action := range o.Actions {
			var a map[string]interface{}
			err := json.Unmarshal([]byte(action), &a)
			if err != nil {
				return nil, err
			}
			// NOTE: "id" should not be saved as currently implemented, so any
			// "id" value in the input would be coming from the config
			delete(a, "id")
			op.Actions = append(op.Actions, a)
		}

		ops = append(ops, op)
	}
	return ops, nil
}

func (r *RuleModel) BuildCreateReferencedQuestionRuleInstanceInput() (client.CreateReferencedQuestionRuleInstanceInput, error) {
	rule := client.CreateReferencedQuestionRuleInstanceInput{
		QuestionId:                      r.QuestionId.ValueString(),
		Tags:                            tagsToStringSlice(r.Tags),
		Labels:                          labelsToClientLabels(r.Labels),
		Name:                            r.Name.ValueString(),
		Description:                     r.Description.ValueString(),
		SpecVersion:                     int(r.SpecVersion.ValueInt64()),
		Outputs:                         outputsToStringSlice(r.Outputs),
		PollingInterval:                 client.SchedulerPollingInterval(r.PollingInterval.ValueString()),
		NotifyOnFailure:                 r.NotifyOnFailure.ValueBool(),
		TriggerActionsOnNewEntitiesOnly: r.TriggerOnNewOnly.ValueBool(),
		IgnorePreviousResults:           r.IgnorePreviousResults.ValueBool(),
		ResourceGroupId:                 r.ResourceGroupId.ValueString(),
	}

	var err error
	rule.Operations, err = r.buildOperations()
	if err != nil {
		return rule, err
	}
	templates, err := json.Marshal(r.Templates)
	if err != nil {
		return rule, err
	}
	err = json.Unmarshal(templates, &rule.Templates)
	if err != nil {
		return rule, err
	}

	return rule, nil
}

func (r *RuleModel) BuildUpdateReferencedQuestionRuleInstanceInput() (client.UpdateReferencedQuestionRuleInstanceInput, error) {
	rule := client.UpdateReferencedQuestionRuleInstanceInput{
		Id:                              r.Id.ValueString(),
		Name:                            r.Name.ValueString(),
		Description:                     r.Description.ValueString(),
		Version:                         int(r.Version.ValueInt64()),
		SpecVersion:                     int(r.SpecVersion.ValueInt64()),
		QuestionId:                      r.QuestionId.ValueString(),
		PollingInterval:                 client.SchedulerPollingInterval(r.PollingInterval.ValueString()),
		Outputs:                         outputsToStringSlice(r.Outputs),
		Tags:                            tagsToStringSlice(r.Tags),
		Labels:                          labelsToClientLabels(r.Labels),
		NotifyOnFailure:                 r.NotifyOnFailure.ValueBool(),
		TriggerActionsOnNewEntitiesOnly: r.TriggerOnNewOnly.ValueBool(),
		IgnorePreviousResults:           r.IgnorePreviousResults.ValueBool(),
		ResourceGroupId:                 r.ResourceGroupId.ValueString(),
	}

	var err error
	rule.Operations, err = r.buildOperations()
	if err != nil {
		return rule, err
	}
	templates, err := json.Marshal(r.Templates)
	if err != nil {
		return rule, err
	}
	err = json.Unmarshal(templates, &rule.Templates)
	if err != nil {
		return rule, err
	}

	rule.Operations, err = r.buildOperations()
	if err != nil {
		return rule, err
	}

	return rule, nil
}

func (r *RuleModel) BuildCreateInlineQuestionRuleInstanceInput() (client.CreateInlineQuestionRuleInstanceInput, error) {
	rule := client.CreateInlineQuestionRuleInstanceInput{
		Tags:                            tagsToStringSlice(r.Tags),
		Labels:                          labelsToClientLabels(r.Labels),
		Name:                            r.Name.ValueString(),
		Description:                     r.Description.ValueString(),
		SpecVersion:                     int(r.SpecVersion.ValueInt64()),
		Outputs:                         outputsToStringSlice(r.Outputs),
		PollingInterval:                 client.SchedulerPollingInterval(r.PollingInterval.ValueString()),
		NotifyOnFailure:                 r.NotifyOnFailure.ValueBool(),
		TriggerActionsOnNewEntitiesOnly: r.TriggerOnNewOnly.ValueBool(),
		IgnorePreviousResults:           r.IgnorePreviousResults.ValueBool(),
		ResourceGroupId:                 r.ResourceGroupId.ValueString(),
	}

	var err error
	rule.Operations, err = r.buildOperations()
	if err != nil {
		return rule, err
	}
	templates, err := json.Marshal(r.Templates)
	if err != nil {
		return rule, err
	}
	err = json.Unmarshal(templates, &rule.Templates)
	if err != nil {
		return rule, err
	}

	if len(r.Question) > 0 && len(r.Question[0].Queries) > 0 {
		queries := make([]client.J1QueryInput, len(r.Question[0].Queries))
		for i, query := range r.Question[0].Queries {
			queries[i] = client.J1QueryInput{
				Query:          query.Query,
				Name:           query.Name,
				Version:        query.Version,
				IncludeDeleted: query.IncludedDeleted,
			}
		}
		rule.Question = client.RuleQuestionDetailsInput{
			Queries: queries,
		}
	}

	return rule, nil
}

func (r *RuleModel) BuildUpdateInlineQuestionRuleInstanceInput() (client.UpdateInlineQuestionRuleInstanceInput, error) {
	rule := client.UpdateInlineQuestionRuleInstanceInput{
		Id:                              r.Id.ValueString(),
		Version:                         int(r.Version.ValueInt64()),
		State:                           client.RuleStateInput{},
		Tags:                            tagsToStringSlice(r.Tags),
		Labels:                          labelsToClientLabels(r.Labels),
		Name:                            r.Name.ValueString(),
		Description:                     r.Description.ValueString(),
		SpecVersion:                     int(r.SpecVersion.ValueInt64()),
		Outputs:                         outputsToStringSlice(r.Outputs),
		PollingInterval:                 client.SchedulerPollingInterval(r.PollingInterval.ValueString()),
		NotifyOnFailure:                 r.NotifyOnFailure.ValueBool(),
		TriggerActionsOnNewEntitiesOnly: r.TriggerOnNewOnly.ValueBool(),
		IgnorePreviousResults:           r.IgnorePreviousResults.ValueBool(),
		ResourceGroupId:                 r.ResourceGroupId.ValueString(),
	}

	var err error
	rule.Operations, err = r.buildOperations()
	if err != nil {
		return rule, err
	}

	templates, err := json.Marshal(r.Templates)
	if err != nil {
		return rule, err
	}
	err = json.Unmarshal(templates, &rule.Templates)
	if err != nil {
		return rule, err
	}

	if len(r.Question) > 0 && len(r.Question[0].Queries) > 0 {
		queries := make([]client.J1QueryInput, len(r.Question[0].Queries))
		for i, query := range r.Question[0].Queries {
			queries[i] = client.J1QueryInput{
				Query:          query.Query,
				Name:           query.Name,
				Version:        query.Version,
				IncludeDeleted: query.IncludedDeleted,
			}
		}
		rule.Question = client.RuleQuestionDetailsInput{
			Queries: queries,
		}
	}

	return rule, nil
}

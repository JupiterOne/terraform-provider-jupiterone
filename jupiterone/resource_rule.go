package jupiterone

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

const MIN_RULE_NAME_LENGTH = 1
const MAX_RULE_NAME_LENGTH = 255

// Ensure provider defined types fully satisfy framework interfaces
var _ resource.Resource = &QuestionRuleResource{}
var _ resource.ResourceWithConfigure = &QuestionRuleResource{}
var _ resource.ResourceWithConfigValidators = &QuestionRuleResource{}
var _ resource.ResourceWithModifyPlan = &QuestionRuleResource{}

type QuestionRuleResource struct {
	version string
	client  *client.JupiterOneClient
}

type RuleQuestion struct {
	Queries []*QueryModel `json:"queries" tfsdk:"queries"`
}

// RuleModel represents the terraform representation of the rule
type RuleModel struct {
	Id              types.String      `json:"id,omitempty" tfsdk:"id"`
	Name            string            `json:"name" tfsdk:"name"`
	Description     string            `json:"description" tfsdk:"description"`
	Version         types.Int64       `json:"version,omitempty" tfsdk:"version"`
	SpecVersion     types.Int64       `json:"specVersion,omitempty" tfsdk:"spec_version"`
	PollingInterval string            `json:"pollingInterval" tfsdk:"polling_interval"`
	Templates       map[string]string `json:"templates" tfsdk:"templates"`
	Question        []*RuleQuestion   `json:"question,omitempty" tfsdk:"question"`
	QuestionId      types.String      `json:"questionId,omitempty" tfsdk:"question_id"`
	QuestionName    types.String      `json:"questionName,omitempty" tfsdk:"question_name"`
	// Operations TODO: breaking change for new version to do more in the
	// HCL and/or make better use of things like jsonencode
	Operations string   `json:"operations" tfsdk:"operations"`
	Outputs    []string `json:"outputs" tfsdk:"outputs"`
	Tags       []string `json:"tags" tfsdk:"tags"`
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
	r.client = p.Client
}

// Schema implements resource.ResourceWithConfigure
func (*QuestionRuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	var RulePollingIntervals = []string{"DISABLED", "THIRTY_MINUTES", "ONE_HOUR", "ONE_DAY", "ONE_WEEK"}

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
				Computed:    true,
				Optional:    true,
				PlanModifiers: []planmodifier.Int64{
					Int64DefaultValue(types.Int64Value(1)),
				},
			},
			"polling_interval": schema.StringAttribute{
				Description: "Frequency of automated rule evaluation. Defaults to ONE_DAY.",
				Computed:    true,
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					StringDefaultValue(RulePollingIntervals[3]),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(RulePollingIntervals...),
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
			"question_name": schema.StringAttribute{
				Description:        "Specifies the name of a question to be used in rule evaluation.",
				DeprecationMessage: "The question_name identifier is deprecated. Prefer to use a question's id property with question_id to reference a jupiterone_question in a jupiterone_rule.",
				Optional:           true,
			},
			"operations": schema.StringAttribute{
				Description: "Actions that are executed when a corresponding condition is met.",
				Required:    true,
				// PlanModifiers currently tries to diff the json objects and
				// ignore formatting changes, but long term should probably
				// be a TODO for a more complete schema and encouraging
				// jsonencode() usage instead.
				PlanModifiers: []planmodifier.String{
					jsonIgnoreDiffPlanModifier(),
				},
				// TODO: similar to above, longer term is to define more of the
				// schema for HCL and encourage use of jsonencode()
				// Alternative: Look for a JSONString CustomType that comes
				// with it's own validation in it's marshalling
				Validators: []validator.String{
					jsonValidator{},
				},
			},
			"outputs": schema.ListAttribute{
				Description: "Names of properties that can be used throughout the rule evaluation process and will be included in each record of a rule evaluation. (e.g. queries.query0.total)",
				ElementType: types.StringType,
				Optional:    true,
			},
			"tags": schema.ListAttribute{
				Description: "Comma separated list of tags to apply to the rule.",
				ElementType: types.StringType,
				Optional:    true,
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
								Attributes: questionQuerySchemaAttributes(),
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
			path.MatchRoot("question_name"),
		),
		resourcevalidator.AtLeastOneOf(
			path.MatchRoot("question"),
			path.MatchRoot("question_id"),
			path.MatchRoot("question_name"),
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

// Create implements resource.ResourceWithConfigure
func (r *QuestionRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *RuleModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: in future versions, make operations more explicit in the schema
	// TODO: This map can probably be replaces by existing structs as well
	rule, err := data.BuildQuestionRuleInstance()
	if err != nil {
		resp.Diagnostics.AddError("failed to build rule from configuration", err.Error())
		return
	}

	rule, err = r.client.CreateQuestionRuleInstance(rule)
	if err != nil {
		resp.Diagnostics.AddError("failed to create rule", err.Error())
		return
	}

	data.Id = types.StringValue(rule.Id)
	data.Version = types.Int64Value(int64(rule.Version))

	// TODO: This should _probably_ be done whenever values are read from the server
	// and stored back to state, but version 0.5.0 didn't do this and relied
	// solely on the suppress diff func, so leave this out for test/state
	// compatibility until more improvements worth upgrading the state for are
	// implemented
	/* data.Operations, err = processOperationsState(rule.Operations)
	if err != nil {
		resp.Diagnostics.AddError("failed to save operations state", err.Error())
		return
	} */

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

	if err := r.client.DeleteQuestionRuleInstance(data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to delete rule", err.Error())
	}
}

// Read implements resource.ResourceWithConfigure
func (r *QuestionRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RuleModel

	// Read Terraform ste into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := r.client.GetQuestionRuleInstanceByID(data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to get rule", err.Error())
		return
	}

	data = RuleModel{
		Id:              types.StringValue(rule.Id),
		Name:            rule.Name,
		Description:     rule.Description,
		Version:         types.Int64Value(int64(rule.Version)),
		SpecVersion:     types.Int64Value(int64(rule.SpecVersion)),
		PollingInterval: rule.PollingInterval,
		Templates:       rule.Templates,
		Outputs:         rule.Outputs,
		Tags:            rule.Tags,
	}

	if rule.QuestionId != "" {
		data.QuestionId = types.StringValue(rule.QuestionId)
	}
	if rule.QuestionName != "" {
		data.QuestionName = types.StringValue(rule.QuestionName)
	}
	if queries := rule.Question["queries"]; len(queries) > 0 {
		data.Question = []*RuleQuestion{{Queries: []*QueryModel{
			{
				Name:    queries[0]["name"],
				Query:   queries[0]["query"],
				Version: queries[0]["version"],
			},
		}}}
	}

	data.Operations, err = processOperationsState(rule.Operations)
	if err != nil {
		resp.Diagnostics.AddError("failed to process operations for rule", err.Error())
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update implements resource.ResourceWithConfigure
func (r *QuestionRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RuleModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

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

	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: in future versions, make operations more explicit in the schema
	// TODO: This map can probably be replaces by existing structs as well
	rule, err := data.BuildQuestionRuleInstance()
	if err != nil {
		resp.Diagnostics.AddError("failed to create rule", err.Error())
		return
	}

	rule, err = r.client.UpdateQuestionRuleInstance(rule)
	if err != nil {
		resp.Diagnostics.AddError("failed to create rule", err.Error())
		return
	}

	data.Id = types.StringValue(rule.Id)
	data.Version = types.Int64Value(int64(rule.Version))

	// TODO: This should _probably_ be done whenever values are read from the server
	// and stored back to state, but version 0.5.0 only did this on Read and relied
	// solely on the suppress diff func, so leave this out for test/state
	// compatibility until more improvements worth upgrading the state for are
	// implemented
	//data.Operations, err = processOperationsState(rule.Operations)
	//if err != nil {
	//	resp.Diagnostics.AddError("failed to save operations state", err.Error())
	//	return
	//}

	tflog.Trace(ctx, "Updated rule",
		map[string]interface{}{"title": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func processOperationsState(ruleOperations []client.RuleOperation) (string, error) {
	// Because we store the Operations as a raw string. The id property, which
	// is set after the creation of the question creates a diff that would cause
	// the Operations to update on every terraform apply
	for _, op := range ruleOperations {
		for _, action := range op.Actions {
			delete(action, "id")
		}
	}

	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(ruleOperations)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (r *RuleModel) BuildQuestionRuleInstance() (*client.QuestionRuleInstance, error) {
	rule := &client.QuestionRuleInstance{
		Id:              r.Id.ValueString(),
		Name:            r.Name,
		Description:     r.Description,
		SpecVersion:     int(r.SpecVersion.ValueInt64()),
		Version:         int(r.Version.ValueInt64()),
		PollingInterval: r.PollingInterval,
		Templates:       r.Templates,
		QuestionId:      r.QuestionId.ValueString(),
		QuestionName:    r.QuestionName.ValueString(),
		Outputs:         r.Outputs,
		Tags:            r.Tags,
	}

	ops := make([]client.RuleOperation, 0)
	err := json.Unmarshal([]byte(r.Operations), &ops)
	if err != nil {
		return nil, err
	}
	rule.Operations = ops

	if len(r.Question) > 0 && len(r.Question[0].Queries) > 0 {
		rule.Question = map[string][]map[string]string{
			"queries": {
				{
					"name":    r.Question[0].Queries[0].Name,
					"query":   r.Question[0].Queries[0].Query,
					"version": r.Question[0].Queries[0].Version,
				},
			},
		}
	}

	return rule, nil
}

func jsonIgnoreDiffPlanModifier() planmodifier.String {
	return jsonIgnoreDiff{}
}

type jsonIgnoreDiff struct {
}

// Description implements planmodifier.String
func (jsonIgnoreDiff) Description(context.Context) string {
	return "Compares json for object equality to ignore formatting changes"
}

// MarkdownDescription implements planmodifier.String
func (j jsonIgnoreDiff) MarkdownDescription(ctx context.Context) string {
	return j.Description(ctx)
}

// PlanModifyString implements planmodifier.String
func (jsonIgnoreDiff) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// always apply new values
	if req.StateValue.ValueString() == "" {
		return
	}

	var oldValue interface{}
	err := json.Unmarshal([]byte(req.StateValue.ValueString()), &oldValue)
	if err != nil {
		resp.Diagnostics.AddError("Invalid operations json in old state", err.Error())
		return
	}

	var newValue interface{}
	err = json.Unmarshal([]byte(req.PlanValue.ValueString()), &newValue)
	if err != nil {
		resp.Diagnostics.AddError("Invalid operations json in old state", err.Error())
		return
	}

	if reflect.DeepEqual(oldValue, newValue) {
		resp.PlanValue = req.StateValue
	}
}

func Int64DefaultValue(v types.Int64) planmodifier.Int64 {
	return &int64DefaultValuePlanModifier{v}
}

// int64DefaultValuePlanModifier is based on the example at:
// https://developer.hashicorp.com/terraform/plugin/framework/migrating/attributes-blocks/default-values
type int64DefaultValuePlanModifier struct {
	DefaultValue types.Int64
}

var _ planmodifier.Int64 = (*int64DefaultValuePlanModifier)(nil)

func (apm *int64DefaultValuePlanModifier) Description(ctx context.Context) string {
	return "sets a default value for an int64 value"
}

func (apm *int64DefaultValuePlanModifier) MarkdownDescription(ctx context.Context) string {
	return apm.Description(ctx)
}

func (apm *int64DefaultValuePlanModifier) PlanModifyInt64(ctx context.Context, req planmodifier.Int64Request, res *planmodifier.Int64Response) {
	// If the attribute configuration is not null, we are done here
	if !req.ConfigValue.IsNull() {
		return
	}

	// If the attribute plan is "known" and "not null", then a previous plan modifier in the sequence
	// has already been applied, and we don't want to interfere.
	if !req.PlanValue.IsUnknown() && !req.PlanValue.IsNull() {
		return
	}

	res.PlanValue = apm.DefaultValue
}

func StringDefaultValue(v string) planmodifier.String {
	return &stringDefaultValuePlanModifier{
		DefaultValue: types.StringValue(v),
	}
}

type stringDefaultValuePlanModifier struct {
	DefaultValue types.String
}

var _ planmodifier.String = (*stringDefaultValuePlanModifier)(nil)

func (apm *stringDefaultValuePlanModifier) Description(ctx context.Context) string {
	return "sets a default value for an string value"
}

func (apm *stringDefaultValuePlanModifier) MarkdownDescription(ctx context.Context) string {
	return apm.Description(ctx)
}

func (apm *stringDefaultValuePlanModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, res *planmodifier.StringResponse) {
	// If the attribute configuration is not null, we are done here
	if !req.ConfigValue.IsNull() {
		return
	}

	// If the attribute plan is "known" and "not null", then a previous plan modifier in the sequence
	// has already been applied, and we don't want to interfere.
	if !req.PlanValue.IsUnknown() && !req.PlanValue.IsNull() {
		return
	}

	res.PlanValue = apm.DefaultValue
}

var _ validator.String = jsonValidator{}

// oneOfValidator validates that the value matches one of expected values.
type jsonValidator struct {
}

// Description implements validator.String
func (jsonValidator) Description(context.Context) string {
	return "string value must be valid JSON"
}

// MarkdownDescription implements validator.String
func (v jsonValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

// ValidateString implements validator.String
func (v jsonValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsUnknown() {
		return
	}

	var d interface{}
	err := json.Unmarshal([]byte(req.ConfigValue.ValueString()), &d)
	if err != nil {
		resp.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
			req.Path,
			v.Description(ctx),
			req.ConfigValue.String(),
		))
	}
}

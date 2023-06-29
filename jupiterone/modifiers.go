package jupiterone

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func BoolDefaultValuePlanModifier(b bool) planmodifier.Bool {
	return &boolDefaultValuePlanModifier{
		DefaultValue: types.BoolValue(b),
	}
}

type boolDefaultValuePlanModifier struct {
	DefaultValue types.Bool
}

var _ planmodifier.Bool = (*boolDefaultValuePlanModifier)(nil)

func (pm *boolDefaultValuePlanModifier) Description(ctx context.Context) string {
	return "sets a default value for a bool value"
}

func (pm *boolDefaultValuePlanModifier) MarkdownDescription(ctx context.Context) string {
	return pm.Description(ctx)
}

func (pm *boolDefaultValuePlanModifier) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, res *planmodifier.BoolResponse) {
	// If the attribute configuration is not null, we are done here
	if !req.ConfigValue.IsNull() {
		return
	}

	// If the attribute plan is "known" and "not null", then a previous plan modifier in the sequence
	// has already been applied, and we don't want to interfere.
	if !req.PlanValue.IsUnknown() && !req.PlanValue.IsNull() {
		return
	}

	res.PlanValue = pm.DefaultValue
}

var _ planmodifier.String = (*jsonIgnoreDiff)(nil)
var _ planmodifier.List = (*jsonIgnoreDiff)(nil)
var _ planmodifier.Map = (*jsonIgnoreDiff)(nil)

func jsonIgnoreDiffPlanModifier() planmodifier.String {
	return jsonIgnoreDiff{}
}

func jsonIgnoreDiffPlanModifierList() planmodifier.List {
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
func (j jsonIgnoreDiff) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.ConfigValue.IsNull() {
		return
	}

	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}

	// always apply new values
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}

	var oldValue interface{}
	err := json.Unmarshal([]byte(req.StateValue.ValueString()), &oldValue)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Invalid json in state for %s", req.Path), err.Error())
		return
	}

	var newValue interface{}
	err = json.Unmarshal([]byte(req.PlanValue.ValueString()), &newValue)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Invalid json in plan for %s", req.Path), err.Error())
		return
	}

	if reflect.DeepEqual(oldValue, newValue) {
		resp.PlanValue = req.StateValue
	}
}

// PlanModifyList implements planmodifier.List
func (j jsonIgnoreDiff) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if req.ConfigValue.IsNull() {
		return
	}

	if req.StateValue.IsUnknown() || req.StateValue.IsNull() {
		return
	}

	var vals []types.String
	err := req.ConfigValue.ElementsAs(ctx, &vals, false)
	if err != nil {
		resp.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
			req.Path,
			"not a valid string: "+j.Description(ctx),
			req.ConfigValue.String(),
		))
		return
	}

	var state []types.String
	err = req.StateValue.ElementsAs(ctx, &state, false)
	if err != nil {
		resp.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
			req.Path,
			"not a valid string: "+j.Description(ctx),
			req.StateValue.String(),
		))
		return
	}

	if len(vals) != len(state) {
		// don't try to match up because there is going to an add or remove
		// that must be processed
		return
	}

	// TODO: in theory, the order doesn't matter if the ids match, so try
	// to match up ids instead of just relying on the order
	for i := 0; i < len(vals); i++ {
		if vals[i].IsNull() || vals[i].IsUnknown() {
			// this would be weird, but without matching up list elements,
			// just let it trigger for now
			return
		}

		if state[i].IsNull() || state[i].IsUnknown() {
			return
		}

		var oldValue map[string]interface{}
		err := json.Unmarshal([]byte(state[i].ValueString()), &oldValue)
		if err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Invalid json in plan for %s", req.Path), err.Error())
			return
		}
		delete(oldValue, "id")

		var newValue map[string]interface{}
		err = json.Unmarshal([]byte(vals[i].ValueString()), &newValue)
		if err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Invalid json in state for %s", req.Path), err.Error())
			return
		}
		delete(newValue, "id")

		if !reflect.DeepEqual(oldValue, newValue) {
			return
		}
	}

	resp.PlanValue = req.StateValue
}

// PlanModifyMap implements planmodifier.Map
func (jsonIgnoreDiff) PlanModifyMap(ctx context.Context, req planmodifier.MapRequest, resp *planmodifier.MapResponse) {
	if req.ConfigValue.IsNull() {
		return
	}

	if req.StateValue.IsUnknown() || req.StateValue.IsNull() {
		return
	}

	oldValues := req.StateValue.Elements()
	for k, v := range req.PlanValue.Elements() {
		o, ok := oldValues[k]
		if !ok {
			// something new, go ahead with the plan
			return
		}

		s, ok := v.(types.String)
		if !ok {
			// this is very bad and shouldn't haven't gotten past validation
			resp.Diagnostics.AddError(fmt.Sprintf("Invalid value type for json string in plan for %s", req.Path), "invalid value")
			return
		}

		var newValue map[string]interface{}
		err := json.Unmarshal([]byte(s.ValueString()), &newValue)
		if err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Invalid json in plan for %s", req.Path), err.Error())
			return
		}

		s, ok = o.(types.String)
		if !ok {
			// this is also very bad and shouldn't haven't gotten into state,
			// but continue and let apply try to save a new valid value
			resp.Diagnostics.AddWarning(fmt.Sprintf("Invalid json in state for %s, this likely a bug in the provider", req.Path), err.Error())
			return
		}
		var oldValue map[string]interface{}
		err = json.Unmarshal([]byte(s.ValueString()), &oldValue)
		if err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Invalid json in plan for %s", req.Path), err.Error())
			return
		}

		if !reflect.DeepEqual(oldValue, newValue) {
			return
		}

		delete(oldValues, k)
	}

	if len(oldValues) > 0 {
		// something was removed in the plan
		return
	}

	resp.PlanValue = req.StateValue
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

// var _ validator.List = jsonValidator{}
var _ validator.Map = jsonValidator{}

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

	// TODO: check if optional?
	if req.ConfigValue.IsNull() {
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

// ValidateList implements validator.List
// func (v jsonValidator) ValidateList(ctx context.Context, req validator.ListRequest, resp *validator.ListResponse) {
// 	//
// }

// ValidateMap implements validator.Map
func (v jsonValidator) ValidateMap(ctx context.Context, req validator.MapRequest, resp *validator.MapResponse) {
	for _, val := range req.ConfigValue.Elements() {
		s, ok := val.(types.String)
		if !ok {
			resp.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
				req.Path,
				v.Description(ctx),
				val.String(),
			))
		}

		var d interface{}
		err := json.Unmarshal([]byte(s.ValueString()), &d)
		if err != nil {
			resp.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
				req.Path,
				v.Description(ctx),
				req.ConfigValue.String(),
			))
		}
	}
}

package jupiterone

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

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
	if req.StateValue.IsUnknown() || req.StateValue.IsNull() {
		return
	}

	var state []string
	err := req.StateValue.ElementsAs(ctx, &state, false)
	if err != nil {
		resp.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
			req.Path,
			"not a valid array: "+j.Description(ctx),
			req.StateValue.String(),
		))
		return
	}

	var vals []string
	err = req.PlanValue.ElementsAs(ctx, &vals, false)
	if err != nil {
		resp.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
			req.Path,
			"not a valid array: "+j.Description(ctx),
			req.ConfigValue.String(),
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
		var oldValue map[string]interface{}
		err := json.Unmarshal([]byte(state[i]), &oldValue)
		if err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Invalid json in plan for %s", req.Path), err.Error())
			return
		}
		delete(oldValue, "id")

		var newValue map[string]interface{}
		err = json.Unmarshal([]byte(vals[i]), &newValue)
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
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Invalid value type for json string in state for %s, this is likely a bug in the provider", req.Path),
				"Expected types.String",
			)
			return
		}
		var oldValue map[string]interface{}
		err = json.Unmarshal([]byte(s.ValueString()), &oldValue)
		if err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Invalid json in state for %s", req.Path), err.Error())
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

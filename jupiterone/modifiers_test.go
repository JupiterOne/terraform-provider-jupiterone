package jupiterone

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func TestJsonIgnoreDiffModifierList(t *testing.T) {
	// This test is to isolate the jsonIgnoreDiff logic that is exercised
	// the the resource_framework_test.go.
	ctx := context.TODO()

	testCases := []struct {
		name         string
		configValue  types.List
		planValue    types.List
		stateValue   types.List
		expectedPlan types.List
	}{
		{
			name:         "all_null",
			configValue:  types.ListNull(types.StringType),
			planValue:    types.ListNull(types.StringType),
			stateValue:   types.ListNull(types.StringType),
			expectedPlan: types.ListNull(types.StringType),
		},
		{
			name:         "empty_plan_null_state",
			configValue:  types.ListValueMust(types.StringType, []attr.Value{}),
			planValue:    types.ListValueMust(types.StringType, []attr.Value{}),
			stateValue:   types.ListNull(types.StringType),
			expectedPlan: types.ListValueMust(types.StringType, []attr.Value{}),
		},
		{
			name:         "empty_state_null_plan",
			configValue:  types.ListNull(types.StringType),
			planValue:    types.ListNull(types.StringType),
			stateValue:   types.ListValueMust(types.StringType, []attr.Value{}),
			expectedPlan: types.ListValueMust(types.StringType, []attr.Value{}),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			req := planmodifier.ListRequest{
				ConfigValue: tt.configValue,
				PlanValue:   tt.planValue,
				StateValue:  tt.stateValue,
			}
			resp := &planmodifier.ListResponse{
				PlanValue: tt.planValue,
			}

			mod := &jsonIgnoreDiff{}

			mod.PlanModifyList(ctx, req, resp)

			assert.False(t, resp.Diagnostics.HasError())
			assert.Equal(t, tt.expectedPlan, resp.PlanValue)
		})
	}

}

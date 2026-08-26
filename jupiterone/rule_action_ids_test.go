package jupiterone

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// apiAction builds the `map[string]interface{}` form of an action, which is how
// actions arrive from the API inside the opaque `[JSON!]!` operations field.
func apiAction(t *testing.T, j string) interface{} {
	t.Helper()
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(j), &m))
	return m
}

// plannedOps builds the operations input as `buildOperations` produces it: one
// []string of action JSON per operation, already stripped of any config `id`.
func plannedOps(t *testing.T, ops ...[]string) []client.RuleOperationInput {
	t.Helper()
	out := make([]client.RuleOperationInput, 0, len(ops))
	for _, actions := range ops {
		op := client.RuleOperationInput{}
		for _, a := range actions {
			op.Actions = append(op.Actions, apiAction(t, a))
		}
		out = append(out, op)
	}
	return out
}

// priorOps builds the prior-state operations, where each action is the JSON
// string held in Terraform state.
func priorOps(ops ...[]string) []RuleOperation {
	out := make([]RuleOperation, 0, len(ops))
	for _, actions := range ops {
		out = append(out, RuleOperation{When: types.StringNull(), Actions: actions})
	}
	return out
}

// actionIDs returns the "id" of every action, "" where there is none.
func actionIDs(t *testing.T, ops []client.RuleOperationInput) [][]string {
	t.Helper()
	out := make([][]string, 0, len(ops))
	for _, op := range ops {
		ids := make([]string, 0, len(op.Actions))
		for _, a := range op.Actions {
			m, ok := a.(map[string]interface{})
			require.True(t, ok, "action is not a map: %#v", a)
			if id, ok := m["id"].(string); ok {
				ids = append(ids, id)
			} else {
				ids = append(ids, "")
			}
		}
		out = append(out, ids)
	}
	return out
}

func TestNewOperations_KeepsActionIds(t *testing.T) {
	ops := []client.RuleOperationOutput{
		{
			When: apiAction(t, `{"type":"FILTER","specVersion":1}`),
			Actions: []interface{}{
				apiAction(t, `{"type":"SET_PROPERTY","targetProperty":"alertLevel","id":"aaa-111"}`),
				apiAction(t, `{"type":"CREATE_ALERT","id":"bbb-222"}`),
			},
		},
	}

	got, err := newOperations(ops)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Len(t, got[0].Actions, 2)

	var first map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(got[0].Actions[0]), &first))
	assert.Equal(t, "aaa-111", first["id"], "Read must keep the server action id in state")

	var second map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(got[0].Actions[1]), &second))
	assert.Equal(t, "bbb-222", second["id"], "Read must keep the server action id in state")
}

func TestMergeActionIds(t *testing.T) {
	setProperty := `{"type":"SET_PROPERTY","targetProperty":"alertLevel","targetValue":"HIGH"}`
	setPropertyWithID := `{"id":"aaa-111","type":"SET_PROPERTY","targetProperty":"alertLevel","targetValue":"HIGH"}`
	setPropertyEdited := `{"type":"SET_PROPERTY","targetProperty":"alertLevel","targetValue":"CRITICAL"}`
	createAlert := `{"type":"CREATE_ALERT"}`
	createAlertWithID := `{"id":"bbb-222","type":"CREATE_ALERT"}`
	tagEntities := `{"type":"TAG_ENTITIES","tags":[{"name":"tf","value":"1"}]}`

	testCases := []struct {
		name     string
		planned  []client.RuleOperationInput
		prior    []RuleOperation
		expected [][]string
	}{
		{
			name:     "unchanged actions keep their ids",
			planned:  plannedOps(t, []string{setProperty, createAlert}),
			prior:    priorOps([]string{setPropertyWithID, createAlertWithID}),
			expected: [][]string{{"aaa-111", "bbb-222"}},
		},
		{
			name:     "an action edited in place keeps its id",
			planned:  plannedOps(t, []string{setPropertyEdited, createAlert}),
			prior:    priorOps([]string{setPropertyWithID, createAlertWithID}),
			expected: [][]string{{"aaa-111", "bbb-222"}},
		},
		{
			name:     "reordered actions keep their own ids",
			planned:  plannedOps(t, []string{createAlert, setProperty}),
			prior:    priorOps([]string{setPropertyWithID, createAlertWithID}),
			expected: [][]string{{"bbb-222", "aaa-111"}},
		},
		{
			name:     "an appended action gets no id",
			planned:  plannedOps(t, []string{setProperty, createAlert, tagEntities}),
			prior:    priorOps([]string{setPropertyWithID, createAlertWithID}),
			expected: [][]string{{"aaa-111", "bbb-222", ""}},
		},
		{
			name:     "a removed action does not shift ids onto its neighbour",
			planned:  plannedOps(t, []string{createAlert}),
			prior:    priorOps([]string{setPropertyWithID, createAlertWithID}),
			expected: [][]string{{"bbb-222"}},
		},
		{
			name:     "a duplicated action does not reuse the same id twice",
			planned:  plannedOps(t, []string{createAlert, createAlert}),
			prior:    priorOps([]string{createAlertWithID}),
			expected: [][]string{{"bbb-222", ""}},
		},
		{
			name:     "ids are not borrowed across operations",
			planned:  plannedOps(t, []string{createAlert}, []string{setProperty}),
			prior:    priorOps([]string{createAlertWithID}),
			expected: [][]string{{"bbb-222"}, {""}},
		},
		{
			name:     "no prior state yields no ids",
			planned:  plannedOps(t, []string{setProperty, createAlert}),
			prior:    nil,
			expected: [][]string{{"", ""}},
		},
		{
			name:     "prior state without ids yields no ids",
			planned:  plannedOps(t, []string{setProperty}),
			prior:    priorOps([]string{setProperty}),
			expected: [][]string{{""}},
		},
		{
			name:     "an id reaching the planned input from config is discarded",
			planned:  plannedOps(t, []string{`{"id":"not-from-state","type":"CREATE_ALERT"}`}),
			prior:    nil,
			expected: [][]string{{""}},
		},
		{
			name:     "state is authoritative over an id supplied in config",
			planned:  plannedOps(t, []string{`{"id":"not-from-state","type":"CREATE_ALERT"}`}),
			prior:    priorOps([]string{createAlertWithID}),
			expected: [][]string{{"bbb-222"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, mergeActionIds(tc.planned, tc.prior))
			assert.Equal(t, tc.expected, actionIDs(t, tc.planned))
		})
	}
}

func TestMergeActionIds_InvalidPriorStateJSON(t *testing.T) {
	planned := plannedOps(t, []string{`{"type":"CREATE_ALERT"}`})
	prior := priorOps([]string{"not json"})

	assert.Error(t, mergeActionIds(planned, prior))
}

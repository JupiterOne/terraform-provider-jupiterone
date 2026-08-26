package jupiterone

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// actionIdTestRuleName is the rule managed by TestInlineRuleInstance_ActionIdsAreStable.
const actionIdTestRuleName = "tf-provider-test-rule-action-ids"

// ruleActionIds reads the "id" of every action of the rule's first operation
// out of the resource's state.
func ruleActionIds(s *terraform.State, resourceName string) ([]string, error) {
	rs, ok := s.RootModule().Resources[resourceName]
	if !ok {
		return nil, fmt.Errorf("%s not found in state", resourceName)
	}

	const countKey = "operations.0.actions.#"
	count, err := strconv.Atoi(rs.Primary.Attributes[countKey])
	if err != nil {
		return nil, fmt.Errorf("no actions found at %s: %w", countKey, err)
	}

	ids := make([]string, 0, count)
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("operations.0.actions.%d", i)
		var action map[string]interface{}
		if err := json.Unmarshal([]byte(rs.Primary.Attributes[key]), &action); err != nil {
			return nil, fmt.Errorf("invalid action json at %s: %w", key, err)
		}
		id, _ := action["id"].(string)
		ids = append(ids, id)
	}
	return ids, nil
}

// recordRuleActionIds captures the action ids currently in state so that a
// later step can assert they did not change.
func recordRuleActionIds(resourceName string, into *[]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ids, err := ruleActionIds(s, resourceName)
		if err != nil {
			return err
		}
		for i, id := range ids {
			if id == "" {
				return fmt.Errorf("action %d has no id in state, so id stability cannot be asserted", i)
			}
		}
		*into = ids
		return nil
	}
}

// checkRuleActionIdsUnchanged asserts every action still has the id it was
// created with. The ids were captured from the state of an earlier step, so a
// mismatch means the API replaced the actions rather than updating them.
func checkRuleActionIdsUnchanged(resourceName string, expected *[]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ids, err := ruleActionIds(s, resourceName)
		if err != nil {
			return err
		}
		if len(ids) != len(*expected) {
			return fmt.Errorf("expected %d actions, got %d", len(*expected), len(ids))
		}
		for i, id := range ids {
			if id != (*expected)[i] {
				return fmt.Errorf("action %d id changed across the update: was %q, now %q", i, (*expected)[i], id)
			}
		}
		return nil
	}
}

// checkRuleActionContent compares an action of the rule's first operation in
// state to the expected JSON, ignoring the server assigned id.
func checkRuleActionContent(resourceName string, action int, expected string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("%s not found in state", resourceName)
		}

		key := fmt.Sprintf("operations.0.actions.%d", action)
		var got map[string]interface{}
		if err := json.Unmarshal([]byte(rs.Primary.Attributes[key]), &got); err != nil {
			return fmt.Errorf("invalid action json at %s: %w", key, err)
		}
		delete(got, "id")

		var want map[string]interface{}
		if err := json.Unmarshal([]byte(expected), &want); err != nil {
			return fmt.Errorf("invalid expected action json: %w", err)
		}

		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		if string(gotJSON) != string(wantJSON) {
			return fmt.Errorf("action %s: expected %s, got %s", key, wantJSON, gotJSON)
		}
		return nil
	}
}

// TestInlineRuleInstance_ActionIdsAreStable asserts that updating a rule
// updates its existing actions in place. The API mints a fresh id for every
// action sent without one, so an id that survives an update is the only proof
// that the action itself survived it. Action ids are what evaluation history
// and external outputs are correlated by, so churning them on every apply
// severs those links.
func TestInlineRuleInstance_ActionIdsAreStable(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClientsWithReplaySupport(ctx, t)
	defer cleanup(t)

	var actionIds []string

	setProperty := `{"targetValue":"HIGH","type":"SET_PROPERTY","targetProperty":"alertLevel"}`
	setPropertyEdited := `{"targetValue":"CRITICAL","type":"SET_PROPERTY","targetProperty":"alertLevel"}`

	operations := func(setPropertyAction string) string {
		return fmt.Sprintf(`[
				{
					actions = [
						%q,
						%q,
					]
				}
			]`, setPropertyAction, createAlertActionJSON)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckRuleInstanceDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			// Create. The ids are assigned by the API and only reach state on
			// the refresh that precedes the next step.
			{
				Config: testActionIdRuleConfig("Test", operations(setProperty)),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRuleExists(ctx, testRuleResourceName, directClient),
					resource.TestCheckResourceAttr(testRuleResourceName, "operations.0.actions.#", "2"),
				),
			},
			// An update that leaves the actions alone. State has been
			// refreshed, so the ids assigned at create are now in it.
			{
				Config: testActionIdRuleConfig("Updated description", operations(setProperty)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(testRuleResourceName, "description", "Updated description"),
					recordRuleActionIds(testRuleResourceName, &actionIds),
				),
			},
			// A further update to an unrelated field. The refresh at the start
			// of this step reads back what the API actually stored during the
			// previous update, so unchanged ids here prove the API honoured
			// the ids the provider sent rather than replacing the actions.
			{
				Config: testActionIdRuleConfig("Updated again", operations(setProperty)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(testRuleResourceName, "description", "Updated again"),
					checkRuleActionIdsUnchanged(testRuleResourceName, &actionIds),
				),
			},
			// Edit one action in place. jsonIgnoreDiff cannot collapse a list
			// whose content genuinely changed, so the plan for this operation
			// carries no ids and neither does the state written from it. The
			// ids are still sent to the API, taken from prior state, which the
			// next step reads back.
			{
				Config: testActionIdRuleConfig("Updated again", operations(setPropertyEdited)),
				Check: resource.ComposeTestCheckFunc(
					checkRuleActionContent(testRuleResourceName, 0, setPropertyEdited),
					checkRuleActionContent(testRuleResourceName, 1, createAlertActionJSON),
				),
			},
			// No configuration change, so this step only refreshes. The ids it
			// reads back are what the API actually stored while the action was
			// edited, so them being unchanged proves the edit updated the
			// existing actions instead of replacing them.
			{
				Config: testActionIdRuleConfig("Updated again", operations(setPropertyEdited)),
				Check: resource.ComposeTestCheckFunc(
					checkRuleActionContent(testRuleResourceName, 0, setPropertyEdited),
					checkRuleActionIdsUnchanged(testRuleResourceName, &actionIds),
				),
			},
		},
	})
}

func testActionIdRuleConfig(description, operations string) string {
	return fmt.Sprintf(`
		resource "jupiterone_rule" "test" {
			name = %q
			description = %q
			spec_version = 1
			polling_interval = "ONE_WEEK"
			tags = ["tf_acc:1","tf_acc:2"]

			question {
				queries {
					name = "query0"
					query = "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"
					version = "v1"
				}
			}

			outputs = [
				"queries.query0.total",
				"alertLevel"
			]

			operations = %s
		}
	`, actionIdTestRuleName, description, operations)
}

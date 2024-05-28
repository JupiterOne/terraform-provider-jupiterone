package jupiterone

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

var createAlertActionJSON = `{"type":"CREATE_ALERT"}`
var testRuleResourceName = "jupiterone_rule.test"

func TestInlineRuleInstance_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	ruleName := "tf-provider-test-rule"
	operations := getValidOperations()
	operationsUpdate := getValidOperationsWithoutFilter()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckRuleInstanceDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testInlineRuleInstanceBasicConfigWithOperations(ruleName, operations),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRuleExists(ctx, testRuleResourceName, directClient),
					resource.TestCheckResourceAttrSet(testRuleResourceName, "id"),
					resource.TestCheckResourceAttr(testRuleResourceName, "version", "1"),
					resource.TestCheckResourceAttr(testRuleResourceName, "name", ruleName),
					resource.TestCheckResourceAttr(testRuleResourceName, "description", "Test"),
					resource.TestCheckResourceAttr(testRuleResourceName, "spec_version", "1"),
					resource.TestCheckResourceAttr(testRuleResourceName, "polling_interval", "ONE_WEEK"),
					resource.TestCheckResourceAttr(testRuleResourceName, "notify_on_failure", "false"),
					resource.TestCheckResourceAttr(testRuleResourceName, "trigger_on_new_only", "true"),
					resource.TestCheckResourceAttr(testRuleResourceName, "tags.#", "2"),
					resource.TestCheckResourceAttr(testRuleResourceName, "tags.0", "tf_acc:1"),
					resource.TestCheckResourceAttr(testRuleResourceName, "tags.1", "tf_acc:2"),
					resource.TestCheckResourceAttr(testRuleResourceName, "operations.#", "1"),
					resource.TestCheckResourceAttr(testRuleResourceName, "operations.0.actions.#", "2"),
					resource.TestCheckResourceAttr(testRuleResourceName, "operations.0.actions.1", createAlertActionJSON),
					resource.TestCheckResourceAttr(testRuleResourceName, "outputs.#", "2"),
					resource.TestCheckResourceAttr(testRuleResourceName, "outputs.0", "queries.query0.total"),
					resource.TestCheckResourceAttr(testRuleResourceName, "outputs.1", "alertLevel"),
					resource.TestCheckResourceAttr(testRuleResourceName, "question.#", "1"),
					resource.TestCheckResourceAttr(testRuleResourceName, "question.0.queries.#", "1"),
					resource.TestCheckResourceAttr(testRuleResourceName, "question.0.queries.0.name", "query0"),
					resource.TestCheckResourceAttr(testRuleResourceName, "question.0.queries.0.version", "v1"),
					resource.TestCheckResourceAttr(testRuleResourceName, "question.0.queries.0.query", "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"),
				),
			},
			{
				Config: testInlineRuleInstanceBasicConfigWithOperations(ruleName, operationsUpdate),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRuleExists(ctx, testRuleResourceName, directClient),
					resource.TestCheckResourceAttrSet(testRuleResourceName, "id"),
					resource.TestCheckResourceAttr(testRuleResourceName, "version", "2"),
					resource.TestCheckResourceAttr(testRuleResourceName, "name", ruleName),
					resource.TestCheckResourceAttr(testRuleResourceName, "description", "Test"),
					resource.TestCheckResourceAttr(testRuleResourceName, "spec_version", "1"),
					resource.TestCheckResourceAttr(testRuleResourceName, "polling_interval", "ONE_WEEK"),
					resource.TestCheckResourceAttr(testRuleResourceName, "notify_on_failure", "false"),
					resource.TestCheckResourceAttr(testRuleResourceName, "trigger_on_new_only", "true"),
					resource.TestCheckResourceAttr(testRuleResourceName, "tags.#", "2"),
					resource.TestCheckResourceAttr(testRuleResourceName, "tags.0", "tf_acc:1"),
					resource.TestCheckResourceAttr(testRuleResourceName, "tags.1", "tf_acc:2"),
					resource.TestCheckResourceAttr(testRuleResourceName, "operations.0.actions.1", createAlertActionJSON),
					resource.TestCheckResourceAttr(testRuleResourceName, "operations.#", "1"),
					resource.TestCheckResourceAttr(testRuleResourceName, "operations.0.actions.#", "2"),
					resource.TestCheckResourceAttr(testRuleResourceName, "outputs.#", "2"),
					resource.TestCheckResourceAttr(testRuleResourceName, "outputs.0", "queries.query0.total"),
					resource.TestCheckResourceAttr(testRuleResourceName, "outputs.1", "alertLevel"),
					resource.TestCheckResourceAttr(testRuleResourceName, "question.#", "1"),
					resource.TestCheckResourceAttr(testRuleResourceName, "question.0.queries.#", "1"),
					resource.TestCheckResourceAttr(testRuleResourceName, "question.0.queries.0.name", "query0"),
					resource.TestCheckResourceAttr(testRuleResourceName, "question.0.queries.0.version", "v1"),
					resource.TestCheckResourceAttr(testRuleResourceName, "question.0.queries.0.query", "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"),
				),
			},
		},
	})
}

func TestInlineRuleInstance_BasicImport(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	ruleName := "tf-provider-test-rule"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckRuleInstanceDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				ImportState:        true,
				ImportStatePersist: true, // set to true to do destroy the created rule
				ResourceName:       testRuleResourceName,
				// must use recordingClient for createTestRule to return the uuid
				ImportStateId: createTestRule(ctx, t, recordingClient, ruleName),
				Config:        testInlineRuleInstanceBasicConfigWithOperations(ruleName, getValidOperationsWithoutFilter()),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRuleExists(ctx, testRuleResourceName, directClient),
					resource.TestCheckResourceAttrSet(testRuleResourceName, "id"),
					resource.TestCheckResourceAttr(testRuleResourceName, "version", "1"),
					resource.TestCheckResourceAttr(testRuleResourceName, "name", ruleName),
					resource.TestCheckResourceAttr(testRuleResourceName, "description", "Test"),
					resource.TestCheckResourceAttr(testRuleResourceName, "spec_version", "1"),
					resource.TestCheckResourceAttr(testRuleResourceName, "polling_interval", "ONE_WEEK"),
					resource.TestCheckResourceAttr(testRuleResourceName, "trigger_on_new_only", "false"),
					resource.TestCheckResourceAttr(testRuleResourceName, "tags.#", "2"),
					resource.TestCheckResourceAttr(testRuleResourceName, "tags.0", "tf_acc:1"),
					resource.TestCheckResourceAttr(testRuleResourceName, "tags.1", "tf_acc:2"),
					resource.TestCheckResourceAttr(testRuleResourceName, "outputs.#", "2"),
					resource.TestCheckResourceAttr(testRuleResourceName, "outputs.0", "queries.query0.total"),
					resource.TestCheckResourceAttr(testRuleResourceName, "outputs.1", "alertLevel"),
					resource.TestCheckResourceAttr(testRuleResourceName, "question.#", "1"),
					resource.TestCheckResourceAttr(testRuleResourceName, "question.0.queries.#", "1"),
					resource.TestCheckResourceAttr(testRuleResourceName, "question.0.queries.0.name", "query0"),
					resource.TestCheckResourceAttr(testRuleResourceName, "question.0.queries.0.version", "v1"),
					resource.TestCheckResourceAttr(testRuleResourceName, "question.0.queries.0.query", "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"),
				),
			},
		},
	})
}

func TestReferencedQuestionRule_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	ruleName := "tf-provider-test-rule"
	operations := getValidOperations()
	operationsUpdate := getValidOperationsWithoutFilter()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckRuleInstanceDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testReferencedRuleInstanceBasicConfigWithOperations(ruleName, operations),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRuleExists(ctx, testRuleResourceName, directClient),
					resource.TestCheckResourceAttrSet(testRuleResourceName, "id"),
					resource.TestCheckResourceAttr(testRuleResourceName, "version", "1"),
					resource.TestCheckResourceAttr(testRuleResourceName, "question.#", "0"),
					resource.TestCheckResourceAttrPair("jupiterone_question.test", "id", testRuleResourceName, "question_id"),
				),
			},
			{
				Config: testReferencedRuleInstanceBasicConfigWithOperations(ruleName, operationsUpdate),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRuleExists(ctx, testRuleResourceName, directClient),
					resource.TestCheckResourceAttr(testRuleResourceName, "question.#", "0"),
					resource.TestCheckResourceAttrPair("jupiterone_question.test", "id", testRuleResourceName, "question_id"),
				),
			},
		},
	})
}

func TestRuleInstance_Config_Errors(t *testing.T) {
	ctx := context.TODO()

	recordingClient, _, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	rName := acctest.RandomWithPrefix("tf-acc-test")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		Steps: []resource.TestStep{
			{
				Config:      testInlineRuleInstanceBasicConfigWithOperations(rName, "\"not json\""),
				ExpectError: regexp.MustCompile(`list of object required`),
			},
			{
				Config:      testInlineRuleInstanceBasicConfigWithOperations(rName, getInvalidOperations()),
				ExpectError: regexp.MustCompile(`invalid character`),
			},
			{
				Config:      testInlineRuleInstanceBasicConfigWithOperations("", getValidOperations()),
				ExpectError: regexp.MustCompile(`Attribute name string length must be between 1 and 255, got: 0`),
			},
			{
				Config:      testRuleInstanceBasicConfigWithPollingInterval(rName, "INVALID_POLLING_INTERVAL"),
				ExpectError: regexp.MustCompile(`Attribute polling_interval value must be one of:`),
			},
		},
	})
}

// createTestRule directly creates a rule for testing. Because the id must be
// return for the import and other tests, this must be called with a recorder
// client
func createTestRule(ctx context.Context, t *testing.T, qlient graphql.Client, name string) string {
	r, err := client.CreateInlineQuestionRuleInstance(ctx, qlient, client.CreateInlineQuestionRuleInstanceInput{
		Name:            name,
		Description:     "test",
		Tags:            []string{"tf_acc:1", "tf_acc:2"},
		SpecVersion:     1,
		Outputs:         []string{"queries.query0.total", "alertLevel"},
		PollingInterval: client.SchedulerPollingIntervalOneDay,
		NotifyOnFailure: false,
		Question: client.RuleQuestionDetailsInput{
			Queries: []client.J1QueryInput{
				{
					Name:    "query0",
					Query:   "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true",
					Version: "v1",
				},
			},
		},
		Operations: []client.RuleOperationInput{},
	})
	if err != nil {
		t.Log("error creating rule for import test", err)
		t.FailNow()
	}

	return r.CreateQuestionRuleInstance.Id
}

func testAccCheckRuleExists(ctx context.Context, ruleName string, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		resource := s.RootModule().Resources[ruleName]

		return ruleExistsHelper(ctx, resource.Primary.ID, qlient)
	}
}

func ruleExistsHelper(ctx context.Context, id string, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	duration := 10 * time.Second
	err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
		_, err := client.GetQuestionRuleInstance(ctx, qlient, id)

		if err == nil {
			return nil
		}

		if err != nil && strings.Contains(err.Error(), "Rule instance does not exist.") {
			return retry.RetryableError(fmt.Errorf("Rule instance does not exist (id=%q)", id))
		}

		return retry.NonRetryableError(err)
	})

	if err != nil {
		return err
	}

	return nil
}

func testAccCheckRuleInstanceDestroy(ctx context.Context, qlient graphql.Client) func(*terraform.State) error {
	return func(s *terraform.State) error {
		resource := s.RootModule().Resources[testRuleResourceName]
		if resource == nil {
			hclog.Default().Debug("No resource found for rule name", "resource_name", testRuleResourceName)
			return nil
		}
		hclog.Default().Debug("Attempting to delete resource for rule name", "resource_name", testRuleResourceName, "resource_id", resource.Primary.ID)
		return ruleInstanceDestroyHelper(ctx, resource.Primary.ID, qlient)
	}
}

func ruleInstanceDestroyHelper(ctx context.Context, id string, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	duration := 10 * time.Second
	err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
		_, err := client.GetQuestionRuleInstance(ctx, qlient, id)

		if err == nil {
			return retry.RetryableError(fmt.Errorf("Rule instance still exists (id=%q)", id))
		}

		if err != nil && strings.Contains(err.Error(), "Rule instance does not exist.") {
			return nil
		}

		return retry.NonRetryableError(err)
	})

	if err != nil {
		return err
	}

	return nil
}

func getInvalidOperations() string {
	return `[
		{
			when = "not json"
			actions = [
					"still not json",
					"also not json",
				]
			}
	]`
}

func getValidOperations() string {
	return fmt.Sprintf(`[
			{
				when = %q
				actions = [
					%q,
					%q,
				]
			}
		]`,
		`{"type":"FILTER","specVersion":1,"condition":"{{queries.query0.total != 0}}"}`,
		`{"targetValue":"HIGH","type":"SET_PROPERTY","targetProperty":"alertLevel"}`,
		createAlertActionJSON)
}

func getValidOperationsWithoutFilter() string {
	return fmt.Sprintf(`[
			{
				actions = [
					%q,
					%q,
				]
			}
		]`,
		`{"targetValue":"HIGH","type":"SET_PROPERTY","targetProperty":"alertLevel"}`,
		createAlertActionJSON)
}

func testInlineRuleInstanceBasicConfigWithOperations(rName string, operations string) string {
	return fmt.Sprintf(`
		resource "jupiterone_rule" "test" {
			name = %q
			description = "Test"
			spec_version = 1
			polling_interval = "ONE_WEEK"
			notify_on_failure = false
			trigger_on_new_only = true
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
	`, rName, operations)
}

func testReferencedRuleInstanceBasicConfigWithOperations(rName string, operations string) string {
	return fmt.Sprintf(`
		resource "jupiterone_question" "test" {
			title = %q
			description = "Test"
			tags = ["tf_acc:1","tf_acc:2"]

			query {
				name = "query0"
				query = "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"
				version = "v1"
			}
		}

		resource "jupiterone_rule" "test" {
			name = %q
			description = "Test"
			spec_version = 1
			polling_interval = "ONE_WEEK"
			tags = ["tf_acc:1","tf_acc:2"]
			notify_on_failure = false

			question_id = jupiterone_question.test.id

			outputs = [
				"queries.query0.total",
				"alertLevel"
			]

			operations = %s
		}
	`, rName, rName, operations)
}

func testRuleInstanceBasicConfigWithPollingInterval(rName string, pollingInterval string) string {
	return fmt.Sprintf(`
		provider "jupiterone" {}

		resource "jupiterone_rule" "test" {
			name = %q
			description = "Test"
			spec_version = 1
			polling_interval = %q
			notify_on_failure = false

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
	`, rName, pollingInterval, getValidOperations())
}

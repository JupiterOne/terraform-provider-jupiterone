package jupiterone

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/terraform-plugin-sdk/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

func TestRuleInstance_Basic(t *testing.T) {
	ctx := context.TODO()

	recorder, cleanup := setupCassettes(t.Name())
	defer cleanup(t)
	testHttpClient := cleanhttp.DefaultClient()
	testHttpClient.Transport = recorder
	qlient := client.NewQlientFromEnv(ctx, testHttpClient)

	ruleName := "tf-provider-test-rule"
	resourceName := "jupiterone_rule.test"
	operations := getValidOperations()
	operationsUpdate := getValidOperationsWithoutConditions()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(qlient),
		CheckDestroy:             testAccCheckRuleInstanceDestroy(ctx, resourceName, qlient),
		Steps: []resource.TestStep{
			{
				Config: testRuleInstanceBasicConfigWithOperations(ruleName, operations),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRuleExists(ctx, resourceName, qlient),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "version", "1"),
					resource.TestCheckResourceAttr(resourceName, "name", ruleName),
					resource.TestCheckResourceAttr(resourceName, "description", "Test"),
					resource.TestCheckResourceAttr(resourceName, "spec_version", "1"),
					resource.TestCheckResourceAttr(resourceName, "polling_interval", "ONE_WEEK"),
					resource.TestCheckResourceAttr(resourceName, "tags.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "tags.0", "tf_acc:1"),
					resource.TestCheckResourceAttr(resourceName, "tags.1", "tf_acc:2"),
					resource.TestCheckResourceAttr(resourceName, "operations.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "operations.0.actions.#", "2"),
					// FIXME: check operatons resource.TestCheckResourceAttr(resourceName, "operations", operations),
					resource.TestCheckResourceAttr(resourceName, "outputs.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "outputs.0", "queries.query0.total"),
					resource.TestCheckResourceAttr(resourceName, "outputs.1", "alertLevel"),
					resource.TestCheckResourceAttr(resourceName, "question.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "question.0.queries.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "question.0.queries.0.name", "query0"),
					resource.TestCheckResourceAttr(resourceName, "question.0.queries.0.version", "v1"),
					resource.TestCheckResourceAttr(resourceName, "question.0.queries.0.query", "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"),
				),
			},
			{
				Config: testRuleInstanceBasicConfigWithOperations(ruleName, operationsUpdate),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRuleExists(ctx, resourceName, qlient),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "version", "2"),
					resource.TestCheckResourceAttr(resourceName, "name", ruleName),
					resource.TestCheckResourceAttr(resourceName, "description", "Test"),
					resource.TestCheckResourceAttr(resourceName, "spec_version", "1"),
					resource.TestCheckResourceAttr(resourceName, "polling_interval", "ONE_WEEK"),
					resource.TestCheckResourceAttr(resourceName, "tags.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "tags.0", "tf_acc:1"),
					resource.TestCheckResourceAttr(resourceName, "tags.1", "tf_acc:2"),
					// FIXME: resource.TestCheckResourceAttr(resourceName, "operations", operationsUpdate),
					resource.TestCheckResourceAttr(resourceName, "operations.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "operations.0.actions.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "outputs.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "outputs.0", "queries.query0.total"),
					resource.TestCheckResourceAttr(resourceName, "outputs.1", "alertLevel"),
					resource.TestCheckResourceAttr(resourceName, "question.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "question.0.queries.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "question.0.queries.0.name", "query0"),
					resource.TestCheckResourceAttr(resourceName, "question.0.queries.0.version", "v1"),
					resource.TestCheckResourceAttr(resourceName, "question.0.queries.0.query", "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"),
				),
			},
		},
	})
}

func TestRuleInstance_Config_Errors(t *testing.T) {
	ctx := context.TODO()

	recorder, cleanup := setupCassettes(t.Name())
	defer cleanup(t)
	testHttpClient := cleanhttp.DefaultClient()
	testHttpClient.Transport = recorder
	qlient := client.NewQlientFromEnv(ctx, testHttpClient)

	rName := acctest.RandomWithPrefix("tf-acc-test")
	resourceName := "jupiterone_rule.test"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(qlient),
		CheckDestroy:             testAccCheckRuleInstanceDestroy(ctx, resourceName, qlient),
		Steps: []resource.TestStep{
			{
				Config:      testRuleInstanceBasicConfigWithOperations(rName, "\"not json\""),
				ExpectError: regexp.MustCompile(`list of object required`),
			},
			{
				Config:      testRuleInstanceBasicConfigWithOperations(rName, getInvalidOperations()),
				ExpectError: regexp.MustCompile(`string value must be valid JSON`),
			},
			{
				Config:      testRuleInstanceBasicConfigWithOperations("", getValidOperations()),
				ExpectError: regexp.MustCompile(`Attribute name string length must be between 1 and 255, got: 0`),
			},
			{
				Config:      testRuleInstanceBasicConfigWithPollingInterval(rName, "INVALID_POLLING_INTERVAL"),
				ExpectError: regexp.MustCompile(`Attribute polling_interval value must be one of:`),
			},
		},
	})
}

func testAccCheckRuleExists(ctx context.Context, ruleName string, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		resource := s.RootModule().Resources[ruleName]

		err := ruleExistsHelper(ctx, resource.Primary.ID, qlient)
		if err != nil {
			return err
		}
		return nil
	}
}

func ruleExistsHelper(ctx context.Context, id string, qlient graphql.Client) error {
	duration := 10 * time.Second
	if isReplaying() {
		// no reason to wait as long on replays, but the retries would be recorded and
		// have to be exercised and this can't be set to 0.
		duration = time.Second
	}
	err := resource.RetryContext(ctx, duration, func() *resource.RetryError {
		_, err := client.GetQuestionRuleInstance(ctx, qlient, id)

		if err == nil {
			return nil
		}

		if err != nil && strings.Contains(err.Error(), "Rule instance does not exist.") {
			return resource.RetryableError(fmt.Errorf("Rule instance does not exist (id=%q)", id))
		}

		return resource.NonRetryableError(err)
	})

	if err != nil {
		return err
	}

	return nil
}

func testAccCheckRuleInstanceDestroy(ctx context.Context, resourceName string, qlient graphql.Client) func(*terraform.State) error {
	return func(s *terraform.State) error {
		resource := s.RootModule().Resources[resourceName]

		if err := ruleInstanceDestroyHelper(ctx, resource.Primary.ID, qlient); err != nil {
			return err
		}
		return nil
	}
}

func ruleInstanceDestroyHelper(ctx context.Context, id string, qlient graphql.Client) error {
	duration := 10 * time.Second
	if isReplaying() {
		// no reason to wait as long on replays, but the retries would be recorded and
		// have to be exercised and this can't be set to 0.
		duration = time.Second
	}
	err := resource.RetryContext(ctx, duration, func() *resource.RetryError {
		_, err := client.GetQuestionRuleInstance(ctx, qlient, id)

		if err == nil {
			return resource.RetryableError(fmt.Errorf("Rule instance still exists (id=%q)", id))
		}

		if err != nil && strings.Contains(err.Error(), "Rule instance does not exist.") {
			return nil
		}

		return resource.NonRetryableError(err)
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
		`{"type":"CREATE_ALERT"}`)
}

func getValidOperationsWithoutConditions() string {
	return fmt.Sprintf(`[
			{
				actions = [
					%q,
					%q,
				]
			}
		]`,
		`{"targetValue":"HIGH","type":"SET_PROPERTY","targetProperty":"alertLevel"}`,
		`{"type":"CREATE_ALERT"}`)
}

func testRuleInstanceBasicConfigWithOperations(rName string, operations string) string {
	return fmt.Sprintf(`
		resource "jupiterone_rule" "test" {
			name = %q
			description = "Test"
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
	`, rName, operations)
}

func testRuleInstanceBasicConfigWithPollingInterval(rName string, pollingInterval string) string {
	return fmt.Sprintf(`
		resource "jupiterone_rule" "test" {
			name = %q
			description = "Test"
			spec_version = 1
			polling_interval = %q

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

package jupiterone

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/terraform-plugin-sdk/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/logging"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

func TestRuleInstance_Basic(t *testing.T) {
	ctx := context.TODO()

	recorder, cleanup := setupCassettes(t.Name())
	defer cleanup(t)
	testHttpClient := cleanhttp.DefaultClient()
	testHttpClient.Transport = logging.NewTransport("JupiterOne", recorder)
	// testJ1Client is used for direct calls for CheckDestroy/etc.
	testJ1Client, err := client.NewClientFromEnv(ctx, testHttpClient)
	if err != nil {
		t.Fatal("error configuring check client", err)
	}

	ruleName := "tf-provider-rule"
	resourceName := "jupiterone_rule.test"
	operations := getValidOperations()
	operationsUpdate := getValidOperationsWithoutConditions()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(testJ1Client),
		CheckDestroy:             testAccCheckRuleInstanceDestroy(ctx, resourceName, testJ1Client),
		Steps: []resource.TestStep{
			{
				Config: testRuleInstanceBasicConfigWithOperations(ruleName, operations),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRuleExists(ctx, resourceName, testJ1Client),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "version", "1"),
					resource.TestCheckResourceAttr(resourceName, "name", ruleName),
					resource.TestCheckResourceAttr(resourceName, "description", "Test"),
					resource.TestCheckResourceAttr(resourceName, "spec_version", "1"),
					resource.TestCheckResourceAttr(resourceName, "polling_interval", "ONE_WEEK"),
					resource.TestCheckResourceAttr(resourceName, "tags.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "tags.0", "tag1"),
					resource.TestCheckResourceAttr(resourceName, "tags.1", "tag2"),
					resource.TestCheckResourceAttr(resourceName, "operations", operations),
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
					testAccCheckRuleExists(ctx, resourceName, testJ1Client),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "version", "2"),
					resource.TestCheckResourceAttr(resourceName, "name", ruleName),
					resource.TestCheckResourceAttr(resourceName, "description", "Test"),
					resource.TestCheckResourceAttr(resourceName, "spec_version", "1"),
					resource.TestCheckResourceAttr(resourceName, "polling_interval", "ONE_WEEK"),
					resource.TestCheckResourceAttr(resourceName, "tags.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "tags.0", "tag1"),
					resource.TestCheckResourceAttr(resourceName, "tags.1", "tag2"),
					resource.TestCheckResourceAttr(resourceName, "operations", operationsUpdate),
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
	testHttpClient.Transport = logging.NewTransport("JupiterOne", recorder)
	// testJ1Client is used for direct calls for CheckDestroy/etc.
	testJ1Client, err := client.NewClientFromEnv(ctx, testHttpClient)
	if err != nil {
		t.Fatal("error configuring check client", err)
	}

	rName := acctest.RandomWithPrefix("tf-acc-test")
	resourceName := "jupiterone_rule.test"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(testJ1Client),
		CheckDestroy:             testAccCheckRuleInstanceDestroy(ctx, resourceName, testJ1Client),
		Steps: []resource.TestStep{
			{
				Config:      testRuleInstanceBasicConfigWithOperations(rName, "not json"),
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

func getValidOperations() string {
	return `[{"when":{"type":"FILTER","specVersion":1,"condition":"{{queries.query0.total != 0}}"},"actions":[{"targetValue":"HIGH","type":"SET_PROPERTY","targetProperty":"alertLevel"},{"type":"CREATE_ALERT"}]}]`
}

func getValidOperationsWithoutConditions() string {
	return `[{"actions":[{"targetValue":"HIGH","type":"SET_PROPERTY","targetProperty":"alertLevel"},{"type":"CREATE_ALERT"}]}]`
}

func testAccCheckRuleExists(ctx context.Context, ruleName string, client *client.JupiterOneClient) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		resource := s.RootModule().Resources[ruleName]

		err := ruleExistsHelper(ctx, resource.Primary.ID, client)
		if err != nil {
			return err
		}
		return nil
	}
}

func ruleExistsHelper(ctx context.Context, id string, client *client.JupiterOneClient) error {
	err := resource.RetryContext(ctx, 10*time.Second, func() *resource.RetryError {
		ruleInstance, err := client.GetQuestionRuleInstanceByID(id)

		if ruleInstance != nil {
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

func testAccCheckRuleInstanceDestroy(ctx context.Context, resourceName string, client *client.JupiterOneClient) func(*terraform.State) error {
	return func(s *terraform.State) error {
		resource := s.RootModule().Resources[resourceName]

		if err := ruleInstanceDestroyHelper(ctx, resource.Primary.ID, client); err != nil {
			return err
		}
		return nil
	}
}

func ruleInstanceDestroyHelper(ctx context.Context, id string, client *client.JupiterOneClient) error {
	err := resource.RetryContext(ctx, 30*time.Second, func() *resource.RetryError {
		ruleInstance, err := client.GetQuestionRuleInstanceByID(id)

		if ruleInstance != nil {
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

func testRuleInstanceBasicConfigWithOperations(rName string, operations string) string {
	return fmt.Sprintf(`
		resource "jupiterone_rule" "test" {
			name = %q
			description = "Test"
			spec_version = 1
			polling_interval = "ONE_WEEK"
			tags = ["tag1","tag2"]

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

			operations = %q
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

			tags = ["tag1","tag2"]
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

			operations = %q
		}
	`, rName, pollingInterval, getValidOperations())
}

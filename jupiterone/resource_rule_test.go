package jupiterone

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

func TestRuleInstance_Basic(t *testing.T) {
	accProviders, cleanup := testAccProviders(t)
	defer cleanup(t)
	accProvider := testAccProvider(t, accProviders)
	rName := acctest.RandomWithPrefix("tf-acc-test")
	resourceName := "jupiterone_rule.test"

	operations := getValidOperations()
	operationsUpdate := getValidOperationsWithoutConditions()

	ctx := context.Background()

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    accProviders,
		CheckDestroy: testAccCheckRuleInstanceDestroy(ctx, accProvider),
		Steps: []resource.TestStep{
			{
				Config: testRuleInstanceBasicConfigWithOperations(rName, operations),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRuleExists(ctx, accProvider),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "version", "1"),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					resource.TestCheckResourceAttr(resourceName, "description", "Test"),
					resource.TestCheckResourceAttr(resourceName, "spec_version", "1"),
					resource.TestCheckResourceAttr(resourceName, "polling_interval", "ONE_DAY"),
					resource.TestCheckResourceAttr(resourceName, "operations", operations),
					resource.TestCheckResourceAttr(resourceName, "outputs.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "outputs.0", "queries.query0.total"),
					resource.TestCheckResourceAttr(resourceName, "outputs.1", "alertLevel"),
					resource.TestCheckResourceAttr(resourceName, "question.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "question.0.queries.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "question.0.queries.0.name", "query0"),
					resource.TestCheckResourceAttr(resourceName, "question.0.queries.0.version", "v1"),
					resource.TestCheckResourceAttr(resourceName, "question.0.queries.0.query", "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"),
					resource.TestCheckResourceAttr(resourceName, "question.0.queries.0.results_are", "UNKNOWN"),
				),
			},
			{
				Config: testRuleInstanceBasicConfigWithOperations(rName, operationsUpdate),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRuleExists(ctx, accProvider),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "version", "2"),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					resource.TestCheckResourceAttr(resourceName, "description", "Test"),
					resource.TestCheckResourceAttr(resourceName, "spec_version", "1"),
					resource.TestCheckResourceAttr(resourceName, "polling_interval", "ONE_DAY"),
					resource.TestCheckResourceAttr(resourceName, "operations", operationsUpdate),
					resource.TestCheckResourceAttr(resourceName, "outputs.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "outputs.0", "queries.query0.total"),
					resource.TestCheckResourceAttr(resourceName, "outputs.1", "alertLevel"),
					resource.TestCheckResourceAttr(resourceName, "question.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "question.0.queries.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "question.0.queries.0.name", "query0"),
					resource.TestCheckResourceAttr(resourceName, "question.0.queries.0.version", "v1"),
					resource.TestCheckResourceAttr(resourceName, "question.0.queries.0.query", "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"),
					resource.TestCheckResourceAttr(resourceName, "question.0.queries.0.results_are", "UNKNOWN"),
				),
			},
		},
	})
}

func TestRuleInstance_Config_Errors(t *testing.T) {
	accProviders, cleanup := testAccProviders(t)
	defer cleanup(t)
	accProvider := testAccProvider(t, accProviders)
	rName := acctest.RandomWithPrefix("tf-acc-test")
	ctx := context.Background()

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    accProviders,
		CheckDestroy: testAccCheckRuleInstanceDestroy(ctx, accProvider),
		Steps: []resource.TestStep{
			{
				Config:      testRuleInstanceBasicConfigWithOperations(rName, "not json"),
				ExpectError: regexp.MustCompile(`"operations" contains an invalid JSON`),
			},
			{
				Config:      testRuleInstanceBasicConfigWithOperations("", getValidOperations()),
				ExpectError: regexp.MustCompile(`expected length of name to be in the range \(1 - 255\)`),
			},
			{
				Config:      testRuleInstanceBasicConfigWithPollingInterval(rName, "INVALID_POLLING_INTERVAL"),
				ExpectError: regexp.MustCompile(`expected polling_interval to be one of \[DISABLED THIRTY_MINUTES ONE_HOUR ONE_DAY\], got INVALID_POLLING_INTERVAL`),
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

func testAccCheckRuleExists(ctx context.Context, accProvider *schema.Provider) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		providerConf := accProvider.Meta().(*ProviderConfiguration)
		client := providerConf.Client

		if err := ruleExistsHelper(ctx, s, client); err != nil {
			return err
		}
		return nil
	}
}

func ruleExistsHelper(ctx context.Context, s *terraform.State, client *client.JupiterOneClient) error {
	for _, r := range s.RootModule().Resources {
		err := resource.RetryContext(ctx, 10*time.Second, func() *resource.RetryError {
			id := r.Primary.ID
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
	}

	return nil
}

func testAccCheckRuleInstanceDestroy(ctx context.Context, accProvider *schema.Provider) func(*terraform.State) error {
	return func(s *terraform.State) error {
		providerConf := accProvider.Meta().(*ProviderConfiguration)
		client := providerConf.Client

		if err := ruleInstanceDestroyHelper(ctx, s, client); err != nil {
			return err
		}
		return nil
	}
}

func ruleInstanceDestroyHelper(ctx context.Context, s *terraform.State, client *client.JupiterOneClient) error {
	for _, r := range s.RootModule().Resources {
		err := resource.RetryContext(ctx, 30*time.Second, func() *resource.RetryError {
			id := r.Primary.ID
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
	}

	return nil
}

func testRuleInstanceBasicConfigWithOperations(rName string, operations string) string {
	return fmt.Sprintf(`
		resource "jupiterone_rule" "test" {
			name = %q
			description = "Test"
			spec_version = 1
			polling_interval = "ONE_DAY"

			question {
				queries {
					name = "query0"
					query = "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"
					version = "v1"
					results_are = "UNKNOWN"
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

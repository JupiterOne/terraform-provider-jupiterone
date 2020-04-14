package jupiterone

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"

	jupiterone "github.com/jupiterone/terraform-provider-jupiterone/jupiterone_client"
)

func TestQuestion_Basic(t *testing.T) {
	accProviders, cleanup := testAccProviders(t)
	defer cleanup(t)
	accProvider := testAccProvider(t, accProviders)
	rName := acctest.RandomWithPrefix("tf-acc-test")
	resourceName := "jupiterone_question.test"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    accProviders,
		CheckDestroy: testAccCheckQuestionDestroy(accProvider),
		Steps: []resource.TestStep{
			{
				Config: testQuestionBasicConfigWithTags(rName, rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckQuestionExists(accProvider),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "title", rName),
					resource.TestCheckResourceAttr(resourceName, "description", "Test"),
					resource.TestCheckResourceAttr(resourceName, "tags.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.0", rName),
					resource.TestCheckResourceAttr(resourceName, "query.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "query.0.name", "query0"),
					resource.TestCheckResourceAttr(resourceName, "query.0.version", "v1"),
					resource.TestCheckResourceAttr(resourceName, "query.0.query", "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"),
				),
			},
			{
				Config: testQuestionBasicConfigWithTags(rName, rName+"-1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckQuestionExists(accProvider),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "title", rName),
					resource.TestCheckResourceAttr(resourceName, "description", "Test"),
					resource.TestCheckResourceAttr(resourceName, "tags.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.0", rName+"-1"),
					resource.TestCheckResourceAttr(resourceName, "query.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "query.0.name", "query0"),
					resource.TestCheckResourceAttr(resourceName, "query.0.version", "v1"),
					resource.TestCheckResourceAttr(resourceName, "query.0.query", "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"),
				),
			},
		},
	})
}

func testAccCheckQuestionExists(accProvider *schema.Provider) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		providerConf := accProvider.Meta().(*ProviderConfiguration)
		client := providerConf.Client

		if err := questionExistsHelper(s, client); err != nil {
			return err
		}
		return nil
	}
}

func questionExistsHelper(s *terraform.State, client *jupiterone.JupiterOneClient) error {
	for _, r := range s.RootModule().Resources {
		err := resource.Retry(10*time.Second, func() *resource.RetryError {
			id := r.Primary.ID
			question, err := client.GetQuestion(id)

			if question != nil {
				return nil
			}

			if err != nil && strings.Contains(err.Error(), "Cannot fetch question that does not exist") {
				return resource.RetryableError(fmt.Errorf("Question does not exist (id=%q)", id))
			}

			return resource.NonRetryableError(err)
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func testAccCheckQuestionDestroy(accProvider *schema.Provider) func(*terraform.State) error {
	return func(s *terraform.State) error {
		providerConf := accProvider.Meta().(*ProviderConfiguration)
		client := providerConf.Client

		if err := questionDestroyHelper(s, client); err != nil {
			return err
		}
		return nil
	}
}

func questionDestroyHelper(s *terraform.State, client *jupiterone.JupiterOneClient) error {
	for _, r := range s.RootModule().Resources {
		err := resource.Retry(30*time.Second, func() *resource.RetryError {
			id := r.Primary.ID
			question, err := client.GetQuestion(id)

			if question != nil {
				return resource.RetryableError(fmt.Errorf("Question still exists (id=%q)", id))
			}

			if err != nil && strings.Contains(err.Error(), "Cannot fetch question that does not exist") {
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

func testQuestionBasicConfigWithTags(rName string, tag string) string {
	return fmt.Sprintf(`
resource "jupiterone_question" "test" {
	title = %q
	description = "Test"
	tags = [%q]

	query {
		name = "query0"
		query = "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"
		version = "v1"
	}
}
`, rName, tag)
}

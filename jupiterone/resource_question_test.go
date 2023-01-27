package jupiterone

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/logging"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

func TestQuestion_Basic(t *testing.T) {
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

	resourceName := "jupiterone_question.test"
	title := "tf-test-question"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(testJ1Client),
		CheckDestroy:             testAccCheckQuestionDestroy(ctx, testJ1Client),
		Steps: []resource.TestStep{
			{
				Config: testQuestionBasicConfigWithTags(title, "testing-tag-1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckQuestionExists(ctx, testJ1Client),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "title", title),
					resource.TestCheckResourceAttr(resourceName, "description", "Test"),
					resource.TestCheckResourceAttr(resourceName, "tags.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.0", "testing-tag-1"),
					resource.TestCheckResourceAttr(resourceName, "query.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "query.0.name", "query0"),
					resource.TestCheckResourceAttr(resourceName, "query.0.version", "v1"),
					resource.TestCheckResourceAttr(resourceName, "query.0.query", "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"),
				),
			},
			{
				Config: testQuestionBasicConfigWithTags(title, "testing-tag-2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckQuestionExists(ctx, testJ1Client),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "title", title),
					resource.TestCheckResourceAttr(resourceName, "description", "Test"),
					resource.TestCheckResourceAttr(resourceName, "tags.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.0", "testing-tag-2"),
					resource.TestCheckResourceAttr(resourceName, "query.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "query.0.name", "query0"),
					resource.TestCheckResourceAttr(resourceName, "query.0.version", "v1"),
					resource.TestCheckResourceAttr(resourceName, "query.0.query", "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"),
				),
			},
		},
	})
}

func testAccCheckQuestionExists(ctx context.Context, client *client.JupiterOneClient) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if err := questionExistsHelper(ctx, s, client); err != nil {
			return err
		}
		return nil
	}
}

func questionExistsHelper(ctx context.Context, s *terraform.State, client *client.JupiterOneClient) error {
	for _, r := range s.RootModule().Resources {
		err := resource.RetryContext(ctx, 10*time.Second, func() *resource.RetryError {
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

func testAccCheckQuestionDestroy(ctx context.Context, client *client.JupiterOneClient) func(*terraform.State) error {
	return func(s *terraform.State) error {
		if err := questionDestroyHelper(ctx, s, client); err != nil {
			return err
		}
		return nil
	}
}

func questionDestroyHelper(ctx context.Context, s *terraform.State, client *client.JupiterOneClient) error {
	for _, r := range s.RootModule().Resources {
		err := resource.RetryContext(ctx, 30*time.Second, func() *resource.RetryError {
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
		provider "jupiterone" {}

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

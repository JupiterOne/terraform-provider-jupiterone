package jupiterone

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

func TestQuestion_Basic(t *testing.T) {
	ctx := context.TODO()

	recorder, cleanup := setupCassettes(t.Name())
	defer cleanup(t)
	testHttpClient := cleanhttp.DefaultClient()
	testHttpClient.Transport = recorder
	qlient, err := client.NewQlientFromEnv(ctx, testHttpClient)
	if err != nil {
		t.Fatal("error configuring check client", err)
	}

	resourceName := "jupiterone_question.test"
	questionName := "tf-provider-test-question"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(qlient),
		CheckDestroy:             testAccCheckQuestionDestroy(ctx, qlient),
		Steps: []resource.TestStep{
			{
				Config: testQuestionBasicConfigWithTags(questionName, "tf_acc:1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckQuestionExists(ctx, qlient),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "title", questionName),
					resource.TestCheckResourceAttr(resourceName, "description", "Test"),
					resource.TestCheckResourceAttr(resourceName, "tags.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.0", "tf_acc:1"),
					resource.TestCheckResourceAttr(resourceName, "query.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "query.0.name", "query0"),
					resource.TestCheckResourceAttr(resourceName, "query.0.version", "v1"),
					resource.TestCheckResourceAttr(resourceName, "query.0.query", "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"),
				),
			},
			{
				Config: testQuestionBasicConfigWithTags(questionName, "tf_acc:2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckQuestionExists(ctx, qlient),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "title", questionName),
					resource.TestCheckResourceAttr(resourceName, "description", "Test"),
					resource.TestCheckResourceAttr(resourceName, "tags.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.0", "tf_acc:2"),
					resource.TestCheckResourceAttr(resourceName, "query.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "query.0.name", "query0"),
					resource.TestCheckResourceAttr(resourceName, "query.0.version", "v1"),
					resource.TestCheckResourceAttr(resourceName, "query.0.query", "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"),
				),
			},
		},
	})
}

func testAccCheckQuestionExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if err := questionExistsHelper(ctx, s, qlient); err != nil {
			return err
		}
		return nil
	}
}

func questionExistsHelper(ctx context.Context, s *terraform.State, qlient graphql.Client) error {
	duration := 10 * time.Second
	if isReplaying() {
		// no reason to wait as long on replays, but the retries would be recorded and
		// have to be exercised and this can't be set to 0.
		duration = time.Second
	}
	for _, r := range s.RootModule().Resources {
		err := resource.RetryContext(ctx, duration, func() *resource.RetryError {
			id := r.Primary.ID
			_, err := client.GetQuestionById(ctx, qlient, id)

			if err == nil {
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

func testAccCheckQuestionDestroy(ctx context.Context, qlient graphql.Client) func(*terraform.State) error {
	return func(s *terraform.State) error {
		if err := questionDestroyHelper(ctx, s, qlient); err != nil {
			return err
		}
		return nil
	}
}

func questionDestroyHelper(ctx context.Context, s *terraform.State, qlient graphql.Client) error {
	duration := 10 * time.Second
	if isReplaying() {
		// no reason to wait as long on replays, but the retries would be recorded and
		// have to be exercised and this can't be set to 0.
		duration = time.Second
	}
	for _, r := range s.RootModule().Resources {
		err := resource.RetryContext(ctx, duration, func() *resource.RetryError {
			id := r.Primary.ID
			_, err := client.GetQuestionById(ctx, qlient, id)

			if err == nil {
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

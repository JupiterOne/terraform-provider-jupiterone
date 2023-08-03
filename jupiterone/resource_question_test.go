package jupiterone

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

func TestQuestion_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	resourceName := "jupiterone_question.test"
	questionTitle := "tf-provider-test-question"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckQuestionDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testQuestionBasicConfigWithTags(questionTitle, "tf_acc:1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckQuestionExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "title", questionTitle),
					resource.TestCheckResourceAttr(resourceName, "description", "Test"),
					resource.TestCheckResourceAttr(resourceName, "show_trend", "false"),
					resource.TestCheckResourceAttr(resourceName, "tags.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.0", "tf_acc:1"),
					resource.TestCheckResourceAttr(resourceName, "query.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "query.0.name", "query0"),
					resource.TestCheckResourceAttr(resourceName, "query.0.version", "v1"),
					resource.TestCheckResourceAttr(resourceName, "query.0.query", "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"),
				),
			},
			{
				Config: testQuestionBasicConfigWithTags(questionTitle, "tf_acc:2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckQuestionExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "title", questionTitle),
					resource.TestCheckResourceAttr(resourceName, "description", "Test"),
					resource.TestCheckResourceAttr(resourceName, "show_trend", "false"),
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

func TestQuestion_BasicImport(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	resourceName := "jupiterone_question.test"
	questionTitle := "tf-provider-test-question"
	questionTags := []string{"tf_acc:1"}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckQuestionDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				ImportState:   true,
				ResourceName:  resourceName,
				ImportStateId: createTestQuestion(ctx, t, recordingClient, questionTitle, questionTags),
				Config:        testQuestionBasicConfigWithTags(questionTitle, questionTags[0]),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckQuestionExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "title", questionTitle),
					resource.TestCheckResourceAttr(resourceName, "description", "Test"),
					resource.TestCheckResourceAttr(resourceName, "show_trend", "false"),
					resource.TestCheckResourceAttr(resourceName, "tags.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.0", "tf_acc:1"),
					resource.TestCheckResourceAttr(resourceName, "query.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "query.0.name", "query0"),
					resource.TestCheckResourceAttr(resourceName, "query.0.version", "v1"),
					resource.TestCheckResourceAttr(resourceName, "query.0.query", "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"),
				),
			},
		},
	})
}

func TestQuestion_Config_Errors(t *testing.T) {
	ctx := context.TODO()

	recordingClient, _, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	questionTitle := acctest.RandomWithPrefix("tf-provider-test-question")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		Steps: []resource.TestStep{
			{
				Config:      testQuestionBasicConfigWithShowTrend(questionTitle, "INVALID_SHOW_TREND"),
				ExpectError: regexp.MustCompile(`Inappropriate value for attribute "show_trend"`),
			},
		},
	})
}

// createTestQuestion directly calls the client to create a question directly
// for import or other tests. Because the id must be returned, this must
// called with the recorder client.
func createTestQuestion(ctx context.Context, t *testing.T, qlient graphql.Client, title string, tags []string) string {
	r, err := client.CreateQuestion(ctx, qlient, client.CreateQuestionInput{
		Title:           title,
		Description:     "test",
		Tags:            tags,
		PollingInterval: client.SchedulerPollingIntervalOneDay,
		Queries: []client.QuestionQueryInput{
			{
				Name:       "query0",
				Query:      "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true",
				Version:    "v1",
				ResultsAre: client.QueryResultsAreBad,
			},
		},
	})
	if err != nil {
		t.Log("error creating question for import test", err)
		t.FailNow()
	}

	return r.CreateQuestion.Id
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
	if qlient == nil {
		return nil
	}

	duration := 10 * time.Second
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
	if qlient == nil {
		return nil
	}

	duration := 10 * time.Second
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

func testQuestionBasicConfigWithShowTrend(rName string, showTrend string) string {
	return fmt.Sprintf(`
		provider "jupiterone" {}

		resource "jupiterone_question" "test" {
			title = %q
			description = "Test"
			show_trend = %q

			query {
				name = "query0"
				query = "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"
				version = "v1"
			}
		}
	`, rName, showTrend)
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

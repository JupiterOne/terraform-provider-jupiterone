package jupiterone

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

const testGroupName = "tf-provider-acc test Group"
const testGroupResourceName = "jupiterone_group.test"
const testWebLink = "https://community.askj1.com/kb/articles/795-compliance-api-endpoints"

func TestGroup_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	updatedName := "tf-provider-acc-test updated name"
	testDescription := "tf-provider acceptance test group"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckGroupDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testFrameworkBasicConfig(testFrameworkName, "[]") + testGroupEmptyConfig(testGroupName, testFrameworkResourceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testGroupResourceName, "framework_id"),
					resource.TestCheckResourceAttr(testGroupResourceName, "name", testGroupName),
					resource.TestCheckNoResourceAttr(testGroupResourceName, "description"),
					resource.TestCheckNoResourceAttr(testGroupResourceName, "display_category"),
					resource.TestCheckNoResourceAttr(testGroupResourceName, "web_link"),
				),
			},
			{
				Config: testFrameworkBasicConfig(testFrameworkName, "[]") + testGroupBasicConfig(testGroupName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testGroupResourceName, "id"),
					resource.TestCheckResourceAttrSet(testGroupResourceName, "framework_id"),
					resource.TestCheckResourceAttr(testGroupResourceName, "name", testGroupName),
					resource.TestCheckResourceAttr(testGroupResourceName, "description", testDescription),
					resource.TestCheckResourceAttr(testGroupResourceName, "display_category", "first"),
					resource.TestCheckResourceAttr(testGroupResourceName, "web_link", testWebLink),
				),
			},
			{
				Config: testFrameworkBasicConfig(testFrameworkName, "[]") + testGroupBasicConfig(updatedName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testGroupResourceName, "framework_id"),
					resource.TestCheckResourceAttrSet(testGroupResourceName, "description"),
					resource.TestCheckResourceAttr(testGroupResourceName, "name", updatedName),
					resource.TestCheckResourceAttr(testGroupResourceName, "display_category", "first"),
					resource.TestCheckResourceAttr(testGroupResourceName, "web_link", testWebLink),
				),
			},
			{
				Config: testFrameworkBasicConfig(testFrameworkName, "[]") + testGroupEmptyConfig(testGroupName, testFrameworkResourceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGroupExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testGroupResourceName, "framework_id"),
					resource.TestCheckResourceAttr(testGroupResourceName, "name", testGroupName),
					resource.TestCheckNoResourceAttr(testGroupResourceName, "description"),
					resource.TestCheckNoResourceAttr(testGroupResourceName, "display_category"),
					resource.TestCheckNoResourceAttr(testGroupResourceName, "web_link"),
				),
			},
		},
	})
}

func testAccCheckGroupExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		duration := 10 * time.Second
		for _, r := range s.RootModule().Resources {
			if r.Type != "jupiterone_group" {
				continue
			}
			frameworkId, ok := r.Primary.Attributes["framework_id"]
			if !ok {
				return errors.New("no framework id in group")
			}
			err := resource.RetryContext(ctx, duration, func() *resource.RetryError {
				id := r.Primary.ID
				_, err := getGroup(ctx, qlient, frameworkId, id)

				if err == nil {
					return nil
				}

				if err != nil && strings.Contains(err.Error(), "Could not find compliance framework with id") {
					return resource.RetryableError(fmt.Errorf("Group does not exist (id=%q)", id))
				}

				return resource.NonRetryableError(err)
			})

			if err != nil {
				return err
			}
		}

		return nil
	}
}

func testAccCheckGroupDestroy(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		duration := 10 * time.Second
		for _, r := range s.RootModule().Resources {
			if r.Type != "jupiterone_group" {
				continue
			}
			frameworkId, ok := r.Primary.Attributes["framework_id"]
			if !ok {
				return errors.New("no framework id in group")
			}
			err := resource.RetryContext(ctx, duration, func() *resource.RetryError {
				id := r.Primary.ID
				_, err := getGroup(ctx, qlient, frameworkId, id)

				if err == nil {
					return resource.RetryableError(fmt.Errorf("Group still exists (id=%q)", id))
				}

				if err != nil && strings.Contains(err.Error(), "Could not find compliance framework with id") {
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
}

// testGroupBasicConfig must be added to a provider and framework definition
func testGroupBasicConfig(name string) string {
	return fmt.Sprintf(`
	resource "jupiterone_group" "test" {
		name           = %q
		framework_id = %s.id
		description = "tf-provider acceptance test group"
		display_category = "first"

		web_link = "https://community.askj1.com/kb/articles/795-compliance-api-endpoints"
	}
	`,
		name, testFrameworkResourceName)
}

// testGroupEmptyConfig must be added to a provider and framework definition
func testGroupEmptyConfig(name, frameworkResourceName string) string {
	return fmt.Sprintf(`
	resource "jupiterone_group" "test" {
		name           = %q
		framework_id = %s.id
	}
	`,
		name, frameworkResourceName)
}

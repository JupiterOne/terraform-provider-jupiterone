package jupiterone

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

const testFrameworkItemName = "tf-provider-acc test FrameworkItem"
const testFrameworkItemResourceName = "jupiterone_frameworkitem.test"

func TestFrameworkItem_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	updatedName := "tf-provider-acc-test updated name"
	testDescription := "tf-provider acceptance test framework item"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckFrameworkItemDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testFrameworkBasicConfig(testFrameworkName, "[]") +
					testGroupBasicConfig(testFrameworkItemName) +
					testFrameworkItemEmptyConfig(testFrameworkItemName, testFrameworkResourceName, testGroupResourceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckFrameworkItemExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testFrameworkItemResourceName, "framework_id"),
					resource.TestCheckResourceAttr(testFrameworkItemResourceName, "name", testFrameworkItemName),
					resource.TestCheckResourceAttr(testFrameworkItemResourceName, "ref", "test-requirement-1"),
					resource.TestCheckNoResourceAttr(testFrameworkItemResourceName, "description"),
					resource.TestCheckNoResourceAttr(testFrameworkItemResourceName, "display_category"),
					resource.TestCheckNoResourceAttr(testFrameworkItemResourceName, "web_link"),
				),
			},
			{
				Config: testFrameworkBasicConfig(testFrameworkName, "[]") +
					testGroupBasicConfig(testFrameworkItemName) +
					testFrameworkItemBasicConfig(testFrameworkItemName, testFrameworkResourceName, testGroupResourceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckFrameworkItemExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testFrameworkItemResourceName, "id"),
					resource.TestCheckResourceAttrSet(testFrameworkItemResourceName, "framework_id"),
					resource.TestCheckResourceAttr(testFrameworkItemResourceName, "name", testFrameworkItemName),
					resource.TestCheckResourceAttr(testFrameworkItemResourceName, "ref", "test-requirement-1"),
					resource.TestCheckResourceAttr(testFrameworkItemResourceName, "description", testDescription),
					resource.TestCheckResourceAttr(testFrameworkItemResourceName, "display_category", "second"),
					resource.TestCheckResourceAttr(testFrameworkItemResourceName, "web_link", testWebLink),
				),
			},
			{
				Config: testFrameworkBasicConfig(testFrameworkName, "[]") +
					testGroupBasicConfig(testFrameworkItemName) +
					testFrameworkItemBasicConfig(updatedName, testFrameworkResourceName, testGroupResourceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckFrameworkItemExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testFrameworkItemResourceName, "framework_id"),
					resource.TestCheckResourceAttrSet(testFrameworkItemResourceName, "description"),
					resource.TestCheckResourceAttr(testFrameworkItemResourceName, "name", updatedName),
					resource.TestCheckResourceAttr(testFrameworkItemResourceName, "ref", "test-requirement-1"),
					resource.TestCheckResourceAttr(testFrameworkItemResourceName, "display_category", "second"),
					resource.TestCheckResourceAttr(testFrameworkItemResourceName, "web_link", testWebLink),
				),
			},
			{
				Config: testFrameworkBasicConfig(testFrameworkName, "[]") +
					testGroupBasicConfig(testFrameworkItemName) +
					testFrameworkItemEmptyConfig(updatedName, testFrameworkResourceName, testGroupResourceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckFrameworkItemExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testFrameworkItemResourceName, "framework_id"),
					resource.TestCheckResourceAttr(testFrameworkItemResourceName, "name", updatedName),
					resource.TestCheckNoResourceAttr(testFrameworkItemResourceName, "description"),
					resource.TestCheckNoResourceAttr(testFrameworkItemResourceName, "display_category"),
					resource.TestCheckNoResourceAttr(testFrameworkItemResourceName, "web_link"),
				),
			},
		},
	})
}

func testAccCheckFrameworkItemExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		duration := 10 * time.Second
		for _, r := range s.RootModule().Resources {
			if r.Type != "jupiterone_frameworkitem" {
				continue
			}
			err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
				id := r.Primary.ID
				_, err := client.GetComplianceFrameworkItemById(ctx, qlient, id)

				if err == nil {
					return nil
				}

				if strings.Contains(err.Error(), "Could not find compliance framework item with id") {
					return retry.RetryableError(fmt.Errorf("FrameworkItem does not exist (id=%q)", id))
				}

				return retry.NonRetryableError(err)
			})

			if err != nil {
				return err
			}
		}

		return nil
	}
}

func testAccCheckFrameworkItemDestroy(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		duration := 10 * time.Second
		for _, r := range s.RootModule().Resources {
			if r.Type != "jupiterone_frameworkitem" {
				continue
			}
			err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
				id := r.Primary.ID
				_, err := client.GetComplianceFrameworkItemById(ctx, qlient, id)

				if err == nil {
					return retry.RetryableError(fmt.Errorf("FrameworkItem still exists (id=%q)", id))
				}

				if strings.Contains(err.Error(), "Could not find compliance framework item with id") {
					return nil
				}

				return retry.NonRetryableError(err)
			})

			if err != nil {
				return err
			}
		}

		return nil
	}
}

// testFrameworkItemBasicConfig must be added to a provider and framework definition
func testFrameworkItemBasicConfig(name, frameworkResourceName, groupResourceName string) string {
	return fmt.Sprintf(`
	resource "jupiterone_frameworkitem" "test" {
		name           = %q
		ref = "test-requirement-1"
		framework_id = %s.id
		group_id = %s.id
		description = "tf-provider acceptance test framework item"
		display_category = "second"

		web_link = "https://community.askj1.com/kb/articles/795-compliance-api-endpoints"
	}
	`,
		name, frameworkResourceName, groupResourceName)
}

// testFrameworkItemEmptyConfig must be added to a provider and framework definition
func testFrameworkItemEmptyConfig(name, frameworkResourceName, groupResourceName string) string {
	return fmt.Sprintf(`
	resource "jupiterone_frameworkitem" "test" {
		name         = %q
		ref          = "test-requirement-1"
		framework_id = %s.id
		group_id     = %s.id
	}
	`,
		name, frameworkResourceName, groupResourceName)
}

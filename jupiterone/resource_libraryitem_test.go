package jupiterone

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

const testLibraryItemName = "tf-provider-acc test LibraryItem"
const testLibraryItemResourceName = "jupiterone_libraryitem.test"

func TestLibraryItem_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	updatedName := "tf-provider-acc-test updated name"
	testDescription := "tf-provider acceptance test library item"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckLibraryItemDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testLibraryItemEmptyConfig(testLibraryItemName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLibraryItemExists(ctx, directClient),
					resource.TestCheckResourceAttr(testLibraryItemResourceName, "name", testLibraryItemName),
					resource.TestCheckResourceAttr(testLibraryItemResourceName, "ref", "test-requirement-1"),
					resource.TestCheckNoResourceAttr(testLibraryItemResourceName, "description"),
					resource.TestCheckNoResourceAttr(testLibraryItemResourceName, "display_category"),
					resource.TestCheckNoResourceAttr(testLibraryItemResourceName, "web_link"),
				),
			},
			{
				Config: testLibraryItemBasicConfig(testLibraryItemName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLibraryItemExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testLibraryItemResourceName, "id"),
					resource.TestCheckResourceAttr(testLibraryItemResourceName, "name", testLibraryItemName),
					resource.TestCheckResourceAttr(testLibraryItemResourceName, "ref", "test-requirement-2"),
					resource.TestCheckResourceAttr(testLibraryItemResourceName, "description", testDescription),
					resource.TestCheckResourceAttr(testLibraryItemResourceName, "display_category", "third"),
					resource.TestCheckResourceAttr(testLibraryItemResourceName, "web_link", testWebLink),
				),
			},
			{
				Config: testLibraryItemBasicConfig(updatedName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLibraryItemExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testLibraryItemResourceName, "description"),
					resource.TestCheckResourceAttr(testLibraryItemResourceName, "name", updatedName),
					resource.TestCheckResourceAttr(testLibraryItemResourceName, "ref", "test-requirement-2"),
					resource.TestCheckResourceAttr(testLibraryItemResourceName, "display_category", "third"),
					resource.TestCheckResourceAttr(testLibraryItemResourceName, "web_link", testWebLink),
				),
			},
			{
				Config: testLibraryItemEmptyConfig(updatedName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLibraryItemExists(ctx, directClient),
					resource.TestCheckResourceAttr(testLibraryItemResourceName, "name", updatedName),
					resource.TestCheckResourceAttr(testLibraryItemResourceName, "ref", "test-requirement-1"),
					resource.TestCheckNoResourceAttr(testLibraryItemResourceName, "description"),
					resource.TestCheckNoResourceAttr(testLibraryItemResourceName, "display_category"),
					resource.TestCheckNoResourceAttr(testLibraryItemResourceName, "web_link"),
				),
			},
		},
	})
}

func testAccCheckLibraryItemExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		duration := 10 * time.Second
		for _, r := range s.RootModule().Resources {
			if r.Type != "jupiterone_libraryitem" {
				continue
			}
			err := resource.RetryContext(ctx, duration, func() *resource.RetryError {
				id := r.Primary.ID
				_, err := client.GetComplianceLibraryItemById(ctx, qlient, id)

				if err == nil {
					return nil
				}

				if err != nil && strings.Contains(err.Error(), "Could not find compliance library item with id") {
					return resource.RetryableError(fmt.Errorf("LibraryItem does not exist (id=%q)", id))
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

func testAccCheckLibraryItemDestroy(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		duration := 10 * time.Second
		for _, r := range s.RootModule().Resources {
			if r.Type != "jupiterone_libraryitem" {
				continue
			}
			err := resource.RetryContext(ctx, duration, func() *resource.RetryError {
				id := r.Primary.ID
				_, err := client.GetComplianceLibraryItemById(ctx, qlient, id)

				if err == nil {
					return resource.RetryableError(fmt.Errorf("LibraryItem still exists (id=%q)", id))
				}

				if err != nil && strings.Contains(err.Error(), "Could not find compliance library item with id") {
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

// testLibraryItemBasicConfig
func testLibraryItemBasicConfig(name string) string {
	return fmt.Sprintf(`
	resource "jupiterone_libraryitem" "test" {
		name           = %q
		ref = "test-requirement-2"
		description = "tf-provider acceptance test library item"
		display_category = "third"

		web_link = "https://community.askj1.com/kb/articles/795-compliance-api-endpoints"
	}
	`,
		name)
}

// testLibraryItemEmptyConfig must be added to a provider and framework definition
func testLibraryItemEmptyConfig(name string) string {
	return fmt.Sprintf(`
	resource "jupiterone_libraryitem" "test" {
		name         = %q
		ref          = "test-requirement-1"
	}
	`,
		name)
}

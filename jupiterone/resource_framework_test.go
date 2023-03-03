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

const testFrameworkName = "tf-provider-acc-test-framework"
const testFrameworkResourceName = "jupiterone_framework.test"

func TestFramework_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	updatedName := "Updated Framework Name"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckFrameworkDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testFrameworkBasicConfig(testFrameworkName, testEnvScopeFilters),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckFrameworkExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testFrameworkResourceName, "id"),
					resource.TestCheckResourceAttr(testFrameworkResourceName, "name", testFrameworkName),
					resource.TestCheckResourceAttr(testFrameworkResourceName, "version", "v1"),
					resource.TestCheckResourceAttr(testFrameworkResourceName, "scope_filters.#", "1"),
				),
			},
			{
				Config: testFrameworkBasicConfig(updatedName, "[]"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckFrameworkExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testFrameworkResourceName, "id"),
					resource.TestCheckResourceAttr(testFrameworkResourceName, "name", updatedName),
					resource.TestCheckResourceAttr(testFrameworkResourceName, "version", "v1"),
					resource.TestCheckResourceAttr(testFrameworkResourceName, "scope_filters.#", "0"),
				),
			},
		},
	})
}

func testAccCheckFrameworkExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		duration := 10 * time.Second
		for _, r := range s.RootModule().Resources {
			err := resource.RetryContext(ctx, duration, func() *resource.RetryError {
				id := r.Primary.ID
				_, err := client.GetComplianceFrameworkById(ctx, qlient, id)

				if err == nil {
					return nil
				}

				if err != nil && strings.Contains(err.Error(), "Could not find compliance framework with id") {
					return resource.RetryableError(fmt.Errorf("Framework does not exist (id=%q)", id))
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

func testAccCheckFrameworkDestroy(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		duration := 10 * time.Second
		for _, r := range s.RootModule().Resources {
			err := resource.RetryContext(ctx, duration, func() *resource.RetryError {
				id := r.Primary.ID
				_, err := client.GetComplianceFrameworkById(ctx, qlient, id)

				if err == nil {
					return resource.RetryableError(fmt.Errorf("Framework still exists (id=%q)", id))
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

const testEnvScopeFilters = `[
			jsonencode({
				"env" : "prod"
			}),
		]
`

func testFrameworkBasicConfig(name, scopeFilters string) string {
	return fmt.Sprintf(`
	provider "jupiterone" {}

	resource "jupiterone_framework" "test" {
		name           = %q
		version        = "v1"
		framework_type = "STANDARD"

		web_link = "https://community.askj1.com/kb/articles/795-compliance-api-endpoints"

		scope_filters = %s
	}
	`,
		name, scopeFilters)
}

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

const testControlFrameworkName = "tf-provider-acc-test-control-framework"
const testControlFrameworkResourceName = "jupiterone_control_framework.test"

func TestControlFramework_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClientsWithReplaySupport(ctx, t)
	defer cleanup(t)

	updatedName := "tf-provider-acc-test updated control framework"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckControlFrameworkDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testControlFrameworkBasicConfig(testControlFrameworkName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckControlFrameworkExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testControlFrameworkResourceName, "id"),
					resource.TestCheckResourceAttr(testControlFrameworkResourceName, "name", testControlFrameworkName),
					resource.TestCheckResourceAttr(testControlFrameworkResourceName, "description", "acceptance test control framework"),
					resource.TestCheckResourceAttr(testControlFrameworkResourceName, "owner", "test-owner@jupiterone.com"),
				),
			},
			{
				Config: testControlFrameworkBasicConfig(updatedName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckControlFrameworkExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testControlFrameworkResourceName, "id"),
					resource.TestCheckResourceAttr(testControlFrameworkResourceName, "name", updatedName),
					resource.TestCheckResourceAttr(testControlFrameworkResourceName, "description", "acceptance test control framework"),
					resource.TestCheckResourceAttr(testControlFrameworkResourceName, "owner", "test-owner@jupiterone.com"),
				),
			},
		},
	})
}

func testAccCheckControlFrameworkExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		duration := 10 * time.Second
		for _, r := range s.RootModule().Resources {
			if r.Type != "jupiterone_control_framework" {
				continue
			}
			err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
				id := r.Primary.ID
				_, err := client.GetFrameworkById(ctx, qlient, id)

				if err == nil {
					return nil
				}

				if strings.Contains(err.Error(), "Could not find") {
					return retry.RetryableError(fmt.Errorf("ControlFramework does not exist (id=%q)", id))
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

func testAccCheckControlFrameworkDestroy(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		duration := 10 * time.Second
		for _, r := range s.RootModule().Resources {
			if r.Type != "jupiterone_control_framework" {
				continue
			}
			err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
				id := r.Primary.ID
				_, err := client.GetFrameworkById(ctx, qlient, id)

				if err == nil {
					return retry.RetryableError(fmt.Errorf("ControlFramework still exists (id=%q)", id))
				}

				if strings.Contains(err.Error(), "Could not find") || strings.Contains(err.Error(), "not found") {
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

func testControlFrameworkBasicConfig(name string) string {
	return fmt.Sprintf(`
	provider "jupiterone" {}

	resource "jupiterone_control_framework" "test" {
		name        = %q
		description = "acceptance test control framework"
		owner       = "test-owner@jupiterone.com"
	}
	`,
		name)
}

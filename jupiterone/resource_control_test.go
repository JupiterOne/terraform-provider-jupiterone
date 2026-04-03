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

const testControlName = "tf-provider-acc-test-control"
const testControlResourceName = "jupiterone_control.test"

func TestControl_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClientsWithReplaySupport(ctx, t)
	defer cleanup(t)

	updatedName := "tf-provider-acc-test updated control"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckControlDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testControlBasicConfig(testControlName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckControlExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testControlResourceName, "id"),
					resource.TestCheckResourceAttr(testControlResourceName, "name", testControlName),
					resource.TestCheckResourceAttr(testControlResourceName, "description", "acceptance test control"),
					resource.TestCheckResourceAttr(testControlResourceName, "owner", "test-owner@jupiterone.com"),
					resource.TestCheckResourceAttr(testControlResourceName, "state", "DRAFT"),
				),
			},
			{
				Config: testControlBasicConfig(updatedName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckControlExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testControlResourceName, "id"),
					resource.TestCheckResourceAttr(testControlResourceName, "name", updatedName),
					resource.TestCheckResourceAttr(testControlResourceName, "description", "acceptance test control"),
					resource.TestCheckResourceAttr(testControlResourceName, "owner", "test-owner@jupiterone.com"),
					resource.TestCheckResourceAttr(testControlResourceName, "state", "DRAFT"),
				),
			},
		},
	})
}

func testAccCheckControlExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		duration := 10 * time.Second
		for _, r := range s.RootModule().Resources {
			if r.Type != "jupiterone_control" {
				continue
			}
			err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
				id := r.Primary.ID
				_, err := client.GetControlById(ctx, qlient, id)

				if err == nil {
					return nil
				}

				if strings.Contains(err.Error(), "Could not find") || strings.Contains(err.Error(), "not found") {
					return retry.RetryableError(fmt.Errorf("Control does not exist (id=%q)", id))
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

func testAccCheckControlDestroy(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		duration := 10 * time.Second
		for _, r := range s.RootModule().Resources {
			if r.Type != "jupiterone_control" {
				continue
			}
			err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
				id := r.Primary.ID
				_, err := client.GetControlById(ctx, qlient, id)

				if err == nil {
					return retry.RetryableError(fmt.Errorf("Control still exists (id=%q)", id))
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

func testControlBasicConfig(name string) string {
	return fmt.Sprintf(`
	provider "jupiterone" {}

	resource "jupiterone_control" "test" {
		name        = %q
		description = "acceptance test control"
		owner       = "test-owner@jupiterone.com"
		state       = "DRAFT"
	}
	`,
		name)
}

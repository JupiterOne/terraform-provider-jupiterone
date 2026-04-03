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

const testControlTestName = "tf-provider-acc-test-control-test"
const testControlTestResourceName = "jupiterone_control_test.test"

func TestControlTest_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClientsWithReplaySupport(ctx, t)
	defer cleanup(t)

	updatedName := "tf-provider-acc-test updated control test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckControlTestDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testControlTestBasicConfig(testControlTestName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckControlTestExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testControlTestResourceName, "id"),
					resource.TestCheckResourceAttr(testControlTestResourceName, "name", testControlTestName),
					resource.TestCheckResourceAttr(testControlTestResourceName, "description", "acceptance test control test"),
					resource.TestCheckResourceAttr(testControlTestResourceName, "queries.#", "1"),
					resource.TestCheckResourceAttr(testControlTestResourceName, "queries.0.name", "Find all users"),
					resource.TestCheckResourceAttr(testControlTestResourceName, "queries.0.results_are", "GOOD"),
				),
			},
			{
				Config: testControlTestBasicConfig(updatedName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckControlTestExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testControlTestResourceName, "id"),
					resource.TestCheckResourceAttr(testControlTestResourceName, "name", updatedName),
					resource.TestCheckResourceAttr(testControlTestResourceName, "description", "acceptance test control test"),
					resource.TestCheckResourceAttr(testControlTestResourceName, "queries.#", "1"),
				),
			},
		},
	})
}

func testAccCheckControlTestExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		duration := 10 * time.Second
		for _, r := range s.RootModule().Resources {
			if r.Type != "jupiterone_control_test" {
				continue
			}
			err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
				id := r.Primary.ID
				_, err := client.GetControlTestById(ctx, qlient, id)

				if err == nil {
					return nil
				}

				if strings.Contains(err.Error(), "Could not find") || strings.Contains(err.Error(), "not found") {
					return retry.RetryableError(fmt.Errorf("ControlTest does not exist (id=%q)", id))
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

func testAccCheckControlTestDestroy(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		duration := 10 * time.Second
		for _, r := range s.RootModule().Resources {
			if r.Type != "jupiterone_control_test" {
				continue
			}
			err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
				id := r.Primary.ID
				_, err := client.GetControlTestById(ctx, qlient, id)

				if err == nil {
					return retry.RetryableError(fmt.Errorf("ControlTest still exists (id=%q)", id))
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

func testControlTestBasicConfig(name string) string {
	return fmt.Sprintf(`
	provider "jupiterone" {}

	resource "jupiterone_control" "test" {
		name  = "tf-provider-acc-test-control-for-test"
		owner = "test-owner@jupiterone.com"
		state = "DRAFT"
	}

	resource "jupiterone_control_test" "test" {
		name        = %q
		control_id  = jupiterone_control.test.id
		description = "acceptance test control test"

		queries = [
			{
				name        = "Find all users"
				query       = "FIND User"
				results_are = "GOOD"
			},
		]
	}
	`, name)
}

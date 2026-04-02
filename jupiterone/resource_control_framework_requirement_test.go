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

const testControlFrameworkRequirementTitle = "tf-provider-acc-test requirement"
const testControlFrameworkRequirementResourceName = "jupiterone_control_framework_requirement.test"

func TestControlFrameworkRequirement_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	updatedTitle := "tf-provider-acc-test updated requirement"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckControlFrameworkRequirementDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testControlFrameworkRequirementFullConfig(testControlFrameworkRequirementTitle),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckControlFrameworkRequirementExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testControlFrameworkRequirementResourceName, "id"),
					resource.TestCheckResourceAttrSet(testControlFrameworkRequirementResourceName, "framework_id"),
					resource.TestCheckResourceAttr(testControlFrameworkRequirementResourceName, "title", testControlFrameworkRequirementTitle),
					resource.TestCheckResourceAttr(testControlFrameworkRequirementResourceName, "description", "acceptance test requirement"),
					resource.TestCheckResourceAttr(testControlFrameworkRequirementResourceName, "identifier", "REQ-001"),
					resource.TestCheckResourceAttr(testControlFrameworkRequirementResourceName, "priority", "HIGH"),
					resource.TestCheckResourceAttr(testControlFrameworkRequirementResourceName, "section", "Section A"),
				),
			},
			{
				Config: testControlFrameworkRequirementFullConfig(updatedTitle),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckControlFrameworkRequirementExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testControlFrameworkRequirementResourceName, "id"),
					resource.TestCheckResourceAttrSet(testControlFrameworkRequirementResourceName, "framework_id"),
					resource.TestCheckResourceAttr(testControlFrameworkRequirementResourceName, "title", updatedTitle),
					resource.TestCheckResourceAttr(testControlFrameworkRequirementResourceName, "description", "acceptance test requirement"),
					resource.TestCheckResourceAttr(testControlFrameworkRequirementResourceName, "identifier", "REQ-001"),
					resource.TestCheckResourceAttr(testControlFrameworkRequirementResourceName, "priority", "HIGH"),
					resource.TestCheckResourceAttr(testControlFrameworkRequirementResourceName, "section", "Section A"),
				),
			},
		},
	})
}

func testAccCheckControlFrameworkRequirementExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		duration := 10 * time.Second
		for _, r := range s.RootModule().Resources {
			if r.Type != "jupiterone_control_framework_requirement" {
				continue
			}
			err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
				id := r.Primary.ID
				_, err := client.GetRequirementById(ctx, qlient, id)

				if err == nil {
					return nil
				}

				if strings.Contains(err.Error(), "Could not find") {
					return retry.RetryableError(fmt.Errorf("ControlFrameworkRequirement does not exist (id=%q)", id))
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

func testAccCheckControlFrameworkRequirementDestroy(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		duration := 10 * time.Second
		for _, r := range s.RootModule().Resources {
			if r.Type != "jupiterone_control_framework_requirement" {
				continue
			}
			err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
				id := r.Primary.ID
				_, err := client.GetRequirementById(ctx, qlient, id)

				if err == nil {
					return retry.RetryableError(fmt.Errorf("ControlFrameworkRequirement still exists (id=%q)", id))
				}

				if strings.Contains(err.Error(), "Could not find") {
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

func testControlFrameworkRequirementFullConfig(title string) string {
	return fmt.Sprintf(`
	provider "jupiterone" {}

	resource "jupiterone_control_framework" "test" {
		name        = "tf-provider-acc-test-req-framework"
		description = "framework for requirement testing"
	}

	resource "jupiterone_control_framework_requirement" "test" {
		title        = %q
		framework_id = jupiterone_control_framework.test.id
		description  = "acceptance test requirement"
		identifier   = "REQ-001"
		priority     = "HIGH"
		section      = "Section A"
	}
	`,
		title)
}

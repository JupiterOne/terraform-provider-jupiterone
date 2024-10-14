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

func TestWidget_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	resourceName := "jupiterone_widget.test"
	widgetTitle := "tf-provider-test-widget"
	widgetType := "number"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy: func(s *terraform.State) error {
			return testAccCheckWidgetDestroy(ctx, directClient)(s)
		},
		Steps: []resource.TestStep{
			{
				Config: testWidgetBasicConfig(widgetTitle, widgetType),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckWidgetExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "title", widgetTitle),
					resource.TestCheckResourceAttr(resourceName, "type", widgetType),
				),
			},
		},
	})
}

func testAccCheckWidgetDestroy(ctx context.Context, qlient graphql.Client) func(*terraform.State) error {
	return func(s *terraform.State) error {
		if err := widgetDestroyHelper(ctx, s, qlient); err != nil {
			return err
		}
		return nil
	}
}

func widgetDestroyHelper(ctx context.Context, s *terraform.State, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	duration := 10 * time.Second
	var dashboardId string
	for _, r := range s.RootModule().Resources {
		if r.Type == "jupiterone_dashboard" {
			dashboardId = r.Primary.ID
		}
		if r.Type == "jupiterone_widget" {
			err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
				id := r.Primary.ID
				_, err := client.GetWidget(ctx, qlient, dashboardId, "Account", id)

				if err == nil {
					return retry.RetryableError(fmt.Errorf("Widget still exists (id=%q)", id))
				}

				if strings.Contains(err.Error(), "Resource not found") {
					return nil
				}

				return retry.NonRetryableError(err)
			})

			if err != nil {
				return err
			}
		}
	}
	return nil
}

func testAccCheckWidgetExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if err := widgetExistsHelper(ctx, s, qlient); err != nil {
			return err
		}
		return nil
	}
}

func widgetExistsHelper(ctx context.Context, s *terraform.State, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	duration := 10 * time.Second
	var dashboardId string
	for _, r := range s.RootModule().Resources {
		err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
			if r.Type == "jupiterone_dashboard" {
				dashboardId = r.Primary.ID
			}
			if r.Type == "jupiterone_widget" {
				id := r.Primary.ID
				_, err := client.GetWidget(ctx, qlient, dashboardId, "Account", id)

				if err == nil {
					return nil
				}

				if strings.Contains(err.Error(), "Resource not found") {
					return retry.RetryableError(fmt.Errorf("Widget does not exist (id=%q)", id))
				}

				return retry.NonRetryableError(err)
			}
			return nil
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func testWidgetBasicConfig(widgetTitle string, widgetType string) string {
	return fmt.Sprintf(`
		provider "jupiterone" {}

		resource "jupiterone_dashboard" "test" {
			name = "tf-provider-test-dashboard"
			type = "Account"
		}

		resource "jupiterone_widget" "test" {
			title = %q
			dashboard_id = jupiterone_dashboard.test.id
			type = %q
			description = "This is a test widget"

			config = {
				queries = [{
					name = "Query1"
					query = "FIND *"
				}]
			}
		}
	`, widgetTitle, widgetType)
}

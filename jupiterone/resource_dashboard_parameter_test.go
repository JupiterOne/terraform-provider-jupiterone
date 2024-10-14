package jupiterone

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

func TestDashboardParameter_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	resourceName := "jupiterone_dashboard_parameter.test"
	dashboardResourceName := "jupiterone_dashboard.test"
	label := "Test Parameter"
	name := "testParameter"
	valueType := "string"
	paramType := "QUERY_VARIABLE"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccCheckDashboardParameterDestroy(ctx, directClient),
		),
		Steps: []resource.TestStep{
			{
				Config: testDashboardParameterBasicConfig(label, name, valueType, paramType),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDashboardParameterExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrPair(resourceName, "dashboard_id", dashboardResourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "label", label),
					resource.TestCheckResourceAttr(resourceName, "name", name),
					resource.TestCheckResourceAttr(resourceName, "value_type", valueType),
					resource.TestCheckResourceAttr(resourceName, "type", paramType),
				),
			},
			{
				Config: testDashboardParameterBasicConfig(label, name, valueType, paramType),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDashboardParameterExists(ctx, directClient),
				),
			},
		},
	})
}

func testAccCheckDashboardParameterDestroy(ctx context.Context, qlient graphql.Client) func(*terraform.State) error {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		for _, r := range s.RootModule().Resources {
			if r.Type == "jupiterone_dashboard_parameter" {
				id := r.Primary.ID
				time.Sleep(5 * time.Second)
				_, err := client.DashboardParameter(ctx, qlient, id)

				if err == nil {
					return fmt.Errorf("Dashboard parameter still exists (id=%q)", id)
				}

				if strings.Contains(err.Error(), "Dashboard parameter not found") {
					return nil
				}

				return fmt.Errorf("Unexpected error checking dashboard parameter (id=%s): %v", id, err)
			}
		}
		return nil
	}
}

func testAccCheckDashboardParameterExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return fmt.Errorf("graphql client is nil")
		}

		for _, r := range s.RootModule().Resources {
			if r.Type == "jupiterone_dashboard_parameter" {
				id := r.Primary.ID
				_, err := client.DashboardParameter(ctx, qlient, id)

				if err != nil {
					return fmt.Errorf("error getting dashboard parameter (id=%q): %w", id, err)
				}
			}
		}
		return nil
	}
}

func testDashboardParameterBasicConfig(label, name, valueType, paramType string) string {
	return fmt.Sprintf(`
		provider "jupiterone" {}

		resource "jupiterone_dashboard" "test" {
			name = "TF Test Dashboard 2"
			type = "Account"
		}

		resource "jupiterone_dashboard_parameter" "test" {
			dashboard_id = jupiterone_dashboard.test.id
			label = %q
			name = %q
			value_type = %q
			type = %q
			disable_custom_input = false
			require_value = true
		}
	`, label, name, valueType, paramType)
}

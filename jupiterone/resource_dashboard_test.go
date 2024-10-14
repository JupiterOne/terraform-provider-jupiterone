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

func TestDashboard_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	resourceName := "jupiterone_dashboard.test"
	dashboardName := "tf-provider-test-dashboard"
	dashboardType := client.BoardTypeAccount

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckDashboardDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testDashboardBasicConfig(dashboardName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDashboardExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", dashboardName),
					resource.TestCheckResourceAttr(resourceName, "type", string(dashboardType)),
				),
			},
		},
	})
}

func TestDashboard_BasicImport(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	resourceName := "jupiterone_dashboard.test"
	dashboardName := "tf-provider-test-dashboard-import"
	dashboardType := client.BoardTypeAccount

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		Steps: []resource.TestStep{
			{
				ImportState:   true,
				ResourceName:  resourceName,
				ImportStateId: createTestDashboard(ctx, t, recordingClient, dashboardName),
				Config:        testDashboardBasicConfig(dashboardName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDashboardExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", dashboardName),
					resource.TestCheckResourceAttr(resourceName, "type", string(dashboardType)),
				),
			},
		},
	})
}

// createTestDashboard directly calls the client to create a dashboard directly
// for import or other tests. Because the id must be returned, this must
// called with the recorder client.
func createTestDashboard(ctx context.Context, t *testing.T, qlient graphql.Client, name string) string {
	r, err := client.CreateDashboard(ctx, qlient, client.CreateInsightsDashboardInput{
		Name: name,
		Type: client.BoardTypeAccount,
	})
	if err != nil {
		t.Log("error creating dashboard import test", err)
		t.FailNow()
	}

	return r.CreateDashboard.Id
}

func testAccCheckDashboardExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if err := dashboardExistsHelper(ctx, s, qlient); err != nil {
			return err
		}
		return nil
	}
}

func dashboardExistsHelper(ctx context.Context, s *terraform.State, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	duration := 10 * time.Second
	for _, r := range s.RootModule().Resources {
		err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
			id := r.Primary.ID
			_, err := client.GetDashboard(ctx, qlient, id)

			if err == nil {
				return nil
			}

			if strings.Contains(err.Error(), "Dashboard string does not exist") {
				return retry.RetryableError(fmt.Errorf("Dashboard does not exist (id=%q)", id))
			}

			return retry.NonRetryableError(err)
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func testAccCheckDashboardDestroy(ctx context.Context, qlient graphql.Client) func(*terraform.State) error {
	return func(s *terraform.State) error {
		if err := dashboardDestroyHelper(ctx, s, qlient); err != nil {
			return err
		}
		return nil
	}
}

func dashboardDestroyHelper(ctx context.Context, s *terraform.State, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	duration := 10 * time.Second
	for _, r := range s.RootModule().Resources {
		err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
			id := r.Primary.ID
			_, err := client.GetDashboard(ctx, qlient, id)

			if err == nil {
				return retry.RetryableError(fmt.Errorf("Dashboard still exists (id=%q)", id))
			}

			if strings.Contains(err.Error(), "does not exist") {
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

func testDashboardBasicConfig(rName string) string {
	return fmt.Sprintf(`
		provider "jupiterone" {}

		resource "jupiterone_dashboard" "test" {
			name = %q
			type = "Account"
		}
	`, rName)
}

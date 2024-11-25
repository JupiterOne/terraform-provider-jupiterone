package jupiterone

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

func TestIntegration_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	resourceName := "jupiterone_integration.test"
	integrationName := acctest.RandomWithPrefix("tf-acc-test")
	integrationDefinitionId := "8013680b-311a-4c2e-b53b-c8735fd97a5c"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckIntegrationDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testIntegrationBasicConfig(integrationName, integrationDefinitionId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckIntegrationExists(ctx, resourceName, directClient),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", integrationName),
					resource.TestCheckResourceAttr(resourceName, "integration_definition_id", integrationDefinitionId),
					resource.TestCheckResourceAttr(resourceName, "polling_interval", "ONE_DAY"),
					resource.TestCheckResourceAttr(resourceName, "description", "Test integration"),
					resource.TestCheckResourceAttr(resourceName, "config.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "config.key", "value"),
					resource.TestCheckResourceAttr(resourceName, "resource_group_id", "rg-123456"),
				),
			},
			// Add a second step to check if the resource is stable after refresh
			{
				Config:   testIntegrationBasicConfig(integrationName, integrationDefinitionId),
				PlanOnly: true,
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckIntegrationExists(ctx context.Context, resourceName string, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No Integration ID is set")
		}

		_, err := client.GetIntegrationInstance(ctx, qlient, rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("error fetching integration with resource name %s and id %s, %v", resourceName, rs.Primary.ID, err)
		}

		return nil
	}
}

func testAccCheckIntegrationDestroy(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "jupiterone_integration" {
				continue
			}

			time.Sleep(5 * time.Second)

			_, err := client.GetIntegrationInstance(ctx, qlient, rs.Primary.ID)
			if err == nil {
				return fmt.Errorf("Integration still exists")
			}

			if !strings.Contains(err.Error(), "does not exist") {
				return err
			}
		}

		return nil
	}
}

func testIntegrationBasicConfig(name, integrationDefinitionId string) string {
	return fmt.Sprintf(`
resource "jupiterone_integration" "test" {
  name                        = %q
  integration_definition_id   = %q
  polling_interval            = "ONE_DAY"
  description                 = "Test integration"
  config = {
    key = "value"
  }

  resource_group_id = "rg-123456"
}
`, name, integrationDefinitionId)
}

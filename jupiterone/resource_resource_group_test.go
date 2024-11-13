package jupiterone

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

func TestResourceGroup_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	resourceName := "jupiterone_resource_group.test_resource_group"
	resourceGroupName := "tf-provider-test-resource-group"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckResourceGroupDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testResourceGroupConfig(resourceGroupName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceGroupExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", resourceGroupName),
				),
			},
		},
	})
}

func testAccCheckResourceGroupExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if err := resourceGroupExistsHelper(ctx, s, qlient); err != nil {
			return err
		}
		return nil
	}
}

func resourceGroupExistsHelper(ctx context.Context, s *terraform.State, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	duration := 10 * time.Second
	for _, r := range s.RootModule().Resources {
		err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
			id := r.Primary.ID
			_, err := client.GetResourceGroup(ctx, qlient, id)

			if err == nil {
				return nil
			}

			if strings.Contains(err.Error(), "Item not found") {
				return retry.RetryableError(fmt.Errorf("Resource group does not exist (id=%q)", id))
			}

			return retry.NonRetryableError(err)
		})

		if err != nil {
			return err
		}
	}

	return nil

}

func testAccCheckResourceGroupDestroy(ctx context.Context, qlient graphql.Client) func(*terraform.State) error {
	return func(s *terraform.State) error {
		resource := s.RootModule().Resources[resourceName]

		if resource == nil {
			hclog.Default().Debug("No resource found for permission name", "resource_name", resourceName)
			return nil
		}
		hclog.Default().Debug("Attempting to delete resource for permission name", "resource_name", resourceName, "resource_id", resource.Primary.ID)
		if err := resourceGroupDestroyHelper(ctx, s, qlient); err != nil {
			return err
		}
		return nil
	}
}

func resourceGroupDestroyHelper(ctx context.Context, s *terraform.State, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	duration := 10 * time.Second
	for _, r := range s.RootModule().Resources {
		err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
			id := r.Primary.ID
			_, err := client.GetResourceGroup(ctx, qlient, id)

			if err == nil {
				return retry.RetryableError(fmt.Errorf("Resource group still exists (id=%q)", id))
			}

			if strings.Contains(err.Error(), "Item not found") {
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

func testResourceGroupConfig(name string) string {
	return fmt.Sprintf(`
		resource "jupiterone_resource_group" "test_resource_group" {
			name = %q
		}
	`, name)
}

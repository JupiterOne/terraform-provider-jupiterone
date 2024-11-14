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

var resourceName = "jupiterone_resource_permission.test"
var subjectId = "example-group-id"

func TestResourcePermission_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckResourcePermissionDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testResourcePermissionConfig(subjectId, "group", "rule", "*", "*", true, true, true, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourcePermissionExists(ctx, directClient),
					resource.TestCheckResourceAttr(resourceName, "subject_id", subjectId),
					resource.TestCheckResourceAttr(resourceName, "subject_type", "group"),
					resource.TestCheckResourceAttr(resourceName, "resource_area", "rule"),
					resource.TestCheckResourceAttr(resourceName, "resource_type", "*"),
					resource.TestCheckResourceAttr(resourceName, "resource_id", "*"),
					resource.TestCheckResourceAttr(resourceName, "can_create", "true"),
					resource.TestCheckResourceAttr(resourceName, "can_read", "true"),
					resource.TestCheckResourceAttr(resourceName, "can_update", "true"),
					resource.TestCheckResourceAttr(resourceName, "can_delete", "true"),
				),
			},
		},
	})
}

func testAccCheckResourcePermissionExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if err := resourcePermissionExistsHelper(ctx, qlient); err != nil {
			return err
		}
		return nil
	}
}

func resourcePermissionExistsHelper(ctx context.Context, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	duration := 10 * time.Second
	err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
		_, err := client.GetResourcePermissions(ctx, qlient, client.GetResourcePermissionsFilter{SubjectId: "example-group-id", SubjectType: "group", ResourceArea: "rule", ResourceType: "*", ResourceId: "*"}, "", 10)

		if err == nil {
			return nil
		}

		if strings.Contains(err.Error(), "Cannot fetch resource permission that does not exist") {
			return retry.RetryableError(fmt.Errorf("Resource permission does not exist"))
		}

		return retry.NonRetryableError(err)
	})

	if err != nil {
		return err
	}

	return nil
}

func testAccCheckResourcePermissionDestroy(ctx context.Context, qlient graphql.Client) func(*terraform.State) error {
	return func(s *terraform.State) error {
		resource := s.RootModule().Resources[resourceName]
		if resource == nil {
			hclog.Default().Debug("No resource found for permission name", "resource_name", resourceName)
			return nil
		}
		hclog.Default().Debug("Attempting to delete resource for permission name", "resource_name", resourceName, "resource_id", resource.Primary.ID)
		return resourcePermissionDestroyHelper(ctx, qlient)
	}
}

func resourcePermissionDestroyHelper(ctx context.Context, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	duration := 10 * time.Second
	err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
		_, err := client.GetResourcePermissions(ctx, qlient, client.GetResourcePermissionsFilter{SubjectId: "example-group-id", SubjectType: "group", ResourceArea: "rule", ResourceType: "*", ResourceId: "*"}, "", 10)

		if err == nil {
			return retry.RetryableError(fmt.Errorf("Permission set still exists"))
		}

		if strings.Contains(err.Error(), "Permission set does not exist.") {
			return nil
		}

		return retry.NonRetryableError(err)
	})

	if err != nil {
		return err
	}

	return nil
}

func testResourcePermissionConfig(subjectId, subjectType, resourceArea, resourceType, resourceID string, canCreate, canRead, canUpdate, canDelete bool) string {
	return fmt.Sprintf(`
		provider "jupiterone" {}


		resource "jupiterone_resource_permission" "test" {
			subject_id    = %q
			subject_type  = %q
			resource_area = %q
			resource_type = %q
			resource_id   = %q
			can_create    = %t
			can_read      = %t
			can_update    = %t
			can_delete    = %t
		}
	`, subjectId, subjectType, resourceArea, resourceType, resourceID, canCreate, canRead, canUpdate, canDelete)
}

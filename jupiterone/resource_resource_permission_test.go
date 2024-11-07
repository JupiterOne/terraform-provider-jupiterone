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

var resourceName = "jupiterone_resource_permission_test"
var subjectName = "example-group-id"

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
				Config: testResourcePermissionConfig(subjectName, "group", "rule", "*", "*", true, true, true, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourcePermissionExists(ctx, directClient),
					// resource.TestCheckResourceAttr(resourceName, "subject_id", subjectID),
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

	// groups, err := client.GetGroupsByName(ctx, qlient, "TF Test Group")
	// if err != nil {
	// 	return fmt.Errorf("error fetching groups: %v", err)
	// }

	// // Grab one group that has the same name as the given name
	// if len(groups.IamGetGroupList.Items) == 0 {
	// 	return fmt.Errorf("no group found with the exact given name")
	// }

	// var group *client.GetGroupsByNameIamGetGroupListIamGroupPageItemsIamGroup

	// for _, groupData := range groups.IamGetGroupList.Items {
	// 	if groupData.GroupName == "TF Test Group" {
	// 		group = &groupData
	// 		break
	// 	}
	// }

	// if group == nil {
	// 	return fmt.Errorf("no group found with the exact given name")
	// }

	duration := 10 * time.Second
	err2 := retry.RetryContext(ctx, duration, func() *retry.RetryError {
		_, err := client.GetResourcePermission(ctx, qlient, client.GetResourcePermissionsFilter{SubjectId: group.Id, SubjectType: "group", ResourceArea: "rule", ResourceType: "*", ResourceId: "*"}, "", 10)

		if err == nil {
			return nil
		}

		if strings.Contains(err.Error(), "Cannot fetch resource permission that does not exist") {
			return retry.RetryableError(fmt.Errorf("Resource permission does not exist"))
		}

		return retry.NonRetryableError(err)
	})

	if err2 != nil {
		return err2
	}

	return nil
}

func testAccCheckResourcePermissionDestroy(ctx context.Context, qlient graphql.Client) func(*terraform.State) error {
	return func(s *terraform.State) error {
		if err := resourcePermissionDestroyHelper(ctx, s, qlient); err != nil {
			return err
		}
		return nil
	}
}

func resourcePermissionDestroyHelper(ctx context.Context, s *terraform.State, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	// duration := 10 * time.Second
	for _, r := range s.RootModule().Resources {
		if r.Type != "jupiterone_resource_permission" {
			continue
		}
		// err := retry.RetryContext(ctx, duration, func() *retry.RetryError {

		// })

		// if err != nil {
		// 	return err
		// }
	}
	return nil
}

// func testAccCheckResourcePermissionDestroy(ctx context.Context, qlient graphql.Client) func(*terraform.State) error {
// 	return func(s *terraform.State) error {
// 		resource := s.RootModule().Resources[resourceName]
// 		if resource == nil {
// 			hclog.Default().Debug("No resource permission found", "resource_name", resourceName)
// 			return nil
// 		}
// 		hclog.Default().Debug("Attempting to delete resource")
// 		if err := resourcePermissionDestroyHelper(ctx, qlient); err != nil {
// 			return err
// 		}
// 		return nil
// 	}
// }

// func resourcePermissionDestroyHelper(ctx context.Context, qlient graphql.Client) error {
// 	if qlient == nil {
// 		return nil
// 	}

// 	duration := 10 * time.Second

// 	err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
// 		_, err := client.GetResourcePermission(ctx, qlient, client.GetResourcePermissionsFilter{SubjectId: subjectID, SubjectType: "group", ResourceArea: "rule", ResourceType: "*", ResourceId: "*"}, "", 10)

// 		if err == nil {
// 			return retry.RetryableError(fmt.Errorf("Resource permission still exists"))
// 		}

// 		if strings.Contains(err.Error(), "Cannot fetch resource permission that does not exist") {
// 			return nil
// 		}

// 		return retry.NonRetryableError(err)
// 	})

// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

func testResourcePermissionConfig(subjectName, subjectType, resourceArea, resourceType, resourceID string, canCreate, canRead, canUpdate, canDelete bool) string {
	return fmt.Sprintf(`
		provider "jupiterone" {}

		resource "jupiterone_user_group" "test_group" {
			name = %q
			description = "Test user group for Terraform resource permission provider test"
			permissions = []
		}

		resource "jupiterone_resource_permission" "jz_questions_permissions" {
			subject_id    = jupiterone_user_group.test_group.id
			subject_type  = %q
			resource_area = %q
			resource_type = %q
			resource_id   = %q
			can_create    = %t
			can_read      = %t
			can_update    = %t
			can_delete    = %t
		}
	`, subjectName, subjectType, resourceArea, resourceType, resourceID, canCreate, canRead, canUpdate, canDelete)
}

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

func TestUserGroup_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	resourceName := "jupiterone_user_group.test"
	userGroupName := "tf-provider-test-user-group"
	userGroupDescription := "description of " + userGroupName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckUserGroupDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testUserGroupBasicConfig(userGroupName, userGroupDescription),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserGroupExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", userGroupName),
					resource.TestCheckResourceAttr(resourceName, "description", userGroupDescription),
					resource.TestCheckResourceAttr(resourceName, "permissions.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "permissions.0", "readGraph"),
					resource.TestCheckResourceAttr(resourceName, "query_policy.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "query_policy.0._class.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "query_policy.0._class.0", "User"),
				),
			},
		},
	})
}

func TestUserGroup_BasicImport(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	resourceName := "jupiterone_user_group.test"
	userGroupName := "tf-provider-test-user-group-import"
	userGroupDescription := "description of " + userGroupName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		Steps: []resource.TestStep{
			{
				ImportState:   true,
				ResourceName:  resourceName,
				ImportStateId: createTestUserGroup(ctx, t, recordingClient, userGroupName, userGroupDescription),
				Config:        testUserGroupBasicConfig(userGroupName, userGroupDescription),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserGroupExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", userGroupName),
					resource.TestCheckResourceAttr(resourceName, "description", userGroupDescription),
				),
			},
		},
	})
}

// createTestUserGroup directly calls the client to create a user group directly
// for import or other tests. Because the id must be returned, this must
// called with the recorder client.
func createTestUserGroup(ctx context.Context, t *testing.T, qlient graphql.Client, name string, description string) string {
	r, err := client.CreateUserGroup(ctx, qlient, name, description, nil, nil)
	if err != nil {
		t.Log("error creating user group for import test", err)
		t.FailNow()
	}

	return r.CreateIamGroup.Id
}

func testAccCheckUserGroupExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if err := userGroupExistsHelper(ctx, s, qlient); err != nil {
			return err
		}
		return nil
	}
}

func userGroupExistsHelper(ctx context.Context, s *terraform.State, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	duration := 10 * time.Second
	for _, r := range s.RootModule().Resources {
		err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
			id := r.Primary.ID
			_, err := client.GetUserGroup(ctx, qlient, id)

			if err == nil {
				return nil
			}

			if err != nil && strings.Contains(err.Error(), "Group string does not exist") {
				return retry.RetryableError(fmt.Errorf("User group does not exist (id=%q)", id))
			}

			return retry.NonRetryableError(err)
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func testAccCheckUserGroupDestroy(ctx context.Context, qlient graphql.Client) func(*terraform.State) error {
	return func(s *terraform.State) error {
		if err := userGroupDestroyHelper(ctx, s, qlient); err != nil {
			return err
		}
		return nil
	}
}

func userGroupDestroyHelper(ctx context.Context, s *terraform.State, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	duration := 10 * time.Second
	for _, r := range s.RootModule().Resources {
		err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
			id := r.Primary.ID
			_, err := client.GetUserGroup(ctx, qlient, id)

			if err == nil {
				return retry.RetryableError(fmt.Errorf("User group still exists (id=%q)", id))
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

func testUserGroupBasicConfig(rName string, description string) string {
	return fmt.Sprintf(`
		provider "jupiterone" {}

		resource "jupiterone_user_group" "test" {
			name = %q
			description = %q
			permissions = ["readGraph"]
			query_policy = [{"_class": ["User"]}]
		}
	`, rName, description)
}

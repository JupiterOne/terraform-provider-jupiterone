package jupiterone

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

func TestUserGroupMembership_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	resourceName := "jupiterone_user_group_membership.test_user"
	userGroupName := "tf-provider-test-user-group"
	email := "test.user@jupiterone.com"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckUserGroupMembershipDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testUserGroupMembershipBasicConfig(userGroupName, email),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckUserGroupMembershipExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "email", email),
				),
			},
		},
	})
}

func testAccCheckUserGroupMembershipExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if err := userGroupMembershipExistsHelper(ctx, s, qlient); err != nil {
			return err
		}
		return nil
	}
}

func userGroupMembershipExistsHelper(ctx context.Context, s *terraform.State, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	duration := 10 * time.Second
	for _, r := range s.RootModule().Resources {
		if r.Type != "jupiterone_user_group_membership" {
			continue
		}
		err := retry.RetryContext(ctx, duration, func() *retry.RetryError {

			email, ok := r.Primary.Attributes["email"]
			if !ok {
				return retry.NonRetryableError(errors.New("no email in membership"))
			}
			groupId, ok2 := r.Primary.Attributes["group_id"]
			if !ok2 {
				return retry.NonRetryableError(errors.New("no group_id in membership"))
			}

			// Check to make sure user is part of the given group
			var usersResponse, _ = client.GetUsersByEmail(ctx, qlient, email)

			if len(usersResponse.IamGetUserList.Items) > 0 {
				var user = usersResponse.IamGetUserList.Items[0]

				for _, group := range user.UserGroups.Items {
					if group.Id == groupId {
						return nil
					}
				}
			}

			// If no user or user does not have a group, check to make sure the user does not have an pending invitations
			var invitations, _ = client.GetInvitations(ctx, qlient)

			for _, invite := range invitations.IamGetAccount.AccountInvitations.Items {
				if invite.Email == email && invite.GroupId == groupId && invite.Status != "REVOKED" {
					return nil
				}
			}

			return retry.RetryableError(fmt.Errorf("User group membership does not exist (email=%q, groupId=%q)", email, groupId))
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func testAccCheckUserGroupMembershipDestroy(ctx context.Context, qlient graphql.Client) func(*terraform.State) error {
	return func(s *terraform.State) error {
		if err := userGroupMembershipDestroyHelper(ctx, s, qlient); err != nil {
			return err
		}
		return nil
	}
}

func userGroupMembershipDestroyHelper(ctx context.Context, s *terraform.State, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	duration := 10 * time.Second
	for _, r := range s.RootModule().Resources {
		if r.Type != "jupiterone_user_group_membership" {
			continue
		}
		err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
			email, ok := r.Primary.Attributes["email"]
			if !ok {
				return retry.NonRetryableError(errors.New("no email in membership"))
			}
			groupId, ok2 := r.Primary.Attributes["group_id"]
			if !ok2 {
				return retry.NonRetryableError(errors.New("no group_id in membership"))
			}

			// Check to make sure user is not part of the given group
			var usersResponse, _ = client.GetUsersByEmail(ctx, qlient, email)

			if len(usersResponse.IamGetUserList.Items) > 0 {
				var user = usersResponse.IamGetUserList.Items[0]

				for _, group := range user.UserGroups.Items {
					if group.Id == groupId {
						return retry.RetryableError(fmt.Errorf("User group membership still exists (email=%q, groupId=%q)", email, groupId))
					}
				}
			}

			// Check to make sure the user does not have an pending invitations
			var invitations, _ = client.GetInvitations(ctx, qlient)

			for _, invite := range invitations.IamGetAccount.AccountInvitations.Items {
				if invite.Email == email && invite.GroupId == groupId && invite.Status != "REVOKED" {
					return retry.RetryableError(fmt.Errorf("Invite still exists (email=%q, groupId=%q)", email, groupId))
				}
			}

			return nil
		})

		if err != nil {
			return err
		}
	}
	return nil
}

func testUserGroupMembershipBasicConfig(userGroupName string, email string) string {
	return fmt.Sprintf(`
		provider "jupiterone" {}

		resource "jupiterone_user_group" "test_group" {
			name = %q
			description = "Test user group for Terraform provider membership test"
			permissions = []
		}

		resource "jupiterone_user_group_membership" "test_user" {
			group_id = jupiterone_user_group.test_group.id
			email = %q
		}
	`, userGroupName, email)
}

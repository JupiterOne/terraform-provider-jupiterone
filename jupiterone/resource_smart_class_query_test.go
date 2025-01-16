package jupiterone

import (
	"context"
	"fmt"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

func TestSmartClassQuery_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	resourceName := "jupiterone_smart_class_query.test"
	smartClassResourceName := "jupiterone_smart_class.test"
	_query := "FIND User"
	_description := "find users"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckSmartClassQueryDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testSmartClassQueryBasicConfig(_query, _description),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckSmartClassQueryExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrPair(resourceName, "smart_class_id", smartClassResourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "query", _query),
					resource.TestCheckResourceAttr(resourceName, "description", _description),
				),
			},
		},
	})
}

func testAccCheckSmartClassQueryExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if err := smartClassQueryExistsHelper(ctx, s, qlient); err != nil {
			return err
		}
		return nil
	}
}

func smartClassQueryExistsHelper(ctx context.Context, s *terraform.State, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	for _, r := range s.RootModule().Resources {
		if r.Type == "jupiterone_smart_class_query" {
			id := r.Primary.ID
			_, err := client.GetSmartClassQuery(ctx, qlient, id)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func testAccCheckSmartClassQueryDestroy(ctx context.Context, qlient graphql.Client) func(*terraform.State) error {
	return func(s *terraform.State) error {
		if err := smartClassQueryDestroyHelper(ctx, s, qlient); err != nil {
			return err
		}
		return nil
	}
}

func smartClassQueryDestroyHelper(ctx context.Context, s *terraform.State, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	err := smartClassQueryExistsHelper(ctx, s, qlient)

	if err == nil {
		return fmt.Errorf("Smart class query still exists")
	}

	return nil
}

func testSmartClassQueryBasicConfig(_query string, _description string) string {
	return fmt.Sprintf(`
		provider "jupiterone" {}

		resource "jupiterone_smart_class" "test" {
			tag_name = "TfProviderTagTest"
			description = "xyz"
		}

		resource "jupiterone_smart_class_query" "test" {
			smart_class_id = jupiterone_smart_class.test.id
			query = %q
			description = %q
		}
	`, _query, _description)
}

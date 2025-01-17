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

func TestSmartClassTag_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	resourceName := "jupiterone_smart_class_tag.test"
	smartClassResourceName := "jupiterone_smart_class.test"
	_name := "tagname"
	_type := "boolean"
	_value := "true"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckSmartClassTagDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testSmartClassTagBasicConfig(_name, _type, _value),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckSmartClassTagExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrPair(resourceName, "smart_class_id", smartClassResourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", _name),
					resource.TestCheckResourceAttr(resourceName, "type", _type),
					resource.TestCheckResourceAttr(resourceName, "value", _value),
				),
			},
		},
	})
}

func testAccCheckSmartClassTagExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if err := smartClassTagExistsHelper(ctx, s, qlient); err != nil {
			return err
		}
		return nil
	}
}

func smartClassTagExistsHelper(ctx context.Context, s *terraform.State, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	var smartClassTag client.GetSmartClassSmartClassTagsSmartClassTag
	for _, r := range s.RootModule().Resources {
		if r.Type == "jupiterone_smart_class" {
			id := r.Primary.ID
			smartClassResponse, err := client.GetSmartClass(ctx, qlient, id)
			if err != nil {
				return err
			}

			for _, tag := range smartClassResponse.SmartClass.Tags {
				if tag.Name == "tagname" {
					smartClassTag = tag
					break
				}
			}
		}
	}

	if smartClassTag.Name != "tagname" {
		return fmt.Errorf("Smart class tag does not exist")
	}

	return nil
}

func testAccCheckSmartClassTagDestroy(ctx context.Context, qlient graphql.Client) func(*terraform.State) error {
	return func(s *terraform.State) error {
		if err := smartClassTagDestroyHelper(ctx, s, qlient); err != nil {
			return err
		}
		return nil
	}
}

func smartClassTagDestroyHelper(ctx context.Context, s *terraform.State, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	err := smartClassTagExistsHelper(ctx, s, qlient)

	if err == nil {
		return fmt.Errorf("Smart class tag still exists")
	}

	return nil
}

func testSmartClassTagBasicConfig(_name string, _type string, _value string) string {
	return fmt.Sprintf(`
		provider "jupiterone" {}

		resource "jupiterone_smart_class" "test" {
			tag_name = "TfProviderTagTest"
			description = "xyz"
		}

		resource "jupiterone_smart_class_tag" "test" {
			smart_class_id = jupiterone_smart_class.test.id
			name = %q
			type = %q
			value = %q
		}
	`, _name, _type, _value)
}

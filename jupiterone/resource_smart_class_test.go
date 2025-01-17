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

func TestSmartClass_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	resourceName := "jupiterone_smart_class.test"
	smartClassTagName := "TfProviderTestSmartClass"
	smartClassDescription := "description of " + smartClassTagName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckSmartClassDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testSmartClassBasicConfig(smartClassTagName, smartClassDescription),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckSmartClassExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "tag_name", smartClassTagName),
					resource.TestCheckResourceAttr(resourceName, "description", smartClassDescription),
				),
			},
		},
	})
}

func TestSmartClass_BasicImport(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	resourceName := "jupiterone_smart_class.test"
	smartClassTagName := "TfProviderTestImport"
	smartClassDescription := "description of " + smartClassTagName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		Steps: []resource.TestStep{
			{
				ImportState:   true,
				ResourceName:  resourceName,
				ImportStateId: createTestSmartClass(ctx, t, recordingClient, smartClassTagName, smartClassDescription),
				Config:        testSmartClassBasicConfig(smartClassTagName, smartClassDescription),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckSmartClassExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "tag_name", smartClassTagName),
					resource.TestCheckResourceAttr(resourceName, "description", smartClassDescription),
				),
			},
		},
	})
}

func createTestSmartClass(ctx context.Context, t *testing.T, qlient graphql.Client, tagName string, description string) string {
	r, err := client.CreateSmartClass(ctx, qlient, client.CreateSmartClassInput{
		TagName:     tagName,
		Description: description,
	})
	if err != nil {
		t.Log("error creating smart class import test", err)
		t.FailNow()
	}

	return r.CreateSmartClass.Id
}

func testAccCheckSmartClassExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if err := smartClassExistsHelper(ctx, s, qlient); err != nil {
			return err
		}
		return nil
	}
}

func smartClassExistsHelper(ctx context.Context, s *terraform.State, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	duration := 10 * time.Second
	for _, r := range s.RootModule().Resources {
		err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
			id := r.Primary.ID
			_, err := client.GetSmartClass(ctx, qlient, id)

			if err == nil {
				return nil
			}

			if strings.Contains(err.Error(), "Smart class string does not exist") {
				return retry.RetryableError(fmt.Errorf("Smart class does not exist (id=%q)", id))
			}

			return retry.NonRetryableError(err)
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func testAccCheckSmartClassDestroy(ctx context.Context, qlient graphql.Client) func(*terraform.State) error {
	return func(s *terraform.State) error {
		if err := smartClassDestroyHelper(ctx, s, qlient); err != nil {
			return err
		}
		return nil
	}
}

func smartClassDestroyHelper(ctx context.Context, s *terraform.State, qlient graphql.Client) error {
	if qlient == nil {
		return nil
	}

	duration := 10 * time.Second
	for _, r := range s.RootModule().Resources {
		err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
			id := r.Primary.ID
			_, err := client.GetSmartClass(ctx, qlient, id)

			if err == nil {
				return retry.RetryableError(fmt.Errorf("Smart class still exists (id=%q)", id))
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

func testSmartClassBasicConfig(tagName string, description string) string {
	return fmt.Sprintf(`
		provider "jupiterone" {}

		resource "jupiterone_smart_class" "test" {
			tag_name = %q
			description = %q
		}
	`, tagName, description)
}

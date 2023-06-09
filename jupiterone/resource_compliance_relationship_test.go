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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRelationshipConfig(frameworkItemId, libraryItemId string, relationship client.LibraryItemToFrameworkItemRelationshipType) string {
	return fmt.Sprintf(`
	resource jupiterone_compliance_relationship tf_acc_link_test {
		framework_item_id = %q
		library_item_id = %q
		relationship_type = %q
	}
	`, frameworkItemId, libraryItemId, relationship)
}

func TestComplianceRelationship_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClients(ctx, t)
	defer cleanup(t)

	// Because the library item is independent of the of the framework and
	// child resources, terraform may perform the creations out of order, so
	// creating them all in Steps.Config elements can be unpredictable.
	// Necessary fixtures must be created before the tests and since the
	// generated UUIDs are necessary, this must use the recordingClient.
	frameworkId, frameworkItemId, libraryItemId, err := createTestRelationshipFixture(ctx, recordingClient)
	require.NoError(t, err, "error creating test resources for compliancerelationship, resources may need to manually removed")
	defer func() {
		err := deleteTestRelationshipFixture(ctx, recordingClient, frameworkId, libraryItemId)
		assert.NoError(t, err, "error removing test resources for compliancerelationship, resources may need to manually removed")
	}()

	testRelationshipResourceName := "jupiterone_compliance_relationship.tf_acc_link_test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckRelationshipDestroy(ctx, directClient, frameworkItemId, libraryItemId),
		Steps: []resource.TestStep{
			{
				Config: testRelationshipConfig(frameworkItemId, libraryItemId, client.LibraryItemToFrameworkItemRelationshipTypeInheritEvidence),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRelationshipExists(ctx, directClient, frameworkItemId, libraryItemId, string(client.LibraryItemToFrameworkItemRelationshipTypeInheritEvidence)),
					resource.TestCheckResourceAttrSet(testRelationshipResourceName, "id"),
					resource.TestCheckResourceAttr(testRelationshipResourceName, "framework_item_id", frameworkItemId),
					resource.TestCheckResourceAttr(testRelationshipResourceName, "library_item_id", libraryItemId),
					resource.TestCheckResourceAttr(testRelationshipResourceName, "relationship_type", string(client.LibraryItemToFrameworkItemRelationshipTypeInheritEvidence)),
				),
			},
			{
				Config: testRelationshipConfig(frameworkItemId, libraryItemId, client.LibraryItemToFrameworkItemRelationshipTypeIgnoreEvidence),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRelationshipExists(ctx, directClient, frameworkItemId, libraryItemId, string(client.LibraryItemToFrameworkItemRelationshipTypeIgnoreEvidence)),
					resource.TestCheckResourceAttrSet(testRelationshipResourceName, "id"),
					resource.TestCheckResourceAttr(testRelationshipResourceName, "framework_item_id", frameworkItemId),
					resource.TestCheckResourceAttr(testRelationshipResourceName, "library_item_id", libraryItemId),
					resource.TestCheckResourceAttr(testRelationshipResourceName, "relationship_type", string(client.LibraryItemToFrameworkItemRelationshipTypeIgnoreEvidence)),
				),
			},
		},
	})
}

func testAccCheckRelationshipExists(ctx context.Context, qlient graphql.Client, frameworkItemId, libraryItemId, relType string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		duration := 10 * time.Second
		for _, r := range s.RootModule().Resources {
			if r.Type != "jupiterone_compliancerelationship" {
				continue
			}
			err := retry.RetryContext(ctx, duration, func() *retry.RetryError {

				resp, err := client.GetComplianceFrameworkItemRelationshipsById(ctx, qlient, frameworkItemId)

				if err == nil {
					for _, l := range resp.ComplianceFrameworkItem.LibraryItems.InheritedEvidenceLibraryItems {
						if l.Id == libraryItemId {
							if relType == string(client.LibraryItemToFrameworkItemRelationshipTypeInheritEvidence) {
								return nil
							}
							return retry.RetryableError(fmt.Errorf("Relationship should be `Inherit` relationship for relationship (framwork_item_id=%q, library_item_id=%q)", frameworkItemId, libraryItemId))
						}
					}
					for _, l := range resp.ComplianceFrameworkItem.LibraryItems.IgnoredEvidenceLibraryItems {
						if l.Id == libraryItemId {
							if relType == string(client.LibraryItemToFrameworkItemRelationshipTypeIgnoreEvidence) {
								return nil
							}
							return retry.RetryableError(fmt.Errorf("Relationship should be `Ignore` relationship for relationship (framwork_item_id=%q, library_item_id=%q)", frameworkItemId, libraryItemId))
						}
					}
					return retry.RetryableError(fmt.Errorf("Relationship does not exist for fraemwork item (id=%q)", frameworkItemId))
				}

				if err != nil && strings.Contains(err.Error(), "Could not find compliance relationship for framework") {
					return retry.RetryableError(err)
				}

				return retry.NonRetryableError(err)
			})

			if err != nil {
				return err
			}
		}

		return nil
	}
}

func testAccCheckRelationshipDestroy(ctx context.Context, qlient graphql.Client, frameworkItemId, libraryItemId string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		duration := 10 * time.Second
		err := retry.RetryContext(ctx, duration, func() *retry.RetryError {

			resp, err := client.GetComplianceFrameworkItemRelationshipsById(ctx, qlient, frameworkItemId)

			if err == nil {
				for _, l := range resp.ComplianceFrameworkItem.LibraryItems.InheritedEvidenceLibraryItems {
					if l.Id == libraryItemId {
						return retry.RetryableError(fmt.Errorf("Relationships still exists for framework item (id=%q)", frameworkItemId))
					}
				}
				for _, l := range resp.ComplianceFrameworkItem.LibraryItems.IgnoredEvidenceLibraryItems {
					if l.Id == libraryItemId {
						return retry.RetryableError(fmt.Errorf("Relationships still exists for framework item (id=%q)", frameworkItemId))
					}
				}
				return nil
			}

			if err != nil && strings.Contains(err.Error(), "Could not find") {
				return nil
			}

			return retry.NonRetryableError(err)
		})

		if err != nil {
			return err
		}

		return nil
	}
}

func deleteTestRelationshipFixture(ctx context.Context, qlient graphql.Client, frameworkId, libraryItemId string) error {
	_, err := client.DeleteComplianceFramework(ctx, qlient, client.DeleteComplianceFrameworkInput{Id: frameworkId})
	if err != nil {
		return err

	}

	_, err = client.DeleteComplianceLibraryItem(ctx, qlient, libraryItemId)
	return err
}

func createTestRelationshipFixture(ctx context.Context, qlient graphql.Client) (frameworkId, frameworkItemId, libraryItemId string, err error) {
	var f *client.CreateComplianceFrameworkResponse
	f, err = client.CreateComplianceFramework(ctx, qlient, client.CreateComplianceFrameworkInput{
		Name:          "tf-acc-test-framework",
		Version:       "1",
		FrameworkType: client.ComplianceFrameworkTypeStandard,
	})
	if err != nil {
		return
	}
	frameworkId = f.CreateComplianceFramework.Id

	var g *client.CreateComplianceGroupResponse
	g, err = client.CreateComplianceGroup(ctx, qlient, client.CreateComplianceGroupInput{
		Name:        "tf-acc-test-group",
		FrameworkId: frameworkId,
	})
	if err != nil {
		return
	}

	var i *client.CreateComplianceFrameworkItemResponse
	i, err = client.CreateComplianceFrameworkItem(ctx, qlient, client.CreateComplianceFrameworkItemInput{
		Name:        "tf-acc-test-item",
		FrameworkId: frameworkId,
		GroupId:     g.CreateComplianceGroup.Id,
	})
	if err != nil {
		return
	}
	frameworkItemId = i.CreateComplianceFrameworkItem.Id

	var l *client.CreateComplianceLibraryItemResponse
	l, err = client.CreateComplianceLibraryItem(ctx, qlient, client.CreateComplianceLibraryItemInput{
		Name: "tf-acc-test-control",
	})
	if err != nil {
		return
	}
	libraryItemId = l.CreateComplianceLibraryItem.Id

	return frameworkId, frameworkItemId, libraryItemId, nil
}

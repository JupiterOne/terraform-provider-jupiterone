package jupiterone

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

const (
	asCodeSharedControl   = "jupiterone_control.shared"
	asCodeUnmanagedLinks  = "jupiterone_control.unmanaged_links"
	asCodeSharedTest      = "jupiterone_control_test.on_shared"
	asCodeUnmanagedTest   = "jupiterone_control_test.on_unmanaged_links"
	asCodeRequirementOne  = "jupiterone_control_framework_requirement.one"
	asCodeRequirementTwo  = "jupiterone_control_framework_requirement.two"
	asCodeUnmanagedLinked = "linked outside terraform"
)

// TestControl_ComplianceAsCode covers the compliance-as-code workflow: one control
// satisfying several requirements, one query reused by several controls, links
// made outside Terraform surviving refresh and update, and import round-trips.
func TestControl_ComplianceAsCode(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClientsWithReplaySupport(ctx, t)
	defer cleanup(t)

	ids := map[string]string{}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckControlDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testComplianceAsCodeConfig("unmanaged links control"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(asCodeSharedControl, "source_id"),
					resource.TestCheckResourceAttr(asCodeSharedControl, "requirement_ids.#", "2"),
					resource.TestCheckResourceAttrPair(asCodeSharedControl, "requirement_ids.0", asCodeRequirementOne, "id"),
					resource.TestCheckResourceAttrPair(asCodeSharedControl, "requirement_ids.1", asCodeRequirementTwo, "id"),
					resource.TestCheckResourceAttr(asCodeSharedControl, "state", "LIVE"),
					resource.TestCheckResourceAttr(asCodeUnmanagedLinks, "requirement_ids.#", "0"),
					resource.TestCheckResourceAttrPair(asCodeSharedTest, "control_id", asCodeSharedControl, "id"),
					resource.TestCheckResourceAttrPair(asCodeUnmanagedTest, "control_id", asCodeUnmanagedLinks, "id"),
					resource.TestCheckResourceAttrPair(asCodeSharedTest, "query", asCodeUnmanagedTest, "query"),
					captureResourceId(ids, asCodeUnmanagedLinks),
					captureResourceId(ids, asCodeRequirementOne),
				),
			},
			{
				// Simulates a framework import or UI edit linking the control.
				PreConfig: func() {
					current, err := client.GetControlById(ctx, directClient, ids[asCodeUnmanagedLinks])
					if err != nil {
						t.Fatalf("failed to read control before linking: %v", err)
					}
					_, err = client.UpdateControl(ctx, directClient, client.UpdateControlInput{
						Id:             ids[asCodeUnmanagedLinks],
						Name:           current.Control.Name,
						Description:    current.Control.Description,
						RequirementIds: &client.ListUpdateInput{Add: []string{ids[asCodeRequirementOne]}},
					})
					if err != nil {
						t.Fatalf("failed to link requirement outside terraform: %v", err)
					}
				},
				Config: testComplianceAsCodeConfig("unmanaged links control"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(asCodeUnmanagedLinks, "requirement_ids.#", "1"),
					resource.TestCheckResourceAttrPair(asCodeUnmanagedLinks, "requirement_ids.0", asCodeRequirementOne, "id"),
				),
			},
			{
				Config: testComplianceAsCodeConfig(asCodeUnmanagedLinked),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(asCodeUnmanagedLinks, "description", asCodeUnmanagedLinked),
					resource.TestCheckResourceAttr(asCodeUnmanagedLinks, "requirement_ids.#", "1"),
					testAccCheckControlLinkedTo(ctx, directClient, ids, asCodeUnmanagedLinks, asCodeRequirementOne),
				),
			},
			{
				ResourceName:      asCodeSharedControl,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:            asCodeSharedTest,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"query_name"},
			},
			{
				ResourceName:      asCodeRequirementOne,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func captureResourceId(ids map[string]string, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}
		ids[name] = rs.Primary.ID
		return nil
	}
}

func testAccCheckControlLinkedTo(ctx context.Context, qlient graphql.Client, ids map[string]string, control, requirement string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		if qlient == nil {
			return nil
		}

		result, err := client.GetControlById(ctx, qlient, ids[control])
		if err != nil {
			return err
		}

		for _, item := range result.ControlRequirements.Items {
			if item.Id == ids[requirement] {
				return nil
			}
		}

		linked := make([]string, 0, len(result.ControlRequirements.Items))
		for _, item := range result.ControlRequirements.Items {
			linked = append(linked, item.Id)
		}
		sort.Strings(linked)
		return fmt.Errorf("control %s lost its link to requirement %s; linked: %v", ids[control], ids[requirement], linked)
	}
}

// Resources are chained with depends_on because cassettes replay by request order.
func testComplianceAsCodeConfig(unmanagedDescription string) string {
	return fmt.Sprintf(`
provider "jupiterone" {}

locals {
  mfa_query = "FIND User WITH mfaEnabled != true"
}

resource "jupiterone_control_framework" "as_code" {
  name        = "tf-provider-acc-test-as-code-framework"
  description = "compliance as code acceptance test"
}

resource "jupiterone_control_framework_requirement" "one" {
  title        = "tf-provider-acc-test-as-code-req-one"
  framework_id = jupiterone_control_framework.as_code.id
}

resource "jupiterone_control_framework_requirement" "two" {
  title        = "tf-provider-acc-test-as-code-req-two"
  framework_id = jupiterone_control_framework.as_code.id
  depends_on   = [jupiterone_control_framework_requirement.one]
}

resource "jupiterone_control" "shared" {
  name  = "tf-provider-acc-test-as-code-shared"
  state = "LIVE"
  requirement_ids = [
    jupiterone_control_framework_requirement.one.id,
    jupiterone_control_framework_requirement.two.id,
  ]
}

resource "jupiterone_control" "unmanaged_links" {
  name        = "tf-provider-acc-test-as-code-unmanaged-links"
  description = %q
  depends_on  = [jupiterone_control.shared]
}

resource "jupiterone_control_test" "on_shared" {
  name        = "tf-provider-acc-test-as-code-mfa-shared"
  control_id  = jupiterone_control.shared.id
  query       = local.mfa_query
  results_are = "BAD"
  depends_on  = [jupiterone_control.unmanaged_links]
}

resource "jupiterone_control_test" "on_unmanaged_links" {
  name        = "tf-provider-acc-test-as-code-mfa-unmanaged"
  control_id  = jupiterone_control.unmanaged_links.id
  query       = local.mfa_query
  results_are = "BAD"
  depends_on  = [jupiterone_control_test.on_shared]
}
`, unmanagedDescription)
}

package jupiterone

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestControl_MitreTechnique covers the three transitions that matter for a
// nullable, server-validated field: setting it, changing it, and clearing it.
// Clearing is the interesting one, because it has to send an explicit null;
// an empty string fails the server's MITRE format validation and omitting the
// field would silently preserve the old value.
func TestControl_MitreTechnique(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClientsWithReplaySupport(ctx, t)
	defer cleanup(t)

	resourceName := "jupiterone_control.mitre"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckControlDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testControlMitreConfig(`mitre_technique = "T1078"`),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckControlExists(ctx, directClient),
					resource.TestCheckResourceAttr(resourceName, "mitre_technique", "T1078"),
				),
			},
			{
				// A sub-technique, exercising the optional .### suffix.
				Config: testControlMitreConfig(`mitre_technique = "T1059.001"`),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckControlExists(ctx, directClient),
					resource.TestCheckResourceAttr(resourceName, "mitre_technique", "T1059.001"),
				),
			},
			{
				// Removing the attribute must clear it rather than leave
				// T1059.001 in place.
				Config: testControlMitreConfig(``),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckControlExists(ctx, directClient),
					resource.TestCheckNoResourceAttr(resourceName, "mitre_technique"),
				),
			},
		},
	})
}

func testControlMitreConfig(mitre string) string {
	return fmt.Sprintf(`
	provider "jupiterone" {}

	resource "jupiterone_control" "mitre" {
		name        = "tf-provider-acc-test-control-mitre"
		description = "acceptance test control with a MITRE technique"
		owner       = "test-owner@jupiterone.com"
		state       = "DRAFT"
		%s
	}
	`, mitre)
}

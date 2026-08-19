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

const testAttestationResourceName = "jupiterone_control_attestation.test"

// A fixed, far-future expiry keeps recorded cassettes replayable. Deriving it
// from time.Now() would bake a moving value into the request bodies.
const testAttestationExpiry = "2030-01-31T00:00:00Z"
const testAttestationUpdatedExpiry = "2031-06-30T12:00:00Z"

func TestControlAttestation_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClientsWithReplaySupport(ctx, t)
	defer cleanup(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckAttestationRevoked(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testAttestationConfig("SOC 2 vendor report reviewed", testAttestationExpiry),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAttestationExists(ctx, directClient),
					resource.TestCheckResourceAttrSet(testAttestationResourceName, "id"),
					resource.TestCheckResourceAttrSet(testAttestationResourceName, "control_id"),
					resource.TestCheckResourceAttr(testAttestationResourceName, "subject", "SOC 2 vendor report reviewed"),
					resource.TestCheckResourceAttr(testAttestationResourceName, "expires_on", testAttestationExpiry),
					resource.TestCheckResourceAttr(testAttestationResourceName, "owner", "test-owner@jupiterone.com"),
					resource.TestCheckResourceAttr(testAttestationResourceName, "document_link", "https://example.com/soc2.pdf"),
					// Set from the request context, never sent by the provider.
					resource.TestCheckResourceAttrSet(testAttestationResourceName, "author"),
				),
			},
			{
				Config: testAttestationConfig("SOC 2 vendor report re-reviewed", testAttestationUpdatedExpiry),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAttestationExists(ctx, directClient),
					resource.TestCheckResourceAttr(testAttestationResourceName, "subject", "SOC 2 vendor report re-reviewed"),
					resource.TestCheckResourceAttr(testAttestationResourceName, "expires_on", testAttestationUpdatedExpiry),
				),
			},
		},
	})
}

// findAttestation locates an attestation through the only available read path:
// listing the owning control's attestations and matching by id.
func findAttestation(ctx context.Context, qlient graphql.Client, controlId, id string) (*client.GetAttestationsByControlIdAttestationsAttestation, error) {
	result, err := client.GetAttestationsByControlId(ctx, qlient, controlId)
	if err != nil {
		return nil, err
	}

	for i := range result.Attestations {
		if result.Attestations[i].Id == id {
			return &result.Attestations[i], nil
		}
	}

	return nil, nil
}

func testAccCheckAttestationExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		duration := 10 * time.Second
		for _, r := range s.RootModule().Resources {
			if r.Type != "jupiterone_control_attestation" {
				continue
			}

			controlId := r.Primary.Attributes["control_id"]
			id := r.Primary.ID

			err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
				found, err := findAttestation(ctx, qlient, controlId, id)
				if err != nil {
					if strings.Contains(err.Error(), "Could not find") || strings.Contains(err.Error(), "not found") {
						return retry.RetryableError(fmt.Errorf("attestation does not exist (id=%q)", id))
					}
					return retry.NonRetryableError(err)
				}

				if found == nil {
					return retry.RetryableError(fmt.Errorf("attestation does not exist (id=%q)", id))
				}

				if found.Revoked {
					return retry.NonRetryableError(fmt.Errorf("attestation is revoked (id=%q)", id))
				}

				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	}
}

// testAccCheckAttestationRevoked verifies destroy revoked the attestation.
// Revocation is a soft delete, so the record persists and the assertion is that
// it reads as revoked rather than that it has disappeared.
func testAccCheckAttestationRevoked(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}

		duration := 10 * time.Second
		for _, r := range s.RootModule().Resources {
			if r.Type != "jupiterone_control_attestation" {
				continue
			}

			controlId := r.Primary.Attributes["control_id"]
			id := r.Primary.ID

			err := retry.RetryContext(ctx, duration, func() *retry.RetryError {
				found, err := findAttestation(ctx, qlient, controlId, id)
				if err != nil {
					// The owning control is destroyed in the same run, so its
					// attestations become unreachable. That is a pass.
					if strings.Contains(err.Error(), "Could not find") || strings.Contains(err.Error(), "not found") {
						return nil
					}
					return retry.NonRetryableError(err)
				}

				if found == nil || found.Revoked {
					return nil
				}

				return retry.RetryableError(fmt.Errorf("attestation still active (id=%q)", id))
			})

			if err != nil {
				return err
			}
		}

		return nil
	}
}

func testAttestationConfig(subject, expiresOn string) string {
	return fmt.Sprintf(`
	provider "jupiterone" {}

	# Attestations are rejected against a non-LIVE control, because compliance
	# status is a LIVE-only concept.
	resource "jupiterone_control" "attested" {
		name  = "tf-provider-acc-test-control-for-attestation"
		owner = "test-owner@jupiterone.com"
		state = "LIVE"
	}

	resource "jupiterone_control_attestation" "test" {
		control_id    = jupiterone_control.attested.id
		subject       = %q
		description   = "acceptance test attestation"
		expires_on    = %q
		owner         = "test-owner@jupiterone.com"
		document_link = "https://example.com/soc2.pdf"
	}
	`, subject, expiresOn)
}

package jupiterone

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestCCMDataSources_Basic builds a small framework, requirement, control,
// control test and attestation, then reads all of it back through the data
// sources in one pass. Doing it in a single test keeps the fixtures and the
// cassette to one set.
//
// Assertions stay on values the provider controls. Evaluated fields such as
// status are not asserted, because a freshly created control has not been
// evaluated yet and the result would depend on the evaluation cadence.
func TestCCMDataSources_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClientsWithReplaySupport(ctx, t)
	defer cleanup(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckControlDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			{
				Config: testCCMDataSourcesConfig(),
				Check: resource.ComposeTestCheckFunc(
					// jupiterone_control, looked up by id.
					resource.TestCheckResourceAttrPair(
						"data.jupiterone_control.by_id", "id",
						"jupiterone_control.ds", "id"),
					resource.TestCheckResourceAttr("data.jupiterone_control.by_id", "name",
						"tf-provider-acc-test-control-datasource"),
					resource.TestCheckResourceAttr("data.jupiterone_control.by_id", "mitre_technique", "T1078"),
					resource.TestCheckResourceAttr("data.jupiterone_control.by_id", "state", "LIVE"),
					resource.TestCheckResourceAttr("data.jupiterone_control.by_id", "number_of_tests", "1"),
					resource.TestCheckResourceAttr("data.jupiterone_control.by_id", "has_valid_attestation", "true"),
					resource.TestCheckResourceAttr("data.jupiterone_control.by_id", "framework_ids.#", "1"),

					// jupiterone_controls, filtered to the framework created here
					// so the result set is deterministic.
					resource.TestCheckResourceAttr("data.jupiterone_controls.in_framework", "controls.#", "1"),
					resource.TestCheckResourceAttr("data.jupiterone_controls.in_framework", "controls.0.name",
						"tf-provider-acc-test-control-datasource"),
					resource.TestCheckResourceAttr("data.jupiterone_controls.in_framework", "controls.0.mitre_technique", "T1078"),

					// jupiterone_control_tests, scoped to the control.
					resource.TestCheckResourceAttr("data.jupiterone_control_tests.for_control", "control_tests.#", "1"),
					resource.TestCheckResourceAttr("data.jupiterone_control_tests.for_control", "control_tests.0.queries.#", "1"),
					resource.TestCheckResourceAttrSet("data.jupiterone_control_tests.for_control", "control_tests.0.referenced_rule_id"),

					// jupiterone_control_framework_stats for the new framework.
					resource.TestCheckResourceAttr("data.jupiterone_control_framework_stats.scorecard", "stats.#", "1"),
					resource.TestCheckResourceAttr("data.jupiterone_control_framework_stats.scorecard", "stats.0.number_of_requirements", "1"),
					resource.TestCheckResourceAttrSet("data.jupiterone_control_framework_stats.scorecard", "stats.0.number_of_controls"),

					// jupiterone_attestations, scoped to the control.
					resource.TestCheckResourceAttr("data.jupiterone_attestations.for_control", "attestations.#", "1"),
					resource.TestCheckResourceAttr("data.jupiterone_attestations.for_control", "attestations.0.subject",
						"data source acceptance attestation"),
					resource.TestCheckResourceAttr("data.jupiterone_attestations.for_control", "attestations.0.expires_on",
						testAttestationExpiry),
					resource.TestCheckResourceAttr("data.jupiterone_attestations.for_control", "attestations.0.state", "ACTIVE"),
					resource.TestCheckResourceAttr("data.jupiterone_attestations.for_control", "attestations.0.revoked", "false"),
				),
			},
		},
	})
}

func testCCMDataSourcesConfig() string {
	return `
	provider "jupiterone" {}

	resource "jupiterone_control_framework" "ds" {
		name        = "tf-provider-acc-test-framework-datasource"
		description = "acceptance test framework for data sources"
		owner       = "test-owner@jupiterone.com"
	}

	resource "jupiterone_control_framework_requirement" "ds" {
		framework_id = jupiterone_control_framework.ds.id
		title        = "tf-provider-acc-test-requirement-datasource"
		description  = "acceptance test requirement for data sources"
		priority     = "HIGH"
		section      = "1"
	}

	# LIVE, because an attestation cannot be created against a non-LIVE control.
	resource "jupiterone_control" "ds" {
		name            = "tf-provider-acc-test-control-datasource"
		description     = "acceptance test control for data sources"
		owner           = "test-owner@jupiterone.com"
		state           = "LIVE"
		mitre_technique = "T1078"
		requirement_ids = [jupiterone_control_framework_requirement.ds.id]
	}

	resource "jupiterone_control_test" "ds" {
		name        = "tf-provider-acc-test-control-test-datasource"
		control_id  = jupiterone_control.ds.id
		query       = "FIND User WITH mfaEnabled = false"
		query_name  = "users without mfa"
		results_are = "BAD"
	}

	# Chained after the control test so the two writes cannot race.
	resource "jupiterone_control_attestation" "ds" {
		control_id = jupiterone_control.ds.id
		subject    = "data source acceptance attestation"
		expires_on = "2030-01-31T00:00:00Z"
		owner      = "test-owner@jupiterone.com"

		depends_on = [jupiterone_control_test.ds]
	}

	# The data sources are chained deliberately. Terraform would otherwise read
	# independent data sources in parallel, and the cassette matcher pairs
	# requests to recorded interactions by order rather than by body, so a
	# parallel read replays against the wrong response. Chaining makes the
	# request order deterministic.
	data "jupiterone_control" "by_id" {
		id = jupiterone_control.ds.id

		depends_on = [jupiterone_control_attestation.ds]
	}

	data "jupiterone_controls" "in_framework" {
		framework_id = [jupiterone_control_framework.ds.id]

		depends_on = [data.jupiterone_control.by_id]
	}

	data "jupiterone_control_tests" "for_control" {
		control_id = jupiterone_control.ds.id

		depends_on = [data.jupiterone_controls.in_framework]
	}

	data "jupiterone_control_framework_stats" "scorecard" {
		framework_ids = [jupiterone_control_framework.ds.id]

		depends_on = [data.jupiterone_control_tests.for_control]
	}

	# jupiterone_control_frameworks is deliberately NOT exercised here. The
	# underlying controlFrameworks query supports only cursor and includeDeleted,
	# so it always returns every framework in the account together with all their
	# requirements. Recording that produced an 8 MB cassette containing internal
	# framework content and colleagues' email addresses, which has no business in
	# a public repository. The data source is covered by unit tests instead.
	data "jupiterone_attestations" "for_control" {
		control_id = jupiterone_control.ds.id

		depends_on = [data.jupiterone_control_framework_stats.scorecard]
	}
	`
}

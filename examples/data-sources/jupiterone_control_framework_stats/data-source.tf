# The scorecard numbers behind the framework compliance view.
data "jupiterone_control_framework_stats" "soc2" {
  framework_ids = [jupiterone_control_framework.soc2.id]
}

locals {
  soc2_stats = one(data.jupiterone_control_framework_stats.soc2.stats)
}

output "soc2_posture" {
  value = {
    passing = local.soc2_stats.number_of_passing_controls
    failing = local.soc2_stats.number_of_failing_controls

    # A subset of the passing count: controls passing on a signed attestation
    # rather than on a test result.
    attested = local.soc2_stats.number_of_attested_controls

    # Controls with no test attached are not being evaluated at all.
    untested = local.soc2_stats.number_of_controls - local.soc2_stats.number_of_controls_with_tests

    failing_critical_requirements = local.soc2_stats.number_of_failing_critical_requirements
  }
}

# Omitting framework_ids reports on every framework.
data "jupiterone_control_framework_stats" "everything" {}

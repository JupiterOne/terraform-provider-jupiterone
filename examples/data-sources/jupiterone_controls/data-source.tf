# Every filter is optional. With none set, all controls are returned.
data "jupiterone_controls" "failing_live" {
  states = ["LIVE"]
  status = ["FAIL"]
}

# Controls nobody is evaluating: live, but with no control tests attached.
data "jupiterone_controls" "unevaluated" {
  states        = ["LIVE"]
  effectiveness = "NO_TESTS"
}

# Passing only because somebody signed off, rather than because a test proves it.
data "jupiterone_controls" "attested_only" {
  states                = ["LIVE"]
  has_valid_attestation = true
}

# MITRE matching is by technique family: T1059 also returns controls tagged
# T1059.001, and T1059.001 also returns controls tagged T1059.
data "jupiterone_controls" "credential_access" {
  mitre_techniques = ["T1078", "T1110"]
}

output "failing_control_names" {
  value = [for c in data.jupiterone_controls.failing_live.controls : c.name]
}

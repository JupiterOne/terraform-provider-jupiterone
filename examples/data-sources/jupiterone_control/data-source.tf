# Managed control resources carry only declarative configuration. Evaluation
# results are read through data sources, so refreshing a control resource never
# churns on a status change.
data "jupiterone_control" "mfa" {
  id = jupiterone_control.mfa.id
}

# A control can also be looked up by its identifier in its source, which is how
# to reach controls that came from a catalog rather than from Terraform.
data "jupiterone_control" "from_catalog" {
  source_id = "cis-v8-5.3"
}

output "mfa_compliance" {
  value = {
    status = data.jupiterone_control.mfa.status

    # NO_TESTS means nothing is evaluating the control, which is a different
    # problem from the control failing.
    effectiveness = data.jupiterone_control.mfa.effective_status

    # Independent of status: a failing test is never masked by an attestation.
    attested = data.jupiterone_control.mfa.has_valid_attestation
  }
}

# Frameworks together with their requirements.
#
# This is how to attach Terraform-managed controls to a framework the provider
# does not manage, such as one imported from the J1 or UCF catalog: read the
# requirement IDs here rather than hard-coding them.
data "jupiterone_control_frameworks" "all" {}

locals {
  soc2 = one([
    for f in data.jupiterone_control_frameworks.all.control_frameworks : f
    if f.name == "SOC 2"
  ])

  # Requirement IDs keyed by their identifier, e.g. "CC6.1".
  soc2_requirements = {
    for r in local.soc2.requirements : r.identifier => r.id if r.identifier != null
  }
}

resource "jupiterone_control" "mfa_for_soc2" {
  name            = "MFA enforced on all human identities"
  owner           = "security-team@example.com"
  state           = "LIVE"
  requirement_ids = [local.soc2_requirements["CC6.1"]]
}

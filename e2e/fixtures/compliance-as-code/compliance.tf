terraform {
  required_providers {
    jupiterone = {
      source = "JupiterOne/jupiterone"
    }
  }
}

provider "jupiterone" {}

variable "run_id" {
  type = string
}

locals {
  prefix    = "tfe2e-${var.run_id}"
  mfa_query = "FIND User WITH mfaEnabled != true"
}

resource "jupiterone_control_framework" "managed" {
  name        = "${local.prefix} managed framework"
  description = "e2e: framework managed by terraform"
}

resource "jupiterone_control_framework_requirement" "access" {
  title        = "${local.prefix} access requirement"
  framework_id = jupiterone_control_framework.managed.id
}

resource "jupiterone_control_framework_requirement" "identity" {
  title        = "${local.prefix} identity requirement"
  framework_id = jupiterone_control_framework.managed.id
}

resource "jupiterone_control" "mfa" {
  name  = "${local.prefix} MFA enforced"
  state = "LIVE"
  requirement_ids = [
    jupiterone_control_framework_requirement.access.id,
    jupiterone_control_framework_requirement.identity.id,
  ]
}

resource "jupiterone_control" "least_privilege" {
  name  = "${local.prefix} least privilege"
  state = "LIVE"
}

resource "jupiterone_control_test" "mfa" {
  for_each = {
    mfa             = jupiterone_control.mfa.id
    least_privilege = jupiterone_control.least_privilege.id
  }

  name        = "${local.prefix} users without MFA (${each.key})"
  control_id  = each.value
  query       = local.mfa_query
  results_are = "BAD"
}

output "mfa_control_id" {
  value = jupiterone_control.mfa.id
}

output "reused_control_id" {
  value = jupiterone_control.least_privilege.id
}

output "reused_control_source_id" {
  value = jupiterone_control.least_privilege.source_id
}

resource "jupiterone_control_framework_requirement" "example" {
  framework_id = jupiterone_control_framework.example.id
  title        = "Access Control Policy"
  identifier   = "AC-1"
  priority     = "HIGH"
}

resource "jupiterone_control" "example" {
  name              = "Role-Based Access Control Implementation"
  description       = "Enforces RBAC across all systems to ensure least-privilege access"
  owner             = "security-team@example.com"
  state             = "LIVE"
  identifier        = "CTRL-AC-1"
  catalog           = "Internal Controls"
  remediation       = "Review and update IAM policies to enforce RBAC."
  exception_process = "Exceptions require CISO approval and must be reviewed quarterly."
  requirement_ids   = [jupiterone_control_framework_requirement.example.id]

  # The MITRE ATT&CK technique this control addresses, either a technique
  # (T1078) or a sub-technique (T1078.004). Filtering matches the whole family,
  # so a control tagged T1078.004 is also returned when filtering on T1078.
  mitre_technique = "T1078"
}

# Compliance status is not an attribute of the managed resource, because it is
# an evaluation result rather than desired state. Read it through the data
# source instead.
data "jupiterone_control" "example" {
  id = jupiterone_control.example.id
}

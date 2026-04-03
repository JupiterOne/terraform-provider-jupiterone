resource "jupiterone_control_framework_requirement" "example" {
  framework_id = jupiterone_control_framework.example.id
  title        = "Access Control Policy"
  identifier   = "AC-1"
  priority     = "HIGH"
}

resource "jupiterone_control" "example" {
  name            = "Role-Based Access Control Implementation"
  description     = "Enforces RBAC across all systems to ensure least-privilege access"
  owner           = "security-team@example.com"
  state           = "LIVE"
  identifier      = "CTRL-AC-1"
  catalog         = "Internal Controls"
  remediation     = "Review and update IAM policies to enforce RBAC."
  exception_process = "Exceptions require CISO approval and must be reviewed quarterly."
  requirement_ids = [jupiterone_control_framework_requirement.example.id]
}

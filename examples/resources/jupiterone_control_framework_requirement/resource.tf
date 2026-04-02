resource "jupiterone_control_framework" "example" {
  name  = "My Custom Control Framework"
  owner = "security@example.com"
}

resource "jupiterone_control_framework_requirement" "example" {
  framework_id = jupiterone_control_framework.example.id
  title        = "Access Control Policy"
  description  = "All systems must enforce role-based access control"
  identifier   = "AC-1"
  priority     = "HIGH"
  section      = "Access Control"
}

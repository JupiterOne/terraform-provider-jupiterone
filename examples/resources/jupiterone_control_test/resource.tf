resource "jupiterone_control" "example" {
  name  = "Role-Based Access Control Implementation"
  owner = "security-team@example.com"
  state = "LIVE"
}

resource "jupiterone_control_test" "example" {
  name        = "RBAC Coverage Test"
  control_id  = jupiterone_control.example.id
  description = "Verifies that all users have role assignments"
  query       = "FIND User THAT HAS Role"
  results_are = "GOOD"
}

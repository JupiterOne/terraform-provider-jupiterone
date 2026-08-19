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

# A control test evaluates exactly one query. To evaluate several conditions
# against the same control, attach several control tests to it.
resource "jupiterone_control_test" "example_orphaned_roles" {
  name        = "Orphaned Role Test"
  control_id  = jupiterone_control.example.id
  description = "Flags roles with no users assigned"

  # query_name names the query itself, which defaults to the name of the
  # control test when left unset.
  query_name  = "roles without users"
  query       = "FIND Role THAT !ASSIGNED User"
  results_are = "BAD"
}


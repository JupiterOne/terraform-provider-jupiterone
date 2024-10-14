provider "jupiterone" {
  # Configuration for the provider, such as API key, account ID, and region, can be set here or via environment variables.
}

resource "jupiterone_dashboard" "example_dashboard" {
  name = "Example Dashboard"
  type = "Account"
}

resource "jupiterone_dashboard_parameter" "example_parameter" {
  dashboard_id = jupiterone_dashboard.example_dashboard.id
  label        = "Example Parameter"
  name         = "example_parameter"
  value_type   = "string"
  type         = "QUERY_VARIABLE"
  default      = "default_value"
  disable_custom_input = false
  require_value = true

  options = [
    "option1",
    "option2",
    "option3"
  ]
}

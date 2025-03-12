resource "jupiterone_account_parameter" "cto_email" {
  name       = "ctoEmail"
  value      = "josh@jupiterone.com"
  value_type = "string"
}

resource "jupiterone_account_parameter" "jira_password" {
  name       = "jiraPassword"
  value      = "password123"
  value_type = "string"
  secret     = true
}

resource "jupiterone_account_parameter" "critical_severity" {
  name       = "criticalSeverity"
  value      = "1"
  value_type = "number"
}

resource "jupiterone_account_parameter" "account_active" {
  name       = "accountActive"
  value      = "false"
  value_type = "boolean"
}

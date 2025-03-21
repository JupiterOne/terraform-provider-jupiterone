---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "jupiterone_account_parameter Resource - terraform-provider-jupiterone"
subcategory: ""
description: |-
  A saved JupiterOne Account Parameter.
---

# jupiterone_account_parameter (Resource)

A saved JupiterOne Account Parameter.

## Example Usage

```terraform
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
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) The name of the account parameter. Must be unique. Must contain no spaces, just alphanumeric characters, and underscores.
- `value` (String) The value of the account parameter. This string value gets parsed based on the value_type.
- `value_type` (String) The type of the value. Possible values: string, number, boolean.

### Optional

- `secret` (Boolean) Whether or not the value can be retrieved from the api. Defaults to false. If it is secret then it cannot be retrieved through the API and will show as changed for every terraform plan.

### Read-Only

- `id` (String) The ID of this resource.



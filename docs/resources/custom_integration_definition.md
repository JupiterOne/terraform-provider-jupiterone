---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "jupiterone_custom_integration_definition Resource - terraform-provider-jupiterone"
subcategory: ""
description: |-
  A custom integration definition in JupiterOne
---

# jupiterone_custom_integration_definition (Resource)

A custom integration definition in JupiterOne

## Example Usage

```terraform
resource "jupiterone_custom_integration_definition" "example" {
  name             = "Custom Rapid 7"
  integration_type = "custom-rapid-7"
  icon             = "custom_earth"
  docs_web_link    = "https://docs.rapid7.com/"
  description      = "We cannot use the J1 rapid 7 integration because it is not supported in the US East region. This is a custom integration that uses the Rapid7 API to get data."
  integration_category = [
    "Device Management"
  ]
  custom_definition_type = "cft"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `custom_definition_type` (String) Type of custom definition. Must be either 'custom' or 'cft'
- `description` (String) Description of the custom integration definition
- `docs_web_link` (String) Documentation web link for the integration
- `icon` (String) Icon for the integration. Must be one of the preloaded icon names like 'custom_earth', 'custom_jupiter', etc. See custom integration definition UI for a full list of icons.
- `integration_category` (List of String) Category of integration
- `integration_type` (String) Type of integration. Should be unique across JupiterOne. Should be a kebab-case string (lowercase with hyphens), e.g. 'jupiterone-example-integration'
- `name` (String) Name of the custom integration definition

### Read-Only

- `id` (String) Unique identifier for the custom integration definition



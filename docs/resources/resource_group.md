---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "jupiterone_resource_group Resource - terraform-provider-jupiterone"
subcategory: ""
description: |-
  JupiterOne Resource Group
---

# jupiterone_resource_group (Resource)

JupiterOne Resource Group

## Example Usage

```terraform
resource "jupiterone_resource_group" "resource" {
  name = "Engineering"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) The name of the resource group.

### Read-Only

- `id` (String) The ID of this resource.



---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "jupiterone_smart_class_query Resource - terraform-provider-jupiterone"
subcategory: ""
description: |-
  A smart class query is a J1QL query that finds entities to associate with a smart class
---

# jupiterone_smart_class_query (Resource)

A smart class query is a J1QL query that finds entities to associate with a smart class

## Example Usage

```terraform
resource "jupiterone_smart_class_query" "query1" {
  smart_class_id = jupiterone_smart_class.example.id
  query          = "Find User with active=true"
  description    = "Find all active users"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `description` (String) A description of the smart class query
- `query` (String) The J1QL query to find entities for the smart class
- `smart_class_id` (String) The ID of the smart class to associate the query with

### Read-Only

- `id` (String) The ID of this resource.



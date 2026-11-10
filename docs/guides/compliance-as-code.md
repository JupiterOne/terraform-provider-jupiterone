---
page_title: "Managing compliance as code"
subcategory: ""
description: |-
  Manage frameworks, requirements, controls and control tests with Terraform, including frameworks imported from JSON.
---

# Managing compliance as code

This guide covers how continuous control monitoring objects relate to each other, and how to keep them in
Terraform without creating unmanaged duplicates.

| Resource | Web UI name | Belongs to |
|---|---|---|
| `jupiterone_control_framework` | Framework | - |
| `jupiterone_control_framework_requirement` | Requirement | exactly one framework |
| `jupiterone_control` | Control | any number of requirements, in any number of frameworks |
| `jupiterone_control_test` | Control test | exactly one control |

## One control, many requirements

A control can satisfy several requirements, including requirements in different frameworks. List them in
`requirement_ids`:

```terraform
resource "jupiterone_control" "mfa" {
  name  = "MFA enforced"
  state = "LIVE"
  requirement_ids = [
    jupiterone_control_framework_requirement.soc2_cc6_1.id,
    jupiterone_control_framework_requirement.iso_a_9_4_2.id,
  ]
}
```

When `requirement_ids` is set, it is authoritative: Terraform removes links that are not in the list. When it
is not set, Terraform leaves requirement links alone, so links made by a framework import or in the web UI are
kept. The current links are always readable from state.

## One query, many controls

A control test belongs to exactly one control. The API has no way to attach one control test to several
controls. To evaluate the same query under several controls, create one control test per control:

```terraform
resource "jupiterone_control_test" "mfa" {
  for_each = {
    access   = jupiterone_control.access.id
    identity = jupiterone_control.identity.id
  }

  name        = "Users without MFA (${each.key})"
  control_id  = each.value
  query       = "FIND User WITH mfaEnabled != true"
  results_are = "BAD"
}
```

## Frameworks imported from JSON

A framework imported from JSON in the web UI creates its requirements, controls and control tests outside
Terraform. There are two ways to keep these under Terraform management.

### Reuse Terraform-managed controls

Each `jupiterone_control` exports a `source_id`. Put it on the matching control in the framework JSON:

```json
{
  "name": "Internal baseline",
  "description": "Imported from JSON",
  "requirements": [
    {
      "displayName": "Strong authentication",
      "description": "All users authenticate with MFA",
      "controls": [
        { "displayName": "MFA enforced", "sourceId": "<jupiterone_control.mfa.source_id>" }
      ]
    }
  ]
}
```

The import links the existing control to the new requirement instead of creating a copy. It does not change
the control's state and does not add control tests to it. Controls in the JSON without a matching `sourceId`
are created as new controls.

Leave `requirement_ids` unset on a control that an imported framework links to, or add the imported
requirement IDs to it. Otherwise the next `terraform apply` removes the link the import made.

### Adopt the imported objects

Bring the imported framework, requirements, controls and control tests into state with `import` blocks. Find
their IDs with the `jupiterone_control_frameworks`, `jupiterone_controls` and `jupiterone_control_tests` data
sources, then let Terraform write the configuration:

```terraform
import {
  to = jupiterone_control_framework.baseline
  id = "<framework id>"
}

import {
  to = jupiterone_control.encryption
  id = "<control id>"
}
```

```shell
terraform plan -generate-config-out=generated.tf
terraform apply
```

Every resource supports import by ID, and an imported resource plans with no changes.

-> Destroying a framework also deletes its requirements, but keeps its controls and their control tests. Manage
or remove those separately.

## Moving rules to control tests

Each control test is backed by its own rule, which the API creates. An existing `jupiterone_rule` cannot be
attached to a control. To move a rule's query under a control, add a `jupiterone_control_test` with the same
query and remove the `jupiterone_rule` in the same change. Applying both together avoids running the query
twice.

## Control state

`state = "LIVE"` on a control persists. Updating other attributes, refreshing, or linking the control from a
framework import does not change it.

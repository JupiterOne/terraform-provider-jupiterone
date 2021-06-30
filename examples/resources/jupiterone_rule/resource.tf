resource "jupiterone_rule" "unencrypted_critical_data_stores" {
  name = "unencrypted-critical-data-stores"
  description = "Unencrypted data store with classification label of 'critical' or 'sensitive' or 'confidential' or 'restricted'"
  polling_interval = "ONE_DAY"

  question {
    queries {
      name = "query0"
      query = "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"
      version = "v1"
    }
  }

  outputs = [
    "queries.query0.total",
    "alertLevel"
  ]

  operations = <<EOF
    [
      {
        "when": {
          "type": "FILTER",
          "specVersion": 1,
          "condition": "{{queries.query0.total != 0}}"
        },
        "actions": [
          {
            "targetValue": "HIGH",
            "type": "SET_PROPERTY",
            "targetProperty": "alertLevel"
          },
          {
            "type": "CREATE_ALERT"
          }
        ]
      }
    ]
  EOF
}
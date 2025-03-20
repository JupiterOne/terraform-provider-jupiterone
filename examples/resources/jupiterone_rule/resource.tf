resource "jupiterone_rule" "unencrypted_critical_data_stores" {
  name             = "unencrypted-critical-data-stores"
  description      = "Unencrypted data store with classification label of 'critical' or 'sensitive' or 'confidential' or 'restricted'"
  polling_interval = "ONE_DAY"

  question {
    queries {
      name    = "query0"
      query   = "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"
      version = "v1"
    }
  }

  tags = ["exampletag"]

  outputs = [
    "queries.query0.total",
    "alertLevel"
  ]

  operations = [
    {
      when = jsonencode({
        "type" : "FILTER",
        "condition" : [
          "AND",
          [
            "queries.query0.total",
            "<",
            1000
          ]
        ]
      }),
      actions = [
        jsonencode({
          "targetValue" : "INFO",
          "type" : "SET_PROPERTY",
          "targetProperty" : "alertLevel"
        }),
        jsonencode({
          "type" : "CREATE_ALERT"
        })
      ]
    }
  ]
}


resource "jupiterone_rule" "users_without_mfa" {
  name             = "users-without-mfa"
  description      = "Users who do not have mfa enabled."
  polling_interval = "ONE_DAY"

  question_id = jupiterone_question.users_without_mfa.id

  tags = ["critical"]

  outputs = [
    "queries.query0.total",
    "alertLevel"
  ]

  operations = [
    {
      when = jsonencode({
        "type" : "FILTER",
        "condition" : "{{queries.query0.total != 0}}"
      }),
      actions = [
        jsonencode({
          "targetValue" : "INFO",
          "type" : "SET_PROPERTY",
          "targetProperty" : "alertLevel"
        }),
        jsonencode({
          "type" : "CREATE_ALERT"
        })
      ]
    }
  ]
}


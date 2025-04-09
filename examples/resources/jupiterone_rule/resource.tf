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

resource "jupiterone_rule" "unencrypted_critical_data_stores_jira" {
  name             = "users-without-mfa-jira"
  description      = "Create Jira when there are unencrypter dat astores."
  polling_interval = "ONE_DAY"

  question {
    queries {
      name    = "query0"
      query   = "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"
      version = "v1"
    }
  }

  tags = ["critical"]

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
            ">",
            0
          ]
        ]
      }),
      actions = [
        jsonencode({
          "integrationInstanceId" : "ec1a4975-7196-4f15-9466-2119c9d4aa19",
          "id" : "a886f8a1-a433-41b9-8adf-9b2386b0147f",
          "type" : "CREATE_JIRA_TICKET",
          "entityClass" : "Test",
          "summary" : "There are brandons up in here",
          "issueType" : "Task",
          "project" : "JJJ",
          "autoResolve" : true,
          "updateContentOnChanges" : false,
          "resolvedStatus" : "Done",
          "additionalFields" : {
            "description" : {
              "type" : "doc",
              "version" : 1,
              "content" : [
                {
                  "type" : "paragraph",
                  "content" : [
                    {
                      "type" : "text",
                      "text" : "{{alertWebLink}}\n\n**Affected Items:**\n\n* {{queries.query0.data|mapProperty('displayName')|join('\n* ')}}\n* {{queries.query0.data|mapProperty('tag.AccountName')|join('\n* ')}}"
                    }
                  ]
                }
              ]
            }
          }
        })
      ]
    }
  ]
}


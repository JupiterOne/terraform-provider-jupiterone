resource "jupiterone_integration" "example_aws" {
  name                        = "Custom"
  integration_definition_id   = "8013680b-311a-4c2e-b53b-c8735fd97a5c"
  polling_interval            = "THIRTY_MINUTES"
  description                 = "Custom integration"
  
  config = {
  }
}


resource "jupiterone_integration" "example_custom_file_transfer" {
  name                      = "Custom File Transfer"
  integration_definition_id = "4c66100d-3771-473d-99ce-cbe638b5ab50"
  polling_interval          = "THIRTY_MINUTES"
  description               = "Custom integration"
  
  config = jsonencode({
    "@tag" = {
      "AccountName" = "Custom",
    },
    "entities" = [
      {
        "id" = "Test",
        "uniqueIdentifier" = "758ba675-ff35-46aa-ae88-fd2d421a3c1f",
        "_class" = "ThreatIntel",
        "_keyField" = "test",
        "_type" = "test"
      }
    ]
  })
}

resource "jupiterone_integration" "example_custom_integration" {
  name                      = "Custom Integration"
  integration_definition_id = "8013680b-311a-4c2e-b53b-c8735fd97a5c"
  polling_interval          = "THIRTY_MINUTES"
  description               = "Custom integration"
  
  config = jsonencode({
    "@tag" = {
      "AccountName" = "Custom Integration",
    }
  })
}
data "jupiterone_integration_external_id" "for_aws" {}

resource "jupiterone_integration" "example_custom_integration" {
  name                      = "jupiterone-integration-dev"
  integration_definition_id = "7a669809-6e55-45b9-bf23-aa27613118e9" // AWS
  polling_interval          = "ONE_WEEK"
  description               = "Custom integration"

  config = jsonencode({
    "roleArn" : aws_iam_role.jupiterone.arn,
    "accountId" : data.aws_caller_identity.current.id,
    "@tag" : {
      "AccountName" : "jupiterone-integration-dev"
    },
    "collectSensitiveData" : true,
    "imagesFidingsMaxDaysInPast" : "7",
    "externalId" : data.jupiterone_integration_external_id.for_aws.id,
  })

  ingestion_sources_overrides = [
    {
      ingestion_source_id = "fetch-accessanalyzer-findings"
      enabled             = "true"
    },
    {
      ingestion_source_id = "fetch-acm-certificates"
      enabled             = "true"
    },
    {
      ingestion_source_id = "fetch-apigateway-rest-apis"
      enabled             = "true"
    }
  ]
}

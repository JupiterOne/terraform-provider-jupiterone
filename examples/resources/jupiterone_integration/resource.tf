resource "jupiterone_integration" "example_aws" {
  name                      = "Custom"
  integration_definition_id = "8013680b-311a-4c2e-b53b-c8735fd97a5c"
  polling_interval          = "THIRTY_MINUTES"
  description               = "Custom integration"

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
        "id"               = "Test",
        "uniqueIdentifier" = "758ba675-ff35-46aa-ae88-fd2d421a3c1f",
        "_class"           = "ThreatIntel",
        "_keyField"        = "test",
        "_type"            = "test"
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


# AWS

data "jupiterone_integration_external_id" "for_aws" {}

data "aws_caller_identity" "current" {}

resource "aws_iam_role" "jupiterone" {
  name = "JupiterOne2"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": ["arn:aws:iam::564077667165:root","arn:aws:iam::916604380196:root"]
      },
      "Action": "sts:AssumeRole",
      "Condition": {
        "StringEquals": {
          "sts:ExternalId": "${data.jupiterone_integration_external_id.for_aws.id}"
        }
      }
    }
  ]
}
EOF
}

resource "aws_iam_policy" "jupiterone_security_audit_policy" {
  name   = "JupiterOneSecurityAudit3"
  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Resource": "*",
      "Action": [
        "batch:Describe*",
        "batch:List*",
        "cloudhsm:Describe*",
        "cloudhsm:List*",
        "cloudwatch:GetMetricData",
        "codebuild:BatchGetReportGroups",
        "codebuild:List*",
        "ec2:GetEbsDefaultKmsKeyId",
        "eks:Describe*",
        "eks:List*",
        "fms:List*",
        "glacier:List*",
        "glue:GetJob",
        "glue:List*",
        "lambda:GetFunction",
        "lex:List*",
        "macie2:GetFindings",
        "redshift-serverless:List*",
        "ses:List*",
        "signer:List*",
        "sns:GetSubscriptionAttributes",
        "ssm:GetDocument"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "apigateway:GET"
      ],
      "Resource": [
        "arn:aws:apigateway:*::/*"
      ]
    }
  ]
}
EOF
}

resource "aws_iam_role_policy_attachment" "jupiterone_security_audit_policy_attachment" {
  role       = aws_iam_role.jupiterone.name
  policy_arn = aws_iam_policy.jupiterone_security_audit_policy.arn
}
resource "aws_iam_role_policy_attachment" "aws_security_audit_policy_attachment" {
  role       = aws_iam_role.jupiterone.name
  policy_arn = "arn:aws:iam::aws:policy/SecurityAudit"
}

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

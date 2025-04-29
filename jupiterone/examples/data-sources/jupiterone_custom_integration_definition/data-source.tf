terraform {
  required_providers {
    jupiterone = {
      source = "jupiterone/jupiterone"
    }
  }
}

# Get the custom integration definition by integration type
data "jupiterone_custom_integration_definition" "example" {
  integration_type = "example-custom-integration"
}

# Create an integration instance using the custom integration definition
resource "jupiterone_integration" "example" {
  name                        = "Example Integration"
  integration_definition_id   = data.jupiterone_custom_integration_definition.example.id
  description                 = "Example integration created using custom integration definition"
  polling_interval            = "ONE_DAY"
  config                      = jsonencode({})
  ingestion_sources_overrides = []
}

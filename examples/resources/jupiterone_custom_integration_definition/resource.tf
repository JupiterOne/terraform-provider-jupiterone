resource "jupiterone_custom_integration_definition" "example" {
  name             = "Custom Rapid 7"
  integration_type = "custom-rapid-7"
  icon             = "custom_earth"
  docs_web_link    = "https://docs.rapid7.com/"
  description      = "We cannot use the J1 rapid 7 integration because it is not supported in the US East region. This is a custom integration that uses the Rapid7 API to get data."
  integration_category = [
    "Device Management"
  ]
  custom_definition_type = "cft"
}

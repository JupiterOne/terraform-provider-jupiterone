provider "jupiterone" {}

resource "jupiterone_framework" "custom_standard" {
  name           = "Custom Standard"
  version        = "v1"
  framework_type = "STANDARD"

  web_link = "https://community.askj1.com/kb/articles/795-compliance-api-endpoints"
}

resource "jupiterone_group" "custom_group_1" {
  name         = "Custom Group 1"
  framework_id = jupiterone_framework.custom_standard.id
  description  = "Custom Group 1 are Requirements for Managing Resources"
}

resource "jupiterone_frameworkitem" "change_control" {
  framework_id = jupiterone_framework.custom_standard.id
  group_id     = jupiterone_group.custom_group_1.id

  name        = "Change Control"
  ref         = "test-requirement-1"
  description = "Changes should be controlled"

  web_link = "https://community.askj1.com/kb/articles/795-compliance-api-endpoints"
}

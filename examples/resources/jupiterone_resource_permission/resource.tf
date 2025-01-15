resource "jupiterone_resource_group" "engineering" {
  name = "Engineering"
}

resource "jupiterone_dashboard" "compliance" {
  name              = "Compliance"
  type              = "Account"
  resource_group_id = jupiterone_resource_group.engineering.id
}

resource "jupiterone_dashboard" "device_matrix" {
  name              = "Device Matrix"
  type              = "Account"
  resource_group_id = jupiterone_resource_group.engineering.id
}

resource "jupiterone_dashboard" "key_insights" {
  name = "Key Insights"
  type = "Account"
}

resource "jupiterone_user_group" "engineering" {
  name        = "Engineering"
  description = "This group can view and manage all dashboards in the Engineering resource group as well as view the Key Insights dashboard."
}

resource "jupiterone_resource_permission" "engineering_dashboard_engineering_resource_group" {
  subject_type  = "group"
  subject_id    = jupiterone_user_group.engineering.id
  resource_area = "dashboard"
  resource_type = "resource_group"
  resource_id   = jupiterone_resource_group.engineering.id
  can_create    = true
  can_read      = true
  can_update    = true
  can_delete    = true
}

resource "jupiterone_resource_permission" "engineering_dashboard_key_insights" {
  subject_type  = "group"
  subject_id    = jupiterone_user_group.engineering.id
  resource_area = "dashboard"
  resource_type = "dashboard"
  resource_id   = jupiterone_dashboard.key_insights.id
  can_create    = false
  can_read      = true
  can_update    = false
  can_delete    = false
}

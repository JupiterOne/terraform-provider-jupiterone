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

resource "jupiterone_resource_permission" "engineering_compliance" {
  subject_type  = "group"
  subject_id    = jupiterone_user_group.engineering.id
  resource_area = "dashboard"
  resource_type = "resource_group"
  resource_id   = "*"
  canCreate     = true
  canRead       = true
  canUpdate     = true
  canDelete     = true
}

resource "jupiterone_resource_permission" "engineering_compliance" {
  subject_type  = "group"
  subject_id    = jupiterone_user_group.engineering.id
  resource_area = "dashboard"
  resource_type = "dashboard"
  resource_id   = jupiterone_dashboard.key_insights.id
  canCreate     = false
  canRead       = true
  canUpdate     = false
  canDelete     = false
}

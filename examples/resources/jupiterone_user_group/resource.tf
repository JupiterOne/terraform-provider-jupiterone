resource "jupiterone_user_group" "insights_admin" {
  name          = "Insights Admin"
  description   = "This group can create team dashboards."
  permissions   = ["adminInsights", "readGraph"]
}

resource "jupiterone_user_group" "hr_insights_readonly" {
  name          = "HR Insights Readonly"
  description   = "This group can view team dashboards and create personal boards. They can only view jupiterone_user graph entities."
  permissions   = ["accessInsights", "readGraph"]
  query_policy  = [
    {"_class": ["User"]},
    {"_type": ["jupiterone_user"]}
  ]
}
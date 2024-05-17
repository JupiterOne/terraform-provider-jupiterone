resource "jupiterone_user_group_membership" "existing_user_to_insights_admin_group" {
  group_id = jupiterone_user_group.insights_admin.id
  email = "brandon.pfeiffer@jupiterone.com"
}

resource "jupiterone_user_group_membership" "existing_user_to_insights_readonly_group" {
  group_id = jupiterone_user_group.hr_insights_readonly.id
  email = "brandon.pfeiffer@jupiterone.com"
}

resource "jupiterone_user_group_membership" "new_user_to_users_group" {
  group_id = data.jupiterone_user_group.administrators.id
  email = "brandon.pfeiffer+may164@jupiterone.com"
}
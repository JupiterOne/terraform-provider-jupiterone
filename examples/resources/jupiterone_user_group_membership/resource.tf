resource "jupiterone_user_group_membership" "existing_user_to_insights_admin_group" {
  group_id = jupiterone_user_group.insights_admin.id
  email = "existing.user@jupiterone.com"
}

resource "jupiterone_user_group_membership" "existing_user_to_insights_readonly_group" {
  group_id = jupiterone_user_group.hr_insights_readonly.id
  email = "existing.user@jupiterone.com"
}

resource "jupiterone_user_group_membership" "new_user_to_admin_group" {
  group_id = data.jupiterone_user_group.administrators.id
  email = "new.user@jupiterone.com"
}
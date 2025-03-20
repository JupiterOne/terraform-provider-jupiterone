data "jupiterone_j1ql_result" "test" {
  query = {
    query = <<EOF
      FIND aws_iam_user WITH role = 'admin' AS U 
      RETURN U.username as username
    EOF
  }
}

data "jupiterone_user_group" "administrators" {
  name = "Administrators"
}

resource "jupiterone_user_group_membership" "admin_memberships" {
  for_each = { for idx, item in jsondecode(data.jupiterone_j1ql_result.test.data_json) : idx => item }
  group_id = data.jupiterone_user_group.administrators.id
  email    = "${each.value.username}@jupiterone.com"
}

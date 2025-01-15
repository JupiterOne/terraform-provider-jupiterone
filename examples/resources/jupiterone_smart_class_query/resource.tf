resource "jupiterone_smart_class_query" "query1" {
  smart_class_id = jupiterone_smart_class.example.id
  query          = "Find User with active=true"
  description    = "Find all active users"
}

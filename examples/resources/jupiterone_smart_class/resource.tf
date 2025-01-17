resource "jupiterone_smart_class" "example" {
  tag_name    = "Example"
  description = "Example smart class"
}

resource "jupiterone_smart_class_query" "query1" {
  smart_class_id = jupiterone_smart_class.example.id
  query          = "Find User"
  description    = "Example query"
}

resource "jupiterone_smart_class_tag" "tag2" {
  smart_class_id = jupiterone_smart_class.example.id
  name           = "user"
  type           = "boolean"
  value          = "true"
}


resource "jupiterone_smart_class_tag" "tag1" {
  smart_class_id = jupiterone_smart_class.example.id
  name           = "person"
  type           = "boolean"
  value          = "true"
}

resource "jupiterone_smart_class_tag" "tag2" {
  smart_class_id = jupiterone_smart_class.example.id
  name           = "worth"
  type           = "number"
  value          = "50000"
}

resource "jupiterone_smart_class_tag" "tag3" {
  smart_class_id = jupiterone_smart_class.example.id
  name           = "label"
  type           = "string"
  value          = "example"
}

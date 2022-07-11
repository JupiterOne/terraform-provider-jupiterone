resource "jupiterone_question" "unencrypted_critical_data_stores" {
  title = "Unencrypted critical data stores"
  description = "Unencrypted data store with classification label of 'critical' or 'sensitive' or 'confidential' or 'restricted'"
  tags = ["hello"]

  query {
    name = "query0"
    query = "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"
    version = "v1"
    results_are = "BAD"
  }
}

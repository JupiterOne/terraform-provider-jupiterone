# The evaluation results of a single control test, including the record count
# from the last run.
data "jupiterone_control_test" "mfa" {
  id = jupiterone_control_test.mfa.id
}

output "mfa_test_results" {
  value = {
    status            = data.jupiterone_control_test.mfa.status
    last_evaluated_on = data.jupiterone_control_test.mfa.last_evaluated_on

    # Always exactly one element: a control test is backed by a single rule
    # holding one query, even though the API models queries as a list.
    record_count = data.jupiterone_control_test.mfa.queries[0].record_count

    # false means the query returned no data, so the result proves nothing.
    effective = data.jupiterone_control_test.mfa.queries[0].effective
  }
}

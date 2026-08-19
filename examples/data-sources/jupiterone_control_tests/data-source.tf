# Every control test attached to a control, with its evaluation results.
data "jupiterone_control_tests" "mfa" {
  control_id = jupiterone_control.mfa.id
}

# Surface tests that ran but had nothing to evaluate. These read as passing
# without proving anything, so they are worth reviewing separately from failures.
output "tests_without_data" {
  value = [
    for t in data.jupiterone_control_tests.mfa.control_tests : t.name
    if length([for q in t.queries : q if q.effective == false]) > 0
  ]
}

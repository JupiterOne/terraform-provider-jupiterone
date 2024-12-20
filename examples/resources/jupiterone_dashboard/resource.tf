resource "jupiterone_dashboard" "compliance" {
  name          = "Compliance Dashboard"
  type          = "Account"
}

resource "jupiterone_dashboard_parameter" "controlname" {
  dashboard_id         = jupiterone_dashboard.compliance.id
  label                = "Control Name"
  name                 = "controlName"
  value_type           = "string"
  type                 = "QUERY_VARIABLE"
  disable_custom_input = true
  require_value        = false
}

resource "jupiterone_widget" "compliant-controls" {
  title        = "Number of compliant controls"
  dashboard_id = jupiterone_dashboard.compliance.id
  description  = "Count of all controls that are compliant across all frameworks."
  type         = "number"

  config = {
    queries = [{
      name  = "Query1"
      query = "FIND jupiterone_rule WITH displayName~= {{${jupiterone_dashboard_parameter.controlname.name}}} AS ENT RETURN count(ENT) AS value"
    }]
    settings = jsonencode({ number : { success : { limitCondition : "greaterThan", val1 : "0" } } })
  }
}

resource "jupiterone_widget" "compliance-score" {
  title        = "Compliance score"
  dashboard_id = jupiterone_dashboard.compliance.id
  description  = "Percentage of compliance requirements that have no issues."
  type         = "number"

  config = {
    queries = [{
      name = "Query1"
      query      = "FIND jupiterone_rule WITH [tag.CIS2.0]=true AS rules (THAT REPORTED jupiterone_rule_alert AS alerts)? RETURN (1-count(alerts)/count(rules))*100 AS value"
    }]
    settings = jsonencode({ number : {  error : { limitCondition : "greaterThan", val1 : "0" } } })
  }
}

resource "jupiterone_widget" "table-non-compliant-entities" {
  title        = "All entites that are failing controls"
  dashboard_id = jupiterone_dashboard.compliance.id
  description  = "All entites that are failing controls."
  type         = "table"

  config = {
    queries = [{
      name = "Query1"
      query      = "FIND * WITH _source='integration-managed' AS ent THAT RELATES TO jupiterone_rule_alert WITH [tag.CIS2.0]=true AS Alert RETURN ent.displayName, ent._type, Alert.[tag.CIS2.0], Alert.[tag.1.1], Alert.[tag.1.2], Alert.[tag.1.3], Alert.[tag.2.1]"
    }]
  }
}

resource "jupiterone_widget" "plot-compliance-per-control" {
  title        = "Number of non-compliant entities per control"
  dashboard_id = jupiterone_dashboard.compliance.id
  description  = "Number of non-compliant entities per control."
  type         = "bar"

  config = {
    queries = [{
      name = "Query1"
      query      = "FIND jupiterone_rule WITH [tag.CIS2.0]=true AS Control (THAT REPORTED jupiterone_rule_alert AS Alert)? RETURN Control.displayName AS x, Coalesce(Alert.totalNumberOfAffectedEntities,0) as y"
    }]
  }
}
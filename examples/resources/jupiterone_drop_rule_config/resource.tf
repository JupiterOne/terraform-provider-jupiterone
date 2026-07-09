# Manage the account's drop-rule configuration (admin only). There is one
# configuration per account, so declare at most one of these resources.
#
# Drop rules skip ingesting matching entities during integration sync. They do
# NOT cover mapper/MRR entities, and the account owns the outcome of anything it
# drops. Condition `value` is JSON-encoded (use `jsonencode(...)`).

resource "jupiterone_drop_rule_config" "this" {
  enabled = true

  rules = [
    # Drop non-fine-tuneable AWS Bedrock foundation models.
    {
      id   = "aws-bedrock-noise"
      type = "aws_bedrock_foundation_model"
      conditions = [
        {
          property = "isFineTuneable"
          op       = "eq"
          value    = jsonencode(false)
        }
      ]
    },

    # Drop default subnets, matched by class instead of type.
    {
      id    = "aws-default-subnets"
      class = "Network"
      conditions = [
        {
          property = "defaultForAz"
          op       = "eq"
          value    = jsonencode(true)
        }
      ]
    },

    # Drop AWS-managed IAM policies via a startsWith prefix match.
    {
      id   = "aws-managed-iam-policies"
      type = "aws_iam_policy"
      conditions = [
        {
          property = "arn"
          op       = "startsWith"
          value    = jsonencode("arn:aws:iam::aws:policy/")
        }
      ]
    },
  ]
}

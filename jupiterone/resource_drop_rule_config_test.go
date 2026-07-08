package jupiterone

import (
	"context"
	"fmt"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

const dropRuleConfigResource = "jupiterone_drop_rule_config.test"

func TestDropRuleConfig_Basic(t *testing.T) {
	ctx := context.TODO()

	recordingClient, directClient, cleanup := setupTestClientsWithReplaySupport(ctx, t)
	defer cleanup(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(recordingClient),
		CheckDestroy:             testAccCheckDropRuleConfigDestroy(ctx, directClient),
		Steps: []resource.TestStep{
			// Create with a single rule.
			{
				Config: testDropRuleConfigSingle,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDropRuleConfigExists(ctx, directClient),
					resource.TestCheckResourceAttr(dropRuleConfigResource, "enabled", "true"),
					resource.TestCheckResourceAttrSet(dropRuleConfigResource, "version"),
					resource.TestCheckResourceAttr(dropRuleConfigResource, "rules.#", "1"),
					resource.TestCheckResourceAttr(dropRuleConfigResource, "rules.0.id", "aws-bedrock-noise"),
					resource.TestCheckResourceAttr(dropRuleConfigResource, "rules.0.type", "aws_bedrock_foundation_model"),
					resource.TestCheckResourceAttr(dropRuleConfigResource, "rules.0.conditions.#", "1"),
					resource.TestCheckResourceAttr(dropRuleConfigResource, "rules.0.conditions.0.property", "isFineTuneable"),
					resource.TestCheckResourceAttr(dropRuleConfigResource, "rules.0.conditions.0.op", "eq"),
					resource.TestCheckResourceAttr(dropRuleConfigResource, "rules.0.conditions.0.value", "false"),
				),
			},
			// Update: add a second rule (uses an `in` condition) and keep the first.
			{
				Config: testDropRuleConfigTwo,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDropRuleConfigExists(ctx, directClient),
					resource.TestCheckResourceAttr(dropRuleConfigResource, "rules.#", "2"),
					resource.TestCheckResourceAttr(dropRuleConfigResource, "rules.1.id", "aws-default-networking"),
					resource.TestCheckResourceAttr(dropRuleConfigResource, "rules.1.type", "aws_subnet"),
					resource.TestCheckResourceAttr(dropRuleConfigResource, "rules.1.conditions.0.op", "eq"),
					resource.TestCheckResourceAttr(dropRuleConfigResource, "rules.1.conditions.0.value", "true"),
				),
			},
			// Import.
			{
				ResourceName:      dropRuleConfigResource,
				ImportState:       true,
				ImportStateId:     dropRuleConfigID,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckDropRuleConfigExists(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}
		current, err := client.GetDropRulesConfig(ctx, qlient)
		if err != nil {
			return err
		}
		if current.DropRulesConfigBeta.Version == 0 {
			return fmt.Errorf("drop rule config does not exist")
		}
		return nil
	}
}

func testAccCheckDropRuleConfigDestroy(ctx context.Context, qlient graphql.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if qlient == nil {
			return nil
		}
		current, err := client.GetDropRulesConfig(ctx, qlient)
		if err != nil {
			return err
		}
		// Delete disables the singleton and clears its rules (there is no
		// delete-config mutation), so "destroyed" means dormant.
		if current.DropRulesConfigBeta.Enabled {
			return fmt.Errorf("drop rule config still enabled after destroy")
		}
		if current.DropRulesConfigBeta.RuleCount != 0 {
			return fmt.Errorf("drop rule config still has %d rules after destroy", current.DropRulesConfigBeta.RuleCount)
		}
		return nil
	}
}

const testDropRuleConfigSingle = `
resource "jupiterone_drop_rule_config" "test" {
  enabled = true
  rules = [
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
    }
  ]
}
`

const testDropRuleConfigTwo = `
resource "jupiterone_drop_rule_config" "test" {
  enabled = true
  rules = [
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
    {
      id   = "aws-default-networking"
      type = "aws_subnet"
      conditions = [
        {
          property = "defaultForAz"
          op       = "eq"
          value    = jsonencode(true)
        }
      ]
    }
  ]
}
`

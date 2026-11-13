package jupiterone

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	acctest "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationConfigHash(t *testing.T) {
	a, err := integrationConfigHash(`{"roleArn":"arn","accessKey":"secret","nested":{"b":1,"a":[1,2]}}`)
	require.NoError(t, err)

	reordered, err := integrationConfigHash(`{ "nested": {"a":[1,2], "b":1}, "accessKey":"secret", "roleArn":"arn" }`)
	require.NoError(t, err)
	assert.Equal(t, a, reordered, "key order and spacing must not change the hash")

	for name, other := range map[string]string{
		"a changed value":   `{"roleArn":"arn","accessKey":"rotated","nested":{"b":1,"a":[1,2]}}`,
		"an added key":      `{"roleArn":"arn","accessKey":"secret","nested":{"b":1,"a":[1,2]},"extra":true}`,
		"a reordered array": `{"roleArn":"arn","accessKey":"secret","nested":{"b":1,"a":[2,1]}}`,
	} {
		hash, err := integrationConfigHash(other)
		require.NoError(t, err, name)
		assert.NotEqual(t, a, hash, name)
	}

	_, err = integrationConfigHash(`not json`)
	assert.Error(t, err)
}

// What a config plans when state holds masked values, with and without a
// hash of the config Terraform last sent.
func TestPlanMaskedConfig(t *testing.T) {
	// A definition where the API masks every value.
	allMasked := `{"accessKey":"***masked***","roleArn":"***masked***"}`
	oneMasked := `{"accessKey":"***masked***","roleArn":"arn:one"}`
	sent := `{"roleArn":"arn:one","accessKey":"secret-1"}`
	sentHash, err := integrationConfigHash(sent)
	require.NoError(t, err)

	testCases := []struct {
		name         string
		state        string
		config       string
		sentHash     string
		keepState    bool
		undetectable bool
	}{
		{
			name:      "applied before, config unchanged",
			state:     allMasked,
			config:    `{"accessKey":"secret-1","roleArn":"arn:one"}`,
			sentHash:  sentHash,
			keepState: true,
		},
		{
			name:     "applied before, a masked secret rotated",
			state:    allMasked,
			config:   `{"roleArn":"arn:one","accessKey":"secret-2"}`,
			sentHash: sentHash,
		},
		{
			name:     "applied before, a masked non-secret changed",
			state:    allMasked,
			config:   `{"roleArn":"arn:two","accessKey":"secret-1"}`,
			sentHash: sentHash,
		},
		{
			name:     "applied before, a key added",
			state:    allMasked,
			config:   `{"roleArn":"arn:one","accessKey":"secret-1","region":"us"}`,
			sentHash: sentHash,
		},
		{
			name:         "imported and not applied, config differs only under masked values",
			state:        allMasked,
			config:       `{"roleArn":"anything","accessKey":"anything"}`,
			keepState:    true,
			undetectable: true,
		},
		{
			name:   "imported and not applied, a value that is not masked differs",
			state:  oneMasked,
			config: `{"roleArn":"arn:two","accessKey":"secret-1"}`,
		},
		{
			name:   "imported and not applied, a key added",
			state:  allMasked,
			config: `{"roleArn":"arn:one","accessKey":"secret-1","region":"us"}`,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			keepState, undetectable := planMaskedConfig(tt.state, tt.config, tt.sentHash)
			assert.Equal(t, tt.keepState, keepState, "keepState")
			assert.Equal(t, tt.undetectable, undetectable, "undetectable")
		})
	}
}

func TestContainsMaskedValue(t *testing.T) {
	assert.True(t, containsMaskedValue(map[string]interface{}{"accessKey": "***masked***"}))
	assert.True(t, containsMaskedValue(map[string]interface{}{"auth": map[string]interface{}{"token": "***masked***"}}))
	assert.True(t, containsMaskedValue(map[string]interface{}{"keys": []interface{}{"a", "***masked***"}}))

	assert.False(t, containsMaskedValue(map[string]interface{}{"accessKey": "real", "n": 1.0, "on": true, "none": nil}))
	// Only a value that is exactly the marker counts.
	assert.False(t, containsMaskedValue(map[string]interface{}{"note": "value shown as ***masked*** in the UI"}))
	assert.False(t, containsMaskedValue(map[string]interface{}{"***masked***": "a key, not a value"}))
}

func TestSameIngestionSourceOverrides(t *testing.T) {
	acm := IngestionSourceOverride{IngestionSourceId: "fetch-acm-certificates", Enabled: true}
	s3 := IngestionSourceOverride{IngestionSourceId: "fetch-s3-buckets", Enabled: false}
	s3On := IngestionSourceOverride{IngestionSourceId: "fetch-s3-buckets", Enabled: true}

	assert.True(t, sameIngestionSourceOverrides([]IngestionSourceOverride{acm, s3}, []IngestionSourceOverride{s3, acm}))
	assert.True(t, sameIngestionSourceOverrides(nil, []IngestionSourceOverride{}))

	assert.False(t, sameIngestionSourceOverrides([]IngestionSourceOverride{acm, s3}, []IngestionSourceOverride{acm, s3On}), "enabled differs")
	assert.False(t, sameIngestionSourceOverrides([]IngestionSourceOverride{acm, s3}, []IngestionSourceOverride{acm}), "one removed")
	assert.False(t, sameIngestionSourceOverrides([]IngestionSourceOverride{acm, acm}, []IngestionSourceOverride{acm, s3}), "a repeated pair is not another pair")
}

func overridesList(t *testing.T, overrides ...IngestionSourceOverride) types.List {
	t.Helper()
	elementType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"ingestion_source_id": types.StringType,
		"enabled":             types.BoolType,
	}}
	list, diags := types.ListValueFrom(context.Background(), elementType, overrides)
	require.False(t, diags.HasError(), "%v", diags)
	return list
}

func TestIngestionSourcesOverridesOrderModifier(t *testing.T) {
	acm := IngestionSourceOverride{IngestionSourceId: "fetch-acm-certificates", Enabled: true}
	s3 := IngestionSourceOverride{IngestionSourceId: "fetch-s3-buckets", Enabled: false}
	s3On := IngestionSourceOverride{IngestionSourceId: "fetch-s3-buckets", Enabled: true}
	elementType := overridesList(t).ElementType(context.Background())

	testCases := []struct {
		name       string
		config     types.List
		state      types.List
		keepsState bool
	}{
		{name: "same overrides, other order", config: overridesList(t, s3, acm), state: overridesList(t, acm, s3), keepsState: true},
		{name: "an override toggled", config: overridesList(t, s3On, acm), state: overridesList(t, acm, s3)},
		{name: "an override added", config: overridesList(t, s3, acm), state: overridesList(t, acm)},
		{name: "nothing in state", config: overridesList(t, s3, acm), state: types.ListNull(elementType)},
		{name: "nothing configured", config: types.ListNull(elementType), state: overridesList(t, acm, s3)},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			req := planmodifier.ListRequest{ConfigValue: tt.config, PlanValue: tt.config, StateValue: tt.state}
			resp := &planmodifier.ListResponse{PlanValue: tt.config}

			ingestionSourcesOverridesOrderModifier{}.PlanModifyList(context.Background(), req, resp)

			assert.False(t, resp.Diagnostics.HasError())
			if tt.keepsState {
				assert.Equal(t, tt.state, resp.PlanValue)
			} else {
				assert.Equal(t, tt.config, resp.PlanValue)
			}
		})
	}
}

// A source instance or collector pool cleared outside Terraform must show as
// drift, like the other optional strings.
func TestIntegrationRead_ClearedIdsShowAsDrift(t *testing.T) {
	model := IntegrationModel{
		Config:                      types.StringValue(`{}`),
		SourceIntegrationInstanceId: types.StringValue("source-1"),
		CollectorPoolId:             types.StringValue("pool-1"),
	}
	diags := applyIntegrationInstance(&model, client.GetIntegrationInstanceIntegrationInstance{
		Name:   "aws",
		Config: map[string]interface{}{},
	})
	require.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, types.StringValue(""), model.SourceIntegrationInstanceId)
	assert.Equal(t, types.StringValue(""), model.CollectorPoolId)

	// Never set, and the API has none: stays null, so nothing is planned.
	unset := IntegrationModel{Config: types.StringValue(`{}`)}
	diags = applyIntegrationInstance(&unset, client.GetIntegrationInstanceIntegrationInstance{
		Name:   "aws",
		Config: map[string]interface{}{},
	})
	require.False(t, diags.HasError(), "%v", diags)
	assert.True(t, unset.SourceIntegrationInstanceId.IsNull())
	assert.True(t, unset.CollectorPoolId.IsNull())
}

// The overrides are deliberately not in id order, which is the order the API
// returns them in.
func testIntegrationMaskAllConfig(name, roleArn, accessKey string) string {
	return fmt.Sprintf(`
resource "jupiterone_integration" "test" {
  name                      = %q
  integration_definition_id = "7a669809-6e55-45b9-bf23-aa27613118e9"
  polling_interval          = "ONE_DAY"

  config = jsonencode({
    roleArn   = %q
    accessKey = %q
  })

  ingestion_sources_overrides = [
    {
      ingestion_source_id = "fetch-s3-buckets"
      enabled             = false
    },
    {
      ingestion_source_id = "fetch-acm-certificates"
      enabled             = true
    },
  ]
}
`, name, roleArn, accessKey)
}

// Through Terraform itself, against an API that masks every config value:
// import with overrides out of order plans no change; once Terraform has
// applied the integration, a change to any config value, a rotated secret
// included, plans an update and sends the new value; and the masked marker in
// the configuration is refused.
func TestIntegration_ImportWhenEveryConfigValueIsMasked(t *testing.T) {
	const roleArn = "arn:aws:iam::123456789012:role/jupiterone"

	api := newFakeIntegrationAPI()
	api.maskAll = true
	api.instances["existing-1"] = map[string]interface{}{
		"id":                      "existing-1",
		"name":                    "aws-production",
		"pollingInterval":         "ONE_DAY",
		"integrationDefinitionId": "7a669809-6e55-45b9-bf23-aa27613118e9",
		"description":             "",
		"resourceGroupId":         "",
		"config": map[string]interface{}{
			"roleArn":   roleArn,
			"accessKey": "secret-1",
		},
		"pollingIntervalCronExpression": map[string]interface{}{"hour": 0, "dayOfWeek": 0},
		"ingestionSourcesOverrides": []interface{}{
			map[string]interface{}{"ingestionSourceId": "fetch-acm-certificates", "enabled": true},
			map[string]interface{}{"ingestionSourceId": "fetch-s3-buckets", "enabled": false},
		},
	}

	resourceName := "jupiterone_integration.test"

	sentConfig := func(update int, key string) (interface{}, error) {
		_, _, _, updates := api.snapshot()
		if len(updates) != update {
			return nil, fmt.Errorf("expected %d update(s), got %d", update, len(updates))
		}
		config, _ := updates[update-1]["config"].(map[string]interface{})
		return config[key], nil
	}

	acctest.Test(t, acctest.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(api),
		Steps: []acctest.TestStep{
			{
				Config:             testIntegrationMaskAllConfig("aws-production", roleArn, "secret-1"),
				ResourceName:       resourceName,
				ImportState:        true,
				ImportStateId:      "existing-1",
				ImportStatePersist: true,
			},
			{
				// Overrides in another order than the API returns: no change.
				Config:   testIntegrationMaskAllConfig("aws-production", roleArn, "secret-1"),
				PlanOnly: true,
			},
			{
				// The first apply after the import, for a rename. It sends the
				// configured values, never the marker.
				Config: testIntegrationMaskAllConfig("aws-production-renamed", roleArn, "secret-1"),
				ConfigPlanChecks: acctest.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
				Check: func(*terraform.State) error {
					for key, want := range map[string]string{"roleArn": roleArn, "accessKey": "secret-1"} {
						got, err := sentConfig(1, key)
						if err != nil {
							return err
						}
						if got != want {
							return fmt.Errorf("update 1 sent %s = %v, want %v", key, got, want)
						}
					}
					return nil
				},
			},
			{
				Config:   testIntegrationMaskAllConfig("aws-production-renamed", roleArn, "secret-1"),
				PlanOnly: true,
			},
			{
				// A rotated secret alone, with every value masked in state.
				Config: testIntegrationMaskAllConfig("aws-production-renamed", roleArn, "secret-2"),
				ConfigPlanChecks: acctest.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
				Check: func(*terraform.State) error {
					got, err := sentConfig(2, "accessKey")
					if err != nil {
						return err
					}
					if got != "secret-2" {
						return fmt.Errorf("the rotated secret was not sent; sent %v", got)
					}
					return nil
				},
			},
			{
				Config:   testIntegrationMaskAllConfig("aws-production-renamed", roleArn, "secret-2"),
				PlanOnly: true,
			},
			{
				// A value that is not a secret, also masked by this API.
				Config: testIntegrationMaskAllConfig("aws-production-renamed", roleArn+"-2", "secret-2"),
				ConfigPlanChecks: acctest.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
				Check: func(*terraform.State) error {
					got, err := sentConfig(3, "roleArn")
					if err != nil {
						return err
					}
					if got != roleArn+"-2" {
						return fmt.Errorf("the changed role was not sent; sent %v", got)
					}
					return nil
				},
			},
			{
				// The marker copied out of state is refused before anything is sent.
				Config:      testIntegrationMaskAllConfig("aws-production-renamed", roleArn+"-2", integrationConfigMaskedValue),
				ExpectError: regexp.MustCompile(`Config contains the masked marker`),
			},
			{
				// Back to a valid configuration, which the destroy that ends the
				// test runs with.
				Config:   testIntegrationMaskAllConfig("aws-production-renamed", roleArn+"-2", "secret-2"),
				PlanOnly: true,
			},
		},
	})

	_, _, _, updates := api.snapshot()
	assert.Len(t, updates, 3, "the refused configuration must not reach the API")
}

// Straight after an import nothing has been applied, so a change under a
// masked value cannot be seen: no change is planned. This pins the documented
// limit; the plan carries a warning (see TestIntegrationMaskedConfigModifier).
func TestIntegration_ImportedAndNotAppliedCannotSeeMaskedChange(t *testing.T) {
	api := newFakeIntegrationAPI()
	api.maskAll = true
	api.instances["existing-1"] = map[string]interface{}{
		"id":                            "existing-1",
		"name":                          "aws-production",
		"pollingInterval":               "ONE_DAY",
		"integrationDefinitionId":       "7a669809-6e55-45b9-bf23-aa27613118e9",
		"config":                        map[string]interface{}{"roleArn": "arn:one", "accessKey": "secret-1"},
		"pollingIntervalCronExpression": map[string]interface{}{"hour": 0, "dayOfWeek": 0},
		"ingestionSourcesOverrides": []interface{}{
			map[string]interface{}{"ingestionSourceId": "fetch-acm-certificates", "enabled": true},
			map[string]interface{}{"ingestionSourceId": "fetch-s3-buckets", "enabled": false},
		},
	}

	acctest.Test(t, acctest.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(api),
		Steps: []acctest.TestStep{
			{
				Config:             testIntegrationMaskAllConfig("aws-production", "arn:one", "secret-1"),
				ResourceName:       "jupiterone_integration.test",
				ImportState:        true,
				ImportStateId:      "existing-1",
				ImportStatePersist: true,
			},
			{
				Config:   testIntegrationMaskAllConfig("aws-production", "arn:one", "secret-rotated"),
				PlanOnly: true,
			},
		},
	})
}

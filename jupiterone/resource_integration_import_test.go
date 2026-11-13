package jupiterone

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	acctest "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeIntegrationAPI stands in for the JupiterOne GraphQL API for the four
// integration instance operations, so import, plan and apply can be exercised
// with no account and no recorded cassette. Like the real API it returns the
// values of secret config keys as the masked marker (or every value, with
// maskAll), and it returns ingestion source overrides in its own order
// (sorted by id), not the caller's.
type fakeIntegrationAPI struct {
	mu         sync.Mutex
	instances  map[string]map[string]interface{}
	secretKeys map[string]bool
	// maskAll returns every config value masked, which is what the API does
	// for a definition whose fields do not opt out of masking.
	maskAll bool
	nextID  int

	ops     []string
	created []string
	deleted []string
	updates []map[string]interface{}
}

func newFakeIntegrationAPI(secretKeys ...string) *fakeIntegrationAPI {
	api := &fakeIntegrationAPI{
		instances:  map[string]map[string]interface{}{},
		secretKeys: map[string]bool{},
	}
	for _, key := range secretKeys {
		api.secretKeys[key] = true
	}
	return api
}

func (api *fakeIntegrationAPI) MakeRequest(ctx context.Context, req *graphql.Request, resp *graphql.Response) error {
	api.mu.Lock()
	defer api.mu.Unlock()

	api.ops = append(api.ops, req.OpName)

	var variables map[string]interface{}
	raw, err := json.Marshal(req.Variables)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &variables); err != nil {
		return err
	}

	var data interface{}
	switch req.OpName {
	case "GetIntegrationInstance":
		id, _ := variables["id"].(string)
		instance, ok := api.instances[id]
		if !ok {
			return errors.New("Integration instance not found")
		}
		data = map[string]interface{}{"integrationInstance": api.asReturned(instance)}
	case "CreateIntegrationInstance":
		api.nextID++
		id := fmt.Sprintf("created-%d", api.nextID)
		instance, _ := variables["instance"].(map[string]interface{})
		instance["id"] = id
		api.instances[id] = instance
		api.created = append(api.created, id)
		data = map[string]interface{}{"createIntegrationInstance": api.asReturned(instance)}
	case "UpdateIntegrationInstance":
		id, _ := variables["id"].(string)
		instance, ok := api.instances[id]
		if !ok {
			return errors.New("Integration instance not found")
		}
		update, _ := variables["update"].(map[string]interface{})
		api.updates = append(api.updates, update)
		for key, value := range update {
			instance[key] = value
		}
		data = map[string]interface{}{"updateIntegrationInstance": api.asReturned(instance)}
	case "DeleteIntegrationInstance":
		id, _ := variables["id"].(string)
		delete(api.instances, id)
		api.deleted = append(api.deleted, id)
		data = map[string]interface{}{"deleteIntegrationInstance": map[string]interface{}{"success": true}}
	default:
		return fmt.Errorf("fakeIntegrationAPI: unexpected operation %q", req.OpName)
	}

	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, resp.Data)
}

// asReturned copies an instance the way the API returns it: secret config
// values masked, overrides sorted by ingestion source id.
func (api *fakeIntegrationAPI) asReturned(instance map[string]interface{}) map[string]interface{} {
	returned := map[string]interface{}{}
	for key, value := range instance {
		returned[key] = value
	}

	if config, ok := instance["config"].(map[string]interface{}); ok {
		masked := map[string]interface{}{}
		for key, value := range config {
			if api.maskAll || api.secretKeys[key] {
				masked[key] = integrationConfigMaskedValue
			} else {
				masked[key] = value
			}
		}
		returned["config"] = masked
	}

	if overrides, ok := instance["ingestionSourcesOverrides"].([]interface{}); ok {
		sorted := append([]interface{}{}, overrides...)
		sort.SliceStable(sorted, func(i, j int) bool {
			left, _ := sorted[i].(map[string]interface{})["ingestionSourceId"].(string)
			right, _ := sorted[j].(map[string]interface{})["ingestionSourceId"].(string)
			return left < right
		})
		returned["ingestionSourcesOverrides"] = sorted
	}

	return returned
}

func (api *fakeIntegrationAPI) snapshot() (ops, created, deleted []string, updates []map[string]interface{}) {
	api.mu.Lock()
	defer api.mu.Unlock()
	return append([]string{}, api.ops...), append([]string{}, api.created...), append([]string{}, api.deleted...), append([]map[string]interface{}{}, api.updates...)
}

// importAndRead runs what `terraform import` runs inside the provider: the
// import, then the read that follows it, starting from an empty state.
func importAndRead(t *testing.T, api *fakeIntegrationAPI, id string) IntegrationModel {
	t.Helper()
	ctx := context.Background()

	r := &IntegrationResource{qlient: api}

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	empty := tfsdk.State{
		Schema: schemaResp.Schema,
		Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil),
	}

	importResp := &resource.ImportStateResponse{State: empty}
	r.ImportState(ctx, resource.ImportStateRequest{ID: id}, importResp)
	require.False(t, importResp.Diagnostics.HasError(), "import: %v", importResp.Diagnostics)

	readResp := &resource.ReadResponse{State: importResp.State}
	r.Read(ctx, resource.ReadRequest{State: importResp.State}, readResp)
	require.False(t, readResp.Diagnostics.HasError(), "read: %v", readResp.Diagnostics)

	var model IntegrationModel
	require.False(t, readResp.State.Get(ctx, &model).HasError())
	return model
}

func existingIntegration() map[string]interface{} {
	return map[string]interface{}{
		"id":                      "existing-1",
		"name":                    "aws-production",
		"pollingInterval":         "ONE_DAY",
		"integrationDefinitionId": "7a669809-6e55-45b9-bf23-aa27613118e9",
		"description":             "Production AWS account",
		"resourceGroupId":         "rg-123456",
		"config": map[string]interface{}{
			"roleArn":   "arn:aws:iam::123456789012:role/jupiterone",
			"accessKey": "not-a-real-secret",
		},
		"pollingIntervalCronExpression": map[string]interface{}{"hour": 0, "dayOfWeek": 0},
	}
}

// AC-1.1: importing an existing integration by id puts its attributes into
// state as the API returns them.
func TestIntegrationImport_FillsState(t *testing.T) {
	api := newFakeIntegrationAPI("accessKey")
	api.instances["existing-1"] = existingIntegration()

	model := importAndRead(t, api, "existing-1")

	assert.Equal(t, "existing-1", model.Id.ValueString())
	assert.Equal(t, "aws-production", model.Name.ValueString())
	assert.Equal(t, "ONE_DAY", model.PollingInterval.ValueString())
	assert.Equal(t, "7a669809-6e55-45b9-bf23-aa27613118e9", model.IntegrationDefinitionId.ValueString())
	assert.Equal(t, "Production AWS account", model.Description.ValueString())
	assert.Equal(t, "rg-123456", model.ResourceGroupId.ValueString())
	assert.JSONEq(t, `{"roleArn":"arn:aws:iam::123456789012:role/jupiterone","accessKey":"***masked***"}`, model.Config.ValueString(),
		"config is read after an import, with secret values as the API masks them")
}

// AC-1.2: importing makes no write call to the API.
func TestIntegrationImport_WritesNothing(t *testing.T) {
	api := newFakeIntegrationAPI("accessKey")
	api.instances["existing-1"] = existingIntegration()

	importAndRead(t, api, "existing-1")

	ops, created, deleted, updates := api.snapshot()
	assert.Equal(t, []string{"GetIntegrationInstance"}, ops)
	assert.Empty(t, created)
	assert.Empty(t, deleted)
	assert.Empty(t, updates)
}

// AC-2.1 and AC-2.2: an optional attribute the API has no value for reads back
// null, which is what a configuration that omits it holds.
func TestIntegrationRead_EmptyOptionalStringsAreNull(t *testing.T) {
	api := newFakeIntegrationAPI()
	instance := existingIntegration()
	instance["description"] = ""
	instance["resourceGroupId"] = ""
	api.instances["existing-1"] = instance

	model := importAndRead(t, api, "existing-1")

	assert.True(t, model.Description.IsNull(), "description")
	assert.True(t, model.ResourceGroupId.IsNull(), "resource_group_id")
}

func TestOptionalStringFromAPI(t *testing.T) {
	assert.True(t, optionalStringFromAPI("", types.StringNull()).IsNull())
	assert.Equal(t, types.StringValue(""), optionalStringFromAPI("", types.StringValue("")),
		"an explicit empty string already in state is kept")
	assert.Equal(t, types.StringValue("x"), optionalStringFromAPI("x", types.StringNull()))
	assert.Equal(t, types.StringValue(""), optionalStringFromAPI("", types.StringValue("was set")),
		"a value cleared outside Terraform is reported, not hidden")
}

// AC-2.3: a configured cron of hour 0 on day 0 is still in state after a refresh.
func TestCronExpressionFromAPI(t *testing.T) {
	testCases := []struct {
		name      string
		hour      int
		dayOfWeek int
		inState   types.String
		expected  types.String
	}{
		{
			name:     "midnight on Sunday in state is kept",
			inState:  types.StringValue(`{"dayOfWeek":0,"hour":0}`),
			expected: types.StringValue(`{"dayOfWeek":0,"hour":0}`),
		},
		{
			name:     "no cron in state and 0/0 from the API stays null",
			inState:  types.StringNull(),
			expected: types.StringNull(),
		},
		{
			name: "the string in state is kept when it means the same thing",
			hour: 5, dayOfWeek: 3,
			inState:  types.StringValue(`{ "dayOfWeek": 3, "hour": 5 }`),
			expected: types.StringValue(`{ "dayOfWeek": 3, "hour": 5 }`),
		},
		{
			name: "a cron changed outside Terraform is read",
			hour: 6, dayOfWeek: 3,
			inState:  types.StringValue(`{"dayOfWeek":3,"hour":5}`),
			expected: types.StringValue(`{"hour":6,"dayOfWeek":3}`),
		},
		{
			name: "a cron is read when state has none",
			hour: 5, dayOfWeek: 3,
			inState:  types.StringNull(),
			expected: types.StringValue(`{"hour":5,"dayOfWeek":3}`),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			got, diags := cronExpressionFromAPI(tt.hour, tt.dayOfWeek, tt.inState)
			assert.False(t, diags.HasError())
			assert.Equal(t, tt.expected, got)
		})
	}
}

// AC-2.4: overrides the API returns in a different order from state read back
// in state's order.
func TestIngestionSourcesOverridesFromAPI(t *testing.T) {
	fromAPI := []client.GetIntegrationInstanceIntegrationInstanceIngestionSourcesOverrides{
		{IngestionSourceId: "a", Enabled: true},
		{IngestionSourceId: "b", Enabled: false},
		{IngestionSourceId: "c", Enabled: true},
	}

	t.Run("state order is kept", func(t *testing.T) {
		inState := []IngestionSourceOverride{{IngestionSourceId: "c", Enabled: true}, {IngestionSourceId: "a", Enabled: true}, {IngestionSourceId: "b", Enabled: false}}
		got := ingestionSourcesOverridesFromAPI(&inState, fromAPI)
		require.NotNil(t, got)
		assert.Equal(t, inState, *got)
	})

	t.Run("the API's value wins and new overrides follow, sorted", func(t *testing.T) {
		inState := []IngestionSourceOverride{{IngestionSourceId: "b", Enabled: true}}
		got := ingestionSourcesOverridesFromAPI(&inState, fromAPI)
		require.NotNil(t, got)
		assert.Equal(t, []IngestionSourceOverride{
			{IngestionSourceId: "b", Enabled: false},
			{IngestionSourceId: "a", Enabled: true},
			{IngestionSourceId: "c", Enabled: true},
		}, *got)
	})

	t.Run("an override removed outside Terraform is dropped", func(t *testing.T) {
		inState := []IngestionSourceOverride{{IngestionSourceId: "gone", Enabled: true}, {IngestionSourceId: "a", Enabled: true}}
		got := ingestionSourcesOverridesFromAPI(&inState, fromAPI[:1])
		require.NotNil(t, got)
		assert.Equal(t, []IngestionSourceOverride{{IngestionSourceId: "a", Enabled: true}}, *got)
	})

	t.Run("none from the API is null", func(t *testing.T) {
		inState := []IngestionSourceOverride{{IngestionSourceId: "a", Enabled: true}}
		assert.Nil(t, ingestionSourcesOverridesFromAPI(&inState, nil))
	})
}

// AC-3.1: config is marked sensitive in the resource schema.
func TestIntegrationSchema_ConfigIsSensitive(t *testing.T) {
	r := &IntegrationResource{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, resp)

	assert.True(t, resp.Schema.Attributes["config"].IsSensitive())
}

// AC-1.3, for an integration that has been imported and not yet applied (no
// hash of a sent config): a key state holds as masked plans no change,
// whatever the configuration sets it to, and the plan warns. Anything else
// still plans.
func TestIntegrationMaskedConfigModifier(t *testing.T) {
	masked := `{"accessKey":"***masked***","roleArn":"arn:aws:iam::123456789012:role/jupiterone"}`

	testCases := []struct {
		name          string
		state         types.String
		config        string
		keepsStateVal bool
	}{
		{
			name:          "configuration sets the masked secret",
			state:         types.StringValue(masked),
			config:        `{"roleArn":"arn:aws:iam::123456789012:role/jupiterone","accessKey":"the-real-secret"}`,
			keepsStateVal: true,
		},
		{
			name:   "a non-secret value changed",
			state:  types.StringValue(masked),
			config: `{"roleArn":"arn:aws:iam::123456789012:role/other","accessKey":"the-real-secret"}`,
		},
		{
			name:   "a key was added",
			state:  types.StringValue(masked),
			config: `{"roleArn":"arn:aws:iam::123456789012:role/jupiterone","accessKey":"the-real-secret","extra":true}`,
		},
		{
			name:   "a key was removed",
			state:  types.StringValue(masked),
			config: `{"roleArn":"arn:aws:iam::123456789012:role/jupiterone"}`,
		},
		{
			name:   "state with no masked value is left to plan as before",
			state:  types.StringValue(`{"accessKey":"old-secret"}`),
			config: `{"accessKey":"new-secret"}`,
		},
		{
			name:   "nothing in state",
			state:  types.StringNull(),
			config: `{"accessKey":"the-real-secret"}`,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			planned := types.StringValue(tt.config)
			req := planmodifier.StringRequest{ConfigValue: planned, PlanValue: planned, StateValue: tt.state}
			resp := &planmodifier.StringResponse{PlanValue: planned}

			integrationMaskedConfigModifier{}.PlanModifyString(context.Background(), req, resp)

			assert.False(t, resp.Diagnostics.HasError())
			if tt.keepsStateVal {
				assert.Equal(t, tt.state, resp.PlanValue)
				assert.Equal(t, 1, resp.Diagnostics.WarningsCount(), "a change that cannot be detected must warn")
			} else {
				assert.Equal(t, planned, resp.PlanValue)
				assert.Equal(t, 0, resp.Diagnostics.WarningsCount())
			}
		})
	}
}

func TestEqualIgnoringMasked_Nested(t *testing.T) {
	var inState, planned interface{}
	require.NoError(t, json.Unmarshal([]byte(`{"auth":{"token":"***masked***","user":"svc"},"regions":["us","eu"]}`), &inState))

	require.NoError(t, json.Unmarshal([]byte(`{"auth":{"token":"real","user":"svc"},"regions":["us","eu"]}`), &planned))
	assert.True(t, equalIgnoringMasked(inState, planned))

	require.NoError(t, json.Unmarshal([]byte(`{"auth":{"token":"real","user":"other"},"regions":["us","eu"]}`), &planned))
	assert.False(t, equalIgnoringMasked(inState, planned))

	require.NoError(t, json.Unmarshal([]byte(`{"auth":{"token":"real","user":"svc"},"regions":["us"]}`), &planned))
	assert.False(t, equalIgnoringMasked(inState, planned))
}

func testIntegrationImportedConfig(name string) string {
	return fmt.Sprintf(`
resource "jupiterone_integration" "test" {
  name                      = %q
  integration_definition_id = "7a669809-6e55-45b9-bf23-aa27613118e9"
  polling_interval          = "ONE_DAY"

  config = jsonencode({
    roleArn   = "arn:aws:iam::123456789012:role/jupiterone"
    accessKey = "the-real-secret"
  })

  ingestion_sources_overrides = [
    {
      ingestion_source_id = "fetch-acm-certificates"
      enabled             = true
    },
    {
      ingestion_source_id = "fetch-s3-buckets"
      enabled             = false
    },
  ]
}
`, name)
}

// AC-1.3, AC-1.4, AC-4.1 and AC-4.2, through Terraform itself: an existing
// integration is imported, a matching configuration plans no change, a rename
// updates it in place without writing the masked marker back, and destroy
// deletes exactly that instance.
func TestIntegration_ImportThenManage(t *testing.T) {
	api := newFakeIntegrationAPI("accessKey")
	api.instances["existing-1"] = map[string]interface{}{
		"id":                      "existing-1",
		"name":                    "aws-production",
		"pollingInterval":         "ONE_DAY",
		"integrationDefinitionId": "7a669809-6e55-45b9-bf23-aa27613118e9",
		"description":             "",
		"resourceGroupId":         "",
		"config": map[string]interface{}{
			"roleArn":   "arn:aws:iam::123456789012:role/jupiterone",
			"accessKey": "the-real-secret",
		},
		"pollingIntervalCronExpression": map[string]interface{}{"hour": 0, "dayOfWeek": 0},
		"ingestionSourcesOverrides": []interface{}{
			map[string]interface{}{"ingestionSourceId": "fetch-s3-buckets", "enabled": false},
			map[string]interface{}{"ingestionSourceId": "fetch-acm-certificates", "enabled": true},
		},
	}

	resourceName := "jupiterone_integration.test"

	acctest.Test(t, acctest.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(api),
		CheckDestroy: func(*terraform.State) error {
			_, created, deleted, _ := api.snapshot()
			if len(created) != 0 {
				return fmt.Errorf("expected nothing to be created, got %v", created)
			}
			if len(deleted) != 1 || deleted[0] != "existing-1" {
				return fmt.Errorf("expected destroy to delete exactly existing-1, got %v", deleted)
			}
			return nil
		},
		Steps: []acctest.TestStep{
			{
				Config:             testIntegrationImportedConfig("aws-production"),
				ResourceName:       resourceName,
				ImportState:        true,
				ImportStateId:      "existing-1",
				ImportStatePersist: true,
			},
			{
				Config:   testIntegrationImportedConfig("aws-production"),
				PlanOnly: true,
			},
			{
				PreConfig: func() {
					ops, _, _, _ := api.snapshot()
					for _, op := range ops {
						if op != "GetIntegrationInstance" {
							t.Errorf("import and plan must only read, but called %s", op)
						}
					}
				},
				Config: testIntegrationImportedConfig("aws-production-renamed"),
				ConfigPlanChecks: acctest.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
				Check: acctest.ComposeTestCheckFunc(
					acctest.TestCheckResourceAttr(resourceName, "id", "existing-1"),
					acctest.TestCheckResourceAttr(resourceName, "name", "aws-production-renamed"),
					func(*terraform.State) error {
						_, _, _, updates := api.snapshot()
						if len(updates) != 1 {
							return fmt.Errorf("expected one update, got %d", len(updates))
						}
						config, _ := updates[0]["config"].(map[string]interface{})
						if config["accessKey"] != "the-real-secret" {
							return fmt.Errorf("the update must send the configured secret, not the masked marker; sent %v", config["accessKey"])
						}
						return nil
					},
				),
			},
			{
				Config:   testIntegrationImportedConfig("aws-production-renamed"),
				PlanOnly: true,
			},
		},
	})
}

func testIntegrationCreatedConfig(definitionID, secret string) string {
	return fmt.Sprintf(`
resource "jupiterone_integration" "test" {
  name                      = "tf-managed"
  integration_definition_id = %q
  polling_interval          = "ONE_WEEK"

  polling_interval_cron_expression = jsonencode({
    hour      = 0
    dayOfWeek = 0
  })

  config = jsonencode({
    apiToken = %q
  })

  ingestion_sources_overrides = [
    {
      ingestion_source_id = "zeta"
      enabled             = true
    },
    {
      ingestion_source_id = "alpha"
      enabled             = false
    },
  ]
}
`, definitionID, secret)
}

// AC-2.1 to AC-2.4, AC-3.2 and AC-4.3, through Terraform itself: an integration
// created with no description or resource group, a midnight-Sunday cron and
// overrides in the caller's order plans no change after a refresh; a changed
// secret that state holds unmasked updates in place; a changed definition id
// still replaces.
func TestIntegration_RefreshDoesNotDrift(t *testing.T) {
	api := newFakeIntegrationAPI()

	resourceName := "jupiterone_integration.test"
	firstDefinition := "7a669809-6e55-45b9-bf23-aa27613118e9"
	secondDefinition := "8013680b-311a-4c2e-b53b-c8735fd97a5c"

	acctest.Test(t, acctest.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(api),
		Steps: []acctest.TestStep{
			{
				Config: testIntegrationCreatedConfig(firstDefinition, "first-secret"),
				Check: acctest.ComposeTestCheckFunc(
					acctest.TestCheckResourceAttr(resourceName, "id", "created-1"),
					acctest.TestCheckNoResourceAttr(resourceName, "description"),
					acctest.TestCheckNoResourceAttr(resourceName, "resource_group_id"),
					acctest.TestCheckResourceAttr(resourceName, "polling_interval_cron_expression", `{"dayOfWeek":0,"hour":0}`),
					acctest.TestCheckResourceAttr(resourceName, "ingestion_sources_overrides.0.ingestion_source_id", "zeta"),
					acctest.TestCheckResourceAttr(resourceName, "ingestion_sources_overrides.1.ingestion_source_id", "alpha"),
				),
			},
			{
				Config:   testIntegrationCreatedConfig(firstDefinition, "first-secret"),
				PlanOnly: true,
			},
			{
				Config: testIntegrationCreatedConfig(firstDefinition, "rotated-secret"),
				ConfigPlanChecks: acctest.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
				Check: acctest.TestCheckResourceAttr(resourceName, "id", "created-1"),
			},
			{
				Config: testIntegrationCreatedConfig(secondDefinition, "rotated-secret"),
				ConfigPlanChecks: acctest.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: acctest.TestCheckResourceAttr(resourceName, "id", "created-2"),
			},
		},
	})
}

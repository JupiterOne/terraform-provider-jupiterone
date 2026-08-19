package jupiterone

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These cover the CCM validation and conversion logic that has no business
// reaching the API to be checked. They need no credentials and no cassettes.

func TestMitreTechniquePattern(t *testing.T) {
	valid := []string{"T1078", "T1059.001", "T0001", "T9999.999"}
	for _, v := range valid {
		assert.True(t, mitreTechniquePattern.MatchString(v), "expected %q to be valid", v)
	}

	invalid := []string{
		"",              // empty, which is why clearing must send null
		"TA0001",        // a tactic, not a technique
		"T107",          // too few digits
		"T10788",        // too many digits
		"t1078",         // lower case
		"T1059.1",       // sub-technique too short
		"T1059.0011",    // sub-technique too long
		"T1078 ",        // trailing space
		"T1078.001.002", /* doubly nested */
	}
	for _, v := range invalid {
		assert.False(t, mitreTechniquePattern.MatchString(v), "expected %q to be invalid", v)
	}
}

func TestRFC3339Validator(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name    string
		value   types.String
		wantErr bool
	}{
		{name: "utc", value: types.StringValue("2030-01-31T00:00:00Z")},
		{name: "offset", value: types.StringValue("2030-01-31T00:00:00+01:00")},
		{name: "fractional", value: types.StringValue("2030-01-31T00:00:00.500Z")},
		{name: "null is left to the required check", value: types.StringNull()},
		{name: "unknown is deferred", value: types.StringUnknown()},
		{name: "date only", value: types.StringValue("2030-01-31"), wantErr: true},
		{name: "no zone", value: types.StringValue("2030-01-31T00:00:00"), wantErr: true},
		{name: "epoch millis", value: types.StringValue("1893456000000"), wantErr: true},
		{name: "nonsense", value: types.StringValue("next tuesday"), wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &validator.StringResponse{}
			rfc3339Validator{}.ValidateString(ctx, validator.StringRequest{
				Path:        path.Root("expires_on"),
				ConfigValue: tc.value,
			}, resp)

			assert.Equal(t, tc.wantErr, resp.Diagnostics.HasError())
		})
	}
}

func TestExpiresOnRoundTrip(t *testing.T) {
	// A whole second renders without a fractional part.
	ms, err := rfc3339ToEpochMillis("2030-01-31T00:00:00Z")
	require.NoError(t, err)
	assert.Equal(t, "2030-01-31T00:00:00Z", epochMillisToRFC3339(ms))

	// Millisecond precision survives the round trip.
	ms, err = rfc3339ToEpochMillis("2030-01-31T00:00:00.5Z")
	require.NoError(t, err)
	assert.Equal(t, "2030-01-31T00:00:00.500Z", epochMillisToRFC3339(ms))

	_, err = rfc3339ToEpochMillis("not a timestamp")
	assert.Error(t, err)
}

// TestSameInstant is the guard against a permanent diff: a configured value
// carrying a UTC offset refers to the same moment as its Z-normalised
// equivalent, so reading state must not rewrite it.
func TestSameInstant(t *testing.T) {
	utcMillis, err := rfc3339ToEpochMillis("2030-01-30T23:00:00Z")
	require.NoError(t, err)

	assert.True(t, sameInstant("2030-01-31T00:00:00+01:00", utcMillis),
		"an offset timestamp naming the same instant must compare equal")
	assert.True(t, sameInstant("2030-01-30T23:00:00Z", utcMillis))

	assert.False(t, sameInstant("2030-01-31T00:00:00Z", utcMillis),
		"a genuinely different instant must be detected as drift")
	assert.False(t, sameInstant("", utcMillis))
	assert.False(t, sameInstant("garbage", utcMillis))
}

func TestOptionalString(t *testing.T) {
	assert.Nil(t, optionalString(types.StringNull()))
	assert.Nil(t, optionalString(types.StringUnknown()))
	assert.Nil(t, optionalString(types.StringValue("")),
		"an empty string must become null, since owner is validated as an email")

	v := optionalString(types.StringValue("someone@jupiterone.com"))
	require.NotNil(t, v)
	assert.Equal(t, "someone@jupiterone.com", *v)
}

func TestOptionalBool(t *testing.T) {
	assert.Nil(t, optionalBool(types.BoolNull()))
	assert.Nil(t, optionalBool(types.BoolUnknown()))

	// false is a meaningful filter and must survive as an explicit value.
	f := optionalBool(types.BoolValue(false))
	require.NotNil(t, f)
	assert.False(t, *f)

	tr := optionalBool(types.BoolValue(true))
	require.NotNil(t, tr)
	assert.True(t, *tr)
}

func TestStringSlice(t *testing.T) {
	ctx := context.Background()

	nullList, diags := stringSlice(ctx, types.ListNull(types.StringType))
	assert.False(t, diags.HasError())
	assert.Nil(t, nullList, "an unset list must be omitted from the request, not sent empty")

	list, d := types.ListValueFrom(ctx, types.StringType, []string{"PASS", "FAIL"})
	require.False(t, d.HasError())

	values, diags := stringSlice(ctx, list)
	assert.False(t, diags.HasError())
	assert.Equal(t, []string{"PASS", "FAIL"}, values)
}

// TestCCMDataSourceSchemas checks each CCM data source schema against the Go
// model that populates it. A mismatch between a tfsdk tag and a schema
// attribute otherwise only surfaces when Read runs against a live API, so this
// catches it without credentials.
//
// It matters most for jupiterone_control_frameworks, which has no acceptance
// test: the controlFrameworks query cannot be filtered, so recording it would
// embed the whole account in a cassette.
func TestCCMDataSourceSchemas(t *testing.T) {
	ctx := context.Background()

	dataSources := map[string]datasource.DataSource{
		"jupiterone_control":                 NewControlDataSource(),
		"jupiterone_controls":                NewControlsDataSource(),
		"jupiterone_control_test":            NewControlTestDataSource(),
		"jupiterone_control_tests":           NewControlTestsDataSource(),
		"jupiterone_control_frameworks":      NewControlFrameworksDataSource(),
		"jupiterone_control_framework_stats": NewControlFrameworkStatsDataSource(),
		"jupiterone_attestations":            NewAttestationsDataSource(),
	}

	for name, ds := range dataSources {
		t.Run(name, func(t *testing.T) {
			resp := &datasource.SchemaResponse{}
			ds.Schema(ctx, datasource.SchemaRequest{}, resp)

			require.False(t, resp.Diagnostics.HasError(), "schema produced errors: %v", resp.Diagnostics)
			assert.NotEmpty(t, resp.Schema.Attributes)

			// terraform-plugin-testing requires an id on everything it puts in
			// state, including list data sources.
			_, hasId := resp.Schema.Attributes["id"]
			assert.True(t, hasId, "data source must expose an id attribute")
		})
	}
}

// TestCCMNestedObjectModelsMatchSchemas converts a zero-valued model into each
// nested object type. Conversion fails loudly when the model's tfsdk tags and
// the schema attributes disagree.
func TestCCMNestedObjectModelsMatchSchemas(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name    string
		objType attr.Type
		model   any
	}{
		// Nested list fields get a typed null rather than the Go zero value: a
		// zero types.List carries no element type and would fail conversion for
		// reasons unrelated to whether the tags match.
		{"control", controlItemObject().Type(), []controlDataSourceItemModel{{
			FrameworkIds: types.ListNull(types.StringType),
		}}},
		{"control_test", controlTestItemObject().Type(), []controlTestDataSourceItemModel{{
			Queries: types.ListNull(controlTestQueryObject().Type()),
		}}},
		{"control_test_query", controlTestQueryObject().Type(), []controlTestQueryStateModel{{}}},
		{"control_framework", controlFrameworkObject().Type(), []controlFrameworkStateModel{{
			Requirements: types.ListNull(controlFrameworkRequirementObject().Type()),
		}}},
		{"framework_requirement", controlFrameworkRequirementObject().Type(), []controlFrameworkRequirementStateModel{{}}},
		{"framework_stats", controlFrameworkStatsObject().Type(), []controlFrameworkStatsStateModel{{
			SectionStats: types.ListNull(controlFrameworkSectionStatsObject().Type()),
		}}},
		{"framework_section_stats", controlFrameworkSectionStatsObject().Type(), []controlFrameworkSectionStatsStateModel{{}}},
		{"attestation", attestationObject().Type(), []attestationStateModel{{}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, diags := types.ListValueFrom(ctx, tc.objType, tc.model)
			assert.False(t, diags.HasError(),
				"model does not match schema: %v", diags)
		})
	}
}

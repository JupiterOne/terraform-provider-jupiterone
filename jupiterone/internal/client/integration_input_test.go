package client

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// When the integration resource updates an instance it does not set UpdateMode.
// The field is an enum (UpdateConfigMode) whose only valid values are MERGE and
// PARTIAL_REPLACE, so the empty zero value must be omitted from the request or
// the API rejects it with:
//
//	Value "" does not exist in "UpdateConfigMode" enum.
//
// Regression test for the v1.18.0 breakage of jupiterone_integration updates.
func TestUpdateIntegrationInstanceInputOmitsEmptyUpdateMode(t *testing.T) {
	input := UpdateIntegrationInstanceInput{
		Name:            "example",
		PollingInterval: IntegrationPollingIntervalThirtyMinutes,
		Description:     "example",
		Config:          map[string]interface{}{"foo": "bar"},
	}

	b, err := json.Marshal(input)
	assert.NoError(t, err)

	assert.NotContains(t, string(b), `"updateMode":""`,
		"an unset updateMode must be omitted so the API does not reject the empty enum value")
}

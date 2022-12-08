package jupiterone

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func removeCRFromString(s string) string {
	return strings.ReplaceAll(s, "\r", "")
}

func interfaceSliceToStringSlice(l []interface{}) []string {
	ret := make([]string, len(l))

	for i, tag := range l {
		ret[i] = tag.(string)
	}

	return ret
}

func jsonDiffSuppressFunc(k, oldValue, newValue string, d *schema.ResourceData) bool {
	var old, new interface{}
	// Errors during json.Unmarshal likely mean that newValue is empty or deleted.
	// In this case we shouldn't suppress the diff and let downstream code
	// handle the changes and updates.
	err := json.Unmarshal([]byte(oldValue), &old)
	if err != nil {
		return false
	}

	err = json.Unmarshal([]byte(newValue), &new)
	if err != nil {
		return false
	}

	return reflect.DeepEqual(old, new)
}

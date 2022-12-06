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
	err := json.Unmarshal([]byte(oldValue), &old)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal([]byte(newValue), &new)
	if err != nil {
		panic(err)
	}

	return reflect.DeepEqual(old, new)
}

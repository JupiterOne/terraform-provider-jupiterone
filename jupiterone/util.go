package jupiterone

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
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

func queriesToMapStringInterface(c []client.QuestionQuery) ([]map[string]interface{}, error) {
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	err := enc.Encode(c)
	if err != nil {
		return nil, err
	}

	msi := []map[string]interface{}{}
	err = json.Unmarshal(buf.Bytes(), &msi)
	if err != nil {
		return nil, err
	}
	return msi, nil
}

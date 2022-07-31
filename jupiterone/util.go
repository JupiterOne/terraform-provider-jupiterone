package jupiterone

import "strings"

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

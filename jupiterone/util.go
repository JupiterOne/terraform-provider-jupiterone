package jupiterone

import "strings"

func removeCRFromString(s string) string {
	return strings.ReplaceAll(s, "\r", "")
}

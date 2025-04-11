package utils

import (
	"fmt"
	"strings"
)

// PrettyPrintMap takes a map and returns it formatted as a nice string
func PrettyPrintMap(data map[string]interface{}) string {
	var sb strings.Builder
	for k, v := range data {
		sb.WriteString(fmt.Sprintf("%s: %v\n", k, v))
	}
	return sb.String()
}

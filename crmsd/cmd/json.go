package cmd

import (
	"strings"
	"encoding/json"
)

func isJsonKind() bool {
	return strings.HasPrefix(globalFlags.OutputFormat, OutFormat_JSON)
}

func toJson(o interface{}) ([]byte, error) {
	switch globalFlags.OutputFormat {
	case OutFormat_JSON:
		return json.MarshalIndent(o, "", "  ")
	default: // case OutFormat_JSON_Compact:
		return json.Marshal(o)
	}
}

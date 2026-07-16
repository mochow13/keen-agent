package llm

import (
	"bytes"
	"encoding/json"
	"strings"
)

func serializeToolOutput(output any) string {
	if output == nil {
		output = map[string]any{}
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(output); err != nil {
		return "{}"
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

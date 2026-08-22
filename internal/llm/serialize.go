package llm

import (
	"bytes"
	"encoding/json"
	"strings"
)

func serializeJSON(v any) string {
	if v == nil {
		v = map[string]any{}
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		return "{}"
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

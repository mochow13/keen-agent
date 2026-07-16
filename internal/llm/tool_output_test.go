package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSerializeToolOutputPreservesHTMLCharacters(t *testing.T) {
	content := `if a < b && b > c { println("ok") }`
	got := serializeToolOutput(map[string]any{"content": content})

	if !json.Valid([]byte(got)) {
		t.Fatalf("serializeToolOutput() returned invalid JSON: %q", got)
	}
	for _, escaped := range []string{`\u003c`, `\u003e`, `\u0026`} {
		if strings.Contains(got, escaped) {
			t.Fatalf("serializeToolOutput() contains HTML escape %q: %s", escaped, got)
		}
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if decoded["content"] != content {
		t.Fatalf("content = %q, want %q", decoded["content"], content)
	}
}

func TestSerializeToolOutputNilAndUnsupportedValues(t *testing.T) {
	if got := serializeToolOutput(nil); got != "{}" {
		t.Fatalf("serializeToolOutput(nil) = %q, want %q", got, "{}")
	}
	if got := serializeToolOutput(make(chan int)); got != "{}" {
		t.Fatalf("serializeToolOutput(channel) = %q, want %q", got, "{}")
	}
}

package subagents

import (
	"strings"
	"testing"
)

func TestParseProfileMinimal(t *testing.T) {
	profile, warnings, err := ParseProfile("explorer.md", []byte(`---
name: explorer
description: Explores code.
---

Explore with focus.
`))
	if err != nil {
		t.Fatalf("ParseProfile returned error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if profile.Name != "explorer" {
		t.Fatalf("expected name explorer, got %q", profile.Name)
	}
	if profile.Description != "Explores code." {
		t.Fatalf("unexpected description %q", profile.Description)
	}
	if profile.Instructions != "Explore with focus." {
		t.Fatalf("unexpected instructions %q", profile.Instructions)
	}
	if len(profile.Tools) != 0 {
		t.Fatalf("expected tools to inherit by omission, got %v", profile.Tools)
	}
}

func TestParseProfileRequiresName(t *testing.T) {
	_, _, err := ParseProfile("missing.md", []byte(`---
description: Explores code.
---
Body.
`))
	if err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("expected missing name error, got %v", err)
	}
}

func TestParseProfileRequiresDescription(t *testing.T) {
	_, _, err := ParseProfile("missing.md", []byte(`---
name: explorer
---
Body.
`))
	if err == nil || !strings.Contains(err.Error(), "description") {
		t.Fatalf("expected missing description error, got %v", err)
	}
}

func TestParseProfileWarnsUnknownFields(t *testing.T) {
	_, warnings, err := ParseProfile("explorer.md", []byte(`---
name: explorer
description: Explores code.
color: blue
---
Body.
`))
	if err != nil {
		t.Fatalf("ParseProfile returned error: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "color") {
		t.Fatalf("expected color warning, got %v", warnings)
	}
}

func TestParseProfileRequiresFrontmatter(t *testing.T) {
	_, _, err := ParseProfile("missing.md", []byte("Body only."))
	if err == nil || !strings.Contains(err.Error(), "frontmatter") {
		t.Fatalf("expected missing frontmatter error, got %v", err)
	}
}

func TestParseProfileRequiresClosingFrontmatterDelimiter(t *testing.T) {
	_, _, err := ParseProfile("broken.md", []byte(`---
name: explorer
description: Explores code.
`))
	if err == nil || !strings.Contains(err.Error(), "closing frontmatter") {
		t.Fatalf("expected missing closing delimiter error, got %v", err)
	}
}

func TestParseProfileParsesOptionalFields(t *testing.T) {
	profile, warnings, err := ParseProfile("reviewer.md", []byte(`---
name: reviewer
description: Reviews code.
tools:
  - grep
  - ""
provider: anthropic
model: claude
thinking_effort: high
timeout_seconds: 30
hidden: true
---
Review with focus.
`))
	if err != nil {
		t.Fatalf("ParseProfile returned error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if profile.Provider != "anthropic" || profile.Model != "claude" || profile.ThinkingEffort != "high" {
		t.Fatalf("unexpected model fields: %+v", profile)
	}
	if profile.TimeoutSeconds != 30 || !profile.Hidden {
		t.Fatalf("unexpected runtime fields: %+v", profile)
	}
	if len(profile.Tools) != 1 || profile.Tools[0] != "grep" {
		t.Fatalf("expected trimmed tools, got %v", profile.Tools)
	}
}

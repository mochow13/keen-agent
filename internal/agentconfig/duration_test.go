package agentconfig

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestDuration_UnmarshalYAML(t *testing.T) {
	var d Duration
	if err := yaml.Unmarshal([]byte("90s"), &d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Std() != 90*time.Second {
		t.Errorf("expected 90s, got %v", d.Std())
	}
}

func TestDuration_UnmarshalYAML_Invalid(t *testing.T) {
	var d Duration
	if err := yaml.Unmarshal([]byte("not-a-duration"), &d); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDuration_MarshalYAML(t *testing.T) {
	d := Duration(5 * time.Minute)
	out, err := yaml.Marshal(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "5m0s\n" {
		t.Errorf("expected 5m0s, got %q", string(out))
	}
}

package tools

import (
	"context"
	"testing"
)

func TestRegistryWithout_RemovesNamedToolsFromCopy(t *testing.T) {
	registry := NewRegistry()
	for _, name := range []string{"read_file", "write_file", "edit_file", "bash"} {
		if err := registry.Register(&dummyRegistryTool{name: name}); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}

	filtered := registry.Without("write_file", "edit_file")

	if filtered.Count() != 2 {
		t.Fatalf("expected 2 tools, got %d", filtered.Count())
	}
	for _, name := range []string{"read_file", "bash"} {
		if _, ok := filtered.Get(name); !ok {
			t.Fatalf("expected %s to remain", name)
		}
	}
	for _, name := range []string{"write_file", "edit_file"} {
		if _, ok := filtered.Get(name); ok {
			t.Fatalf("expected %s to be removed", name)
		}
		if _, ok := registry.Get(name); !ok {
			t.Fatalf("expected original registry to keep %s", name)
		}
	}
}

func TestRegistryAll_ReturnsToolsSortedByName(t *testing.T) {
	registry := NewRegistry()
	for _, name := range []string{"write_file", "bash", "read_file", "edit_file"} {
		if err := registry.Register(&dummyRegistryTool{name: name}); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}

	got := registry.All()
	want := []string{"bash", "edit_file", "read_file", "write_file"}
	if len(got) != len(want) {
		t.Fatalf("expected %d tools, got %d", len(want), len(got))
	}
	for i, tool := range got {
		if tool.Name() != want[i] {
			t.Fatalf("tool %d: expected %q, got %q", i, want[i], tool.Name())
		}
	}
}

type dummyRegistryTool struct {
	name        string
	description string
	schema      map[string]any
	executed    bool
}

func (d *dummyRegistryTool) Name() string { return d.name }

func (d *dummyRegistryTool) Description() string { return d.description }

func (d *dummyRegistryTool) InputSchema() map[string]any { return d.schema }

func (d *dummyRegistryTool) Execute(ctx context.Context, input any) (any, error) {
	d.executed = true
	return map[string]any{"executed": true}, nil
}

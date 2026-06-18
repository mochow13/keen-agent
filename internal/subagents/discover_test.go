package subagents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverAndLoadMetadata(t *testing.T) {
	workingDir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectDir := filepath.Join(workingDir, ".keen-agent", "agents")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("create project agents dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "reviewer.md"), []byte(`---
name: reviewer
description: Reviews code.
tools:
  - grep
---
Review with focus.
`), 0o644); err != nil {
		t.Fatalf("write project profile: %v", err)
	}

	bundledDir := filepath.Join(t.TempDir(), "bundled")
	if err := os.MkdirAll(bundledDir, 0o755); err != nil {
		t.Fatalf("create bundled dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundledDir, "explorer.md"), []byte(`---
name: explorer
description: Explores code.
---
Explore with focus.
`), 0o644); err != nil {
		t.Fatalf("write bundled profile: %v", err)
	}

	discovery := Discover(workingDir, bundledDir)
	if len(discovery.Profiles) != 2 {
		t.Fatalf("expected 2 discovered profiles, got %d: %+v", len(discovery.Profiles), discovery.Profiles)
	}

	loaded := LoadMetadata(discovery)
	if len(loaded.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", loaded.Warnings)
	}
	if len(loaded.Profiles) != 2 {
		t.Fatalf("expected 2 loaded profiles, got %d: %+v", len(loaded.Profiles), loaded.Profiles)
	}
	if loaded.Profiles[0].Name != "explorer" || loaded.Profiles[1].Name != "reviewer" {
		t.Fatalf("expected sorted profiles explorer/reviewer, got %+v", loaded.Profiles)
	}
	if loaded.Profiles[1].Instructions != "Review with focus." {
		t.Fatalf("unexpected reviewer instructions: %q", loaded.Profiles[1].Instructions)
	}
}

func TestLoadMetadataWarnsAndSkipsUnreadableProfile(t *testing.T) {
	discovery := Discovery{
		Profiles:  []Profile{{Name: "missing", Description: "missing"}},
		locations: []string{filepath.Join(t.TempDir(), "missing.md")},
	}

	loaded := LoadMetadata(discovery)
	if len(loaded.Profiles) != 0 {
		t.Fatalf("expected missing profile to be skipped, got %+v", loaded.Profiles)
	}
	if len(loaded.Warnings) != 1 || !strings.Contains(loaded.Warnings[0], "failed to load") {
		t.Fatalf("expected failed load warning, got %v", loaded.Warnings)
	}
}

func TestCatalogSkipsHiddenProfiles(t *testing.T) {
	catalog := Catalog([]Profile{
		{Name: "visible", Description: "Visible work."},
		{Name: "hidden", Description: "Hidden work.", Hidden: true},
	})

	if !strings.Contains(catalog, "visible: Visible work.") {
		t.Fatalf("expected visible profile in catalog: %s", catalog)
	}
	if strings.Contains(catalog, "hidden") {
		t.Fatalf("expected hidden profile to be omitted: %s", catalog)
	}
}

func TestCatalogEmptyWhenNoVisibleProfiles(t *testing.T) {
	if got := Catalog([]Profile{{Name: "hidden", Description: "Hidden work.", Hidden: true}}); got != "" {
		t.Fatalf("expected empty catalog, got %q", got)
	}
}

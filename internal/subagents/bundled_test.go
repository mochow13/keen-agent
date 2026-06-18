package subagents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureBundledExtractsProfiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	root, err := EnsureBundled()
	if err != nil {
		t.Fatalf("EnsureBundled returned error: %v", err)
	}
	wantRoot := filepath.Join(home, ".keen-agent", "agents", "bundled")
	if root != wantRoot {
		t.Fatalf("expected root %q, got %q", wantRoot, root)
	}

	data, err := os.ReadFile(filepath.Join(root, "explorer.md"))
	if err != nil {
		t.Fatalf("expected bundled explorer profile: %v", err)
	}
	if !strings.Contains(string(data), "name: explorer") {
		t.Fatalf("bundled explorer missing expected frontmatter: %s", data)
	}
}

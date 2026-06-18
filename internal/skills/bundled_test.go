package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureBundled_OverwritesExistingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	target := filepath.Join(home, ".keen-agent", "skills", "bundled", "commit", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("STALE"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := EnsureBundled(); err != nil {
		t.Fatalf("EnsureBundled error: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "STALE" {
		t.Fatal("expected EnsureBundled to overwrite stale file")
	}
}

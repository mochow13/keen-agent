package filesearch

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSearchEmptyQuery(t *testing.T) {
	s := NewFileSearcher(t.TempDir(), nil)
	if got := s.Search("", 10); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestGitLsFilesHandlesSpecialPaths(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	pathWithNewline := "dir/file\nname.txt"
	pathWithSpaces := " dir/spaced name.txt "
	writeFile(t, dir, pathWithNewline, "newline")
	writeFile(t, dir, pathWithSpaces, "spaces")
	runGit(t, dir, "init")
	runGit(t, dir, "add", pathWithNewline, pathWithSpaces)

	paths, ok := gitLsFiles(dir)
	if !ok {
		t.Fatal("expected git ls-files to succeed")
	}
	if !containsPath(paths, pathWithNewline) {
		t.Fatalf("expected newline path in results, got %q", paths)
	}
	if !containsPath(paths, pathWithSpaces) {
		t.Fatalf("expected space-padded path in results, got %q", paths)
	}
}

func writeFile(t *testing.T, dir, path, content string) {
	t.Helper()
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func containsPath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}

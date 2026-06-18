package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

//go:embed all:bundled
var bundledFS embed.FS

const bundledEmbedRoot = "bundled"

// EnsureBundled extracts embedded skill files into <home>/.keen-agent/skills/bundled,
// overwriting on each call so the bundled set always matches the binary version.
// Returns the absolute bundled root path and any extraction error. Returns
// ("", nil) when the home directory cannot be resolved — bundled skills simply
// won't appear in that case.
func EnsureBundled() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", nil
	}
	root := filepath.Join(home, ".keen-agent", "skills", "bundled")
	if err := os.RemoveAll(root); err != nil {
		return "", fmt.Errorf("clear bundled skills dir: %w", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("create bundled skills dir: %w", err)
	}

	walkErr := fs.WalkDir(bundledFS, bundledEmbedRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == bundledEmbedRoot {
			return nil
		}
		rel := strings.TrimPrefix(p, bundledEmbedRoot+"/")
		target := filepath.Join(root, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := bundledFS.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", p, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if walkErr != nil {
		return root, fmt.Errorf("extract bundled skills: %w", walkErr)
	}
	return root, nil
}

// bundledNames returns the directory names of all embedded skills. Useful for
// tests; not used in production code.
func bundledNames() []string {
	entries, err := bundledFS.ReadDir(bundledEmbedRoot)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := bundledFS.ReadFile(path.Join(bundledEmbedRoot, e.Name(), "SKILL.md")); err != nil {
			continue
		}
		out = append(out, e.Name())
	}
	return out
}

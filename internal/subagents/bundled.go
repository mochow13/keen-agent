package subagents

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed all:bundled
var bundledFS embed.FS

const bundledEmbedRoot = "bundled"

func EnsureBundled() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", nil
	}
	root := filepath.Join(home, ".keen-agent", "agents", "bundled")
	if err := os.RemoveAll(root); err != nil {
		return "", fmt.Errorf("clear bundled subagents dir: %w", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("create bundled subagents dir: %w", err)
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
		return root, fmt.Errorf("extract bundled subagents: %w", walkErr)
	}
	return root, nil
}

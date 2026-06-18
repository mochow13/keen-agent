package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
)

type ToolSet map[string]struct{}

func (ts ToolSet) MarshalJSON() ([]byte, error) {
	names := make([]string, 0, len(ts))
	for name := range ts {
		names = append(names, name)
	}
	slices.Sort(names)
	return json.Marshal(names)
}

func (ts *ToolSet) UnmarshalJSON(data []byte) error {
	var names []string
	if err := json.Unmarshal(data, &names); err != nil {
		return err
	}
	*ts = make(ToolSet, len(names))
	for _, name := range names {
		(*ts)[name] = struct{}{}
	}
	return nil
}

func (ts ToolSet) Contains(name string) bool {
	_, ok := ts[name]
	return ok
}

type ProjectPermissions struct {
	Allow ToolSet `json:"allow"`
}

func NewProjectPermissions() *ProjectPermissions {
	return &ProjectPermissions{
		Allow: make(ToolSet),
	}
}

func projectPermissionsPath(workingDir string) string {
	return filepath.Join(workingDir, ".keen-agent", "permissions.json")
}

func LoadProjectPermissions(workingDir string) (*ProjectPermissions, error) {
	data, err := os.ReadFile(projectPermissionsPath(workingDir))
	if errors.Is(err, os.ErrNotExist) {
		return NewProjectPermissions(), nil
	}
	if err != nil {
		return nil, err
	}
	perms := NewProjectPermissions()
	if err := json.Unmarshal(data, perms); err != nil {
		return nil, err
	}
	return perms, nil
}

func SaveProjectPermissions(workingDir string, perms *ProjectPermissions) error {
	dir := filepath.Join(workingDir, ".keen-agent")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(perms, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(projectPermissionsPath(workingDir), data, 0644)
}

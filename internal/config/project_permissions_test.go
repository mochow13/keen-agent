package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProjectPermissions_MissingFile(t *testing.T) {
	dir := t.TempDir()

	perms, err := LoadProjectPermissions(dir)
	if err != nil {
		t.Fatalf("LoadProjectPermissions() error = %v", err)
	}
	if len(perms.Allow) != 0 {
		t.Fatalf("expected empty allow, got %v", perms.Allow)
	}
}

func TestProjectPermissions_SaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()

	perms := NewProjectPermissions()
	perms.Allow["bash"] = struct{}{}
	perms.Allow["read_file"] = struct{}{}

	if err := SaveProjectPermissions(dir, perms); err != nil {
		t.Fatalf("SaveProjectPermissions() error = %v", err)
	}

	loaded, err := LoadProjectPermissions(dir)
	if err != nil {
		t.Fatalf("LoadProjectPermissions() error = %v", err)
	}

	if !loaded.Allow.Contains("bash") || !loaded.Allow.Contains("read_file") {
		t.Errorf("expected allow={bash, read_file}, got %v", loaded.Allow)
	}
	if loaded.Allow.Contains("write_file") {
		t.Errorf("unexpected entry in allow: %v", loaded.Allow)
	}
}

func TestLoadProjectPermissions_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".keen-agent"), 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	path := filepath.Join(dir, ".keen-agent", "permissions.json")
	if err := os.WriteFile(path, []byte("{not json"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if _, err := LoadProjectPermissions(dir); err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestToolSet_MarshalAsJSONArray(t *testing.T) {
	perms := NewProjectPermissions()
	perms.Allow["bash"] = struct{}{}
	perms.Allow["edit_file"] = struct{}{}

	data, err := json.Marshal(perms)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var raw struct {
		Allow []string `json:"allow"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("expected JSON arrays, got unmarshal error: %v (data=%s)", err, data)
	}
	if len(raw.Allow) != 2 || raw.Allow[0] != "bash" || raw.Allow[1] != "edit_file" {
		t.Errorf("expected sorted [bash, edit_file], got %v", raw.Allow)
	}
}

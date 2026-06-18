package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigEnabledDefaultsTrue(t *testing.T) {
	if !((Config{}).Enabled("demo")) {
		t.Fatal("expected missing config to default enabled")
	}
	cfg := Config{IsEnabled: map[string]bool{"demo": false}}
	if cfg.Enabled("demo") {
		t.Fatal("expected disabled skill")
	}
}

func TestConfigSetStatus(t *testing.T) {
	var cfg Config
	cfg.SetStatus("demo", StatusDisabled)
	if cfg.Enabled("demo") {
		t.Fatal("expected skill disabled")
	}
}

func TestConfigRemoveStatus(t *testing.T) {
	cfg := Config{IsEnabled: map[string]bool{"demo": false}}
	cfg.RemoveStatus("demo")
	if _, ok := cfg.IsEnabled["demo"]; ok {
		t.Fatal("expected skill status removed")
	}
	if !cfg.Enabled("demo") {
		t.Fatal("expected removed skill status to default enabled")
	}
}

func TestLoadSaveConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := Config{IsEnabled: map[string]bool{"demo": false}}
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	loaded := LoadConfig()
	if loaded.Enabled("demo") {
		t.Fatal("expected saved disabled state")
	}
	if _, err := os.Stat(filepath.Join(home, ".keen-agent", "skills", "config.json")); err != nil {
		t.Fatalf("expected config file: %v", err)
	}
}

func TestLoadConfigMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := LoadConfig()
	if !cfg.Enabled("demo") {
		t.Fatal("expected default enabled")
	}
}

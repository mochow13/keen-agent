package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

type Loader struct{}

func NewLoader() *Loader {
	return &Loader{}
}

func (l *Loader) Load() (*GlobalConfig, error) {
	cfg := DefaultGlobalConfig()

	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("config file not found, using defaults")
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config: %w. %s", err, ConfigFixHint)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w. %s", err, ConfigFixHint)
	}

	slog.Debug("config loaded", "provider", cfg.ActiveProvider)
	return cfg, nil
}

func (l *Loader) Save(cfg *GlobalConfig) error {
	if err := os.MkdirAll(ConfigDir(), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cfg); err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	path := ConfigPath()
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	slog.Debug("config saved", "path", path)
	return nil
}

func (l *Loader) Exists() bool {
	_, err := os.Stat(ConfigPath())
	return !os.IsNotExist(err)
}

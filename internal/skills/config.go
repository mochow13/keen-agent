package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Status bool

const (
	StatusDisabled Status = false
	StatusEnabled  Status = true
)

type Config struct {
	IsEnabled map[string]bool `json:"is_enabled"`
}

func LoadConfig() Config {
	path, err := ConfigPath()
	if err != nil {
		return Config{IsEnabled: map[string]bool{}}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{IsEnabled: map[string]bool{}}
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{IsEnabled: map[string]bool{}}
	}
	if cfg.IsEnabled == nil {
		cfg.IsEnabled = map[string]bool{}
	}
	return cfg
}

func SaveConfig(cfg Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if cfg.IsEnabled == nil {
		cfg.IsEnabled = map[string]bool{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".keen-agent", "skills", "config.json"), nil
}

func (c Config) Enabled(name string) bool {
	if c.IsEnabled == nil {
		return true
	}
	enabled, ok := c.IsEnabled[name]
	if !ok {
		return true
	}
	return enabled
}

func (c *Config) SetStatus(name string, status Status) {
	if c.IsEnabled == nil {
		c.IsEnabled = map[string]bool{}
	}
	c.IsEnabled[name] = bool(status)
}

func (c *Config) RemoveStatus(name string) {
	if c.IsEnabled == nil {
		return
	}
	delete(c.IsEnabled, name)
}

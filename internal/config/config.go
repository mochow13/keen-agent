package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	ProviderAnthropic   = "anthropic"
	ProviderOpenAI      = "openai"
	ProviderOpenAICodex = "openai-codex"
	ProviderGoogleAI    = "googleai"
	ProviderMoonshotAI  = "moonshotai"
	ProviderDeepSeek    = "deepseek"
	ProviderZAI         = "zai"
	ProviderMiniMax     = "minimax"
	ProviderOpenCodeGo  = "opencode-go"
	ProviderBedrock     = "amazon-bedrock"
)

const ConfigFixHint = "To fix configs manually, check ~/.keen-agent/configs.json"

type GlobalConfig struct {
	ActiveProvider    string                    `json:"active_provider"`
	ActiveModel       string                    `json:"active_model"`
	ThinkingEffort    string                    `json:"thinking_effort,omitempty"`
	ShowThinking      *bool                     `json:"show_thinking,omitempty"`
	AdversaryProvider string                    `json:"adversary_provider,omitempty"`
	AdversaryModel    string                    `json:"adversary_model,omitempty"`
	Providers         map[string]ProviderConfig `json:"providers"`
}

type ProviderConfig struct {
	Models  []string          `json:"models"`
	APIKey  string            `json:"api_key"`
	BaseURL string            `json:"base_url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

func (p ProviderConfig) hasModel(model string) bool {
	return slices.Contains(p.Models, model)
}

type SessionConfig struct {
	Provider string
	APIKey   string
	Model    string
}

type ResolvedConfig struct {
	Provider       string
	APIKey         string
	Model          string
	ThinkingEffort string
	BaseURL        string
	AuthMode       string
	Headers        map[string]string
}

const (
	AuthModeAPIKey = "api_key"
	AuthModeOAuth  = "oauth"
	AuthModeAWS    = "aws"
)

func (g *GlobalConfig) GetProviderConfig(provider string) (ProviderConfig, bool) {
	cfg, ok := g.Providers[provider]
	return cfg, ok
}

func (g *GlobalConfig) SetProviderConfig(provider string, cfg ProviderConfig) {
	if g.Providers == nil {
		g.Providers = make(map[string]ProviderConfig)
	}
	g.Providers[provider] = cfg
}

func (g *GlobalConfig) AddModel(provider string, model string) {
	if model == "" {
		return
	}
	cfg, ok := g.GetProviderConfig(provider)
	if !ok {
		cfg = ProviderConfig{}
	}
	if slices.Contains(cfg.Models, model) {
		return
	}
	cfg.Models = append(cfg.Models, model)
	g.SetProviderConfig(provider, cfg)
}

func (g *GlobalConfig) GetFirstModel(provider string) string {
	cfg, ok := g.GetProviderConfig(provider)
	if !ok {
		return ""
	}
	if len(cfg.Models) > 0 {
		return cfg.Models[0]
	}
	return ""
}

func Resolve(global *GlobalConfig, session *SessionConfig) (*ResolvedConfig, error) {
	provider := session.Provider
	if provider == "" {
		provider = global.ActiveProvider
	}
	if provider == "" {
		return nil, fmt.Errorf("no provider configured. %s", ConfigFixHint)
	}

	providerGlobal, ok := global.GetProviderConfig(provider)
	if !ok {
		providerGlobal = ProviderConfig{}
	}
	apiKey := normalizeAPIKey(firstNonEmpty(session.APIKey, providerGlobal.APIKey))
	if RequiresAPIKey(provider) && apiKey == "" {
		return nil, fmt.Errorf("no API key configured for %s. %s", provider, ConfigFixHint)
	}

	model := firstNonEmpty(
		session.Model,
		global.ActiveModel,
		global.GetFirstModel(provider),
	)

	resolved := &ResolvedConfig{
		Provider:       provider,
		APIKey:         apiKey,
		Model:          model,
		ThinkingEffort: global.ThinkingEffort,
		BaseURL:        providerGlobal.BaseURL,
		AuthMode:       AuthModeForProvider(provider),
		Headers:        providerGlobal.Headers,
	}

	slog.Debug("config resolved", "provider", resolved.Provider, "model", resolved.Model)
	return resolved, nil
}

func RequiresAPIKey(provider string) bool {
	return AuthModeForProvider(provider) == AuthModeAPIKey
}

func SupportsAPIKey(provider string) bool {
	return RequiresAPIKey(provider) || provider == ProviderBedrock
}

func AuthModeForProvider(provider string) string {
	if provider == ProviderBedrock {
		return AuthModeAWS
	}
	if provider == ProviderOpenAICodex {
		return AuthModeOAuth
	}
	return AuthModeAPIKey
}

func DefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		Providers: make(map[string]ProviderConfig),
	}
}

func ResolveAdversary(global *GlobalConfig) (*ResolvedConfig, error) {
	if global.AdversaryProvider == "" || global.AdversaryModel == "" {
		return nil, fmt.Errorf("adversary model not configured")
	}
	provCfg := global.Providers[global.AdversaryProvider]
	return &ResolvedConfig{
		Provider: global.AdversaryProvider,
		Model:    global.AdversaryModel,
		APIKey:   provCfg.APIKey,
		BaseURL:  provCfg.BaseURL,
		AuthMode: AuthModeForProvider(global.AdversaryProvider),
		Headers:  provCfg.Headers,
	}, nil
}

func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".keen-agent", "configs.json")
}

func ConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".keen-agent")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func normalizeAPIKey(key string) string {
	return strings.TrimSpace(key)
}

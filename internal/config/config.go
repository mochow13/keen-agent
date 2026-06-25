package config

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
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
	Models       []string          `json:"models"`
	APIKey       string            `json:"api_key"`
	APIKeyHelper string            `json:"api_key_helper,omitempty"`
	BaseURL      string            `json:"base_url,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
}

type ResolvedConfig struct {
	Provider       string
	APIKey         string
	APIKeyHelper   string
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

func ResolveProviderAPIKey(provider string, providerCfg ProviderConfig) (string, error) {
	if strings.TrimSpace(providerCfg.APIKeyHelper) != "" {
		apiKey, err := runAPIKeyHelper(provider, providerCfg.APIKeyHelper)
		if err != nil {
			return "", err
		}
		return apiKey, nil
	}

	apiKey := normalizeAPIKey(providerCfg.APIKey)
	if RequiresAPIKey(provider) && apiKey == "" {
		return "", fmt.Errorf("no API key configured for %s. %s", provider, ConfigFixHint)
	}
	return apiKey, nil
}

func runAPIKeyHelper(provider string, helper string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", helper)
	return runAPIKeyHelperCommand(ctx, provider, cmd)
}

func runAPIKeyHelperCommand(ctx context.Context, provider string, cmd *exec.Cmd) (string, error) {
	output, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("apiKeyHelper timed out for %s", provider)
	}
	if err != nil {
		return "", fmt.Errorf("apiKeyHelper failed for %s: %w", provider, err)
	}

	apiKey := normalizeAPIKey(string(output))
	if apiKey == "" {
		return "", fmt.Errorf("apiKeyHelper returned empty API key for %s", provider)
	}
	return apiKey, nil
}

func ResolveAdversary(global *GlobalConfig) (*ResolvedConfig, error) {
	if global.AdversaryProvider == "" || global.AdversaryModel == "" {
		return nil, fmt.Errorf("adversary model not configured")
	}
	provCfg := global.Providers[global.AdversaryProvider]
	apiKey, err := ResolveProviderAPIKey(global.AdversaryProvider, provCfg)
	if err != nil {
		return nil, err
	}
	return &ResolvedConfig{
		Provider: global.AdversaryProvider,
		Model:    global.AdversaryModel,
		APIKey:   apiKey,
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

func normalizeAPIKey(key string) string {
	return strings.TrimSpace(key)
}

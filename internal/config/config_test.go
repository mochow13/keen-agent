package config

import (
	"strings"
	"testing"
)

func TestGlobalConfig_GetProviderConfig(t *testing.T) {
	g := &GlobalConfig{
		Providers: map[string]ProviderConfig{
			ProviderAnthropic: {Models: []string{"claude-3-sonnet"}, APIKey: "sk-ant-test"},
			ProviderOpenAI:    {Models: []string{"gpt-4o"}, APIKey: "sk-test"},
			ProviderGoogleAI:  {Models: []string{"gemini-1.5-pro"}, APIKey: "test-key"},
		},
	}

	pc, ok := g.GetProviderConfig(ProviderAnthropic)
	if !ok {
		t.Fatal("expected to find provider config")
	}
	if pc.APIKey != "sk-ant-test" {
		t.Errorf("expected api key 'sk-ant-test', got %q", pc.APIKey)
	}
	if len(pc.Models) != 1 || pc.Models[0] != "claude-3-sonnet" {
		t.Errorf("expected models ['claude-3-sonnet'], got %v", pc.Models)
	}
}

func TestGlobalConfig_GetProviderConfig_NotFound(t *testing.T) {
	g := &GlobalConfig{}

	_, ok := g.GetProviderConfig("unknown")
	if ok {
		t.Error("expected not to find provider config")
	}
}

func TestGlobalConfig_SetProviderConfig(t *testing.T) {
	g := &GlobalConfig{}
	cfg := ProviderConfig{Models: []string{"gpt-4o"}, APIKey: "sk-test"}

	g.SetProviderConfig(ProviderOpenAI, cfg)

	pc, ok := g.GetProviderConfig(ProviderOpenAI)
	if !ok {
		t.Fatal("expected to find provider config")
	}
	if len(pc.Models) != 1 || pc.Models[0] != "gpt-4o" {
		t.Errorf("expected models ['gpt-4o'], got %v", pc.Models)
	}
	if pc.APIKey != "sk-test" {
		t.Errorf("expected api key 'sk-test', got %q", pc.APIKey)
	}
}

func TestGlobalConfig_AddModel(t *testing.T) {
	g := &GlobalConfig{}

	g.AddModel(ProviderAnthropic, "claude-3-sonnet")

	pc, _ := g.GetProviderConfig(ProviderAnthropic)
	if len(pc.Models) != 1 || pc.Models[0] != "claude-3-sonnet" {
		t.Errorf("expected models ['claude-3-sonnet'], got %v", pc.Models)
	}

	g.AddModel(ProviderAnthropic, "claude-3-sonnet")
	pc, _ = g.GetProviderConfig(ProviderAnthropic)
	if len(pc.Models) != 1 {
		t.Errorf("expected 1 model, got %d", len(pc.Models))
	}

	g.AddModel(ProviderAnthropic, "claude-3-opus")
	pc, _ = g.GetProviderConfig(ProviderAnthropic)
	if len(pc.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(pc.Models))
	}
}

func TestResolveProviderAPIKey_TrimsAPIKey(t *testing.T) {
	apiKey, err := ResolveProviderAPIKey(ProviderMiniMax, ProviderConfig{APIKey: "\n  minimax-key\t"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if apiKey != "minimax-key" {
		t.Fatalf("expected trimmed API key, got %q", apiKey)
	}
}

func TestResolveProviderAPIKey_AllowsMissingAPIKeyForOAuthAndAWS(t *testing.T) {
	if apiKey, err := ResolveProviderAPIKey(ProviderOpenAICodex, ProviderConfig{}); err != nil || apiKey != "" {
		t.Fatalf("openai-codex API key = %q, err = %v", apiKey, err)
	}
	if apiKey, err := ResolveProviderAPIKey(ProviderBedrock, ProviderConfig{}); err != nil || apiKey != "" {
		t.Fatalf("bedrock API key = %q, err = %v", apiKey, err)
	}
}

func TestResolveProviderAPIKey_UsesHelperOverConfiguredAPIKey(t *testing.T) {
	apiKey, err := ResolveProviderAPIKey(ProviderAnthropic, ProviderConfig{
		APIKey:       "stored-key",
		APIKeyHelper: "printf ' helper-key\\n'",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if apiKey != "helper-key" {
		t.Fatalf("expected helper API key, got %q", apiKey)
	}
}

func TestResolveProviderAPIKey_HelperFailureFails(t *testing.T) {
	_, err := ResolveProviderAPIKey(ProviderAnthropic, ProviderConfig{APIKeyHelper: "exit 7"})
	if err == nil {
		t.Fatal("expected error for failing helper")
	}
	if !strings.Contains(err.Error(), "apiKeyHelper failed") {
		t.Fatalf("expected helper failure error, got %v", err)
	}
}

func TestResolveProviderAPIKey_MissingAPIKey(t *testing.T) {
	_, err := ResolveProviderAPIKey(ProviderAnthropic, ProviderConfig{})
	if err == nil {
		t.Fatal("expected error for missing API key, got nil")
	}
}

func TestDefaultGlobalConfig(t *testing.T) {
	cfg := DefaultGlobalConfig()

	if cfg == nil {
		t.Fatal("expected non-nil config, got nil")
	}
	if cfg.ActiveProvider != "" {
		t.Errorf("expected empty ActiveProvider, got %q", cfg.ActiveProvider)
	}
	if cfg.ActiveModel != "" {
		t.Errorf("expected empty ActiveModel, got %q", cfg.ActiveModel)
	}
	if cfg.Providers == nil {
		t.Error("expected non-nil Providers map")
	}
}

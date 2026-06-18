package widgets

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mochow13/keen-agent/internal/auth"
	"github.com/mochow13/keen-agent/internal/config"
	"github.com/mochow13/keen-agent/internal/providers"
)

func TestIsValidBaseURL_Empty(t *testing.T) {
	if err := isValidBaseURL(""); err != nil {
		t.Errorf("expected empty URL to be valid, got %v", err)
	}
}

func TestIsValidBaseURL_ValidHTTPS(t *testing.T) {
	cases := []string{
		"https://api.example.com",
		"https://api.example.com/v1",
		"http://localhost:8080",
		"http://localhost:8080/v1/",
	}
	for _, c := range cases {
		if err := isValidBaseURL(c); err != nil {
			t.Errorf("expected %q to be valid, got %v", c, err)
		}
	}
}

func TestIsValidBaseURL_InvalidScheme(t *testing.T) {
	cases := []string{
		"ftp://example.com",
		"example.com",
		"//example.com",
	}
	for _, c := range cases {
		if err := isValidBaseURL(c); err == nil {
			t.Errorf("expected %q to be invalid, got nil", c)
		}
	}
}

func TestIsValidBaseURL_MissingHost(t *testing.T) {
	if err := isValidBaseURL("https://"); err == nil {
		t.Error("expected URL with no host to be invalid")
	}
}

func TestModelSelection_OpenAICodexSkipsAPIKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	registry := &providers.Registry{
		Providers: []providers.Provider{
			{
				ID:   config.ProviderOpenAICodex,
				Name: "Codex (ChatGPT OAuth)",
				Models: []providers.Model{
					{
						ID:              "gpt-5.4",
						Name:            "GPT-5.4",
						ThinkingEfforts: []string{"low", "medium", "high", "xhigh"},
					},
				},
			},
		},
	}
	global := config.DefaultGlobalConfig()
	resolved := &config.ResolvedConfig{}
	store := auth.NewStoreAt(t.TempDir() + "/auth.json")
	if err := store.Set(config.ProviderOpenAICodex, auth.OAuthCredential{
		Type:         "oauth",
		AccessToken:  "access",
		RefreshToken: "refresh",
	}); err != nil {
		t.Fatalf("seed auth store: %v", err)
	}
	manager := auth.NewOAuthManager(store)

	completed := false
	m := NewWithAuthManager(registry, global, config.NewLoader(), resolved, manager, func(provider, model, apiKey string) error {
		completed = true
		if apiKey != "" {
			t.Fatalf("expected empty API key, got %q", apiKey)
		}
		return nil
	})

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("did not expect OAuth command for existing credentials")
	}
	if m.Step != StepModel {
		t.Fatalf("expected StepModel, got %v", m.Step)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.Step != StepThinking {
		t.Fatalf("expected StepThinking, got %v", m.Step)
	}
	m, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !completed {
		t.Fatal("expected completion")
	}
	if cmd == nil {
		t.Fatal("expected completion command")
	}
	if resolved.Provider != config.ProviderOpenAICodex || resolved.Model != "gpt-5.4" {
		t.Fatalf("unexpected resolved config: %+v", resolved)
	}
	if resolved.APIKey != "" || resolved.BaseURL != "" {
		t.Fatalf("expected no API key/base URL for Codex, got %+v", resolved)
	}
}

func TestModelSelection_BedrockAPIKeyCanBeSkipped(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	registry := &providers.Registry{
		Providers: []providers.Provider{
			{
				ID:   config.ProviderBedrock,
				Name: "Amazon Bedrock",
				Models: []providers.Model{
					{ID: "global.anthropic.claude-sonnet-4-6", Name: "Claude Sonnet 4.6"},
				},
			},
		},
	}
	global := config.DefaultGlobalConfig()
	resolved := &config.ResolvedConfig{}

	completed := false
	m := New(registry, global, config.NewLoader(), resolved, func(provider, model, apiKey string) error {
		completed = true
		if apiKey != "" {
			t.Fatalf("expected empty Bedrock API key for AWS auth fallback, got %q", apiKey)
		}
		return nil
	})
	var cmd tea.Cmd
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.Step != StepAPIKey {
		t.Fatalf("expected optional StepAPIKey, got %v", m.Step)
	}
	m, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !completed {
		t.Fatal("expected completion without API key")
	}
	if cmd == nil {
		t.Fatal("expected completion command")
	}
	if resolved.APIKey != "" {
		t.Fatalf("expected empty resolved API key, got %q", resolved.APIKey)
	}
	if resolved.AuthMode != config.AuthModeAWS {
		t.Fatalf("expected AWS auth mode, got %q", resolved.AuthMode)
	}
}

func TestModelSelection_LongModelListScrollsWithCursor(t *testing.T) {
	models := make([]providers.Model, 14)
	for i := range models {
		models[i] = providers.Model{
			ID:   fmt.Sprintf("model-%02d", i+1),
			Name: fmt.Sprintf("Model %02d", i+1),
		}
	}
	registry := &providers.Registry{
		Providers: []providers.Provider{
			{
				ID:     config.ProviderOpenCodeGo,
				Name:   "OpenCode Go",
				Models: models,
			},
		},
	}

	m := New(registry, config.DefaultGlobalConfig(), config.NewLoader(), &config.ResolvedConfig{}, func(provider, model, apiKey string) error {
		return nil
	})
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("did not expect command when entering model selection")
	}
	if m.Step != StepModel {
		t.Fatalf("expected StepModel, got %v", m.Step)
	}

	initial := m.renderModelSelection()
	if !strings.Contains(initial, "Model 01") {
		t.Fatalf("expected first model in initial view, got %q", initial)
	}
	if strings.Contains(initial, "Model 14") {
		t.Fatalf("did not expect final model before scrolling, got %q", initial)
	}
	if !strings.Contains(initial, "↓") {
		t.Fatalf("expected downward more indicator, got %q", initial)
	}

	for i := 0; i < 13; i++ {
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}

	scrolled := m.renderModelSelection()
	if !strings.Contains(scrolled, "Model 14") {
		t.Fatalf("expected final model after scrolling, got %q", scrolled)
	}
	if strings.Contains(scrolled, "Model 01") {
		t.Fatalf("did not expect first model after scrolling to bottom, got %q", scrolled)
	}
	if !strings.Contains(scrolled, "↑") {
		t.Fatalf("expected upward more indicator, got %q", scrolled)
	}
}

func TestVisibleListRangeKeepsCursorVisible(t *testing.T) {
	tests := []struct {
		name       string
		cursor     int
		count      int
		maxVisible int
		wantStart  int
		wantEnd    int
	}{
		{name: "empty", cursor: 0, count: 0, maxVisible: 7, wantStart: 0, wantEnd: 0},
		{name: "short list", cursor: 2, count: 4, maxVisible: 7, wantStart: 0, wantEnd: 4},
		{name: "top", cursor: 0, count: 14, maxVisible: 7, wantStart: 0, wantEnd: 7},
		{name: "middle", cursor: 6, count: 14, maxVisible: 7, wantStart: 3, wantEnd: 10},
		{name: "bottom", cursor: 13, count: 14, maxVisible: 7, wantStart: 7, wantEnd: 14},
	}

	for _, tt := range tests {
		gotStart, gotEnd := visibleListRange(tt.cursor, tt.count, tt.maxVisible)
		if gotStart != tt.wantStart || gotEnd != tt.wantEnd {
			t.Fatalf("%s: expected %d:%d, got %d:%d", tt.name, tt.wantStart, tt.wantEnd, gotStart, gotEnd)
		}
		if tt.count > 0 && tt.count > tt.maxVisible && (tt.cursor < gotStart || tt.cursor >= gotEnd) {
			t.Fatalf("%s: cursor %d outside visible range %d:%d", tt.name, tt.cursor, gotStart, gotEnd)
		}
	}
}

package providers

import (
	"testing"
)

func TestLoad(t *testing.T) {
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if reg == nil {
		t.Fatal("Load() returned nil registry")
	}
	if len(reg.Providers) == 0 {
		t.Error("Load() returned empty providers list")
	}

	for _, p := range reg.Providers {
		for _, m := range p.Models {
			if m.ContextWindow <= 0 {
				t.Errorf("model %s/%s has invalid context_window %d", p.ID, m.ID, m.ContextWindow)
			}
		}
	}
}

func TestRegistry_GetProvider(t *testing.T) {
	reg := &Registry{
		Providers: []Provider{
			{ID: "anthropic", Name: "Anthropic"},
			{ID: "openai", Name: "OpenAI"},
		},
	}

	p, ok := reg.GetProvider("anthropic")
	if !ok {
		t.Error("GetProvider('anthropic') should return true")
	}
	if p.ID != "anthropic" || p.Name != "Anthropic" {
		t.Errorf("GetProvider returned wrong provider: %+v", p)
	}

	_, ok = reg.GetProvider("unknown")
	if ok {
		t.Error("GetProvider('unknown') should return false")
	}
}

func TestRegistry_GetModelContextWindow(t *testing.T) {
	reg := &Registry{
		Providers: []Provider{
			{
				ID: "openai",
				Models: []Model{
					{ID: "gpt-5.4", ContextWindow: 1050000},
				},
			},
		},
	}

	got, ok := reg.GetModelContextWindow("openai", "gpt-5.4")
	if !ok {
		t.Fatal("expected lookup success")
	}
	if got != 1050000 {
		t.Fatalf("expected 1050000, got %d", got)
	}

	if _, ok := reg.GetModelContextWindow("openai", "unknown"); ok {
		t.Fatal("expected unknown model lookup to fail")
	}

	if _, ok := reg.GetModelContextWindow("unknown", "gpt-5.4"); ok {
		t.Fatal("expected unknown provider lookup to fail")
	}
}

func TestModel_ThinkingEffortsLoadFromYAML(t *testing.T) {
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// claude-opus-4-6 should have thinking efforts
	m, ok := reg.GetModel("anthropic", "claude-opus-4-6")
	if !ok {
		t.Fatal("expected to find claude-opus-4-6")
	}
	if !m.SupportsThinkingEffort() {
		t.Error("expected claude-opus-4-6 to support thinking effort")
	}
	if len(m.ThinkingEfforts) != 4 {
		t.Errorf("expected 4 efforts for claude-opus-4-6, got %d: %v", len(m.ThinkingEfforts), m.ThinkingEfforts)
	}
	for _, effort := range m.ThinkingEfforts {
		if effort == "off" {
			t.Errorf("did not expect anthropic off effort, got %v", m.ThinkingEfforts)
		}
	}

	// claude-haiku-4-5 should NOT have thinking efforts
	haiku, ok := reg.GetModel("anthropic", "claude-haiku-4-5")
	if !ok {
		t.Fatal("expected to find claude-haiku-4-5")
	}
	if haiku.SupportsThinkingEffort() {
		t.Error("expected claude-haiku-4-5 to NOT support thinking effort")
	}

	// gpt-5.4 should have off and xhigh
	gpt, ok := reg.GetModel("openai", "gpt-5.4")
	if !ok {
		t.Fatal("expected to find gpt-5.4")
	}
	if !gpt.SupportsThinkingEffort() {
		t.Error("expected gpt-5.4 to support thinking effort")
	}
	foundXHigh := false
	for _, e := range gpt.ThinkingEfforts {
		if e == "xhigh" {
			foundXHigh = true
		}
	}
	if !foundXHigh {
		t.Errorf("expected gpt-5.4 to have xhigh effort, got %v", gpt.ThinkingEfforts)
	}

	codex, ok := reg.GetModel("openai-codex", "gpt-5.4")
	if !ok {
		t.Fatal("expected to find openai-codex/gpt-5.4")
	}
	if codex.ContextWindow != 256000 {
		t.Fatalf("expected openai-codex/gpt-5.4 context 256000, got %d", codex.ContextWindow)
	}

	deepseek, ok := reg.GetModel("deepseek", "deepseek-v4-pro")
	if !ok {
		t.Fatal("expected to find deepseek-v4-pro")
	}
	if !deepseek.SupportsThinkingEffort() {
		t.Error("expected deepseek-v4-pro to support thinking effort")
	}
	expectedDeepSeek := []string{"off", "high", "max"}
	if len(deepseek.ThinkingEfforts) != len(expectedDeepSeek) {
		t.Fatalf("expected deepseek-v4-pro efforts %v, got %v", expectedDeepSeek, deepseek.ThinkingEfforts)
	}
	for i, effort := range expectedDeepSeek {
		if deepseek.ThinkingEfforts[i] != effort {
			t.Fatalf("expected deepseek-v4-pro efforts %v, got %v", expectedDeepSeek, deepseek.ThinkingEfforts)
		}
	}

	minimaxProvider, ok := reg.GetProvider("minimax")
	if !ok {
		t.Fatal("expected to find minimax provider")
	}
	if len(minimaxProvider.Models) != 2 {
		t.Fatalf("expected 2 minimax models, got %d", len(minimaxProvider.Models))
	}
	minimaxM27, ok := reg.GetModel("minimax", "minimax-m2.7")
	if !ok {
		t.Fatal("expected to find minimax/minimax-m2.7")
	}
	if minimaxM27.ContextWindow != 204800 {
		t.Fatalf("expected minimax-m2.7 context 204800, got %d", minimaxM27.ContextWindow)
	}
	if minimaxM27.SupportsThinkingEffort() {
		t.Fatalf("expected minimax-m2.7 to omit thinking efforts, got %v", minimaxM27.ThinkingEfforts)
	}

	opencode, ok := reg.GetProvider("opencode-go")
	if !ok {
		t.Fatal("expected to find opencode-go provider")
	}
	if len(opencode.Models) != 13 {
		t.Fatalf("expected 15 opencode-go models, got %d", len(opencode.Models))
	}

	qwen, ok := reg.GetModel("opencode-go", "qwen3.6-plus")
	if !ok {
		t.Fatal("expected to find opencode-go/qwen3.6-plus")
	}
	if qwen.ContextWindow != 1000000 {
		t.Fatalf("expected qwen3.6-plus context 1000000, got %d", qwen.ContextWindow)
	}
	expectedQwen := []string{"enabled", "disabled"}
	for i, effort := range expectedQwen {
		if qwen.ThinkingEfforts[i] != effort {
			t.Fatalf("expected qwen3.6-plus efforts %v, got %v", expectedQwen, qwen.ThinkingEfforts)
		}
	}

	qwenMax, ok := reg.GetModel("opencode-go", "qwen3.7-max")
	if !ok {
		t.Fatal("expected to find opencode-go/qwen3.7-max")
	}
	if qwenMax.SupportsThinkingEffort() {
		t.Fatalf("expected qwen3.7-max to omit thinking efforts, got %v", qwenMax.ThinkingEfforts)
	}

	minimax, ok := reg.GetModel("opencode-go", "minimax-m2.7")
	if !ok {
		t.Fatal("expected to find opencode-go/minimax-m2.7")
	}
	if minimax.SupportsThinkingEffort() {
		t.Fatalf("expected minimax-m2.7 to omit thinking efforts, got %v", minimax.ThinkingEfforts)
	}
}

func TestRegistry_GetModel(t *testing.T) {
	reg := &Registry{
		Providers: []Provider{
			{
				ID: "anthropic",
				Models: []Model{
					{ID: "claude-opus-4-6", ThinkingEfforts: []string{"low", "medium", "high", "max"}},
					{ID: "claude-haiku-4-5"},
				},
			},
		},
	}

	m, ok := reg.GetModel("anthropic", "claude-opus-4-6")
	if !ok {
		t.Fatal("expected to find claude-opus-4-6")
	}
	if !m.SupportsThinkingEffort() {
		t.Error("expected SupportsThinkingEffort() true")
	}

	haiku, ok := reg.GetModel("anthropic", "claude-haiku-4-5")
	if !ok {
		t.Fatal("expected to find claude-haiku-4-5")
	}
	if haiku.SupportsThinkingEffort() {
		t.Error("expected SupportsThinkingEffort() false for haiku")
	}

	_, ok = reg.GetModel("anthropic", "unknown")
	if ok {
		t.Error("expected GetModel to return false for unknown model")
	}

	_, ok = reg.GetModel("unknown", "claude-opus-4-6")
	if ok {
		t.Error("expected GetModel to return false for unknown provider")
	}
}

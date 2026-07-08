package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mochow13/keen-agent/internal/agentconfig"
	"github.com/mochow13/keen-agent/internal/config"
	"github.com/mochow13/keen-agent/internal/llm"
	keenmcp "github.com/mochow13/keen-agent/internal/mcp"
	"github.com/mochow13/keen-agent/internal/providers"
)

func TestNewRootCommand(t *testing.T) {
	cmd := NewRootCommand("0.1.0")

	if cmd.Use != "keen-agent" {
		t.Errorf("command Use = %q, want 'keen-agent'", cmd.Use)
	}

	if cmd.Version != "0.1.0" {
		t.Errorf("command Version = %q, want '0.1.0'", cmd.Version)
	}

	if cmd.Short == "" {
		t.Error("command Short should not be empty")
	}

	if cmd.Long == "" {
		t.Error("command Long should not be empty")
	}
}

func TestNewRootCommand_HasRunCommand(t *testing.T) {
	cmd := NewRootCommand("0.1.0")

	runCmd, _, err := cmd.Find([]string{"run"})
	if err != nil {
		t.Fatalf("Find(run) error = %v", err)
	}
	if runCmd == nil || runCmd.Name() != "run" {
		t.Fatalf("expected run command, got %#v", runCmd)
	}
}

func TestNewRootCommand_HasResumeFlag(t *testing.T) {
	cmd := NewRootCommand("0.1.0")

	if cmd.Flags().Lookup("resume") == nil {
		t.Fatal("expected root command to have --resume flag")
	}
}

func TestNewRootCommand_RunCommandHasModelProviderFlags(t *testing.T) {
	cmd := NewRootCommand("0.1.0")

	runCmd, _, err := cmd.Find([]string{"run"})
	if err != nil {
		t.Fatalf("Find(run) error = %v", err)
	}

	for _, name := range []string{"model", "provider"} {
		if runCmd.Flags().Lookup(name) == nil {
			t.Fatalf("expected run command to have --%s flag", name)
		}
	}
}

func TestStartMCPRuntimeStartsWithE2EConfigAndCloses(t *testing.T) {
	previous := newMCPManager
	defer func() { newMCPManager = previous }()

	fake := &fakeMCPRuntime{}
	var gotOptions int
	newMCPManager = func(opts ...keenmcp.Option) (keenmcp.Runtime, error) {
		gotOptions = len(opts)
		return fake, nil
	}

	manager, closeMCP, err := startMCPRuntime(context.Background(), nil)
	closeMCP()
	if err != nil {
		t.Fatalf("startMCPRuntime() error = %v", err)
	}

	if manager != fake {
		t.Fatalf("manager = %#v, want fake", manager)
	}
	if gotOptions != 0 {
		t.Fatalf("options length = %d, want 0", gotOptions)
	}
	if fake.starts != 1 {
		t.Fatalf("starts = %d, want 1", fake.starts)
	}
	if fake.closes != 1 {
		t.Fatalf("closes = %d, want 1", fake.closes)
	}
}

func TestStartMCPRuntimeIsBestEffortOnCreateError(t *testing.T) {
	previous := newMCPManager
	defer func() { newMCPManager = previous }()

	newMCPManager = func(opts ...keenmcp.Option) (keenmcp.Runtime, error) {
		return nil, errors.New("boom")
	}

	manager, closeMCP, err := startMCPRuntime(context.Background(), nil)
	closeMCP()
	if manager != nil {
		t.Fatalf("manager = %#v, want nil", manager)
	}
	if err == nil {
		t.Fatal("startMCPRuntime() error = nil, want error")
	}
}

func TestStartMCPRuntimeClosesAfterStartError(t *testing.T) {
	previous := newMCPManager
	defer func() { newMCPManager = previous }()

	fake := &fakeMCPRuntime{startErr: errors.New("boom")}
	newMCPManager = func(opts ...keenmcp.Option) (keenmcp.Runtime, error) {
		return fake, nil
	}

	manager, closeMCP, err := startMCPRuntime(context.Background(), nil)
	closeMCP()
	if manager != nil {
		t.Fatalf("manager = %#v, want nil", manager)
	}
	if err == nil {
		t.Fatal("startMCPRuntime() error = nil, want error")
	}

	if fake.starts != 1 {
		t.Fatalf("starts = %d, want 1", fake.starts)
	}
	if fake.closes != 1 {
		t.Fatalf("closes = %d, want 1", fake.closes)
	}
}

func TestApplyRunOverrides(t *testing.T) {
	globalCfg := &config.GlobalConfig{
		Providers: map[string]config.ProviderConfig{
			config.ProviderAnthropic: {
				APIKey:  "anthropic-key",
				Models:  []string{"claude-3"},
				BaseURL: "https://anthropic.example",
			},
			config.ProviderOpenCodeGo: {
				APIKey:  "opencode-key",
				Models:  []string{"kimi-k2.6"},
				BaseURL: "https://opencode.example",
			},
		},
	}
	resolvedCfg := &config.ResolvedConfig{
		Provider: config.ProviderAnthropic,
		APIKey:   "anthropic-key",
		Model:    "claude-3",
		BaseURL:  "https://anthropic.example",
		AuthMode: config.AuthModeAPIKey,
	}

	err := applyRunOverrides(globalCfg, resolvedCfg, config.ProviderOpenCodeGo, "qwen3.6-plus")
	if err != nil {
		t.Fatalf("applyRunOverrides() error = %v", err)
	}

	if resolvedCfg.Provider != config.ProviderOpenCodeGo {
		t.Fatalf("Provider = %q, want %q", resolvedCfg.Provider, config.ProviderOpenCodeGo)
	}
	if resolvedCfg.APIKey != "opencode-key" {
		t.Fatalf("APIKey = %q, want opencode-key", resolvedCfg.APIKey)
	}
	if resolvedCfg.BaseURL != "https://opencode.example" {
		t.Fatalf("BaseURL = %q, want https://opencode.example", resolvedCfg.BaseURL)
	}
	if resolvedCfg.Model != "qwen3.6-plus" {
		t.Fatalf("Model = %q, want qwen3.6-plus", resolvedCfg.Model)
	}
}

type fakeMCPRuntime struct {
	startErr error
	closeErr error
	starts   int
	closes   int
}

func (f *fakeMCPRuntime) Start(context.Context) error {
	f.starts++
	return f.startErr
}

func (f *fakeMCPRuntime) Close() error {
	f.closes++
	return f.closeErr
}

func (f *fakeMCPRuntime) Servers() []keenmcp.ServerStatus {
	return nil
}

func (f *fakeMCPRuntime) Status(server string) keenmcp.ServerStatus {
	return keenmcp.ServerStatus{Name: server}
}

func (f *fakeMCPRuntime) WaitInitialScan(context.Context) error {
	return nil
}

func (f *fakeMCPRuntime) ListTools(context.Context, string) ([]keenmcp.Tool, error) {
	return nil, nil
}

func (f *fakeMCPRuntime) Refresh(context.Context, string, ...keenmcp.RefreshOption) error {
	return nil
}

func (f *fakeMCPRuntime) CallTool(context.Context, string, string, map[string]any) (*keenmcp.ToolResult, error) {
	return &keenmcp.ToolResult{}, nil
}

func TestApplyRunOverrides_ProviderUsesFirstConfiguredModel(t *testing.T) {
	globalCfg := &config.GlobalConfig{
		Providers: map[string]config.ProviderConfig{
			config.ProviderOpenCodeGo: {
				APIKey: "opencode-key",
				Models: []string{"kimi-k2.6"},
			},
		},
	}
	resolvedCfg := &config.ResolvedConfig{
		Provider: config.ProviderAnthropic,
		Model:    "claude-3",
	}

	err := applyRunOverrides(globalCfg, resolvedCfg, config.ProviderOpenCodeGo, "")
	if err != nil {
		t.Fatalf("applyRunOverrides() error = %v", err)
	}

	if resolvedCfg.Model != "kimi-k2.6" {
		t.Fatalf("Model = %q, want kimi-k2.6", resolvedCfg.Model)
	}
}

func TestApplyRunOverrides_ProviderUsesAPIKeyHelper(t *testing.T) {
	globalCfg := &config.GlobalConfig{
		Providers: map[string]config.ProviderConfig{
			config.ProviderOpenCodeGo: {
				APIKey:       "stored-key",
				APIKeyHelper: "printf 'helper-key'",
				Models:       []string{"kimi-k2.6"},
			},
		},
	}
	resolvedCfg := &config.ResolvedConfig{
		Provider: config.ProviderAnthropic,
		APIKey:   "anthropic-key",
		Model:    "claude-3",
	}

	err := applyRunOverrides(globalCfg, resolvedCfg, config.ProviderOpenCodeGo, "")
	if err != nil {
		t.Fatalf("applyRunOverrides() error = %v", err)
	}

	if resolvedCfg.APIKey != "helper-key" {
		t.Fatalf("APIKey = %q, want helper-key", resolvedCfg.APIKey)
	}
}

func TestBuildRunPrompt(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{name: "args only", args: []string{"hello", "there"}, want: "hello there"},
		{name: "stdin only", stdin: " from stdin\n", want: "from stdin"},
		{name: "args and stdin", args: []string{"hello"}, stdin: "from stdin\n", want: "hello\nfrom stdin"},
		{name: "empty", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildRunPrompt(tt.args, tt.stdin)
			if got != tt.want {
				t.Fatalf("buildRunPrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewRootCommand_HasAgentFlag(t *testing.T) {
	cmd := NewRootCommand("0.1.0")
	if cmd.Flags().Lookup("agent") == nil {
		t.Fatal("expected root command to have --agent flag")
	}
}

func TestNewRunCommand_HasAgentFlag(t *testing.T) {
	cmd := NewRootCommand("0.1.0")
	runCmd, _, err := cmd.Find([]string{"run"})
	if err != nil {
		t.Fatalf("Find(run) error = %v", err)
	}
	if runCmd.Flags().Lookup("agent") == nil {
		t.Fatal("expected run command to have --agent flag")
	}
}

func TestNewRootCommand_HasModeFlag(t *testing.T) {
	cmd := NewRootCommand("0.1.0")
	if cmd.Flags().Lookup("mode") == nil {
		t.Fatal("expected root command to have --mode flag")
	}
}

func TestNewRunCommand_HasModeFlag(t *testing.T) {
	cmd := NewRootCommand("0.1.0")
	runCmd, _, err := cmd.Find([]string{"run"})
	if err != nil {
		t.Fatalf("Find(run) error = %v", err)
	}
	if runCmd.Flags().Lookup("mode") == nil {
		t.Fatal("expected run command to have --mode flag")
	}
}

func TestNewRootCommand_HasValidateCommand(t *testing.T) {
	cmd := NewRootCommand("0.1.0")
	validateCmd, _, err := cmd.Find([]string{"validate"})
	if err != nil {
		t.Fatalf("Find(validate) error = %v", err)
	}
	if validateCmd == nil || validateCmd.Name() != "validate" {
		t.Fatalf("expected validate command, got %#v", validateCmd)
	}
	if validateCmd.Flags().Lookup("agent") == nil {
		t.Fatal("expected validate command to have --agent flag")
	}
}

func TestValidateCommand_ValidConfig(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "agent.yaml")
	if err := os.WriteFile(path, []byte("name: test\nsystem_prompt: hi\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cmd := NewRootCommand("0.1.0")
	cmd.SetArgs([]string{"validate", "--agent", path})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, output = %q", err, out.String())
	}
	if !strings.Contains(out.String(), "is valid") {
		t.Fatalf("expected valid output, got %q", out.String())
	}
}

func TestValidateCommand_InvalidConfig(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "agent.yaml")
	if err := os.WriteFile(path, []byte("name: \"\"\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cmd := NewRootCommand("0.1.0")
	cmd.SetArgs([]string{"validate", "--agent", path})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
	// Mirror what cmd/main.go prints so the assertion reflects real CLI output.
	_, _ = fmt.Fprintf(&out, "Error: %v\n", err)

	if !strings.Contains(out.String(), "Error:") {
		t.Fatalf("expected error output, got %q", out.String())
	}
	summary := fmt.Sprintf("agent config %q is invalid", path)
	if strings.Count(out.String(), summary) != 1 {
		t.Fatalf("expected summary %q exactly once, got %q", summary, out.String())
	}
}

func TestResolveModeOverride(t *testing.T) {
	cfg := &agentconfig.Config{DefaultMode: agentconfig.ModePlan}

	tests := []struct {
		name    string
		flag    string
		cfg     *agentconfig.Config
		want    llm.AgentMode
		wantErr bool
	}{
		{"config default plan", "", cfg, llm.ModePlan, false},
		{"flag overrides config", "build", cfg, llm.ModeBuild, false},
		{"flag plan", "plan", cfg, llm.ModePlan, false},
		{"invalid flag", "debug", cfg, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveModeOverride(tt.cfg, tt.flag)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveModeOverride() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("resolveModeOverride() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveAgentConfigPath_Explicit(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "custom.yaml")
	if err := os.WriteFile(path, []byte("name: test\nsystem_prompt: hi\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := resolveAgentConfigPath(tmp, path)
	if err != nil {
		t.Fatalf("resolveAgentConfigPath() error = %v", err)
	}
	if got != path {
		t.Fatalf("resolveAgentConfigPath() = %q, want %q", got, path)
	}
}

func TestResolveAgentConfigPath_ExplicitMissing(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "missing.yaml")

	_, err := resolveAgentConfigPath(tmp, path)
	if err == nil {
		t.Fatal("expected error for missing explicit path")
	}
}

func TestResolveAgentConfigPath_RequiresExplicitPath(t *testing.T) {
	tmp := t.TempDir()

	_, err := resolveAgentConfigPath(tmp, "")
	if err == nil {
		t.Fatal("expected error when --agent is not provided")
	}
}

func TestLoadAgentConfig_LoadsAndValidates(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "agent.yaml")
	if err := os.WriteFile(path, []byte("name: test\nsystem_prompt: hi\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cfg, err := loadAgentConfig(tmp, path)
	if err != nil {
		t.Fatalf("loadAgentConfig() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("loadAgentConfig() returned nil config")
	}
	if cfg.Name != "test" {
		t.Fatalf("cfg.Name = %q, want test", cfg.Name)
	}
}

func TestLoadAgentConfig_RequiresExplicitPath(t *testing.T) {
	tmp := t.TempDir()

	_, err := loadAgentConfig(tmp, "")
	if err == nil {
		t.Fatal("expected error when --agent is not provided")
	}
}

func TestLoadAgentConfig_RejectsInvalidConfig(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "agent.yaml")
	if err := os.WriteFile(path, []byte("name: \"\"\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := loadAgentConfig(tmp, path)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestResolveSessionConfig_UsesAgentModelWhenAvailable(t *testing.T) {
	globalCfg := &config.GlobalConfig{
		ActiveProvider: config.ProviderAnthropic,
		ActiveModel:    "claude-active",
		Providers: map[string]config.ProviderConfig{
			"google": {
				APIKey: "google-key",
				Models: []string{"gemini-test"},
			},
			config.ProviderAnthropic: {
				APIKey: "anthropic-key",
				Models: []string{"claude-active"},
			},
		},
	}
	registry := &providers.Registry{
		Providers: []providers.Provider{
			{ID: "google", Models: []providers.Model{{ID: "gemini-test"}}},
		},
	}
	agentCfg := &agentconfig.Config{
		Model: &agentconfig.ModelRef{Provider: "google", ModelID: "gemini-test"},
	}

	resolved, needsSetup, warning, err := resolveSessionConfig(globalCfg, registry, agentCfg)
	if err != nil {
		t.Fatalf("resolveSessionConfig() error = %v", err)
	}
	if resolved.Provider != "google" {
		t.Fatalf("Provider = %q, want google", resolved.Provider)
	}
	if resolved.Model != "gemini-test" {
		t.Fatalf("Model = %q, want gemini-test", resolved.Model)
	}
	if resolved.APIKey != "google-key" {
		t.Fatalf("APIKey = %q, want google-key", resolved.APIKey)
	}
	if needsSetup {
		t.Fatal("needsSetup = true, want false")
	}
	if warning != "" {
		t.Fatalf("warning = %q, want empty", warning)
	}
	if globalCfg.ActiveProvider != config.ProviderAnthropic {
		t.Fatalf("globalCfg.ActiveProvider was modified to %q", globalCfg.ActiveProvider)
	}
}

func TestResolveSessionConfig_WarnsWhenAgentModelMissing(t *testing.T) {
	globalCfg := &config.GlobalConfig{
		ActiveProvider: config.ProviderAnthropic,
		ActiveModel:    "claude-active",
		Providers: map[string]config.ProviderConfig{
			config.ProviderAnthropic: {
				APIKey: "anthropic-key",
				Models: []string{"claude-active"},
			},
		},
	}
	registry := &providers.Registry{
		Providers: []providers.Provider{
			{ID: config.ProviderAnthropic, Models: []providers.Model{{ID: "claude-active"}}},
		},
	}
	agentCfg := &agentconfig.Config{
		Model: &agentconfig.ModelRef{Provider: "google", ModelID: "missing-model"},
	}

	resolved, _, warning, err := resolveSessionConfig(globalCfg, registry, agentCfg)
	if err != nil {
		t.Fatalf("resolveSessionConfig() error = %v", err)
	}
	if resolved.Provider != config.ProviderAnthropic || resolved.Model != "claude-active" {
		t.Fatalf("fallback model = %s/%s, want anthropic/claude-active", resolved.Provider, resolved.Model)
	}
	if warning == "" {
		t.Fatal("expected warning for missing agent model")
	}
}

func TestResolveSessionConfig_WarnsWhenAgentModelIncomplete(t *testing.T) {
	globalCfg := &config.GlobalConfig{
		ActiveProvider: config.ProviderAnthropic,
		ActiveModel:    "claude-active",
		Providers: map[string]config.ProviderConfig{
			config.ProviderAnthropic: {
				APIKey: "anthropic-key",
				Models: []string{"claude-active"},
			},
		},
	}
	registry := &providers.Registry{
		Providers: []providers.Provider{
			{ID: config.ProviderAnthropic, Models: []providers.Model{{ID: "claude-active"}}},
		},
	}
	agentCfg := &agentconfig.Config{
		Model: &agentconfig.ModelRef{Provider: "google"},
	}

	resolved, _, warning, err := resolveSessionConfig(globalCfg, registry, agentCfg)
	if err != nil {
		t.Fatalf("resolveSessionConfig() error = %v", err)
	}
	if resolved.Provider != config.ProviderAnthropic || resolved.Model != "claude-active" {
		t.Fatalf("fallback model = %s/%s, want anthropic/claude-active", resolved.Provider, resolved.Model)
	}
	if warning == "" {
		t.Fatal("expected warning for incomplete model block")
	}
}

func TestResolveSessionConfig_FallsBackToActiveModel(t *testing.T) {
	globalCfg := &config.GlobalConfig{
		ActiveProvider: config.ProviderAnthropic,
		ActiveModel:    "claude-active",
		Providers: map[string]config.ProviderConfig{
			config.ProviderAnthropic: {
				APIKey: "anthropic-key",
				Models: []string{"claude-active"},
			},
		},
	}
	registry := &providers.Registry{
		Providers: []providers.Provider{
			{ID: config.ProviderAnthropic, Models: []providers.Model{{ID: "claude-active"}}},
		},
	}

	resolved, _, warning, err := resolveSessionConfig(globalCfg, registry, nil)
	if err != nil {
		t.Fatalf("resolveSessionConfig() error = %v", err)
	}
	if resolved.Provider != config.ProviderAnthropic || resolved.Model != "claude-active" {
		t.Fatalf("model = %s/%s, want anthropic/claude-active", resolved.Provider, resolved.Model)
	}
	if warning != "" {
		t.Fatalf("warning = %q, want empty", warning)
	}
}

func TestResolveSessionConfig_NoModelAvailable(t *testing.T) {
	globalCfg := &config.GlobalConfig{Providers: map[string]config.ProviderConfig{}}
	registry := &providers.Registry{}

	resolved, needsSetup, warning, err := resolveSessionConfig(globalCfg, registry, nil)
	if err != nil {
		t.Fatalf("resolveSessionConfig() error = %v", err)
	}
	if resolved.Provider != "" || resolved.Model != "" {
		t.Fatalf("expected empty resolved config, got %s/%s", resolved.Provider, resolved.Model)
	}
	if !needsSetup {
		t.Fatal("needsSetup = false, want true")
	}
	if warning != "" {
		t.Fatalf("warning = %q, want empty", warning)
	}
}

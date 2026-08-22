package repl

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/mochow13/keen-agent/internal/agentconfig"
	"github.com/mochow13/keen-agent/internal/config"
	"github.com/mochow13/keen-agent/internal/llm"
	"github.com/mochow13/keen-agent/internal/mcp"
	"github.com/mochow13/keen-agent/internal/session"
	"github.com/mochow13/keen-agent/internal/tools"
)

type recordingHeadlessClient struct {
	events     []llm.StreamEvent
	messages   [][]llm.Message
	registries []*tools.Registry
	opts       [][]llm.StreamOptions
}

func (c *recordingHeadlessClient) StreamChat(ctx context.Context, messages []llm.Message, toolRegistry *tools.Registry, opts ...llm.StreamOptions) (<-chan llm.StreamEvent, error) {
	c.messages = append(c.messages, llm.CloneMessages(messages))
	c.registries = append(c.registries, toolRegistry)
	c.opts = append(c.opts, append([]llm.StreamOptions(nil), opts...))
	ch := make(chan llm.StreamEvent, len(c.events))
	go func() {
		defer close(ch)
		for _, event := range c.events {
			select {
			case ch <- event:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (c *recordingHeadlessClient) Reset() {}

func TestRunHeadless_CreatesSessionAndWritesText(t *testing.T) {
	workingDir := setupHeadlessTestHome(t)
	client := &recordingHeadlessClient{events: []llm.StreamEvent{
		{Type: llm.StreamEventTypeChunk, Content: "hello"},
		{Type: llm.StreamEventTypeUsage, Usage: &llm.TokenUsage{InputTokens: 3, OutputTokens: 2, TotalTokens: 5}},
		{Type: llm.StreamEventTypeDone},
	}}
	var out bytes.Buffer

	result, err := RunHeadless(context.Background(), HeadlessRunOptions{
		WorkingDir: workingDir,
		Config:     headlessTestConfig(),
		Client:     client,
		Prompt:     "say hi",
		Out:        &out,
	})
	if err != nil {
		t.Fatalf("RunHeadless() error = %v", err)
	}
	if result.SessionID == "" {
		t.Fatal("expected session id")
	}
	if result.OpenCodeSessionID == "" || result.OpenCodeSessionID == result.SessionID {
		t.Fatalf("expected hyphen-stripped OpenCode session id, got %q from %q", result.OpenCodeSessionID, result.SessionID)
	}
	if result.Text != "hello" || out.String() != "hello\n" {
		t.Fatalf("unexpected output result=%q out=%q", result.Text, out.String())
	}
	if result.Usage == nil || result.Usage.InputTokens != 3 || result.Usage.OutputTokens != 2 || result.Usage.TotalTokens != 5 {
		t.Fatalf("unexpected usage: %#v", result.Usage)
	}

	events := loadOnlyHeadlessSessionEvents(t, workingDir)
	if len(events) != 3 {
		t.Fatalf("expected session started, user, assistant events; got %d", len(events))
	}
	if events[1].UserMessage == nil || events[1].UserMessage.Content != "say hi" {
		t.Fatalf("unexpected user event: %#v", events[1].UserMessage)
	}
	if events[2].AssistantTurn == nil || events[2].AssistantTurn.Message != "hello" {
		t.Fatalf("unexpected assistant event: %#v", events[2].AssistantTurn)
	}
}

type headlessMCPRuntime struct{}

func (headlessMCPRuntime) Start(context.Context) error           { return nil }
func (headlessMCPRuntime) Close() error                          { return nil }
func (headlessMCPRuntime) Servers() []mcp.ServerStatus           { return nil }
func (headlessMCPRuntime) Status(string) mcp.ServerStatus        { return mcp.ServerStatus{} }
func (headlessMCPRuntime) WaitInitialScan(context.Context) error { return nil }
func (headlessMCPRuntime) ListTools(context.Context, string) ([]mcp.Tool, error) {
	return nil, nil
}
func (headlessMCPRuntime) CallTool(context.Context, string, string, map[string]any) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{}, nil
}
func (headlessMCPRuntime) Refresh(context.Context, string, ...mcp.RefreshOption) error { return nil }

func TestRunHeadless_RegistersMCPToolWhenEnabled(t *testing.T) {
	workingDir := setupHeadlessTestHome(t)
	client := &recordingHeadlessClient{events: []llm.StreamEvent{{Type: llm.StreamEventTypeDone}}}

	_, err := RunHeadless(context.Background(), HeadlessRunOptions{
		WorkingDir: workingDir,
		Config:     headlessTestConfig(),
		AgentCfg:   &agentconfig.Config{MCPConfigPaths: agentconfig.StringOrArray{"mcp.json"}},
		Client:     client,
		MCP:        headlessMCPRuntime{},
		Prompt:     "prompt",
	})
	if err != nil {
		t.Fatalf("RunHeadless() error = %v", err)
	}
	if len(client.registries) != 1 {
		t.Fatalf("expected one tool registry, got %d", len(client.registries))
	}
	if _, ok := client.registries[0].Get("call_mcp_tool"); !ok {
		t.Fatal("expected configured MCP tool to be registered for headless run")
	}
}

func TestRunHeadless_ResumesSessionConversation(t *testing.T) {
	workingDir := setupHeadlessTestHome(t)
	firstClient := &recordingHeadlessClient{events: []llm.StreamEvent{
		{Type: llm.StreamEventTypeChunk, Content: "first response"},
		{Type: llm.StreamEventTypeDone},
	}}

	first, err := RunHeadless(context.Background(), HeadlessRunOptions{
		WorkingDir: workingDir,
		Config:     headlessTestConfig(),
		Client:     firstClient,
		Prompt:     "first prompt",
	})
	if err != nil {
		t.Fatalf("first RunHeadless() error = %v", err)
	}

	secondClient := &recordingHeadlessClient{events: []llm.StreamEvent{
		{Type: llm.StreamEventTypeChunk, Content: "second response"},
		{Type: llm.StreamEventTypeDone},
	}}
	_, err = RunHeadless(context.Background(), HeadlessRunOptions{
		WorkingDir: workingDir,
		Config:     headlessTestConfig(),
		Client:     secondClient,
		SessionID:  first.SessionID,
		Prompt:     "second prompt",
	})
	if err != nil {
		t.Fatalf("second RunHeadless() error = %v", err)
	}
	if len(secondClient.messages) != 1 {
		t.Fatalf("expected one StreamChat call, got %d", len(secondClient.messages))
	}
	got := messageContents(secondClient.messages[0])
	want := []string{"first prompt", "first response", "second prompt"}
	if !containsOrderedSuffix(got, want) {
		t.Fatalf("expected conversation suffix %#v, got %#v", want, got)
	}
	if len(secondClient.opts) != 1 || len(secondClient.opts[0]) != 1 || secondClient.opts[0][0].SessionID != first.SessionID {
		t.Fatalf("expected session stream option %q, got %#v", first.SessionID, secondClient.opts)
	}
}

func TestRunHeadless_PersistsHistoricalToolActivity(t *testing.T) {
	workingDir := setupHeadlessTestHome(t)
	client := &recordingHeadlessClient{events: []llm.StreamEvent{
		{Type: llm.StreamEventTypeChunk, Content: "Let me inspect."},
		{Type: llm.StreamEventTypeToolStart, ToolCall: &llm.ToolCall{Name: "read_file", Input: map[string]any{"path": filepath.Join(workingDir, "a.go")}}},
		{Type: llm.StreamEventTypeToolEnd, ToolCall: &llm.ToolCall{Name: "read_file", Input: map[string]any{"path": filepath.Join(workingDir, "a.go")}}},
		{Type: llm.StreamEventTypeChunk, Content: " Found it."},
		{Type: llm.StreamEventTypeDone},
	}}

	if _, err := RunHeadless(context.Background(), HeadlessRunOptions{
		WorkingDir: workingDir,
		Config:     headlessTestConfig(),
		Client:     client,
		Prompt:     "inspect",
	}); err != nil {
		t.Fatalf("RunHeadless() error = %v", err)
	}

	events := loadOnlyHeadlessSessionEvents(t, workingDir)
	memory := events[len(events)-1].AssistantTurn.TurnMemory
	if memory == nil || len(memory.ToolActivity) != 1 {
		t.Fatalf("expected historical tool activity, got %#v", memory)
	}
	activity := memory.ToolActivity[0]
	if activity.Tool != "read_file" || activity.Input["path"] != "a.go" || activity.TextOffset != len("Let me inspect.") {
		t.Fatalf("unexpected historical tool activity %#v", activity)
	}
}

func TestRunHeadless_WritesJSON(t *testing.T) {
	workingDir := setupHeadlessTestHome(t)
	client := &recordingHeadlessClient{events: []llm.StreamEvent{
		{Type: llm.StreamEventTypeChunk, Content: "json response"},
		{Type: llm.StreamEventTypeDone},
	}}
	var out bytes.Buffer

	_, err := RunHeadless(context.Background(), HeadlessRunOptions{
		WorkingDir: workingDir,
		Config:     headlessTestConfig(),
		Client:     client,
		Prompt:     "prompt",
		Format:     HeadlessFormatJSON,
		Out:        &out,
	})
	if err != nil {
		t.Fatalf("RunHeadless() error = %v", err)
	}

	var decoded HeadlessRunResult
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("decode json output: %v", err)
	}
	if decoded.SessionID == "" || decoded.Text != "json response" {
		t.Fatalf("unexpected json result: %#v", decoded)
	}
}

func TestRunHeadless_PlanModeOnlyRegistersReadOnlyBuiltins(t *testing.T) {
	workingDir := setupHeadlessTestHome(t)
	client := &recordingHeadlessClient{events: []llm.StreamEvent{{Type: llm.StreamEventTypeDone}}}

	_, err := RunHeadless(context.Background(), HeadlessRunOptions{
		WorkingDir: workingDir,
		Config:     headlessTestConfig(),
		AgentCfg: &agentconfig.Config{
			DefaultMode:    agentconfig.ModePlan,
			MCPConfigPaths: agentconfig.StringOrArray{"mcp.json"},
		},
		Client: client,
		MCP:    headlessMCPRuntime{},
		Prompt: "prompt",
		Mode:   llm.ModePlan,
	})
	if err != nil {
		t.Fatalf("RunHeadless() error = %v", err)
	}
	if len(client.registries) != 1 {
		t.Fatalf("expected one tool registry, got %d", len(client.registries))
	}
	registry := client.registries[0]
	for _, name := range []string{"read_file", "glob", "grep", "web_fetch"} {
		if _, ok := registry.Get(name); !ok {
			t.Fatalf("expected %s in plan-mode registry", name)
		}
	}
	for _, name := range []string{"write_file", "edit_file", "bash", "call_mcp_tool", "delegate_task"} {
		if _, ok := registry.Get(name); ok {
			t.Fatalf("did not expect %s in plan-mode registry", name)
		}
	}
}

func TestRunHeadless_BuildModeOverridesConfiguredPlanMode(t *testing.T) {
	workingDir := setupHeadlessTestHome(t)
	client := &recordingHeadlessClient{events: []llm.StreamEvent{{Type: llm.StreamEventTypeDone}}}

	_, err := RunHeadless(context.Background(), HeadlessRunOptions{
		WorkingDir: workingDir,
		Config:     headlessTestConfig(),
		AgentCfg: &agentconfig.Config{
			DefaultMode:    agentconfig.ModePlan,
			MCPConfigPaths: agentconfig.StringOrArray{"mcp.json"},
			SubagentsDirs:  agentconfig.StringOrArray{"subagents"},
		},
		Client: client,
		MCP:    headlessMCPRuntime{},
		Prompt: "prompt",
		Mode:   llm.ModeBuild,
	})
	if err != nil {
		t.Fatalf("RunHeadless() error = %v", err)
	}
	if len(client.registries) != 1 {
		t.Fatalf("expected one tool registry, got %d", len(client.registries))
	}
	for _, name := range []string{"read_file", "glob", "grep", "web_fetch", "write_file", "edit_file", "bash", "call_mcp_tool", "delegate_task"} {
		if _, ok := client.registries[0].Get(name); !ok {
			t.Fatalf("expected %s in build-mode registry", name)
		}
	}
}

func TestRunHeadless_PlanModeSetsSystemPrompt(t *testing.T) {
	workingDir := setupHeadlessTestHome(t)
	client := &recordingHeadlessClient{events: []llm.StreamEvent{
		{Type: llm.StreamEventTypeChunk, Content: "ok"},
		{Type: llm.StreamEventTypeDone},
	}}

	_, err := RunHeadless(context.Background(), HeadlessRunOptions{
		WorkingDir: workingDir,
		Config:     headlessTestConfig(),
		Client:     client,
		Prompt:     "prompt",
		Mode:       llm.ModePlan,
	})
	if err != nil {
		t.Fatalf("RunHeadless() error = %v", err)
	}
	if len(client.messages) != 1 || len(client.messages[0]) == 0 {
		t.Fatalf("expected one StreamChat call with messages, got %#v", client.messages)
	}
	if client.messages[0][0].Role != llm.RoleSystem {
		t.Fatalf("expected first message to be system, got %q", client.messages[0][0].Role)
	}
	if !strings.Contains(client.messages[0][0].Content, "# Active mode: plan") {
		t.Fatalf("expected plan mode system prompt, got %q", client.messages[0][0].Content)
	}
}

func setupHeadlessTestHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	workingDir := filepath.Join(tmp, "project")
	if err := os.MkdirAll(workingDir, 0755); err != nil {
		t.Fatalf("create working dir: %v", err)
	}
	return workingDir
}

func headlessTestConfig() *config.ResolvedConfig {
	return &config.ResolvedConfig{
		Provider: config.ProviderOpenAI,
		APIKey:   "test-key",
		Model:    "test-model",
	}
}

func loadOnlyHeadlessSessionEvents(t *testing.T, workingDir string) []session.Event {
	t.Helper()
	store, err := session.NewStore(workingDir, "agent")
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	summaries, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected one session, got %d", len(summaries))
	}
	loaded, err := store.Load(summaries[0])
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return loaded.Events
}

func messageContents(messages []llm.Message) []string {
	contents := make([]string, 0, len(messages))
	for _, message := range messages {
		contents = append(contents, message.Content)
	}
	return contents
}

func containsOrderedSuffix(got []string, want []string) bool {
	if len(got) < len(want) {
		return false
	}
	got = got[len(got)-len(want):]
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestRunHeadless_ProgressStreamsChunksAndToolEnds(t *testing.T) {
	workingDir := setupHeadlessTestHome(t)
	client := &recordingHeadlessClient{events: []llm.StreamEvent{
		{Type: llm.StreamEventTypeChunk, Content: "Inspecting"},
		{Type: llm.StreamEventTypeToolStart, ToolCall: &llm.ToolCall{Name: "read_file", Input: map[string]any{"path": filepath.Join(workingDir, "a.go")}}},
		{Type: llm.StreamEventTypeToolEnd, ToolCall: &llm.ToolCall{Name: "read_file", Input: map[string]any{"path": filepath.Join(workingDir, "a.go")}}},
		{Type: llm.StreamEventTypeChunk, Content: " done."},
		{Type: llm.StreamEventTypeDone},
	}}
	var progress bytes.Buffer
	var out bytes.Buffer

	_, err := RunHeadless(context.Background(), HeadlessRunOptions{
		WorkingDir: workingDir,
		Config:     headlessTestConfig(),
		Client:     client,
		Prompt:     "inspect",
		Out:        &out,
		Progress:   &progress,
	})
	if err != nil {
		t.Fatalf("RunHeadless() error = %v", err)
	}

	prog := ansi.Strip(progress.String())
	if !strings.Contains(prog, "Inspecting") {
		t.Fatalf("expected progress to contain chunk text, got %q", prog)
	}
	if !strings.Contains(prog, "Read") {
		t.Fatalf("expected progress to contain tool name, got %q", prog)
	}
	if strings.Count(prog, "\n") < 1 {
		t.Fatalf("expected progress to include newlines around tool output, got %q", prog)
	}
}

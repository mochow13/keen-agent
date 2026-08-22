package tooling

import (
	"context"
	"slices"
	"testing"

	"github.com/mochow13/keen-agent/internal/agentconfig"
	replappstate "github.com/mochow13/keen-agent/internal/cli/repl/appstate"
	replpermissions "github.com/mochow13/keen-agent/internal/cli/repl/permissions"
	"github.com/mochow13/keen-agent/internal/config"
	"github.com/mochow13/keen-agent/internal/llm"
	"github.com/mochow13/keen-agent/internal/mcp"
	"github.com/mochow13/keen-agent/internal/tools"
)

type fakeLLMClient struct{}

func (f *fakeLLMClient) StreamChat(ctx context.Context, messages []llm.Message, toolRegistry *tools.Registry, opts ...llm.StreamOptions) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent)
	close(ch)
	return ch, nil
}

func (f *fakeLLMClient) Reset() {}

type fakeMCPRuntime struct{}

func (f *fakeMCPRuntime) Start(context.Context) error           { return nil }
func (f *fakeMCPRuntime) Close() error                          { return nil }
func (f *fakeMCPRuntime) Servers() []mcp.ServerStatus           { return nil }
func (f *fakeMCPRuntime) Status(string) mcp.ServerStatus        { return mcp.ServerStatus{} }
func (f *fakeMCPRuntime) WaitInitialScan(context.Context) error { return nil }
func (f *fakeMCPRuntime) ListTools(context.Context, string) ([]mcp.Tool, error) {
	return nil, nil
}
func (f *fakeMCPRuntime) CallTool(context.Context, string, string, map[string]any) (*mcp.ToolResult, error) {
	return nil, nil
}
func (f *fakeMCPRuntime) Refresh(context.Context, string, ...mcp.RefreshOption) error { return nil }

func toolNames(t *testing.T, state *replappstate.AppState) []string {
	t.Helper()
	var names []string
	for _, tool := range state.GetToolRegistry().All() {
		names = append(names, tool.Name())
	}
	slices.Sort(names)
	return names
}

func TestSetupToolRegistry_DefaultRegistersNoConditionalTools(t *testing.T) {
	work := t.TempDir()
	state := replappstate.New(&fakeLLMClient{}, work)
	permissionRequester := replpermissions.NewAutoApproveRequester()
	diffEmitter := NewDiffEmitter()
	resolvedCfg := &config.ResolvedConfig{}

	SetupToolRegistry(work, state, permissionRequester, diffEmitter, nil, resolvedCfg, nil)

	names := toolNames(t, state)
	want := []string{"bash", "edit_file", "glob", "grep", "read_file", "web_fetch", "write_file"}
	if !slices.Equal(names, want) {
		t.Errorf("registered tools = %v, want %v", names, want)
	}
}

func TestSetupToolRegistry_ExcludesListedTools(t *testing.T) {
	work := t.TempDir()
	state := replappstate.New(&fakeLLMClient{}, work)
	permissionRequester := replpermissions.NewAutoApproveRequester()
	diffEmitter := NewDiffEmitter()
	resolvedCfg := &config.ResolvedConfig{}
	agentCfg := &agentconfig.Config{
		BuiltinTools: &agentconfig.BuiltinTools{
			Exclude: []string{"bash", "write_file"},
		},
	}

	SetupToolRegistry(work, state, permissionRequester, diffEmitter, nil, resolvedCfg, agentCfg)

	names := toolNames(t, state)
	for _, excluded := range []string{"bash", "write_file"} {
		if slices.Contains(names, excluded) {
			t.Errorf("expected %q to be excluded, but it was registered", excluded)
		}
	}
	want := []string{"edit_file", "glob", "grep", "read_file", "web_fetch"}
	if !slices.Equal(names, want) {
		t.Errorf("registered tools = %v, want %v", names, want)
	}
}

func TestSetupToolRegistry_IncludesMCPToolWhenConfigDirsPresent(t *testing.T) {
	work := t.TempDir()
	state := replappstate.New(&fakeLLMClient{}, work)
	permissionRequester := replpermissions.NewAutoApproveRequester()
	diffEmitter := NewDiffEmitter()
	resolvedCfg := &config.ResolvedConfig{}
	agentCfg := &agentconfig.Config{
		MCPConfigPaths: agentconfig.StringOrArray{"./mcp.json"},
	}

	SetupToolRegistry(work, state, permissionRequester, diffEmitter, &fakeMCPRuntime{}, resolvedCfg, agentCfg)

	names := toolNames(t, state)
	if !slices.Contains(names, "call_mcp_tool") {
		t.Errorf("expected call_mcp_tool to be registered when mcp_config_paths is set")
	}
	if slices.Contains(names, "delegate_task") {
		t.Errorf("expected delegate_task to be excluded when subagents_dirs is not set")
	}
}

func TestSetupToolRegistry_IncludesDelegateToolWhenSubagentsDirsPresent(t *testing.T) {
	work := t.TempDir()
	state := replappstate.New(&fakeLLMClient{}, work)
	permissionRequester := replpermissions.NewAutoApproveRequester()
	diffEmitter := NewDiffEmitter()
	resolvedCfg := &config.ResolvedConfig{}
	agentCfg := &agentconfig.Config{
		SubagentsDirs: agentconfig.StringOrArray{"./subagents"},
	}

	SetupToolRegistry(work, state, permissionRequester, diffEmitter, nil, resolvedCfg, agentCfg)

	names := toolNames(t, state)
	if !slices.Contains(names, "delegate_task") {
		t.Errorf("expected delegate_task to be registered when subagents_dirs is set")
	}
	if slices.Contains(names, "call_mcp_tool") {
		t.Errorf("expected call_mcp_tool to be excluded when mcp_config_paths is not set")
	}
}

func TestSetupToolRegistry_DoesNotExcludeRequiredIntegrationTools(t *testing.T) {
	work := t.TempDir()
	state := replappstate.New(&fakeLLMClient{}, work)
	permissionRequester := replpermissions.NewAutoApproveRequester()
	diffEmitter := NewDiffEmitter()
	resolvedCfg := &config.ResolvedConfig{}
	agentCfg := &agentconfig.Config{
		BuiltinTools: &agentconfig.BuiltinTools{
			Exclude: []string{"call_mcp_tool", "delegate_task"},
		},
		MCPConfigPaths: agentconfig.StringOrArray{"./mcp.json"},
		SubagentsDirs:  agentconfig.StringOrArray{"./subagents"},
	}

	SetupToolRegistry(work, state, permissionRequester, diffEmitter, &fakeMCPRuntime{}, resolvedCfg, agentCfg)

	names := toolNames(t, state)
	for _, required := range []string{"call_mcp_tool", "delegate_task"} {
		if !slices.Contains(names, required) {
			t.Errorf("expected required integration tool %q to remain registered", required)
		}
	}
}

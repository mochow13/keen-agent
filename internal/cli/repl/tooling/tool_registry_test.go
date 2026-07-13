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
	"github.com/mochow13/keen-agent/internal/tools"
)

type fakeLLMClient struct{}

func (f *fakeLLMClient) StreamChat(ctx context.Context, messages []llm.Message, toolRegistry *tools.Registry, opts ...llm.StreamOptions) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent)
	close(ch)
	return ch, nil
}

func (f *fakeLLMClient) Reset() {}

func toolNames(t *testing.T, state *replappstate.AppState) []string {
	t.Helper()
	var names []string
	for _, tool := range state.GetToolRegistry().All() {
		names = append(names, tool.Name())
	}
	slices.Sort(names)
	return names
}

func TestSetupToolRegistry_DefaultRegistersAllTools(t *testing.T) {
	work := t.TempDir()
	state := replappstate.New(&fakeLLMClient{}, work)
	permissionRequester := replpermissions.NewAutoApproveRequester()
	diffEmitter := NewDiffEmitter()
	resolvedCfg := &config.ResolvedConfig{}

	SetupToolRegistry(work, state, permissionRequester, diffEmitter, nil, resolvedCfg, nil)

	names := toolNames(t, state)
	want := []string{"bash", "delegate_task", "edit_file", "glob", "grep", "read_file", "web_fetch", "write_file"}
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
	want := []string{"delegate_task", "edit_file", "glob", "grep", "read_file", "web_fetch"}
	if !slices.Equal(names, want) {
		t.Errorf("registered tools = %v, want %v", names, want)
	}
}


package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	keenmcp "github.com/mochow13/keen-agent/internal/mcp"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type mockMCPRuntime struct {
	callToolFn  func(ctx context.Context, server, tool string, arguments map[string]any) (*keenmcp.ToolResult, error)
	listToolsFn func(ctx context.Context, server string) ([]keenmcp.Tool, error)
}

func (m *mockMCPRuntime) Start(ctx context.Context) error { return nil }
func (m *mockMCPRuntime) Close() error                    { return nil }
func (m *mockMCPRuntime) Servers() []keenmcp.ServerStatus { return nil }
func (m *mockMCPRuntime) Status(string) keenmcp.ServerStatus {
	return keenmcp.ServerStatus{}
}
func (m *mockMCPRuntime) WaitInitialScan(ctx context.Context) error { return nil }
func (m *mockMCPRuntime) ListTools(ctx context.Context, server string) ([]keenmcp.Tool, error) {
	if m.listToolsFn != nil {
		return m.listToolsFn(ctx, server)
	}
	return nil, nil
}
func (m *mockMCPRuntime) Refresh(ctx context.Context, server string, opts ...keenmcp.RefreshOption) error {
	return nil
}
func (m *mockMCPRuntime) CallTool(ctx context.Context, server, tool string, arguments map[string]any) (*keenmcp.ToolResult, error) {
	if m.callToolFn != nil {
		return m.callToolFn(ctx, server, tool, arguments)
	}
	return &keenmcp.ToolResult{}, nil
}

func TestCallMCPTool_Name(t *testing.T) {
	tool := NewCallMCPTool(&mockMCPRuntime{}, &mockPermissionRequester{allow: true})
	if tool.Name() != "call_mcp_tool" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "call_mcp_tool")
	}
}

func TestCallMCPTool_ExecuteSuccess(t *testing.T) {
	runtime := &mockMCPRuntime{
		callToolFn: func(_ context.Context, server, tool string, _ map[string]any) (*keenmcp.ToolResult, error) {
			if server != "github" || tool != "list_issues" {
				t.Errorf("unexpected call: server=%s tool=%s", server, tool)
			}
			return &keenmcp.ToolResult{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: "issue #1"},
				},
			}, nil
		},
	}
	callTool := NewCallMCPTool(runtime, &mockPermissionRequester{allow: true})

	result, err := callTool.Execute(context.Background(), map[string]any{
		"server": "github",
		"tool":   "list_issues",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("Execute() result type = %T, want map[string]any", result)
	}
	if m["content"] != "issue #1" {
		t.Errorf("content = %q, want %q", m["content"], "issue #1")
	}
}

func TestCallMCPTool_RejectsMissingRequiredArguments(t *testing.T) {
	runtime := &mockMCPRuntime{
		listToolsFn: func(_ context.Context, server string) ([]keenmcp.Tool, error) {
			if server != "context7" {
				t.Fatalf("unexpected list tools server=%q", server)
			}
			return []keenmcp.Tool{
				{
					Name: "resolve-library-id",
					InputSchema: map[string]any{
						"type":     "object",
						"required": []string{"libraryName"},
						"properties": map[string]any{
							"libraryName": map[string]any{"type": "string"},
						},
					},
				},
			}, nil
		},
		callToolFn: func(_ context.Context, _, _ string, _ map[string]any) (*keenmcp.ToolResult, error) {
			t.Fatal("CallTool should not be called with missing required arguments")
			return nil, nil
		},
	}
	permissions := &mockPermissionRequester{allow: true}
	callTool := NewCallMCPTool(runtime, permissions)

	_, err := callTool.Execute(context.Background(), map[string]any{
		"server":    "context7",
		"tool":      "resolve-library-id",
		"arguments": map[string]any{},
	})
	if err == nil {
		t.Fatal("Execute() expected error for missing required arguments")
	}
	if !strings.Contains(err.Error(), "libraryName") {
		t.Fatalf("Execute() error = %v, want missing field name", err)
	}
	if !strings.Contains(err.Error(), "~/.keen-agent/skills/mcp:context7/schemas/resolve-library-id.json") {
		t.Fatalf("Execute() error = %v, want schema path", err)
	}
	if permissions.called {
		t.Fatal("permission should not be requested for invalid MCP arguments")
	}
}

func TestCallMCPTool_PermissionDenied(t *testing.T) {
	callTool := NewCallMCPTool(&mockMCPRuntime{}, &mockPermissionRequester{allow: false})

	_, err := callTool.Execute(context.Background(), map[string]any{
		"server": "github",
		"tool":   "create_issue",
	})
	if err == nil {
		t.Fatal("Execute() expected error for denied permission")
	}
}

func TestCallMCPTool_PropagatesMCPError(t *testing.T) {
	runtime := &mockMCPRuntime{
		callToolFn: func(_ context.Context, _, _ string, _ map[string]any) (*keenmcp.ToolResult, error) {
			return nil, errors.New("server disconnected")
		},
	}
	callTool := NewCallMCPTool(runtime, &mockPermissionRequester{allow: true})

	_, err := callTool.Execute(context.Background(), map[string]any{
		"server": "github",
		"tool":   "create_issue",
	})
	if err == nil {
		t.Fatal("Execute() expected error from MCP runtime")
	}
}

func TestCallMCPTool_MissingServer(t *testing.T) {
	callTool := NewCallMCPTool(&mockMCPRuntime{}, &mockPermissionRequester{allow: true})
	_, err := callTool.Execute(context.Background(), map[string]any{
		"tool": "create_issue",
	})
	if err == nil {
		t.Fatal("Execute() expected error for missing server")
	}
}

func TestCallMCPTool_MissingTool(t *testing.T) {
	callTool := NewCallMCPTool(&mockMCPRuntime{}, &mockPermissionRequester{allow: true})
	_, err := callTool.Execute(context.Background(), map[string]any{
		"server": "github",
	})
	if err == nil {
		t.Fatal("Execute() expected error for missing tool")
	}
}

func TestCallMCPTool_NilPermissionRequester(t *testing.T) {
	callTool := NewCallMCPTool(&mockMCPRuntime{}, nil)
	_, err := callTool.Execute(context.Background(), map[string]any{
		"server": "github",
		"tool":   "list_issues",
	})
	if err == nil {
		t.Fatal("Execute() expected error when permissionRequester is nil")
	}
}

func TestCallMCPTool_CheckCacheIsNoOp(t *testing.T) {
	callTool := NewCallMCPTool(&mockMCPRuntime{}, &mockPermissionRequester{allow: true})

	_, err := callTool.Execute(context.Background(), map[string]any{
		"server":     "github",
		"tool":       "list_issues",
		"checkCache": true,
	})
	if err != nil {
		t.Fatalf("Execute() with checkCache error = %v", err)
	}
}

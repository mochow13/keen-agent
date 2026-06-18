package mcp

import (
	"context"
	"encoding/json"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type Runtime interface {
	Start(context.Context) error
	Close() error
	Servers() []ServerStatus
	Status(string) ServerStatus
	WaitInitialScan(context.Context) error
	ListTools(context.Context, string) ([]Tool, error)
	CallTool(context.Context, string, string, map[string]any) (*ToolResult, error)
	Refresh(context.Context, string, ...RefreshOption) error
}

type ServerState string

const (
	StateConfigured   ServerState = "configured"
	StateConnecting   ServerState = "connecting"
	StateConnected    ServerState = "connected"
	StateDisconnected ServerState = "disconnected"
	StateAuthRequired ServerState = "auth_required"
	StateAuthFailed   ServerState = "auth_failed"
)

type ServerStatus struct {
	Name                    string
	Transport               string
	AuthType                string
	State                   ServerState
	LastConnectedAt         time.Time
	LastToolRefreshAt       time.Time
	LastError               string
	ToolCount               int
	Endpoint                string
	StdioCommand            string
	NegotiatedProtocol      string
	NegotiatedServerName    string
	NegotiatedServerVersion string
	Description             string
}

type Tool struct {
	Name         string
	Title        string
	Description  string
	InputSchema  any
	OutputSchema any
}

type ToolResult struct {
	Content           []mcpsdk.Content
	StructuredContent any
	IsError           bool
	Meta              map[string]any
}

func copyTools(tools []Tool) []Tool {
	if tools == nil {
		return nil
	}
	out := make([]Tool, len(tools))
	copy(out, tools)
	return out
}

func cloneMapString(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func schemaJSON(schema any) []byte {
	if schema == nil {
		return nil
	}
	data, err := json.Marshal(schema)
	if err != nil {
		return nil
	}
	return data
}

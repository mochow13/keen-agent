package repl

import (
	"time"

	replpermissions "github.com/mochow13/keen-agent/internal/cli/repl/permissions"
	repltooling "github.com/mochow13/keen-agent/internal/cli/repl/tooling"
	"github.com/mochow13/keen-agent/internal/llm"
	keenmcp "github.com/mochow13/keen-agent/internal/mcp"
)

type llmChunkMsg string
type llmReasoningChunkMsg string
type llmDoneMsg struct{}
type llmIncompleteMsg struct {
	err error
}
type llmErrorMsg struct {
	err error
}
type llmRetryMsg struct {
	err     error
	attempt int
}
type llmToolStartMsg struct {
	toolCall *llm.ToolCall
}
type llmToolEndMsg struct {
	toolCall *llm.ToolCall
}
type llmUsageMsg struct {
	usage *llm.TokenUsage
}
type permissionReadyMsg struct {
	req *replpermissions.Request
}
type diffReadyMsg struct {
	req repltooling.DiffRequest
}
type compactionDoneMsg struct{}
type compactionErrMsg struct {
	err error
}
type updateCheckMsg struct {
	latest string
}
type mcpStartupStatusMsg struct {
	Statuses []keenmcp.ServerStatus
	Err      error
}
type mcpConnectDoneMsg struct {
	Server string
	Status keenmcp.ServerStatus
	Err    error
}

type copyNotificationExpiredMsg struct {
	expiresAt int64
}

type bangOutputMsg struct {
	stream string
	line   string
}
type bangDoneMsg struct {
	err      error
	exitCode int
	timedOut bool
	canceled bool
	duration time.Duration
}

type btwChunkMsg string
type btwDoneMsg struct{}
type btwErrorMsg struct {
	err error
}

type adversaryChunkMsg string
type adversaryDoneMsg struct{}
type adversaryErrorMsg struct {
	err error
}

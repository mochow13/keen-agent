package repl

import (
	replpermissions "github.com/mochow13/keen-agent/internal/cli/repl/permissions"
	"github.com/mochow13/keen-agent/internal/llm"
	"github.com/mochow13/keen-agent/internal/tools"
)

type streamSegmentType string

const (
	segmentAssistant  streamSegmentType = "assistant"
	segmentReasoning  streamSegmentType = "reasoning"
	segmentToolStart  streamSegmentType = "tool_start"
	segmentToolEnd    streamSegmentType = "tool_end"
	segmentBash       streamSegmentType = "bash"
	segmentPermission streamSegmentType = "permission"
	segmentDiff       streamSegmentType = "diff"
)

type streamSegment struct {
	kind             streamSegmentType
	content          string
	toolCall         *llm.ToolCall
	command          string
	summary          string
	output           string
	renderedLines    []string
	permissionReq    *replpermissions.Request
	permissionCursor int
	diffLines        []tools.EditDiffLine
}

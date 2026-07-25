package repl

import (
	"path/filepath"
	"testing"

	replappstate "github.com/mochow13/keen-agent/internal/cli/repl/appstate"
	reploutput "github.com/mochow13/keen-agent/internal/cli/repl/output"
	"github.com/mochow13/keen-agent/internal/llm"
)

func TestHandleLLMDone_AttachesTurnMemoryToAssistantMessage(t *testing.T) {
	workingDir := t.TempDir()
	sh := NewStreamHandler(nil)
	sh.Start(make(<-chan llm.StreamEvent), "Loading...")
	sh.HandleChunk("working")
	sh.HandleToolStart(&llm.ToolCall{Name: "edit_file", Input: map[string]any{"path": "nested/a.go"}})
	sh.HandleToolEnd(&llm.ToolCall{Name: "edit_file", Input: map[string]any{"path": "nested/a.go"}})
	sh.HandleChunk("done")

	m := replModel{
		streamHandler: sh,
		showSpinner:   true,
		width:         80,
		appState:      replappstate.New(nil, workingDir),
		output:        reploutput.NewOutputBuilder(80, ""),
	}
	m.startAssistantTurnMemory()

	updated, _ := m.handleLLMDone()

	messages := updated.appState.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("expected one stored message, got %#v", messages)
	}
	if messages[0].TurnMemory == nil {
		t.Fatal("expected assistant turn memory")
	}
	if len(messages[0].TurnMemory.ToolActivity) != 1 || messages[0].TurnMemory.ToolActivity[0].TextOffset != len("working") || messages[0].TurnMemory.ToolActivity[0].Input["path"] != filepath.Join("nested", "a.go") {
		t.Fatalf("unexpected tool activity %#v", messages[0].TurnMemory.ToolActivity)
	}
}

func TestCollectHistoricalToolActivity_RecordsOffsetsTargetsAndStatus(t *testing.T) {
	workingDir := t.TempDir()
	segments := []streamSegment{
		{kind: segmentToolEnd, toolCall: &llm.ToolCall{Name: "glob", Input: map[string]any{"path": workingDir, "pattern": "**/*.go"}}},
		{kind: segmentAssistant, content: "Inspecting. "},
		{kind: segmentToolEnd, toolCall: &llm.ToolCall{Name: "read_file", Input: map[string]any{"path": filepath.Join(workingDir, "a.go")}}},
		{kind: segmentToolEnd, toolCall: &llm.ToolCall{Name: "edit_file", Error: "failed", Input: map[string]any{"path": filepath.Join(workingDir, "a.go")}}},
		{kind: segmentAssistant, content: "Done."},
		{kind: segmentBash, command: "go test ./...", toolCall: &llm.ToolCall{Name: "bash"}},
	}

	got := collectHistoricalToolActivity(segments, workingDir)
	if len(got) != 4 {
		t.Fatalf("expected four activities, got %#v", got)
	}
	if got[0].TextOffset != 0 || got[0].Input["pattern"] != "**/*.go" {
		t.Fatalf("unexpected glob activity %#v", got[0])
	}
	if got[1].TextOffset != len("Inspecting. ") || got[1].Input["path"] != "a.go" || got[1].Status != "success" {
		t.Fatalf("unexpected read activity %#v", got[1])
	}
	if got[2].TextOffset != got[1].TextOffset || got[2].Status != "error" {
		t.Fatalf("unexpected edit activity %#v", got[2])
	}
	if got[3].TextOffset != len("Inspecting. Done.") || got[3].Input["command"] != "go test ./..." {
		t.Fatalf("unexpected bash activity %#v", got[3])
	}
}

func TestCollectHistoricalToolActivity_ExtractsMCPWithoutArguments(t *testing.T) {
	segments := []streamSegment{{
		kind: segmentToolEnd,
		toolCall: &llm.ToolCall{
			Name: "call_mcp_tool",
			Input: map[string]any{
				"server":    "requested-server",
				"tool":      "requested-tool",
				"arguments": map[string]any{"secret": "do not retain"},
			},
			Output: map[string]any{
				"server":  "context7",
				"tool":    "query-docs",
				"content": "do not retain",
			},
		},
	}}

	got := collectHistoricalToolActivity(segments, "")
	if len(got) != 1 {
		t.Fatalf("expected one activity, got %#v", got)
	}
	if got[0].Tool != "call_mcp_tool" || got[0].Input["server"] != "context7" || got[0].Input["tool"] != "query-docs" || len(got[0].Input) != 2 {
		t.Fatalf("unexpected MCP activity %#v", got[0])
	}
}

func TestRebuildTurnMemoryFromSegments_DropsAbandonedOutcomes(t *testing.T) {
	workingDir := t.TempDir()
	m := replModel{appState: replappstate.New(nil, workingDir)}
	m.startAssistantTurnMemory()
	m.rebuildTurnMemoryFromSegments([]streamSegment{
		{kind: segmentToolEnd, toolCall: &llm.ToolCall{Name: "write_file", Input: map[string]any{"path": "kept.go"}}},
	})
	memory := m.consumeTurnMemory()
	if memory == nil || len(memory.ToolActivity) != 1 || memory.ToolActivity[0].Input["path"] != "kept.go" {
		t.Fatalf("expected only surviving outcome, got %#v", memory)
	}
}

func TestCollectHistoricalToolActivity_SanitizesTargets(t *testing.T) {
	segments := []streamSegment{
		{kind: segmentToolEnd, toolCall: &llm.ToolCall{Name: "web_fetch", Input: map[string]any{"url": "https://user:pass@example.com/docs?token=secret#section"}}},
		{kind: segmentBash, command: "curl -H 'Authorization: secret' example.com", toolCall: &llm.ToolCall{Name: "bash"}},
	}

	got := collectHistoricalToolActivity(segments, "")
	if got[0].Input["url"] != "https://example.com/docs" {
		t.Fatalf("expected sanitized URL, got %#v", got[0])
	}
	if len(got[1].Input) != 0 {
		t.Fatalf("expected sensitive command input to be omitted, got %#v", got[1])
	}
}

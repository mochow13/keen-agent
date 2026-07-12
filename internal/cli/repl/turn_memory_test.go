package repl

import (
	"path/filepath"
	"testing"

	replappstate "github.com/mochow13/keen-agent/internal/cli/repl/appstate"
	reploutput "github.com/mochow13/keen-agent/internal/cli/repl/output"
	"github.com/mochow13/keen-agent/internal/llm"
)

func TestTurnMemoryAccumulator_DeduplicatesChangedFiles(t *testing.T) {
	acc := newTurnMemoryAccumulator()

	acc.RecordToolEnd(&llm.ToolCall{
		Name:   "write_file",
		Output: map[string]any{"path": "a.go"},
	})
	acc.RecordToolEnd(&llm.ToolCall{
		Name:   "edit_file",
		Output: map[string]any{"path": "a.go"},
	})
	acc.RecordToolEnd(&llm.ToolCall{
		Name:   "edit_file",
		Output: map[string]any{"path": "b.go"},
	})

	memory := acc.Build()
	if memory == nil {
		t.Fatal("expected turn memory")
	}
	if len(memory.FilesChanged) != 2 {
		t.Fatalf("expected 2 changed files, got %#v", memory.FilesChanged)
	}
	if memory.FilesChanged[0] != "a.go" || memory.FilesChanged[1] != "b.go" {
		t.Fatalf("expected stable file ordering, got %#v", memory.FilesChanged)
	}
}

func TestTurnMemoryAccumulator_RecordsFailedBashOnly(t *testing.T) {
	acc := newTurnMemoryAccumulator()

	acc.RecordToolEnd(&llm.ToolCall{
		Name:   "bash",
		Output: map[string]any{"command": "go test ./...", "exit_code": 1},
	})
	acc.RecordToolEnd(&llm.ToolCall{
		Name:   "bash",
		Output: map[string]any{"command": "go build ./...", "exit_code": 0},
	})

	memory := acc.Build()
	if memory == nil {
		t.Fatal("expected turn memory")
	}
	if len(memory.FailedBash) != 1 {
		t.Fatalf("expected one failed bash command, got %#v", memory.FailedBash)
	}
	if memory.FailedBash[0].Command != "go test ./..." || memory.FailedBash[0].ExitCode != 1 {
		t.Fatalf("unexpected failed bash entry %#v", memory.FailedBash[0])
	}
}

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
	relativeFile := filepath.Join("nested", "a.go")
	m.recordToolMemory(&llm.ToolCall{
		Name:   "edit_file",
		Output: map[string]any{"path": filepath.Join(workingDir, relativeFile)},
	})
	m.recordToolMemory(&llm.ToolCall{
		Name:   "bash",
		Output: map[string]any{"command": "go test ./...", "exit_code": 1},
	})

	updated, _ := m.handleLLMDone()

	messages := updated.appState.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("expected one stored message, got %#v", messages)
	}
	if messages[0].TurnMemory == nil {
		t.Fatal("expected assistant turn memory")
	}
	if len(messages[0].TurnMemory.FilesChanged) != 1 || messages[0].TurnMemory.FilesChanged[0] != relativeFile {
		t.Fatalf("unexpected files changed %#v", messages[0].TurnMemory.FilesChanged)
	}
	if len(messages[0].TurnMemory.FailedBash) != 1 {
		t.Fatalf("unexpected failed bash entries %#v", messages[0].TurnMemory.FailedBash)
	}
	if len(messages[0].TurnMemory.ToolActivity) != 1 || messages[0].TurnMemory.ToolActivity[0].TextOffset != len("working") {
		t.Fatalf("unexpected tool activity %#v", messages[0].TurnMemory.ToolActivity)
	}
}

func TestRecordToolMemory_UsesRelativePathFromWorkingDir(t *testing.T) {
	workingDir := t.TempDir()
	m := replModel{
		appState: replappstate.New(nil, workingDir),
	}
	m.startAssistantTurnMemory()

	targetPath := filepath.Join(workingDir, "dir", "file.go")
	m.recordToolMemory(&llm.ToolCall{
		Name:   "write_file",
		Output: map[string]any{"path": targetPath},
	})

	memory := m.consumeTurnMemory()
	if memory == nil || len(memory.FilesChanged) != 1 {
		t.Fatalf("expected one changed file, got %#v", memory)
	}
	if memory.FilesChanged[0] != filepath.Join("dir", "file.go") {
		t.Fatalf("expected relative changed file path, got %#v", memory.FilesChanged)
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
	if got[0].TextOffset != 0 || got[0].Target != "**/*.go" {
		t.Fatalf("unexpected glob activity %#v", got[0])
	}
	if got[1].TextOffset != len("Inspecting. ") || got[1].Target != "a.go" || got[1].Status != "success" {
		t.Fatalf("unexpected read activity %#v", got[1])
	}
	if got[2].TextOffset != got[1].TextOffset || got[2].Status != "error" {
		t.Fatalf("unexpected edit activity %#v", got[2])
	}
	if got[3].TextOffset != len("Inspecting. Done.") || got[3].Target != "go test ./..." {
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
	if got[0].Server != "context7" || got[0].Tool != "query-docs" || got[0].Target != "" {
		t.Fatalf("unexpected MCP activity %#v", got[0])
	}
}

func TestRebuildTurnMemoryFromSegments_DropsAbandonedOutcomes(t *testing.T) {
	workingDir := t.TempDir()
	m := replModel{appState: replappstate.New(nil, workingDir)}
	m.startAssistantTurnMemory()
	m.recordToolMemory(
		&llm.ToolCall{Name: "edit_file", Input: map[string]any{"path": "abandoned.go"}})

	m.rebuildTurnMemoryFromSegments([]streamSegment{
		{kind: segmentToolEnd, toolCall: &llm.ToolCall{Name: "write_file", Input: map[string]any{"path": "kept.go"}}},
	})
	memory := m.consumeTurnMemory()
	if memory == nil || len(memory.FilesChanged) != 1 || memory.FilesChanged[0] != "kept.go" {
		t.Fatalf("expected only surviving outcome, got %#v", memory)
	}
}

func TestCollectHistoricalToolActivity_SanitizesTargets(t *testing.T) {
	segments := []streamSegment{
		{kind: segmentToolEnd, toolCall: &llm.ToolCall{Name: "web_fetch", Input: map[string]any{"url": "https://user:pass@example.com/docs?token=secret#section"}}},
		{kind: segmentBash, command: "curl -H 'Authorization: secret' example.com", toolCall: &llm.ToolCall{Name: "bash"}},
	}

	got := collectHistoricalToolActivity(segments, "")
	if got[0].Target != "https://example.com/docs" {
		t.Fatalf("expected sanitized URL, got %#v", got[0])
	}
	if got[1].Target != "" {
		t.Fatalf("expected sensitive command target to be omitted, got %#v", got[1])
	}
}

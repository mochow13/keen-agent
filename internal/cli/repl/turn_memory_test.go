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

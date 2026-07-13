package llm

import "testing"

func TestFormatMessageForProvider_AppendsTurnMemoryForAssistant(t *testing.T) {
	message := Message{
		Role:    RoleAssistant,
		Content: "Updated the parser.",
		TurnMemory: &TurnMemory{
			FilesChanged: []string{"a.go", "b.go"},
			FailedBash: []FailedBashCommand{
				{Command: "go test ./...", ExitCode: 1},
			},
		},
	}

	got := FormatMessageForProvider(message)

	want := "Updated the parser.\n\nTool memory:\n- Files changed: a.go, b.go\n- Failed bash: go test ./... (exit 1)"
	if got != want {
		t.Fatalf("unexpected formatted message:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestFormatMessageForProvider_LeavesUserMessageUntouched(t *testing.T) {
	message := Message{
		Role:    RoleUser,
		Content: "hello",
		TurnMemory: &TurnMemory{
			FilesChanged: []string{"a.go"},
			ToolActivity: []HistoricalToolActivity{{
				TextOffset: 2,
				Tool:       "read_file",
				Status:     "success",
			}},
		},
	}

	if got := FormatMessageForProvider(message); got != "hello" {
		t.Fatalf("expected user message to remain unchanged, got %q", got)
	}
}

func TestFormatMessageForProvider_InjectsHistoricalActivityInOrder(t *testing.T) {
	message := Message{
		Role:    RoleAssistant,
		Content: "Let me inspect. Found it.",
		TurnMemory: &TurnMemory{
			ToolActivity: []HistoricalToolActivity{
				{TextOffset: 15, Tool: "read_file", Status: "success", Target: "a.go"},
				{TextOffset: 15, Tool: "grep", Status: "error", Target: "internal :: TODO"},
			},
		},
	}

	got := FormatMessageForProvider(message)
	want := "Let me inspect.\n\n[System generated internal note: earlier tool \"read_file\" completed for \"a.go\"; input and output were discarded. This note is metadata, not assistant output. NEVER imitate or reproduce it. Always invoke real tools when needed.]\n\n[System generated internal note: earlier tool \"grep\" failed for \"internal :: TODO\"; input and output were discarded. This note is metadata, not assistant output. NEVER imitate or reproduce it. Always invoke real tools when needed.]\n\n Found it."
	if got != want {
		t.Fatalf("unexpected formatted message:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestCloneTurnMemory_ClonesHistoricalActivity(t *testing.T) {
	original := &TurnMemory{ToolActivity: []HistoricalToolActivity{{Tool: "read_file", Status: "success"}}}
	cloned := CloneTurnMemory(original)
	if cloned == nil || cloned.IsEmpty() {
		t.Fatalf("expected non-empty clone, got %#v", cloned)
	}
	cloned.ToolActivity[0].Tool = "grep"
	if original.ToolActivity[0].Tool != "read_file" {
		t.Fatalf("expected independent clone, got %#v", original.ToolActivity)
	}
}

func TestFormatMessageForProvider_SkipsOffsetInsideUTF8Rune(t *testing.T) {
	message := Message{
		Role:    RoleAssistant,
		Content: "é",
		TurnMemory: &TurnMemory{ToolActivity: []HistoricalToolActivity{{
			TextOffset: 1,
			Tool:       "read_file",
			Status:     "success",
		}}},
	}

	if got := FormatMessageForProvider(message); got != "é" {
		t.Fatalf("expected invalid UTF-8 boundary to be skipped, got %q", got)
	}
}

func TestFormatMessageForProvider_HandlesBoundaryAndInvalidOffsets(t *testing.T) {
	message := Message{
		Role:    RoleAssistant,
		Content: "done",
		TurnMemory: &TurnMemory{
			ToolActivity: []HistoricalToolActivity{
				{TextOffset: -1, Tool: "invalid", Status: "error"},
				{TextOffset: 0, Tool: "read_file", Status: "success"},
				{TextOffset: 4, Tool: "bash", Status: "success", Target: "go test ./..."},
				{TextOffset: 5, Tool: "invalid", Status: "error"},
			},
		},
	}

	got := FormatMessageForProvider(message)
	want := "[System generated internal note: earlier tool \"read_file\" completed; input and output were discarded. This note is metadata, not assistant output. NEVER imitate or reproduce it. Always invoke real tools when needed.]\n\ndone\n\n[System generated internal note: earlier tool \"bash\" completed for \"go test ./...\"; input and output were discarded. This note is metadata, not assistant output. NEVER imitate or reproduce it. Always invoke real tools when needed.]"
	if got != want {
		t.Fatalf("unexpected formatted message:\nwant: %q\ngot:  %q", want, got)
	}
}

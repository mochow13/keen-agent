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

func TestHistoricalMessageSteps_PreservesActivityOrder(t *testing.T) {
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

	steps := historicalMessageSteps(2, message)
	if len(steps) != 2 {
		t.Fatalf("expected two steps, got %#v", steps)
	}
	if steps[0].Text != "Let me inspect." || steps[1].Text != " Found it." {
		t.Fatalf("unexpected step text: %#v", steps)
	}
	if len(steps[0].Activities) != 2 {
		t.Fatalf("expected two grouped activities, got %#v", steps[0].Activities)
	}
	if steps[0].Activities[0].ID != "historical_2_0" || steps[0].Activities[0].Tool != "read_file" {
		t.Fatalf("unexpected first activity: %#v", steps[0].Activities[0])
	}
	if steps[0].Activities[1].ID != "historical_2_1" || steps[0].Activities[1].Tool != "grep" {
		t.Fatalf("unexpected second activity: %#v", steps[0].Activities[1])
	}
}

func TestHistoricalToolResult_UsesStatusAwareText(t *testing.T) {
	if got := historicalToolResult("success"); got != historicalToolSuccessResult {
		t.Fatalf("unexpected success result: %q", got)
	}
	if got := historicalToolResult("error"); got != historicalToolFailureResult {
		t.Fatalf("unexpected failure result: %q", got)
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

func TestHistoricalMessageSteps_HandlesBoundaryAndInvalidOffsets(t *testing.T) {
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

	steps := historicalMessageSteps(0, message)
	if len(steps) != 3 {
		t.Fatalf("expected three steps, got %#v", steps)
	}
	if steps[0].Text != "" || steps[0].Activities[0].Tool != "read_file" {
		t.Fatalf("unexpected leading step: %#v", steps[0])
	}
	if steps[1].Text != "done" || steps[1].Activities[0].Tool != "bash" {
		t.Fatalf("unexpected trailing activity step: %#v", steps[1])
	}
	if steps[2].Text != "" || len(steps[2].Activities) != 0 {
		t.Fatalf("unexpected final step: %#v", steps[2])
	}
}

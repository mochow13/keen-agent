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
		},
	}

	if got := FormatMessageForProvider(message); got != "hello" {
		t.Fatalf("expected user message to remain unchanged, got %q", got)
	}
}

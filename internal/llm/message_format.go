package llm

import (
	"strconv"
	"strings"
)

func FormatMessageForProvider(message Message) string {
	content := message.Content
	if message.Role != RoleAssistant || message.TurnMemory == nil || message.TurnMemory.IsEmpty() {
		return content
	}

	lines := make([]string, 0, 1+len(message.TurnMemory.FailedBash)+1)
	lines = append(lines, "Tool memory:")
	if len(message.TurnMemory.FilesChanged) > 0 {
		lines = append(lines, "- Files changed: "+strings.Join(message.TurnMemory.FilesChanged, ", "))
	}
	for _, failed := range message.TurnMemory.FailedBash {
		lines = append(lines, "- Failed bash: "+failed.Command+" (exit "+strconv.Itoa(failed.ExitCode)+")")
	}

	if content == "" {
		return strings.Join(lines, "\n")
	}

	return content + "\n\n" + strings.Join(lines, "\n")
}

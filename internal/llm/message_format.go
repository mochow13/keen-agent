package llm

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	historicalToolSuccessResult = `{"status":"success","output_retained":false}`
	historicalToolFailureResult = `{"status":"error","output_retained":false}`
)

type historicalMessageStep struct {
	Text       string
	Activities []historicalToolInvocation
}

type historicalToolInvocation struct {
	ID     string
	Tool   string
	Status string
}

func FormatMessageForProvider(message Message) string {
	content := message.Content
	if message.Role != RoleAssistant || message.TurnMemory == nil || message.TurnMemory.IsEmpty() {
		return content
	}

	lines := make([]string, 0, 1+len(message.TurnMemory.FailedBash)+1)
	if len(message.TurnMemory.FilesChanged) > 0 || len(message.TurnMemory.FailedBash) > 0 {
		lines = append(lines, "Tool memory:")
	}
	if len(message.TurnMemory.FilesChanged) > 0 {
		lines = append(lines, "- Files changed: "+strings.Join(message.TurnMemory.FilesChanged, ", "))
	}
	for _, failed := range message.TurnMemory.FailedBash {
		lines = append(lines, "- Failed bash: "+failed.Command+" (exit "+strconv.Itoa(failed.ExitCode)+")")
	}

	if len(lines) == 0 {
		return content
	}
	if content == "" {
		return strings.Join(lines, "\n")
	}

	return content + "\n\n" + strings.Join(lines, "\n")
}

func historicalMessageSteps(messageIndex int, message Message) []historicalMessageStep {
	if message.Role != RoleAssistant || message.TurnMemory == nil || len(message.TurnMemory.ToolActivity) == 0 {
		return []historicalMessageStep{{Text: FormatMessageForProvider(message)}}
	}

	steps := make([]historicalMessageStep, 0, len(message.TurnMemory.ToolActivity)+1)
	cursor := 0
	activityIndex := 0
	for _, activity := range message.TurnMemory.ToolActivity {
		if activity.TextOffset < cursor || activity.TextOffset > len(message.Content) || activity.Tool == "" {
			continue
		}
		if activity.TextOffset > 0 && activity.TextOffset < len(message.Content) && !utf8.RuneStart(message.Content[activity.TextOffset]) {
			continue
		}

		invocation := historicalToolInvocation{
			ID:     "historical_" + strconv.Itoa(messageIndex) + "_" + strconv.Itoa(activityIndex),
			Tool:   historicalProviderToolName(activity),
			Status: activity.Status,
		}
		activityIndex++

		if len(steps) > 0 && activity.TextOffset == cursor && len(steps[len(steps)-1].Activities) > 0 {
			steps[len(steps)-1].Activities = append(steps[len(steps)-1].Activities, invocation)
			continue
		}

		steps = append(steps, historicalMessageStep{
			Text:       message.Content[cursor:activity.TextOffset],
			Activities: []historicalToolInvocation{invocation},
		})
		cursor = activity.TextOffset
	}

	if len(steps) == 0 {
		return []historicalMessageStep{{Text: FormatMessageForProvider(message)}}
	}

	finalMessage := message
	finalMessage.Content = message.Content[cursor:]
	steps = append(steps, historicalMessageStep{Text: FormatMessageForProvider(finalMessage)})
	return steps
}

func historicalToolResult(status string) string {
	if status == "success" {
		return historicalToolSuccessResult
	}
	return historicalToolFailureResult
}

func historicalProviderToolName(activity HistoricalToolActivity) string {
	if activity.Server != "" {
		return "call_mcp_tool"
	}
	return activity.Tool
}

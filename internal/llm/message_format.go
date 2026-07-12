package llm

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"
)

func FormatMessageForProvider(message Message) string {
	content := message.Content
	if message.Role != RoleAssistant || message.TurnMemory == nil || message.TurnMemory.IsEmpty() {
		return content
	}

	content = injectHistoricalToolActivity(content, message.TurnMemory.ToolActivity)

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

func injectHistoricalToolActivity(content string, activities []HistoricalToolActivity) string {
	if len(activities) == 0 {
		return content
	}

	var result strings.Builder
	cursor := 0
	wroteActivity := false
	for _, activity := range activities {
		if activity.TextOffset < cursor || activity.TextOffset > len(content) || activity.Tool == "" {
			continue
		}
		if activity.TextOffset > 0 && activity.TextOffset < len(content) && !utf8.RuneStart(content[activity.TextOffset]) {
			continue
		}

		result.WriteString(content[cursor:activity.TextOffset])
		if wroteActivity && activity.TextOffset == cursor {
			trimBuilderSuffix(&result, "\n\n")
		}
		writeHistoricalToolActivity(&result, activity)
		cursor = activity.TextOffset
		wroteActivity = true
	}
	result.WriteString(content[cursor:])
	return strings.TrimSpace(result.String())
}

func writeHistoricalToolActivity(result *strings.Builder, activity HistoricalToolActivity) {
	payload, _ := json.Marshal(activityPayload(activity))

	if result.Len() > 0 && !strings.HasSuffix(result.String(), "\n\n") {
		if strings.HasSuffix(result.String(), "\n") {
			result.WriteByte('\n')
		} else {
			result.WriteString("\n\n")
		}
	}
	result.WriteString("<historical_tool_activity>\n")
	result.Write(payload)
	result.WriteString("\n</historical_tool_activity>\n\n")
}

func trimBuilderSuffix(result *strings.Builder, suffix string) {
	value := result.String()
	if strings.HasSuffix(value, suffix) {
		result.Reset()
		result.WriteString(strings.TrimSuffix(value, suffix))
	}
}

func activityPayload(activity HistoricalToolActivity) map[string]string {
	payload := map[string]string{
		"tool":   activity.Tool,
		"status": activity.Status,
	}
	if activity.Target != "" {
		payload["target"] = activity.Target
	}
	if activity.Server != "" {
		payload["server"] = activity.Server
	}
	return payload
}

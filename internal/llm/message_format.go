package llm

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"
)

type historicalMessageStep struct {
	Text       string
	Activities []historicalToolInvocation
}

type historicalToolInvocation struct {
	ID       string
	Tool     string
	Input    map[string]any
	Status   string
	ExitCode *int
}

func FormatMessageForProvider(message Message) string {
	return message.Content
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
			ID:       "historical_" + strconv.Itoa(messageIndex) + "_" + strconv.Itoa(activityIndex),
			Tool:     activity.Tool,
			Input:    cloneHistoricalToolInput(activity.Input),
			Status:   activity.Status,
			ExitCode: activity.ExitCode,
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

func historicalToolArguments(input map[string]any) string {
	if len(input) == 0 {
		return `{}`
	}
	return marshalHistoricalJSON(input)
}

func historicalToolResult(invocation historicalToolInvocation) string {
	result := map[string]any{"status": invocation.Status, "output_retained": false}
	if invocation.ExitCode != nil {
		result["exit_code"] = *invocation.ExitCode
	}
	return marshalHistoricalJSON(result)
}

func marshalHistoricalJSON(value any) string {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return `{}`
	}
	return strings.TrimSuffix(buffer.String(), "\n")
}

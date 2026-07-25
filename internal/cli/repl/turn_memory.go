package repl

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mochow13/keen-agent/internal/llm"
)

const maxHistoricalToolTargetBytes = 256

var sensitiveTargetPattern = regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password|passwd|authorization|credential)`)

type turnMemoryAccumulator struct {
	toolActivity []llm.HistoricalToolActivity
}

func newTurnMemoryAccumulator() *turnMemoryAccumulator {
	return &turnMemoryAccumulator{}
}

func (a *turnMemoryAccumulator) RecordToolActivity(segments []streamSegment, workingDir string) {
	if a == nil {
		return
	}
	a.toolActivity = collectHistoricalToolActivity(segments, workingDir)
}

func collectHistoricalToolActivity(segments []streamSegment, workingDir string) []llm.HistoricalToolActivity {
	textOffset := 0
	activities := make([]llm.HistoricalToolActivity, 0)

	for _, segment := range segments {
		switch segment.kind {
		case segmentAssistant:
			textOffset += len(segment.content)
		case segmentToolEnd:
			if segment.toolCall != nil {
				activities = append(activities, historicalToolActivity(segment.toolCall, textOffset, workingDir, ""))
			}
		case segmentBash:
			if segment.toolCall != nil {
				activities = append(activities, historicalToolActivity(segment.toolCall, textOffset, workingDir, segment.command))
			}
		}
	}

	return activities
}

func historicalToolActivity(toolCall *llm.ToolCall, textOffset int, workingDir, bashCommand string) llm.HistoricalToolActivity {
	activity := llm.HistoricalToolActivity{
		TextOffset: textOffset,
		Tool:       toolCall.Name,
		Input:      historicalToolInput(toolCall, workingDir, bashCommand),
		Status:     "success",
	}
	if toolCall.Error != "" {
		activity.Status = "error"
	}
	if toolCall.Name == "bash" {
		if exitCode, ok := extractIntField(toolCall.Output, "exit_code"); ok && exitCode != 0 {
			activity.ExitCode = &exitCode
		}
	}
	return activity
}

func historicalToolInput(toolCall *llm.ToolCall, workingDir, bashCommand string) map[string]any {
	if toolCall == nil {
		return nil
	}

	input := make(map[string]any)
	addPath := func(key string) {
		if value := boundedTarget(relativizePath(toolStringField(toolCall, key), workingDir)); value != "" {
			input[key] = value
		}
	}
	addString := func(key, value string) {
		if value = boundedTarget(value); value != "" {
			input[key] = value
		}
	}

	switch toolCall.Name {
	case "read_file":
		addPath("path")
		copyOptionalInt(input, toolCall.Input, "offset")
		copyOptionalInt(input, toolCall.Input, "limit")
	case "write_file", "edit_file":
		addPath("path")
	case "glob":
		addPath("path")
		addString("pattern", inputStringField(toolCall, "pattern"))
	case "grep":
		addPath("path")
		addString("pattern", inputStringField(toolCall, "pattern"))
		addString("include", inputStringField(toolCall, "include"))
		addString("output_mode", inputStringField(toolCall, "output_mode"))
	case "bash":
		if bashCommand == "" {
			bashCommand = toolStringField(toolCall, "command")
		}
		addString("command", bashCommand)
	case "web_fetch":
		addString("url", sanitizedURL(inputStringField(toolCall, "url")))
	case "call_mcp_tool":
		addString("server", toolStringField(toolCall, "server"))
		addString("tool", toolStringField(toolCall, "tool"))
	case "delegate_task":
		addString("agent", inputStringField(toolCall, "agent"))
	}
	if len(input) == 0 {
		return nil
	}
	return input
}

func copyOptionalInt(destination map[string]any, input map[string]any, key string) {
	if input == nil {
		return
	}
	switch value := input[key].(type) {
	case int:
		destination[key] = value
	case int32:
		destination[key] = int(value)
	case int64:
		destination[key] = int(value)
	case float64:
		destination[key] = int(value)
	}
}

func toolStringField(toolCall *llm.ToolCall, key string) string {
	if toolCall == nil {
		return ""
	}
	if value := extractStringField(toolCall.Output, key); value != "" {
		return value
	}
	return inputStringField(toolCall, key)
}

func inputStringField(toolCall *llm.ToolCall, key string) string {
	if toolCall == nil || toolCall.Input == nil {
		return ""
	}
	value, _ := toolCall.Input[key].(string)
	return value
}

func sanitizedURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func boundedTarget(target string) string {
	target = strings.Join(strings.Fields(target), " ")
	if sensitiveTargetPattern.MatchString(target) {
		return ""
	}
	if len(target) <= maxHistoricalToolTargetBytes {
		return target
	}

	limit := maxHistoricalToolTargetBytes - len("...")
	for limit > 0 && !utf8.RuneStart(target[limit]) {
		limit--
	}
	return target[:limit] + "..."
}

func (a *turnMemoryAccumulator) Build() *llm.TurnMemory {
	if a == nil || len(a.toolActivity) == 0 {
		return nil
	}

	return &llm.TurnMemory{ToolActivity: append([]llm.HistoricalToolActivity(nil), a.toolActivity...)}
}

func extractStringField(output any, key string) string {
	result, ok := output.(map[string]any)
	if !ok {
		return ""
	}
	value, _ := result[key].(string)
	return value
}

func extractIntField(output any, key string) (int, bool) {
	result, ok := output.(map[string]any)
	if !ok {
		return 0, false
	}

	switch value := result[key].(type) {
	case int:
		return value, true
	case int32:
		return int(value), true
	case int64:
		return int(value), true
	case float64:
		return int(value), true
	default:
		return 0, false
	}
}

func (m *replModel) startAssistantTurnMemory() {
	if m == nil {
		return
	}
	m.turnMemory = newTurnMemoryAccumulator()
}

func (m *replModel) recordHistoricalToolActivity(segments []streamSegment) {
	if m == nil || m.turnMemory == nil {
		return
	}
	m.turnMemory.RecordToolActivity(segments, m.turnMemoryWorkingDir())
}

func (m *replModel) rebuildTurnMemoryFromSegments(segments []streamSegment) {
	if m == nil || m.turnMemory == nil {
		return
	}
	m.turnMemory = newTurnMemoryAccumulator()
	m.recordHistoricalToolActivity(segments)
}

func (m *replModel) consumeTurnMemory() *llm.TurnMemory {
	if m == nil || m.turnMemory == nil {
		return nil
	}
	memory := m.turnMemory.Build()
	m.turnMemory = nil
	return memory
}

func (m *replModel) clearTurnMemory() {
	if m == nil {
		return
	}
	m.turnMemory = nil
}

func (m *replModel) turnMemoryWorkingDir() string {
	if m == nil {
		return ""
	}
	if m.appState != nil && m.appState.WorkingDir() != "" {
		return m.appState.WorkingDir()
	}
	if m.ctx != nil {
		return m.ctx.workingDir
	}
	return ""
}

func relativizePath(path string, workingDir string) string {
	if path == "" || workingDir == "" || !filepath.IsAbs(path) {
		return path
	}

	relativePath, err := filepath.Rel(workingDir, path)
	if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return path
	}
	return relativePath
}

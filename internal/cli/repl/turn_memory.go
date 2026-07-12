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
	filesChanged []string
	seenFiles    map[string]struct{}
	failedBash   []llm.FailedBashCommand
	toolActivity []llm.HistoricalToolActivity
}

func newTurnMemoryAccumulator() *turnMemoryAccumulator {
	return &turnMemoryAccumulator{
		seenFiles: make(map[string]struct{}),
	}
}

func (a *turnMemoryAccumulator) RecordToolEnd(toolCall *llm.ToolCall) {
	if a == nil || toolCall == nil {
		return
	}

	switch toolCall.Name {
	case "write_file", "edit_file":
		if toolCall.Error != "" {
			return
		}
		path := extractStringField(toolCall.Output, "path")
		if path == "" {
			path = inputStringField(toolCall, "path")
		}
		if path != "" {
			a.addFileChanged(path)
		}
	case "bash":
		if toolCall.Error != "" {
			return
		}
		exitCode, ok := extractIntField(toolCall.Output, "exit_code")
		if !ok || exitCode == 0 {
			return
		}
		command := extractStringField(toolCall.Output, "command")
		if command == "" && toolCall.Input != nil {
			command, _ = toolCall.Input["command"].(string)
		}
		if command == "" {
			return
		}
		a.failedBash = append(a.failedBash, llm.FailedBashCommand{
			Command:  command,
			ExitCode: exitCode,
		})
	}
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
		Status:     "success",
	}
	if toolCall.Error != "" {
		activity.Status = "error"
	}

	if toolCall.Name == "call_mcp_tool" {
		activity.Server = boundedTarget(toolStringField(toolCall, "server"))
		activity.Tool = boundedTarget(toolStringField(toolCall, "tool"))
		if activity.Tool == "" {
			activity.Tool = toolCall.Name
		}
		return activity
	}

	activity.Target = historicalToolTarget(toolCall, workingDir, bashCommand)
	return activity
}

func historicalToolTarget(toolCall *llm.ToolCall, workingDir, bashCommand string) string {
	var target string
	switch toolCall.Name {
	case "read_file", "write_file", "edit_file":
		target = relativizePath(toolStringField(toolCall, "path"), workingDir)
	case "glob", "grep":
		path := relativizePath(inputStringField(toolCall, "path"), workingDir)
		pattern := inputStringField(toolCall, "pattern")
		target = joinTarget(path, pattern)
	case "bash":
		target = bashCommand
		if target == "" {
			target = toolStringField(toolCall, "command")
		}
	case "web_fetch":
		target = sanitizedURL(inputStringField(toolCall, "url"))
	case "delegate_task":
		target = inputStringField(toolCall, "agent")
	}
	return boundedTarget(target)
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

func joinTarget(path, detail string) string {
	if path == "" || path == "." {
		return detail
	}
	if detail == "" {
		return path
	}
	return path + " :: " + detail
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
	if a == nil || (len(a.filesChanged) == 0 && len(a.failedBash) == 0 && len(a.toolActivity) == 0) {
		return nil
	}

	return &llm.TurnMemory{
		FilesChanged: append([]string(nil), a.filesChanged...),
		FailedBash:   append([]llm.FailedBashCommand(nil), a.failedBash...),
		ToolActivity: append([]llm.HistoricalToolActivity(nil), a.toolActivity...),
	}
}

func (a *turnMemoryAccumulator) addFileChanged(path string) {
	if path == "" {
		return
	}
	if _, exists := a.seenFiles[path]; exists {
		return
	}
	a.seenFiles[path] = struct{}{}
	a.filesChanged = append(a.filesChanged, path)
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

func (m *replModel) recordToolMemory(toolCall *llm.ToolCall) {
	if m == nil || m.turnMemory == nil {
		return
	}
	if toolCall != nil && (toolCall.Name == "write_file" || toolCall.Name == "edit_file") {
		toolCall = cloneToolCallWithRelativePath(toolCall, m.turnMemoryWorkingDir())
	}
	m.turnMemory.RecordToolEnd(toolCall)
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
	workingDir := m.turnMemoryWorkingDir()
	for _, segment := range segments {
		if segment.toolCall == nil || (segment.kind != segmentToolEnd && segment.kind != segmentBash) {
			continue
		}
		toolCall := segment.toolCall
		if toolCall.Name == "write_file" || toolCall.Name == "edit_file" {
			toolCall = cloneToolCallWithRelativePath(toolCall, workingDir)
		}
		m.turnMemory.RecordToolEnd(toolCall)
	}
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

func cloneToolCallWithRelativePath(toolCall *llm.ToolCall, workingDir string) *llm.ToolCall {
	if toolCall == nil {
		return nil
	}

	cloned := *toolCall
	if toolCall.Input != nil {
		cloned.Input = cloneInput(toolCall.Input)
	}

	result, ok := toolCall.Output.(map[string]any)
	if !ok {
		return &cloned
	}

	clonedOutput := make(map[string]any, len(result))
	for key, value := range result {
		clonedOutput[key] = value
	}
	if path, ok := clonedOutput["path"].(string); ok {
		clonedOutput["path"] = relativizePath(path, workingDir)
	}
	cloned.Output = clonedOutput
	return &cloned
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

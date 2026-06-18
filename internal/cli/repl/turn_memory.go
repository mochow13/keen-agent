package repl

import (
	"path/filepath"

	"github.com/mochow13/keen-agent/internal/llm"
)

type turnMemoryAccumulator struct {
	filesChanged []string
	seenFiles    map[string]struct{}
	failedBash   []llm.FailedBashCommand
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
		if path := extractStringField(toolCall.Output, "path"); path != "" {
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

func (a *turnMemoryAccumulator) Build() *llm.TurnMemory {
	if a == nil || (len(a.filesChanged) == 0 && len(a.failedBash) == 0) {
		return nil
	}

	return &llm.TurnMemory{
		FilesChanged: append([]string(nil), a.filesChanged...),
		FailedBash:   append([]llm.FailedBashCommand(nil), a.failedBash...),
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
	if path == "" || workingDir == "" {
		return path
	}

	relativePath, err := filepath.Rel(workingDir, path)
	if err != nil {
		return path
	}
	return relativePath
}

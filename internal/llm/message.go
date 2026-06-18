package llm

import "time"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role       Role
	Content    string
	TurnMemory *TurnMemory
}

type TurnMemory struct {
	FilesChanged []string            `json:"files_changed,omitempty"`
	FailedBash   []FailedBashCommand `json:"failed_bash,omitempty"`
}

type FailedBashCommand struct {
	Command  string `json:"command"`
	ExitCode int    `json:"exit_code"`
}

func CloneMessage(message Message) Message {
	cloned := message
	cloned.TurnMemory = CloneTurnMemory(message.TurnMemory)
	return cloned
}

func CloneMessages(messages []Message) []Message {
	result := make([]Message, len(messages))
	for i, message := range messages {
		result[i] = CloneMessage(message)
	}
	return result
}

func CloneTurnMemory(memory *TurnMemory) *TurnMemory {
	if memory == nil {
		return nil
	}

	cloned := &TurnMemory{}
	if len(memory.FilesChanged) > 0 {
		cloned.FilesChanged = append([]string(nil), memory.FilesChanged...)
	}
	if len(memory.FailedBash) > 0 {
		cloned.FailedBash = append([]FailedBashCommand(nil), memory.FailedBash...)
	}
	return cloned
}

func (m *TurnMemory) IsEmpty() bool {
	return m == nil || (len(m.FilesChanged) == 0 && len(m.FailedBash) == 0)
}

type StreamEventType string

const (
	StreamEventTypeChunk          StreamEventType = "chunk"
	StreamEventTypeReasoningChunk StreamEventType = "reasoning_chunk"
	StreamEventTypeDone           StreamEventType = "done"
	StreamEventTypeError          StreamEventType = "error"
	StreamEventTypeToolStart      StreamEventType = "tool_start"
	StreamEventTypeToolEnd        StreamEventType = "tool_end"
	StreamEventTypeUsage          StreamEventType = "usage"
	StreamEventTypeRetry          StreamEventType = "retry"
	StreamEventTypeIncomplete     StreamEventType = "incomplete"
)

type TokenUsage struct {
	InputTokens     int
	OutputTokens    int
	TotalTokens     int
	ReasoningTokens int
	CachedTokens    int
}

type StreamEvent struct {
	Type     StreamEventType
	Content  string
	Error    error
	ToolCall *ToolCall
	Usage    *TokenUsage
	Attempt  int
}

type ToolCall struct {
	Name     string
	Input    map[string]any
	Output   any
	Error    string
	Duration time.Duration
}

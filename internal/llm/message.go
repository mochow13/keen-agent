package llm

import (
	"encoding/json"
	"time"
)

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
	ToolActivity []HistoricalToolActivity `json:"tool_activity,omitempty"`
}

type HistoricalToolActivity struct {
	TextOffset int            `json:"text_offset"`
	Tool       string         `json:"tool"`
	Input      map[string]any `json:"input,omitempty"`
	Status     string         `json:"status"`
	ExitCode   *int           `json:"exit_code,omitempty"`
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
	if len(memory.ToolActivity) > 0 {
		cloned.ToolActivity = make([]HistoricalToolActivity, len(memory.ToolActivity))
		for i, activity := range memory.ToolActivity {
			cloned.ToolActivity[i] = activity
			cloned.ToolActivity[i].Input = cloneHistoricalToolInput(activity.Input)
		}
	}
	return cloned
}

func cloneHistoricalToolInput(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}

	encoded, err := json.Marshal(input)
	if err != nil {
		return map[string]any{}
	}
	var cloned map[string]any
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		return map[string]any{}
	}
	return cloned
}

func (m *TurnMemory) IsEmpty() bool {
	return m == nil || len(m.ToolActivity) == 0
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

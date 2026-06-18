package repl

import (
	"context"

	"github.com/mochow13/keen-agent/internal/llm"
	"github.com/mochow13/keen-agent/internal/tools"
)

type mockLLMClient struct {
	streamChatFunc func(ctx context.Context, messages []llm.Message, toolRegistry *tools.Registry) (<-chan llm.StreamEvent, error)
	resetCount     int
}

func (m *mockLLMClient) StreamChat(ctx context.Context, messages []llm.Message, toolRegistry *tools.Registry, opts ...llm.StreamOptions) (<-chan llm.StreamEvent, error) {
	if m.streamChatFunc != nil {
		return m.streamChatFunc(ctx, messages, toolRegistry)
	}
	ch := make(chan llm.StreamEvent)
	close(ch)
	return ch, nil
}

func (m *mockLLMClient) Reset() {
	m.resetCount++
}

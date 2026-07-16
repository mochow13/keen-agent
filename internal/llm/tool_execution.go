package llm

import (
	"context"
	"fmt"

	"github.com/mochow13/keen-agent/internal/tools"
)

func executeValidatedTool(
	ctx context.Context,
	registry *tools.Registry,
	name string,
	input map[string]any,
	eventCh chan<- StreamEvent,
) (any, error, bool) {
	if registry == nil {
		return nil, fmt.Errorf("tool registry not available"), false
	}
	tool, exists := registry.Get(name)
	if !exists {
		return nil, fmt.Errorf("tool %q not found", name), false
	}
	if err := tools.ValidateInput(ctx, tool, input); err != nil {
		return nil, err, false
	}
	eventCh <- StreamEvent{
		Type: StreamEventTypeToolStart,
		ToolCall: &ToolCall{
			Name:  name,
			Input: input,
		},
	}
	output, err := tool.Execute(ctx, input)
	return output, err, true
}

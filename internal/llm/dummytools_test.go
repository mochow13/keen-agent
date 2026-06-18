package llm

import (
	"context"
	"errors"
)

type successTool struct{}

func (t *successTool) Name() string { return "success_tool" }

func (t *successTool) Description() string { return "always succeeds" }

func (t *successTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{
				"type":        "string",
				"description": "The message to process",
			},
		},
		"required": []string{"message"},
	}
}

func (t *successTool) Execute(ctx context.Context, input any) (any, error) {
	params, ok := input.(map[string]any)
	if !ok {
		return nil, errors.New("invalid input type")
	}
	message, _ := params["message"].(string)
	return map[string]any{"result": "processed: " + message}, nil
}

type failingTool struct{}

func (t *failingTool) Name() string { return "failing_tool" }

func (t *failingTool) Description() string { return "always fails" }

func (t *failingTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
	}
}

func (t *failingTool) Execute(ctx context.Context, input any) (any, error) {
	return nil, errors.New("tool failed")
}

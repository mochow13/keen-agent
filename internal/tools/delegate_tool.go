package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

type SubagentRunner interface {
	RunDelegate(ctx context.Context, agent, task string, timeoutSeconds int) (any, error)
}

type DelegateTool struct {
	runner SubagentRunner
}

type delegateInput struct {
	Agent          string `json:"agent"`
	Task           string `json:"task"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

func NewDelegateTool(runner SubagentRunner) *DelegateTool {
	return &DelegateTool{runner: runner}
}

func (t *DelegateTool) Name() string {
	return "delegate_task"
}

func (t *DelegateTool) Description() string {
	return "Delegate a bounded task to a named subagent. Provide clear instructions and relevant paths."
}

func (t *DelegateTool) InputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"agent", "task"},
		"properties": map[string]any{
			"agent": map[string]any{
				"type":        "string",
				"description": "Name of the subagent profile to run, for example explorer.",
			},
			"task": map[string]any{
				"type":        "string",
				"description": "Bounded task for the subagent. Include relevant directories or file paths when possible.",
			},
			"timeout_seconds": map[string]any{
				"type":        "integer",
				"description": "Optional child runtime timeout in seconds.",
			},
		},
	}
}

func (t *DelegateTool) Execute(ctx context.Context, input any) (any, error) {
	if t.runner == nil {
		return nil, fmt.Errorf("subagent runner not configured")
	}
	parsed, err := parseDelegateInput(input)
	if err != nil {
		return nil, err
	}
	result, runErr := t.runner.RunDelegate(ctx, parsed.Agent, parsed.Task, parsed.TimeoutSeconds)
	if runErr != nil {
		return result, runErr
	}
	return result, nil
}

func parseDelegateInput(input any) (delegateInput, error) {
	var parsed delegateInput
	data, err := json.Marshal(input)
	if err != nil {
		return parsed, err
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return parsed, err
	}
	if parsed.Agent == "" {
		return parsed, fmt.Errorf("agent is required")
	}
	if parsed.Task == "" {
		return parsed, fmt.Errorf("task is required")
	}
	return parsed, nil
}

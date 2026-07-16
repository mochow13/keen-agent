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
	return "Delegate a bounded task to a named subagent. If you say you will ask, delegate to, or use a subagent, call this tool instead of merely describing delegation. Provide clear instructions and relevant paths."
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

func (t *DelegateTool) ValidateInput(_ context.Context, input any) error {
	_, err := parseDelegateInput(input)
	return err
}

func (t *DelegateTool) Execute(ctx context.Context, input any) (any, error) {
	if t.runner == nil {
		return nil, fmt.Errorf("subagent runner not configured")
	}
	data, _ := json.Marshal(input)
	var parsed delegateInput
	_ = json.Unmarshal(data, &parsed)
	result, runErr := t.runner.RunDelegate(ctx, parsed.Agent, parsed.Task, parsed.TimeoutSeconds)
	if runErr != nil {
		return result, runErr
	}
	return result, nil
}

func parseDelegateInput(input any) (delegateInput, error) {
	var parsed delegateInput
	params, ok := input.(map[string]any)
	if !ok {
		return parsed, fmt.Errorf("invalid input: expected map[string]any, got %T", input)
	}
	if _, exists := params["agent"]; !exists {
		return parsed, missingRequiredParameter("delegate_task", "agent", `{"agent":"<subagent profile>","task":"<bounded task with relevant paths>"}`, "Use a listed subagent profile and provide a self-contained task")
	}
	if _, exists := params["task"]; !exists {
		return parsed, missingRequiredParameter("delegate_task", "task", `{"agent":"<subagent profile>","task":"<bounded task with relevant paths>"}`, "Use a listed subagent profile and provide a self-contained task")
	}
	data, err := json.Marshal(input)
	if err != nil {
		return parsed, err
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return parsed, err
	}
	if parsed.Agent == "" {
		return parsed, fmt.Errorf("invalid input: agent must be a non-empty string")
	}
	if parsed.Task == "" {
		return parsed, fmt.Errorf("invalid input: task must be a non-empty string")
	}
	return parsed, nil
}

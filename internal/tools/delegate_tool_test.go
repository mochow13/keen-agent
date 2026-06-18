package tools

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type mockSubagentRunner struct {
	result any
	err    error

	called         bool
	agent          string
	task           string
	timeoutSeconds int
}

func (m *mockSubagentRunner) RunDelegate(ctx context.Context, agent, task string, timeoutSeconds int) (any, error) {
	m.called = true
	m.agent = agent
	m.task = task
	m.timeoutSeconds = timeoutSeconds
	return m.result, m.err
}

func TestDelegateTool_Metadata(t *testing.T) {
	tool := NewDelegateTool(&mockSubagentRunner{})

	if tool.Name() != "delegate_task" {
		t.Fatalf("Name() = %q, want %q", tool.Name(), "delegate_task")
	}
	if tool.Description() == "" {
		t.Fatal("Description() should not be empty")
	}
}

func TestDelegateTool_InputSchema(t *testing.T) {
	tool := NewDelegateTool(&mockSubagentRunner{})
	schema := tool.InputSchema()

	if schema["type"] != "object" {
		t.Fatalf("schema type = %v, want object", schema["type"])
	}
	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("required type = %T, want []string", schema["required"])
	}
	if !reflect.DeepEqual(required, []string{"agent", "task"}) {
		t.Fatalf("required = %v, want [agent task]", required)
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties type = %T, want map[string]any", schema["properties"])
	}
	for _, name := range []string{"agent", "task", "timeout_seconds"} {
		if _, ok := properties[name]; !ok {
			t.Fatalf("properties missing %q", name)
		}
	}
	if _, ok := properties["max_turns"]; ok {
		t.Fatal("schema should not include max_turns")
	}
	if _, ok := properties["return_format"]; ok {
		t.Fatal("schema should not include return_format")
	}
}

func TestDelegateTool_ExecutePassesInputToRunner(t *testing.T) {
	runner := &mockSubagentRunner{result: map[string]any{"status": "completed"}}
	tool := NewDelegateTool(runner)

	result, err := tool.Execute(context.Background(), map[string]any{
		"agent":           "explorer",
		"task":            "Inspect internal/tools.",
		"timeout_seconds": 30,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !runner.called {
		t.Fatal("runner was not called")
	}
	if runner.agent != "explorer" {
		t.Fatalf("agent = %q, want explorer", runner.agent)
	}
	if runner.task != "Inspect internal/tools." {
		t.Fatalf("task = %q, want delegated task", runner.task)
	}
	if runner.timeoutSeconds != 30 {
		t.Fatalf("timeoutSeconds = %d, want 30", runner.timeoutSeconds)
	}
	if !reflect.DeepEqual(result, runner.result) {
		t.Fatalf("result = %#v, want %#v", result, runner.result)
	}
}

func TestDelegateTool_ExecuteAllowsOmittedTimeout(t *testing.T) {
	runner := &mockSubagentRunner{result: "ok"}
	tool := NewDelegateTool(runner)

	_, err := tool.Execute(context.Background(), map[string]any{
		"agent": "explorer",
		"task":  "Inspect docs.",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if runner.timeoutSeconds != 0 {
		t.Fatalf("timeoutSeconds = %d, want 0", runner.timeoutSeconds)
	}
}

func TestDelegateTool_ExecuteReturnsRunnerPartialResultOnError(t *testing.T) {
	wantErr := errors.New("subagent failed")
	runner := &mockSubagentRunner{
		result: map[string]any{"status": "error"},
		err:    wantErr,
	}
	tool := NewDelegateTool(runner)

	result, err := tool.Execute(context.Background(), map[string]any{
		"agent": "explorer",
		"task":  "Inspect docs.",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute() error = %v, want %v", err, wantErr)
	}
	if !reflect.DeepEqual(result, runner.result) {
		t.Fatalf("result = %#v, want %#v", result, runner.result)
	}
}

func TestDelegateTool_ExecuteRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantErr string
	}{
		{name: "missing agent", input: map[string]any{"task": "Inspect docs."}, wantErr: "agent is required"},
		{name: "missing task", input: map[string]any{"agent": "explorer"}, wantErr: "task is required"},
		{name: "non-integer timeout", input: map[string]any{"agent": "explorer", "task": "Inspect docs.", "timeout_seconds": "30"}, wantErr: "cannot unmarshal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockSubagentRunner{}
			tool := NewDelegateTool(runner)

			_, err := tool.Execute(context.Background(), tt.input)
			if err == nil {
				t.Fatal("Execute() expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Execute() error = %v, want containing %q", err, tt.wantErr)
			}
			if runner.called {
				t.Fatal("runner should not be called for invalid input")
			}
		})
	}
}

func TestDelegateTool_ExecuteRejectsMissingRunner(t *testing.T) {
	tool := NewDelegateTool(nil)

	_, err := tool.Execute(context.Background(), map[string]any{
		"agent": "explorer",
		"task":  "Inspect docs.",
	})
	if err == nil {
		t.Fatal("Execute() expected error")
	}
	if !strings.Contains(err.Error(), "subagent runner not configured") {
		t.Fatalf("Execute() error = %v, want runner configuration error", err)
	}
}

package tools

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type delegateCall struct {
	agent          string
	task           string
	timeoutSeconds int
}

type mockSubagentRunner struct {
	result any
	err    error

	mu     sync.Mutex
	calls  []delegateCall
	byTask map[string]struct {
		result any
		err    error
	}

	started chan struct{}
	release chan struct{}
}

func (m *mockSubagentRunner) RunDelegate(ctx context.Context, agent, task string, timeoutSeconds int) (any, error) {
	m.mu.Lock()
	m.calls = append(m.calls, delegateCall{agent: agent, task: task, timeoutSeconds: timeoutSeconds})
	m.mu.Unlock()
	if m.started != nil {
		m.started <- struct{}{}
	}
	if m.release != nil {
		select {
		case <-m.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.byTask != nil {
		if outcome, ok := m.byTask[task]; ok {
			return outcome.result, outcome.err
		}
	}
	return m.result, m.err
}

func (m *mockSubagentRunner) recordedCalls() []delegateCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]delegateCall(nil), m.calls...)
}

func TestDelegateTool_Metadata(t *testing.T) {
	tool := NewDelegateTool(&mockSubagentRunner{})

	if tool.Name() != "delegate_task" {
		t.Fatalf("Name() = %q, want %q", tool.Name(), "delegate_task")
	}
	if !strings.Contains(tool.Description(), "up to 10") || !strings.Contains(tool.Description(), "parallel") {
		t.Fatalf("Description() = %q, want parallel limit", tool.Description())
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
	if !reflect.DeepEqual(required, []string{"tasks"}) {
		t.Fatalf("required = %v, want [tasks]", required)
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties type = %T, want map[string]any", schema["properties"])
	}
	tasks, ok := properties["tasks"].(map[string]any)
	if !ok {
		t.Fatalf("tasks type = %T, want map[string]any", properties["tasks"])
	}
	if tasks["maxItems"] != 10 {
		t.Fatalf("tasks.maxItems = %v, want 10", tasks["maxItems"])
	}
	items, ok := tasks["items"].(map[string]any)
	if !ok {
		t.Fatalf("tasks.items type = %T, want map[string]any", tasks["items"])
	}
	itemProperties, ok := items["properties"].(map[string]any)
	if !ok {
		t.Fatalf("items.properties type = %T, want map[string]any", items["properties"])
	}
	for _, field := range []string{"agent", "task", "timeout_seconds"} {
		if _, ok := itemProperties[field]; !ok {
			t.Fatalf("items.properties missing %q", field)
		}
	}
}

func TestDelegateTool_ExecutePassesTasksToRunner(t *testing.T) {
	runner := &mockSubagentRunner{result: "ok"}
	tool := NewDelegateTool(runner)

	output, err := tool.Execute(context.Background(), map[string]any{
		"tasks": []any{
			map[string]any{"agent": "explorer", "task": "inspect internal/llm", "timeout_seconds": 120},
			map[string]any{"agent": "reviewer", "task": "review README.md"},
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	calls := runner.recordedCalls()
	if len(calls) != 2 {
		t.Fatalf("recorded calls = %d, want 2", len(calls))
	}
	byAgent := make(map[string]delegateCall, len(calls))
	for _, call := range calls {
		byAgent[call.agent] = call
	}
	explorerCall, ok := byAgent["explorer"]
	if !ok || explorerCall.task != "inspect internal/llm" || explorerCall.timeoutSeconds != 120 {
		t.Fatalf("explorer call = %+v, want task inspect internal/llm with timeout 120", explorerCall)
	}
	reviewerCall, ok := byAgent["reviewer"]
	if !ok || reviewerCall.task != "review README.md" || reviewerCall.timeoutSeconds != 0 {
		t.Fatalf("reviewer call = %+v, want task review README.md with timeout 0", reviewerCall)
	}

	payload, ok := output.(map[string]any)
	if !ok {
		t.Fatalf("output type = %T, want map[string]any", output)
	}
	if payload["completed"] != 2 || payload["failed"] != 0 {
		t.Fatalf("counts = completed %v failed %v, want 2/0", payload["completed"], payload["failed"])
	}
	results, ok := payload["results"].([]delegateResult)
	if !ok {
		t.Fatalf("results type = %T, want []delegateResult", payload["results"])
	}
	if results[0].Agent != "explorer" || results[1].Agent != "reviewer" {
		t.Fatalf("results order = %+v, want input order", results)
	}
}

func TestDelegateTool_ExecuteRunsTasksInParallel(t *testing.T) {
	runner := &mockSubagentRunner{
		result:  "ok",
		started: make(chan struct{}, 2),
		release: make(chan struct{}),
	}
	tool := NewDelegateTool(runner)

	done := make(chan error, 1)
	go func() {
		_, err := tool.Execute(context.Background(), map[string]any{
			"tasks": []any{
				map[string]any{"agent": "explorer", "task": "first"},
				map[string]any{"agent": "reviewer", "task": "second"},
			},
		})
		done <- err
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-runner.started:
		case <-time.After(2 * time.Second):
			t.Fatalf("task %d did not start before earlier tasks finished", i+1)
		}
	}
	close(runner.release)
	if err := <-done; err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestDelegateTool_ExecuteReturnsPerTaskErrors(t *testing.T) {
	runner := &mockSubagentRunner{
		byTask: map[string]struct {
			result any
			err    error
		}{
			"good": {result: "fine"},
			"bad":  {err: errors.New("boom")},
		},
	}
	tool := NewDelegateTool(runner)

	output, err := tool.Execute(context.Background(), map[string]any{
		"tasks": []any{
			map[string]any{"agent": "explorer", "task": "good"},
			map[string]any{"agent": "reviewer", "task": "bad"},
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, want per-task errors", err)
	}

	payload, ok := output.(map[string]any)
	if !ok {
		t.Fatalf("output type = %T, want map[string]any", output)
	}
	if payload["completed"] != 1 || payload["failed"] != 1 {
		t.Fatalf("counts = completed %v failed %v, want 1/1", payload["completed"], payload["failed"])
	}
	results, ok := payload["results"].([]delegateResult)
	if !ok {
		t.Fatalf("results type = %T, want []delegateResult", payload["results"])
	}
	if results[0].Error != "" || results[0].Result != "fine" {
		t.Fatalf("first result = %+v, want success", results[0])
	}
	if results[1].Error != "boom" {
		t.Fatalf("second result error = %q, want boom", results[1].Error)
	}
	failedByAgent, ok := payload["failed_by_agent"].(map[string]int)
	if !ok || failedByAgent["reviewer"] != 1 {
		t.Fatalf("failed_by_agent = %v, want reviewer:1", payload["failed_by_agent"])
	}
}

func TestDelegateTool_ValidateInputRejectsInvalidInput(t *testing.T) {
	tool := NewDelegateTool(&mockSubagentRunner{})

	tests := []struct {
		name  string
		input any
		want  string
	}{
		{name: "non map", input: "explorer", want: "expected map[string]any"},
		{name: "missing tasks", input: map[string]any{}, want: "missing required \"tasks\" parameter"},
		{name: "empty tasks", input: map[string]any{"tasks": []any{}}, want: "at least one task"},
		{name: "missing agent", input: map[string]any{"tasks": []any{map[string]any{"task": "x"}}}, want: "tasks[0].agent"},
		{name: "missing task", input: map[string]any{"tasks": []any{map[string]any{"agent": "explorer"}}}, want: "tasks[0].task"},
	}

	tooMany := make([]any, 11)
	for i := range tooMany {
		tooMany[i] = map[string]any{"agent": "explorer", "task": "x"}
	}
	tests = append(tests, struct {
		name  string
		input any
		want  string
	}{name: "too many tasks", input: map[string]any{"tasks": tooMany}, want: "at most 10"})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tool.ValidateInput(context.Background(), tt.input)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateInput() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestDelegateTool_ExecuteRejectsMissingRunner(t *testing.T) {
	tool := NewDelegateTool(nil)

	_, err := tool.Execute(context.Background(), map[string]any{
		"tasks": []any{map[string]any{"agent": "explorer", "task": "inspect"}},
	})
	if err == nil || !strings.Contains(err.Error(), "subagent runner not configured") {
		t.Fatalf("Execute() error = %v, want runner not configured", err)
	}
}

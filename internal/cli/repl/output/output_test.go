package output

import (
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mochow13/keen-agent/internal/llm"
)

func TestNewOutputBuilder(t *testing.T) {
	ob := NewOutputBuilder(80, "")

	if ob.width != 80 {
		t.Errorf("width = %d, want 80", ob.width)
	}

	if !ob.IsEmpty() {
		t.Error("new OutputBuilder should be empty")
	}
}

func TestOutputBuilder_AddLine(t *testing.T) {
	ob := NewOutputBuilder(80, "")
	ob.AddLine("hello")
	ob.AddLine("world")

	lines := ob.GetLines()
	if len(lines) != 2 {
		t.Errorf("len(lines) = %d, want 2", len(lines))
	}

	if lines[0] != "hello" {
		t.Errorf("lines[0] = %q, want 'hello'", lines[0])
	}

	if lines[1] != "world" {
		t.Errorf("lines[1] = %q, want 'world'", lines[1])
	}
}

func TestOutputBuilder_AddEmptyLine(t *testing.T) {
	ob := NewOutputBuilder(80, "")
	ob.AddLine("hello")
	ob.AddEmptyLine()
	ob.AddLine("world")

	lines := ob.GetLines()
	if len(lines) != 3 {
		t.Errorf("len(lines) = %d, want 3", len(lines))
	}

	if lines[1] != "" {
		t.Errorf("lines[1] = %q, want empty string", lines[1])
	}
}

func TestOutputBuilder_SetLines(t *testing.T) {
	ob := NewOutputBuilder(80, "")
	ob.SetLines([]string{"a", "b", "c"})

	lines := ob.GetLines()
	if len(lines) != 3 {
		t.Errorf("len(lines) = %d, want 3", len(lines))
	}

	if lines[0] != "a" {
		t.Errorf("lines[0] = %q, want 'a'", lines[0])
	}
}

func TestOutputBuilder_Join(t *testing.T) {
	ob := NewOutputBuilder(80, "")
	ob.AddLine("line1")
	ob.AddLine("line2")
	ob.AddLine("line3")

	result := ob.Join()
	expected := "line1\nline2\nline3"
	if result != expected {
		t.Errorf("Join() = %q, want %q", result, expected)
	}
}

func TestOutputBuilder_IsEmpty(t *testing.T) {
	ob := NewOutputBuilder(80, "")

	if !ob.IsEmpty() {
		t.Error("IsEmpty() should be true for new builder")
	}

	ob.AddLine("content")

	if ob.IsEmpty() {
		t.Error("IsEmpty() should be false after adding line")
	}
}

func TestFormatToolInput_ShowsRelativePathToWorkingDir(t *testing.T) {
	workingDir := filepath.Join(string(filepath.Separator), "tmp", "project")
	got := FormatToolInput("read_file", map[string]any{
		"path": filepath.Join(workingDir, "internal", "cli", "repl", "output.go"),
	}, workingDir)

	if got != "path=internal/cli/repl/output.go" {
		t.Fatalf("expected relative path display, got %q", got)
	}
}

func TestFormatToolInput_KeepsRelativePathInput(t *testing.T) {
	got := FormatToolInput("read_file", map[string]any{"path": "internal/cli/repl/output.go"}, "/tmp/project")

	if got != "path=internal/cli/repl/output.go" {
		t.Fatalf("expected relative input path to remain unchanged, got %q", got)
	}
}

func TestFormatToolInput_WriteFileShowsOnlyRelativePath(t *testing.T) {
	workingDir := filepath.Join(string(filepath.Separator), "tmp", "project")
	got := FormatToolInput("write_file", map[string]any{
		"path":    filepath.Join(workingDir, "README.md"),
		"content": "ignored",
	}, workingDir)

	if got != "path=README.md" {
		t.Fatalf("expected write_file UI to show only relative path, got %q", got)
	}
}

func TestFormatToolInput_SeparatesArgumentsWithDots(t *testing.T) {
	got := FormatToolInput("grep", map[string]any{
		"include":     "*.go",
		"output_mode": "content",
		"path":        "internal/cli/repl",
		"pattern":     "FormatToolInput",
	}, "/tmp/project")

	expected := "path=internal/cli/repl • pattern=FormatToolInput"
	if got != expected {
		t.Fatalf("expected dot-separated tool arguments, got %q", got)
	}
}

func TestFormatToolInput_CallMCPToolShowsOnlyServerAndTool(t *testing.T) {
	got := FormatToolInput("call_mcp_tool", map[string]any{
		"server": "context7",
		"tool":   "query-docs",
		"arguments": map[string]any{
			"query":     "React useEffect API reference",
			"libraryId": "/reactjs/react.dev",
		},
		"checkCache": false,
	}, "/tmp/project")

	if got != "context7/query-docs" {
		t.Fatalf("expected call_mcp_tool input to show only server/tool, got %q", got)
	}
}

func TestFormatToolInput_DelegateTaskShowsOnlyAgent(t *testing.T) {
	got := FormatToolInput("delegate_task", map[string]any{
		"agent":           "explorer",
		"task":            "Inspect internal/subagents and summarize the design.",
		"timeout_seconds": 120,
	}, "/tmp/project")

	if got != "agent=explorer" {
		t.Fatalf("expected delegate_task input to show only agent, got %q", got)
	}
}

func TestFormatToolEnd_DoesNotAddTrailingNewline(t *testing.T) {
	got := FormatToolEnd(&llm.ToolCall{Name: "call_mcp_tool", Duration: 5})

	if strings.HasSuffix(got, "\n") {
		t.Fatalf("expected no trailing newline in tool end, got %q", got)
	}
}

func TestOutputBuilder_AddUserInput(t *testing.T) {
	ob := NewOutputBuilder(80, "")
	style := lipgloss.NewStyle()

	ob.AddUserInput("hello", style)

	lines := ob.GetLines()
	// 1 top padding + 1 content line + 1 bottom padding + 1 trailing empty line
	if len(lines) != 4 {
		t.Errorf("len(lines) = %d, want 4 (top pad + content + bottom pad + empty)", len(lines))
	}

	if !strings.Contains(ob.Join(), "hello") {
		t.Errorf("output should contain 'hello', got %q", ob.Join())
	}
}

func TestOutputBuilder_AddUserInput_MultiLine(t *testing.T) {
	ob := NewOutputBuilder(80, "")
	style := lipgloss.NewStyle()

	ob.AddUserInput("line1\nline2", style)

	lines := ob.GetLines()
	// 1 top padding + 2 content lines + 1 bottom padding + 1 trailing empty line
	if len(lines) != 5 {
		t.Errorf("len(lines) = %d, want 5", len(lines))
	}

	joined := ob.Join()
	if !strings.Contains(joined, "line1") {
		t.Errorf("output should contain 'line1', got %q", joined)
	}

	if !strings.Contains(joined, "line2") {
		t.Errorf("output should contain 'line2', got %q", joined)
	}
}

func TestOutputBuilder_AddUserInput_WrappedLinesAreIndented(t *testing.T) {
	ob := NewOutputBuilder(12, "")
	style := lipgloss.NewStyle()

	ob.AddUserInput("hello world", style)

	lines := ob.GetLines()
	if len(lines) != 5 {
		t.Fatalf("len(lines) = %d, want 5", len(lines))
	}

	line := ansi.Strip(lines[2])
	if strings.Contains(line, "hello") || !strings.HasPrefix(line, "    world") {
		t.Errorf("wrapped line should be indented, got %q", line)
	}
}

func TestOutputBuilder_AddUserInput_FitsWithinWidthAfterPadding(t *testing.T) {
	ob := NewOutputBuilder(12, "")
	style := lipgloss.NewStyle()

	ob.AddUserInput("hello world", style)

	for _, line := range ob.GetLines() {
		if width := lipgloss.Width(line); width > 12 {
			t.Fatalf("line width = %d, want <= 12: %q", width, line)
		}
	}
}

func TestOutputBuilder_AddAssistantResponse(t *testing.T) {
	ob := NewOutputBuilder(80, "")
	style := lipgloss.NewStyle()

	ob.AddAssistantResponse("response text", style)

	lines := ob.GetLines()
	if len(lines) != 2 {
		t.Errorf("len(lines) = %d, want 2 (response + empty)", len(lines))
	}

	if !strings.Contains(lines[0], "response text") {
		t.Errorf("lines[0] should contain 'response text', got %q", lines[0])
	}
}

func TestOutputBuilder_AddAssistantResponse_MultiLine(t *testing.T) {
	ob := NewOutputBuilder(80, "")
	style := lipgloss.NewStyle()

	ob.AddAssistantResponse("line1\nline2\nline3", style)

	lines := ob.GetLines()
	if len(lines) != 4 {
		t.Errorf("len(lines) = %d, want 4", len(lines))
	}
}

func TestOutputBuilder_AddError(t *testing.T) {
	ob := NewOutputBuilder(80, "")
	style := lipgloss.NewStyle()

	ob.AddError("something went wrong", style)

	lines := ob.GetLines()
	if len(lines) != 2 {
		t.Errorf("len(lines) = %d, want 2 (error + empty)", len(lines))
	}

	if !strings.Contains(lines[0], "Error: something went wrong") {
		t.Errorf("lines[0] should contain error message, got %q", lines[0])
	}
}

func TestOutputBuilder_AddStyledLine(t *testing.T) {
	ob := NewOutputBuilder(80, "")
	style := lipgloss.NewStyle()

	ob.AddStyledLine("styled content", style)

	lines := ob.GetLines()
	if len(lines) != 1 {
		t.Errorf("len(lines) = %d, want 1", len(lines))
	}

	if !strings.Contains(lines[0], "styled content") {
		t.Errorf("lines[0] should contain 'styled content', got %q", lines[0])
	}
}

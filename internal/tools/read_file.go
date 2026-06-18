package tools

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mochow13/keen-agent/internal/filesystem"
)

const (
	maxFileSize   = 25 * 1024 * 1024 // 25MB
	defaultLimit  = 1000
	maxLineLength = 1000
)

type ReadFileTool struct {
	guard               *filesystem.Guard
	permissionRequester PermissionRequester
}

func NewReadFileTool(guard *filesystem.Guard, permissionRequester PermissionRequester) *ReadFileTool {
	return &ReadFileTool{
		guard:               guard,
		permissionRequester: permissionRequester,
	}
}

func (t *ReadFileTool) Name() string {
	return "read_file"
}

func (t *ReadFileTool) Description() string {
	return `Read a UTF-8 text file after filesystem policy + user permission checks.

- Use this when you know the exact file path and need its contents
- Do not use this when you are unsure of the filename — use glob to find it first
- Do not use this to search for content across files — use grep instead
- By default, this returns up to 1000 lines from the start of the file
- Use offset and limit to read a specific line range
- Call this tool in parallel when you already know multiple files you need to inspect
- Avoid many tiny repeated reads. If you need surrounding context, read a larger window with limit
- If the result is truncated, continue with offset only when the missing section is needed
- For code tracing, read the primary implementation file first, then callers/tests only if needed

IMPORTANT:
- The file must be valid UTF-8 text and under 25 MB. Binary files and files with invalid UTF-8 are rejected.
- Content is returned with line numbers as "N: text". When copying text into edit_file oldString, do not include the line number prefix.
- Long lines are truncated to keep tool results bounded`
}

func (t *ReadFileTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Absolute or relative path to the file to read",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "Optional 1-based line number to start reading from (defaults to 1)",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Optional maximum number of lines to return (defaults to 1000)",
			},
		},
		"required":             []string{"path"},
		"additionalProperties": false,
	}
}

func (t *ReadFileTool) Execute(ctx context.Context, input any) (any, error) {
	params, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input: expected map[string]any, got %T", input)
	}

	pathValue, ok := params["path"]
	if !ok {
		return nil, fmt.Errorf("invalid input: missing required 'path' parameter")
	}

	path, ok := pathValue.(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("invalid input: path must be a non-empty string")
	}

	offset, err := optionalPositiveInt(params, "offset", 1)
	if err != nil {
		return nil, err
	}
	limit, err := optionalPositiveInt(params, "limit", defaultLimit)
	if err != nil {
		return nil, err
	}

	resolvedPath, err := t.guard.ResolvePath(path)
	if err != nil {
		return nil, fmt.Errorf("path resolution failed: %w", err)
	}

	permission := t.guard.CheckPath(path, "read")

	switch permission {
	case filesystem.PermissionDenied:
		return nil, fmt.Errorf("permission denied by policy: path %q is blocked", path)
	case filesystem.PermissionPending:
		if t.permissionRequester == nil {
			return nil, fmt.Errorf("permission denied: user approval required but not available")
		}
		allowed, err := t.permissionRequester.RequestPermission(ctx, t.Name(), path, resolvedPath, false)
		if err != nil {
			return nil, fmt.Errorf("permission request failed: %w", err)
		}
		if !allowed {
			return nil, fmt.Errorf("permission denied by user: read access rejected for path %q", path)
		}
	}

	rawContent, err := readFileContent(resolvedPath)
	if err != nil {
		return nil, err
	}

	content, totalLines, truncated, err := formatFileContent(rawContent, offset, limit)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"path":        resolvedPath,
		"content":     content,
		"bytes_read":  len(rawContent),
		"offset":      offset,
		"limit":       limit,
		"total_lines": totalLines,
		"truncated":   truncated,
	}, nil
}

func optionalPositiveInt(params map[string]any, name string, defaultValue int) (int, error) {
	value, exists := params[name]
	if !exists {
		return defaultValue, nil
	}

	switch v := value.(type) {
	case int:
		if v > 0 {
			return v, nil
		}
	case int64:
		if v > 0 {
			return int(v), nil
		}
	case float64:
		i := int(v)
		if v == float64(i) && i > 0 {
			return i, nil
		}
	}

	return 0, fmt.Errorf("invalid input: %s must be a positive integer", name)
}

func readFileContent(path string) ([]byte, error) {
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not found: file %q does not exist", path)
		}
		return nil, fmt.Errorf("not accessible: %w", err)
	}

	if stat.IsDir() {
		return nil, fmt.Errorf("not a file: %q is a directory", path)
	}

	if stat.Size() > maxFileSize {
		return nil, fmt.Errorf("file too large: %q is %d bytes (max %d)", path, stat.Size(), maxFileSize)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read failed: %w", err)
	}

	if containsNullByte(content) {
		return nil, fmt.Errorf("not a text file: %q contains null bytes (likely binary)", path)
	}

	if !utf8.Valid(content) {
		return nil, fmt.Errorf("not a text file: %q contains invalid UTF-8", path)
	}

	return content, nil
}

func containsNullByte(content []byte) bool {
	return slices.Contains(content, 0x00)
}

func formatFileContent(content []byte, offset, limit int) (string, int, bool, error) {
	lines := splitLines(string(content))
	total := len(lines)
	if total == 0 {
		if offset > 1 {
			return "", total, false, fmt.Errorf("offset %d is out of range for empty file", offset)
		}
		return "", total, false, nil
	}
	if offset > total {
		return "", total, false, fmt.Errorf("offset %d is out of range for file with %d lines", offset, total)
	}

	start := offset - 1
	end := min(start+limit, total)
	truncated := end < total

	numbered := make([]string, 0, end-start)
	for i, line := range lines[start:end] {
		runes := []rune(line)
		if len(runes) > maxLineLength {
			line = string(runes[:maxLineLength]) + "... (line truncated)"
		}
		numbered = append(numbered, fmt.Sprintf("%d: %s", start+i+1, line))
	}

	return strings.Join(numbered, "\n"), total, truncated, nil
}

func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

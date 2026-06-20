package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	keenmcp "github.com/mochow13/keen-agent/internal/mcp"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type CallMCPTool struct {
	manager             keenmcp.Runtime
	permissionRequester PermissionRequester
}

func NewCallMCPTool(manager keenmcp.Runtime, permissionRequester PermissionRequester) *CallMCPTool {
	return &CallMCPTool{
		manager:             manager,
		permissionRequester: permissionRequester,
	}
}

func (t *CallMCPTool) Name() string {
	return "call_mcp_tool"
}

func (t *CallMCPTool) Description() string {
	return `Call a tool on a connected MCP (Model Context Protocol) server.

Before calling, you must read the server's skill file to discover available tools, then you must read
the tool's schema file to understand the required arguments:
- Skill file:   ~/.keen-agent/skills/mcp:<server>/SKILL.md
- Schema file:  ~/.keen-agent/skills/mcp:<server>/schemas/<tool>.json

IMPORTANT:
- Use the bare configured server name, for example "context7", not the skill name
  "mcp:context7" and not a combined path like "mcp:context7/resolve-library-id".
- Use the exact MCP tool name as it appears in the skill's "Available tools"
  table, for example "resolve-library-id". Do not guess, abbreviate, or
  transform names (e.g. swap "-" for "_"). If a call fails with "tool not
  found", re-read the skill file and use a name from that table.
- Arguments must match the tool's input schema exactly.
- Set checkCache to false or omit it (reserved for future use).`
}

func (t *CallMCPTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"server": map[string]any{
				"type":        "string",
				"description": "The bare MCP server name as configured, for example context7",
			},
			"tool": map[string]any{
				"type":        "string",
				"description": "The exact MCP tool name to call on the server, for example resolve-library-id",
			},
			"arguments": map[string]any{
				"type":        "object",
				"description": "Key-value arguments matching the tool's input schema",
			},
			"checkCache": map[string]any{
				"type":        "boolean",
				"description": "Reserved for future caching; set to false or omit",
			},
		},
		"required":             []string{"server", "tool"},
		"additionalProperties": false,
	}
}

func (t *CallMCPTool) Execute(ctx context.Context, input any) (any, error) {
	params, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input: expected map[string]any, got %T", input)
	}

	server, err := requiredString(params, "server")
	if err != nil {
		return nil, err
	}
	tool, err := requiredString(params, "tool")
	if err != nil {
		return nil, err
	}
	server, tool, err = normalizeMCPCallTarget(server, tool)
	if err != nil {
		return nil, err
	}

	var arguments map[string]any
	if raw, exists := params["arguments"]; exists && raw != nil {
		if m, ok := raw.(map[string]any); ok {
			arguments = m
		}
	}

	_ = params["checkCache"] // reserved, no-op

	if err := t.validateRequiredArguments(ctx, server, tool, arguments); err != nil {
		return nil, err
	}

	argsJSON := ""
	if len(arguments) > 0 {
		data, jsonErr := json.MarshalIndent(arguments, "", "  ")
		if jsonErr == nil {
			argsJSON = string(data)
		}
	}

	if t.permissionRequester == nil {
		return nil, fmt.Errorf("permission denied: user approval required but not available")
	}
	allowed, err := t.permissionRequester.RequestPermission(ctx, t.Name(), server+"/"+tool, argsJSON, false)
	if err != nil {
		return nil, fmt.Errorf("permission request failed: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("permission denied by user: call_mcp_tool rejected for %s/%s", server, tool)
	}

	result, err := t.manager.CallTool(ctx, server, tool, arguments)
	if err != nil {
		if result != nil && len(result.Content) > 0 {
			content := formatMCPContent(result.Content)
			if content != "" {
				return nil, fmt.Errorf("%w\n%s", err, content)
			}
		}
		return nil, err
	}

	return map[string]any{
		"server":  server,
		"tool":    tool,
		"content": formatMCPContent(result.Content),
	}, nil
}

func requiredString(params map[string]any, name string) (string, error) {
	v, ok := params[name]
	if !ok {
		return "", fmt.Errorf("invalid input: missing required %q parameter", name)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("invalid input: %q must be a non-empty string", name)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("invalid input: %q must be a non-empty string", name)
	}
	return s, nil
}

func normalizeMCPCallTarget(server, tool string) (string, string, error) {
	const skillPrefix = "mcp:"

	server = strings.TrimSpace(server)
	tool = strings.TrimSpace(tool)
	server = strings.TrimPrefix(server, skillPrefix)
	if strings.Contains(server, "/") {
		return "", "", fmt.Errorf("invalid input: server must be a bare MCP server name, got %q", server)
	}

	if strings.HasPrefix(tool, skillPrefix+server+"/") {
		tool = strings.TrimPrefix(tool, skillPrefix+server+"/")
	} else if strings.HasPrefix(tool, server+"/") {
		tool = strings.TrimPrefix(tool, server+"/")
	}
	tool = strings.TrimSpace(tool)

	if server == "" {
		return "", "", fmt.Errorf("invalid input: %q must be a non-empty string", "server")
	}
	if tool == "" {
		return "", "", fmt.Errorf("invalid input: %q must be a non-empty string", "tool")
	}
	return server, tool, nil
}

func formatMCPContent(content []mcpsdk.Content) string {
	parts := make([]string, 0, len(content))
	for _, item := range content {
		switch c := item.(type) {
		case *mcpsdk.TextContent:
			if c.Text != "" {
				parts = append(parts, c.Text)
			}
		default:
			data, err := json.Marshal(item)
			if err == nil {
				parts = append(parts, string(data))
			}
		}
	}
	return strings.Join(parts, "\n")
}

func (t *CallMCPTool) validateRequiredArguments(ctx context.Context, server, tool string, arguments map[string]any) error {
	tools, err := t.manager.ListTools(ctx, server)
	if err != nil {
		return err
	}
	for _, candidate := range tools {
		if candidate.Name != tool {
			continue
		}
		missing := missingRequiredArguments(candidate.InputSchema, arguments)
		if len(missing) == 0 {
			return nil
		}
		return fmt.Errorf(`invalid input: arguments missing required fields for %s/%s: %s. 
			Read schema file: ~/.keen-agent/skills/mcp:%s/schemas/%s.json`, server, tool, strings.Join(missing, ", "), server, tool)
	}
	if len(tools) > 0 {
		return toolNotFoundError(server, tool)
	}
	return nil
}

func toolNotFoundError(server, requested string) error {
	return fmt.Errorf(`invalid input: tool %q not found on MCP server %q.
		Re-read ~/.keen-agent/skills/mcp:%s/SKILL.md and use an exact name from its Available tools table`, requested, server, server)
}

func missingRequiredArguments(schema any, arguments map[string]any) []string {
	required := requiredFields(schema)
	if len(required) == 0 {
		return nil
	}

	missing := make([]string, 0, len(required))
	for _, field := range required {
		if _, ok := arguments[field]; !ok {
			missing = append(missing, field)
		}
	}
	return missing
}

func requiredFields(schema any) []string {
	if schema == nil {
		return nil
	}
	data, err := json.Marshal(schema)
	if err != nil {
		return nil
	}
	var decoded struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil
	}
	return decoded.Required
}

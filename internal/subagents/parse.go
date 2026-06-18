package subagents

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type frontmatter struct {
	Name           string   `yaml:"name"`
	Description    string   `yaml:"description"`
	Tools          []string `yaml:"tools"`
	Provider       string   `yaml:"provider"`
	Model          string   `yaml:"model"`
	ThinkingEffort string   `yaml:"thinking_effort"`
	TimeoutSeconds int      `yaml:"timeout_seconds"`
	Hidden         bool     `yaml:"hidden"`
}

func ParseProfile(path string, data []byte) (Profile, []string, error) {
	content := string(data)
	if strings.TrimSpace(content) == "" {
		return Profile{}, nil, fmt.Errorf("empty profile")
	}

	fmText, body, hasFrontmatter, err := splitFrontmatter(content)
	if err != nil {
		return Profile{}, nil, err
	}
	if !hasFrontmatter {
		return Profile{}, nil, fmt.Errorf("missing YAML frontmatter")
	}

	var raw map[string]any
	if err := yaml.Unmarshal([]byte(fmText), &raw); err != nil {
		return Profile{}, nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	var fm frontmatter
	if err := yaml.Unmarshal([]byte(fmText), &fm); err != nil {
		return Profile{}, nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	name := strings.TrimSpace(fm.Name)
	if name == "" {
		return Profile{}, nil, fmt.Errorf("missing required frontmatter field: name")
	}
	description := strings.TrimSpace(fm.Description)
	if description == "" {
		return Profile{}, nil, fmt.Errorf("missing required frontmatter field: description")
	}

	return Profile{
		Name:           name,
		Description:    description,
		Tools:          trimStrings(fm.Tools),
		Provider:       strings.TrimSpace(fm.Provider),
		Model:          strings.TrimSpace(fm.Model),
		ThinkingEffort: strings.TrimSpace(fm.ThinkingEffort),
		TimeoutSeconds: fm.TimeoutSeconds,
		Hidden:         fm.Hidden,
		Instructions:   strings.TrimSpace(body),
	}, unknownFieldWarnings(name, raw), nil
}

func splitFrontmatter(content string) (string, string, bool, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(strings.TrimSuffix(lines[0], "\r")) != "---" {
		return "", "", false, nil
	}

	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(strings.TrimSuffix(lines[i], "\r")) == "---" {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n"), true, nil
		}
	}

	return "", "", false, fmt.Errorf("missing closing frontmatter delimiter")
}

func unknownFieldWarnings(name string, raw map[string]any) []string {
	known := map[string]bool{
		"name": true, "description": true, "tools": true, "provider": true,
		"model": true, "thinking_effort": true, "timeout_seconds": true,
		"hidden": true,
	}
	var warnings []string
	for field := range raw {
		if !known[field] {
			warnings = append(warnings, fmt.Sprintf("Subagent %s has unknown field %q", name, field))
		}
	}
	return warnings
}

func trimStrings(values []string) []string {
	var result []string
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

package skills

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Skill struct {
	Name        string
	Description string
	Location    string
}

type frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func ParseSkillMetadata(path string, data []byte) (Skill, error) {
	content := string(data)
	if strings.TrimSpace(content) == "" {
		return Skill{}, fmt.Errorf("empty SKILL.md")
	}

	fmText, _, hasFrontmatter, err := splitFrontmatter(content)
	if err != nil {
		return Skill{}, err
	}
	if !hasFrontmatter {
		return Skill{}, fmt.Errorf("missing YAML frontmatter")
	}

	var fm frontmatter
	if err := yaml.Unmarshal([]byte(fmText), &fm); err != nil {
		return Skill{}, fmt.Errorf("parse frontmatter: %w", err)
	}

	name := strings.TrimSpace(fm.Name)
	if name == "" {
		return Skill{}, fmt.Errorf("missing required frontmatter field: name")
	}

	description := strings.TrimSpace(fm.Description)
	if description == "" {
		return Skill{}, fmt.Errorf("missing required frontmatter field: description")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return Skill{}, err
	}

	return Skill{
		Name:        name,
		Description: description,
		Location:    absPath,
	}, nil
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

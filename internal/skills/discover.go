package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Discovery struct {
	Skills   []Skill
	Warnings []string
}

func Discover(workingDir, bundledDir string) Discovery {
	var result Discovery
	seen := map[string]bool{}

	for _, root := range discoveryRoots(workingDir, bundledDir) {
		matches, err := filepath.Glob(filepath.Join(root, "*", "SKILL.md"))
		if err != nil {
			continue
		}
		sort.Strings(matches)
		for _, skillPath := range matches {
			dirName := filepath.Base(filepath.Dir(skillPath))
			if seen[dirName] {
				continue
			}
			seen[dirName] = true

			absPath, err := filepath.Abs(skillPath)
			if err != nil {
				continue
			}
			result.Skills = append(result.Skills, Skill{
				Name:        dirName,
				Description: dirName,
				Location:    absPath,
			})
		}
	}

	return result
}

func LoadMetadata(discovery Discovery) Discovery {
	result := Discovery{Warnings: append([]string(nil), discovery.Warnings...)}
	result.Skills = make([]Skill, 0, len(discovery.Skills))
	byName := map[string]string{}

	for _, discovered := range discovery.Skills {
		data, err := os.ReadFile(discovered.Location)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Skill %s failed to load: %v", discovered.Name, err))
			continue
		}
		skill, err := ParseSkillMetadata(discovered.Location, data)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Skill %s failed to load: %v", discovered.Name, err))
			continue
		}
		if existing, dup := byName[skill.Name]; dup {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Skill %s skipped: name %q already used by %s", discovered.Name, skill.Name, existing))
			continue
		}
		byName[skill.Name] = discovered.Name
		result.Skills = append(result.Skills, skill)
	}

	sort.Slice(result.Skills, func(i, j int) bool {
		return result.Skills[i].Name < result.Skills[j].Name
	})
	return result
}

func Catalog(all []Skill, cfg Config) string {
	enabled := make([]Skill, 0, len(all))
	for _, skill := range all {
		if cfg.Enabled(skill.Name) {
			enabled = append(enabled, skill)
		}
	}
	if len(enabled) == 0 {
		return ""
	}

	sort.Slice(enabled, func(i, j int) bool {
		return enabled[i].Name < enabled[j].Name
	})

	var sb strings.Builder
	sb.WriteString("## Available Skills\n\n")
	sb.WriteString(`You have access to specialized skills. To activate a skill, use the read_file tool
to read the skill's SKILL.md file at one of these paths, then follow the instructions
within. Resolve relative paths in skill instructions against the skill directory
containing SKILL.md:
`)
	for _, skill := range enabled {
		sb.WriteString("- ")
		sb.WriteString(skill.Name)
		sb.WriteString(": ")
		sb.WriteString(skill.Description)
		sb.WriteString(" → read ")
		sb.WriteString(skill.Location)
		sb.WriteString("\n")
	}
	sb.WriteString("IMPORTANT: If any user message in this conversation begins with " +
		"`[Activate skill: <name>]`, the SKILL.md body for that skill has already been " +
		"provided inline in that message — do not call read_file on its path.")
	return strings.TrimRight(sb.String(), "\n")
}

func discoveryRoots(workingDir, bundledDir string) []string {
	roots := []string{
		filepath.Join(workingDir, ".agents", "skills"),
		filepath.Join(workingDir, ".keen-agent", "skills"),
		filepath.Join(workingDir, ".claude", "skills"),
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		roots = append(roots,
			filepath.Join(home, ".agents", "skills"),
			filepath.Join(home, ".keen-agent", "skills"),
			filepath.Join(home, ".claude", "skills"),
		)
	}
	if strings.TrimSpace(bundledDir) != "" {
		roots = append(roots, bundledDir)
	}
	return roots
}

func Find(skills []Skill, name string) (Skill, bool) {
	for _, skill := range skills {
		if skill.Name == name {
			return skill, true
		}
	}
	return Skill{}, false
}

func ActivationMessage(skill Skill, args []string) (string, error) {
	content, err := os.ReadFile(skill.Location)
	if err != nil {
		return "", fmt.Errorf("read skill %s: %w", skill.Name, err)
	}

	body := substituteArgs(strings.TrimSpace(string(content)), args)

	var sb strings.Builder
	sb.WriteString("[Activate skill: ")
	sb.WriteString(skill.Name)
	sb.WriteString("]\n\n")
	sb.WriteString(body)
	return sb.String(), nil
}

var argPlaceholder = regexp.MustCompile(`\$ARGUMENTS\b|\$([1-9])\b`)

func substituteArgs(body string, args []string) string {
	return argPlaceholder.ReplaceAllStringFunc(body, func(match string) string {
		if match == "$ARGUMENTS" {
			return strings.Join(args, " ")
		}
		idx := int(match[1]-'0') - 1
		if idx < len(args) {
			return args[idx]
		}
		return ""
	})
}

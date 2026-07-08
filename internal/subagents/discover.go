package subagents

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Discover(configDirs []string, workingDir, bundledDir string) Discovery {
	var result Discovery
	seen := map[string]bool{}

	for _, root := range discoveryRoots(configDirs, workingDir, bundledDir) {
		matches, err := filepath.Glob(filepath.Join(root, "*.md"))
		if err != nil {
			continue
		}
		sort.Strings(matches)
		for _, profilePath := range matches {
			key := strings.TrimSuffix(filepath.Base(profilePath), filepath.Ext(profilePath))
			if seen[key] {
				continue
			}
			seen[key] = true

			absPath, err := filepath.Abs(profilePath)
			if err != nil {
				continue
			}
			result.Profiles = append(result.Profiles, Profile{
				Name:        key,
				Description: key,
			})
			result.locations = append(result.locations, absPath)
		}
	}

	return result
}

func LoadMetadata(discovery Discovery) Discovery {
	result := Discovery{Warnings: append([]string(nil), discovery.Warnings...)}
	result.Profiles = make([]Profile, 0, len(discovery.Profiles))
	byName := map[string]string{}

	for i, discovered := range discovery.Profiles {
		location := discovery.locations[i]
		data, err := os.ReadFile(location)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Subagent %s failed to load: %v", discovered.Name, err))
			continue
		}
		profile, warnings, err := ParseProfile(location, data)
		result.Warnings = append(result.Warnings, warnings...)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Subagent %s failed to load: %v", discovered.Name, err))
			continue
		}
		if existing, dup := byName[profile.Name]; dup {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Subagent %s skipped: name %q already used by %s", discovered.Name, profile.Name, existing))
			continue
		}
		byName[profile.Name] = discovered.Name
		result.Profiles = append(result.Profiles, profile)
	}

	sort.Slice(result.Profiles, func(i, j int) bool {
		return result.Profiles[i].Name < result.Profiles[j].Name
	})
	return result
}

func Catalog(profiles []Profile) string {
	visible := make([]Profile, 0, len(profiles))
	for _, profile := range profiles {
		if !profile.Hidden {
			visible = append(visible, profile)
		}
	}
	if len(visible) == 0 {
		return ""
	}

	sort.Slice(visible, func(i, j int) bool {
		return visible[i].Name < visible[j].Name
	})

	var sb strings.Builder
	sb.WriteString("## Available Subagents\n\n")
	sb.WriteString("You can delegate bounded, independent read-only work with the `delegate_task` tool.\n")
	sb.WriteString("Use a subagent only when the work can be handed off as a self-contained task, has a clear objective, and matches a listed subagent's description.\n")
	sb.WriteString("Do not use subagents for quick lookups, direct edits, commands, ambiguous requests that need clarification, or tightly coupled implementation work.\n")
	sb.WriteString("When delegating, choose the matching subagent and pass a targeted task with relevant paths or inputs, what to look for, and the expected concise findings. Synthesize returned findings yourself.\n\n")
	for _, profile := range visible {
		sb.WriteString("- ")
		sb.WriteString(profile.Name)
		sb.WriteString(": ")
		sb.WriteString(profile.Description)
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func discoveryRoots(configDirs []string, workingDir, bundledDir string) []string {
	var roots []string
	roots = append(roots, configDirs...)
	return roots
}

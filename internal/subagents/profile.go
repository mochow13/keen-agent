package subagents

import "strings"

type Profile struct {
	Name           string
	Description    string
	Tools          []string
	Provider       string
	Model          string
	ThinkingEffort string
	TimeoutSeconds int
	Hidden         bool
	Instructions   string
}

type Discovery struct {
	Profiles  []Profile
	Warnings  []string
	locations []string
}

func Find(profiles []Profile, name string) (Profile, bool) {
	for _, profile := range profiles {
		if profile.Name == name {
			return profile, true
		}
	}
	return Profile{}, false
}

func readOnlyTools(profile Profile) []string {
	allowed := []string{"read_file", "glob", "grep"}
	if len(profile.Tools) == 0 {
		return allowed
	}
	requested := map[string]bool{}
	for _, tool := range profile.Tools {
		requested[strings.TrimSpace(tool)] = true
	}
	var result []string
	for _, tool := range allowed {
		if requested[tool] {
			result = append(result, tool)
		}
	}
	return result
}

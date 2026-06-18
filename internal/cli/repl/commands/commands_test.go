package commands

import "testing"

func TestFilterIncludesSkillsReloadSuggestion(t *testing.T) {
	results := Filter("/skills r")

	for _, result := range results {
		if result.Name == SkillsReload {
			return
		}
	}

	t.Fatalf("expected %q suggestion, got %#v", SkillsReload, results)
}

func TestFilterIncludesSkillsStatusSuggestion(t *testing.T) {
	results := Filter("/skills s")

	for _, result := range results {
		if result.Name == SkillsStatus {
			return
		}
	}

	t.Fatalf("expected %q suggestion, got %#v", SkillsStatus, results)
}

package commands

import "testing"

func TestFilterIncludesSkillsReloadSuggestion(t *testing.T) {
	results := Filter("/skills r")
	if len(results) != 1 || results[0].Name != SkillsReload {
		t.Fatalf("Filter() = %#v, want only %q", results, SkillsReload)
	}
}

func TestFilterIncludesSkillsStatusSuggestion(t *testing.T) {
	results := Filter("/skills s")
	if len(results) != 1 || results[0].Name != SkillsStatus {
		t.Fatalf("Filter() = %#v, want only %q", results, SkillsStatus)
	}
}

func TestAvailableSuggestionsHidesDisabledHelpers(t *testing.T) {
	results := FilterAvailable("/", false, false)
	for _, result := range results {
		if result.Name == Btw || result.Name == Adversary || result.Name == AdversaryModel {
			t.Fatalf("disabled helper command was suggested: %q", result.Name)
		}
	}
}

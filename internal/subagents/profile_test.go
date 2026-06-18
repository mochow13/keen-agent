package subagents

import "testing"

func TestFind(t *testing.T) {
	profile, ok := Find([]Profile{{Name: "explorer"}}, "explorer")
	if !ok {
		t.Fatal("expected profile to be found")
	}
	if profile.Name != "explorer" {
		t.Fatalf("unexpected profile: %+v", profile)
	}

	if _, ok := Find([]Profile{{Name: "explorer"}}, "missing"); ok {
		t.Fatal("expected missing profile not to be found")
	}
}

func TestReadOnlyToolsDefaultsAndFilters(t *testing.T) {
	if got := readOnlyTools(Profile{}); !sameStrings(got, []string{"read_file", "glob", "grep"}) {
		t.Fatalf("expected default read-only tools, got %v", got)
	}

	got := readOnlyTools(Profile{Tools: []string{"grep", "write_file", "", "read_file"}})
	if !sameStrings(got, []string{"read_file", "grep"}) {
		t.Fatalf("expected read-only intersection, got %v", got)
	}
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, root, name, content string) string {
	t.Helper()
	return writeSkillAt(t, filepath.Join(root, ".agents", "skills"), name, content)
}

func writeSkillAt(t *testing.T, skillsDir, name, content string) string {
	t.Helper()
	dir := filepath.Join(skillsDir, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	return path
}

func TestDiscover_OrderAndDedup(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	writeSkill(t, dir1, "skill1", "---\nname: skill1\ndescription: From dir1\n---\nBody")
	writeSkill(t, dir2, "skill1", "---\nname: skill1\ndescription: From dir2\n---\nBody")
	writeSkill(t, dir2, "skill2", "---\nname: skill2\ndescription: From dir2\n---\nBody")

	// dir1 comes first, so its skill1 should win over dir2's skill1
	result := LoadMetadata(Discover([]string{filepath.Join(dir1, ".agents", "skills"), filepath.Join(dir2, ".agents", "skills")}))
	if len(result.Skills) != 2 {
		t.Fatalf("expected 2 skills after dedup, got %d", len(result.Skills))
	}

	if result.Skills[0].Name == "skill1" {
		if result.Skills[0].Description != "From dir1" {
			t.Fatalf("expected dir1 to win, got %#v", result.Skills[0])
		}
	} else {
		if result.Skills[1].Description != "From dir1" {
			t.Fatalf("expected dir1 to win, got %#v", result.Skills[1])
		}
	}
}

func TestLoadMetadata_InvalidYAMLWarnsAndSkips(t *testing.T) {
	work := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	writeSkill(t, work, "bad", "---\nname: [\n---\nBody")

	result := LoadMetadata(Discover([]string{filepath.Join(work, ".agents", "skills")}))
	if len(result.Skills) != 0 {
		t.Fatalf("expected no skills, got %#v", result.Skills)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "Skill bad failed to load") {
		t.Fatalf("unexpected warnings: %#v", result.Warnings)
	}
}

func TestLoadMetadata_ReadsNameAndDescription(t *testing.T) {
	work := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	writeSkill(t, work, "demo", "---\nname: demo\ndescription: Demo skill\n---\nBody")

	result := LoadMetadata(Discover([]string{filepath.Join(work, ".agents", "skills")}))
	if len(result.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(result.Skills))
	}
	if result.Skills[0].Description != "Demo skill" {
		t.Fatalf("expected metadata description, got %#v", result.Skills[0])
	}
}

func TestLoadMetadata_FrontmatterNameOverridesDirName(t *testing.T) {
	work := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	writeSkill(t, work, "any-dir", "---\nname: real-name\ndescription: Demo skill\n---\nBody")

	result := LoadMetadata(Discover([]string{filepath.Join(work, ".agents", "skills")}))
	if len(result.Skills) != 1 || result.Skills[0].Name != "real-name" {
		t.Fatalf("expected frontmatter name to win, got %#v", result.Skills)
	}
}

func TestLoadMetadata_DuplicateFrontmatterNameWarns(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()
	t.Setenv("HOME", home)
	writeSkill(t, work, "project-dir", "---\nname: shared\ndescription: From project\n---\nBody")
	writeSkill(t, home, "global-dir", "---\nname: shared\ndescription: From global\n---\nBody")

	result := LoadMetadata(Discover([]string{filepath.Join(work, ".agents", "skills"), filepath.Join(home, ".agents", "skills")}))
	if len(result.Skills) != 1 {
		t.Fatalf("expected 1 skill after dedup, got %#v", result.Skills)
	}
	if result.Skills[0].Description != "From project" {
		t.Fatalf("expected project to win fm.Name collision, got %#v", result.Skills[0])
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "name \"shared\" already used") {
		t.Fatalf("expected collision warning, got %#v", result.Warnings)
	}
}

func TestCatalog_Empty(t *testing.T) {
	if got := Catalog(nil, Config{}); got != "" {
		t.Fatalf("expected empty catalog, got %q", got)
	}
}

func TestCatalog_IncludesEnabledSkills(t *testing.T) {
	got := Catalog([]Skill{{Name: "demo", Description: "Demo skill", Location: "/tmp/demo/SKILL.md"}}, Config{})
	if !strings.Contains(got, "## Available Skills") {
		t.Fatalf("expected skills heading, got %q", got)
	}
	if !strings.Contains(got, "- demo: Demo skill → read /tmp/demo/SKILL.md") {
		t.Fatalf("expected skill entry, got %q", got)
	}
	if !strings.Contains(got, "[Activate skill: <name>]") {
		t.Fatalf("expected catalog to reference activation marker, got %q", got)
	}
}

func TestCatalog_ExcludesDisabledSkills(t *testing.T) {
	cfg := Config{IsEnabled: map[string]bool{"demo": false}}
	got := Catalog([]Skill{{Name: "demo", Description: "Demo skill", Location: "/tmp/demo/SKILL.md"}}, cfg)
	if got != "" {
		t.Fatalf("expected empty catalog for disabled skill, got %q", got)
	}
}

func TestFind(t *testing.T) {
	skill, ok := Find([]Skill{{Name: "one"}}, "one")
	if !ok || skill.Name != "one" {
		t.Fatalf("expected to find skill")
	}
	if _, ok := Find([]Skill{{Name: "one"}}, "two"); ok {
		t.Fatalf("expected missing skill")
	}
}

func writeSkillFile(t *testing.T, content string) Skill {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}
	return Skill{Name: "demo", Location: path}
}

func TestActivationMessage(t *testing.T) {
	skill := writeSkillFile(t, "# Demo\nargs=$ARGUMENTS")
	got, err := ActivationMessage(skill, []string{"foo", "bar"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "[Activate skill: demo]") {
		t.Fatalf("expected activation header, got %q", got)
	}
	if !strings.Contains(got, "args=foo bar") {
		t.Fatalf("expected $ARGUMENTS substitution, got %q", got)
	}
	if !strings.Contains(got, "# Demo") {
		t.Fatalf("expected skill content, got %q", got)
	}
}

func TestActivationMessageNoArgs(t *testing.T) {
	skill := writeSkillFile(t, "# Demo\nargs=$ARGUMENTS done")
	got, err := ActivationMessage(skill, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "args= done") {
		t.Fatalf("expected $ARGUMENTS to substitute to empty, got %q", got)
	}
}

func TestActivationMessagePositional(t *testing.T) {
	skill := writeSkillFile(t, "first=$1 second=$2 third=$3")
	got, err := ActivationMessage(skill, []string{"a", "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "first=a second=b third=") {
		t.Fatalf("expected positional substitution with empty fallback, got %q", got)
	}
}

func TestActivationMessagePreservesDollarTen(t *testing.T) {
	skill := writeSkillFile(t, "literal=$10")
	got, err := ActivationMessage(skill, []string{"a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "literal=$10") {
		t.Fatalf("expected $10 untouched, got %q", got)
	}
}

func TestActivationMessageMissingFile(t *testing.T) {
	skill := Skill{Name: "demo", Location: "/nonexistent/SKILL.md"}
	_, err := ActivationMessage(skill, nil)
	if err == nil {
		t.Fatal("expected error for missing skill file")
	}
}

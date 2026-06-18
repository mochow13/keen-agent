package skills

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSkillMetadata_WithFrontmatter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo", "SKILL.md")
	skill, err := ParseSkillMetadata(path, []byte("---\nname: demo\ndescription: Demo skill\n---\nBody"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill.Name != "demo" || skill.Description != "Demo skill" {
		t.Fatalf("unexpected skill: %#v", skill)
	}
	if !filepath.IsAbs(skill.Location) {
		t.Fatalf("expected absolute location, got %q", skill.Location)
	}
}

func TestParseSkillMetadata_NameDifferentFromDirIsAllowed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "any-dir", "SKILL.md")
	skill, err := ParseSkillMetadata(path, []byte("---\nname: real-name\ndescription: Demo skill\n---\nBody"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill.Name != "real-name" {
		t.Fatalf("expected frontmatter name to win, got %q", skill.Name)
	}
}

func TestParseSkillMetadata_NoFrontmatterErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo", "SKILL.md")
	_, err := ParseSkillMetadata(path, []byte("plain markdown"))
	if err == nil || !strings.Contains(err.Error(), "frontmatter") {
		t.Fatalf("expected frontmatter error, got %v", err)
	}
}

func TestParseSkillMetadata_EmptyErrors(t *testing.T) {
	_, err := ParseSkillMetadata(filepath.Join(t.TempDir(), "SKILL.md"), []byte(" \n\t"))
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty error, got %v", err)
	}
}

func TestParseSkillMetadata_InvalidYAML(t *testing.T) {
	_, err := ParseSkillMetadata(filepath.Join(t.TempDir(), "SKILL.md"), []byte("---\nname: [\n---\nBody"))
	if err == nil {
		t.Fatal("expected YAML parse error")
	}
}

func TestParseSkillMetadata_MissingNameErrors(t *testing.T) {
	_, err := ParseSkillMetadata(filepath.Join(t.TempDir(), "SKILL.md"), []byte("---\ndescription: Demo skill\n---\nBody"))
	if err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("expected missing name error, got %v", err)
	}
}

func TestParseSkillMetadata_MissingDescriptionErrors(t *testing.T) {
	_, err := ParseSkillMetadata(filepath.Join(t.TempDir(), "SKILL.md"), []byte("---\nname: demo\n---\nBody"))
	if err == nil || !strings.Contains(err.Error(), "description") {
		t.Fatalf("expected missing description error, got %v", err)
	}
}

func TestParseSkillMetadata_BlankFieldsError(t *testing.T) {
	_, err := ParseSkillMetadata(filepath.Join(t.TempDir(), "SKILL.md"), []byte("---\nname: demo\ndescription: \"   \"\n---\nBody"))
	if err == nil || !strings.Contains(err.Error(), "description") {
		t.Fatalf("expected blank description error, got %v", err)
	}
}

package llm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mochow13/keen-agent/internal/skills"
)

func TestBuild_ContainsIdentity(t *testing.T) {
	dir := t.TempDir()
	result := Build(dir, "", "", ModeBuild)
	if !strings.Contains(result, "Keen Agent") {
		t.Error("expected output to contain 'Keen Agent'")
	}
}

func TestBuild_ContainsWorkingDir(t *testing.T) {
	dir := t.TempDir()
	result := Build(dir, "", "", ModeBuild)
	if !strings.Contains(result, dir) {
		t.Errorf("expected output to contain working dir %q", dir)
	}
}

func TestBuild_AgentsMd_Found(t *testing.T) {
	dir := t.TempDir()
	content := "## My Project\nSome instructions here."
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(content), 0644)

	result := Build(dir, "", "", ModeBuild)
	if !strings.Contains(result, "# Project Instructions") {
		t.Error("expected project instructions section")
	}
	if !strings.Contains(result, "My Project") {
		t.Error("expected AGENTS.md content in output")
	}
}

func TestBuild_AgentsMd_WalkUp(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "subdir")
	os.MkdirAll(child, 0755)
	os.WriteFile(filepath.Join(parent, "AGENTS.md"), []byte("parent instructions"), 0644)

	result := Build(child, "", "", ModeBuild)
	if !strings.Contains(result, "parent instructions") {
		t.Error("expected AGENTS.md from parent directory")
	}
}

func TestBuild_ClaudeMd_Fallback(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("claude instructions"), 0644)

	result := Build(dir, "", "", ModeBuild)
	if !strings.Contains(result, "claude instructions") {
		t.Error("expected CLAUDE.md content as fallback")
	}
}

func TestBuild_NoInstructionFile(t *testing.T) {
	dir := t.TempDir()
	result := Build(dir, "", "", ModeBuild)
	if strings.Contains(result, "# Project Instructions") {
		t.Error("expected no project instructions section when no file exists")
	}
}

func TestBuild_AgentsMd_Truncation(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("x", 10*1024)
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(content), 0644)

	result := Build(dir, "", "", ModeBuild)
	if !strings.Contains(result, "[truncated") {
		t.Error("expected truncation note for large AGENTS.md")
	}
}

func TestBuild_AgentsMd_Empty(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(""), 0644)

	result := Build(dir, "", "", ModeBuild)
	if strings.Contains(result, "# Project Instructions") {
		t.Error("expected no project instructions for empty AGENTS.md")
	}
}

func TestBuild_IncludesSkillsCatalog(t *testing.T) {
	dir := t.TempDir()
	catalog := skills.Catalog([]skills.Skill{{Name: "demo", Description: "Demo skill", Location: "/tmp/demo/SKILL.md"}}, skills.Config{})

	result := Build(dir, catalog, "", ModeBuild)
	if !strings.Contains(result, "## Available Skills") {
		t.Fatal("expected skills catalog")
	}
	if !strings.Contains(result, "- demo: Demo skill") {
		t.Fatalf("expected demo skill in catalog, got %q", result)
	}
}

func TestBuild_PlanIncludesPlanInstructions(t *testing.T) {
	result := Build(t.TempDir(), "", "", ModePlan)
	for _, expected := range []string{"# Active mode: plan", "write_file and edit_file are not available", "/mode build or Shift+Tab"} {
		if !strings.Contains(result, expected) {
			t.Fatalf("expected %q in plan prompt, got %q", expected, result)
		}
	}
}

func TestBuild_BuildIncludesBuildInstructions(t *testing.T) {
	result := Build(t.TempDir(), "", "", ModeBuild)
	if !strings.Contains(result, "# Active mode: build") {
		t.Fatalf("expected build mode prompt, got %q", result)
	}
	if strings.Contains(result, "write_file and edit_file are not available") {
		t.Fatalf("did not expect plan restrictions in build prompt, got %q", result)
	}
}

func TestBuild_ModeInstructionsAreAtEnd(t *testing.T) {
	dir := t.TempDir()
	catalog := skills.Catalog([]skills.Skill{{Name: "demo", Description: "Demo skill", Location: "/tmp/demo/SKILL.md"}}, skills.Config{})

	result := Build(dir, catalog, "", ModePlan)
	modeIndex := strings.Index(result, "# Active mode: plan")
	if modeIndex == -1 {
		t.Fatal("expected active mode section")
	}
	if strings.Contains(result[modeIndex:], "Working directory:") {
		t.Fatal("expected working directory before mode section")
	}
	if strings.Contains(result[modeIndex:], "## Available Skills") {
		t.Fatal("expected skills catalog before mode section")
	}
}

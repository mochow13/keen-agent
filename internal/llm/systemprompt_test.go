package llm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mochow13/keen-agent/internal/agentconfig"
	"github.com/mochow13/keen-agent/internal/skills"
)

func buildDefault(dir string, mode AgentMode) string {
	return Build(dir, "", "", mode, nil)
}

func TestBuild_ContainsIdentity(t *testing.T) {
	dir := t.TempDir()
	result := buildDefault(dir, ModeBuild)
	if !strings.Contains(result, "Keen Agent") {
		t.Error("expected output to contain 'Keen Agent'")
	}
}

func TestBuild_ContainsWorkingDir(t *testing.T) {
	dir := t.TempDir()
	result := buildDefault(dir, ModeBuild)
	if !strings.Contains(result, dir) {
		t.Errorf("expected output to contain working dir %q", dir)
	}
}

func TestBuild_AgentsMd_Found(t *testing.T) {
	dir := t.TempDir()
	content := "## My Project\nSome instructions here."
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(content), 0644)

	result := buildDefault(dir, ModeBuild)
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

	result := buildDefault(child, ModeBuild)
	if !strings.Contains(result, "parent instructions") {
		t.Error("expected AGENTS.md from parent directory")
	}
}

func TestBuild_ClaudeMd_Fallback(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("claude instructions"), 0644)

	result := buildDefault(dir, ModeBuild)
	if !strings.Contains(result, "claude instructions") {
		t.Error("expected CLAUDE.md content as fallback")
	}
}

func TestBuild_NoInstructionFile(t *testing.T) {
	dir := t.TempDir()
	result := buildDefault(dir, ModeBuild)
	if strings.Contains(result, "# Project Instructions") {
		t.Error("expected no project instructions section when no file exists")
	}
}

func TestBuild_AgentsMd_Truncation(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("x", 10*1024)
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(content), 0644)

	result := buildDefault(dir, ModeBuild)
	if !strings.Contains(result, "[truncated") {
		t.Error("expected truncation note for large AGENTS.md")
	}
}

func TestBuild_AgentsMd_Empty(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(""), 0644)

	result := buildDefault(dir, ModeBuild)
	if strings.Contains(result, "# Project Instructions") {
		t.Error("expected no project instructions for empty AGENTS.md")
	}
}

func TestBuild_IncludesSkillsCatalog(t *testing.T) {
	dir := t.TempDir()
	catalog := skills.Catalog([]skills.Skill{{Name: "demo", Description: "Demo skill", Location: "/tmp/demo/SKILL.md"}}, skills.Config{})

	result := Build(dir, catalog, "", ModeBuild, nil)
	if !strings.Contains(result, "## Available Skills") {
		t.Fatal("expected skills catalog")
	}
	if !strings.Contains(result, "- demo: Demo skill") {
		t.Fatalf("expected demo skill in catalog, got %q", result)
	}
}

func TestBuild_PlanIncludesPlanInstructions(t *testing.T) {
	result := buildDefault(t.TempDir(), ModePlan)
	for _, expected := range []string{"# Active mode: plan", "write_file and edit_file are not available", "/mode build or Shift+Tab"} {
		if !strings.Contains(result, expected) {
			t.Fatalf("expected %q in plan prompt, got %q", expected, result)
		}
	}
}

func TestBuild_BuildIncludesBuildInstructions(t *testing.T) {
	result := buildDefault(t.TempDir(), ModeBuild)
	if !strings.Contains(result, "# Active mode: build") {
		t.Fatalf("expected build mode prompt, got %q", result)
	}
	if strings.Contains(result, "write_file and edit_file are not available") {
		t.Fatalf("did not expect plan restrictions in build prompt, got %q", result)
	}
}

func TestBuild_IncludesToolFollowThroughInstructions(t *testing.T) {
	result := buildDefault(t.TempDir(), ModeBuild)
	for _, expected := range []string{
		"Tool use is an action, not narration",
		"your next step should be the corresponding tool call",
		"Never claim that you read a file",
	} {
		if !strings.Contains(result, expected) {
			t.Fatalf("expected %q in prompt, got %q", expected, result)
		}
	}
}

func TestBuild_ModeInstructionsAreAtEnd(t *testing.T) {
	dir := t.TempDir()
	catalog := skills.Catalog([]skills.Skill{{Name: "demo", Description: "Demo skill", Location: "/tmp/demo/SKILL.md"}}, skills.Config{})

	result := Build(dir, catalog, "", ModePlan, nil)
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

func TestBuild_ConfigPersona(t *testing.T) {
	dir := t.TempDir()
	cfg := &agentconfig.Config{
		SystemPrompt: "You are a PostgreSQL DBA agent.",
	}
	result := Build(dir, "", "", ModeBuild, cfg)
	if !strings.Contains(result, "PostgreSQL DBA agent") {
		t.Fatal("expected config persona in output")
	}
	if strings.Contains(result, "Keen Agent") {
		t.Fatal("expected default persona to be replaced by config persona")
	}
}

func TestBuild_ConfigPersonaWithFiles(t *testing.T) {
	dir := t.TempDir()
	promptFile := filepath.Join(dir, "extra.md")
	os.WriteFile(promptFile, []byte("Additional context about databases."), 0644)

	cfg := &agentconfig.Config{
		SystemPrompt:      "You are a DBA.",
		SystemPromptFiles: agentconfig.StringOrArray{promptFile},
	}

	result := Build(dir, "", "", ModeBuild, cfg)
	if !strings.Contains(result, "You are a DBA.") {
		t.Fatal("expected inline system_prompt")
	}
	if !strings.Contains(result, "Additional context about databases.") {
		t.Fatal("expected system_prompt_files content")
	}
}

func TestBuild_ConfigFallbackToDefault(t *testing.T) {
	dir := t.TempDir()
	cfg := &agentconfig.Config{}
	result := Build(dir, "", "", ModeBuild, cfg)
	if !strings.Contains(result, "Keen Agent") {
		t.Fatal("expected default persona when config has no system_prompt")
	}
}

func TestBuild_ModeOverlay(t *testing.T) {
	dir := t.TempDir()
	cfg := &agentconfig.Config{
		SystemPrompt: "Base persona.",
		Modes: map[string]agentconfig.ModeConfig{
			agentconfig.ModeBuild: {
				SystemPrompt: "Custom build overlay instructions.",
			},
		},
	}

	result := Build(dir, "", "", ModeBuild, cfg)
	if !strings.Contains(result, "Custom build overlay instructions.") {
		t.Fatal("expected mode overlay in output")
	}
	modeIdx := strings.Index(result, "# Active mode: build")
	overlayIdx := strings.Index(result, "Custom build overlay instructions.")
	if overlayIdx <= modeIdx {
		t.Fatal("expected mode overlay after built-in mode constraints")
	}
}

func TestBuild_ModeOverlayPlanNotInBuild(t *testing.T) {
	dir := t.TempDir()
	cfg := &agentconfig.Config{
		SystemPrompt: "Base persona.",
		Modes: map[string]agentconfig.ModeConfig{
			agentconfig.ModePlan: {
				SystemPrompt: "Plan-only overlay.",
			},
		},
	}

	result := Build(dir, "", "", ModeBuild, cfg)
	if strings.Contains(result, "Plan-only overlay.") {
		t.Fatal("plan overlay should not appear in build mode")
	}
}

func TestBuild_ModeOverlayWithFiles(t *testing.T) {
	dir := t.TempDir()
	overlayFile := filepath.Join(dir, "build-overlay.md")
	os.WriteFile(overlayFile, []byte("File-based build overlay."), 0644)

	cfg := &agentconfig.Config{
		SystemPrompt: "Base.",
		Modes: map[string]agentconfig.ModeConfig{
			agentconfig.ModeBuild: {
				SystemPrompt:      "Inline overlay.",
				SystemPromptFiles: agentconfig.StringOrArray{overlayFile},
			},
		},
	}
	result := Build(dir, "", "", ModeBuild, cfg)
	if !strings.Contains(result, "Inline overlay.") {
		t.Fatal("expected inline mode overlay")
	}
	if !strings.Contains(result, "File-based build overlay.") {
		t.Fatal("expected file-based mode overlay")
	}
	inlineIdx := strings.Index(result, "Inline overlay.")
	fileIdx := strings.Index(result, "File-based build overlay.")
	if fileIdx < inlineIdx {
		t.Fatal("expected file overlay after inline overlay")
	}
}

func TestBuild_ConfigProjectInstructions(t *testing.T) {
	dir := t.TempDir()
	instrFile := filepath.Join(dir, "custom-instructions.md")
	os.WriteFile(instrFile, []byte("Custom project rules."), 0644)

	cfg := &agentconfig.Config{
		SystemPrompt:        "Persona.",
		ProjectInstructions: instrFile,
	}
	result := Build(dir, "", "", ModeBuild, cfg)
	if !strings.Contains(result, "Custom project rules.") {
		t.Fatal("expected custom project instructions from config")
	}
	if !strings.Contains(result, "# Project Instructions") {
		t.Fatal("expected project instructions header")
	}
}

func TestBuildBtwPrompt_Default(t *testing.T) {
	result := BuildBtwPrompt("/tmp", nil)
	if !strings.Contains(result, "helper agent") {
		t.Fatal("expected default btw prompt")
	}
	if !strings.Contains(result, "/tmp") {
		t.Fatal("expected working dir")
	}
}

func TestBuildBtwPrompt_ConfigOverride(t *testing.T) {
	cfg := &agentconfig.Config{
		Btw: &agentconfig.BtwConfig{
			Enabled:      true,
			SystemPrompt: "Custom btw instructions.",
		},
	}
	result := BuildBtwPrompt("/tmp", cfg)
	if !strings.Contains(result, "Custom btw instructions.") {
		t.Fatal("expected custom btw prompt")
	}
	if strings.Contains(result, "helper agent") {
		t.Fatal("expected default btw prompt to be replaced")
	}
}

func TestBuildAdversaryPrompt_Default(t *testing.T) {
	result := BuildAdversaryPrompt("/tmp", nil)
	if !strings.Contains(result, "adversarial critic") {
		t.Fatal("expected default adversary prompt")
	}
}

func TestBuildAdversaryPrompt_ConfigOverride(t *testing.T) {
	cfg := &agentconfig.Config{
		Adversary: &agentconfig.AdversaryConfig{
			Enabled:      true,
			SystemPrompt: "Custom adversary instructions.",
		},
	}
	result := BuildAdversaryPrompt("/tmp", cfg)
	if !strings.Contains(result, "Custom adversary instructions.") {
		t.Fatal("expected custom adversary prompt")
	}
	if strings.Contains(result, "adversarial critic") {
		t.Fatal("expected default adversary prompt to be replaced")
	}
}

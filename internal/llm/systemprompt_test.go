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

func TestBuild_DefaultPersonaIsGeneralPurpose(t *testing.T) {
	result := buildDefault(t.TempDir(), ModeBuild)
	for _, expected := range []string{
		"general-purpose AI agent",
		"role, domain, and priorities are defined by the user's request",
		"Build mode allows you to take action",
	} {
		if !strings.Contains(result, expected) {
			t.Errorf("expected default prompt to contain %q", expected)
		}
	}
}

func TestBuild_BuiltInPromptsAreDomainNeutral(t *testing.T) {
	prompts := strings.ToLower(strings.Join([]string{
		defaultPersona,
		buildModePrompt,
		planModePrompt,
		compactionPrompt,
		defaultBtwPrompt,
		defaultAdversaryPrompt,
	}, "\n"))
	for _, codingSpecific := range []string{
		"coding agent",
		"software engineering",
		"codebase",
		"code changes",
		"bugs",
		"refactor",
		"go.mod",
		"package.json",
		"git commit",
		"git reset",
	} {
		if strings.Contains(prompts, codingSpecific) {
			t.Errorf("built-in prompts contain coding-specific instruction %q", codingSpecific)
		}
	}
}

func TestBuild_ContainsWorkingDir(t *testing.T) {
	dir := t.TempDir()
	result := buildDefault(dir, ModeBuild)
	if !strings.Contains(result, dir) {
		t.Errorf("expected output to contain working dir %q", dir)
	}
}

func TestBuild_DoesNotDiscoverImplicitInstructionFiles(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"AGENTS.md": "agent instructions",
		"CLAUDE.md": "claude instructions",
		"GEMINI.md": "gemini instructions",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	result := buildDefault(dir, ModeBuild)
	for _, unexpected := range []string{"# Project Instructions", "agent instructions", "claude instructions", "gemini instructions"} {
		if strings.Contains(result, unexpected) {
			t.Fatalf("unexpected implicit instruction %q in prompt", unexpected)
		}
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
	for _, expected := range []string{"# Active mode: plan", "Focus on the user's defined role and task", "Do not modify files", "actions that require changes", "/mode build or Shift+Tab"} {
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

func TestBuild_IncludesEssentialToolUseInstructions(t *testing.T) {
	result := buildDefault(t.TempDir(), ModeBuild)
	for _, expected := range []string{
		"Treat tool results as evidence only after the tool call completes",
		"If a tool fails, is unavailable, or is denied",
		"When current or mutable external information is needed",
		"Do not narrate tool use before acting",
	} {
		if !strings.Contains(result, expected) {
			t.Fatalf("expected %q in prompt, got %q", expected, result)
		}
	}

	for _, removed := range []string{
		"Raw tool arguments and outputs are only retained within the current turn",
		"Prior-turn tool calls may appear as system-generated provider tool blocks",
		"A successful tool call remains usable for the rest of the current turn",
		"Do not add a separate summary for your own memory",
	} {
		if strings.Contains(result, removed) {
			t.Fatalf("did not expect obsolete tool-memory instruction %q in prompt", removed)
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
	if !strings.Contains(result, "# Tool use") || strings.Index(result, "# Tool use") > strings.Index(result, "PostgreSQL DBA agent") {
		t.Fatal("expected harness contract before config persona")
	}
	if strings.Contains(result, "# Working style") {
		t.Fatal("expected default style to be replaced by config persona")
	}
	for _, expected := range []string{"# Tool use", "# Safety"} {
		if !strings.Contains(result, expected) {
			t.Fatalf("expected harness contract section %q to be retained", expected)
		}
	}
	if strings.Index(result, harnessContract) > strings.Index(result, cfg.SystemPrompt) {
		t.Fatal("expected harness contract before inline system_prompt")
	}
}

func TestBuild_ConfigPersonaWithFiles(t *testing.T) {
	dir := t.TempDir()
	promptFile := filepath.Join(dir, "extra.md")
	if err := os.WriteFile(promptFile, []byte("Additional context about databases."), 0644); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	cfg := &agentconfig.Config{
		SystemPrompt:      "You are a DBA.",
		SystemPromptFiles: agentconfig.StringOrArray{promptFile},
	}

	result := Build(dir, "", "", ModeBuild, cfg)
	for _, expected := range []string{harnessContract, "You are a DBA.", "Additional context about databases."} {
		if !strings.Contains(result, expected) {
			t.Fatalf("expected %q in prompt", expected)
		}
	}
	if strings.Contains(result, "# Working style") {
		t.Fatal("expected default style to be replaced by config persona")
	}
	contractIdx := strings.Index(result, harnessContract)
	inlineIdx := strings.Index(result, "You are a DBA.")
	fileIdx := strings.Index(result, "Additional context about databases.")
	if contractIdx > inlineIdx || inlineIdx > fileIdx {
		t.Fatal("expected harness contract, inline system_prompt, then system_prompt_files content")
	}
}

func TestBuild_ConfigFilesOnlyReplaceDefaultStyle(t *testing.T) {
	dir := t.TempDir()
	promptFile := filepath.Join(dir, "persona.md")
	if err := os.WriteFile(promptFile, []byte("You are a support triage agent."), 0644); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	cfg := &agentconfig.Config{
		SystemPromptFiles: agentconfig.StringOrArray{promptFile},
	}
	result := Build(dir, "", "", ModeBuild, cfg)
	if !strings.Contains(result, "You are a support triage agent.") {
		t.Fatal("expected file persona in prompt")
	}
	if !strings.Contains(result, "# Tool use") {
		t.Fatal("expected harness contract to be retained")
	}
	if strings.Contains(result, "# Working style") {
		t.Fatal("expected default style to be replaced by config persona files")
	}
}

func TestBuild_ConfigFallbackToDefault(t *testing.T) {
	dir := t.TempDir()
	cfg := &agentconfig.Config{}
	result := Build(dir, "", "", ModeBuild, cfg)
	if !strings.Contains(result, "Keen Agent") {
		t.Fatal("expected default persona when config has no system_prompt")
	}
	if !strings.Contains(result, "# Working style") {
		t.Fatal("expected default style when config has no system_prompt")
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

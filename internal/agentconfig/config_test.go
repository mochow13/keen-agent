package agentconfig

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "agent.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return path
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}

func TestLoad_FullConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "prompts", "additional.md"), "additional")
	writeFile(t, filepath.Join(dir, "prompts", "build.md"), "build")
	writeFile(t, filepath.Join(dir, "prompts", "btw.md"), "btw")
	writeFile(t, filepath.Join(dir, "prompts", "adversary.md"), "adversary")
	writeFile(t, filepath.Join(dir, "schemas", "run_query.json"), `{"type":"object"}`)
	if err := os.MkdirAll(filepath.Join(dir, "subagents"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "skills"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mcp-config.json"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}

	content := `name: "SQL DBA Agent"
model:
  provider: anthropic
  model_id: claude-sonnet-4-20250514
system_prompt: |
  You are a PostgreSQL DBA.
system_prompt_files:
  - ./prompts/additional.md
project_instructions: AGENT_RULES.md
default_mode: plan
modes:
  build:
    system_prompt: Build mode prompt.
    system_prompt_files:
      - ./prompts/build.md
  plan:
    system_prompt: Plan mode prompt.
btw:
  enabled: true
  context_messages: 10
  model:
    provider: openai
    model_id: gpt-5.4-mini
  system_prompt: Be concise.
  system_prompt_files:
    - ./prompts/btw.md
adversary:
  enabled: true
  model:
    provider: anthropic
    model_id: claude-sonnet-4-20250514
  system_prompt: Find bugs.
  system_prompt_files:
    - /etc/keen/adversary.md
builtin_tools:
  exclude:
    - write_file
    - bash
  bash:
    permission: requires_approval
    rules:
      - match: ["rm", "drop"]
        permission: deny
functions:
  - name: run_query
    description: Execute a read-only SQL query
    command: python3 ./functions/run_query.py
    input_schema_file: ./schemas/run_query.json
    read_only: true
    permission: auto_approve
    timeout: 30s
    max_retries: 2
subagents_dirs:
  - ./subagents
mcp_config_dirs:
  - ./mcp-config.json
skills_dirs:
  - ./skills
`

	path := writeConfig(t, dir, content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Name != "SQL DBA Agent" {
		t.Errorf("expected name 'SQL DBA Agent', got %q", cfg.Name)
	}
	if cfg.Model == nil || cfg.Model.Provider != "anthropic" || cfg.Model.ModelID != "claude-sonnet-4-20250514" {
		t.Errorf("unexpected model: %+v", cfg.Model)
	}
	if cfg.SystemPrompt != "You are a PostgreSQL DBA.\n" {
		t.Errorf("unexpected system prompt: %q", cfg.SystemPrompt)
	}
	if len(cfg.SystemPromptFiles) != 1 || cfg.SystemPromptFiles[0] != "./prompts/additional.md" {
		t.Errorf("unexpected system prompt files: %v", cfg.SystemPromptFiles)
	}
	if cfg.ProjectInstructions != "AGENT_RULES.md" {
		t.Errorf("unexpected project instructions: %q", cfg.ProjectInstructions)
	}
	if cfg.EffectiveDefaultMode() != "plan" {
		t.Errorf("expected default mode plan, got %q", cfg.EffectiveDefaultMode())
	}
	if len(cfg.Modes) != 2 {
		t.Errorf("expected 2 modes, got %d", len(cfg.Modes))
	}
	if cfg.Modes["build"].SystemPrompt != "Build mode prompt." {
		t.Errorf("unexpected build mode prompt: %q", cfg.Modes["build"].SystemPrompt)
	}
	if len(cfg.Modes["build"].SystemPromptFiles) != 1 {
		t.Errorf("expected 1 build mode prompt file, got %d", len(cfg.Modes["build"].SystemPromptFiles))
	}
	if cfg.Btw == nil || !cfg.Btw.Enabled {
		t.Fatal("expected btw enabled")
	}
	if cfg.Btw.ContextMessages != 10 {
		t.Errorf("expected btw context_messages 10, got %d", cfg.Btw.ContextMessages)
	}
	if cfg.Btw.Model == nil || cfg.Btw.Model.Provider != "openai" {
		t.Errorf("unexpected btw model: %+v", cfg.Btw.Model)
	}
	if cfg.Adversary == nil || !cfg.Adversary.Enabled {
		t.Fatal("expected adversary enabled")
	}
	if len(cfg.Adversary.SystemPromptFiles) != 1 || cfg.Adversary.SystemPromptFiles[0] != "/etc/keen/adversary.md" {
		t.Errorf("unexpected adversary prompt files: %v", cfg.Adversary.SystemPromptFiles)
	}
	if cfg.BuiltinTools == nil || len(cfg.BuiltinTools.Exclude) != 2 {
		t.Errorf("unexpected builtin tools: %+v", cfg.BuiltinTools)
	}
	if cfg.BuiltinTools.Bash.Permission != "requires_approval" {
		t.Errorf("unexpected bash permission: %q", cfg.BuiltinTools.Bash.Permission)
	}
	if len(cfg.BuiltinTools.Bash.Rules) != 1 || cfg.BuiltinTools.Bash.Rules[0].Permission != "deny" {
		t.Errorf("unexpected bash rules: %+v", cfg.BuiltinTools.Bash.Rules)
	}
	if len(cfg.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(cfg.Functions))
	}
	fn := cfg.Functions[0]
	if fn.Name != "run_query" {
		t.Errorf("expected function name run_query, got %q", fn.Name)
	}
	if fn.EffectivePermission() != "auto_approve" {
		t.Errorf("expected permission auto_approve, got %q", fn.EffectivePermission())
	}
	if fn.EffectiveTimeout() != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", fn.EffectiveTimeout())
	}

	resolvedSPF := cfg.ResolvedSystemPromptFiles()
	if len(resolvedSPF) != 1 || resolvedSPF[0] != filepath.Join(dir, "prompts", "additional.md") {
		t.Errorf("unexpected resolved system prompt files: %v", resolvedSPF)
	}
	if cfg.ResolvedProjectInstructions() != filepath.Join(dir, "AGENT_RULES.md") {
		t.Errorf("unexpected resolved project instructions: %q", cfg.ResolvedProjectInstructions())
	}
	resolvedBuild := cfg.ResolvedModeSystemPromptFiles("build")
	if len(resolvedBuild) != 1 || resolvedBuild[0] != filepath.Join(dir, "prompts", "build.md") {
		t.Errorf("unexpected resolved build mode prompt files: %v", resolvedBuild)
	}
	if len(cfg.ResolvedModeSystemPromptFiles("plan")) != 0 {
		t.Errorf("expected empty plan mode prompt files, got %v", cfg.ResolvedModeSystemPromptFiles("plan"))
	}
	resolvedBtw := cfg.ResolvedBtwSystemPromptFiles()
	if len(resolvedBtw) != 1 || resolvedBtw[0] != filepath.Join(dir, "prompts", "btw.md") {
		t.Errorf("unexpected resolved btw prompt files: %v", resolvedBtw)
	}
	resolvedAdv := cfg.ResolvedAdversarySystemPromptFiles()
	if len(resolvedAdv) != 1 || resolvedAdv[0] != "/etc/keen/adversary.md" {
		t.Errorf("unexpected resolved adversary prompt files: %v", resolvedAdv)
	}
	resolvedSub := cfg.ResolvedSubagentsDirs()
	if len(resolvedSub) != 1 || resolvedSub[0] != filepath.Join(dir, "subagents") {
		t.Errorf("unexpected resolved subagents dirs: %v", resolvedSub)
	}
	resolvedMCP := cfg.ResolvedMCPConfigDirs()
	if len(resolvedMCP) != 1 || resolvedMCP[0] != filepath.Join(dir, "mcp-config.json") {
		t.Errorf("unexpected resolved mcp config dirs: %v", resolvedMCP)
	}
	resolvedSkills := cfg.ResolvedSkillsDirs()
	if len(resolvedSkills) != 1 || resolvedSkills[0] != filepath.Join(dir, "skills") {
		t.Errorf("unexpected resolved skills dirs: %v", resolvedSkills)
	}
	if cfg.ResolvedFunctionInputSchemaFile(0) != filepath.Join(dir, "schemas", "run_query.json") {
		t.Errorf("unexpected resolved function schema file: %q", cfg.ResolvedFunctionInputSchemaFile(0))
	}
	if cfg.ResolvedFunctionInputSchemaFile(1) != "" {
		t.Errorf("expected empty out-of-range function schema file, got %q", cfg.ResolvedFunctionInputSchemaFile(1))
	}
}

func TestLoad_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "name: minimal\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.EffectiveDefaultMode() != DefaultMode {
		t.Errorf("expected default mode %q, got %q", DefaultMode, cfg.EffectiveDefaultMode())
	}
	if cfg.ResolveConfigPath("./rel") != filepath.Join(dir, "rel") {
		t.Errorf("expected relative path resolved against config dir")
	}
	if cfg.ResolveConfigPath("/abs") != "/abs" {
		t.Errorf("expected absolute path unchanged")
	}
	if cfg.BaseDir() != dir {
		t.Errorf("expected base dir %q, got %q", dir, cfg.BaseDir())
	}
}

func TestPathResolution(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `name: paths
system_prompt_files:
  - ./prompts/main.md
project_instructions: AGENT_RULES.md
subagents_dirs:
  - /var/agents/subagents
skills_dirs:
  - ./skills
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := cfg.ResolveConfigPath("./rel"); got != filepath.Join(dir, "rel") {
		t.Errorf("ResolveConfigPath(./rel) = %q, want %q", got, filepath.Join(dir, "rel"))
	}
	if got := cfg.ResolveConfigPath("/abs"); got != "/abs" {
		t.Errorf("ResolveConfigPath(/abs) = %q, want /abs", got)
	}
	if got := cfg.ResolveConfigPath(""); got != "" {
		t.Errorf("ResolveConfigPath(\"\") = %q, want empty", got)
	}

	resolvedSPF := cfg.ResolvedSystemPromptFiles()
	if len(resolvedSPF) != 1 || resolvedSPF[0] != filepath.Join(dir, "prompts", "main.md") {
		t.Errorf("ResolvedSystemPromptFiles() = %v, want %q", resolvedSPF, filepath.Join(dir, "prompts", "main.md"))
	}
	if got := cfg.ResolvedProjectInstructions(); got != filepath.Join(dir, "AGENT_RULES.md") {
		t.Errorf("ResolvedProjectInstructions() = %q, want %q", got, filepath.Join(dir, "AGENT_RULES.md"))
	}
	resolvedSub := cfg.ResolvedSubagentsDirs()
	if len(resolvedSub) != 1 || resolvedSub[0] != "/var/agents/subagents" {
		t.Errorf("ResolvedSubagentsDirs() = %v, want /var/agents/subagents", resolvedSub)
	}
	resolvedSkills := cfg.ResolvedSkillsDirs()
	if len(resolvedSkills) != 1 || resolvedSkills[0] != filepath.Join(dir, "skills") {
		t.Errorf("ResolvedSkillsDirs() = %v, want %q", resolvedSkills, filepath.Join(dir, "skills"))
	}

	cwd, _ := os.Getwd()
	if got := cfg.ResolveCwdPath("./rel"); got != filepath.Join(cwd, "rel") {
		t.Errorf("ResolveCwdPath(./rel) = %q, want %q", got, filepath.Join(cwd, "rel"))
	}
	if got := cfg.ResolveCwdPath("/abs"); got != "/abs" {
		t.Errorf("ResolveCwdPath(/abs) = %q, want /abs", got)
	}
	if got := cfg.ResolveCwdPath(""); got != "" {
		t.Errorf("ResolveCwdPath(\"\") = %q, want empty", got)
	}
}

func TestLoad_StringOrArrayBackwardCompat(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `name: compat
system_prompt: hi
subagents_dirs: ./subagents
mcp_config_dirs: ./mcp.json
skills_dirs: ./skills
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.SubagentsDirs) != 1 || cfg.SubagentsDirs[0] != "./subagents" {
		t.Errorf("unexpected subagents_dirs: %v", cfg.SubagentsDirs)
	}
	if len(cfg.MCPConfigDirs) != 1 || cfg.MCPConfigDirs[0] != "./mcp.json" {
		t.Errorf("unexpected mcp_config_dirs: %v", cfg.MCPConfigDirs)
	}
	if len(cfg.SkillsDirs) != 1 || cfg.SkillsDirs[0] != "./skills" {
		t.Errorf("unexpected skills_dirs: %v", cfg.SkillsDirs)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestFunctionDef_Defaults(t *testing.T) {
	fn := FunctionDef{}
	if fn.EffectivePermission() != PermissionAutoApprove {
		t.Errorf("expected default permission %q, got %q", PermissionAutoApprove, fn.EffectivePermission())
	}
	if fn.EffectiveTimeout() != DefaultFunctionTimeout {
		t.Errorf("expected default timeout %v, got %v", DefaultFunctionTimeout, fn.EffectiveTimeout())
	}
}

func TestValidate_Minimal(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "name: minimal\nsystem_prompt: hello\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	res := Validate(cfg)
	if !res.OK() {
		t.Fatalf("expected validation to pass, got errors: %+v", res.Errors)
	}
}

func TestValidate_RequiredFields(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "name: missing-prompt\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	res := Validate(cfg)
	if res.OK() {
		t.Fatal("expected validation to fail")
	}
	found := false
	for _, e := range res.Errors {
		if e.Path == "system_prompt" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected system_prompt error, got: %+v", res.Errors)
	}
}

func TestValidate_InvalidMode(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "name: bad-mode\nsystem_prompt: hi\ndefault_mode: debug\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	res := Validate(cfg)
	if res.OK() {
		t.Fatal("expected validation to fail")
	}
	found := false
	for _, e := range res.Errors {
		if e.Path == "default_mode" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected default_mode error, got: %+v", res.Errors)
	}
}

func TestValidate_MissingSystemPromptFile(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "name: missing-file\nsystem_prompt_files:\n  - ./missing.md\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	res := Validate(cfg)
	if res.OK() {
		t.Fatal("expected validation to fail")
	}
	found := false
	for _, e := range res.Errors {
		if e.Path == "system_prompt_files[0]" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected system_prompt_files error, got: %+v", res.Errors)
	}
}

func TestValidate_FunctionErrors(t *testing.T) {
	dir := t.TempDir()
	content := `name: bad-fn
system_prompt: hi
functions:
  - name: ""
    description: ""
    command: ""
    input_schema_file: ./schema.txt
`
	path := writeConfig(t, dir, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	res := Validate(cfg)
	if res.OK() {
		t.Fatal("expected validation to fail")
	}
	paths := map[string]bool{}
	for _, e := range res.Errors {
		paths[e.Path] = true
	}
	want := map[string]bool{
		"functions[0].name":              true,
		"functions[0].description":       true,
		"functions[0].command":           true,
		"functions[0].input_schema_file": true,
	}
	for p := range want {
		if !paths[p] {
			t.Errorf("expected error at %s, got errors: %+v", p, res.Errors)
		}
	}
}

func TestValidate_BuiltinToolsExclude(t *testing.T) {
	dir := t.TempDir()
	content := `name: bad-exclude
system_prompt: hi
builtin_tools:
  exclude:
    - call_mcp_tool
    - unknown_tool
`
	path := writeConfig(t, dir, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	res := Validate(cfg)
	if res.OK() {
		t.Fatal("expected validation to fail")
	}
	foundCore := false
	foundUnknown := false
	for _, e := range res.Errors {
		if e.Path == "builtin_tools.exclude" {
			if contains(e.Message, "cannot be excluded") {
				foundCore = true
			}
			if contains(e.Message, "not a known") {
				foundUnknown = true
			}
		}
	}
	if !foundCore {
		t.Errorf("expected error for excluding core tool, got: %+v", res.Errors)
	}
	if !foundUnknown {
		t.Errorf("expected error for unknown tool, got: %+v", res.Errors)
	}
}

func TestValidate_SubagentFrontmatter(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "subagents")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(subDir, "bad.md"), "---\n---\nbody")

	content := `name: subagent-test
system_prompt: hi
subagents_dirs:
  - ./subagents
`
	path := writeConfig(t, dir, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	res := Validate(cfg)
	if res.OK() {
		t.Fatal("expected validation to fail")
	}
	foundName := false
	foundDesc := false
	for _, e := range res.Errors {
		if contains(e.Path, "bad.md.name") {
			foundName = true
		}
		if contains(e.Path, "bad.md.description") {
			foundDesc = true
		}
	}
	if !foundName {
		t.Errorf("expected missing name error, got: %+v", res.Errors)
	}
	if !foundDesc {
		t.Errorf("expected missing description error, got: %+v", res.Errors)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

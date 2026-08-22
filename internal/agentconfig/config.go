package agentconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ModePlan  = "plan"
	ModeBuild = "build"

	PermissionAutoApprove      = "auto_approve"
	PermissionRequiresApproval = "requires_approval"
	PermissionDeny             = "deny"

	DefaultMode = ModeBuild
)

// StringOrArray accepts either a single YAML string or a sequence of strings.
type StringOrArray []string

func (s *StringOrArray) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		*s = []string{n.Value}
		return nil
	}
	if n.Kind == yaml.SequenceNode {
		vals := make([]string, len(n.Content))
		for i, c := range n.Content {
			if c.Kind != yaml.ScalarNode {
				return fmt.Errorf("expected string at position %d, got %s", i, kindName(c.Kind))
			}
			vals[i] = c.Value
		}
		*s = vals
		return nil
	}
	return fmt.Errorf("expected string or array of strings, got %s", kindName(n.Kind))
}

type Config struct {
	Name                string                `yaml:"name"`
	ASCIIArt            string                `yaml:"ascii_art,omitempty"`
	Model               *ModelRef             `yaml:"model,omitempty"`
	SystemPrompt        string                `yaml:"system_prompt,omitempty"`
	SystemPromptFiles   StringOrArray         `yaml:"system_prompt_files,omitempty"`
	ProjectInstructions string                `yaml:"project_instructions,omitempty"`
	DefaultMode         string                `yaml:"default_mode,omitempty"`
	Modes               map[string]ModeConfig `yaml:"modes,omitempty"`
	Btw                 *BtwConfig            `yaml:"btw,omitempty"`
	Adversary           *AdversaryConfig      `yaml:"adversary,omitempty"`
	BuiltinTools        *BuiltinTools         `yaml:"builtin_tools,omitempty"`
	SubagentsDirs       StringOrArray         `yaml:"subagents_dirs,omitempty"`
	MCPConfigPaths      StringOrArray         `yaml:"mcp_config_paths,omitempty"`
	SkillsDirs          StringOrArray         `yaml:"skills_dirs,omitempty"`
	baseDir             string
	cwd                 string
}

func (c *Config) BaseDir() string { return c.baseDir }

func (c *Config) ResolveConfigPath(p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(c.baseDir, p)
}

func (c *Config) ResolveCwdPath(p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(c.cwd, p)
}

func (c *Config) ResolvedSystemPromptFiles() []string {
	return c.resolveAll(c.SystemPromptFiles)
}

func (c *Config) ResolvedProjectInstructions() string {
	return c.ResolveConfigPath(c.ProjectInstructions)
}

func (c *Config) ResolvedModeSystemPromptFiles(mode string) []string {
	return c.resolveAll(c.Modes[mode].SystemPromptFiles)
}

func (c *Config) ResolvedBtwSystemPromptFiles() []string {
	if c.Btw == nil {
		return nil
	}
	return c.resolveAll(c.Btw.SystemPromptFiles)
}

func (c *Config) ResolvedAdversarySystemPromptFiles() []string {
	if c.Adversary == nil {
		return nil
	}
	return c.resolveAll(c.Adversary.SystemPromptFiles)
}

func (c *Config) ResolvedSubagentsDirs() []string {
	return c.resolveAll(c.SubagentsDirs)
}

func (c *Config) ResolvedMCPConfigPaths() []string {
	return c.resolveAll(c.MCPConfigPaths)
}

func (c *Config) ResolvedSkillsDirs() []string {
	return c.resolveAll(c.SkillsDirs)
}

func (c *Config) resolveAll(paths StringOrArray) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		out = append(out, c.ResolveConfigPath(p))
	}
	return out
}

func (c *Config) EffectiveDefaultMode() string {
	if c.DefaultMode == "" {
		return DefaultMode
	}
	return c.DefaultMode
}

func (c *Config) AgentSlug() string {
	name := strings.ToLower(strings.TrimSpace(c.Name))
	name = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		name = "agent"
	}

	hash := sha256.Sum256([]byte(c.baseDir))
	hashStr := hex.EncodeToString(hash[:])[:8]

	return fmt.Sprintf("%s-%s", name, hashStr)
}

type ModelRef struct {
	Provider string `yaml:"provider,omitempty"`
	ModelID  string `yaml:"model_id,omitempty"`
}

func (m *ModelRef) IsSet() bool {
	return m != nil && (m.Provider != "" || m.ModelID != "")
}

func (m *ModelRef) IsComplete() bool {
	return m != nil && m.Provider != "" && m.ModelID != ""
}

type ModeConfig struct {
	SystemPrompt      string        `yaml:"system_prompt,omitempty"`
	SystemPromptFiles StringOrArray `yaml:"system_prompt_files,omitempty"`
}

type BtwConfig struct {
	Enabled           bool          `yaml:"enabled"`
	ContextMessages   int           `yaml:"context_messages,omitempty"`
	SystemPrompt      string        `yaml:"system_prompt,omitempty"`
	SystemPromptFiles StringOrArray `yaml:"system_prompt_files,omitempty"`
}

func (c *Config) BtwEnabled() bool {
	return c != nil && c.Btw != nil && c.Btw.Enabled
}

func (c *Config) AdversaryEnabled() bool {
	return c != nil && c.Adversary != nil && c.Adversary.Enabled
}

func (c *Config) BtwContextMessages() int {
	if !c.BtwEnabled() {
		return 0
	}
	return c.Btw.ContextMessages
}

type AdversaryConfig struct {
	Enabled           bool          `yaml:"enabled"`
	Model             *ModelRef     `yaml:"model,omitempty"`
	SystemPrompt      string        `yaml:"system_prompt,omitempty"`
	SystemPromptFiles StringOrArray `yaml:"system_prompt_files,omitempty"`
}

type BuiltinTools struct {
	Exclude []string    `yaml:"exclude,omitempty"`
	Bash    *BashPolicy `yaml:"bash,omitempty"`
}

type BashPolicy struct {
	Permission string     `yaml:"permission,omitempty"`
	Rules      []BashRule `yaml:"rules,omitempty"`
}

type BashRule struct {
	Match      []string `yaml:"match,omitempty"`
	Permission string   `yaml:"permission,omitempty"`
}

func Load(path string) (*Config, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config path: %w", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read agent config %q: %w", absPath, err)
	}

	var cfg Config
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse agent config %q: %w", absPath, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("failed to parse agent config %q: expected a single YAML document", absPath)
	}

	cfg.baseDir = filepath.Dir(absPath)
	cfg.cwd, _ = os.Getwd()

	return &cfg, nil
}

func kindName(k yaml.Kind) string {
	switch k {
	case yaml.ScalarNode:
		return "scalar"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	default:
		return "unknown"
	}
}

// ValidationIssue is a single fatal error or warning produced by Validate.
type ValidationIssue struct {
	Path    string
	Message string
}

// ValidationResult collects fatal errors and warnings separately.
type ValidationResult struct {
	Errors   []ValidationIssue
	Warnings []ValidationIssue
}

func (r *ValidationResult) OK() bool {
	return len(r.Errors) == 0
}

func (r *ValidationResult) addError(path, msg string) {
	r.Errors = append(r.Errors, ValidationIssue{Path: path, Message: msg})
}

func (r *ValidationResult) addWarning(path, msg string) {
	r.Warnings = append(r.Warnings, ValidationIssue{Path: path, Message: msg})
}

func Validate(cfg *Config) *ValidationResult {
	res := &ValidationResult{}
	if cfg == nil {
		res.addError("", "config is nil")
		return res
	}

	validateRequiredFields(cfg, res)
	validateScalarShape(cfg, res)
	validateFileExistence(cfg, res)
	validateContent(cfg, res)
	validateCrossReferences(cfg, res)

	return res
}

func validateRequiredFields(cfg *Config, res *ValidationResult) {
	if strings.TrimSpace(cfg.Name) == "" {
		res.addError("name", "required field missing")
	}
	if strings.TrimSpace(cfg.SystemPrompt) == "" && len(cfg.SystemPromptFiles) == 0 {
		res.addError("system_prompt", "either system_prompt or system_prompt_files must be provided")
	}
}

func validateScalarShape(cfg *Config, res *ValidationResult) {
	if cfg.DefaultMode != "" && cfg.DefaultMode != ModePlan && cfg.DefaultMode != ModeBuild {
		res.addError("default_mode", fmt.Sprintf("must be %q or %q", ModePlan, ModeBuild))
	}
	for mode := range cfg.Modes {
		if mode != ModePlan && mode != ModeBuild {
			res.addError("modes", fmt.Sprintf("unknown mode %q; only %q and %q are allowed", mode, ModePlan, ModeBuild))
		}
	}
	if cfg.Model != nil && !cfg.Model.IsComplete() {
		res.addError("model", "model block requires both provider and model_id")
	}
	if cfg.Btw != nil && cfg.Btw.Enabled {
		if cfg.Btw.ContextMessages <= 0 {
			res.addError("btw.context_messages", "must be positive")
		}
	}
	if cfg.Adversary != nil && cfg.Adversary.Enabled {
		if cfg.Adversary.Model != nil && !cfg.Adversary.Model.IsComplete() {
			res.addError("adversary.model", "model block requires both provider and model_id")
		}
	}
}

func validateFileExistence(cfg *Config, res *ValidationResult) {
	for i, p := range cfg.ResolvedSystemPromptFiles() {
		if _, err := os.Stat(p); err != nil {
			res.addError(fmt.Sprintf("system_prompt_files[%d]", i), fmt.Sprintf("%q: %v", p, err))
		}
	}
	for mode := range cfg.Modes {
		for i, p := range cfg.ResolvedModeSystemPromptFiles(mode) {
			if _, err := os.Stat(p); err != nil {
				res.addError(fmt.Sprintf("modes.%s.system_prompt_files[%d]", mode, i), fmt.Sprintf("%q: %v", p, err))
			}
		}
	}
	if cfg.Btw != nil && cfg.Btw.Enabled {
		for i, p := range cfg.ResolvedBtwSystemPromptFiles() {
			if _, err := os.Stat(p); err != nil {
				res.addError(fmt.Sprintf("btw.system_prompt_files[%d]", i), fmt.Sprintf("%q: %v", p, err))
			}
		}
	}
	if cfg.Adversary != nil && cfg.Adversary.Enabled {
		for i, p := range cfg.ResolvedAdversarySystemPromptFiles() {
			if _, err := os.Stat(p); err != nil {
				res.addError(fmt.Sprintf("adversary.system_prompt_files[%d]", i), fmt.Sprintf("%q: %v", p, err))
			}
		}
	}
	for i, p := range cfg.ResolvedMCPConfigPaths() {
		if _, err := os.Stat(p); err != nil {
			res.addError(fmt.Sprintf("mcp_config_paths[%d]", i), fmt.Sprintf("%q: %v", p, err))
		}
	}
	for i, p := range cfg.ResolvedSkillsDirs() {
		if fi, err := os.Stat(p); err != nil {
			res.addError(fmt.Sprintf("skills_dirs[%d]", i), fmt.Sprintf("%q: %v", p, err))
		} else if !fi.IsDir() {
			res.addError(fmt.Sprintf("skills_dirs[%d]", i), fmt.Sprintf("%q is not a directory", p))
		}
	}
	for i, p := range cfg.ResolvedSubagentsDirs() {
		if fi, err := os.Stat(p); err != nil {
			res.addError(fmt.Sprintf("subagents_dirs[%d]", i), fmt.Sprintf("%q: %v", p, err))
		} else if !fi.IsDir() {
			res.addError(fmt.Sprintf("subagents_dirs[%d]", i), fmt.Sprintf("%q is not a directory", p))
		}
	}
}

func validateContent(cfg *Config, res *ValidationResult) {
	names := map[string]string{}
	for i, dir := range cfg.ResolvedSubagentsDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // already reported
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			errPath := fmt.Sprintf("subagents_dirs[%d].%s", i, e.Name())
			name, err := validateSubagentFrontmatter(path, errPath, res)
			if err != nil || name == "" {
				continue
			}
			if first, exists := names[name]; exists {
				res.addError(errPath+".name", fmt.Sprintf("duplicate subagent name %q; already used by %s", name, first))
				continue
			}
			names[name] = errPath
		}
	}
}

func validateSubagentFrontmatter(path, errPath string, res *ValidationResult) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		res.addError(errPath, fmt.Sprintf("failed to read: %v", err))
		return "", err
	}
	content := string(data)
	if !strings.HasPrefix(content, "---") {
		res.addError(errPath, "missing YAML frontmatter")
		return "", fmt.Errorf("missing frontmatter")
	}
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		res.addError(errPath, "invalid YAML frontmatter")
		return "", fmt.Errorf("invalid frontmatter")
	}
	var front struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(parts[1]), &front); err != nil {
		res.addError(errPath, fmt.Sprintf("invalid YAML frontmatter: %v", err))
		return "", err
	}
	name := strings.TrimSpace(front.Name)
	if name == "" {
		res.addError(errPath+".name", "required field missing")
	}
	if strings.TrimSpace(front.Description) == "" {
		res.addError(errPath+".description", "required field missing")
	}
	return name, nil
}

func validateCrossReferences(cfg *Config, res *ValidationResult) {
	excludable := map[string]bool{
		"read_file": true, "write_file": true, "edit_file": true,
		"web_fetch": true, "glob": true, "grep": true, "bash": true,
	}
	nonExcludable := map[string]bool{
		"call_mcp_tool": true,
		"delegate_task": true,
	}
	if cfg.BuiltinTools != nil {
		for _, name := range cfg.BuiltinTools.Exclude {
			if nonExcludable[name] {
				res.addError("builtin_tools.exclude", fmt.Sprintf("%q cannot be excluded", name))
			} else if !excludable[name] {
				res.addError("builtin_tools.exclude", fmt.Sprintf("%q is not a known excludable built-in tool", name))
			}
		}
	}
}

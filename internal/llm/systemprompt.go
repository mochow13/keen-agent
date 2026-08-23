package llm

import (
	"fmt"
	"os"
	"strings"

	"github.com/mochow13/keen-agent/internal/agentconfig"
	"github.com/mochow13/keen-agent/internal/memory"
)

type AgentMode string

const (
	ModeBuild AgentMode = "build"
	ModePlan  AgentMode = "plan"
)

const harnessContract = `# Tool use
- Treat tool results as evidence only after the tool call completes or when the result is explicitly present in the conversation.
- If a tool fails, is unavailable, or is denied, say so rather than implying success.
- When current or mutable external information is needed, use an appropriate available tool before relying on it.

# Safety
- Never expose, log, or persist secrets, credentials, private keys, or API keys.
- Refuse requests that facilitate malicious or harmful activity.
- Do not perform destructive actions without the user's explicit permission.`

const buildModePrompt = `

# Active mode: build
- Build mode allows you to take action to complete the user's request within the active instructions and permissions.
`

const planModePrompt = `# Active mode: plan
- Focus on the user's defined role and task. Use read-only tools to investigate and provide plans, explanations, risks, and verification steps.
- Do not modify files, system state, network resources, or external services.
- For actions that require changes, ask the user to switch to build mode with /mode build or Shift+Tab.`

const compactionPrompt = `You are an AI agent for compacting long conversation history.
Your task is to produce a concise but complete summary of the conversation provided. The summary
will replace the earlier part of the conversation so that work can continue without losing important
context. The summary has to be useful and concise.

Structure your summary as follows:

## Goal
What goal(s) is the user trying to accomplish?

## Key Instructions
Important instructions or constraints given by the user.

## Discoveries
Notable facts learned about the subject, context, requirements, or constraints.

## Accomplished
What has been completed, what is in progress, and what remains.

## Relevant Resources
A structured list of files, sources, tools, or other resources that are still important to continue the task.`

const maxInstructionsSize = 8 * 1024

func Build(workingDir, skillsCatalog, subagentsCatalog string, mode AgentMode, agentCfg *agentconfig.Config) string {
	var sb strings.Builder

	persona := resolvePersona(agentCfg)
	sb.WriteString(persona)
	sb.WriteString(fmt.Sprintf("\n\nWorking directory: %s", workingDir))

	instructions := resolveProjectInstructions(agentCfg)
	if instructions != "" {
		sb.WriteString("\n\n")
		sb.WriteString(instructions)
	}

	mem := memorySection(workingDir)
	if mem != "" {
		sb.WriteString("\n\n")
		sb.WriteString(mem)
	}

	if skillsCatalog != "" {
		sb.WriteString("\n\n")
		sb.WriteString(skillsCatalog)
	}

	if subagentsCatalog != "" {
		sb.WriteString("\n\n")
		sb.WriteString(subagentsCatalog)
	}

	modeStr := string(mode)
	if modeStr == "" {
		modeStr = agentconfig.ModeBuild
	}

	if mode == ModePlan {
		sb.WriteString(planModePrompt)
	} else {
		sb.WriteString(buildModePrompt)
	}

	modeOverlay := resolveModeOverlay(agentCfg, modeStr)
	if modeOverlay != "" {
		sb.WriteString("\n\n")
		sb.WriteString(modeOverlay)
	}

	return sb.String()
}

func BuildCompactionPrompt(extraPrompt string) string {
	if trimmed := strings.TrimSpace(extraPrompt); trimmed != "" {
		return compactionPrompt + "\n\nIMPORTANT! User has provided a specific instruction. So take it into consideration: " + trimmed
	}
	return compactionPrompt
}
func BuildAutoCompactionPrompt() string {
	return compactionPrompt + `

This is an internal agent checkpoint. Keen retains the most recent user message verbatim outside this summary. Do not reproduce that user message verbatim. Preserve active-loop progress and meaningful tool results. Output only the structured summary with no preamble.`
}

const defaultBtwPrompt = `You are a helper agent for Keen Agent answering a quick side question ("btw") separate from the main task.

Use the supplied recent conversation context and your knowledge. You have no tool access.

- Be concise and direct.
- Answer the question asked without taking over the main task.`

func BuildBtwPrompt(workingDir string, agentCfg *agentconfig.Config) string {
	prompt := resolveBtwPrompt(agentCfg)
	return prompt + fmt.Sprintf("\n\nWorking directory: %s", workingDir)
}

const defaultAdversaryPrompt = `You are an adversarial critic reviewing the main agent's work in this conversation.

Identify important factual errors, unsafe actions, unsupported assumptions, missed constraints, and risks. Challenge weak reasoning and suggest useful alternatives when appropriate.

Be brief and direct. Lead with the most important issue. If nothing significant is wrong, say so in one sentence.`

func BuildAdversaryPrompt(workingDir string, agentCfg *agentconfig.Config) string {
	prompt := resolveAdversaryPrompt(agentCfg)
	return prompt + fmt.Sprintf("\n\nWorking directory: %s", workingDir)
}

func memorySection(workingDir string) string {
	content := memory.Load(workingDir)
	if content == "" {
		return ""
	}
	return "# Memory\n\n" + content
}

func resolvePersona(cfg *agentconfig.Config) string {
	parts := []string{harnessContract}
	if cfg == nil {
		return harnessContract
	}
	if inline := strings.TrimSpace(cfg.SystemPrompt); inline != "" {
		parts = append(parts, inline)
	}
	for _, f := range cfg.ResolvedSystemPromptFiles() {
		if content := readFileContent(f); content != "" {
			parts = append(parts, content)
		}
	}
	return strings.Join(parts, "\n\n")
}

func resolveProjectInstructions(cfg *agentconfig.Config) string {
	if cfg == nil {
		return ""
	}
	var parts []string
	for _, path := range cfg.ResolvedProjectInstructionPaths() {
		if content := readFileContent(path); content != "" {
			parts = append(parts, fmt.Sprintf("## From %s\n\n%s", path, content))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	content := strings.Join(parts, "\n\n")
	if len(content) > maxInstructionsSize {
		content = content[:maxInstructionsSize] + "\n[truncated]"
	}
	return "# Project Instructions\n\n" + content
}

func resolveModeOverlay(cfg *agentconfig.Config, mode string) string {
	if cfg == nil || len(cfg.Modes) == 0 {
		return ""
	}
	mc, ok := cfg.Modes[mode]
	if !ok {
		return ""
	}

	var parts []string
	if inline := strings.TrimSpace(mc.SystemPrompt); inline != "" {
		parts = append(parts, inline)
	}
	for _, f := range cfg.ResolvedModeSystemPromptFiles(mode) {
		if content := readFileContent(f); content != "" {
			parts = append(parts, content)
		}
	}
	return strings.Join(parts, "\n\n")
}

func resolveBtwPrompt(cfg *agentconfig.Config) string {
	if cfg == nil || cfg.Btw == nil {
		return defaultBtwPrompt
	}

	var parts []string
	if inline := strings.TrimSpace(cfg.Btw.SystemPrompt); inline != "" {
		parts = append(parts, inline)
	}
	for _, f := range cfg.ResolvedBtwSystemPromptFiles() {
		if content := readFileContent(f); content != "" {
			parts = append(parts, content)
		}
	}
	if len(parts) == 0 {
		return defaultBtwPrompt
	}
	return strings.Join(parts, "\n\n")
}

func resolveAdversaryPrompt(cfg *agentconfig.Config) string {
	if cfg == nil || cfg.Adversary == nil {
		return defaultAdversaryPrompt
	}

	var parts []string
	if inline := strings.TrimSpace(cfg.Adversary.SystemPrompt); inline != "" {
		parts = append(parts, inline)
	}
	for _, f := range cfg.ResolvedAdversarySystemPromptFiles() {
		if content := readFileContent(f); content != "" {
			parts = append(parts, content)
		}
	}
	if len(parts) == 0 {
		return defaultAdversaryPrompt
	}
	return strings.Join(parts, "\n\n")
}

func readFileContent(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

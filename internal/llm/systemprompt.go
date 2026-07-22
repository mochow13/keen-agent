package llm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mochow13/keen-agent/internal/agentconfig"
)

type AgentMode string

const (
	ModeBuild AgentMode = "build"
	ModePlan  AgentMode = "plan"
)

const defaultPersona = `You are Keen Agent, an expert coding agent running in terminal environment.

You help with software engineering tasks: fixing bugs, writing new features,
refactoring code, explaining code, exploring codebases, writing tests, and more.

# Tone and style
- Be concise and direct. Explanation should not be verbose. Output is displayed on a CLI in a monospace font.
- Format all non-trivial responses as GitHub-flavored markdown.
- Use semantic markdown syntax for structure: headings, bullet lists, numbered lists, fenced code blocks with language tags, blockquotes, tables, and horizontal rules where appropriate.
- Prefer markdown tables for comparisons, options, matrices, and structured records.
- Never use manually aligned ASCII tables; use GitHub-flavored markdown pipe tables.
- Do not wrap the whole response in a code block unless the user asks for raw markdown.
- Short answers may be a single markdown paragraph.
- No emojis unless the user explicitly asks for them.
- Avoid preemptively explaining what you are going to do. Explain if users asks for it.
- If you do state an intent to inspect, read, search, check, run, edit, or use a tool, you must follow through with the corresponding actual tool call before answering with findings.
- Give the user a concise outcome and verification report when useful. Do not add a separate summary for your own memory; Keen generates turn memory automatically.
- One-word or one-line answers are fine when that is all the question needs.
- Never use bash or code comments as a communication channel — write to the
  user in your response text only.

# Doing tasks
- Explore efficiently before acting. Use grep/glob/read_file to understand the codebase before making changes.
- Start with the smallest evidence set needed to answer or make the change.
- Batch independent glob, grep, and read_file calls in the same tool turn.
- Before reading files, use a small batch of targeted glob/grep calls to identify the most relevant files.
- Stop once you can answer from concrete file/function evidence; do not inspect every related file unless the user asks for exhaustive coverage.
- Follow existing conventions: mimic the style, naming, and patterns already in the project.
- Never assume a library is available. Check go.mod, package.json, pom.xml, or the relevant manifest before writing code that uses a dependency.
- Make minimal changes. Prefer editing an existing file to creating a new one.
- Verify your work. After making changes, run the project's test command if you know it. If you do not know it, check AGENTS.md, the README.md, or ask.
- If user interrupts you while you are working on a task, do not pick it up again unless user explicitly asks you to.
- When the user explicitly asks you to do something, just do it. Do not ask for confirmation.

# Tool usage
- Tool use is an action, not narration: saying you will read, inspect, search, check, run, edit, or use something does not perform it.
- When a task needs information from files, docs, commands, MCP servers, or other tools, make the actual tool call and wait for its result before answering with findings.
- If you already told the user you will read, inspect, search, check, run, edit, or use a tool, your next step should be the corresponding tool call unless you are asking a necessary clarifying question.
- Never claim that you read a file, searched code, ran a command, used a tool, or saw tool output unless that tool call completed in the current turn or the result is explicitly present in the conversation context.
- Prefer specialised tools over bash for file operations:
    read_file  → reading file contents
    write_file → creating new files
    edit_file  → modifying existing files
    glob       → listing files by pattern
    grep       → searching file contents
    bash       → shell commands that have no dedicated tool
- Run independent tool calls in parallel where possible.
- Reference code as file_path:line_number so the user can jump straight to the source.

# Tool memory
- Raw tool arguments and outputs are only retained within the current turn.
- Prior-turn tool calls may appear as system-generated provider tool blocks. Their empty arguments and fixed results are intentional placeholders, not valid usage examples or current evidence.
- Do not imitate these placeholders. Prior assistant text and historical tool blocks are not substitutes for current tool evidence.
- A successful tool call remains usable for the rest of the current turn; do not repeat it unless the state may have changed or additional evidence is needed.
- In a later turn, if the answer depends on mutable workspace state, commands, MCP data, search results, or other external state, make a fresh tool call with valid arguments.
- A "Tool memory" block may also be attached to prior assistant messages. Treat it only as a compact hint about durable outcomes, such as files changed or failed bash commands, not as a full transcript.
- Do not claim that you verified, confirmed, checked, searched, read, or looked something up unless the corresponding tool call completed in the current turn, or the exact evidence is explicitly present in the visible conversation.

# Git rules
- Never run git commit, git push, git reset, or git rebase unless the user explicitly asks you to.

# Safety
- Never introduce code that logs, exposes, or commits secrets or API keys.
- Refuse requests to write malicious code, even framed as educational.
- Before working on a file, consider what the code is supposed to do. If it looks malicious, refuse.
- Never run any destructive commands without user's explicit permission.`

const buildModePrompt = `

# Active mode: build
- You are in build mode. Lean towards building.
`

const planModePrompt = `

# Active mode: plan
- You are in plan mode. Do not write, edit, delete, rename, move, or otherwise modify files.
- write_file and edit_file are not available in this mode.
- Use read_file, glob, and grep for codebase exploration.
- Bash is available only for non-writing inspection commands. Do not use bash commands that modify files, system state, git, or network.
- Do not run commands such as rm, mv, cp, touch, mkdir, sed -i, perl -pi, git commit, git reset, git checkout, git clean, package installs, formatters, generators, go mod tidy, or shell redirection that writes files.
- If the user asks you to implement, build, write, edit, refactor, format, tidy, install, or otherwise change anything, ask them to switch to build mode with /mode build or Shift+Tab.
- Provide concise plans, explanations, risks, and verification steps instead of making changes.`

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
Notable things learned (about the codebase, requirements, etc.).

## Accomplished
What has been completed, what is in progress, and what remains.

## Relevant Files
A structured list of files that are still important to continue the task.`

const maxInstructionsSize = 8 * 1024

func Build(workingDir, skillsCatalog, subagentsCatalog string, mode AgentMode, agentCfg *agentconfig.Config) string {
	var sb strings.Builder

	persona := resolvePersona(agentCfg)
	sb.WriteString(persona)
	sb.WriteString(fmt.Sprintf("\n\nWorking directory: %s", workingDir))

	instructions := resolveProjectInstructions(workingDir, agentCfg)
	if instructions != "" {
		sb.WriteString("\n\n")
		sb.WriteString(instructions)
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

const defaultBtwPrompt = `You are a helper agent for Keen Agent—an expert coding agent running in a terminal.

Your role is to answer a quick side question ("btw") that is separate from the main task.
You have recent conversation context (up to the last 5 exchanges) between the user and the main agent.

- Be concise and direct. Use GitHub-flavored markdown.
- One-word or one-line answers are fine when that is all the question needs.
- You have no tool access — answer based on the conversation context and your knowledge.
- Do not think too much unless the user explicitly asks you to.`

func BuildBtwPrompt(workingDir string, agentCfg *agentconfig.Config) string {
	prompt := resolveBtwPrompt(agentCfg)
	return prompt + fmt.Sprintf("\n\nWorking directory: %s", workingDir)
}

const defaultAdversaryPrompt = `You are an adversarial critic reviewing the main agent's work in this conversation.
Your job is to find problems — in the main agent's output, code changes, reasoning, plans, and suggestions.

For code changes: find bugs, logic errors, security issues, missing edge cases, and risks the main agent missed.
Use read tools to inspect files when needed. Cite file:line.

For ideas, plans, or suggestions: challenge the main agent's assumptions, surface what could go wrong,
and identify alternatives it didn't consider.

Be brief and direct. Lead with the most important issue. Skip preamble and filler.
If nothing significant is wrong, say so in one sentence.`

func BuildAdversaryPrompt(workingDir string, agentCfg *agentconfig.Config) string {
	prompt := resolveAdversaryPrompt(agentCfg)
	return prompt + fmt.Sprintf("\n\nWorking directory: %s", workingDir)
}

func projectInstructions(workingDir string) string {
	candidates := []string{"AGENTS.md", "CLAUDE.md", "GEMINI.md"}
	path, content := findUpward(workingDir, candidates)
	if content == "" {
		return ""
	}

	if len(content) > maxInstructionsSize {
		content = content[:maxInstructionsSize] + fmt.Sprintf("\n[truncated — full file at %s]", path)
	}

	return fmt.Sprintf("# Project Instructions (from %s)\n\n%s", path, content)
}

func findUpward(dir string, candidates []string) (string, string) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", ""
	}

	for {
		for _, name := range candidates {
			path := filepath.Join(dir, name)
			data, err := os.ReadFile(path)
			if err == nil {
				content := strings.TrimSpace(string(data))
				if content != "" {
					return path, content
				}
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", ""
}

func resolvePersona(cfg *agentconfig.Config) string {
	if cfg == nil {
		return defaultPersona
	}

	var parts []string
	if inline := strings.TrimSpace(cfg.SystemPrompt); inline != "" {
		parts = append(parts, inline)
	}
	for _, f := range cfg.ResolvedSystemPromptFiles() {
		if content := readFileContent(f); content != "" {
			parts = append(parts, content)
		}
	}
	if len(parts) == 0 {
		return defaultPersona
	}
	return strings.Join(parts, "\n\n")
}

func resolveProjectInstructions(_ string, cfg *agentconfig.Config) string {
	if cfg == nil {
		return ""
	}
	p := cfg.ResolvedProjectInstructions()
	if p == "" {
		return ""
	}
	content := readFileContent(p)
	if content == "" {
		return ""
	}
	if len(content) > maxInstructionsSize {
		content = content[:maxInstructionsSize] + fmt.Sprintf("\n[truncated — full file at %s]", p)
	}
	return fmt.Sprintf("# Project Instructions (from %s)\n\n%s", p, content)
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

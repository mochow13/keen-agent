# keen-agent — Implementation Plan

A generic, config-driven AI agent runner. Users provide system prompts, MCP
configuration, skills, and subagents — keen-agent handles the agent loop,
TUI, permissions, and LLM interaction.

---

## Overview

keen-agent is a separate binary/repository that extracts and reuses core
infrastructure from keen-code (LLM client, permission system, TUI, built-in tools,
skill loading, MCP client, subagent system) but replaces the hardcoded coding-agent behavior with a
user-defined agent configuration.

### Relationship to keen-code (copy-fork, drift is acceptable)

keen-agent is a **generic agent harness**, not a coding agent. keen-code remains the
opinionated coding agent and keeps its tight couplings (REPL-bound permission flow,
hardcoded persona, `AGENTS.md`/`CLAUDE.md`/`GEMINI.md` discovery, build/plan naming).

The relevant keen-code packages are **copied and forked**, not shared via a common
module. This is a deliberate choice:

- A shared module would force lowest-common-denominator interfaces that satisfy both
consumers, creating coordination overhead and constraining keen-code's couplings.
- keen-agent and keen-code have genuinely different needs (headless operation,
parameterized prompt, opt-in coding tools), so they *should* evolve independently.

**Drift between keen-code and keen-agent is fine and expected.** Copied code is a
bootstrap scaffold; once copied, keen-agent owns it and customizes aggressively —
ripping out coding-specific assumptions rather than preserving parity.

### Namespace isolation and per-agent state

keen-agent must not collide with keen-code on disk or in environment. Anywhere
keen-code reads/writes under a `keen` namespace, keen-agent uses a `keen-agent`
namespace instead.

State is split into:

1. **Shared user account state** — reused across all agents to avoid repeated
  provider setup and OAuth login.
2. **Agent-scoped runtime state** — isolated by agent name for sessions, logs, and
  input history.
3. **User-authored resources** — explicit paths such as `mcp_config_dirs` and
  `skills_dirs`.

Shared state lives directly under `~/.keen-agent/`:

```text
~/.keen-agent/configs.json  # model/provider defaults + API-provider credentials
~/.keen-agent/auth.json    # OAuth credentials for Codex-style providers and MCP OAuth
```

Agent-scoped runtime state uses:

```text
~/.keen-agent/<agent-name>/
```

`<agent-name>` is a filesystem-safe slug derived from the config `name`, with a
stable disambiguator from the absolute `agent.yaml` path if needed to avoid
collisions.


| keen-code                                      | keen-agent                                                                      |
| ---------------------------------------------- | ------------------------------------------------------------------------------- |
| `~/.keen/` (config, sessions, global skills)   | `~/.keen-agent/`                                                                |
| `~/.keen/configs.json` (active provider/model) | `~/.keen-agent/configs.json`                                                    |
| `~/.keen/skills/` (global skills)              | User-selected `skills_dirs` plus optional `~/.keen-agent/skills/` shared skills |
| `~/.keen/sessions/` (or equivalent)            | `~/.keen-agent/<agent-name>/sessions/`                                          |
| `~/.keen/logs/` (or equivalent)                | `~/.keen-agent/<agent-name>/logs/`                                              |
| auth/token storage                             | `~/.keen-agent/auth.json`                                                       |
| input history                                  | `~/.keen-agent/<agent-name>/input-history.jsonl`                                |
| `KEEN_*` env vars                              | `KEEN_AGENT_*` env vars                                                         |


This keeps credentials and model defaults reusable while still isolating each
agent's sessions, logs, and history. The two binaries can coexist on one machine,
and multiple keen-agent builds can coexist without mixing conversation state
accidentally.

**Invocation:**

```bash
# Interactive TUI
keen-agent --agent ./my-agent.yaml

# Headless run
keen-agent run --agent ./my-agent.yaml --provider anthropic --model claude-sonnet-4-20250514 --format json
```

---

## Config Format (`agent.yaml`)

```yaml
name: "SQL DBA Agent"                 # user-facing agent name shown throughout the UI

ascii_art: |
  ____
 |  __|___ _ _  ___ ___
 | | | _| -_| | | .'|  _|
 |_| |___|___|___|__,|_|

model:                                # optional; omit to select a model at runtime via /model
  provider: anthropic                  # provider/model configured in ~/.keen-agent/configs.json
  model_id: claude-sonnet-4-20250514

system_prompt: |
  You are a PostgreSQL DBA. Help the user optimize queries,
  analyze execution plans, and manage database health.

system_prompt_files:
  - ./prompts/additional-context.md  # can contain additional context or the original system prompt itself

# Modes: plan (read-only tools only) | build (all tools)
default_mode: build

# Mode-specific system prompt tuning. These prompts are appended after the
# agent persona/project instructions/tool docs and after the active mode is known.
modes:
  build:
    system_prompt: |
      You are in build mode. Lean toward taking concrete action when the user asks.
  plan:
    system_prompt: |
      You are in plan mode. Do not modify files or system state.
      Use read-only tools to investigate and return concise plans, risks, and verification steps.

# Optional one-shot helper for quick side questions separate from the main task.
btw:
  enabled: true
  context_messages: 10
  system_prompt: |
    Answer quick side questions using recent conversation context.
    Be concise and do not use tools.

# Optional adversarial critic for reviewing the main agent's work/conversation.
adversary:
  enabled: true
  model:                              # optional; omitted means inherit main model
    provider: anthropic
    model_id: claude-sonnet-4-20250514
  system_prompt: |
    You are an adversarial critic. Find bugs, risks, security issues,
    faulty assumptions, and missing edge cases. Cite file:line when possible.

# Built-in tools (read_file, write_file, edit_file, web_fetch, glob, grep, bash)
# All enabled by default. Opt out here.
# call_mcp_tool is auto-included only when mcp_config_dirs is set.
# delegate_task is auto-included only when subagents_dirs is set.
builtin_tools:
  exclude:
    - write_file
    - edit_file
    - bash

# Subagents directories. Each directory contains Markdown files with YAML
# frontmatter (name, description) followed by the subagent's system prompt.
# Subagents are read-only assistants the main agent can delegate bounded tasks
# to via the `delegate_task` built-in tool.
subagents_dirs:
  - ./subagents

# MCP server configuration file paths (JSON). Optional; if omitted,
# no MCP tools are loaded.
mcp_config_dirs:
  - ./mcp-config.json

# Skills directories (agent-local)
skills_dirs:
  - ./skills
```

**Backward compatibility:** `subagents_dirs`, `mcp_config_dirs`, and `skills_dirs` each accept a single string or an array of strings. A single string is treated as a one-element array.

---

## MCP Configuration File

`mcp_config_dirs` is a list of JSON files containing MCP server definitions. If omitted, MCP support is disabled for this agent. Files are processed in order; later files can add servers or override earlier ones by name.

Format:

```json
{
  "servers": [
    {
      "name": "context7",
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp"]
    }
  ]
}
```

---

## Architecture

### Components (extracted/shared from keen-code)


| Component           | Source                          | Notes                                                                                       |
| ------------------- | ------------------------------- | ------------------------------------------------------------------------------------------- |
| LLM client          | keen-code `internal/llm`        | Genkit-based, multi-provider                                                                |
| Permission system   | keen-code `internal/filesystem` | Same guard: cwd=granted, outside=pending, system=denied                                     |
| TUI / REPL          | keen-code `internal/cli/repl`   | Customizable name                                                                           |
| Built-in tools      | keen-code `internal/tools`      | read_file, write_file, edit_file, web_fetch, glob, grep, bash, call_mcp_tool, delegate_task |
| Skill loader        | keen-code skill mechanism       | Agent-local (`skills_dirs`) + optional shared `~/.keen-agent/skills/`                       |
| MCP client          | keen-code MCP integration       | Same server config format; call_mcp_tool auto-included when mcp_config_dirs is set          |
| Subagent system     | keen-code `internal/subagents`  | Discovery, runner, and `delegate_task` tool; auto-included when subagents_dirs is set       |
| Session persistence | keen-code session storage       | Same format under `~/.keen-agent/<agent-name>/sessions/`; `/resume` command in TUI          |


### New components (keen-agent specific)


| Component              | Responsibility                                                                                             |
| ---------------------- | ---------------------------------------------------------------------------------------------------------- |
| Config parser          | Load + validate `agent.yaml`                                                                               |
| Config validator       | `keen-agent validate --agent ./agent.yaml`                                                                 |
| System prompt composer | Assemble prompt from config + tools + project instructions + skills + mode/helper prompt overlays          |
| Mode manager           | plan/build mode with read_only filtering and config-driven prompt tuning                                   |
| Helper agents          | Optional `btw` side-question helper (uses main model) and `adversary` critic with dedicated prompts/models |
| Subagent loader        | Discover and parse subagent profiles from `subagents_dirs`                                                 |
| Subagent runner        | Execute delegated tasks with a restricted tool registry (read_file, glob, grep only)                       |


---

## System Prompt Composition

The main-agent system prompt is assembled in order:

1. **Agent persona** — `system_prompt` field + `system_prompt_files` contents (array, appended in order)
2. **Tool documentation** — auto-generated from callable definitions (built-in tools + MCP tools)
3. **Subagent catalog** — list of available subagents with names and descriptions when `subagents_dirs` is set
4. **Skills catalog** — list of installed skills with descriptions and activation commands when `skills_dirs` is set
5. **Active skill** — skill body when activated via `/skill` or `[Activate skill: ...]`
6. **Mode instructions** — active mode marker plus built-in behavioral constraints
7. **Mode prompt overlay** — optional `modes.<active-mode>.system_prompt` or `system_prompt_files` (array)

Mode-specific prompt overlays are first-class config because `plan` and `build`
are behavioral modes, not just tool filters. This matches the current keen-agent
implementation where `internal/llm/systemprompt.go` appends different prompt
sections for `ModePlan` and `ModeBuild`, and `internal/cli/repl/appstate/state.go`
filters tools in plan mode.

Prompt overlay rules:

- `modes.plan` and `modes.build` may each define `system_prompt` and/or
`system_prompt_files`; file contents are appended after inline text in the order listed.
- Overlays are appended after the built-in mode constraints, so harness authors can
tune tone and workflow without weakening hard safety/tool constraints.
- The effective active mode is `--mode` if provided, otherwise `default_mode`.
- Plan mode still removes non-read-only tools before the LLM sees the registry;
prompt text is guidance, not the enforcement boundary.

---

## Tool Sources

At runtime, keen-agent presents one unified callable surface to the LLM, but the
configuration keeps sources separate:


| Source         | User-facing config | Purpose                                                                   |
| -------------- | ------------------ | ------------------------------------------------------------------------- |
| Built-in tools | `builtin_tools`    | Keen-native capabilities such as file reads, grep, edits, bash, web fetch |
| MCP tools      | `mcp_config_dirs`  | Scalable external/local integrations with discovery and protocol support  |
| Subagents      | `subagents_dirs`   | Focused read-only assistants for delegated investigation and analysis     |


Subagents are lightweight, read-only assistants defined as Markdown files. They
complement the main agent by handling scoped, separable investigation work.
The main agent decides when to call a subagent via the `delegate_task` built-in
tool and synthesizes the returned findings.

---

## Built-in Tools

Available by default:


| Tool          | read_only | Excludable | Permission                                                                  |
| ------------- | --------- | ---------- | --------------------------------------------------------------------------- |
| read_file     | true      | yes        | auto (cwd), pending (outside)                                               |
| write_file    | false     | yes        | auto (cwd), pending (outside)                                               |
| edit_file     | false     | yes        | auto (cwd), pending (outside)                                               |
| web_fetch     | true      | yes        | auto_approve                                                                |
| glob          | true      | yes        | auto_approve                                                                |
| grep          | true      | yes        | auto_approve                                                                |
| bash          | false     | yes        | `isDangerous` heuristic                                                     |
| call_mcp_tool | true      | no         | auto_approve for dispatch; MCP server/tool permissions apply where relevant |
| delegate_task | true      | no         | auto_approve                                                                |


All excludable built-ins can be disabled through `builtin_tools.exclude`.
`call_mcp_tool` is a core runtime tool and cannot be excluded; it is **auto-included
whenever `mcp_config_dirs` is set**, and omitted entirely when `mcp_config_dirs` is
absent. Users control MCP access by pointing the config files to the desired MCP
server definitions.

`delegate_task` follows the same pattern: it is **auto-included whenever
`subagents_dirs` is set**, and omitted entirely when `subagents_dirs` is absent.
Users control subagent availability by pointing the config to the desired
subagent definitions.

Filesystem guard applies identically to keen-code for filesystem tools.

### Bash permission model

bash uses the `**isDangerous` heuristic (model-reported, inherited from keen-code).**
The model flags a command as dangerous; flagged commands always prompt for approval.
This is the existing keen-code behavior and is preserved as-is.

---

## Modes


| Mode  | Behavior                                                              | Default prompt stance                                                                                        |
| ----- | --------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| plan  | Only read_only tools enabled. LLM asked to analyze/plan, not execute. | Do not modify files/system state; inspect with read-only tools; return plans, risks, and verification steps. |
| build | All tools enabled. LLM can take actions.                              | Lean toward concrete action when the user asks; verify changes.                                              |


Default mode is set in config (`default_mode`). User can switch via TUI command
or CLI `--mode` override.

### Mode config

```yaml
default_mode: build

modes:
  plan:
    system_prompt: |
      In plan mode, be skeptical about hidden implementation risk.
      Prefer numbered plans with assumptions and verification steps.
    system_prompt_files:
      - ./prompts/plan-mode.md
  build:
    system_prompt: |
      In build mode, make the smallest safe change and verify it.
    system_prompt_files:
      - ./prompts/build-mode.md
```

Rules:

- Valid modes are `plan` and `build`.
- `default_mode` defaults to `build` when omitted.
- `--mode plan|build` overrides `default_mode` for that process/session.
- TUI mode switches change the active prompt overlay on the next LLM turn.
- `modes.<mode>.system_prompt_files` entries are resolved relative to `agent.yaml`.
- Unknown mode config keys are validation errors.

### Implementation reference from current keen-agent

Current keen-agent already has the shape to generalize:


| Existing implementation                                                          | Generic keen-agent config equivalent                                     |
| -------------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| `llm.ModeBuild` / `llm.ModePlan` in `internal/llm/systemprompt.go`               | `default_mode` + CLI/TUI active mode                                     |
| `buildModePrompt` / `planModePrompt` constants in `internal/llm/systemprompt.go` | Built-in constraints plus `modes.<mode>.system_prompt` overlays          |
| `AppState.StreamChat` removing `write_file` and `edit_file` in plan mode         | Runtime read_only filtering for built-ins and MCP tools where applicable |
| `/mode plan                                                                      | build` and Shift+Tab in the TUI                                          |


---

## Helper Agents: btw and adversary

Current keen-agent includes two special LLM flows that should become configurable
instead of remaining coding-agent assumptions:


| Helper      | Current behavior                                                                                                                        | Generic config need                                                 |
| ----------- | --------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------- |
| `btw`       | One-shot side question using recent conversation context and no tools. Prompt comes from `BuildBtwPrompt`. Uses the main session model. | Optional helper with configurable prompt and context window.        |
| `adversary` | Separate critic model reviews the conversation and has its own prompt from `BuildAdversaryPrompt`.                                      | Optional critic with configurable prompt, model, and output stance. |


### `btw` config

```yaml
btw:
  enabled: true
  context_messages: 10
  model:                              # optional; omitted means inherit main model
    provider: openai
    model_id: gpt-5.4-mini
  system_prompt: |
    You answer quick side questions separate from the main task.
    Be concise and do not use tools.
  system_prompt_files:
    - ./prompts/btw.md
```

Rules:

- If omitted, `btw.enabled` defaults to `false` for generic agents.
- If enabled and `model` is omitted, it inherits the main resolved model/provider.
- `context_messages` bounds recent conversation context included in the one-shot
helper request.
- `btw` has no tool access by default; future tool access should be explicit.

### `adversary` config

```yaml
adversary:
  enabled: true
  model:                              # optional; omitted means inherit main model
    provider: anthropic
    model_id: claude-sonnet-4-20250514
  system_prompt: |
    You are an adversarial critic. Find problems in the main agent's output,
    code changes, assumptions, plans, and suggested verification. Lead with the
    most important issue. Cite file:line when possible.
  system_prompt_files:
    - ./prompts/adversary.md
```

Rules:

- If omitted, `adversary.enabled` defaults to `false` for generic agents.
- If enabled and `model` is omitted, it inherits the main resolved model/provider.
- The adversary gets conversation history transformed so main-agent assistant
messages are clearly attributed as main-agent output.
- The adversary runs one-shot and does not modify the main conversation unless the
user accepts/copies its output.

### Validation

- `btw.context_messages` must be positive when set.
- Helper `model` blocks use the same provider/model validation and resolution rules
as the main `model` block.
- Helper `system_prompt_files` entries must exist and are resolved relative to
`agent.yaml`.

---

## Skills

### Discovery order

1. **Agent-local**: `skills_dirs` from config (relative to config file location), processed in order
2. **Project-local**: `.agents/skills/` or `.keen-agent/skills/` in cwd
3. **Global**: `~/.keen-agent/skills/`

Earlier directories take precedence on name collision; later directories can extend the catalog with new skills.

### Format

Same as keen-code: directory with `SKILL.md` file. MCP-backed skills work identically.

---

## Subagents

Subagents are focused, read-only assistants that the main agent can delegate
bounded tasks to via the `delegate_task` built-in tool. They are useful for
scoped investigation, comparison, and summarization work that is separable from
the main agent's primary task.

### Discovery order

1. **Agent-local**: `subagents_dirs` from config (relative to config file location), processed in order
2. **Project-local**: `.agents/agents/` or `.keen-agent/agents/` in cwd
3. **Global**: `~/.keen-agent/agents/`

Earlier directories take precedence on name collision; later directories can extend the catalog with new subagents.

### Format

Each subagent is a single Markdown file with YAML frontmatter followed by the
subagent's system prompt (the body).

Example subagent file (`./subagents/api-reviewer.md`):

```markdown
---
name: api-reviewer
description: Reviews API-related code and docs for consistency, correctness, and missing edge cases.
---

You are an API review subagent.

Your role is to inspect API-related files using read-only tools and return concise findings to the parent agent.

Guidelines:
- Stay within the delegated task.
- Focus on paths provided by the parent agent first.
- Check routing, handlers, request/response types, validation, errors, and documentation when relevant.
- Return a short summary, relevant files, and key findings with `path:line` references.
- Do not edit files.
- Do not ask the user questions directly; report blockers to the parent agent.
```

### Frontmatter fields

Required fields:


| Field         | Description                                                        |
| ------------- | ------------------------------------------------------------------ |
| `name`        | Unique subagent name used by the main agent.                       |
| `description` | Short description shown to the main agent in the subagent catalog. |


Optional fields:


| Field             | Description                                                                                                 |
| ----------------- | ----------------------------------------------------------------------------------------------------------- |
| `tools`           | Restrict the read-only tools available to the subagent. Only `read_file`, `glob`, and `grep` are supported. |
| `timeout_seconds` | Runtime timeout for the subagent. If omitted, uses a default timeout.                                       |
| `hidden`          | If `true`, the subagent is loaded but not listed in the main agent's subagent catalog.                      |
| `provider`        | Reserved for model/provider override support.                                                               |
| `model`           | Reserved for model override support.                                                                        |
| `thinking_effort` | Reserved for model reasoning-effort override support.                                                       |


### Behavior

- Subagents are **read-only**: they can only use `read_file`, `glob`, and `grep`.
- They do not receive the full parent conversation history.
- They do not support skills or MCP tools.
- They do not spawn additional subagents.
- The `delegate_task` built-in tool is **auto-included** when `subagents_dirs` is set.
- The main agent's model and provider are inherited by subagents unless overridden.

### When to use subagents

Good for: scoped codebase investigation, tracing references, comparing
implementations, reviewing docs against a checklist, summarizing relevant
context before the main agent acts.

Not for: editing files, running shell commands, using skills, handling broad
vague tasks, or replacing the main agent's judgment.

---

## Agent State Layout

keen-agent separates user-authored resources from runtime state:


| Kind                                          | Ownership                     | Path                                                                         |
| --------------------------------------------- | ----------------------------- | ---------------------------------------------------------------------------- |
| Agent config                                  | user-authored                 | `--agent ./agent.yaml`                                                       |
| MCP server config                             | user-authored                 | `mcp_config_dirs` (optional)                                                 |
| Skills                                        | user-authored                 | `skills_dirs`, project-local skills, optional shared `~/.keen-agent/skills/` |
| Subagents                                     | user-authored                 | `subagents_dirs`, optional shared `~/.keen-agent/agents/`                    |
| Provider/model config + API credentials       | shared keen-agent state       | `~/.keen-agent/configs.json`                                                 |
| OAuth token cache for model providers and MCP | shared keen-agent state       | `~/.keen-agent/auth.json`                                                    |
| Sessions                                      | agent-scoped keen-agent state | `~/.keen-agent/<agent-name>/sessions/`                                       |
| Logs                                          | agent-scoped keen-agent state | `~/.keen-agent/<agent-name>/logs/`                                           |
| Input history                                 | agent-scoped keen-agent state | `~/.keen-agent/<agent-name>/input-history.jsonl`                             |


This keeps each user-built agent's sessions, logs, and input history independent,
while model/provider defaults and authentication are shared to avoid repeated setup.
Shared resources remain explicit: users can point multiple agents at the same
`mcp_config_dirs`, `skills_dirs`, or `subagents_dirs` entries if they want reuse.

## Session Persistence

- Same storage format as keen-code, stored under `~/.keen-agent/<agent-name>/sessions/`.
- Sessions tied to working directory + agent config path.
- Resume via `/resume` TUI command.
- No CLI flag for resume.

---

## TUI Customization

The config `name` is the user-facing agent identity. It is shown throughout the UI
instead of `keen-agent`; `keen-agent` is only the CLI binary used to start the
generic agent core with a selected config file.


| Field       | Effect                                                                                       |
| ----------- | -------------------------------------------------------------------------------------------- |
| `name`      | Shown in header, prompt, help text, session labels, logs, and other user-visible UI surfaces |
| `ascii_art` | Optional ASCII banner shown in the TUI header; ignored if empty. No colors or themes yet.    |


---

## Model Configuration

```yaml
model:                     # optional — omit the whole block to select a model at runtime via /model
  provider: anthropic      # provider/model configured in ~/.keen-agent/configs.json
  model_id: claude-sonnet-4-20250514   # anthropic | openai | google | ...
```

- `**model` is optional.** If omitted, the agent starts without a selected model; the user selects one at runtime with the `/model` command.
- When present, `model.provider` / `model.model_id` are validated against `~/.keen-agent/configs.json`; missing provider/model/credentials produce a warning but do not block startup.
- CLI flags (`--provider` / `--model`) override both the config block and any runtime selection.
- Resolution order: **CLI flags → `agent.yaml` `model` block → runtime `/model` selection.**
- Provider determines which API client is used; `model_id` is passed directly to the provider.
- Credential lookup is shared across agents:
  - API-key providers read credentials from `~/.keen-agent/configs.json`.
  - OAuth-backed model providers such as Codex read/write tokens in `~/.keen-agent/auth.json`.
  - MCP servers that authenticate with OAuth also read/write their credentials in `~/.keen-agent/auth.json`.

---

## CLI Interface

The binary is `keen-agent`. Agent config is passed with `--agent` so the same CLI shape works for both interactive TUI and headless runs without conflicting with keen-code's `keen` binary.

```bash
# Run an agent in the interactive TUI
keen-agent --agent ./agent.yaml

# Run with mode override
keen-agent --agent ./agent.yaml --mode plan

# Run headless
keen-agent run --agent ./agent.yaml --format json

# Run headless with provider/model overrides
keen-agent run --agent ./agent.yaml --provider anthropic --model claude-sonnet-4-20250514 --format json

# Validate config
keen-agent validate --agent ./agent.yaml
```

Notes:

- `--agent` is required.
- Config `model.provider` / `model.model_id` are **optional**; when absent, the user selects a model at runtime with `/model`. CLI flags override both the config block and the runtime selection.
- Headless mode keeps the existing `run` style and output `--format` behavior.

---

## Validation (`keen-agent validate`)

Validation is a single, ordered pass that separates fatal errors from warnings.
It runs on `keen-agent validate` and is also executed (with non-fatal warnings
allowed) during normal startup. The validator collects every applicable issue
before reporting so users see the full picture at once.

### Validation flow

1. **Structural parse (fatal)**
  - YAML must be well-formed.
  - Top-level keys must match the `agent.yaml` schema; unknown keys are errors.
  - Required fields `name` and (`system_prompt` or `system_prompt_files`) must be present.
2. **Scalar shape checks (fatal)**
  - `default_mode` must be `plan` or `build` when present.
  - `modes` may only contain `plan` and `build` keys.
  - `builtin_tools.exclude` entries must match allowed sets.
  - `model` block, when present, must have `provider` and `model_id`.
  - Helper `model` block (`adversary`) must have `provider` and `model_id` when present.
3. **File existence checks (fatal)**
  - Each `system_prompt_files` entry exists and is readable.
  - Each `mcp_config_dirs` entry exists and is readable.
  - Each `skills_dirs` directory exists and is readable.
  - Each `subagents_dirs` directory exists and is readable.
  - Each `modes.<mode>.system_prompt_files` entry exists when specified.
  - Helper `system_prompt_files` entries exist when specified.
4. **Content checks (fatal)**
  - Subagent `.md` files contain valid YAML frontmatter with required `name` and `description`.
5. **Cross-reference checks (fatal)**
  - Built-in tool names excluded in `builtin_tools.exclude` must be real, excludable tools.
  - `builtin_tools.exclude` must not list non-excludable core tools (`call_mcp_tool`, `delegate_task`).
  - Callable names must be unique across built-in tools and MCP tools.
  - Subagent names must be unique across discovered subagent profiles (first directory wins, later duplicates are errors).
  - Mode prompt overlays reference only valid modes.
6. **Runtime-readiness checks (warning only)**
  - If `model` is provided, warn when `~/.keen-agent/configs.json` is missing, the provider/model entry is missing, or required credentials are absent.
  - If `adversary` is enabled with a helper `model`, apply the same credential/model warnings.
  - If `mcp_config_dirs` is specified, warn when referenced MCP servers cannot be reached during validation (do not fail; servers may start later).
7. **Result**
  - Any fatal error → validation fails; `keen-agent validate` exits non-zero and the TUI refuses to start.
  - Only warnings → validation succeeds; warnings are printed once at startup and optionally via `/diagnostics`.

### Validation checklist

- YAML schema validity
- Required fields present (`name`, `system_prompt` or `system_prompt_files`)
- `ascii_art`, if present, is a string
- MCP config files exist (only if `mcp_config_dirs` is specified)
- `system_prompt_files` entries exist (if specified)
- `skills_dirs` entries exist (if specified)
- `subagents_dirs` entries exist (if specified); each `.md` file has valid YAML frontmatter with required `name` and `description` fields
- `default_mode` is `plan` or `build`; `modes` only contains `plan`/`build`, and each `system_prompt_files` entry exists if specified
- `btw` config is valid when enabled (`context_messages` positive if set, prompt file exists if specified)
- `adversary` config is valid when enabled (prompt file exists if specified, model resolves if specified)
- No duplicate callable names across built-in tools and MCP tools
- No duplicate subagent names across discovered subagent profiles
- `builtin_tools.exclude` does not include non-excludable core tools such as `call_mcp_tool` or `delegate_task`
- `model` is optional; when omitted the user can select one at runtime with the `/model` command
- If `model` is provided, Keen Agent checks `~/.keen-agent/configs.json`
  - If the file is missing, or the specified provider/model entry is missing, the agent still starts but prints a warning
  - If the resolved provider requires credentials and they are missing from `~/.keen-agent/configs.json` (API-key providers) or `~/.keen-agent/auth.json` (OAuth providers), the agent still starts but prints a warning
- MCP OAuth credentials, when needed, are stored in `~/.keen-agent/auth.json`

---

## Implementation Phases

### Phase 1 — Skeleton + Config

1. Initialize Go module (`github.com/<org>/keen-agent`)
2. Define config structs + YAML parsing, including mode prompt overlays, `ascii_art`, plus `btw` and `adversary` helper config
3. Implement config validation
4. Implement `keen-agent validate --agent ./agent.yaml` command

### Phase 2 — Core Runtime

1. Extract/copy LLM client from keen-code
2. Extract/copy permission system from keen-code
3. Implement system prompt composer with persona/project/tool/skill sections, built-in mode constraints, and config-driven mode/helper prompt overlays
4. Implement mode manager (plan/build + read_only filtering + prompt overlay selection)

### Phase 3 — Built-in Tools + MCP + Subagents

1. Extract/copy built-in tools (read_file, write_file, edit_file, web_fetch, glob, grep, bash, call_mcp_tool, delegate_task)
2. Extract/copy MCP client
3. Extract/copy subagent discovery, profile parser, and runner from keen-code
4. Wire tool registration (built-in via registry + MCP + subagents, with opt-out for excludable built-ins only; `call_mcp_tool` auto-included only when `mcp_config_dirs` is set; `delegate_task` auto-included only when `subagents_dirs` is set)

### Phase 4 — TUI + Skills + Subagents

1. Extract/copy TUI/REPL with customization hooks
2. Extract/copy skill loader with agent-local + global discovery
3. Extract/copy subagent loader with agent-local + global discovery
4. Implement configurable `btw` and `adversary` one-shot helper flows with dedicated prompts (adversary supports optional model override; btw uses main model)
5. Implement session persistence (same format as keen-code)

### Phase 5 — Polish + Ship

1. Implement headless mode (`keen-agent run --agent ... --format ...`)
2. Implement interactive full flow (`keen-agent --agent ...`: config → tools → prompt → loop)
3. Write README + example agent configs
4. Test critical paths (config parsing, mode filtering and mode prompt overlays, permission gating, headless approval path, subagent delegation + read-only tool restriction, `btw` prompt/context behavior, adversary prompt/model)

---

## Risks & Mitigations


| Risk                                                               | Mitigation                                                                                                                                                                                                      |
| ------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Extracting from keen-code creates drift                            | **Accepted by design** — keen-agent is a generic harness and owns its copied code; no shared module                                                                                                             |
| keen-agent and keen-code conflict on disk/env                      | Separate `~/.keen-agent/` namespace and `KEEN_AGENT_`* env prefix                                                                                                                                               |
| Multiple keen-agent builds leak conversation state into each other | Store sessions, logs, and input history under `~/.keen-agent/<agent-name>/`; keep model/provider defaults and auth shared in `~/.keen-agent/configs.json` and `~/.keen-agent/auth.json` to avoid repeated setup |
| Tool output blows up context                                       | Truncate oversized tool output at a sensible default                                                                                                                                                            |
| Users misconfigure tool sources silently                           | `keen-agent validate` catches issues before run                                                                                                                                                                 |
| MCP server failures hard to debug                                  | Surface MCP errors clearly in TUI                                                                                                                                                                               |
| Subagent tasks run too long or hang                                | Respect `timeout_seconds` per profile and overall context timeout; subagent output is bounded                                                                                                                   |


---

## Future (Post-v1)

- Config inheritance (`extends: ./base.yaml`)
- Agent registry/distribution
- HTTP tool type (direct API calls without shell)
- Auto-migration of config format if schema evolves

---

## Remaining Implementation TODO

The following items are derived from the current gap between this plan and the
implementation, plus a selective comparison with keen-code. Keen-agent is a
copy-fork and does **not** target feature parity: port reusable harness,
reliability, and security improvements; do not import keen-code's coding-agent
persona or namespace assumptions. Items are ordered by dependency and impact.

### Current implementation status

- [x] **Add `keen-agent validate --agent ./agent.yaml` and `--mode plan|build`.**
  - The command and flags exist. The validation and runtime semantics below remain
    incomplete and must be completed before considering these features done.

- [x] **Add agent-scoped `~/.keen-agent/<agent-name>/` state directories.**
  - Preserve the shared `~/.keen-agent/configs.json` and `auth.json` paths while
    keeping sessions, logs, and history agent-scoped.

- [ ] **Add config-driven MCP, skills, and subagent wiring.**
  - Config path parsing and initial runtime wiring exist. Config assignment now
    refreshes skills/subagents and headless runs retain the MCP runtime; complete
    the remaining discovery and integration acceptance work below before treating
    this as done.

### P0 — Make configured runtime resources usable

- [x] **Reload configured skills and subagents after assigning `AgentConfig`.**
  - `SetAgentConfig` now refreshes both catalogs, so configured directories are
    reflected when configuration is loaded after `AppState.New`.
  - Coverage proves configured skills and subagent profiles are discoverable.
  - Remaining: add end-to-end interactive/headless activation and delegation
    coverage as part of the integration test work.

- [x] **Pass the MCP runtime into headless runs.**
  - `repl.HeadlessRunOptions` now carries the initialized MCP runtime from
    `internal/cli/cmd/root.go`, allowing the headless registry to register
    `call_mcp_tool` when the agent enables MCP.
  - Coverage proves the configured MCP dispatch tool is registered. Add a real
    configured-server invocation test with the MCP integration suite.

- [ ] **Finish config-driven tool registration.**
  - In `internal/cli/repl/tooling/tool_registry.go`, honor
    `builtin_tools.exclude` for excludable tools.
  - Register `call_mcp_tool` only when `mcp_config_dirs` is non-empty and
    `delegate_task` only when `subagents_dirs` is non-empty; neither can be
    excluded explicitly.
  - Acceptance: excluded `bash` is absent from the LLM registry; an agent with
    neither integration directory has neither core integration tool.

- [ ] **Enforce plan mode through tool capabilities, not a name blacklist.**
  - [x] Add explicit read-only/capability metadata to the built-in tool contract
    in `internal/tools` and use it in the REPL registry/filtering path.
  - [x] In plan mode, expose only tools explicitly marked read-only. `bash`,
    mutating tools, delegated tasks, and opaque MCP calls are non-read-only by
    default.
  - [x] Apply `default_mode` at startup and retain `--mode` as the process override.
  - Remaining acceptance: exercise default plan mode through both startup paths
    with registered built-ins and an MCP runtime.

### P0 — Remove inherited coding-agent assumptions

- [x] **Make `btw` and `adversary` truly opt-in generic helpers.**
  - Omitted helper configuration defaults to disabled; disabled helpers are hidden
    from suggestions and help, rejected by handlers, and do not activate helper UI.
  - `btw.context_messages` controls the one-shot history window, and `/btw`
    always uses the main session model.
  - All adversary configuration access is nil-safe. `/adversary model` remains
    available to select and persist a per-user critic model, which takes
    precedence over an agent-configured adversary model for that user.
  - Acceptance: focused coverage verifies disabled helper rejection, hidden helper
    suggestions, configured `btw` context, and enabled helper command paths.

- [x] **Remove implicit coding-instruction discovery from prompt composition.**
  - Prompt composition no longer automatically loads `AGENTS.md`, `CLAUDE.md`, or
    `GEMINI.md`. Explicit `project_instructions` remains supported.
  - Coverage proves nearby coding-agent instruction files do not affect an
    unconfigured agent prompt.

### P1 — Complete the config contract and discovery model

- [ ] **Use strict YAML decoding and remove the unused `Functions` schema.**
  - Reject unknown top-level and nested configuration keys during decode rather
    than using permissive YAML unmarshalling.
  - Remove `Functions`, `FunctionDef`, `Duration`, and their validation unless a
    separately designed and implemented generic function-tool feature is approved.
  - Acceptance: unknown YAML fields fail validation; the obsolete field is absent
    from code, documentation, and tests.

- [ ] **Implement the documented generic discovery order.**
  - For skills: configured `skills_dirs`, then `.agents/skills/` or
    `.keen-agent/skills/` in the working directory, then `~/.keen-agent/skills/`.
  - For subagents: configured `subagents_dirs`, then `.agents/agents/` or
    `.keen-agent/agents/`, then `~/.keen-agent/agents/`.
  - Do not read keen-code's `.keen`, `~/.keen`, `.claude`, or related paths.
  - Resolve whether fallback discovery should enable delegation by itself; the
    preferred rule is: configured directories opt in to delegation, while fallback
    directories extend discovery after that opt-in.
  - Acceptance: directory precedence and collision behavior are covered by tests,
    and no keen-code namespace path is inspected.

- [ ] **Finish validation, diagnostics, and runtime-readiness warnings.**
  - Keep structural, scalar, file, content, and cross-reference failures fatal.
  - Add fatal duplicate checks for callable names and discovered subagent names.
  - Add non-fatal warnings for unresolved configured model/provider credentials
    and unavailable MCP servers. Print them once at startup and expose them via
    `/diagnostics` if that command is retained/added.
  - Apply equivalent model readiness checks to enabled helpers, including `btw`.
  - Acceptance: validation returns all applicable fatal errors and warnings;
    warning-only `keen-agent validate` exits zero.

### P1 — Selective keen-code reliability ports

- [ ] **Port parallel delegated-task execution and the longer default timeout.**
  - Adapt keen-code's bounded parallel delegation implementation: accept up to ten
    independent tasks, preserve input order in the result, and return per-task
    failures without dropping successful results.
  - Raise the default subagent timeout to 30 minutes while respecting profile and
    caller overrides, cancellation, and read-only restrictions.
  - Keep keen-agent's configured-directory opt-in; do not port keen-code's global
    discovery or coding-specific profile behavior.
  - Acceptance: tests cover concurrent success, one-task failure, cancellation,
    task-count limits, and timeout override behavior.

- [ ] **Port reusable output and continuation improvements selectively.**
  - Adapt richer tool status rendering: friendly names, safe input/result
    truncation, duration, and artifact/result metadata where available.
  - Evaluate retaining bounded, sanitized tool inputs in turn memory and deriving
    outcomes from results. Preserve existing secret-sensitive sanitization; do not
    store credentials or unbounded tool payloads.
  - Acceptance: output remains readable for generic tools, and memory tests prove
    secrets and oversized values are omitted/truncated.

- [ ] **Update dependencies and CI security checks from keen-code.**
  - Reconcile applicable Go/dependency updates rather than copying the entire
    dependency graph blindly.
  - Add reachable-vulnerability scanning (`govulncheck`) and CodeQL when this
    repository's hosting/privacy policy permits it; retain race tests, vet, and
    formatting checks.
  - Acceptance: CI fails on test, vet, formatting, or vulnerability-scan errors;
    dependency updates pass the full test suite.

### Documentation and release readiness

- [ ] **Write README and example agent configurations.**
  - Create a top-level `README.md` covering installation, agent configuration,
    CLI usage, validation, modes, permissions, skills, subagents, MCP, and the
    `~/.keen-agent` namespace.
  - Create `examples/` with minimal, plan/build, subagent, and MCP agents. Include
    example prompts/resources only where needed for validation.
  - Acceptance: every example has a valid `agent.yaml`; documented commands match
    the built binary's behavior.

- [ ] **Run release verification after all implementation items.**
  - Run `gofmt` on modified Go files, `go mod tidy`, `go vet ./...`,
    `go test -race ./...`, and `govulncheck ./...` when configured.
  - Verify interactive and headless startup with representative configs for: no
    integrations, skills/subagents, MCP, plan mode, and enabled helpers.
  - Acceptance: all commands complete without errors and the acceptance tests
    above pass.

### Explicit non-ports from keen-code

The following are intentionally out of scope for the generic v1 harness:

- Keen-code's hard-coded coding persona, automatic coding-instruction discovery,
  and bundled coding workflows such as commit/review/fix-tests.
- Keen-code's `.keen`/`.claude` resource discovery and its state/environment
  namespaces.
- Coding-specific roadmap work such as code-review modes and benchmark corpora.

### Future (Post-v1)

- [ ] **Consider opt-in persistent user/project memory.**
  - This is a generic capability worth evaluating after v1, but it introduces a
    mutable prompt source and is not required for the configured-agent contract.
  - If adopted, use `.keen-agent/` and `~/.keen-agent/` paths, make scope explicit,
    and reject secrets before persistence.

- [ ] Config inheritance (`extends: ./base.yaml`)
- [ ] Agent registry/distribution
- [ ] HTTP tool type (direct API calls without shell)
- [ ] Auto-migration of config format if schema evolves


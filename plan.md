# keen-agent — Implementation Plan

A generic, config-driven AI agent runner. Users provide system prompts, native
functions, MCP configuration, skills, and subagents — keen-agent handles the agent loop,
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

| keen-code | keen-agent |
|-----------|------------|
| `~/.keen/` (config, sessions, global skills) | `~/.keen-agent/` |
| `~/.keen/configs.json` (active provider/model) | `~/.keen-agent/configs.json` |
| `~/.keen/skills/` (global skills) | User-selected `skills_dirs` plus optional `~/.keen-agent/skills/` shared skills |
| `~/.keen/sessions/` (or equivalent) | `~/.keen-agent/<agent-name>/sessions/` |
| `~/.keen/logs/` (or equivalent) | `~/.keen-agent/<agent-name>/logs/` |
| auth/token storage | `~/.keen-agent/auth.json` |
| input history | `~/.keen-agent/<agent-name>/input-history.jsonl` |
| `KEEN_*` env vars | `KEEN_AGENT_*` env vars |

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

appearance:
  ascii_art: |
    ╔═══╗
    ║SQL║
    ╚═══╝
  colors:
    primary: "#4A90D9"              # brand/header/prompt/highlight color
    secondary: "#2ECC71"            # success/tool/border/action color
    accent: "#F5A623"               # warning/emphasis color
    danger: "#E74C3C"               # errors/destructive warnings
    muted: "#8A8A8A"                # subdued metadata/help text
    user_input_background:
      light: "#EEF3FA"
      dark: "#243040"

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

# User-defined native functions: small, explicit function-call extensions.
# For large/discoverable tool sets, use MCP via mcp_config_dirs instead.
functions:
  - name: run_query
    description: "Execute a read-only SQL query against the database"
    command: |
      python3 ./functions/run_query.py
    input_schema_file: ./schemas/run_query.input.json
    read_only: true
    permission: auto_approve  # or: requires_approval
    timeout: 30s
    max_retries: 2

  - name: analyze_customer_segments
    description: "Analyze a large customer segmentation request"
    command: python3 ./functions/analyze_segments.py
    input_schema_file: ./schemas/analyze_customer_segments.input.json
    read_only: true
    permission: auto_approve
    timeout: 60s

  - name: deploy
    description: "Deploy the current migration to staging"
    command: ./scripts/deploy.sh
    input_schema_file: ./schemas/deploy.input.json
    read_only: false
    permission: requires_approval
    timeout: 120s
    max_retries: 0

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

| Component | Source | Notes |
|-----------|--------|-------|
| LLM client | keen-code `internal/llm` | Genkit-based, multi-provider |
| Permission system | keen-code `internal/filesystem` | Same guard: cwd=granted, outside=pending, system=denied |
| TUI / REPL | keen-code `internal/cli/repl` | Customizable name, ASCII art, colors |
| Built-in tools | keen-code `internal/tools` | read_file, write_file, edit_file, web_fetch, glob, grep, bash, call_mcp_tool, delegate_task |
| Skill loader | keen-code skill mechanism | Agent-local (`skills_dirs`) + optional shared `~/.keen-agent/skills/` |
| MCP client | keen-code MCP integration | Same server config format; call_mcp_tool auto-included when mcp_config_dirs is set |
| Subagent system | keen-code `internal/subagents` | Discovery, runner, and `delegate_task` tool; auto-included when subagents_dirs is set |
| Session persistence | keen-code session storage | Same format under `~/.keen-agent/<agent-name>/sessions/`; `/resume` command in TUI |

### New components (keen-agent specific)

| Component | Responsibility |
|-----------|---------------|
| Config parser | Load + validate `agent.yaml` |
| Config validator | `keen-agent validate --agent ./agent.yaml` |
| Native function executor | Run user-defined function commands with schema-validated JSON input over stdin |
| System prompt composer | Assemble prompt from config + tools + project instructions + skills + mode/helper prompt overlays |
| Mode manager | plan/build mode with read_only filtering and config-driven prompt tuning |
| Helper agents | Optional `btw` side-question helper and `adversary` critic with dedicated prompts/models |
| Appearance engine | Apply custom name, ASCII art, color palette to TUI |
| Subagent loader | Discover and parse subagent profiles from `subagents_dirs` |
| Subagent runner | Execute delegated tasks with a restricted tool registry (read_file, glob, grep only) |

---

## System Prompt Composition

The main-agent system prompt is assembled in order:

1. **Agent persona** — `system_prompt` field + `system_prompt_files` contents (array, appended in order)
2. **Tool documentation** — auto-generated from callable definitions (built-in tools + user functions + MCP tools)
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

## Native Function Execution Model

`functions` are user-defined native function calls: small, explicit extensions to
an agent's callable surface. They are intended for simple local functions or
scripts that complement the built-in tools. They are **not** a discovery protocol
or integration framework; large tool families should be exposed through MCP.

### When to use functions vs MCP

| Use case | Prefer |
|----------|--------|
| A small number of explicit local commands | `functions` |
| One-off business logic around local files or internal scripts | `functions` |
| Enhancing built-in/native tools with a few custom capabilities | `functions` |
| Many tools with varied schemas | MCP |
| Dynamic discovery, shared clients, auth flows, lifecycle management | MCP |
| Local tools that already form an integration package | stdio MCP server |

MCP is already supported through `mcp_config_dirs` and is the right path when users
need MCP-like behavior. A local tool bundle can be wrapped as a stdio MCP server
and used by keen-agent without adding another discovery mechanism to `functions`.

### Input schema

Functions expose a JSON Schema object to the LLM. Function authors define that
contract with `input_schema_file`, a path to a JSON file containing the function's
input schema. The path is resolved relative to the `agent.yaml` directory unless
absolute.

Example schema reference:

```yaml
functions:
  - name: score_leads
    description: "Score a batch of leads"
    command: python3 ./functions/score_leads.py
    input_schema_file: ./schemas/score_leads.input.json
```

Example schema file (`./schemas/score_leads.input.json`):

```json
{
  "type": "object",
  "required": ["leads"],
  "properties": {
    "leads": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["id", "company", "events"],
        "properties": {
          "id": { "type": "string" },
          "company": { "type": "string" },
          "events": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "type": { "type": "string" },
                "timestamp": { "type": "string" }
              }
            }
          }
        }
      }
    }
  }
}
```

The loaded schema is passed directly to the LLM as the function input contract.
The LLM calls `score_leads` with a JSON object matching the schema; keen-agent
validates that object before executing the command.

### Argument passing

Function inputs are always passed as **JSON over stdin**:

```text
LLM function-call input JSON → validate against input_schema_file → command stdin
```

For the `score_leads` example, keen-agent runs `python3 ./functions/score_leads.py`
and writes the complete function-call input to stdin as JSON. This avoids OS
environment-size limits and is the correct path for large objects, arrays, or
nested structures.

Python function example:

```python
#!/usr/bin/env python3
import json
import sys

payload = json.load(sys.stdin)
leads = payload["leads"]

result = {"count": len(leads), "scores": []}
print(json.dumps(result))
```

Rules:
- `input_schema_file` must point to a `.json` file containing a JSON Schema object
  with root `type: object`.
- The LLM sees that schema as the function input contract and emits a JSON object
  that matches it.
- keen-agent validates the emitted JSON before spawning the command.
- Function process inherits the parent process environment (for secrets like
  `$DB_HOST`).
- Command runs in the agent's working directory.

### Execution

- Command is executed via `sh -c "<command>"` (supports multi-line, pipes, etc.)
- The validated JSON input is written to process stdin.
- stdout → returned to LLM as function result
- stderr → included in error reporting
- Non-zero exit → error fed back to LLM for retry (up to `max_retries`)

### Permission

Two levels:
- `auto_approve` — function executes without user confirmation
- `requires_approval` — TUI prompts user before execution (same UX as keen-code's dangerous bash commands)

### Mode filtering

- `read_only: true` → available in both plan and build modes
- `read_only: false` (default) → available only in build mode

---

## Native Function → `Tool` Adapter

This is the core mechanism that turns a YAML `functions` entry into a runtime
tool the LLM can call. The LLM APIs still use the generic term "tool", but the
user-facing config calls these entries `functions` to distinguish them from
built-in tools and MCP tools. Each function entry is wrapped in a single adapter
type, `functionTool`, that implements the existing `tools.Tool` interface
(`Name/Description/InputSchema/Execute`). One adapter instance per YAML entry;
the same type handles every user function — no per-function code generation.

### Type

```go
// functionTool adapts one YAML functions entry to the tools.Tool interface.
type functionTool struct {
    name            string
    description     string
    command         string         // raw shell command template (fixed by author)
    inputSchema     map[string]any // loaded JSON Schema object exposed to the LLM
    inputSchemaFile string         // source path, for validation/debug output
    readOnly        bool
    permission      Permission     // auto_approve | requires_approval
    timeout         time.Duration
    maxRetries      int

    runner          CommandRunner  // injected: real sh -c executor, or fake in tests
    approver        PermissionRequester
}
```

`CommandRunner` and `PermissionRequester` are interfaces so the adapter is unit
-testable without spawning real shells or a TUI.

### `Name()` / `Description()`

Return the configured `name` and `description` verbatim. `name` is validated at
config-load time (non-empty, unique across built-in tools + functions + MCP tool
names, matches `^[a-z0-9_]+$`).

### `InputSchema()` — loaded from config

The adapter returns the loaded JSON Schema object. Config loading reads and
validates the `input_schema_file` JSON file first. This is the contract the LLM
sees:

```go
func (t *functionTool) InputSchema() map[string]any {
    return cloneSchema(t.inputSchema)
}
```

- `input_schema_file` is required for every function.
- `input_schema_file` must point to a `.json` file containing a JSON Schema
  object at the root (`type: object`).
- Supported schema keywords include `type`, `description`, `properties`,
  `required`, `items`, `enum`, `default`, `minimum`, `maximum`, `minItems`,
  `maxItems`, and `additionalProperties`.
- Unsupported schema shapes are rejected at config-load, so runtime schema
  loading cannot fail.

### `Execute()` — JSON stdin + bounded run

```go
func (t *functionTool) Execute(ctx context.Context, input any) (any, error) {
    // 1. Decode input into map[string]any (LLM-supplied JSON object).
    // 2. Validate input against t.inputSchema.
    // 3. Permission gate: if requires_approval, call approver; abort on deny.
    // 4. Marshal validated input and write it to process stdin.
    // 5. Run via CommandRunner: sh -c command, with env, cwd, stdin, ctx+timeout.
    // 6. On non-zero exit, retry up to maxRetries (fresh ctx/timeout each try).
    // 7. Return stdout (+stderr on err).
}
```

Key rules already specified above are enforced here:
- JSON stdin handles large and nested inputs without environment-variable size
  limits.
- A missing required field or schema mismatch returns a tool error to the LLM
  (not a crash).
- `requires_approval` functions bypass execution entirely on user denial.

### Construction / registration

```go
func newFunctionTool(def config.FunctionDef, deps Deps) (*functionTool, error)
func registerFunctionTools(reg *tools.Registry, defs []config.FunctionDef, mode AgentMode, deps Deps) error
```

`registerFunctionTools` applies **mode filtering** before registration: in plan
mode, functions with `read_only: false` are skipped (never registered, so the LLM
never sees them). Name collisions with built-in tools or MCP tools are a
registration error surfaced at startup, not silently shadowed.

### Large input behavior

Large function inputs are not placed in environment variables. keen-agent streams
the validated JSON payload to the child process stdin. This means a Python
function-style command can receive arbitrarily nested objects without command-line
quoting problems or env-size limits:

```python
# functions/analyze_segments.py
import json
import sys


def analyze(request: dict) -> dict:
    return {"segment_count": len(request["metrics"])}


payload = json.load(sys.stdin)
print(json.dumps(analyze(payload["request"])))
```

The practical limits are controlled by provider constraints, not the OS env limit:

- Provider function/tool-call limits still apply: the LLM must be able to produce
  the JSON arguments in its function call.
- If payloads are too large for the model context, the user-defined function should
  accept file paths, IDs, or references instead of embedding the full object.

### Note on function execution isolation

Functions run via `sh -c` through `CommandRunner` and **do not** pass through
the `bash` built-in. The bash `isDangerous` heuristic therefore does **not**
apply to functions; their only gate is the per-function `permission` field.
Since function input is delivered over stdin, authors should read JSON from
stdin instead of interpolating model-provided values into shell command strings.

---

## Tool Sources

At runtime, keen-agent presents one unified callable surface to the LLM, but the
configuration keeps sources separate:

| Source | User-facing config | Purpose |
|--------|--------------------|---------|
| Built-in tools | `builtin_tools` | Keen-native capabilities such as file reads, grep, edits, bash, web fetch |
| User functions | `functions` | Small native function-call extensions implemented as local commands |
| MCP tools | `mcp_config_dirs` | Scalable external/local integrations with discovery and protocol support |
| Subagents | `subagents_dirs` | Focused read-only assistants for delegated investigation and analysis |

`functions` intentionally remain explicit: no discovery, no multi-function
catalogs, and no MCP-lite protocol. If users need many tools, varied schemas,
OAuth, shared integration clients, or dynamic discovery, they should expose those
capabilities as MCP servers and point `mcp_config_dirs` at the MCP config files.

Subagents are lightweight, read-only assistants defined as Markdown files. They
complement the main agent by handling scoped, separable investigation work.
The main agent decides when to call a subagent via the `delegate_task` built-in
tool and synthesizes the returned findings.

---

## Built-in Tools

Available by default:

| Tool | read_only | Excludable | Permission |
|------|-----------|------------|------------|
| read_file | true | yes | auto (cwd), pending (outside) |
| write_file | false | yes | auto (cwd), pending (outside) |
| edit_file | false | yes | auto (cwd), pending (outside) |
| web_fetch | true | yes | auto_approve |
| glob | true | yes | auto_approve |
| grep | true | yes | auto_approve |
| bash | false | yes | `isDangerous` heuristic |
| call_mcp_tool | true | no | auto_approve for dispatch; MCP server/tool permissions apply where relevant |
| delegate_task | true | no | auto_approve |

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

bash uses the **`isDangerous` heuristic (model-reported, inherited from keen-code).**
The model flags a command as dangerous; flagged commands always prompt for approval.
This is the existing keen-code behavior and is preserved as-is.

---

## Modes

| Mode | Behavior | Default prompt stance |
|------|----------|-----------------------|
| plan | Only read_only tools enabled. LLM asked to analyze/plan, not execute. | Do not modify files/system state; inspect with read-only tools; return plans, risks, and verification steps. |
| build | All tools enabled. LLM can take actions. | Lean toward concrete action when the user asks; verify changes. |

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

| Existing implementation | Generic keen-agent config equivalent |
|-------------------------|--------------------------------------|
| `llm.ModeBuild` / `llm.ModePlan` in `internal/llm/systemprompt.go` | `default_mode` + CLI/TUI active mode |
| `buildModePrompt` / `planModePrompt` constants in `internal/llm/systemprompt.go` | Built-in constraints plus `modes.<mode>.system_prompt` overlays |
| `AppState.StreamChat` removing `write_file` and `edit_file` in plan mode | Runtime read_only filtering for built-ins, functions, MCP tools where applicable |
| `/mode plan|build` and Shift+Tab in the TUI | Generic mode switch UI backed by config-defined prompt overlays |

---

## Helper Agents: btw and adversary

Current keen-agent includes two special LLM flows that should become configurable
instead of remaining coding-agent assumptions:

| Helper | Current behavior | Generic config need |
|--------|------------------|---------------------|
| `btw` | One-shot side question using recent conversation context and no tools. Prompt comes from `BuildBtwPrompt`. | Optional helper with configurable prompt, context window, and model inheritance/override. |
| `adversary` | Separate critic model reviews the conversation and has its own prompt from `BuildAdversaryPrompt`. | Optional critic with configurable prompt, model, and output stance. |

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

| Field | Description |
|---|---|
| `name` | Unique subagent name used by the main agent. |
| `description` | Short description shown to the main agent in the subagent catalog. |

Optional fields:

| Field | Description |
|---|---|
| `tools` | Restrict the read-only tools available to the subagent. Only `read_file`, `glob`, and `grep` are supported. |
| `timeout_seconds` | Runtime timeout for the subagent. If omitted, uses a default timeout. |
| `hidden` | If `true`, the subagent is loaded but not listed in the main agent's subagent catalog. |
| `provider` | Reserved for model/provider override support. |
| `model` | Reserved for model override support. |
| `thinking_effort` | Reserved for model reasoning-effort override support. |

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

| Kind | Ownership | Path |
|------|-----------|------|
| Agent config | user-authored | `--agent ./agent.yaml` |
| MCP server config | user-authored | `mcp_config_dirs` (optional) |
| Skills | user-authored | `skills_dirs`, project-local skills, optional shared `~/.keen-agent/skills/` |
| Subagents | user-authored | `subagents_dirs`, optional shared `~/.keen-agent/agents/` |
| Provider/model config + API credentials | shared keen-agent state | `~/.keen-agent/configs.json` |
| OAuth token cache for model providers and MCP | shared keen-agent state | `~/.keen-agent/auth.json` |
| Sessions | agent-scoped keen-agent state | `~/.keen-agent/<agent-name>/sessions/` |
| Logs | agent-scoped keen-agent state | `~/.keen-agent/<agent-name>/logs/` |
| Input history | agent-scoped keen-agent state | `~/.keen-agent/<agent-name>/input-history.jsonl` |

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

### Identity

| Field | Effect |
|-------|--------|
| `name` | Shown in header, prompt, help text, session labels, logs, and other user-visible UI surfaces |
| `appearance.ascii_art` | Displayed on startup |

### Theme colors

keen-agent should expose a small theme palette derived from keen-code's
`internal/cli/repl/theme/styles.go`. Users should be able to change colors that
define the agent's visual identity, but should not need to configure every style
or every repeated usage. The appearance engine maps these palette roles onto all
specific UI styles.

| Field | Intended usage |
|-------|----------------|
| `appearance.colors.primary` | Main identity color: title, prompt, model/mode chips, selected suggestions, primary highlights |
| `appearance.colors.secondary` | Support/action color: tool start/success, help commands, borders, model selection, context percentage |
| `appearance.colors.accent` | Warning/emphasis color: shell prompt/chip, update notice, compaction suggestions, non-critical warnings |
| `appearance.colors.danger` | Error/destructive color: errors, denied actions, critical context, destructive warnings |
| `appearance.colors.muted` | Subdued color: metadata, timestamps, hints, descriptions, command output summaries |
| `appearance.colors.user_input_background` | Optional adaptive background for rendered user input blocks |

Use adaptive color values where background-sensitive contrast matters:

```yaml
appearance:
  colors:
    primary: "#4A90D9"
    secondary: "#2ECC71"
    accent: "#F5A623"
    danger: "#E74C3C"
    muted: "#8A8A8A"
    user_input_background:
      light: "#EEF3FA"
      dark: "#243040"
```

Non-theme implementation colors remain derived defaults rather than config fields:

| Derived color | Reason |
|---------------|--------|
| `text_primary`, `text_secondary`, `text_dim` | General readability; should adapt automatically to terminal light/dark mode |
| `rule`, `white`, `black` | Structural/contrast helpers, not agent identity |
| `diff_add`, `diff_remove`, `diff_context`, `diff_hunk` | Semantic diff colors should stay stable and recognizable |
| loading shimmer variants | Animation detail derived from text/primary colors |

Falls back to the keen-agent default palette if not specified. Partial color config
is allowed; missing roles inherit defaults.

---

## Model Configuration

```yaml
model:                     # optional — omit the whole block to select a model at runtime via /model
  provider: anthropic      # provider/model configured in ~/.keen-agent/configs.json
  model_id: claude-sonnet-4-20250514   # anthropic | openai | google | ...
```

- **`model` is optional.** If omitted, the agent starts without a selected model; the user selects one at runtime with the `/model` command.
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

Checks: YAML schema validity
- Required fields present (name, system_prompt or system_prompt_files)
- Function definitions have name + description + command
- Each function defines `input_schema_file`; schema files exist, use `.json`, and contain valid supported JSON Schema objects
- MCP config files exist (only if mcp_config_dirs is specified)
- system_prompt_files entries exist (if specified)
- skills_dirs entries exist (if specified)
- subagents_dirs entries exist (if specified); each `.md` file has valid YAML frontmatter with required `name` and `description` fields
- `default_mode` is `plan` or `build`; `modes` only contains `plan`/`build`, and each `system_prompt_files` entry exists if specified
- `btw` config is valid when enabled (`context_messages` positive if set, prompt file exists if specified, model resolves if specified)
- `adversary` config is valid when enabled (prompt file exists if specified, model resolves if specified)
- No duplicate callable names across built-in tools, functions, and MCP tools
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
2. Define config structs + YAML parsing, including mode prompt overlays plus `btw` and `adversary` helper config
3. Implement config validation
4. Implement `keen-agent validate --agent ./agent.yaml` command

### Phase 2 — Core Runtime

5. Extract/copy LLM client from keen-code
6. Extract/copy permission system from keen-code
7. Implement system prompt composer with persona/project/tool/skill sections, built-in mode constraints, and config-driven mode/helper prompt overlays
8. Implement mode manager (plan/build + read_only filtering + prompt overlay selection)
9. Implement native function → `Tool` adapter:
   - `functionTool` type implementing `tools.Tool`
   - `InputSchema()` from loaded `input_schema_file`
   - `Execute()` with JSON-over-stdin input delivery and schema validation
   - `CommandRunner` + `PermissionRequester` interfaces (testable without real shell/TUI)
   - bounded execution: timeout, retries
   - `registerFunctionTools` with mode filtering + name-collision detection

### Phase 3 — Built-in Tools + MCP + Subagents

10. Extract/copy built-in tools (read_file, write_file, edit_file, web_fetch, glob, grep, bash, call_mcp_tool, delegate_task)
11. Extract/copy MCP client
12. Extract/copy subagent discovery, profile parser, and runner from keen-code
13. Wire tool registration (built-in via registry + functions via `registerFunctionTools` + MCP + subagents, with opt-out for excludable built-ins only; `call_mcp_tool` auto-included only when `mcp_config_dirs` is set; `delegate_task` auto-included only when `subagents_dirs` is set)

### Phase 4 — TUI + Skills + Subagents

14. Extract/copy TUI/REPL with customization hooks
15. Implement appearance engine (name, ASCII art, colors)
16. Extract/copy skill loader with agent-local + global discovery
17. Extract/copy subagent loader with agent-local + global discovery
18. Implement configurable `btw` and `adversary` one-shot helper flows with dedicated prompts and optional model overrides
19. Implement session persistence (same format as keen-code)

### Phase 5 — Polish + Ship

20. Implement headless mode (`keen-agent run --agent ... --format ...`)
21. Implement interactive full flow (`keen-agent --agent ...`: config → tools → prompt → loop)
22. Write README + example agent configs
23. Test critical paths (config parsing, native-function adapter: schema loading + JSON-stdin delivery + required-field validation + mode filtering and mode prompt overlays, permission gating, headless approval path, subagent delegation + read-only tool restriction, `btw` prompt/context behavior, adversary prompt/model)

---

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Extracting from keen-code creates drift | **Accepted by design** — keen-agent is a generic harness and owns its copied code; no shared module |
| keen-agent and keen-code conflict on disk/env | Separate `~/.keen-agent/` namespace and `KEEN_AGENT_*` env prefix |
| Multiple keen-agent builds leak conversation state into each other | Store sessions, logs, and input history under `~/.keen-agent/<agent-name>/`; keep model/provider defaults and auth shared in `~/.keen-agent/configs.json` and `~/.keen-agent/auth.json` to avoid repeated setup |
| Shell injection via function commands | Deliver model-provided inputs only as JSON over stdin; keep configured command strings static |
| Tool output blows up context | Truncate oversized tool output at a sensible default |
| Users misconfigure functions or tool sources silently | `keen-agent validate` catches issues before run |
| MCP server failures hard to debug | Surface MCP errors clearly in TUI |
| Subagent tasks run too long or hang | Respect `timeout_seconds` per profile and overall context timeout; subagent output is bounded |

---

## Future (Post-v1)

- Config inheritance (`extends: ./base.yaml`)
- Agent registry/distribution
- HTTP tool type (direct API calls without shell)
- Auto-migration of config format if schema evolves

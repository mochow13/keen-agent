# Keen Agent

Keen Agent is a configurable, terminal-first AI agent harness for any kind of agentic work. Define an agent's instructions and capabilities in YAML, select an LLM provider, then work interactively or automate a single turn from the command line.

It is designed for people who want more control than a fixed assistant offers—without having to build an agent runtime from scratch. Use it for software engineering, research, operations, writing, analysis, or a workflow specific to you or your team.

## Table of contents

- [Why Keen Agent?](#why-keen-agent)
- [What you can build](#what-you-can-build)
- [Configuring an agent](#configuring-an-agent)
  - [Complete configuration reference](#complete-configuration-reference)
  - [Modes and tool policy](#modes-and-tool-policy)
- [Quickstart](#quickstart)
- [Common commands](#common-commands)
- [Skills, subagents, and MCP](#skills-subagents-and-mcp)
- [Features](#features)
- [Built on Keen Code](#built-on-keen-code)
- [Technology](#technology)
- [Safety model](#safety-model)
- [Development](#development)
- [License](#license)

## Why Keen Agent?

Agentic work is more reliable when the agent has a clear role, task-specific instructions, useful tools, and explicit safety boundaries. Keen Agent brings those pieces together:

- **Configurable agents** — package a persona, system prompt, model preference, modes, tool policy, and optional skills, subagents, or MCP servers in an `agent.yaml` file.
- **Terminal-native workflows** — use an interactive REPL for ongoing work or `run` for scripts, scheduled jobs, and other one-shot tasks.
- **Provider flexibility** — switch providers or models without changing your agent definition.
- **Practical context management** — persist sessions, resume prior work, and compact long conversations.
- **Opt-in extensibility** — combine built-in terminal and filesystem tools with reusable skills, specialist subagents, and Model Context Protocol (MCP) servers when you configure their locations.
- **Safety by default** — filesystem access is guarded, ignored and sensitive paths are blocked, and potentially unsafe operations require approval.

## What you can build

Keen Agent is intentionally domain-neutral. Its behavior comes from the prompts and tools you configure; skills, subagents, and MCP servers are optional additions loaded only from locations declared in `agent.yaml`. For example, you can create agents for:

- Software engineering and repository maintenance
- Research, analysis, and report generation
- Operational runbooks, incident investigation, and terminal workflows
- Documentation, content, and data-processing tasks
- Internal workflows that need a persistent, permission-aware terminal agent

## Configuring an agent

Keen Agent requires an agent configuration. The configuration is strict YAML: unknown fields and invalid combinations are rejected by `keen-agent validate`. `name` and at least one of `system_prompt` or `system_prompt_files` are required.

Create an `agent.yaml` in the directory where you want to work. This complete example shows every supported field; remove the options you do not need.

```yaml
name: Operations Assistant
ascii_art: |
  OPERATIONS ASSISTANT

# Optional. Both provider and model_id must be set when model is present.
model:
  provider: anthropic
  model_id: claude-sonnet-4-6

# At least one of system_prompt or system_prompt_files is required.
system_prompt: |
  You are a pragmatic operations assistant.
  Investigate available evidence before recommending an action.
system_prompt_files:
  - prompts/base.md
project_instructions: instructions.md

default_mode: build
modes:
  plan:
    system_prompt: |
      Analyze and propose a plan. Do not modify files.
    system_prompt_files:
      - prompts/plan.md
  build:
    system_prompt: |
      Carry out the approved action and report the result.
    system_prompt_files:
      - prompts/build.md

# Exclude selected built-in tools.
builtin_tools:
  exclude:
    - web_fetch

# Optional helpers.
btw:
  enabled: true
  context_messages: 5
  system_prompt: Answer the side question directly and concisely.
  system_prompt_files:
    - prompts/btw.md

adversary:
  enabled: true
  model:
    provider: anthropic
    model_id: claude-sonnet-4-6
  system_prompt: Review the proposed work for risks and omissions.
  system_prompt_files:
    - prompts/adversary.md

# All extension directories are opt-in; each accepts one path or a list.
skills_dirs:
  - .keen/skills
subagents_dirs:
  - .keen/subagents
mcp_config_dirs:
  - .keen/mcp
```

All file and directory paths in the configuration are resolved relative to `agent.yaml`; absolute paths remain absolute. `system_prompt_files`, `skills_dirs`, `subagents_dirs`, and `mcp_config_dirs` accept either a single string or a list of strings.

Every system-prompt configuration—the primary agent, each mode, `btw`, and `adversary`—supports `system_prompt`, `system_prompt_files`, or both. For the primary agent, Keen Agent always prepends its harness contract (identity, tool-grounding rules, tool-memory semantics, and safety rules), then appends the inline `system_prompt`, followed by the referenced `system_prompt_files` in order. The built-in style persona (tone, task workflow, tool-usage guidance) is included only when the configuration provides no prompt source of its own. Mode overlays and helper prompts are composed separately.

> **Prompt source required:** the primary agent configuration must define at least one of `system_prompt` or `system_prompt_files`. Either field is sufficient and both may be used together, but they cannot both be missing. Mode overlays and helper prompts are optional; when omitted, they use their normal empty-overlay or built-in fallback behavior.

### Complete configuration reference

| Field | Required | Description |
| --- | --- | --- |
| `name` | Yes | Display name for the agent. It also contributes to the session namespace. |
| `ascii_art` | No | Banner displayed for the agent in the terminal UI. |
| `model.provider` | With `model` | Configured provider identifier to select for this agent. |
| `model.model_id` | With `model` | Model identifier for `model.provider`. `model` must include both fields. |
| `system_prompt` | One prompt source required | Inline base instructions for the primary agent. It is composed with prompt files when both are present. |
| `system_prompt_files` | One prompt source required | One or more files containing base instructions. Each referenced file must exist. |
| `project_instructions` | No | A workspace- or task-specific instruction file. |
| `default_mode` | No | Starting mode: `build` (the default) or `plan`. |
| `modes.plan.system_prompt` | No | Inline instructions added when plan mode is active. |
| `modes.plan.system_prompt_files` | No | One or more instruction files added when plan mode is active. |
| `modes.build.system_prompt` | No | Inline instructions added when build mode is active. |
| `modes.build.system_prompt_files` | No | One or more instruction files added when build mode is active. |
| `builtin_tools.exclude` | No | Built-in tools to remove: `read_file`, `write_file`, `edit_file`, `web_fetch`, `glob`, `grep`, or `bash`. This is the only supported `builtin_tools` setting. |
| `btw.enabled` | No | Enables the `/btw` side-question helper. |
| `btw.context_messages` | When `btw.enabled` is `true` | Positive number of prior messages supplied as helper context. |
| `btw.system_prompt` | No | Inline instructions for the side-question helper. |
| `btw.system_prompt_files` | No | One or more instruction files for the side-question helper. |
| `adversary.enabled` | No | Enables the `/adversary` review helper. |
| `adversary.model.provider` | With `adversary.model` | Provider identifier for adversarial review. |
| `adversary.model.model_id` | With `adversary.model` | Model identifier for adversarial review. Both adversary model fields are required together. |
| `adversary.system_prompt` | No | Inline instructions for adversarial review. |
| `adversary.system_prompt_files` | No | One or more instruction files for adversarial review. |
| `skills_dirs` | No | One or more directories from which to load skills. Nothing is loaded unless configured. |
| `subagents_dirs` | No | One or more directories from which to load subagent profiles. Nothing is loaded unless configured. |
| `mcp_config_dirs` | No | One or more locations containing MCP configuration. No MCP configurations are loaded unless configured. |

Use the validation command after changing configuration:

```bash
keen-agent validate --agent ./agent.yaml
```

### Modes and tool policy

- **`build`** is the default mode and makes the configured built-in tools available.
- **`plan`** makes only read-only built-in tools available, so the agent can inspect and reason without modifying the workspace.
- A mode can be selected with `--mode` or switched during an interactive session with `/mode`.
- `call_mcp_tool` and `delegate_task` cannot be excluded. They are only useful when MCP servers or subagents have been configured.
- `builtin_tools` only supports excluding tools; it does not configure bash permissions.

## Quickstart

### Prerequisites

- Go **1.25.7 or newer** to build from source.
- Credentials for at least one supported LLM provider, or AWS credentials for Amazon Bedrock.

### Install

> A one-line `curl` installation will be released soon.

For now, build a local binary from a clone:

```bash
git clone https://github.com/mochow13/keen-agent.git
cd keen-agent
go build -o keen-agent ./cmd
```

### Start Keen Agent

After creating and validating an `agent.yaml` as described above:

```bash
keen-agent --agent ./agent.yaml
```

On first launch, use `/model` to configure a provider, its credentials, and a model. Keen Agent stores this configuration in `~/.keen-agent/configs.json` with owner-only permissions. You can also use `/thinking` to choose the supported thinking effort for the active model.

Ask for work in the REPL, for example:

```text
> Compare these options, identify the trade-offs, and recommend an approach.
```

Use `/help` at any time for the available interactive commands.

## Common commands

| Command | Purpose |
| --- | --- |
| `keen-agent --agent ./agent.yaml` | Start an interactive session. |
| `keen-agent --agent ./agent.yaml --mode plan` | Start in read-only planning mode. |
| `keen-agent --resume <session-id> --agent ./agent.yaml` | Resume a saved session. |
| `keen-agent run --agent ./agent.yaml "Research this topic"` | Run one non-interactive turn. |
| `keen-agent run --agent ./agent.yaml --format json "Summarize these findings"` | Produce machine-readable one-shot output. |
| `printf 'Analyze this input' \| keen-agent run --agent ./agent.yaml` | Send a prompt through standard input. |
| `keen-agent validate --agent ./agent.yaml` | Validate an agent definition before use. |

A run command can temporarily select a configured provider and model:

```bash
keen-agent run \
  --agent ./agent.yaml \
  --provider anthropic \
  --model claude-sonnet-4-6 \
  "Explain the authentication flow"
```

## Skills, subagents, and MCP

### Skills

Skills are reusable instruction bundles. Keen Agent does not load a skills directory by default: declare one or more `skills_dirs` in `agent.yaml`, then put a `SKILL.md` in those directories and manage them interactively with:

```text
/skills list
/skills status
/skills disable <name>
/skills enable <name>
/skills reload
```

### Subagents

Subagents are focused, read-only assistants defined as Markdown files. Keen Agent does not load a subagent directory by default: declare one or more `subagents_dirs` in `agent.yaml`. Each file requires YAML frontmatter:

```markdown
---
name: explorer
description: Investigate a scoped part of the workspace and report findings.
---

Inspect only the requested files. Return concise findings with file references.
```

List available profiles with `/subagents list`. The primary agent can delegate bounded investigations to them.

### MCP servers

MCP servers are opt-in. Keen Agent does not load MCP configuration directories by default: declare one or more `mcp_config_dirs` in `agent.yaml`. Configuration files found in those directories are merged, with later values taking precedence.

Example local stdio server configuration:

```json
{
  "servers": {
    "example": {
      "command": "npx",
      "args": ["-y", "@example/mcp-server"],
      "env": {
        "EXAMPLE_MODE": "read-only"
      }
    }
  }
}
```

Example remote streamable HTTP server:

```json
{
  "servers": {
    "remote-tools": {
      "url": "https://example.com/mcp",
      "auth": {
        "type": "api_key",
        "key": "configure-this-outside-source-control"
      }
    }
  }
}
```

Use `/mcp`, `/mcp status`, and `/mcp connect` in the REPL to inspect and connect servers. Do not commit credential-bearing MCP configuration files.

## Features

- Interactive terminal UI with streaming Markdown output, model selection, command suggestions, and session picker.
- `plan` and `build` modes. Plan mode exposes only read-only built-in tools; build mode enables the configured tool set.
- Built-in tools for reading, writing, and editing files; globbing and searching; fetching web content; running shell commands; delegating work; and calling MCP tools.
- Built-in permission prompts for filesystem and shell operations.
- Persistent, project-namespaced sessions with `--resume`, `/resume`, and `/sessions`.
- Conversation compaction and optional thinking-token display.
- Optional side-question (`/btw`) and adversarial-review (`/adversary`) helpers.
- Skills and subagents loaded from directories specified by each agent configuration.
- MCP support for local stdio and remote streamable HTTP servers, including API-key and OAuth authentication.
- One-shot plain-text or JSON output for automation.

## Built on Keen Code

Keen Agent is based on [Keen Code](https://github.com/mochow13/keen-code), the terminal-agent harness that provides its interactive runtime, tool loop, and safety foundations. Learn more at the [Keen Code website](https://mochow13.github.io/keen-code/).

Keen Code contributes several harness capabilities that make Keen Agent suitable for configurable agents beyond its coding origins:

- **Permission-aware execution** — a central filesystem guard distinguishes allowed, approval-required, and policy-blocked paths; Git-ignored and sensitive paths stay protected.
- **Lean, durable context** — sessions are namespaced by working directory, while completed turns retain compact tool-activity metadata rather than raw file contents, command output, or external responses.
- **Prompt-efficient extensibility** — configured MCP servers become generated skills: the agent discovers a server's tools and schemas on demand instead of carrying every external tool schema in its system prompt.
- **Controlled delegation** — focused subagents can investigate with explicitly limited, read-only tools, leaving decisions and side effects with the primary agent.
- **Provider-native tool loops** — the harness streams provider responses and tool calls while adapting a shared tool registry to each supported model provider.

## Technology

Keen Agent is written in **Go** and uses:

- Firebase Genkit for Google AI model integration.
- Bubble Tea, Bubbles, Lip Gloss, Glamour, and Ultraviolet for the terminal experience.
- Cobra for the CLI.
- Official SDKs for Anthropic, OpenAI, AWS Bedrock, and MCP.
- `go-git` for Git-aware filesystem safety.

Supported provider integrations include Anthropic, OpenAI, ChatGPT OAuth (Codex), Google AI, Amazon Bedrock, DeepSeek, Moonshot AI, Z.ai, MiniMax, OpenCode Go, and OpenAI-compatible endpoints.

## Safety model

Keen Agent is designed to make agent actions visible and controllable:

- Reads inside the working directory are allowed; reads elsewhere require approval.
- Writes and edits require approval.
- System locations such as `/etc`, `/usr`, `/bin`, `/proc`, and `/dev` are blocked.
- Git-ignored paths and hidden directories under the home directory are blocked to prevent accidental secret access.
- Dangerous shell actions are surfaced for approval. `/allow-permission` can override this behavior, so use it deliberately.

Always review proposed changes and permission requests before approving them.

## Development

```bash
git clone https://github.com/mochow13/keen-agent.git
cd keen-agent
go mod tidy
go test -race ./...
go build ./cmd
```

The main packages are:

| Path | Responsibility |
| --- | --- |
| `cmd` | Application entry point. |
| `internal/cli` | Cobra commands and the interactive REPL. |
| `internal/llm` | Provider clients, streaming, prompts, tool execution, and context reduction. |
| `internal/tools` | Built-in LLM tools and their permission-aware execution. |
| `internal/filesystem` | Filesystem and Git-aware safety guard. |
| `internal/mcp` | MCP configuration, connection lifecycle, and authentication. |
| `internal/skills` | Skill discovery and enablement. |
| `internal/subagents` | Subagent discovery and execution. |
| `internal/session` | Persistent session transcripts and replay. |

## License

Keen Agent is released under the [MIT License](LICENSE).

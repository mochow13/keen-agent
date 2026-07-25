# Keen Agent

Keen Agent is a configurable, terminal-first AI agent harness for software work. It gives teams and individuals a focused way to run purpose-built coding agents: define the agent's instructions and capabilities in YAML, select an LLM provider, then work interactively or automate a single turn from the command line.

It is designed for people who want more control than a fixed assistant offers—without having to build an agent runtime from scratch.

## Why Keen Agent?

AI coding work is more reliable when the agent has a clear role, project-specific instructions, useful tools, and explicit safety boundaries. Keen Agent brings those pieces together:

- **Configurable agents** — package a persona, system prompt, model preference, modes, skills, subagents, and tool policy in an `agent.yaml` file.
- **Terminal-native workflows** — use an interactive REPL for ongoing work or `run` for scripts and CI-style one-shot tasks.
- **Provider flexibility** — switch providers or models without changing your agent definition.
- **Practical context management** — persist sessions, resume prior work, and compact long conversations.
- **Extensible capabilities** — add reusable skills, specialist subagents, and Model Context Protocol (MCP) servers.
- **Safety by default** — filesystem access is guarded, ignored and sensitive paths are blocked, and potentially unsafe operations require approval.

## Features

- Interactive terminal UI with streaming Markdown output, model selection, command suggestions, and session picker.
- `plan` and `build` modes. Plan mode exposes only read-only built-in tools; build mode enables the configured tool set.
- Built-in tools for reading, writing, and editing files; globbing and searching; fetching web content; running shell commands; delegating work; and calling MCP tools.
- Built-in permission prompts, configurable bash policies, and per-tool permission overrides.
- Persistent, project-namespaced sessions with `--resume`, `/resume`, and `/sessions`.
- Conversation compaction and optional thinking-token display.
- Optional side-question (`/btw`) and adversarial-review (`/adversary`) helpers.
- Skills and subagents loaded from directories specified by each agent configuration.
- MCP support for local stdio and remote streamable HTTP servers, including API-key and OAuth authentication.
- One-shot plain-text or JSON output for automation.

## Built on

Keen Agent is written in **Go** and uses:

- [Firebase Genkit](https://firebase.google.com/docs/genkit) for Google AI model integration.
- [Bubble Tea](https://github.com/charmbracelet/bubbletea), Bubbles, Lip Gloss, Glamour, and Ultraviolet for the terminal experience.
- [Cobra](https://github.com/spf13/cobra) for the CLI.
- Official SDKs for Anthropic, OpenAI, AWS Bedrock, Google Gen AI, and MCP.
- `go-git` for Git-aware filesystem safety.

Supported provider integrations include Anthropic, OpenAI, ChatGPT OAuth (Codex), Google AI, Amazon Bedrock, DeepSeek, Moonshot AI, Z.ai, MiniMax, OpenCode Go, and OpenAI-compatible endpoints.

## Quickstart

### Prerequisites

- [Go](https://go.dev/dl/) **1.25.7 or newer** to build from source.
- Credentials for at least one supported LLM provider, or AWS credentials for Amazon Bedrock.

### Install

Install the current source directly:

```bash
go install github.com/mochow13/keen-agent/cmd@latest
```

Or build a local binary from a clone:

```bash
git clone https://github.com/mochow13/keen-agent.git
cd keen-agent
go build -o keen-agent ./cmd
```

### Create an agent definition

Keen Agent requires an agent configuration. Create an `agent.yaml` in the directory where you want to work:

```yaml
name: Project Assistant
system_prompt: |
  You are a careful software engineering assistant.
  Inspect the repository before making changes, keep edits focused,
  and run relevant tests after implementation.
project_instructions: AGENTS.md

default_mode: build

# Optional: pin this agent to a configured provider and model.
# model:
#   provider: anthropic
#   model_id: claude-sonnet-4-6
```

`project_instructions` is optional. When set, it is resolved relative to `agent.yaml`; use it to load repository-specific guidance such as `AGENTS.md`.

### Start Keen Agent

```bash
keen-agent --agent ./agent.yaml
```

On first launch, use `/model` to configure a provider, its credentials, and a model. Keen Agent stores this configuration in `~/.keen-agent/configs.json` with owner-only permissions. You can also use `/thinking` to choose the supported thinking effort for the active model.

Ask for work in the REPL, for example:

```text
> Find the failing tests and make the smallest correct fix.
```

Use `/help` at any time for the available interactive commands.

## Common commands

| Command | Purpose |
| --- | --- |
| `keen-agent --agent ./agent.yaml` | Start an interactive session. |
| `keen-agent --agent ./agent.yaml --mode plan` | Start in read-only planning mode. |
| `keen-agent --resume <session-id> --agent ./agent.yaml` | Resume a saved session. |
| `keen-agent run --agent ./agent.yaml "Review this repository"` | Run one non-interactive turn. |
| `keen-agent run --agent ./agent.yaml --format json "Summarize changes"` | Produce machine-readable one-shot output. |
| `printf 'Review this diff' \| keen-agent run --agent ./agent.yaml` | Send a prompt through standard input. |
| `keen-agent validate --agent ./agent.yaml` | Validate an agent definition before use. |

A run command can temporarily select a configured provider and model:

```bash
keen-agent run \
  --agent ./agent.yaml \
  --provider anthropic \
  --model claude-sonnet-4-6 \
  "Explain the authentication flow"
```

## Configuring an agent

An agent definition is strict YAML: unknown fields and invalid combinations are rejected by `keen-agent validate`.

```yaml
name: Project Assistant
ascii_art: |
  PROJECT ASSISTANT

model:
  provider: anthropic
  model_id: claude-sonnet-4-6

system_prompt: |
  You are a pragmatic coding assistant.
system_prompt_files:
  - prompts/engineering.md
project_instructions: AGENTS.md

default_mode: build
modes:
  plan:
    system_prompt: |
      Analyze and propose a plan. Do not modify files.
  build:
    system_prompt: |
      Implement the approved change and verify it.

builtin_tools:
  exclude:
    - web_fetch
  bash:
    permission: requires_approval
    rules:
      - match: ["go test *", "go vet *"]
        permission: auto_approve

skills_dirs: .keen/skills
subagents_dirs: .keen/subagents
mcp_config_dirs: .keen/mcp

btw:
  enabled: true
  context_messages: 5

adversary:
  enabled: true
  model:
    provider: anthropic
    model_id: claude-sonnet-4-6
```

Paths in `system_prompt_files`, `project_instructions`, `skills_dirs`, `subagents_dirs`, and `mcp_config_dirs` are resolved relative to `agent.yaml`.

### Modes and tool policy

- **`build`** is the default mode and makes the configured built-in tools available.
- **`plan`** makes only read-only built-in tools available, so the agent can inspect and reason without modifying the project.
- A mode can be selected with `--mode` or switched during an interactive session with `/mode`.
- You can exclude `read_file`, `write_file`, `edit_file`, `web_fetch`, `glob`, `grep`, or `bash`. MCP and subagent delegation remain available.
- Bash policy defaults and match-based rules support `auto_approve`, `requires_approval`, and `deny`.

## Skills, subagents, and MCP

### Skills

Skills are reusable instruction bundles. Put a `SKILL.md` in a configured skills directory and manage them interactively with:

```text
/skills list
/skills status
/skills disable <name>
/skills enable <name>
/skills reload
```

### Subagents

Subagents are focused, read-only assistants defined as Markdown files in a configured subagent directory. Each file requires YAML frontmatter:

```markdown
---
name: explorer
description: Investigate a scoped part of the codebase and report findings.
---

Inspect only the requested files. Return concise findings with file references.
```

List available profiles with `/subagents list`. The primary agent can delegate bounded investigations to them.

### MCP servers

By default, Keen Agent loads `~/.keen-agent/mcp/configs.json`. Add agent-specific directories with `mcp_config_dirs`; configuration files found there are merged, with later values taking precedence.

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

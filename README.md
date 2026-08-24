# Keen Agent

**Build and run any agent in your terminal**

[![Latest Release](https://img.shields.io/github/v/release/mochow13/keen-agent?style=flat-square&logo=github)](https://github.com/mochow13/keen-agent/releases/latest)
[![Build Status](https://img.shields.io/github/actions/workflow/status/mochow13/keen-agent/go.yml?branch=main&style=flat-square&logo=githubactions&logoColor=white)](https://github.com/mochow13/keen-agent/actions)
[![Go Version](https://img.shields.io/github/go-mod/go-version/mochow13/keen-agent?style=flat-square&logo=go)](https://go.dev/)
[![License](https://img.shields.io/github/license/mochow13/keen-agent?style=flat-square&logo=opensourceinitiative&logoColor=white)](https://github.com/mochow13/keen-agent/blob/main/LICENSE)

Keen Agent is a configurable, terminal-first AI agent harness for any kind of agentic work. Define an agent's instructions and capabilities in YAML, select an LLM provider, then work interactively or automate a single turn from the command line.

It is designed for people who want more control than a fixed assistant offers—without having to build an agent runtime from scratch. Use it for software engineering, research, operations, writing, analysis, or a workflow specific to you or your team.

Keen Agent grew out of [Keen Code](https://github.com/mochow13/keen-code), an opinionated terminal coding agent. While building Keen Code, it became clear that the same core capabilities—filesystem access, tool execution, permission controls, persistent sessions, model integrations, skills, subagents, and MCP—were useful far beyond software development. Keen Agent was extracted and fleshed out from that foundation to make the runtime domain-neutral and fully configurable: instead of assuming a coding role, it becomes whatever agent the user defines.

## Table of contents

- [Why Keen Agent?](#why-keen-agent)
- [What you can build](#what-you-can-build)
- [Configuring an agent](#configuring-an-agent)
  - [Configuration reference](docs/configuration-reference.md)
  - [Modes and tool policy](#modes-and-tool-policy)
- [Quickstart](#quickstart)
- [Common commands](#common-commands)
- [Skills, subagents, and MCP](#skills-subagents-and-mcp)
- [Features](#features)
- [Origin: extracted from Keen Code](#origin-extracted-from-keen-code)
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

## Extracted from Keen Code

Keen Agent originated in [Keen Code](https://github.com/mochow13/keen-code), a lightweight, opinionated terminal coding agent. Keen Code established the core harness: an interactive terminal experience, provider-native tool loops, filesystem and shell tools, safety boundaries, sessions, skills, subagents, and MCP support.

As Keen Code evolved, it became apparent that these capabilities were not inherently limited to coding. Reading and editing files, searching a workspace, running tools, delegating focused work, and producing durable artifacts are useful primitives for research, operations, analysis, writing, and many other workflows. Keen Agent was therefore extracted from Keen Code and fleshed out as a separate, domain-neutral project.

The key change is **composition instead of a fixed coding identity**. Keen Code is intentionally designed to be a software-engineering agent. Keen Agent keeps the native power of that coding-agent runtime, but lets users define the agent's identity, instructions, modes, model preference, tool policy, skills, subagents, and MCP integrations through configuration. The same terminal harness can consequently become a research assistant, stock-analysis agent, operations investigator, documentation agent, or another agent tailored to a user's workspace.

Keen Code contributed several foundations that make this possible:

- **Permission-aware execution** — a central filesystem guard distinguishes allowed, approval-required, and policy-blocked paths; Git-ignored and sensitive paths stay protected.
- **Lean, durable context** — sessions are namespaced by working directory, while completed turns retain compact tool-activity metadata rather than raw file contents, command output, or external responses.
- **Prompt-efficient extensibility** — configured MCP servers become generated skills: the agent discovers a server's tools and schemas on demand instead of carrying every external tool schema in its system prompt.
- **Controlled delegation** — focused subagents can investigate with explicitly limited, read-only tools, leaving decisions and side effects with the primary agent.
- **Provider-native tool loops** — the harness streams provider responses and tool calls while adapting a shared tool registry to each supported model provider.

Visit the [Keen Code repository](https://github.com/mochow13/keen-code) or the [Keen Code website](https://mochow13.github.io/keen-code/) to learn more about the original coding agent.

## What you can build

Keen Agent is intentionally domain-neutral. Its behavior comes from the prompts, workspace resources, and tools you configure; skills, subagents, and MCP servers are optional additions loaded only from locations declared in the agent YAML. A single terminal runtime can support very different agents, for example:

- **Investment research and portfolio monitoring** — combine company filings, earnings transcripts, portfolio holdings, and market-data MCP tools; delegate financial-statement and risk analysis to specialists; then produce a cited investment memo and flag material changes since the previous review.
- **Production incident investigation** — inspect application logs and runbooks, query systems such as Loki or Grafana through MCP, correlate errors with deployments, and generate an evidence-backed incident timeline, likely causes, and next diagnostic steps without changing production systems.
- **Contract and policy compliance review** — evaluate contracts, completed questionnaires, or operating procedures against local policies and required controls; identify missing clauses or evidence, cite the relevant requirements, and save a structured exception report for human approval.
- **Customer-feedback intelligence** — process exported support tickets, interview notes, and survey responses; cluster recurring problems, quantify their frequency, connect them to known documentation or product areas, and produce a prioritized findings report with representative examples.
- **Research synthesis and evidence review** — search the web and a local paper library, compare claims and methodologies, track conflicting evidence and uncertainty, and create a cited briefing that separates established findings from assumptions and open questions.

These are configurations rather than separate applications. Each agent can live in its own folder with its YAML definition, system prompts, project instructions, reference material, skills, subagents, and MCP configuration. Start it in read-only `plan` mode when it should only investigate, or allow selected tools in `build` mode when it should create or update workspace artifacts.

## Configuring an agent

Keen Agent reads its identity and behavior from the YAML file passed to `--agent`. The file can have any name; `agent.yaml` is only a convention. A minimal definition needs a `name` and at least one prompt source:

```yaml
name: Research Assistant

system_prompt: |
  You are a careful research assistant.
  Cite evidence, distinguish facts from assumptions, and explain uncertainty.

default_mode: plan
```

The `model` field is optional. You can launch Keen Agent first and use `/model` in the interactive UI to authenticate with a provider and select a model. A warning on the initial launch before model setup is expected.

Validate and start the agent:

```bash
keen-agent validate --agent ./research-assistant.yaml
keen-agent --agent ./research-assistant.yaml
```

Agent definitions can also configure prompt files, project instructions, modes, tool exclusions, `/btw`, `/adversary`, skills, subagents, and MCP servers. See the **[complete configuration reference](docs/configuration-reference.md)** for every supported field, accepted values, resource formats, validation rules, and path-resolution behavior.

Relative resource paths in the YAML resolve from the YAML file's directory. The directory from which you launch `keen-agent` remains the agent's filesystem working context. This lets you keep a reusable agent definition separate from the workspace it analyzes, or place everything together in one dedicated agent folder.

### Modes and tool policy

- **`build`** is the default mode and exposes the configured built-in tool set.
- **`plan`** exposes only read-only built-in tools for investigation and planning.
- Select a starting mode with `--mode`, or switch interactively with `/mode`.
- Use `builtin_tools.exclude` to remove built-in capabilities the agent does not need.

For exact tool names, mode overlays, and exclusion behavior, see [Modes](docs/configuration-reference.md#modes) and [`builtin_tools`](docs/configuration-reference.md#builtin_tools) in the configuration reference.

## Quickstart

### Prerequisites

- Go **1.25.7 or newer** to build from source.
- Credentials for at least one supported LLM provider, or AWS credentials for Amazon Bedrock.

### Install

Install the latest released binary on macOS or Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/mochow13/keen-agent/main/scripts/install.sh | bash
```

The installer prints the installed path. It uses `/usr/local/bin` when writable; otherwise it uses `~/.local/bin`. If it reports `~/.local/bin/keen-agent`, add that directory to your `PATH` before running the command:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
keen-agent --help
```

For Bash, add the same `export PATH=...` line to `~/.bashrc` instead. Open a new shell or run `source ~/.bashrc` after adding it. If `keen-agent` is still not found, check the installed location with:

```bash
ls -l /usr/local/bin/keen-agent ~/.local/bin/keen-agent 2>/dev/null
```

```bash
curl -fsSL https://raw.githubusercontent.com/mochow13/keen-agent/main/scripts/install.sh | bash -s -- --version v1.2.3 --dir ~/.local/bin
```

Alternatively, build a local binary from a clone:

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

These extensions are opt-in and loaded only from paths declared by the agent definition:

- **Skills** are reusable instruction bundles stored as `SKILL.md` files. Configure `skills_dirs`, then inspect or manage them with `/skills`.
- **Subagents** are focused, read-only assistants defined in Markdown with YAML frontmatter. Configure `subagents_dirs`, then list available profiles with `/subagents list`.
- **MCP servers** connect external tools over local stdio or remote streamable HTTP. Configure `mcp_config_paths`, then use `/mcp`, `/mcp status`, or `/mcp connect` to manage connections.

The [configuration reference](docs/configuration-reference.md#referenced-resource-formats) documents each resource format, path behavior, subagent fields, MCP transports, and authentication options. Do not commit credential-bearing MCP configuration files.

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

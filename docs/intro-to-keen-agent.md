# Introduction to Keen Agent

Keen Agent is a configurable, terminal-first AI agent harness. Instead of writing an application around an agent framework, you describe an agent in YAML and run it from the command line. The YAML file defines the agent's role, instructions, model preference, operating modes, available tools, and optional extensions.

This makes one runtime useful for many kinds of work: research, operations, analysis, writing, documentation, data processing, repository maintenance, and team-specific workflows.

## Why Keen Agent

Many agent frameworks are code-first: using them means choosing an SDK, implementing a tool loop, managing model providers, persisting conversations, and building a user interface. No-code platforms avoid some of that work, but often require a hosted service or a browser-based workflow builder.

Keen Agent provides another option: **configure an agent as a file and run it locally in the terminal**.

Its main advantages are:

- **No agent application code required.** Define the agent's behavior and capabilities in a YAML file.
- **Domain-neutral by design.** Keen Agent is not limited to coding. Its purpose comes from the instructions you provide.
- **Terminal-native operation.** Use an interactive session for ongoing work or invoke one turn from scripts, pipelines, and scheduled jobs.
- **Portable, reviewable configuration.** YAML agent definitions and prompt files can be versioned and shared with a team.
- **Provider flexibility.** Select a provider and model in the agent definition, configure one interactively, or override them for an individual run.
- **Controlled capabilities.** Exclude built-in tools that an agent does not need, and use read-only plan mode when the agent should inspect without changing anything.
- **Permission-aware execution.** Filesystem and shell operations pass through safety checks; sensitive paths are blocked and potentially unsafe actions require approval.
- **Optional extensibility.** Add reusable skills, focused read-only subagents, or MCP servers only when the agent needs them.
- **Durable terminal workflows.** Sessions can be saved, resumed, and compacted as conversations grow.

A useful mental model is that the YAML file is the agent's portable definition, while Keen Agent supplies the runtime: the terminal interface, model integration, tools, permissions, sessions, and tool-execution loop.

## Why run agents locally

An agent is most useful when it can work with the same artifacts and tools as the user. Running Keen Agent in a terminal places it in a familiar workspace, where it can inspect permitted files, understand directory structure, and—when authorized—create or update the outputs needed to complete a task. This is more practical than repeatedly copying file contents into a chat window and transferring the response back by hand.

### The power of a coding-agent runtime without a coding-only identity

Terminal agents are traditionally treated as coding agents because the terminal naturally exposes the capabilities software work requires: navigating a workspace, reading many related files, searching large collections of text, running tools, making precise edits, and iterating until an output is complete. Those capabilities are not inherently about programming. They are general-purpose building blocks for any task grounded in files, tools, and durable artifacts.

Keen Agent preserves that powerful execution model but separates it from the agent's identity. The runtime supplies the native strengths associated with coding agents—filesystem awareness, shell and tool use, multi-step execution, reviewable changes, permission controls, and persistent sessions—while the user defines what the agent is for.

That purpose is fully composable from local resources:

- **System prompts and project instructions** define the agent's role, standards, and operating procedure.
- **A dedicated workspace** supplies its documents, datasets, templates, policies, and expected outputs.
- **Built-in tool policy** determines which local capabilities the agent may use.
- **Skills** provide reusable procedures for specialized tasks.
- **Subagents** contribute focused, read-only expertise.
- **MCP servers** connect domain-specific tools and external services.
- **Models and modes** control how the agent reasons and whether it may modify the workspace.

This composition turns the terminal from a presumed software-development interface into a general agent environment. The same Keen Agent runtime can become a stock-analysis agent with market-data tools and investment policies, a research assistant with source-evaluation instructions and reference libraries, or an operations agent with runbooks, log access, and incident procedures. Users get the capability of an agent that can act on real working context without having to build a separate application or accept a fixed, vendor-defined assistant for every use case.

Local execution is beneficial because it provides:

- **Direct access to working context.** The agent can read relevant files in the workspace instead of relying on partial excerpts pasted into a conversation. It can connect information spread across reports, notes, configuration, logs, and data files.
- **Useful outputs, not only answers.** With write access enabled, an agent can save a report, update documentation, organize notes, or generate a reusable artifact directly in the workspace.
- **Control over scope and permissions.** Start in read-only plan mode for investigation, exclude unnecessary tools in the agent definition, and approve sensitive operations only when needed. The agent should receive the least capability required for its task.
- **Visibility and reviewability.** File changes remain on the user's machine and can be inspected with normal tools. In a Git repository, changes can be diffed, reviewed, accepted, or reverted before they are shared.
- **Compatibility with existing workflows.** A terminal agent can participate in shell scripts, pipelines, scheduled jobs, and other command-line processes. Standard input and JSON output make it possible to connect the agent to other tools without building a custom application.
- **Less manual movement of information.** Users do not need to upload every document or repeatedly copy responses between a browser and local files. This reduces friction and the risk of working from an outdated excerpt.
- **Ownership of agent configuration.** Prompts, policies, tool restrictions, and extension declarations live in local files that can be reviewed and versioned alongside the work they support.

For example, a local agent could:

- Read a directory of meeting notes, identify unresolved decisions, and write a consolidated action-items document.
- Inspect application logs and operational runbooks, correlate recurring symptoms, and propose an incident investigation plan without changing anything in plan mode.
- Review a collection of research papers or Markdown notes, compare their claims, and produce a cited summary in the same workspace.
- Check policy documents and completed forms for missing information, then generate a review report for a human to approve.
- Process incoming CSV exports on a schedule and emit a JSON or Markdown summary for another command-line workflow.
- Maintain team documentation by finding inconsistent terminology or stale references and, after approval, applying focused edits.

Running locally does not necessarily mean that model inference is local. If the configured provider is a hosted service, prompts and any file content supplied to the model may leave the machine under that provider's terms. Use an appropriate provider, avoid exposing secrets, and review the agent's tools and permissions before giving it access to sensitive workspaces. Local execution still gives users control over the runtime, files, configuration, and approval flow; model data handling depends on the selected provider.

## Configure an agent with YAML

Create a YAML file that defines the agent. The filename `agent.yaml` is only a convention used in some examples—it is not required or reserved. You can use any descriptive filename, such as `stock-analysis.yaml`, `research-assistant.yaml`, or `operations-review.yaml`.

Regardless of its filename, an agent configuration requires:

1. `name`
2. At least one prompt source: `system_prompt` or `system_prompt_files`

### Create a dedicated agent workspace

A useful pattern is to give each agent its own folder. The folder becomes a self-contained workspace for the agent definition and its supporting resources, including system prompts, project instructions, subagents, skills, MCP configuration, reference documents, and generated output.

For example:

```text
agents/
├── stock-analysis/
│   ├── stock-analysis.yaml
│   ├── prompts/
│   │   ├── system.md
│   │   └── investment-policy.md
│   ├── subagents/
│   │   └── market-researcher.md
│   ├── skills/
│   │   └── earnings-review/
│   │       └── SKILL.md
│   ├── mcp.json
│   ├── data/
│   │   └── watchlist.csv
│   └── reports/
└── research-assistant/
    ├── research-assistant.yaml
    ├── prompts/
    │   └── system.md
    ├── subagents/
    ├── skills/
    ├── mcp.json
    └── sources/
```

The stock-analysis definition could refer to its local resources with relative paths:

```yaml
name: Stock Analysis

system_prompt_files:
  - prompts/system.md
  - prompts/investment-policy.md

subagents_dirs:
  - subagents

skills_dirs:
  - skills

mcp_config_paths:
  - mcp.json

default_mode: plan
```

Keen Agent resolves these resource paths relative to the directory containing the YAML definition. Keeping them together makes the agent easier to understand, version, copy, and share without mixing its resources with those of another agent.

Run Keen Agent from the dedicated folder when you want that folder to be the working context for filesystem operations:

```bash
cd agents/stock-analysis
keen-agent --agent ./stock-analysis.yaml
```

A separate agent can have a different definition, prompts, extensions, reference material, and output structure:

```bash
cd agents/research-assistant
keen-agent --agent ./research-assistant.yaml
```

You can also point `--agent` at a definition from another location. The important distinction is that relative resource paths in the YAML are based on the definition file's directory, while the directory from which Keen Agent is launched is the working context for local operations.

Here is a minimal research agent definition, which you could save as `research-assistant.yaml`:

```yaml
name: Research Assistant

system_prompt: |
  You are a careful research assistant.
  Investigate the user's question using available evidence.
  Distinguish facts from assumptions, cite sources when possible,
  and finish with a concise recommendation.

# Optional: start read-only so the agent can inspect but not modify files.
default_mode: plan

# Optional: remove tools this agent should never use.
builtin_tools:
  exclude:
    - write_file
    - edit_file
    - bash
```

This definition is enough to start Keen Agent. On first launch, the `/model` command can configure the provider, credentials, and model.

### Select a model in the YAML file

To make the agent prefer a particular provider and model, add `model`:

```yaml
name: Research Assistant

model:
  provider: anthropic
  model_id: claude-sonnet-4-6

system_prompt: |
  You are a careful research assistant.
  Gather relevant evidence before drawing conclusions.
  Cite sources and clearly communicate uncertainty.

default_mode: plan
```

When `model` is present, both `provider` and `model_id` are required. Credentials are not stored in the agent definition; configure them through `/model` or the authentication mechanism supported by the selected provider. Do not commit credentials to a YAML file.

### Keep longer instructions in separate files

For a substantial agent, keep its instructions in Markdown files instead of putting everything inline:

```yaml
name: Operations Analyst

system_prompt_files:
  - prompts/persona.md
  - prompts/operations-policy.md

project_instructions: team-instructions.md
default_mode: plan

modes:
  plan:
    system_prompt: |
      Investigate the evidence and propose a safe course of action.
      Do not modify files or execute corrective actions.
  build:
    system_prompt: |
      Carry out the approved action, verify the result, and report
      every material change.

builtin_tools:
  exclude:
    - web_fetch
```

All relative resource paths are resolved from the directory containing the agent's YAML file, not from the shell's current directory. `system_prompt` and `system_prompt_files` may be used together; Keen Agent composes the inline prompt and files in their declared order.

### Optional extensions

An agent can opt into additional capability directories and MCP configurations:

```yaml
name: Team Assistant

system_prompt: |
  Follow the team's procedures and use specialist capabilities only
  when they are relevant to the request.

skills_dirs:
  - .keen/skills

subagents_dirs:
  - .keen/subagents

mcp_config_paths:
  - .keen/mcp.json
```

These extensions are not discovered automatically. Keen Agent loads them only from locations explicitly declared in the YAML definition.

### Validate the configuration

Validate an agent after creating or changing it. Pass the actual filename to `--agent`:

```bash
keen-agent validate --agent ./research-assistant.yaml
```

Keen Agent uses strict YAML validation. It reports unknown fields, missing prompt sources, invalid model configuration, unsupported tool names, and missing referenced files before an agent starts.

## Install Keen Agent

### Install a released binary

On macOS or Linux, run:

```bash
curl -fsSL https://raw.githubusercontent.com/mochow13/keen-agent/main/scripts/install.sh | bash
```

The installer uses `/usr/local/bin` when that directory is writable. Otherwise, it installs to `~/.local/bin`. If necessary, add the latter to your `PATH`.

For Zsh:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

For Bash:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

Confirm the installation:

```bash
keen-agent --help
```

A specific release and installation directory can also be selected:

```bash
curl -fsSL https://raw.githubusercontent.com/mochow13/keen-agent/main/scripts/install.sh \
  | bash -s -- --version v1.2.3 --dir ~/.local/bin
```

### Build from source

Building from source requires Go 1.25.7 or newer:

```bash
git clone https://github.com/mochow13/keen-agent.git
cd keen-agent
go build -o keen-agent ./cmd
```

Move the resulting binary to a directory on your `PATH`, or run it as `./keen-agent`.

## Run Keen Agent with the YAML file

### Start an interactive session

From the agent's workspace, pass its YAML filename to `--agent`:

```bash
keen-agent --agent ./research-assistant.yaml
```

Keen Agent validates and loads the definition, then opens its interactive terminal interface. If no usable model has been configured yet, enter:

```text
/model
```

Choose a provider, authenticate or enter the required credentials, and select a model. Provider configuration is stored in `~/.keen-agent/configs.json` with owner-only permissions rather than in the YAML file.

You can then give the configured agent a task:

```text
> Compare the available deployment options and recommend the safest one.
```

Use `/help` to list interactive commands. Useful commands include `/mode` to switch between plan and build modes, `/resume` to resume a session, and `/mcp` to inspect configured MCP servers.

### Select a mode at startup

Start explicitly in read-only plan mode:

```bash
keen-agent --agent ./research-assistant.yaml --mode plan
```

Or start in build mode, where the configured write and shell tools may be available:

```bash
keen-agent --agent ./research-assistant.yaml --mode build
```

Plan mode exposes only read-only built-in tools. Build mode exposes the built-in tools that remain after applying `builtin_tools.exclude`; individual actions are still subject to Keen Agent's permission checks.

### Run one non-interactive task

Use `run` when integrating the agent into a shell script, pipeline, or scheduled job:

```bash
keen-agent run --agent ./research-assistant.yaml "Research this topic and summarize the findings"
```

Return machine-readable output with JSON formatting:

```bash
keen-agent run \
  --agent ./research-assistant.yaml \
  --format json \
  "Analyze these findings and identify the main risks"
```

A prompt can also be supplied through standard input:

```bash
printf 'Summarize the current operational status' \
  | keen-agent run --agent ./research-assistant.yaml
```

Temporarily override the configured provider and model for a single invocation:

```bash
keen-agent run \
  --agent ./research-assistant.yaml \
  --provider anthropic \
  --model claude-sonnet-4-6 \
  "Review this report for unsupported conclusions"
```

With this pattern, the same reviewed YAML definition—regardless of its filename—can power an interactive assistant, an ad hoc command, or repeatable terminal automation without requiring a custom agent application.

# Keen Agent configuration reference

Keen Agent loads an agent definition from the YAML file passed to `--agent`. The file does not have to be named `agent.yaml`; names such as `stock-analysis.yaml` and `research-assistant.yaml` are valid.

```bash
keen-agent --agent ./research-assistant.yaml
keen-agent run --agent ./research-assistant.yaml "Summarize the sources"
keen-agent validate --agent ./research-assistant.yaml
```

This document describes every field accepted by the agent YAML schema, including each nested field. It also describes the file formats referenced by `skills_dirs`, `subagents_dirs`, and `mcp_config_paths`.

## YAML and path rules

- The file must contain exactly one YAML document.
- Unknown fields are rejected, including unknown nested fields.
- `name` is required.
- At least one of `system_prompt` or `system_prompt_files` is required.
- Relative resource paths are resolved from the directory containing the agent YAML file, not from the shell's current directory.
- Absolute resource paths remain absolute.
- Keen Agent's filesystem working directory is still the directory from which `keen-agent` is launched. The YAML directory controls resource path resolution; the launch directory controls the agent's working context.
- `system_prompt_files`, `modes.<mode>.system_prompt_files`, `btw.system_prompt_files`, `adversary.system_prompt_files`, `subagents_dirs`, `mcp_config_paths`, and `skills_dirs` accept either one string or an array of strings.
- `project_instruction_paths` must be an array of strings, even when it contains only one path.

A dedicated workspace can keep an agent and its resources together:

```text
stock-analysis/
├── stock-analysis.yaml
├── prompts/
│   ├── system.md
│   ├── plan.md
│   └── adversary.md
├── instructions/
│   └── ANALYSIS_RULES.md
├── skills/
│   └── earnings-review/
│       └── SKILL.md
├── subagents/
│   └── filings-reviewer.md
└── mcp.json
```

If `stock-analysis.yaml` uses paths such as `./prompts/system.md`, they resolve inside `stock-analysis/` even if Keen Agent is launched with an absolute path to the YAML file.

### Agent resources and the working directory

Keen Agent uses two directory contexts:

1. **Agent configuration directory:** the directory containing the YAML file. Relative paths declared by the YAML are resolved from here, including prompt files, project instructions, skill directories, subagent directories, and MCP configuration files.
2. **Working directory:** the directory from which `keen-agent` is launched. This is the agent's filesystem workspace. Relative paths passed to filesystem tools are resolved here, and shell commands run here.

For example, suppose the reusable agent definition and the data to analyze are stored separately:

```text
~/agents/stock-analysis/
├── stock-analysis.yaml
├── prompts/
│   └── system.md
└── skills/
    └── earnings-review/
        └── SKILL.md

~/work/acme-portfolio/
├── holdings.csv
└── notes.md
```

The agent definition contains paths relative to its own directory:

```yaml
name: Stock Analysis
system_prompt_files: ./prompts/system.md
skills_dirs: ./skills
```

Launch it from the portfolio workspace:

```bash
cd ~/work/acme-portfolio
keen-agent --agent ~/agents/stock-analysis/stock-analysis.yaml
```

In this run:

- `./prompts/system.md` resolves to `~/agents/stock-analysis/prompts/system.md`.
- `./skills` resolves to `~/agents/stock-analysis/skills`.
- The agent's filesystem workspace is `~/work/acme-portfolio`.
- A tool request for `holdings.csv` resolves to `~/work/acme-portfolio/holdings.csv`.
- Shell commands run from `~/work/acme-portfolio`.

This separation makes one agent definition reusable across multiple workspaces. To keep the definition, resources, and working files in one self-contained workspace, launch Keen Agent from the YAML directory instead:

```bash
cd ~/agents/stock-analysis
keen-agent --agent stock-analysis.yaml
```

In that case, the agent configuration directory and working directory are the same.

## Complete example

```yaml
name: Stock Analysis

ascii_art: |
       ╭────────────────────────────────────╮
       │   $  S T O C K   A N A L Y S I S   │
       │                                    │
       │       ╭─╮              ╭────●      │
       │    ╭──╯ ╰─╮      ╭─────╯           │
       │  ──╯      ╰──────╯       ▲         │
       │                         / \        │
       ╰────────────────────────────────────╯

model:
  provider: anthropic
  model_id: claude-sonnet-4-6

system_prompt: |
  You are a careful stock-analysis agent.
  Separate reported facts from estimates and assumptions.

system_prompt_files:
  - ./prompts/system.md
  - ./prompts/risk-policy.md

project_instruction_paths:
  - ./instructions/ANALYSIS_RULES.md
  - ./instructions/OUTPUT_FORMAT.md

default_mode: plan

modes:
  plan:
    system_prompt: |
      Investigate and explain. Do not produce or modify final artifacts.
    system_prompt_files: ./prompts/plan.md
  build:
    system_prompt: |
      Produce the requested reports after checking the evidence.
    system_prompt_files:
      - ./prompts/build.md

btw:
  enabled: true
  context_messages: 10
  system_prompt: Answer side questions briefly.
  system_prompt_files: ./prompts/btw.md

adversary:
  enabled: true
  model:
    provider: openai
    model_id: gpt-5.4
  system_prompt: Challenge unsupported financial conclusions.
  system_prompt_files:
    - ./prompts/adversary.md

builtin_tools:
  exclude:
    - write_file
    - edit_file
    - bash

subagents_dirs:
  - ./subagents

mcp_config_paths:
  - ./mcp.json

skills_dirs:
  - ./skills
```

Every field except `name` and the required prompt source is optional.

## Top-level fields

The accepted top-level fields are:

- `name`
- `ascii_art`
- `model`
- `system_prompt`
- `system_prompt_files`
- `project_instruction_paths`
- `default_mode`
- `modes`
- `btw`
- `adversary`
- `builtin_tools`
- `subagents_dirs`
- `mcp_config_paths`
- `skills_dirs`

No other top-level fields are accepted.

### `name`

Type: string
Required: yes

The human-readable agent name. It appears on the interactive welcome screen and contributes to the identity used for the agent's saved sessions.

The value must not be empty or whitespace-only.

```yaml
name: Research Assistant
```

### `ascii_art`

Type: string
Required: no

Replaces the default Keen Agent artwork on the interactive welcome screen. A YAML block scalar is the easiest way to preserve multiple lines.

```yaml
ascii_art: |
       ╭────────────────────────────────────╮
       │   $  S T O C K   A N A L Y S I S   │
       │                                    │
       │       ╭─╮              ╭────●      │
       │    ╭──╯ ╰─╮      ╭─────╯           │
       │  ──╯      ╰──────╯       ▲         │
       │                         / \        │
       ╰────────────────────────────────────╯
```

This field affects presentation only. It is not included in the model's prompt and does not affect non-interactive output.

### `model`

Type: mapping
Required: no

Selects the preferred model for this agent.

```yaml
model:
  provider: anthropic
  model_id: claude-sonnet-4-6
```

The mapping accepts exactly two fields:

- `provider`: provider ID
- `model_id`: model ID under that provider

If the `model` block is present, both fields are required. An incomplete block fails agent validation.

Supported provider IDs are:

- `anthropic`
- `openai`
- `openai-codex`
- `googleai`
- `moonshotai`
- `deepseek`
- `zai`
- `minimax`
- `opencode-go`
- `amazon-bedrock`
- `openai-compatible`

`model_id` is not a single fixed enumeration. Keen Agent ships a model registry, but providers and OpenAI-compatible endpoints can have additional models. The provider/model pair must be available in the user's global Keen Agent configuration (`~/.keen-agent/configs.json`) for the agent preference to be used. Use `/model` to configure and select available models. The built-in choices are maintained in `internal/providers/registry.yaml`.

Credentials, base URLs, custom headers, and global thinking effort are not fields in the agent YAML. They belong to the global model configuration. Do not put provider API keys in the agent definition.

> **Important:** Provider authentication can be completed through Keen Agent's interactive UI with the `/model` command. The `model` block is optional, so users do not need to configure a provider or model in the agent YAML before starting Keen Agent. If no usable model has been configured yet, the initial launch may show a warning; open `/model`, authenticate or configure a provider, and select a model to continue. Provider credentials remain in the global model configuration rather than the agent definition.

If `model` is omitted, Keen Agent uses the active model from the global configuration. If no active model is ready, launch Keen Agent and use `/model` to authenticate with a provider and select one; an initial warning before completing that flow is expected and does not make the agent YAML invalid. If the YAML's preferred provider/model pair is complete but unavailable or not ready, Keen Agent displays a warning and falls back to the active global model when possible.

### `system_prompt`

Type: string
Required: conditionally

Defines inline instructions for the main agent. Use a YAML block scalar for a multi-line prompt.

```yaml
system_prompt: |
  You are a research assistant.
  Evaluate source quality and state uncertainty explicitly.
```

At least one of `system_prompt` or `system_prompt_files` must be provided. Mode-specific prompts do not satisfy this requirement.

When both fields are present, Keen Agent composes the main persona in this order:

1. Keen Agent's built-in tool-use and safety contract
2. `system_prompt`
3. Each file in `system_prompt_files`, in listed order

The sources are additive; prompt files do not replace the inline prompt.

### `system_prompt_files`

Type: string or array of strings
Required: conditionally

References one or more files containing main system-prompt instructions.

One file:

```yaml
system_prompt_files: ./prompts/system.md
```

Multiple files:

```yaml
system_prompt_files:
  - ./prompts/role.md
  - ./prompts/quality-policy.md
```

Every referenced path must exist, be a readable regular file, and pass the filesystem checks performed during validation. Relative paths resolve from the agent YAML directory. Files are appended in listed order after `system_prompt`.

### `project_instruction_paths`

Type: array of strings
Required: no

Adds project or workspace instructions to the main system prompt.

```yaml
project_instruction_paths:
  - ./instructions/WORKFLOW.md
  - ./instructions/OUTPUT_FORMAT.md
```

Unlike the fields that use the string-or-array form, this field must be a YAML array. Every path must be a readable regular file.

The loaded files are grouped under a `Project Instructions` section and identified by path. Their combined content is limited to 8 KiB; content beyond that limit is truncated. Use this field for workspace policies and conventions, and use `system_prompt_files` for the agent's core role and behavior.

The older field name `project_instructions` is not supported.

### `default_mode`

Type: string
Required: no
Default: `build`

Sets the mode active when a session starts.

Allowed values are:

- `plan`: read-only investigation and planning
- `build`: action-oriented operation within configured tools and permissions

```yaml
default_mode: plan
```

The CLI flag overrides this value for a launch:

```bash
keen-agent --agent ./research-assistant.yaml --mode build
keen-agent run --agent ./research-assistant.yaml --mode plan "Review the evidence"
```

In the interactive REPL, `/mode plan` and `/mode build` switch the active mode. Plan mode filters the active tool registry to tools marked read-only; it is not merely a prompt suggestion.

### `modes`

Type: mapping
Required: no

Adds mode-specific instructions. The only accepted mapping keys are:

- `plan`
- `build`

Each mode mapping accepts exactly these fields:

- `system_prompt`: optional inline string
- `system_prompt_files`: optional string or array of strings

```yaml
modes:
  plan:
    system_prompt: Focus on evidence, assumptions, and open questions.
    system_prompt_files: ./prompts/plan.md
  build:
    system_prompt: Produce the requested artifact and verify it.
    system_prompt_files:
      - ./prompts/build.md
      - ./prompts/output-policy.md
```

Mode prompts are overlays. They are appended after the main agent prompt, project instructions, skills and subagent catalogs, and Keen Agent's built-in instructions for the active mode. They do not replace `system_prompt` or `system_prompt_files`.

A mode does not need to be present in `modes` to be usable. Without an overlay, Keen Agent still applies its built-in plan or build behavior.

All configured mode prompt files must be readable regular files, even if that mode is not the default.

### `btw`

Type: mapping
Required: no

Configures the interactive `/btw <question>` helper. The helper answers a side question without adding that exchange to the main conversation history and has no tool access.

```yaml
btw:
  enabled: true
  context_messages: 10
  system_prompt: |
    Answer the side question directly and concisely.
  system_prompt_files:
    - ./prompts/btw.md
```

The mapping accepts exactly these fields:

- `enabled`: boolean; enables or disables `/btw`
- `context_messages`: integer; maximum number of recent conversation messages supplied to the helper
- `system_prompt`: optional inline helper prompt
- `system_prompt_files`: optional string or array of helper prompt files

When `enabled: true`, `context_messages` is required in practice and must be greater than zero. Omitting it gives the zero value and fails validation.

The helper uses the main agent's resolved LLM client. It receives up to `context_messages` recent messages, excluding the current trailing user message when applicable. It receives no tools. If neither helper prompt source contains non-whitespace content, Keen Agent uses its built-in concise side-question prompt.

When both prompt sources are set, the inline prompt comes first, followed by files in listed order. Prompt-file existence is validated only when the helper is enabled.

This block is primarily an interactive REPL feature; it does not turn `keen-agent run` messages into `/btw` commands.

### `adversary`

Type: mapping
Required: no

Configures the interactive `/adversary [focus]` helper, which reviews the main conversation for factual errors, unsafe actions, missed constraints, unsupported assumptions, and risks.

```yaml
adversary:
  enabled: true
  model:
    provider: openai
    model_id: gpt-5.4
  system_prompt: Be direct and prioritize material issues.
  system_prompt_files: ./prompts/adversary.md
```

The mapping accepts exactly these fields:

- `enabled`: boolean; enables or disables `/adversary`
- `model`: optional model mapping
  - `provider`: provider ID
  - `model_id`: model ID
- `system_prompt`: optional inline critic prompt
- `system_prompt_files`: optional string or array of critic prompt files

If `adversary.model` is present, both `provider` and `model_id` are required. The provider/model pair is resolved through the global model configuration, so its credentials and provider settings must already be configured.

Model selection follows this order:

1. `adversary.model` in the agent YAML
2. The adversary provider/model in the global Keen Agent configuration
3. The main agent's resolved model

The adversary receives the main conversation and a read-only view of the main tool registry. If no custom adversary prompt has content, Keen Agent uses its built-in adversarial-review prompt. When both custom prompt sources are present, the inline prompt comes first and files follow in listed order.

Prompt-file existence is validated only when the adversary is enabled. This block is primarily an interactive REPL feature.

### `builtin_tools`

Type: mapping
Required: no

Restricts Keen Agent's built-in tools.

```yaml
builtin_tools:
  exclude:
    - write_file
    - edit_file
    - bash
```

The mapping currently accepts one field:

- `exclude`: array of built-in tool names to omit

The complete set of excludable tool names is:

- `read_file`
- `write_file`
- `edit_file`
- `web_fetch`
- `glob`
- `grep`
- `bash`

Unknown names fail validation.

The following conditional tools cannot be excluded with this field:

- `call_mcp_tool`: registered when `mcp_config_paths` is configured
- `delegate_task`: registered when `subagents_dirs` is configured

Attempting to list either one fails validation. Remove the corresponding MCP or subagent configuration if the agent must not receive that capability.

`builtin_tools.exclude` controls tool registration, while plan mode independently filters the resulting registry to read-only tools. Permissions and filesystem guards still apply to registered tools.

The older per-tool permission shape under `builtin_tools` is not supported.

### `subagents_dirs`

Type: string or array of strings
Required: no

Lists directories containing read-only subagent profiles.

```yaml
subagents_dirs: ./subagents
```

or:

```yaml
subagents_dirs:
  - ./subagents
  - ./specialists
```

Each path must exist and be a directory. Keen Agent scans `*.md` files directly inside each directory; discovery is not recursive. If at least one directory is configured, the non-excludable `delegate_task` tool is registered.

Subagent names must be unique across all configured directories. Agent validation checks each discovered Markdown file for YAML frontmatter with non-empty `name` and `description` fields.

See [Subagent profile format](#subagent-profile-format) for all profile fields.

### `mcp_config_paths`

Type: string or array of strings
Required: no

References one or more JSON files defining Model Context Protocol servers.

```yaml
mcp_config_paths:
  - ./mcp.json
  - ./mcp-team.json
```

Each path must be a readable regular file containing a valid MCP configuration. Relative paths resolve from the agent YAML directory.

At runtime, server maps are merged in listed order. If later files define the same server name, the later definition replaces the earlier one. When this field contains at least one path, Keen Agent registers the non-excludable `call_mcp_tool` tool and attempts to start the configured MCP servers.

See [MCP configuration file format](#mcp-configuration-file-format) for the JSON schema.

### `skills_dirs`

Type: string or array of strings
Required: no

Lists roots containing reusable skills.

```yaml
skills_dirs:
  - ./skills
  - ./team-skills
```

Each path must exist and be a directory. Under each root, Keen Agent discovers skills using this exact layout:

```text
<skills-root>/<skill-directory>/SKILL.md
```

Discovery is one directory level deep; a `SKILL.md` directly in the root or deeper in a nested tree is not discovered. If two discovered skills use the same metadata name, the later duplicate is skipped with a warning.

See [Skill format](#skill-format) for the `SKILL.md` fields.

## Referenced resource formats

### Subagent profile format

Every subagent is a Markdown file with YAML frontmatter followed by its instructions:

```markdown
---
name: filings-reviewer
description: Reviews regulatory filings for risks and material changes.
tools:
  - read_file
  - glob
  - grep
timeout_seconds: 900
hidden: false
---

You review filing documents. Return concise findings with file references.
Do not make changes to the workspace.
```

The accepted frontmatter fields are:

- `name`: required, non-empty string; the name used by `delegate_task`
- `description`: required, non-empty string; tells the main agent when to delegate
- `tools`: optional array; requested read-only tools
- `provider`: optional string
- `model`: optional string
- `thinking_effort`: optional string
- `timeout_seconds`: optional integer
- `hidden`: optional boolean, default `false`

The Markdown body becomes the subagent's instructions.

#### `tools`

Subagents are restricted to the following read-only tools:

- `read_file`
- `glob`
- `grep`

If `tools` is omitted or empty, all three are available. If it is supplied, only recognized names from this list are enabled; other names do not grant additional tools.

#### `timeout_seconds`

Sets the profile's default execution timeout. A positive timeout passed directly by a delegation request takes precedence. If neither is positive, Keen Agent uses 1,800 seconds.

#### `hidden`

When `true`, the subagent is omitted from the catalog shown to the main model. The profile still exists and can be selected by exact name if a delegation explicitly requests it.

#### `provider`, `model`, and `thinking_effort`

These fields are accepted and parsed as profile metadata. In the current runtime, delegated subagents inherit the main agent's resolved model configuration; these profile fields do not override the client used for delegation. They should not be relied on for per-subagent model selection.

Unknown subagent frontmatter fields produce runtime warnings rather than agent-YAML parse errors.

### Skill format

Each skill lives in its own directory and is defined by `SKILL.md`:

```markdown
---
name: earnings-review
description: Review an earnings release and extract material changes.
---

Read the supplied earnings release and compare it with prior-period data.
Use $ARGUMENTS as the requested review focus.
```

The frontmatter fields are:

- `name`: required, non-empty string
- `description`: required, non-empty string

The Markdown body contains the procedure followed when the skill is activated. Skill bodies can use `$ARGUMENTS` for all activation arguments and `$1` through `$9` for positional arguments.

Skills are enabled by default. Interactive `/skills enable <name>` and `/skills disable <name>` choices are stored in the user's global skill status file, `~/.keen-agent/skills/config.json`; enablement is not configured in the agent YAML.

Agent validation verifies that each `skills_dirs` entry is a directory. Individual skill metadata is loaded at runtime, where malformed skills are skipped with warnings.

### MCP configuration file format

Files referenced by `mcp_config_paths` are JSON, not YAML. The top-level object contains one field, `servers`, which maps server names to server definitions:

```json
{
  "servers": {
    "local-data": {
      "command": "local-data-mcp",
      "args": ["--readonly"],
      "env": {
        "LOG_LEVEL": "warn"
      }
    },
    "remote-research": {
      "url": "https://example.com/mcp",
      "auth": {
        "type": "oauth"
      }
    }
  }
}
```

Server names must be 1–128 characters, begin with a letter or number, and otherwise contain only letters, numbers, underscores, dashes, and dots.

Each server accepts these fields:

- `url`: HTTP or HTTPS MCP endpoint
- `auth`: HTTP authentication mapping
- `command`: executable for a stdio MCP server
- `args`: array of command arguments
- `env`: string-to-string map added to or overriding the child process environment

Transport is inferred; there is no `transport` field:

- If `command` is non-empty, Keen Agent uses stdio transport.
- Otherwise, Keen Agent uses streamable HTTP and requires `url` with an `http` or `https` scheme and a host.

For stdio, `args` and `env` configure the child process. Stdio does not support HTTP authentication; `auth.type` must be omitted, empty, or `none`.

The `auth` mapping accepts these fields:

- `type`: authentication type
- `header`: API-key HTTP header name
- `scheme`: optional scheme prepended to the API key
- `key`: API-key value
- `scopes`: array of OAuth scopes

Allowed `auth.type` values are:

- `none`: no HTTP authentication; also the default when `type` is omitted
- `api_key`: send a configured API key in an HTTP header
- `oauth`: use Keen Agent's OAuth flow

For `api_key`, `key` is required. The defaults are `header: Authorization` and `scheme: Bearer` when no custom header is supplied. If a custom `header` is supplied and `scheme` is omitted, the key is sent without a scheme prefix.

```json
{
  "servers": {
    "remote-api": {
      "url": "https://example.com/mcp",
      "auth": {
        "type": "api_key",
        "header": "X-API-Key",
        "key": "replace-with-key"
      }
    }
  }
}
```

MCP JSON does not perform environment-variable interpolation for `auth.key` or other string fields. Treat MCP files containing credentials as secrets and do not commit them. Prefer OAuth or an appropriately protected local configuration when possible.

## Validation

Validate an agent definition before running it:

```bash
keen-agent validate --agent ./stock-analysis.yaml
```

Validation checks:

- YAML syntax and a single-document file
- rejection of unknown agent YAML fields
- non-empty `name`
- presence of `system_prompt` or `system_prompt_files`
- complete main and adversary model blocks
- allowed default mode and mode keys
- positive `btw.context_messages` when `/btw` is enabled
- existence and readability of referenced prompt and instruction files
- existence and validity of MCP JSON files
- existence of skill and subagent directories
- required subagent `name` and `description` frontmatter
- duplicate subagent names across configured directories
- valid `builtin_tools.exclude` names

Validation checks that model blocks are structurally complete, but model availability is resolved against the user's global model configuration at runtime. Skill contents are also loaded at runtime rather than fully checked by the agent validator.

## Minimal valid configuration

```yaml
name: Research Assistant
system_prompt: You are a careful research assistant.
```

An equivalent file-based prompt is:

```yaml
name: Research Assistant
system_prompt_files: ./prompts/system.md
```

The filename is arbitrary in both cases; pass the chosen path explicitly with `--agent`.
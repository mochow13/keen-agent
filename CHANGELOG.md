# Changelog

All notable changes to Keen Agent are documented in this file.

## [Unreleased]

## [0.2.0] - 2026-08-23

### Added

- Add configuration-driven agents with validation, modes, custom prompts, project instruction files, ASCII art, model selection, and scoped session state.
- Add opt-in skills, subagents, MCP integrations, BTW and adversary helpers, and built-in tool exclusion.
- Add resumable sessions, automatic context compaction, context usage reporting, retained tool activity, and the `/tool-history` command.
- Add OpenAI Responses, ChatGPT OAuth, OpenAI-compatible providers, refreshed model metadata, provider headers, thinking controls, and shell-managed API key refresh.
- Add parallel subagent delegation, automatic dangerous-command classification, streaming headless progress, queued input, and expanded REPL navigation and rendering.

### Changed

- Rename MCP configuration to `mcp_config_paths` and require skills, subagents, and MCP resources to be explicitly configured.
- Rename `project_instructions` to the ordered `project_instruction_paths` array and remove unsupported bash policy configuration; `builtin_tools.exclude` remains supported.
- Prefer agent-specific adversary models over global settings while preserving provider authentication and tuning options.
- Replace the default persona with the configured prompt while retaining the built-in tool-use and safety harness.
- Migrate OpenAI integrations to the v3 SDK and update Go and project dependencies.

### Fixed

- Preserve configured integration tools, historical tool inputs, prompt caching, and helper tool event rendering.
- Improve credential refresh, context-window cleanup, clipboard fallback, interrupt handling, and release artifact generation.
## [0.1.1] - 2026-08-23

### Changed

- Generalize the default agent instructions so they remain domain-neutral.
- Document PATH setup for curl-based installations and clarify the release workflow.

### Fixed

- Remove obsolete npm publishing and correct release archive version handling in the curl installer.

## [0.1.0]

### Added

- Configurable, terminal-first AI agents defined in `agent.yaml`, with provider and model selection, prompt files, project instructions, and plan/build modes.
- Interactive REPL and one-shot `keen-agent run` command, including persistent, resumable sessions and JSON output for automation.
- Built-in, permission-aware filesystem, web, search, and shell tools.
- Context compaction, prompt caching, tool activity history, and configurable context controls.
- Provider integrations for Anthropic, OpenAI, ChatGPT OAuth, Google AI, Amazon Bedrock, DeepSeek, Moonshot AI, Z.ai, MiniMax, OpenCode Go, and OpenAI-compatible APIs.
- Optional skills, read-only subagents, and MCP support for local stdio and remote streamable HTTP servers.
- Agent configuration validation and `api_key_helper` support for shell-managed credentials.
- Curl installer for macOS and Linux release binaries, with SHA-256 verification.

[Unreleased]: https://github.com/mochow13/keen-agent/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/mochow13/keen-agent/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/mochow13/keen-agent/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/mochow13/keen-agent/releases/tag/v0.1.0

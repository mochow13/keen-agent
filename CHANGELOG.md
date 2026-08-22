# Changelog

All notable changes to Keen Agent are documented in this file.

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
- NPM package that installs the `keen-agent` command.

[0.1.0]: https://github.com/mochow13/keen-agent/releases/tag/v0.1.0

# Changelog

All notable changes to Keen Agent are documented in this file.

## [Unreleased]

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

[Unreleased]: https://github.com/mochow13/keen-agent/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/mochow13/keen-agent/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/mochow13/keen-agent/releases/tag/v0.1.0

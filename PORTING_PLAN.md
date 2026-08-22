# Porting Plan: Keen Code → Keen Agent

## Goal

Bring specific capabilities from the upstream `keen-code` repository (`../keen-code`, `main @ 7c4a1c7`, tag `v0.48.1-1-g7c4a1c7`) into the current `keen-agent` repository while preserving every Keen Agent-specific feature.

## Scope

The requested port covers three areas:

1. **All UI improvements** from Keen Code `main`.
2. **All context-management features**: historical tool activity, `/tool-history` command, manual compaction, and automatic compaction.
3. **Provider and model updates**, including the migration to `openai-go/v3`.

## Executive decision: update the current Keen Agent

A wholesale copy of Keen Code would force us to rebuild Keen Agent's identity:

- `internal/agentconfig` and the `--agent` flag
- `keen-agent validate` command
- `builtin_tools.exclude` and MCP/delegate tool gating
- Config-driven system-prompt composition with mode overlays
- Adversary and BTW helper wiring
- Agent-scoped state directories

Keen Code `main` has actually removed `internal/agentconfig` and simplified its config surface. Therefore the correct strategy is to treat Keen Code as an upstream source of truth and port feature **buckets** into the current Agent codebase.

## History relationship

```bash
git merge-base --all HEAD keen-code/main   # exits 1
```

The repositories do **not** share a common Git ancestor. Porting must be done manually or via carefully scoped patches, not via `git merge` or `git cherry-pick`.

## Scale

```text
HEAD..keen-code/main: 380 files changed, +44,877 / -7,315
- 490 commits in keen-code/main not in Agent
- 59 commits in Agent not in keen-code/main (must be preserved)
```

## Bucket order and rationale

Order is chosen to minimize risk and maximize early value:

1. **Dependency and build-tool baseline** (Go 1.25.13, `openai-go/v3`, security patches, CI). Required before provider/model work.
2. **Provider and model updates** (registry, loader, model IDs, thinking support). Mostly additive, high user value.
3. **OpenAI v3 SDK migration**. Required before some provider features can compile.
4. **Tool history and memory subsystem** (`internal/memory`, historical activity, `/tool-history`). Self-contained new package.
5. **Context management** (usage breakdown, reducer changes, manual compaction command).
6. **Automatic compaction** (foundation + provider integration + lifecycle). Depends on memory and context buckets.
7. **UI/REPL improvements** (largest surface, applied last so underlying LLM/context features exist to render).

## Bucket 1 — Dependency and build-tool baseline

### Commits / files of interest

- `12374c9 fix(ci): bump Go to 1.25.13 to resolve stdlib vulnerabilities`
- `93c5a6b chore(deps): bump github.com/go-git/go-git/v5 from 5.19.1 to 5.19.2`
- `3fdddda chore(deps): bump google.golang.org/grpc from 1.79.3 to 1.82.1`
- `e99c4f9 fix(deps): bump golang.org/x/text to v0.39.0`
- `7bbde40 fix(security): upgrade vulnerable Go dependencies`
- `620a54f fix(security): update vulnerable Go dependencies`
- `dfbc9b3 ci(security): scan reachable Go vulnerabilities`
- `0c5b0d1 Add CodeQL Weekly workflow for security scan`
- `63e6d5f ci: publish coverage reports to Codecov`

### Work

1. Update `go.mod`:
   - `go 1.25.7` → `go 1.25.13`
   - `github.com/openai/openai-go v1.8.2` → `github.com/openai/openai-go/v3 v3.50.0`
   - Bump AWS SDK, `go-git`, `grpc`, `golang.org/x/text`, `golang.org/x/crypto`, etc., to Keen Code versions.
2. Run `go mod tidy`.
3. Run `go test -race ./...` and fix any failures caused by dependency changes before proceeding.
4. Copy `.github/workflows/codeql.yml` from Keen Code.
5. Update `.github/workflows/go.yml` to add Codecov upload (keep Agent-specific release naming).

### Safety checks

- Verify `keen-agent` still builds and all existing tests pass.
- Do **not** change module path or binary name.

## Bucket 2 — Provider and model updates

### Commits of interest

- `02fe97b feat(providers): refresh model registry and thinking support`
- `29acb4a feat(providers): add GPT-5.6 models`
- `a0e1b23 feat(providers): add Claude Fable 5 and Sonnet 5 models`
- `3add07b feat(providers): refresh glm models in registry`
- `477ba3c feat(providers): prune superseded models and add Qwen3.7 Plus`
- `accaaad feat(providers): add Kimi K2.7 Code model to registry`
- `d30afcf feat(providers): add Qwen3.7 Max via Anthropic API and tighten plan mode restrictions`
- `1180d89 feat(providers): document openai-compatible provider and load providers without registry models`
- `3ec96d8 feat(providers): add openai-compatible provider and improve historical activity format`
- `9ecf6d4 chore(providers): remove obsolete loader comments`

### Work

1. Copy `internal/providers/registry.yaml` from Keen Code.
2. Merge Agent-only provider/model entries that do not exist in Keen Code (e.g., Agent has some IDs Keen Code pruned; confirm before dropping).
3. Port `internal/providers/loader.go` changes:
   - Loading providers without registry models.
   - Provider-specific model configuration resolution.
4. Update `internal/providers/loader_test.go` with new expectations.
5. Add `thinking_efforts` field handling if not already present.

### Safety checks

- `go test ./internal/providers/...` passes.
- No Agent-specific model IDs or aliases are lost.

## Bucket 3 — OpenAI v3 SDK migration

### Commits of interest

- `0be31f0 feat(llm): migrate to openai-go v3 SDK`

### Work

1. Update import paths from `github.com/openai/openai-go` to `github.com/openai/openai-go/v3` in:
   - `internal/llm/openai.go`
   - `internal/llm/openai_codex.go`
   - `internal/llm/openai_responses.go`
   - Tests for the above.
2. Port API changes from Keen Code:
   - `context_reducer.go` updates.
   - New client construction patterns.
   - Any renamed request/response fields.
3. Run tests for all OpenAI-compatible clients (OpenAI, Codex, DeepSeek, GLM, etc.).

### Safety checks

- `go test -race ./internal/llm/...` passes.
- Compilation succeeds for every provider path.

## Bucket 4 — Tool history and memory subsystem

### Commits of interest

- `4bb7a64 feat(memory): implement global and project memory`
- `7cf423a feat(memory): preserve historical tool activity`
- `63739c3 feat(memory): retain bounded tool inputs`
- `416c23e refactor(turn-memory): derive outcomes from tool outputs on stream segments`
- `eb7f0ee feat(llm): replay historical tools with native blocks`
- `02b98b3 feat(repl): configure cross-turn tool history`
- `c75d961 refactor(llm): align prompt with retained tool history`
- `7b6ab1e fix(llm): retain bounded file edit inputs in tool history`
- `3cebd41 refactor(repl): drop dead turn-memory rebuilds on retry`
- `2754e57 feat(llm): tighten tool-memory anti-hallucination guidance`

### Work

1. Add new package `internal/memory/`:
   - `memory.go`
   - `memory_test.go`
   - `secrets.go`
   - `secrets_test.go`
2. Update `internal/filesystem/guard.go` and tests for memory directory access.
3. Port `internal/llm/message.go` and `internal/llm/message_format.go` changes for historical activity formatting.
4. Port `internal/cli/repl/turn_memory.go` and its tests.
5. Add `/tool-history` slash command wiring in `internal/cli/repl/command_handlers.go` and `internal/cli/repl/commands/commands.go`.
6. Update system prompt to reference memory and tool-history constraints.
7. Update `edit_file.go`, `write_file.go`, `bash.go` to emit bounded/retained inputs as needed.

### Safety checks

- `go test -race ./internal/memory/... ./internal/cli/repl/... ./internal/llm/...` passes.
- Existing Agent tool-history behavior (if any) remains backward-compatible.

## Bucket 5 — Context management

### Commits of interest

- `ef9f09b feat(repl): show context usage breakdown`
- `0940ac7 feat(context): replace word-count heuristic with provider-backed token usage`
- `f6aa1b4 feat(repl): accumulate and display input/output tokens`
- `1f3322c feat(llm): reduce tool context before requests`
- `7a2e941 feat(llm): reduce long tool contexts`
- `6f81516 fix(llm): skip context-reducer targets smaller than placeholder threshold`
- `b6f0a3c feat(llm): clear pendingState on context overflow after reduction`
- `104a339 feat(repl): add manual context compaction command`

### Work

1. Add `internal/llm/context_breakdown.go` + test.
2. Update `internal/cli/repl/context_status.go` to show token input/output and breakdown.
3. Port context-reducer improvements in `internal/llm/context_reducer.go`.
4. Add `/compact` command wiring in command handlers.
5. Update session projection/store to support compaction state if required.

### Safety checks

- Manual `/compact` works in REPL.
- Context status displays accurate token counts.

## Bucket 6 — Automatic compaction

### Commits of interest

- `de6bc9b feat(llm): add automatic compaction foundation`
- `3394b29 feat(llm): integrate automatic compaction providers`
- `9dee4fd feat(prompt): revise agent and compaction instructions`
- `46f26c4 feat(session): persist compaction checkpoints atomically`
- `eeb757c feat(repl): handle automatic compaction lifecycle`
- `6440764 docs(compaction): document compaction flows`

### Work

1. Add `internal/llm/auto_compaction.go` + test.
2. Integrate auto-compaction into each LLM provider client (`anthropic.go`, `bedrock.go`, `genkit.go`, `openai.go`, `openai_codex.go`, `openai_responses.go`).
3. Add compaction checkpoint persistence in `internal/session/store.go`.
4. Wire lifecycle in REPL (`handlers.go`, `headless_run.go`, `repl.go`).
5. Update system prompt with compaction instructions.

### Safety checks

- Auto-compaction triggers when context exceeds configured threshold.
- Compaction checkpoints survive session restart.
- Does not interfere with Agent's existing session resumption (`--resume`).

## Bucket 7 — UI / REPL improvements

### Commits of interest (representative list)

- `6b7e7cb feat(repl): refine initial screen UI`
- `6282f8e feat(repl): replace loading spells with did-you-know tips and add shimmer effect`
- `a3645e0 feat(repl): polish startup screen with last-session hint and rotating tips`
- `6451e88 feat(repl): polish input UX with dynamic height and up-arrow nav`
- `57b33ff feat(repl): improve input metadata and suggestions`
- `460a50d feat(repl): move mode chip to input border and add plan mode styling`
- `675dbe0 feat(repl): refresh status bar glyphs and layout`
- `f87e7de feat(repl): show git branch and provider in location line`
- `e2c2a3b feat(repl): redesign tool status display with friendly labels and metadata`
- `6c501f1 feat(repl): streamline provider and tool output prompts`
- `6d8ded1 feat(repl): use Atom One Dark colors for markdown code highlighting`
- `5dd6eb7 feat(markdown): render table row rules safely`
- `b118052 feat(repl): make URLs in output clickable`
- `e13f396 feat(repl): copy selection on mouse release`
- `3548bf5 feat(history): implement input history navigation`
- `6d0514e feat(repl): queue user inputs while agent is streaming`
- `313ffae feat(repl): show loading spinner for bash and bang commands`
- `6cf2dc7 feat(repl): make ! shell commands async with streaming and cancellation`
- `b759396 feat(repl): add ! prefix for direct shell command execution`
- `a05ebb8 feat(repl): interrupt active work on first Ctrl+C, exit on second`
- `31abc27 feat(repl): add thinking display toggle`
- `ad5f32c feat(repl): stream live progress in headless run`
- `1ef20ad feat(repl): add completion signal for loop control`
- `0b199ac feat(repl): streamline model selection`
- `b97d410 feat(repl): clarify displayed search patterns`
- `1333741 feat(repl): dim tool search path separators`
- `fccf4e4 feat(repl): continue queued prompts after interruption`
- `3ca0d13 feat(repl): show sanitized subagent tool activity`

### Work

1. Port `internal/cli/repl/output/output.go` tool-status redesign.
2. Port `internal/cli/repl/widgets/model_selection.go` changes.
3. Add `internal/cli/repl/git_branch.go` + test for location-line git status.
4. Update `internal/cli/repl/theme/markdown.go` for Atom One Dark colors.
5. Add `internal/cli/repl/markdown/hyperlink.go` and `internal/cli/repl/urldetect/` for clickable URLs.
6. Port `internal/cli/repl/history/history.go` + test for input history.
7. Update `repl.go`, `repl_helpers.go`, `command_handlers.go`, `handlers.go`, `stream_*.go` for new UI behaviors.
8. Add `headless_progress.go` for live headless progress.
9. Port `stream_ask_user.go` if it is required by the updated REPL (check dependencies).

### Safety checks

- `go test -race ./internal/cli/repl/...` passes.
- Interactive REPL smoke test: startup, input, tool call, URL click, history, `/model`, `/compact`, `/tool-history`, `/thinking`, `/clear`, Ctrl+C behavior.

## Testing strategy

After every bucket:

```bash
go mod tidy
gofmt -w <modified .go files>
go test -race ./...
```

If a bucket is too large, split it into sub-buckets and run tests between each.

### Regression checklist

Before declaring the port complete, verify:

- `keen-agent --agent <cfg> validate` works.
- `keen-agent repl --agent <cfg>` starts.
- Built-in tool gating (`builtin_tools.exclude`) still hides excluded tools.
- MCP and delegate tools are still gated behind configured directories.
- Adversary and BTW helpers still function.
- Agent model selection and fallback warnings still work.
- `--resume` restores a previous session.
- `api_key_helper` credential resolution still works.

## Branch / commit strategy

- Create a long-running feature branch `feat/port-keen-code-updates`.
- One commit per bucket, message format: `feat(category): description`.
- Do **not** add co-authors or AI-made tags.
- Keep Agent-only features out of the diff unless the port directly touches them.

## Rollback plan

- Tag the starting commit as `pre-keen-code-port`.
- If any bucket cannot be stabilized within a reasonable time, revert that single bucket commit and continue with the next.
- Maintain a `PORTING_NOTES.md` file during the work to record decisions, conflicts, and dropped features.

## Known risks and mitigations

| Risk | Mitigation |
|------|------------|
| `openai-go/v3` API differs from v1 | Port the whole `0be31f0` diff as one unit and run all OpenAI-path tests. |
| REPL state shape diverged | Port UI commits incrementally; adapt to Agent's `replModel` rather than replacing it. |
| `internal/agentconfig` conflicts with Keen Code config changes | Keep Agent's `agentconfig`; only adopt Keen Code config fields that are orthogonal (e.g., thinking, provider-specific model config). |
| Compaction changes message history shape | Update session store/projection carefully; add migration tests. |
| Provider registry refresh drops Agent-only models | Diff the two registries before applying; preserve Agent-only IDs. |
| Go version bump breaks something | Pin to 1.25.13 only after CI and local tests pass. |

## Out of scope (for this plan)

- New tools (`ask_user`, `atomic_write`, `hashline` edit mode, paginated `read_file`, large `web_fetch` spill).
- Telemetry (`internal/telemetry`).
- Subagent orchestration improvements.
- Documentation site / README / npm wrapper rebranding.
- Security/CI-only changes (can be done separately).

These can be added in follow-up plans once the scoped port is stable.

## Next action

Create branch `feat/port-keen-code-updates` and begin with **Bucket 1** (dependency baseline) and **Bucket 2** (provider/model registry).

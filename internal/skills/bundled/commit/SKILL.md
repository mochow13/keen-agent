---
name: commit
description: Inspect git status, draft a commit message that matches the repo's style, and commit staged changes.
---

# Commit Skill

User intent (optional): $ARGUMENTS

## Steps

1. **Survey the working tree.** Run in parallel:

   - `git status --short`
   - `git diff --staged`
   - `git diff`
   - `git log -n 5 --oneline`

   Use the log to learn the repo's commit-message style (conventional commits, plain sentences, scope prefixes, etc.).

2. **Decide what to stage.**

   - Stage tracked changes the user clearly intends to commit. Name files explicitly with `git add <path>`.
   - If there are untracked files, list them to the user and ask which (if any) should be included. Do not stage them silently.
   - Do not use `git add -A` or `git add .` — they can leak secrets or stray files.

3. **Draft the message.**

   - Lead with *why* over *what*. The diff already shows the what.
   - Match the recent `git log` style.
   - Subject line under ~72 characters. If a body is needed, separate with a blank line.
   - If the user provided intent in the line above, weave it in — do not quote it verbatim.
   - Do not add co-author or "generated with" trailers unless the user explicitly asks.

4. **Commit.** Use a heredoc so multi-line bodies survive shell quoting:

   ```bash
   git commit -m "$(cat <<'EOF'
   <subject>

   <optional body>
   EOF
   )"
   ```

5. **Verify.** Run `git status` and report the result to the user. Do not push — that is a separate, explicit step the user must request.

## On hook failures

If a pre-commit hook fails, do not retry with `--no-verify`. Read the hook output, fix the underlying issue, re-stage the affected files, and create a new commit. Never amend in this case — the prior commit did not happen, so `--amend` would target the wrong thing.

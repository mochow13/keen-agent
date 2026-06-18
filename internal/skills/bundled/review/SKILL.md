---
name: review
description: Review current code changes for bugs, regressions, test gaps, and project-guideline violations.
---

# Review Skill

Review focus (optional): $ARGUMENTS

## Steps

1. **Gather change context.** Run in parallel:

   - `git status --short`
   - `git diff --staged`
   - `git diff`
   - `git ls-files --others --exclude-standard`
   - `git log -n 3 --oneline`

   Review both staged and unstaged changes. Treat untracked files as relevant when
   they appear related to the user's request, and open them directly because
   `git diff` will not show their contents.

2. **Read project conventions.** If `AGENTS.md`, `CLAUDE.md`, or
   `CONTRIBUTING.md` exist at the repo root, read them before judging the diff.

3. **Inspect the changed code.** Open the files and surrounding code needed to
   verify behavior. Do not rely only on the diff when control flow, data shape,
   or tests depend on nearby context.

4. **Review against these checks:**

   **Correctness**

   - No behavior regressions, missed edge cases, or undefined references
   - Error handling matches the risk and existing project patterns
   - Public contracts, CLI behavior, file formats, and compatibility are preserved unless the user asked to change them
   - Imports, variables, or functions made unused by the changes are removed

   **Surgical Changes**

   - Every changed line traces to the user's request
   - No "improvements" to adjacent code, comments, or formatting
   - No refactored code that wasn't broken
   - No deleted pre-existing dead code unless explicitly asked

   **Simplicity**

   - No speculative features or abstractions
   - No unnecessary configurability
   - No error handling for impossible scenarios
   - Functions are as short as they can be while remaining clear

   **Testing**

   - Critical paths have tests
   - New functionality is exercised by tests
   - Existing tests still cover the changed behavior
   - Relevant tests pass, or any unrun tests are called out with the reason

   **Style**

   - Matches existing naming and patterns
   - Minimal comments (only when strictly necessary)
   - Language-specific formatting applied (e.g., `gofmt`)

5. **Report findings first.**

   - Start with bugs, regressions, security issues, and missing critical tests.
   - Include precise `file:line` references for every finding.
   - Order findings by severity: **blocker**, **warning**, then **nit**.
   - Explain the impact and the smallest practical fix.
   - If there are no findings, say so clearly and mention any residual test risk.

6. **Do not apply fixes yourself.** Present the review and let the user decide
   what to fix.

## Output Format

Use this structure:

1. Findings
2. Open questions or assumptions
3. Test coverage and commands run

Keep summaries brief. If there are no findings, the first line should state that
no issues were found.

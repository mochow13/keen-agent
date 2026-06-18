---
name: clarify
description: Ask focused clarifying questions to resolve ambiguous requirements before planning or implementing work.
---

# Clarify Skill

Request to clarify (optional): $ARGUMENTS

## Steps

1. **Restate the goal.** Briefly summarize what the user appears to want in
   concrete terms. Separate confirmed facts from assumptions.

2. **Find the gaps.** Check for missing or ambiguous requirements across the
   areas that matter for the request:

   - Goal and success criteria
   - Scope, non-goals, and acceptable tradeoffs
   - Inputs, outputs, data formats, and examples
   - User-facing behavior, edge cases, and error handling
   - Technical constraints, integrations, dependencies, and environments
   - Compatibility, migration, performance, security, and privacy concerns
   - Verification expectations, tests, rollout, and delivery format

3. **Ask only important questions.**

   - Ask questions that can change the implementation, design, risk, or final
     answer.
   - Group related questions so the user can answer efficiently.
   - Prefer specific questions with clear choices when the options are known.
   - Avoid asking about details that can be discovered from the repo or inferred
     safely from existing project conventions.

4. **Keep momentum.** If reasonable defaults exist, state them after the
   questions so the user can accept them quickly. If the request is safe to
   proceed with assumptions, name the assumptions and continue only when the
   user has asked you to proceed that way.

## Output Format

Use this structure:

1. Understanding
2. Questions
3. Proposed defaults

Keep the response concise. Do not implement, edit files, or run destructive
commands while using this skill unless the user answers the questions or
explicitly asks you to proceed with stated assumptions.

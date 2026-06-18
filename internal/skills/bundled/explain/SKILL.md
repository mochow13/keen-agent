---
name: explain
description: Explain code, behavior, architecture, or diffs with concrete references to the relevant files.
---

# Explain Skill

Topic to explain (optional): $ARGUMENTS

## Steps

1. **Locate the subject.** Identify the files, functions, commands, tests, or
   diff hunks relevant to the user's question.

2. **Read enough context.** Inspect surrounding code, call sites, tests, and
   configuration needed to explain behavior accurately.

3. **Explain from evidence.**

   - Start with the direct answer.
   - Reference concrete files, symbols, commands, or data flow.
   - Distinguish facts from inferences.
   - Call out important edge cases, dependencies, or assumptions.

4. **Keep scope tight.** Do not refactor, edit files, or run broad commands
   unless the user asks for implementation work.

## Output Format

Use concise prose with file references when useful. For complex behavior, include
the execution path or data flow in the order it happens.

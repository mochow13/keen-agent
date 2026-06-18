---
name: refactor
description: Refactor existing code while preserving behavior, keeping changes scoped, and verifying with relevant tests.
---

# Refactor Skill

Refactor goal (optional): $ARGUMENTS

## Steps

1. **Clarify the target.** Identify the specific code, behavior, or design issue
   the user wants improved. If the goal is broad, choose the smallest useful
   scope and state that assumption before editing.

2. **Establish current behavior.** Read the relevant code and tests before
   changing anything. Prefer existing abstractions and local patterns over new
   ones.

3. **Plan a behavior-preserving change.**

   - Keep public APIs, CLI behavior, file formats, and user-visible output
     unchanged unless the user explicitly asked to change them.
   - Avoid speculative abstractions, new configurability, or unrelated cleanup.
   - Split risky work into small steps that can be verified independently.

4. **Edit surgically.**

   - Move, rename, or simplify code only when it directly supports the refactor
     goal.
   - Remove imports, variables, helpers, or tests only when the refactor makes
     them obsolete.
   - Preserve nearby style, naming, and error-handling patterns.
   - Add comments only when the refactor would otherwise make intent harder to
     understand.

5. **Verify behavior.**

   - Run the narrowest relevant tests first.
   - Run broader tests when shared behavior, public contracts, or cross-package
     code changed.
   - Run required formatters and dependency tidy commands for the project.
   - If tests cannot be run, report the exact blocker and residual risk.

6. **Report the result.** Summarize what changed, why it remains behavior
   preserving, and which verification commands passed.

## Guardrails

- Do not refactor unrelated code discovered along the way.
- Do not combine formatting-only churn with semantic changes unless formatting
  is required by the touched language or project.
- Do not leave compatibility assumptions implicit; call them out when they
  affected the approach.

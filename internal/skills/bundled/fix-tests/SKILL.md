---
name: fix-tests
description: Diagnose failing tests, determine whether code or tests are wrong, and make the smallest passing fix.
---

# Fix Tests Skill

Failing test or command (optional): $ARGUMENTS

## Steps

1. **Run or read the failure.** Capture the failing command, package, test name,
   assertion, stack trace, and relevant logs.

2. **Identify intent.** Read the failing test and nearby production code to
   determine the behavior being specified.

3. **Choose the correct fix.**

   - Fix production code when the test exposes a real regression.
   - Fix the test when expectations are stale, flaky, or inconsistent with the
     intended behavior.
   - Do not weaken assertions just to make the suite pass.

4. **Patch narrowly.** Change only the files needed for the failing behavior.
   Preserve existing test style and helper patterns.

5. **Verify.** Rerun the failing test first. Then run the relevant package or
   full suite when the fix touches shared behavior. Run required formatters and
   dependency tidy commands for the project.

6. **Report.** State which tests failed, what caused the failure, what changed,
   and which commands now pass.

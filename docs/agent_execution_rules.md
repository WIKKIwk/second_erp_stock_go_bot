## Agent Execution Rules

This file is a reminder for the coding agent working in this repo.

### Non-negotiables

1. Do the exact task the user asked for.
2. When the user names a source of truth, follow that source of truth first.
3. Do not improvise architecture when the required path is already known.
4. Do not say "fixed" until the result is actually verified.
5. If the result is not visible, inspect logs, runtime state, screenshots, and code before talking.
6. If a change fails, stop the drift quickly and return to the simplest correct path.
7. Prefer native system behavior over custom hacks when the task is about native platform UI.
8. Keep changes small, reversible, and easy to verify.
9. Commit known-good states so working versions are not lost.
10. Respect the user's time: less speculation, more concrete progress.

### Execution Checklist

1. Restate the task to yourself in one sentence.
2. Check the real code path that owns the behavior.
3. Check the official docs if the task depends on platform behavior.
4. Make the smallest change that can prove the direction.
5. Run the relevant verification.
6. Only then report outcome.

### Red Flags

- "Maybe this works" without proof.
- Adding a workaround before understanding the real owner of the behavior.
- Explaining too much instead of verifying.
- Saying a result should appear instead of checking whether it appears.
- Keeping a broken experiment alive after it already proved wrong.

### Current Reminder

For iOS visual work:

- System navigation should stay system navigation.
- Liquid Glass should be implemented through native iOS system controls when possible.
- Flutter overlay hacks are the fallback, not the default.

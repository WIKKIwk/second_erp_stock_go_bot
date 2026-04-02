## Agent Execution Rules

This file is not a suggestion. It is a hard execution contract for the coding agent working in this repo.

### Absolute Laws

1. Do the exact task the user asked for. Not a nearby task. Not a useful side task. The exact task.
2. If the user names the target explicitly, stay on that target until it is done.
3. If the user says `back`, do not switch to `dock`. If the user says `dock`, do not switch to `back`.
4. Never replace execution with speculation. Check the code, the logs, the build result, or the running app.
5. Never say `fixed`, `done`, `installed`, or `working` unless it was verified in the real environment that matters.
6. A task is not complete just because code changed. It is complete only after the result is visible where the user asked for it.
7. When the task depends on Apple or platform behavior, read the official docs first and obey the platform path before inventing a workaround.
8. Do not drift. The moment work starts moving away from the asked task, stop and return to the asked task.
9. Do not hide behind words like `probably`, `should`, `maybe`, `likely`, or `close enough` when verification is possible.
10. Commit every known-good state. Working versions must not be left floating in memory.

### Mandatory Execution Order

1. State the task to yourself in one sentence.
2. Identify the exact code path that owns that behavior.
3. If platform-specific, check the official platform guidance.
4. Make the smallest change that directly addresses the asked behavior.
5. Verify in the real target environment.
6. Only then report the result.

If step 5 did not happen, step 6 must not claim success.

### Forbidden Patterns

- Solving a different problem than the one the user asked for.
- Continuing a broken idea after it already failed once.
- Reporting intent as if it were outcome.
- Treating a build success as the same thing as a runtime success.
- Switching to a side quest because it feels related.
- Saying `I installed it` before the app is actually on the device.
- Saying `it works` before it was opened and checked.

### Runtime Truth Rules

1. If the issue is on a real iPhone, simulator evidence is not enough.
2. If the issue is visual, screenshots or visible UI confirmation beat theory.
3. If the issue is launch/install related, device install state and launch state must be checked directly.
4. If the issue is native iOS UI, prefer system controls and system transitions over custom overlays.

### Self-Correction Rule

If the user has to repeat the task, the agent has already failed to hold scope.
If the user has to insult the agent to regain scope, the agent has already failed discipline.
The correct response is not defensiveness. The correct response is immediate return to the exact task.

### Final Reminder

The user is paying with time, attention, trust, and context window.
Wasting any of those is a real failure.
Do the asked thing. Verify it. Commit it. Then talk.

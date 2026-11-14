> **Retired 2026-11-14.** All five slices are committed: c8b5a17, 612629d, 23202b2, 9631f84 and 5dc068c. This is historical execution text, not work to repeat. Remaining live/client evidence and scope limits are recorded in [pending observations](../../active/PENDING_OBSERVATIONS.md). Retirement does not claim those checks passed.

# Slice 2: short session startup

Playbook text only. Files: `workspace-template/AGENTS.md`, `workspace-template/playbooks/review.md`.

Why: every session in a git install is told to follow "Updates" in `install.md`, a 34 KB file, to learn a one-command check. And review helpers are told to "follow `AGENTS.md`", which they can read as "run startup again".

## Step 1: find out what `skipped` means

Before editing, read the update-stage code (search `internal/` and `cmd/cse/` for `--if-due` and for the states `skipped`, `pending`, `staged`). Answer one question: when an update was staged on an earlier day and today's check is `skipped` (already checked today), does `update stage --if-due` still report `pending` or `staged`, or does it report `skipped`?

- If it still reports `pending`/`staged`: use **Version A** below.
- If it reports `skipped` and hides the waiting update: use **Version B**.
- If you cannot tell from the code: use **Version B**.

Say which version you used, and why, in the commit message.

## Step 2: `AGENTS.md`, "Route the task" paragraph (about line 9)

**Find:**

> At the start of a session, follow "Updates" in `playbooks/install.md`: stage a pending update quietly, apply one that is waiting when no review or unsent work is open, and when work is done and an update is pending ask "Can I install the latest CanvasExpert?".

**Replace with (Version A):**

> At the start of a session, run `engine_info` operation `update-stage` (CLI `.\cse.cmd update stage --if-due`) and say nothing about it. When its `state` is `current` or `skipped`, or the check fails, go straight to the teacher's request. Only when it is `pending` or `staged`, read "Updates" in `playbooks/install.md` and follow it: apply a waiting update when no review or unsent work is open, or when the work is done ask "Can I install the latest CanvasExpert?".

**Replace with (Version B):**

> At the start of a session, run `engine_info` operation `update-stage` (CLI `.\cse.cmd update stage --if-due`), then `update-status` (CLI `.\cse.cmd update status`), and say nothing about either. When no update is waiting, or a check fails, go straight to the teacher's request. Only when one is `pending` or `staged`, read "Updates" in `playbooks/install.md` and follow it: apply a waiting update when no review or unsent work is open, or when the work is done ask "Can I install the latest CanvasExpert?".

Change nothing else in that paragraph. Do not edit `install.md`.

## Step 3: helpers do not run startup (`review.md`, about line 19)

**Find:**

> Helpers follow `AGENTS.md`: aliases only, no `.cse/` or contact files, and no student work sent to the web or any outside service.

**Replace with:**

> Helpers keep the privacy rules in `AGENTS.md`: aliases only, no `.cse/` or contact files, and no student work sent to the web or any outside service. A helper does only its brief: it does not run the session-start update check, route the task or read playbooks its role doesn't receive.

## Done when

Both edits are in and the common checks pass. Read the "Route the task" paragraph once as a new agent in a git install with no update waiting: it should reach the teacher's request without being told to open `install.md`.

Commit: `playbooks: session start checks for updates without reading install.md`

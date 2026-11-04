# HANDOFF C: Family review keeps revision notes after reconnect

Date: 2026-11-04. Slice C of the [unified architecture review](../Unified%20Architecture%20Review.md). Layer: **Engine / MCP adapter** (`internal/mcpserver`). Small, bounded fix.

Before starting: `git fetch`, bring `dev` up to date, read root `AGENTS.md`, [docs/family-contact-flow.md](../../docs/family-contact-flow.md) and the [shared execution notes](../README.md#execution-notes-for-the-active-handoffs). Facts mapped at `2818e36`.

## Why

A teacher reviews family emails (or a grade-update batch) on the private review page and leaves revision notes such as "make card 3 shorter". This scenario loses them:

1. The agent app restarts or reconnects while the page is still open. A new MCP adapter calls `start_parent_review` again.
2. The adapter correctly finds the open page and reconnects to it.
3. When the page closes, `review_status` returns **no revision notes**, though they are saved on disk.

The agent then cannot act on the teacher's notes. Nothing is sent wrongly; the teacher's requested changes are just lost from the agent's view.

## Current code (bug confirmed present)

- `start_parent_review` (`internal/mcpserver/server.go:302–308`) resolves the run **folder** (`dir`) and calls `s.startReviewSession(ctx, dir, []string{"parent-review", dir})`.
- The child `cse parent-review <dir>` (`cmd/cse/remote.go:1001–1006`) reviews `workflow.ParentReviewSource(dir)` (`internal/workflow/family_review.go:22–27`), which returns:
  - `<dir>/contact.json` when it exists (family contact);
  - otherwise `<dir>/drafts.json` (grade update).

  The page marks itself with `hubcoord.MarkPage(root, "review", origin, path)` using that **file** (canonicalized, `hubcoord/pages.go:38`). Review state is saved to `<file>.review.json` (`localui/review.go:217,261`). Send outcomes go to `<dir>/outcomes.json`.
- **Reconnect** (`internal/mcpserver/sessions.go:56–73`): `hubcoord.FindPage(s.root, "review", subject)` matches the folder as a prefix of `rec.Subject` and builds `reviewTask{..., subject: subject /* the folder */, external: true}`. `rec.Subject` (the actual file) is ignored.
- **Status after close** (`sessions.go:472–489`): `result.ResultPath = task.subject` (the folder), then `workflow.LoadReviewState(task.subject)` reads `<folder>.review.json` (`internal/workflow/review.go:85–104`). That file never exists, and `LoadReviewState` returns empty state with a nil error, so the miss is silent. The text at ~487 also says `outcomes.json` is "for family email" only, but grade updates write it too.
- **Fresh start** (`startSession`, `sessions.go:129`) never sets `subject`. It gets `Outcome`, `ResultPath` (`<dir>/outcomes.json`) and `RevisionNotes` from the child's `Review result: {...}` stdout line. Only the reconnect path is affected.

## What to build

1. On reconnect, set `task.subject` to `rec.Subject` (the reviewed file), not the folder the caller passed.
2. After close, report a `ResultPath` consistent with the fresh path: `outcomes.json` beside the reviewed file. Load review state from `rec.Subject` (so `<file>.review.json`).
3. Fix the status wording so it does not say `outcomes.json` is family-email only.
4. Leave the private send decision, origin/secret checks, approval digests and the family-review page unchanged.

## Done when

- **New test** in `internal/mcpserver/sessions_test.go`, beside `TestStartReviewReconnectsToPageLeftOpenByEarlierAdapter` (which uses a file-shaped subject, so it cannot catch this bug). Build a folder-shaped fixture:
  1. create `root/courses/<course>/family-contact/<run>/contact.json`;
  2. `hubcoord.MarkPage(root, "review", url, workflow.ParentReviewSource(dir))`;
  3. `SaveReviewState(ParentReviewSource(dir), …)` with a revision note;
  4. call `startReviewSession(ctx, dir, …)`;
  5. close the page;
  6. assert `review_status` returns the note and `ResultPath` ends in `outcomes.json`.

  Repeat with `drafts.json` (grade update). Use only exported `workflow` helpers; the richer family fixtures are test-only in package `workflow`.
- Existing session tests still pass. Run the full check list from the execution notes.
- Capability records need no change unless they describe reconnect behavior; check `family-contact-flow.md` and `capabilities.go` for a reconnect claim.

## Not in this slice

Cross-PC continuation of a review session (out of scope by decision 4), new review features, any real family send.

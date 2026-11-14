# Slice 5: list a scoring session before pulling

Engine code plus one playbook paragraph. This is the only slice with Go changes. Read `docs/ARCHITECTURE.md`, `docs/SCORING-RULES.md` and [the pull-isolation handoff](../HANDOFF_Scoring_Pull_Isolation_2026-11-14.md) first; its behavior must be preserved.

Why: when the teacher says "scoring session" without naming an assignment, the agent pulls every submission, quiz report and upload that needs grading, then asks what to score. The developer decided the session should list first and pull only what is chosen.

## What to build

`pull-work [COURSE] --session --list` (MCP `pull_work` with `session: true, list: true`) reports which assignments have work to score and downloads nothing.

Rules:

- `--list` requires `--session`. Reject `--list` with an ASSIGNMENT argument or with `--group`, with a plain usage error.
- It makes the same assignment-discovery requests a session pull already makes, and none of the per-assignment submission, quiz-report or file requests. It writes no packet and no other file.
- Course scope, team routing and the teammate `(CE)` rule apply exactly as in a session pull. Do not add a second code path for them.
- No new command, cache or saved state. After the list, the agent runs the ordinary `pull-work COURSE ASSIGNMENT --session` for each chosen assignment.

## Where

Read these fully before editing:

- `internal/workflow/pull_work.go`: `PullWorkOptions` (about line 37), `PullAssignmentSummary`, and `pullWork` (about line 158). In `pullWork`, each course's assignment rows are built with their `NeedsGradingCount` and due state (about lines 319 to 345) **before** `detailedSessionSubmissions` is called (about line 346). That call is the first per-assignment fetch. List mode ends each course's work before it.
- `cmd/cse/remote.go`, `case "pull-work":` (about line 1112): flag parsing and output.
- `cmd/cse/session_report.go`: `printSessionReport`, the session table.
- `internal/mcpserver/server.go`: `PullWorkInput` (about line 82) and the `pull_work` tool (about line 294).

If the seam is not as described (for example, rows are not built before the fetch), stop and report what you found. Do not restructure `pullWork`.

## Steps

1. Add `List bool` to `PullWorkOptions`.
2. In `pullWork`, when `options.Session && options.List`: after the course's rows exist and before `detailedSessionSubmissions`, give each row a final outcome and continue to the next course.
   - Use a new outcome value `listed` for an assignment with work to score or an unknown count.
   - Apply the two skips the session pull already applies without fetching submissions, with their existing `SkipReason` text: Canvas reports zero needing grading, and a closed grading period. Find how the existing loop decides each (about lines 357 to 365). If either decision needs a per-assignment request, leave that row `listed` instead and say so in the commit message.
   - No row may be left `not-reached` on a successful list, and a successful list must not set `report.Partial`.
   - A course whose assignment discovery fails keeps its existing failure reporting.
3. CLI (`remote.go`): parse `--list`; enforce the rules above; pass `List`. Update the usage string and the `pull-work` line in the help text in `cmd/cse/main.go`.
4. Output: in list mode print the session table without the Eligible, Pulled and Timing candidates columns (they are not known), keeping Course, Assignment, Title, Group (weight), Due state, Canvas needs grading, Outcome, Details. Then one line: `Listed only; nothing pulled.` Do not print the "N students in N packets" line. Keep the existing collapsing of "no work needing grading" rows.
5. MCP (`server.go`): add `List bool` with JSON name `list` and a one-line description ("With session: list assignments that have work to score without pulling anything"); return an error when `list` is set without `session`, or with `assignment` or `group_id`; append `--list`. Add a short clause to the `pull_work` tool description.
6. Tests, two only:
   - In `internal/workflow` (follow `targeted_session_test.go` or `session_discovery_test.go` for the fake Canvas server): a synthetic course with two assignments needing grading and one with zero. Assert list mode returns the three rows with the right outcomes and counts, the server saw **no** submission, quiz-report or file requests, and no packet file exists.
   - One CLI or MCP argument test: `--list` without `--session`, and with an assignment, are rejected.
7. Docs: add the option to the pull-work entry in `docs/engine-capabilities.md` and the matching entry and review date in `internal/localui/capabilities.go`. Mark it as covered by local tests only; do not describe it as checked on Canvas Live.

## Playbook (`workspace-template/playbooks/score.md`, "## Pull")

**Find:**

> A bare “scoring session” means pull everything Canvas reports as needing grading, present the assignment list, then ask what the teacher wants scored. If the teacher already narrowed the request, use the report's assignment group and due-state columns to apply that scope with common sense.

**Replace with:**

> When the teacher asks for a scoring session without naming an assignment, for one class or all of them, list first: add `--list` (MCP `list: true`). It shows each assignment that has work to score, with its group, due state and Canvas's needs-grading count, and downloads nothing. Show the teacher that list, ask what they want scored, then pull only those assignments, each with `--session`. When the teacher named the assignment, or asked to score everything, skip the list and pull. Use the list's assignment group and due-state columns to apply a narrowed request (“just the Majors”, “only what's due”) with common sense.

The quotation marks in the Find text are curly (“ ”) in the file. If the text does not match, search for `A bare` in that paragraph.

Do not change the seed Home command in `workspace-template/context/commands.md` ("Start a scoring session for {course}"); under the new wording it lists that class first.

## Done when

- `go test ./...` passes, including the two new tests.
- `go run ./cmd/cse pull-work --list` fails with the usage error (no `--session`).
- An ordinary `pull-work --session` behaves exactly as before: the existing session tests pass unchanged.
- The common checks in the README pass.

Commit: `feat: list a scoring session's assignments before pulling`

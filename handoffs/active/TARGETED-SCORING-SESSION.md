# Targeted scoring session: execution brief

Prepared 2026-10-21 against Dev branch `dev`, revision `8551564`. Engine work with matching seed-playbook and rule-document updates. Nothing here is implemented yet. This brief does not authorize Canvas writes, release, or UserInstall/UserSync adoption.

Read root [AGENTS.md](../../AGENTS.md), [ARCHITECTURE.md](../../docs/ARCHITECTURE.md), [SCORING-RULES.md](../../docs/SCORING-RULES.md) and the seed [score.md](../../workspace-template/playbooks/score.md) before starting.

## Result

When the teacher asks for a scoring session, `pull_work --session` pulls only the work Canvas says needs grading. It returns a short report covering each assignment's group (Daily, Major Grade, …), due state and counts. Today it lists submissions for every assignment in every current course and rebuilds timing evidence for already-graded work. This changes the session pull only. Explicit assignment pulls (`pull-work COURSE ASSIGNMENT`) keep their reassessment behavior: they include all genuine supported submissions.

## Why: the 2026-10-21 session

A Sonnet Cowork session scored "all Daily grades" across ELA7 120638, PAP ELA7 120669 and CS8 121046. Its handoff was provided in chat; only counts and IDs are recorded here.

- The session pull took about 2m47s and was polled six times. It printed a 520-row (~70 KB) table, mostly zeros, for 12 packets and 129 students.
- The agent downloaded ~770 KB of `assignment_groups` JSON with `canvas_get` only to tell Daily from Major Grade.
- Three packets were timing-only (CS8 3697517, CS8 3698355, most of PAP 3697804). Each got its own subagent, and none produced a write. The already-graded timing path held 10 students for "no matching accepted CE-SE timing provenance".
- Every late-timing edit that posted (4) was on newly scored work (`score-and-late`).
- Three packets were not yet due and were skipped by hand (CS8 3703860, ELA7 3705777, PAP 3705779).
- The first session pull failed in 2 ms because `context/school-calendar.md` had no grading periods.

## Evidence already observed

**Canvas Live, read-only, 2026-10-21.** `GET /api/v1/courses/121046/todo?per_page=10` (CS8) returned two `type: grading` items, each with an embedded assignment carrying `needs_grading_count`, `all_dates` and `assignment_group_id`:
- 3698355 Hotline Miami: `needs_grading_count: 1`. The session packet had treated its 2 students as late-only. Unexplained: possibly a submission after the pull. Reconcile during implementation, not by assumption.
- 3703860 Stardew Valley: `needs_grading_count: 20`. Due 2026-10-21T21:00:00Z, so not yet due at the time.
- 3697517, the timing-only packet, was absent, consistent with it needing no grading.

This is one course on one day. It does not establish the field's behavior for New Quizzes, graded discussions, group assignments or resubmissions.

**Code, at this revision.**
- `Assignment` in `internal/workflow/canvas_reads.go:43` has no `NeedsGradingCount` or `AssignmentGroupID`.
- `Assignments()` (`canvas_reads.go:322`) lists without `include[]=all_dates`. Only `GetAssignment` asks for it.
- Discovery in `pullWork` (`internal/workflow/pull_work.go:248-326`) lists submissions for every assignment serially, then filters by period.
- The detailed fetch is one `GetSubmission` per candidate (`fetchSubmissions`, `pull_work.go:866`).
- Candidate checks walk and parse the whole workspace per student: `completedHint` (`pull_work.go:744`), `completedTimingHint` (`session_discovery.go:24`), and `sessionTimingCandidate` → `acceptedTimingRecordExists` (`late_prepare.go:544`).
- No pull check recognizes a completed `comment-only` result. New Quiz written work therefore reappears in every session until the teacher enters item points in SpeedGrader.
- The pull report table is printed in `cmd/cse/remote.go:556`.

## Decisions to confirm before coding

The recommendations are the default. The developer confirms each one, or changes it, before the owning step lands.

1. **Late-timing re-checks on already-graded work become opt-in.** SCORING-RULES.md and ARCHITECTURE.md currently say every broad session prepares timing for "already-graded work" too. Recommended: the default session skips it, and a flag (proposed `--graded-timing`, MCP `graded_timing: true`) restores today's behavior. Newly scored work still gets `score-and-late` preparation. Update both rule documents and `score.md` in the same change.
2. **Not-yet-due work is reported, not pulled.** A student's work is pulled only when their effective due date has passed. Early submissions show as a counted "not due yet" row. An explicit assignment pull still includes them.
3. **Missing grading periods no longer block a session.** With periods configured, keep today's per-student current-period filter. Without them, use needs-grading plus past-due, and print one line saying the period filter was skipped and how to add periods. Add grading periods to the install/setup checklist in `setup.md`.
4. **Signal source.** Use `needs_grading_count` from the course assignment list the Engine already reads, not the To-Do endpoint. The To-Do list drops items the teacher dismissed with "ignore", so it can hide real work. The assignment field costs no extra request. The Engine never calls the To-Do `ignore` URLs.

## Work, in order

### 1. Targeted discovery (Engine)

- Add `NeedsGradingCount *int` and `AssignmentGroupID *int64` to `Assignment`. Add `include[]=all_dates` to `Assignments()`.
- In session mode with no explicit assignment, apply these rules in order:
  1. Skip assignments whose count is present and 0, unless decision 1's flag is on.
  2. Treat an absent count as unknown. Fall back to today's discovery for that assignment, and say so once in the report, not once per student.
  3. If `all_dates` shows every date in the future, report the assignment as not yet due without listing submissions.
- For each remaining assignment, make one detailed submissions list (`Submissions(..., true)`) and filter it with the existing `liveHandled`, visibility, excused, enrollment and period rules. This replaces the discovery list plus per-student `GetSubmission`. Keep the identity check that `fetchSubmissions` does today.
- Run the per-assignment lists concurrently. The client already caps in-flight requests at 4 and handles throttling.
- Group assignments keep the `pullGroupAssignment` path. Targeting only decides whether it is entered.

### 2. One results index per pull (Engine)

Build the index of local `results` and `group-results` rows once per pull, keyed by instance, course and assignment. Answer `completedHint`, `completedTimingHint` and `acceptedTimingRecordExists` from it, with identical matching rules. Use the same index for `acceptedTimingOrigin` in `prepare_late_timing`. Add a comment-only completion check: a succeeded `comment-only` row for the same student and attempt counts as handled for New Quiz session discovery. It still returns if the student submits a new attempt.

### 3. Assignment groups and the report (Engine)

- Read `/api/v1/courses/:id/assignment_groups` without embedded assignments: one small request per course.
- Print one row per assignment that needs attention, with these columns: course, assignment ID, title, group name and weight, due state (past due / not due yet / undated), Canvas needs-grading count, pulled count, and timing candidates. Print a single summary line for everything else.
- Report a mismatch between the Canvas count and the pulled count with its causes: inactive, not visible, excused, unsupported, undated, or outside the period. Count "effective due date is unavailable" as its own category instead of a log line per student.
- Add an optional group filter (proposed `--group NAME`, MCP `group`) so "score all Daily grades" needs no extra download.
- Add "n of N assignments" to the `pending` text from `longCommand` (`internal/mcpserver/pulls.go`) if the subprocess can report it cheaply. Otherwise leave it.

### 4. Guidance and records

- Update the seed `score.md` session section to match: a targeted default, decision 1's flag, the group filter and the not-yet-due row. Update the SCORING-RULES.md and ARCHITECTURE.md timing sentences.
- Do not overwrite teacher-modified installed playbooks.
- Update the scoring and calendar/late-timing rows in `internal/localui/capabilities.go` and [engine-capabilities.md](../../docs/engine-capabilities.md) with today's review date. Keep the line between the read-only CS8 observation above, local tests and pending live checks.

## Tests

Proportionate, using local fixtures. They need not be exhaustive.
- A zero-count assignment makes no submissions request.
- An absent count falls back to discovery.
- An all-future `all_dates` assignment is reported as not yet due.
- The detailed list replaces per-student fetches: compare request counts using the existing `Requests()` counter.
- The graded-timing flag restores today's candidates.
- A completed comment-only row suppresses the New Quiz re-pull, but a new attempt still pulls.
- Missing grading periods give the fallback line, not an error.
- The index returns the same answers as the current walkers on the existing regression fixtures.

## Live checks, after local tests

Read-only checks on CS8 first, then one teacher-run scoring session. These are evidence for the Capabilities page, not prerequisites for ordinary use.
- `needs_grading_count` for a New Quiz with a pending written item, before and after a comment-only post.
- A graded discussion with an ungraded entry.
- A resubmission after grading.
- A group assignment.
- Reconcile the CS8 3698355 count (1) against the 2-row late-only packet.

## Out of scope here

These were noted in the same review and belong to later work. Each needs its own decision.
- Posting's repeated full-submission re-reads (`post.go:175`, `:594`, `:628`; `late_timing.go:84`).
- Serial posting.
- `post_results` not using `longCommand`.
- `body_html` duplicated beside `readable_text` in student files.
- The redundant `validate` before `review` in `score.md`.
- The session handoff's other suggestions: a results scaffold (`results-init`), a saved per-assignment scoring basis, a `score_line` template, New Quiz item points in packets, private `teacher_flags`, adopting hand-set Canvas late status, discussion-entry first-genuine references, and a compact scoring brief for subagents.

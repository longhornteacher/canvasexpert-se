# Handoff: Late-work cutoff and personal dates (2026-11-10)

**Status:** Engine code is partly built in Dev and compiles (`go build ./...` clean at the last check). **No tests exist for the new code, no playbook or doc is updated, nothing is committed, nothing is checked in Canvas Live.** It sits on top of the uncommitted late-work reconciliation ([handoff](HANDOFF_Late_Work_Reconciliation_2026-11-10.md)); commit that first if the developer agrees.

## Policy (developer, 2026-11-10)

School policy:
- ELA7 and ELA7PAP: work more than 3 days late is an automatic zero.
- CS8 (high school credit): late work is not accepted at all.

Class policy:
- "Gloom" (a student alias) gets 50% more time, based on the assigned-to-due spread.

## Decisions (developer, 2026-11-10)

| # | Decision |
|---|---|
| P1 | "3 days" is **school days** (weekends and calendar closures don't count). |
| P2 | Enforcement is a **Canvas lock** at the cutoff (CS8: at the due date). Work never turned in gets its 0 from Canvas's missing-work policy. Whether that policy is on in the three courses is **not checked**. |
| P3 | Gloom's rule is mechanical: a percent saved on the Students page; the Engine gives the student a **personal Canvas due date**. "Assigned" is the Canvas "available from" date, else the creation day. Rounded up. |
| P4 | **Corrections keep their own window** (`correction_school_days`): the cutoff only stops first-time late turn-ins. |
| P5 | Because a lock also blocks corrections: **reopen for on-time**. After the due date, students whose work was on time get a personal lock date through the correction window. This needs a run after each due date; there is no scheduler. |

Assumed, not confirmed by the developer: absences still move a student's due date and cutoff.

## Built so far (compiles; untested unless noted)

- `internal/domain/types.go`, `validation.go`: `Course.LateWork` (per-course `late_work` in `context/courses.md`); new `LateWorkPolicy.LateCutoffSchoolDays *int` (`late_cutoff_school_days`: nil = no limit, 0 = no late work); `Merged` (course lines win over `context/grading.md`).
- `internal/workflow/timetable.go`: `CourseLateWorkPolicy`, `Service.lateWorkFor`; `planTimetable` now takes the course. With a cutoff the timetable shows "Late work accepted through" (or "Late work: not accepted after the due date", with no penalty row) and the default lock date is the regular cutoff, not the extra-time close date. Existing timetable tests pass; **no test for the cutoff rows**.
- `internal/workflow/author.go`: lock note reworded to "the timetable's close date". `engine-docs/formats.md` line 57 still says "extra-time close date".
- `internal/workspace/students.go`: new last column `extra_time_percent` (0-200; days and percent are exclusive), new header generation. Existing workspace tests pass.
- `internal/localui/students.go`, `templates/students.html`: percent box beside the days box. `internal/localui/courses.go`, `templates/courses.html`: "Late-work limit" column (blank = none). Existing localui tests passed before the Courses change; **not rerun since, no new tests**.
- `internal/workflow/personal_dates.go` (new): `Service.PersonalDates(ctx, course, assignmentID, preview, now)`. Per active student it plans a single-student Canvas override: extra time (days, percent, or general allowance), absent on the due date, lock following the student's own due date under a cutoff, and on-time work reopened through the correction window. Dates only move later; an existing personal due date is kept; a student in a shared override is held; group assignments are skipped; owned courses need the `(CE)` tag. Without an assignment ID it covers published assignments due in the last 21 days or later. `extraSchoolDays` is the shared allowance helper. **This file changed on disk after it was written; read it before editing.**
- `internal/workflow/late_sweep.go`: uses `extraSchoolDays` (so a percent works in the sweep) and the per-course policy.
- CLI `personal-dates COURSE [ASSIGNMENT_ID] [--preview]` (`cmd/cse/main.go`, `remote.go`), MCP `personal_dates` (`internal/mcpserver/server.go`), `scripts/check-mcp.mjs` tool count 65. **Not run.**

## Remaining

1. **Verify what is built.** `go build ./...`, `go vet ./...`, `go test -count=1 ./...`, temp build plus `node scripts/check-mcp.mjs`, `git diff --check`. Expect help/reference tests to want the new command documented.
2. **Tests** (fake Canvas server, modelled on `late_sweep_test.go`):
   - timetable: cutoff 3, cutoff 0, teacher-set lock date, per-course line overriding `grading.md`;
   - students table: legacy upgrade, percent save/reload, days-and-percent rejected;
   - Students and Courses forms: percent and limit validation, older form keeps saved values;
   - personal dates: percent student gets due and lock; on-time student reopened after the due date; late student not reopened; absent on the due date; regular student unchanged; rerun writes nothing; shared override held; preview writes nothing.
3. **Open design points to settle while testing**
   - `personal_dates` is a separate step, not part of create. Either call it from the create path or have the playbook run it right after every create in a course with flagged students.
   - Each override is written with an explicit `due_at`, so a reopened student is pinned to the old due date if the teacher later moves it.
   - Section-specific lock dates are ignored when working out a student's current lock.
   - Whether an unpublished assignment returns submission rows is unknown; the code falls back to the assignment's due date.
   - Scoring-session timing (`prepare_late_timing`) does not see the percent; pulls project only `extra_time_school_days`. Either project the percent or rely on the personal Canvas date.
   - An assignment already past the cutoff with no lock (older work) is not zeroed by anything. The developer chose lock-only; say so in the playbook.
4. **Playbooks and context (seed, `workspace-template/`)**
   - `context/grading.md`, `context/courses.md` guidance: per-course `late_work`, the cutoff line, percent vs days.
   - `playbooks/score.md`: Lateness and "Fix late marks" (percent, personal dates); cutoff interplay with the honest-effort floor; new short section "Personal dates and reopening for corrections" (preview, report by alias, go-ahead, write, held rows; keep reasons private).
   - `playbooks/authoring.md`, `edit.md`, `quizzes.md`: timetable rows, lock at the cutoff, run personal dates after create.
   - `playbooks/inbox.md`: replies to late-work requests state the course's limit.
   - `playbooks/parent-updates.md`, `family-contact.md`: wording for a policy zero; never call it "not turned in" without evidence.
   - `playbooks/stad.md`: `late: 0.5` only applies inside the window; none in CS8.
   - `playbooks/group-work.md`, `context/students.md`, `TEACHER.md`, `AGENTS.md` routing, `context/commands.md` (a Home command such as "Reopen for corrections").
   - Student-facing wording goes through `playbooks/review.md`.
5. **Docs and records:** `engine-docs/formats.md`, `engine_reference` help, `docs/mcp-adapter.md`, `docs/engine-capabilities.md` with `internal/localui/capabilities.go` (local tests only, review date), `docs/ARCHITECTURE.md` if the shared/local boundary text lists student columns, `CHANGELOG.md`, `past_seeds.txt` at commit time.
6. **Live checks, each needing its own go-ahead (CS8 has 28 real students):** read the three courses' Canvas late and missing policies; a preview run on one CS8 assignment; one written personal date on a student the developer names, checked in Canvas, including whether the student can submit after the base lock.

## Out of scope unless asked

A scheduled automatic reopen, zeroing late work on assignments that stay open, SIS attendance import, group assignments, changing the school calendar.

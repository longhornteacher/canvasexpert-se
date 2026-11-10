# Handoff: Late-work reconciliation (2026-11-10)

**Status:** On 2026-11-10 the developer accepted every default in §1. **Slices 1-5 are built in Dev the same day, local tests only.** No Canvas Live check has run: the read-only CS8 sweep and one corrected late mark in §3 are still open and each needs its own go-ahead.

As built, differing from the plan below:
- The sweep's row for a student with more than one attempt times the lowest-numbered attempt.
- `absent_dates` in provenance are the saved absences from the Canvas due date through the counted submission; the write is held when that list differs at post time.
- A Home command "Fix late marks" was added to the seed `context/commands.md`.
- Shared-grade group timing is unchanged: members with different absences do not share one decision and are held, as before for any differing deadline.

## Problem

Canvas counts lateness in calendar time from the due date. It ignores weekends, school closures, a student's absences and a student's late-turn-in accommodation, so its late penalties are too harsh until someone corrects each submission by hand.

## What exists today

- `context/school-calendar.md` ships filled in for 2026-27 (terms, closures). `CountSchoolTime` and `AddSchoolDays` (`internal/workspace/school_calendar.go`) skip weekends and closures.
- `prepare_late_timing` (`internal/workflow/late_prepare.go`) recomputes lateness for one pulled packet. The agent supplies the first genuine attempt and any allowance; `postLateTiming` rechecks and writes `late_policy_status` / `seconds_late_override`.
- The Students page has an **Extra time** checkbox (`.cse/students.csv`, `extra_time`). It carries no duration; the agent must find one in `context/grading.md` or `students.md`, and holds the student when there is none.
- Corrections happen only for work being scored in a session, or one assignment at a time as `late-only`.

## Gaps

1. No record of absences anywhere.
2. No per-student number of extra days, so accommodation students are held or asked about every time.
3. No way to fix late marks across a course without scoring each assignment.

## Chosen (developer, 2026-11-10)

- **Course sweep.** One request recomputes every late mark in a course and shows the corrections before any write. Scoring sessions keep their own timing step.
- **Absences are entered in the WebUI.** Nothing is read from the SIS.
- **Accommodation students come from the Students page**, with a new per-student number of extra school days.

## 1. Developer decisions (needed before building)

| # | Question | Proposed default |
|---|---|---|
| D1 | Which absences move a deadline? | Only absences on or after the due date. Absent on the due date: the deadline moves to the same time on the student's next present school day. Absent days after that are not counted as late time. An absence before the due date changes nothing; the teacher gives that student a Canvas date instead. |
| D2 | Which attempt does the sweep time? The scoring rule is "first genuine attempt", which needs someone to read the work. | The earliest attempt. A student with more than one attempt is still proposed, but marked "check first attempt" in the report so the teacher can drop the row. |
| D3 | A student has extra days and also a personal Canvas due date for that assignment. Add the days again? | No. A personal Canvas date (an override naming that student) is taken as already including the allowance. Section dates still get the extra days. |
| D4 | Extra time is checked but no number is saved for the student. | Use `late_work.extra_time_school_days` from `context/grading.md` when set; otherwise hold that student's rows with "no extra-time days saved". |
| D5 | Sweep scope. | Ordinary individual assignments due in the current grading period, where Canvas shows the submission as late. Not group assignments, New Quizzes, excused work, custom grade statuses or missing work. |
| D6 | The grading-timetable work is uncommitted and touches `types.go`, `main.go`, `remote.go`, `grading.md` and the capability files this needs. | Commit it first (with its `past_seeds.txt` refresh), then start slice 1. |

## 2. Slices

Ownership: storage, date math, the sweep and writes are **Engine**. When to run a sweep, how to report it and what to do with "check" rows are **playbook**.

### Slice 1: Extra days per student

- `internal/workspace/students.go`: new last column `extra_time_days` (whole school days, 0-30; 0 = none saved). Add a header generation like the earlier ones; older tables load with 0.
- Students page (`students.go`, `templates/students.html`): a small number box beside Extra time. A number with the box unchecked is rejected with a plain message.
- Pulls project it beside `extra_time` as private `extra_time_school_days` (`pull_work.go`, `group_pull.go`, `domain.StudentWorkDocument`, group member). Same privacy rule as the flag: never in comments, shared feedback or parent reports.
- Tests: legacy table upgrade, save/reload, form validation, packet projection from the active student authority.

### Slice 2: Absences in the WebUI

- New private store `.cse/absences.csv` through `ResolveRulesPath` (follows UserSync like `students.csv`; never model context): `student_ref, instance, date`. One row per school day keeps the file trivially mergeable and the math simple.
- New hub page `/absences` (nav: Classroom, after Students). Course tabs, a date (default today), the roster with a checkbox per student, Save. Below it, recorded absences for the course, newest first, each with Remove. A date range adds one row per school day in the range.
- The page states the calendar it is using (coverage dates, closure count) or the exact problem when `school-calendar.md` is missing or does not cover the date.
- Tests: add, range across a weekend and closure, remove, stale-form rejection, hub course scoping.

### Slice 3: Engine math

- `SchoolCalendar.CountSchoolTime` and `AddSchoolDays` stay as they are. Add one helper that returns a copy of the calendar with a student's absent dates added as closures, and one that applies D1 (move the deadline off absent days).
- `calculateTiming` (`late_prepare.go`) loads the student's absences and uses the adjusted calendar. `SchoolTimeProvenance` gains `absent_dates` so the report can say why.
- `schoolTimeHold` (`late_timing.go`) recomputes with the same absences, so a changed absence list holds the write with "recompute timing" like a changed calendar does today.
- Scoring sessions pick this up with no playbook change beyond one sentence.
- Tests: absent on the due date, absent the day after, absence spanning a weekend, absence plus extra days, absence removed between prepare and post.

### Slice 4: Course sweep

- `cse late-sweep COURSE [--period NAME]`, MCP `late_sweep`. Read-only.
- One paged submissions read for the course with history, filtered to D5. For each late submission: earliest attempt (D2), deadline = Canvas effective due date + extra days (D3, D4) adjusted for absences (D1), counted with slice 3.
- Output under `courses/<course>/late-sweeps/<timestamp>/`: one ordinary `results` document per assignment holding `late-only` rows, plus a summary (corrected, unchanged, held with reason, "check first attempt"). Rows reuse `prepareStudentTiming`, so the existing holds apply: explicit Canvas late status without CE-SE provenance, custom status, hidden assignment, incomplete timing.
- Writing uses the existing `post_results` on each file after the teacher's go-ahead. No new write path.
- Tests against the fake Canvas server: weekend-only lateness becomes on time, a closure week, extra days, personal override not doubled, multi-attempt marked, held reasons, nothing written by the sweep.

### Slice 5: Playbooks, docs, capability records

- `workspace-template/playbooks/score.md` §Lateness: saved extra days replace the "ask for the allowance" step when present; absences are applied by the Engine; a new short "Fix late marks for a class" section for the sweep (report in plain words, wait for the go-ahead, post, report held rows).
- `workspace-template/context/grading.md`: say the per-student number on the Students page wins over the general allowance.
- `workspace-template/TEACHER.md` and `AGENTS.md` routing: "fix late marks" goes to `score.md`.
- `engine-docs/formats.md`, `engine_reference` help, `scripts/check-mcp.mjs` tool count.
- `docs/engine-capabilities.md` + `internal/localui/capabilities.go`: new rows labelled local tests only, review date updated.
- `CHANGELOG.md`, `past_seeds.txt` at commit time.

## 3. Checks

Per [handoffs/README.md](../README.md): `go test ./...`, `go vet ./...`, temp build plus `node scripts/check-mcp.mjs`, `git diff --check`.

Live evidence wanted, each needing its own go-ahead because CS8 has real students: a read-only sweep of CS8, then one corrected late mark on a student the developer names, checked in the Canvas gradebook.

## Out of scope unless asked

SIS attendance import, missing-work handling, group assignments and New Quizzes in the sweep, changing Canvas due dates or overrides, a scheduled automatic sweep.

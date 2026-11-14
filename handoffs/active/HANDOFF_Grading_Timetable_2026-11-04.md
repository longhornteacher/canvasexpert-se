> Historical development notes: the parent-report correction-date integration described below was superseded by the 2.0 parent-update redesign. Parent emails do not advertise correction deadlines.

# Handoff: Grading timetable on new assignments (2026-11-04)

> **Superseded in part 2026-11-13:** see [HANDOFF_Student_Uploads_and_Grading_Offers_2026-11-13.md](HANDOFF_Student_Uploads_and_Grading_Offers_2026-11-13.md) section 2. The corrections row appears only for a teacher-set `correction_school_days` window and never mentions points; there is no default window, and the end-of-block corrections sentence is dropped with no window.

**Status:** On 2026-11-04 the developer accepted every proposed default in §1.

**Slices 1–5 are built in Dev (2026-11-04), not yet committed.** The code is `internal/workflow/timetable.go` with its tests, plus the create and update hooks in `author.go`, the `validate` preview and the parent-report `correctionDeadlineFor`. `go test ./...`, `go vet ./...`, the 58-tool MCP smoke and `git diff --check` passed.

**Slice 0 done (CS8, 2026-11-04, developer go-ahead).**
- A read-only GET of `/late_policy` returned the expected fields: 10 per day, floor 30%.
- A throwaway program in gitignored `.test-workspaces/timetable-probe/` created unpublished assignment 3715542 with the Engine's block. Canvas stored `<details>`, `<summary>` and `data-cse` byte-for-byte, and the Engine regex matched.
- The assignment was deleted and returned 404 afterward. `data-cse` stays the marker.

**Still open:**
- Student view and mobile rendering.
- A full `push` create in Canvas Live.
- `past_seeds.txt` regeneration at commit time, for the changed `authoring.md`, `edit.md` and `TEACHER.md` seeds.

## Goal

Every new assignment with a due date ends with a collapsed block that students and families can open later, showing what happens if the work is late:

```
▸ Grading timetable (late work and corrections)
    Due: Thu 11/05 (extra time: Fri 11/06)
    −10 points per school day late, starting Fri 11/06 (extra time: Mon 11/09)
    Last day for corrections: Wed 11/11 (extra time: Thu 11/12)
    Assignment closes: Fri 11/20 (extra time: Mon 11/23)
```

Dates count school days only (weekdays that aren't closures in `context/school-calendar.md`), using the same rule as scoring. The block replaces the "Corrections available" line that `TEACHER.md` requires today, so the assignment states the correction rule once.

Ownership: the date math and HTML are **Engine** (exact math, safe writes). Whether to include the block, how to explain it to the teacher, and what to do when the policy isn't set are **playbook** judgment.

## 1. Developer decisions (needed before building)

| # | Question | Proposed default |
|---|---|---|
| D1 | The correction deadline: the rule says 4 school days, but the example shows 11/09, which is 2 school days after 11/05. Which is it? | 4 school days, set by the teacher in config |
| D2 | What does "locked at 30" mean? (a) Canvas's late-policy floor (−10/day stops at 30%), (b) the date Canvas stops accepting submissions (`lock_at`), or (c) both | (c): show the floor from Canvas's late policy, and set `lock_at` on the 11th school day |
| D3 | When `lock_at` is set, extra-time students need a later lock date or Canvas locks them out. Either set `lock_at` to the extra-time close date for everyone, or add a per-student override for each flagged student | One later `lock_at` for everyone. No overrides, and no private flag reaches Canvas. Regular students submitting that extra day are already at the floor. |
| D4 | Should the parent-report "Correct by" date switch to the same school-day rule? Today it is `correction_days` (14 calendar days) in `context/parent-updates.md` | Yes, when the school-day setting is configured. The report *window* keeps `correction_days`. |

Out of scope unless the teacher asks: New Quiz instructions, Pages, graded discussions, adding the block to existing assignments in bulk, per-section timetables.

## 2. Configuration

Add an optional `late_work` block to the front matter of `context/grading.md` (teacher-rules authority, so a configured UserSync copy wins):

```yaml
late_work:
  correction_school_days: 4     # last day for corrections = due + N school days
  close_after_school_days: 11   # lock_at = due + N school days (D2)
  extra_time_school_days: 1     # general Extra time allowance; omit = no extra-time dates shown
```

- Every key is optional; a missing key omits that row. With no `late_work` block there is no timetable (the playbook asks the teacher once, then saves the answer).
- `extra_time_school_days` is the teacher's documented general allowance, which `grading.md` already says must be written down because the flag carries no duration. Individual accommodations in `students.md` stay private and are never shown.
- The **penalty and floor are not configured here.** They come from Canvas's course late policy (`GET /api/v1/courses/:course_id/late_policy`: `late_submission_deduction_enabled`, `late_submission_deduction`, `late_submission_interval`, `late_submission_minimum_percent_enabled`, `late_submission_minimum_percent`), because Canvas alone calculates late penalties ([SCORING-RULES.md](../../docs/SCORING-RULES.md)). Deduction disabled → no penalty row. Interval `hour` → state it plainly rather than restating it per day.
- Points: a percent deduction on a 100-point assignment is shown as points ("−10 points"); otherwise as a percent ("−10% per school day").

## 3. Slices

### Slice 0: Canvas rendering check (live, CS8, needs go-ahead)

Create one **unpublished** disposable CS8 assignment whose body ends in

```html
<details data-cse="grading-timetable"><summary>Grading timetable (late work and corrections)</summary><ul><li>…</li></ul></details>
```

Read it back and record whether Canvas keeps `details`/`summary` and the `data-cse` attribute (fallback marker: `id="cse-grading-timetable"`). Check it collapses in the student view (Test Student) and in the mobile app if one is at hand. Delete it after. The marker that survives is the one slices 2–3 use. If `details` is stripped, fall back to a plain heading plus list, still marked, and tell the developer.

### Slice 1: Engine computation

- `internal/workspace/school_calendar.go`: reuse `AddSchoolDays`. It already skips weekends and closures, keeps the wall-clock time and errors when the calendar doesn't cover the result.
- New `internal/workflow/timetable.go`: `BuildTimetable(due time.Time, cal SchoolCalendar, policy LateWorkPolicy, canvas CanvasLatePolicy) (Timetable, error)` returns rows for regular and extra-time students:
  - due date, plus extra-time due = due + allowance
  - penalty start = next school day after each due date (shown only if deduction is enabled)
  - corrections = each due date + `correction_school_days`
  - close = each due date + `close_after_school_days`
  - `RenderTimetableHTML(Timetable) string`: dates as `Thu 11/05` in the calendar timezone, the marker from slice 0, no student data.
- A Canvas client method for the course late policy (read only). If it can't be read, omit the penalty row and report that, without stopping the create.
- Calendar gaps (the calendar doesn't cover a date) stop the timetable, not the assignment. The push reports "timetable omitted: calendar ends before …".
- Tests: a due date before a weekend (the 11/07 Saturday case), a closure inside the window, extra time crossing a weekend, a missing calendar, deduction disabled, an hour interval, no extra-time allowance, a non-100-point assignment.

### Slice 2: Create path

- Assignment create documents (`kind: assignment`, `operation: create`) get optional `grading_timetable: false` to opt out. Default: included when the draft has `due_at` and `late_work` is configured.
- In the push, after Markdown/HTML rendering and tier binding (`deliverHub` / `BindTierLinks` in `internal/workflow/author.go`, `hub.go`), append the rendered block to the end of the body. Appending after rendering is required: the Markdown renderer escapes raw HTML.
- When D2/D3 settle `lock_at`: if the draft gives no `lock_at`, set it to the computed close date. If the draft gives one, keep it and show *that* date in the timetable. Never silently overwrite a teacher date.
- The local review / check-in sample (`internal/workflow/review.go`) shows the block, so the teacher sees the dates before the push.
- Resume: the block is part of the body the Engine records, so a rerun compares the same body and creates no duplicate.

### Slice 3: Keep it current on date changes

A wrong date in front of families is the failure that matters here.

- The assignment `update` operation: when `due_at` (or `lock_at`) changes and the live body contains the marked block, regenerate the block in the outgoing body. The existing three-way body check still applies.
- `edit.md`: a base due-date change on an assignment with a timetable goes through `update` (which carries `due_at`), not `assignment-dates` alone. If the teacher only changes section/student override rows, the base-date timetable stays as it is.
- No new refresh command, and no sweep of old assignments.

### Slice 4: Parent report alignment (D4)

- `internal/workflow/parent_report.go` `correctionDeadlineDays`: when `late_work.correction_school_days` is set, compute "Correct by" with `AddSchoolDays` from the student's due date (the report already loads the school calendar). Otherwise keep `correction_days` calendar days.
- Tests: a report row whose correction deadline crosses a weekend matches the timetable date for the same assignment.

### Slice 5: Playbooks, docs, capability records

- `workspace-template/playbooks/authoring.md` (Standards paragraph): the timetable replaces the "Corrections available" line. Mention it in one line of the check-in ("ends with the grading timetable: corrections through Wed 11/11, closes Fri 11/20"). If `late_work` isn't configured, ask the teacher once for the three numbers and save them in `context/grading.md`. Don't ask again.
- `workspace-template/TEACHER.md` §Corrections: the correction deadline is the configured school-day count when set; students see it in the assignment's grading timetable.
- `workspace-template/context/grading.md` seed: an empty `late_work` block with a one-line comment per key.
- `workspace-template/playbooks/edit.md`: the slice 3 routing.
- `engine-docs/formats.md`: `grading_timetable`, the `late_work` keys, the `lock_at` default.
- `docs/engine-capabilities.md` + `internal/localui/capabilities.go`: a new row with an honest evidence label (local tests, plus slice 0's live rendering check if it ran), with the review date updated.
- `CHANGELOG.md`.

## 4. Checks

Per [handoffs/README.md](../README.md) execution notes: `go test ./...`, `go vet ./...`, the temp build plus `node scripts/check-mcp.mjs`, `git diff --check`. Live evidence is only slice 0 (and, optionally, one unpublished CS8 create through the real push, then delete). No publishing to CS8: it has real students.

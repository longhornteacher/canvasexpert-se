# Parent reports: current grading period

Status: implemented locally in Dev on 2026-10-18; focused tests pass. No new Canvas Live report, parent-email delivery, UserInstall adoption or destination-client validation has been performed. This remains the parent component contract of [the combined calendar/scoring implementation plan](SCHOOL-CALENDAR-SCORING-AND-PARENT-UPDATES.md). Earlier sections preserve the design and evidence gathered before implementation.

## Problem and evidence

The first reports included Term 1 work during Term 2 and caused families stress. `internal/workflow/parent_report.go` has no term filter: Missing/0 can include work of any age, correction candidates depend only on the due-date window, and recent grading activity can bring older assignments into history. Email and printable student reports share this packet, so fixing only agent wording will not fix both outputs.

The user confirmed that term dates are kept separately from Canvas grading periods and pointed to the district calendar's **Grading Periods** section. The supplied source is `C:\Users\adamb\Desktop\2026-27_District_Calendar_EN.pdf` (one page, updated June 9, 2026), read and visually checked on 2026-10-18. The district's matching public copy is <https://resources.finalsite.net/images/v1781036702/pearlandisdorg/balw7nmbdljmzpybofrq/2026-27_District_Calendar_EN.pdf>.

| Period | Start, inclusive | End, inclusive |
|---|---|---|
| Term 1 | 2026-08-19 | 2026-10-08 |
| Term 2 | 2026-10-13 | 2026-12-18 |
| Term 3 | 2027-01-06 | 2027-03-19 |
| Term 4 | 2027-03-22 | 2027-05-27 |

All four periods are required, including Terms 3 and 4. Resolve them from the date on every new report; do not hardcode Term 2 or store a manual current-term switch that must be changed at each boundary.

At initial investigation, both Dev's seed and the connected teacher's active `context/school-calendar.md` contained school-year dates, holidays and bell schedules but omitted these grading periods. The active calendar is in configured UserSync. Earlier score-playbook guidance counted manually; the implemented timing path now reads this same calendar for school-time arithmetic. Reuse the calendar file without creating a second calendar authority.

## Proposed behavior

1. **One calendar authority.** Add a `grading_periods` list to the existing `context/school-calendar.md` front matter. Each entry has `name`, `start_date` and `end_date`. Use the verified dates above in this teacher's calendar. Read via `workspace.ReadTeacherDocument` so configured UserSync remains authoritative. Do not add a second term setting to parent-update preferences or query Canvas enrollment terms as a substitute.
2. **Resolve the current period by the report's as-of date.** Interpret dates in the calendar's existing timezone, including the entire last day. Require exactly one containing period. If dates are missing, malformed or ambiguous, or the date falls in a gap, explain the calendar issue and request its correction; never silently fall back to all-course reports or the preceding term. This requirement applies to parent reports, not unrelated scoring or authoring.
3. **Select all work by due date before classifying it.** Use the student's Canvas effective due date (`cached_due_date`, with assignment `due_at` only as a valid fallback) for both term membership and the retrospective window. Respect an explicitly undated student override. Grading, regrading, submitting or commenting later does not move older-due work into the window. A known assignment without a due date is outside this due-dated scope; do not hold the class merely because deliberately undated work exists. Genuinely unavailable/malformed due-date or assignment-audience evidence needs an item-specific incomplete-data explanation, not an invented missing item. Preserve the published, student-visible and excused exclusions. Known required ungraded coursework belongs in the inventory without a numeric grade.
4. **Intersect the two-week due-date window with the period.** The lower bound is the later of the existing 14-calendar-day lookback and the current period's start; the upper bound is `as_of`, inclusive. Every retrospective list is a view of assignments due in that interval. Recent grading activity is never an additional way to admit an older-due assignment. Due Soon remains a separate seven-calendar-day upcoming section, as the teacher explicitly requested, and cannot cross the period's end. Preserve the correction deadline calculation and teacher override policy; report selection does not rewrite a student's deadline. On October 18, 2026, the recent window therefore starts October 13, not October 4.
5. **Include the complete due-work inventory.** Add `work_due` per report course with every in-window assignment and its honest status and score availability, including completed/perfect work and submitted work awaiting grading. Derive Missing/0, Corrections and scored history from this collection. The teacher's latest instruction replaces the prior exception that Missing/0 could include work older than 14 days: those older assignments are now excluded, even during the same term. Preserve the in-progress positive-grade, zero and no-duplicate-corrections rules within the window. Compute skip eligibility only after filtering/classification, with incomplete-data flags taking precedence over an automatic decision.
6. **Make the scope visible.** Store the resolved period name, dates and effective recent-window start in the saved report packet. Show a compact period label in the review email and printable report, using that saved scope rather than re-resolving the current date when an existing report is opened. Do not describe `canvas_enrollment_current_score` as a verified term average or invent a replacement course-grade calculation; it retains its existing provenance and use for the cutoff.
7. **Preserve saved work.** Existing unscoped runs cannot be made term-safe by recomputing skip eligibility. Give a clear request to pull a fresh report before sending or printing an unscoped run; retain its drafts and delivery outcomes unchanged. No automatic draft migration, deletion, replay or resend. Fresh report generation and draft references must use only the filtered lists.

## Saved scope contract

Keep report `schema: 1` and add `ParentReportPacket.Scope *ParentReportScope` serialized as top-level `scope`. New reports always write a non-null object with these fields:

- `timezone`: the calendar IANA timezone used to resolve the term and rolling window.
- `period_name`, `period_start_date`, `period_end_date`: the resolved grading period's label and inclusive local dates.
- `recent_start_at`: the inclusive later of the period start and `as_of` minus 14 calendar days.
- `recent_end_at`: the inclusive retrospective upper bound, equal to the report's existing `as_of` instant.
- `due_soon_end_at`: the exclusive upper bound for upcoming work, the earlier of `as_of` plus seven calendar days and the midnight immediately after the period's last date.

Use RFC3339 timestamps for the `*_at` fields. Validate timezone, bounds, period/as-of agreement and each inventory row's membership. Scope is saved provenance, not an approval token. Do not re-resolve an old run against today's term when loading it.

An absent/null `scope` is the specific legacy-run detection rule for new sending/printing; preserve its files and explain that a fresh report is needed. A present malformed scope also needs a clear validation error. Every new course section writes `work_due` explicitly, including `[]` for a complete empty window. An empty new inventory is therefore distinct from an old unscoped run. Do not use the schema number, a guessed rollout date or a nonempty assignment list to distinguish them.

## Completeness, rendering and sending

- “Required work” means published Canvas assignments assigned/visible to that student and not excused. Include a matching due date regardless of title, submission state, perfect score or absence of a grade; do not infer optionality from an assignment's name. Retain the existing release/privacy requirements for any grade or feedback shown to a family.
- Build the inventory from the assignment catalog and student evidence, not solely the submission rows that happened to be returned. Request `include[]=visibility` in parent submission reads. If a relevant catalog assignment has no returned row, resolve that student's assignment visibility/effective date with a bounded existing read before claiming a complete report. A missing row alone proves neither missing work nor that the student was assigned it. Reuse existing assignment/override reads; do not create a second audience or calendar subsystem.
- A compact **Work due in this period** table must show every inventory row in both email and printable output. Retain first/current grade history when known, and honest awaiting-grading/no-grade labels otherwise. This table can replace the narrower Grade history table; keep Missing/0 and Corrections as actionable sections above it and Due Soon separately. Agent notes and correction selection remain judgment, but an agent draft cannot omit a completed or awaiting-grading inventory row. Avoid a second duplicate full inventory table.
- Revise draft validation to check completeness against the authoritative inventory and preserve the same data on both rendering paths. Old `history` fields can remain as scored evidence, not the complete assignment inventory. A first score before the window for an included assignment is still legitimate history; it cannot admit an excluded assignment.
- Say **No Missing/0 work in this report window**, rather than the current “Nothing is missing.” Do not imply no old missing work exists or turn incomplete evidence into an all-clear statement.
- Recommended empty-window behavior: if no required work was due in the retrospective window, skip with that reason rather than inventing a send item from older work or Due Soon. Otherwise preserve the configured cutoff policy after scoped Missing/0 normalization. This explicit empty-window rule resolves the current combination of a low course score and an invalid empty draft; it is part of the proposed implementation direction, not already implemented behavior.
- Parent reads do not normalize grades or pretend that pending timing corrections already changed Canvas. After an accepted scoring/normalization run, a fresh parent pull sees the actual Canvas results. The teacher's plan to close older assignments does not authorize automatic closing in this feature.

## Implementation scope

Develop in Dev. Before editing, inspect the working tree: parent classifications already have uncommitted changes in `parent_report.go`, `parent_drafts.go`, their tests, the parent playbook/context, CHANGELOG and capability docs. Build on those changes and do not reset them.

- Add a small calendar reader/type in the existing workspace package, or a narrow parent-report helper using the existing teacher-document reader. No general calendar service, database, scheduler or term-management UI is needed.
- Update `internal/workflow/parent_report.go`, report loading/rendering/validation in `parent_drafts.go`, `parent_send.go` as needed for printable labels, and relevant CLI/MCP help. Keep the existing command/tool invocation compatible; resolve dates from teacher context without asking for term/cutoff inputs on each run.
- Update `workspace-template/context/school-calendar.md` with all four verified grading periods for its existing 2026-27 starter calendar and preserve its holidays and bell schedules. Keep school-specific dates as editable teacher context, never Engine constants. Update the parent playbook/context and setup guidance to retain every grading period when reading a teacher-supplied school calendar, including periods in the second semester. Do not require a new setup ceremony for unrelated work.
- Document the new report metadata in `docs/parent-update-flow.md` and the Engine format reference if applicable. Update `internal/localui/capabilities.go`, `docs/engine-capabilities.md` and CHANGELOG with actual implementation/check status. The current capability note records the limitation only.
- Any update to this teacher's actual calendar belongs in its active UserSync authority, preserves all existing content and is a separate context adoption step. Product seed updates never overwrite customized calendars. The connected UserInstall remains the command/session root.

## Focused verification

Use synthetic dates and fake Canvas responses; this work requires no live Canvas write or email delivery.

- Near the Term 1/2 boundary, exclude a Term 1 zero/missing item, a Term 1 correction whose 14-day deadline is still open, and a Term 1 assignment regraded during Term 2 from every output list.
- Exercise automatic selection of all four periods, including Term 3 starting January 6, 2027, and Term 4 starting March 22, 2027. At the T2/T3 and T3/T4 transitions, exclude the preceding term's work even after a recent regrade. Keep the two-week lower bound inside the newly selected term. Check that the calendar's gaps do not silently select the last term.
- Include work due on Term 2's first day. Later in Term 2, exclude a current-term unresolved zero older than 14 days, including when recently regraded. Include every assignment actually due in the window, with completed/perfect, never-regraded, awaiting-grading and required ungraded examples. Preserve score-history provenance for included assignments.
- Check effective student due dates and inclusive calendar boundaries in America/Chicago. Keep Due Soon inside the period's last day.
- Missing/unknown period configuration must produce the specific calendar explanation. Known undated work stays outside the dated inventory without a class-wide hold; missing relevant visibility/date evidence gets a precise explanation rather than an invented missing status.
- Exercise fresh packet creation through `ParentReport`, loading/draft validation, and both rendered outputs. Assert saved scope, complete inventory despite an incomplete agent selection, absence of out-of-window titles/IDs, separate Due Soon, scoped empty wording and the no-recent-work skip. Cover the short legacy-run rejection without modifying old drafts/outcomes.
- Verify the calendar is read from active UserSync with no inactive local fallback, using the existing fixture pattern. Reuse existing classification/rendering tests rather than adding snapshots or a broad test matrix.

Run the affected workspace/workflow tests and relevant local UI checks. Report local checks as local evidence. A later teacher-observed report can establish live usability; it is not a prerequisite for implementing this fix.

## Execution evidence (2026-10-18)

- Passed: `go test ./internal/workflow -count=1` (includes report-window, renderer, saved-scope/draft validation, privacy, targeted visibility/date completion, empty-window and legacy-run cases).
- Passed: `go test ./internal/workspace -run 'TestSchoolCalendar' -count=1`.
- These local tests cover term selection/window clipping, complete inventory and visibility resolution, individualized/undated dates (including old base dates), targeted-read identity/privacy, unreleased-grade suppression, scoped rendering/draft validation, empty-window decisions and legacy-run preservation. They are not Canvas Live or family-delivery evidence.

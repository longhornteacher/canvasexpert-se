> **Retired 2026-10-18 — Dev implementation complete.** Calendar/timing and parent inventory landed in `aae89b1`; the parent-date/Extra time [follow-up](PARENT-DATES-AND-EXTRA-TIME-FOLLOWUP.md) is also implemented and locally verified. Original execution instructions and dated evidence below are historical. The recorded UserSync calendar adoption is complete; remaining distribution, installed-playbook adoption and authorized Canvas/client observations are consolidated in the [delivery follow-up](CALENDAR-SCORING-DELIVERY.md). No new live result is implied.

# School calendar in scoring sessions and parent updates

Status: implemented locally in Dev on 2026-10-18; focused calendar, parent and late-only tests pass. Canvas Live timing writes, a new parent report, UserInstall calendar adoption, distribution and destination-client validation remain pending. This plan coordinates the [parent-report contract](PARENT-REPORT-CURRENT-TERM.md). Earlier research/design sections are retained as history. Read root AGENTS.md, ARCHITECTURE.md and SCORING-RULES.md before further execution.

## Requested result

The teacher uses the ordinary scoring-session or parent-update request. Neither request needs extra calendar, term or lateness instructions.

- Every scoring session automatically considers school-day lateness normalization, including submitted work Canvas already considers graded. The agent identifies the first genuine submission and established accommodations; the Engine counts school time and prepares the exact timing correction. Canvas calculates point deductions.
- Parent reports include **all required work due in the last two weeks within the current term**, including completed work and work awaiting grading. An older Missing/0 item or recent regrade cannot bring an older due date into the report. The teacher plans to close older assignments; automatic assignment closing is not part of this request.
- The teacher explicitly retained **Due Soon as a separate upcoming section**. Keep its current seven-calendar-day horizon, restricted to the current term.
- Use all four grading periods from the supplied calendar. Selection changes automatically with the date; there is no manual current-term switch.
- Preparing either flow is read-only in Canvas. Normalization is included in ordinary scoring review/posting, with the existing teacher review preference and requested posting scope. Parent delivery keeps its existing private review and Send interaction.

## What the investigation found

| Flow | Pre-implementation finding (research snapshot) | Original design direction |
|---|---|---|
| Session entry | Home copies a plain prompt from `context/commands.md`; `AGENTS.md` says a scoring session first discovers work and does not itself authorize posting | Put automatic preparation in the scoring playbook and typed Engine paths; do not hide writes inside Home or a pull |
| Scoring discovery | `pull_work.go:270` skips `liveHandled` submissions; `:341` also skips completed local grade actions; group discovery has equivalent skips | Assess grading completion and lateness normalization independently |
| Pulled evidence | `AttemptWork` exposes due dates, attempt history and a late boolean; `StudentWorkDocument` exposes entered/current scores and deductions, but no complete observed timing baseline | Include the current timing/status baseline and explain whether a row needs scoring, timing, or both |
| Lateness | `score.md` tells the agent to count with PowerShell; `late_timing.go` only writes supplied timing | Supply a reusable Engine calculation accessible from MCP, where an arbitrary shell is unavailable |
| Timing writes | Only `grade.late_timing` exists; `Post` has grade, comment-only and revise-feedback branches | Add a narrow timing-only operation so fixing an old penalty never rewrites an academic assessment |
| Parent selection | `buildStudentReport` iterates submission rows; Missing/0 is unlimited; recent score changes can select old assignments | Construct the complete due-date inventory first, then classify within it |
| Parent presentation | Agent references choose email rows; printing includes classified lists, and its empty message says “Nothing is missing” | Ensure every in-window assignment appears and qualify empty messages by the saved reporting window |
| Calendar/setup | The calendar stores holidays/year dates/bell schedule; setup asks only for school-year dates and holidays | Add grading periods to that same file and retain them when importing future calendars |
| Installed adoption | Engine seed updates preserve `context/` and teacher overrides; the connected teacher uses UserSync | Merge the supplied period dates into the active calendar during adoption, not merely into the Dev seed |

The focused existing workflow baseline passed on 2026-10-18:

`go test ./internal/workflow -run 'Test(LateTiming|BuildStudentReportSections|Parent|GradebookVersions)' -count=1`

These are existing local tests, not tests of the proposed behavior or new Canvas Live evidence.

## Execution decisions after review

The teacher supplied an Opus 4.6 review on 2026-10-18 recommending execution and identifying three decisions and four contract details. Resolve them here before assigning the slices:

- **A1, accepted timing writes:** use the existing `canvas.Client.WriteAccepted` path. An accepted successful HTTP response, including HTTP 200 with a sparse or empty body, produces `succeeded`; response shape is not a condition of success. Keep pre-write identity/attempt/due-date checks. Retain existing error classification for writes whose acceptance is unknown (such as a transport timeout); omitted response fields do not create uncertainty. This applies to timing attached to grades and timing-only operations through the shared helper. Add no confirmation GET. This follows the existing accepted-write contract; it is not a new policy choice for slice 3.
- **A2, field location:** put `work_purpose` on `domain.StudentWorkDocument`, alongside the assignment/student identity and observed state. It describes this row's role in the session, not an attempt's academic content. It does not belong on `AttemptWork`. Add corresponding purpose metadata to group member/group work where needed without turning it into a grading-completion flag.
- **A3, batch source:** keep one assignment-packet-backed calculation path. Slice 4's session-mode discovery must emit a fresh ordinary packet and student-work evidence for already-graded timing candidates, even though their academic assessment remains handled. The current absence of those packets is what that slice changes. Reuse the existing fetch/packet machinery and retrieve only the evidence needed for timing judgment. `prepare_late_timing` and `late-timing-decisions` reference that packet; do not add an assignment-only alternative that recreates a second evidence reader or accepts unsupported attempt choices. No second full scoring pull, reassessment, rubric interpretation or replacement of completed results is required after session discovery. Batch one assignment packet at a time; course-wide sessions call it for their resulting packets. If a caller has no evidence packet, run the same read-only session discovery for the selected assignment first.
- **Calendar validation:** the calendar type's ordinary `Validate()` validates the supplied calendar data and any supplied periods, but does not require a nonempty period list. A separate current-period resolver requires exactly one containing period. The school-time calculator requires coverage of the counted interval only when school-day exclusions are needed. Thus absent periods do not block explicit-assignment school-time calculation, and already-known on-time work needs no holiday-calendar prerequisite.
- **Visibility:** request `include[]=visibility` explicitly on the parent bulk submission read and any targeted fallback used to complete the due-work inventory. Use the returned value/presence when determining the student's assigned work; a missing row is not visibility evidence.
- **Shared due dates:** retain both the pulled-member comparison (`membersShareDue`) and the full fresh audience comparison (`commonDueDate` after `groupPreflight`) before a propagated timing write. The representative member's due-date guard is additional protection, not a replacement for those full-audience checks. The normalized/accommodated timing choice must also be true for every member.
- **Legacy parent detection:** use a new top-level pointer field `ParentReportPacket.Scope` with JSON key `scope`. Its absence identifies an unscoped legacy run. Define and validate its contents as specified in the parent contract; do not infer scoped behavior from `schema: 1`, a run date or a nonempty assignment list. New runs keep schema 1 and save `work_due` as an explicit collection, including an empty collection when complete.

## Shared calendar and responsibility boundary

Use `context/school-calendar.md` through `workspace.ReadTeacherDocument`, preserving configured UserSync authority and the annotated document body. Add a small typed calendar reader and date helpers, rather than a calendar service, database, scheduler or separate term configuration.

The calendar gains `grading_periods: [{name, start_date, end_date}]`. All dates are inclusive local calendar dates in its existing IANA timezone. The source is the teacher's `C:\Users\adamb\Desktop\2026-27_District_Calendar_EN.pdf`, visually checked 2026-10-18:

| Period | Start | End |
|---|---|---|
| Term 1 | 2026-08-19 | 2026-10-08 |
| Term 2 | 2026-10-13 | 2026-12-18 |
| Term 3 | 2027-01-06 | 2027-03-19 |
| Term 4 | 2027-03-22 | 2027-05-27 |

Keep the current `schema: 1`, year coverage, `non_school_dates`, `non_school_ranges` and bell schedule. The existing 2026-27 starter calendar can include these four editable periods; Engine code contains no district dates.

Validate real dates, valid timezone, ordered ranges, calendar coverage and nonoverlapping grading periods. Period resolution requires exactly one period containing the as-of date. Gaps and holidays are real: do not manufacture a containing period, roll into the preceding one or fall back to unrestricted parent reports. Separate basic calendar validation from requiring periods, so a valid holiday calendar can still support explicit-assignment lateness calculation if grading periods have not yet been configured. Missing context affects the operation that requires it; independent grading can continue with known evidence.

This deliberately updates the current “Engine never reads the calendar” boundary. Judgment remains with the playbooks/agent: genuine versus blank/accidental work, teacher waivers, accommodations and correction policy. The Engine gains only mechanical calendar reading, date-window resolution and elapsed-school-time arithmetic. Revise ARCHITECTURE.md, SCORING-RULES.md and Engine format documentation when that implementation lands, rather than leaving contradictory instructions in force.

## School-time calculation

The input is the student's Canvas effective due date, the agent-selected first genuine attempt from the pulled evidence, and any established accommodated deadline. Keep the original Canvas effective due date separately as the write guard; an additional accommodation must not be counted twice. A bare extra-time flag or tag is not a duration.

1. A genuine submission at or before the applicable deadline has zero normalized lateness. This determination needs no holiday calendar; preserve the existing ability to supply on-time timing without one when the evidence/deadline already establish it.
2. Otherwise split the interval between that deadline and the first genuine submission at local midnights. Sum only the intersections with weekdays that are not excluded by the calendar's individual dates or inclusive ranges.
3. School dates mean full dates, not bell-schedule hours. Early release is still a full date. Preserve partial-day time and do not round penalties in CE-SE. Use timezone-aware calendar boundaries, not repeated UTC 24-hour steps.
4. Return `status: none` for zero school seconds; otherwise `status: late` and the exact integer `seconds_late`. Canvas remains responsible for its configured day/hour rounding, percentage and minimum score.
5. Later drafts inherit the first genuine submission's timing. An earlier blank/placeholder does not establish timely work. If that distinction or a relevant file is unreadable, hold timing for that student, explain why, and continue independent grading.
6. Term boundaries govern automatic discovery; they do not reset the lateness clock. For explicitly selected work, count the real interval across terms when the calendar covers it. Parent lookback and correction deadlines remain calendar-day rules; do not convert either to school days.

Concrete fixture: due Friday 2026-10-16 at 18:00 and first genuine work Tuesday 2026-10-20 at 09:00, America/Chicago, gives Friday 6 hours + Monday 24 hours + Tuesday 9 hours = **39 hours / 140400 seconds**, excluding the weekend. Another fixture should cross the existing October 9/12 closures, and one should span Thanksgiving's range. These are arithmetic examples, not a new grace-period policy.

## Scoring-session integration

### Discovery stays read-only

Extend existing pull logic with a small scoring-session option, proposed CLI `pull-work [COURSE] [ASSIGNMENT] --session` and MCP `pull_work {course?, assignment?, group_id?, session: true}`. The normal scoring-session playbook always supplies it; the teacher's copied prompt remains plain and unchanged. Preserve existing bare pull behavior and argument forms for non-session callers. Use an options struct internally rather than adding unrelated positional parameters.

For broad session discovery, default to the current term in the selected course(s), using each student's actual effective due date. The normalization pass covers the current term, **not just the parent report's two-week window**. An explicitly selected assignment remains available for deliberate older-work assessment; do not silently expand its scope to a course. Missing period configuration blocks automatic broad term selection with a specific calendar explanation; explicit work that has sufficient evidence can still proceed.

For every supported submitted row, decide separately whether it needs academic assessment and whether it needs timing review. A graded/locally completed attempt can require timing-only work. Keep grade completion intact; never make all previously handled work appear ungraded.

Expose an Engine-authored optional `work_purpose` on `domain.StudentWorkDocument`: `score`, `late-only`, or `score-and-late` (an absent field keeps the old scoring interpretation). Include observed current late flag, seconds, nullable policy status and whether that status was actually returned, plus existing score/deduction evidence. Extend group member evidence equivalently. For timing candidates obtain the existing detailed history and whatever actual work is needed to choose the genuine submission; reuse existing fetch/download helpers and write a fresh source packet even when grading is already complete. Avoid downloading every handled student's work when returned timing already establishes that no correction is needed.

Do not infer “automatic Canvas default” from an omitted API field. Preserve teacher-set none/extended/missing or other explicit statuses. A prior CE-SE timing override can be reconsidered when its provenance is present in saved local outcomes. An explicit override of unknown origin needs a specific hold, not a guessed replacement. At write time, preserve a teacher's intervening timing change. Academic grade edits alone do not invalidate timing.

### One typed calculation path

Add a batch preparation function and typed MCP tool, proposed `prepare_late_timing`. Input identifies one assignment packet, including a timing-only packet produced by session discovery, and per-student choices: stable student ref, first genuine attempt, optional accommodated deadline and its private basis. For CLI parity, `prepare-late-timing FILE` reads the same packet reference and choices from a small `schema: 1`, `kind: late-timing-decisions` document under `courses/`. A batch never mixes courses or assignments. No raw user IDs or API write payloads are authored by the model.

The helper resolves all identities/attempts from the packet, reads active calendar context, and returns exact `LateTiming` proposals, unchanged rows and item-specific holds. It makes no Canvas write. Batch by packet so an agent does not need a tool call per student.

Add optional structured `school_time` provenance to generated `LateTiming`: selected first-genuine attempt/reference and deadline actually counted. Keep `due_at` as the unchanged Canvas date guard, and keep `basis` private and readable. Legacy explicitly supplied timing remains compatible. On a fresh normalization post, the Engine can recompute from that structured choice and current calendar rather than parsing free-text reasons. A material calendar/input change holds timing for recalculation; unrelated bell-schedule prose must not invalidate it.

### A timing-only action

Extend ordinary `StudentResult` with `operation: late-only` and a `late_only` body containing the current target attempt, calculated `late_timing` and one Engine-authored outcome. It needs no new assessment, rubric interpretation, criterion scores or feedback. Validate mutually exclusive bodies so a timing-only row cannot accidentally carry grade/rubric/comment work.

- If a student is also being graded, attach the proposal to the existing grade row; do not add a second student row or repeat timing.
- If the academic assessment is already handled, use `late-only` exclusively. Its completion must not count as an academic assessment, and existing completed grade outcomes remain completed.
- Review shows current versus proposed timing, skipped school dates and the private accommodation/genuine-attempt basis. A timing-only card has no invented total or empty student-facing comment. Use the existing review state, revision, deselection and post callback; add no separate approval ritual.
- The writer verifies instance, configured course/student/assignment, current attempt, visibility/excused state and unchanged Canvas due date. Recheck only the timing inputs relevant to this action; a harmless rubric/title change must not force reassessment of a timing-only row.
- The only submission fields written are `late_policy_status` and, when late, `seconds_late_override`. No `posted_grade`, rubric assessment, comment, course late-policy edit or assignment deadline/lock change. Canvas may consequently change the displayed penalized score; the entered academic assessment is preserved.
- An already equivalent live timing needs no PUT solely to create a CE-SE override. Persist pending/accepted/held/failed/uncertain outcomes using the existing vocabulary. Use `WriteAccepted` in the shared timing helper and mark an accepted write `succeeded` even with a sparse/empty response; do not require response identity/timing fields or perform a confirmation GET. The existing late helper currently requires a matching returned response; replace that response-shape gate in this touched path. Reconcile genuine uncertain resumption without blind replay.
- A changed attempt/deadline or unavailable calendar holds timing only; independent honest grade/comment actions continue under their existing guards.

The scoring playbook automatically runs this preparation for all session candidates and reports timing corrected/unchanged/held alongside scoring outcomes. “Start a scoring session” still prepares and discovers before asking what to post; “draft” still saves without posting. Ordinary review or an already authorized review skip governs the actual writes. This is automatic inclusion in the workflow, not an unattended background write.

### Existing group and quiz boundaries

Read `docs/group-work/README.md`, `contracts.md` and slices 09-11 before changing these paths. Include the same normalization preparation in group scoring rather than routing a shared assignment through ordinary individual results.

- Individual group-grading mode can reuse per-student `late-only` under `group-results` after its existing live mode/member/attempt checks. Keep the group-specific card keys and skip checks.
- Shared grading needs an explicit shared `late-only` group action through the saved representative, with the full reviewed audience and existing propagation preflight. Keep the pulled `membersShareDue` and fresh `commonDueDate` checks for every member; the final representative-only check is insufficient by itself. One identical timing decision must be true for every affected member, including effective/accommodated deadline and genuine submission. Different/unknown timing holds only that shared timing action while valid assessment continues. Never loop a propagated timing write over members.
- New Quiz written-work comment-only operations keep their no-lateness boundary. Classic Quiz, checkpoint, anonymous and moderated cases retain their existing supported limits; report those cases specifically rather than claiming they were normalized. Do not broaden quiz grade/timing writes as a side effect of this feature.

## Parent-update integration

The detailed contract is [PARENT-REPORT-CURRENT-TERM.md](PARENT-REPORT-CURRENT-TERM.md). Its central change is an authoritative inventory of required assignments whose **student-effective due date** falls in:

`[max(as_of_in_calendar_zone minus 14 calendar days, current_period_start), as_of]`

This replaces the older “all unresolved Missing/0 regardless of age” exception. Due Soon is a separate set after `as_of`, within seven days and before the period's exclusive end. A later submission, score, comment or regrade does not change the due-date selection.

Add a `work_due` collection to each report course, containing every selected assignment and honest status/score availability, including perfect/completed and awaiting-grading work. Required work means published Canvas assignments assigned/visible to that student and not excused; an assignment title or absence of a grade is not an exclusion. Derive the existing Missing/0, Corrections and grade-history views from that inventory. A current grade above zero still resolves stale missing/no-submission flags; a current zero is a zero, not proof of non-submission. Do not invent missing grades for paper work or ungraded assignments. Known required ungraded coursework belongs in the inventory without a numeric grade. Retain release/privacy requirements for family-visible grades and feedback.

The rendered email and printable report must preserve the inventory. Use a compact “Work due in this period” table, retaining first/current score provenance where known and an honest awaiting-grading/no-grade status otherwise; this can replace the narrower Grade history table rather than adding another duplicate table. Keep Missing/0 and Corrections as actionable sections above it, and Due Soon separately. Agent correction choices and short existing-instruction notes remain judgment; the agent cannot drop completed or awaiting-grading inventory rows. First-score history for an included assignment may precede the reporting window; that does not admit an excluded assignment.

Recommended empty-window behavior: when there is no work due in the retrospective window, skip the email with that explicit reason; Due Soon alone does not trigger one. This resolves the existing low-course-score/empty-draft conflict without inventing coursework or pulling older assignments back in. Otherwise retain the configured cutoff rule after scoped Missing/0 classification. Unresolved data flags take precedence over an automatic skip. This is an explicit proposal for the newly empty window, not a claim that the prior threshold rule already allowed it. Keep `canvas_enrollment_current_score` honestly sourced; do not label it a verified term average or calculate a replacement.

Show term and dates in both outputs. Say “No Missing/0 work in this report window” rather than “Nothing is missing.” Save the scope with the run and use those dates on reopening. Reject legacy unscoped runs for new sending/printing with a clear fresh-pull instruction; preserve drafts and outcomes. Parent generation remains read-only and reflects actual Canvas scores, not simulated post-normalization grades. A scoring run can be followed by a fresh parent pull; do not introduce hidden grade writes or require scoring as a prerequisite for a parent report.

## Implementation slices and review points

Implement in this order, with ordinary focused review between coherent changes. These are execution slices, not requests to launch agents now.

| Slice | Deliverable | Main files / checks |
|---|---|---|
| 1. Calendar | Typed reader, current-period resolution, school-time arithmetic, all four editable periods | New small files in `internal/workspace/`; calendar seed; calendar/date fixtures; UserSync routing |
| 2. Parent report | Due-based complete inventory, strict window, separate Due Soon, scoped outputs and validation | `parent_report.go`, `parent_drafts.go`, `parent_send.go` as needed; parent tests; CLI/MCP descriptions; focused contract |
| 3. Timing-only operation | Validated ordinary/group action, review/post/completion integration, exact timing writer | `internal/domain/types.go`, `scoring.go`, `group_results.go`; `post.go`, `late_timing.go`, `outcomes.go`, `review.go`, group counterparts; fake-Canvas tests |
| 4. Session preparation | Distinct scoring/timing discovery, batch calculator and CLI/MCP entry, automatic playbook steps | `pull_work.go`, `group_pull.go`, common DTOs; `cmd/cse/main.go`, `remote.go`; `internal/mcpserver/server.go` and adapter tests; `scripts/check-mcp.mjs` |
| 5. Adoption and release | Consistent instructions, active calendar merge, packaged product and scoped teacher observation | Architecture/scoring rules, format reference/examples, score/parent/setup/install guidance, context seeds, ownership docs, CHANGELOG and capability pages |

At the start of execution inspect the dirty working tree. The parent classification work and its tests/docs predate this plan; preserve and build on it. No checkout reset, mass formatting or unrelated cleanup.

## Verification that demonstrates the requested behavior

- Calendar: select T1/T2/T3/T4 automatically; inclusive endpoints and real gaps; invalid/overlapping dates; Chicago timezone and daylight-saving weekend; active UserSync wins over inactive local context.
- School time: weekend-only exclusion, named holiday, inclusive closure range, partial days, early release as a full day, already on-time work, first genuine earlier attempt and explicit accommodation without double application. Missing necessary evidence holds that timing rather than becoming a zero.
- Discovery: a graded current-term row with an inflated Canvas late duration produces a fresh packet/student-work source with document-level `work_purpose: late-only` and valid history; the batch calculator uses that packet without a second scoring pull. An ungraded row remains score-and-timing; an accepted prior grade is not regraded; a matching timing does not requeue; explicit older assignment scope is preserved.
- Writes: fake Canvas sees only allowed timing fields; teacher-entered grade and comments stay unchanged; manual statuses and intervening timing edits are respected; changed due/attempt holds timing while independent scoring continues; accepted response with sparse body is complete; retry does not repeat accepted work.
- Group paths: one shared timing action reaches one full validated audience; different member timing or a live mode/member/attempt change holds that action. Individual mode affects only its selected member. Review deselection suppresses the real callback action.
- Parent packet/output: every in-window due assignment appears, including perfect, never-regraded, paper/ungraded and awaiting-grading work; old missing/zero/regraded assignments do not. Test all three period transitions, individualized dates, separate upcoming work clipped at term end, explicit visibility requests, incomplete visibility/date evidence, and scoped empty-state wording. An agent's short draft cannot remove an inventory row from either output. Missing `scope` rejects legacy delivery/printing; a valid scoped empty `work_due` collection is a complete empty window, not a legacy report.
- Parent decisions: a low course score with an empty window yields the explicit no-recent-work skip, not an invalid empty send; Due Soon alone does not send. Preserve current classification, privacy, contact routing and saved-outcome behavior with existing fixtures.
- Exercise CLI and compiled MCP preparation -> saved decisions -> review/post using fake Canvas, including a timing-only document without scoring criteria. A plain session prompt and parent prompt need no teacher-supplied term/cutoff/reminder. Do not replace behavior checks with source-text assertions about playbook phrases.

Run affected package tests as each slice lands. Before producing the ZIP, use the existing build/package path and its required full tests, vet and MCP smoke checks. Local fixtures establish mechanics only. A later specifically scoped live observation should show an existing assessment preserved while Canvas recalculates its late deduction, plus a current-term parent packet; sending a family email is not part of that read/check unless the teacher explicitly asks.

## Adoption is part of delivery

Changing the embedded seed is insufficient: ordinary updates never overwrite customized calendars. After implementation, merge the four verified periods into this teacher's active UserSync calendar while retaining its closures, notes and bell schedule. This is the teacher-requested calendar adoption, not a change to the generic update policy. Reconcile the active scoring and parent playbooks through the existing preservation process. Do not change teacher context just because a version was installed.

Keep session packets, decisions, drafts and outcomes in UserInstall. Build in Dev and distribute through the existing ZIP. No Dev executable should become the teacher's permanent runtime. Update both capability surfaces with the implementation date and actual local/live/pending evidence. The Dev implementation now provides calendar-backed scoring-session discovery and late-only preparation/posting, plus scoped complete parent-report inventory. The teacher's active calendar remains a separate adoption step; changing the shipped seed does not overwrite it.

## Execution evidence (2026-10-18)

- Passed in this checkout: `go test ./internal/workflow -count=1` and `go test ./internal/workspace -run 'TestSchoolCalendar' -count=1`.
- The timing agent reports passing: `go test ./internal/domain ./internal/workflow -run 'TestLateTiming|TestLateOnly|TestPostGroupLateOnly|TestSharedGroupLateOnly'`.
- These are focused local checks. No Canvas Live timing correction, current-period parent pull, family email, or destination-client adoption is claimed here.
- 2026-10-18 follow-up: `scripts/build.ps1 -GoExecutable '.\.tools\go1.27.1\go\bin\go.exe'` passed (tests, vet, build) and wrote the ordinary Dev ZIP `dist/CanvasExpert-SE-1.1.0-beta.1.zip` (SHA256 `22b8ffd0a5e4ed69a20cd5e0cf966890541dec5cd41544c3e4c9be79dbefe29a`); `node scripts/check-mcp.mjs dist/cse.exe` passed with 38 tools and no Canvas writes. This is an unsigned local dev build, not a release or teacher-client pilot.
- Added a synthetic check that an accommodated deadline is the counted basis while the Canvas due date stays the write guard (`TestCalculateTimingCountsAccommodatedDeadlineWithoutChangingCanvasGuard`).

The active UserSync calendar adoption was completed separately on 2026-10-18 using the hash-protected workspace write. The four verified grading periods were merged into the connected calendar; removing the inserted periods from the prepared text reproduced the original bytes, preserving its closures, notes and bell schedule. The resulting file hash is `1b254d8b5ec725e22f1e19b1f8d358f67d4499856300274ecca5680d630a42b9`. This records calendar adoption only; the ZIP install and active-playbook update remain pending.

## Source notes

- Calendar: the supplied PDF above; all four ranges were checked visually in this conversation.
- [Canvas submission API](https://developerdocs.instructure.com/services/canvas/resources/submissions) documents the timing-only fields and optional visibility reads. The current report read does not request visibility explicitly; the complete-inventory implementation should request it.
- [Upstream submission model](https://github.com/instructure/canvas-lms/blob/master/app/models/submission.rb) shows default elapsed timing and explicit status/override handling; [late-policy model](https://github.com/instructure/canvas-lms/blob/master/app/models/late_policy.rb) keeps deduction calculation in Canvas. Upstream source supports design, not proof of the district's deployed behavior.

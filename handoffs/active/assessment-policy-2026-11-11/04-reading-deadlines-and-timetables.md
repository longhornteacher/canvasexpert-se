# Slice 04 - Advance reading, four-school-day eligibility and timetables

Status: **not started**. Kind: existing content/date workflows plus narrow timetable support. Depends on [02](02-contracts-and-exact-math.md); integrate release with [03](03-retake-authoring-and-access.md). Read [README](README.md), school-calendar/late-work contracts and current group/team boundaries as applicable.

## Result

Students can read the replacement material ahead of tutorials when practical, without seeing the test. The original Major advertises tutorial retakes rather than copying/reupload credit. Each student's retake deadline is four school days after the student's effective original due date, with established accommodations and closures respected. Dailies retain their existing correction timetable and instructions.

## Advance reading through existing tools

1. The author creates a separate course Page (preferred for accessible reading) or supported uploaded file containing only the replacement passage(s), necessary source attribution and neutral reading directions. A code/data/table-based retake can expose its equivalent material the same way.
2. Use existing Page/file drafts, validation, agentic content review, publication, exact course-file receipts and optional module entry; do not invent a passage delivery service or new asset format.
3. Give the resource an instructional name, such as `Tutorial retake reading - <assignmentname>`. Its relationship is saved in the existing paired assignment expectations note and local authoring receipts, not inferred from its name.
4. The reading resource's publication/audience is independent of the protected quiz. Use the teacher's requested course audience or original audience; do not publish teacher-only/source material into a broader course accidentally. A reading Page being public to the course must not assign the quiz to that course.
5. The quiz can refer to the same passage text/edition. Review the two copies against the authored source so students do not pre-read a different version. Retain accessible text, images, captions and necessary data; do not distribute a broken signed file link.
6. Never include item prompts/options, answer rationales, exact retake model answers, rubric key details that reveal answers, access code or private notes in the pre-reading resource. A generic criterion list is fine if it does not reveal the response.
7. If a suitable passage cannot practically be shared ahead of time, report that particular limitation and preserve the retake. Do not replace a source or require an additional publishing framework merely to satisfy the preference.

## Dates: use the student's original due date

Current anchors: `internal/workspace` school-calendar functions, `internal/domain/types.go` (`LateWorkPolicy`, snapshot/effective-date records), `internal/workflow/timetable.go`, `assignment_dates.go`, personal-date and late-timing workflows; the 2026-11-10 late-work reconciliation handoff.

Build on existing school-day addition and exact effective override reads. Do not add a second calendar calculator. Required provenance per proposed tutorial recipient:

- Canvas instance/course, exact original/companion and stable student ref.
- Student-effective original due date, including the relevant exact override/baseline.
- Calendar/timezone and four-school-day calculation.
- Established accommodation that changes the effective date/window, without disclosing it to peers.
- Resulting deadline, requested tutorial opening/closing and explicit reason for any hold.

Start from the effective original due date, not the class base date, grade-post date, feedback-post date, retake creation or code distribution. Do not give four more days by recreating the companion. If a student's original extension is already reflected in the effective due date, do not add it a second time. Reuse the established accommodation semantics for any separate documented window extension; do not silently invent or remove one.

Keep the original `close_after_school_days: 11`, late cutoff and first-genuine-attempt lateness rules intact. The companion's tutorial deadline is independent of the original's first-submission lock. It must not inherit an earlier ordinary late-work cutoff simply because QuizForge normally derives `lock_at` from the course policy. Set explicit supported companion dates from this retake calculation.

An unknown original due date or calendar gap does not authorize inventing eligibility. Hold that student's date/release request and explain the missing fact; continue passage authoring, feedback and other students. A past-window request gets a specific eligibility hold; the teacher can explicitly grant an exception through the existing exact date/override workflow. Do not quietly treat a new tutorial request as an exception.

## Timetable changes

`buildTimetable` currently always emits `Last day for corrections` when configured, `Timetable.HTML` calls the block late work/corrections, and `refreshTimetable` only regenerates when due/lock dates change.

1. Add the minimal category/retake context to timetable inputs. The Engine renders literal requested policy labels and exact dates; it does not decide whether an assessment is summative.
2. Daily: retain the correction date and existing due/late/close rows. Student directions may advertise rationale/exemplar writing and revisions under the teacher's existing credit arrangement.
3. Original Major: replace the automatic-correction implication with a tutorial-retake deadline and concise policy text: separate parallel form, tutorials only, retake contribution capped at 70%, original grade retained if higher. State that feedback/exemplars prepare for the retake. Do not imply handwriting or another upload immediately restores points.
4. Companion: identify tutorial-only status, its original and applicable release/window facts; do not advertise another retake of the retake or generic Daily corrections. Keep access code out of student HTML.
5. A Major/Daily or original/companion role change must refresh an existing Engine-owned block even if dates are unchanged. Preserve the rest of the teacher's HTML, using the existing marker and narrow replacement rather than regenerating the whole assignment.
6. Do not publish a misleading ordinary block when category is unresolved. Return a specific preview note or hold the policy-dependent block until the agent supplies the classification; late-work facts can still be shown truthfully. Legacy unrelated content is not a reason to mass-edit existing assignments.
7. Ensure preview, create, exact-date update, category move and Draft PDF use the same resolved timetable semantics. A teacher-selected lock date still wins where it is not being contradicted by a specifically requested tutorial window; report a real conflict rather than silently changing it.

## Tests proportional to this change

- A Friday original due date plus a weekday closure yields the correct fourth school day in the configured timezone; reuse the timetable calendar fixture.
- A student's explicit extended due date anchors the four-day count once, without double extra time.
- An undated student-specific override remains undated; a class base due date is not substituted.
- Daily timetable stays unchanged; Major timetable uses tutorial-retake language and no copying-for-points offer.
- A category-only change refreshes the marked block and preserves surrounding HTML. A normal date change still moves the Engine-generated lock as before.
- A companion does not inherit an earlier ordinary late-work cutoff; an expired/unknown eligible date holds only that release.
- One Page/file-plus-quiz fixture proves the reading resource can publish while the quiz remains unpublished/unassigned and the rendered reading contains no private code/key/questions.

The content-review check for passage quality is playbook judgment, not an Engine semantic test. Do not add prose-search tests or a new matrix of every calendar day.

## Acceptance and completion record

Show synthetic preview output for one Daily, one original Major and one tutorial companion, plus one student with an existing extension. Record actual date semantics and unresolved limits. Update the relevant formats/help and both capability records with local evidence; student-view/mobile appearance and an actual tutorial timing observation remain distinct Live checks.

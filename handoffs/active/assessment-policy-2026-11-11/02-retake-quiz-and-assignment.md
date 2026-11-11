# Slice 02 - Retake quiz creation and assignment

Status: **not started**. Kind: Engine authoring and audience. Needs the config from [01](01-playbooks-and-config.md). Read the [README](README.md) first.

## Result

The agent pushes a reviewed parallel quiz as the retake for a named original. The Engine creates it protected and not counting. After the original's grades are posted, one operation assigns the retake to every student who failed, each with their own deadline. The teacher hands out the access code in tutorials.

## A. Create

Extend `push_quiz` (`internal/workflow/quiz_author.go`) with one setting in `metadata.canvas`: `retake_for`, the original's assignment ID in the same course. With it the Engine:

1. Confirms the original exists in the destination course. A written assignment is a valid original; the retake is always a New Quiz.
2. Sets the title to `TUTORIAL-ONLY RETAKE - <original name>` itself, so the title cannot drift from the original's.
3. Records the original's assignment ID in the retake's instructions with the same hidden-marker technique the timetable block uses. This marker, not the title, is the pairing (see below).
4. Requires a non-blank `access_code` and refuses `published: true`, a module placement or multiple attempts.
5. Creates the quiz unpublished, then on the associated assignment sets `only_visible_to_overrides: true` with no overrides, `omit_from_final_grade: true` and `post_to_sis: false`. `cmd/cse-probe/fixtures.go` already writes the last two; read its recorded evidence before relying on them for New Quizzes.
6. Reports each step's outcome using the existing stage records, so a failed step resumes without a second quiz.

A quiz without `retake_for` behaves exactly as today. The original's own creation and publication never wait on the retake.

The access code stays in the draft file and in Canvas quiz settings. It is shown to the teacher and never rendered into a student handout, Page, comment or status output.

## Pairing rule (used by slices 01, 03 and 04 too)

A retake is the quiz in the same course whose instructions carry the marker with the original's assignment ID. Exactly one, or the Engine holds and says what it found. Two originals with the same name therefore cannot share a retake, and a renamed original keeps its retake.

If the build shows New Quiz instructions cannot keep the marker, fall back to the title and require uniqueness in both directions: one retake with that title and one assignment with the original's name. Record which rule was built.

## B. Assign to failing students

One new operation, `retake assign COURSE ORIGINAL` with a preview (MCP tool `retake`, action `assign`). It is run after Major grades are posted and can be re-run as more grades post or after a partial failure.

1. Find the retake by the pairing rule.
2. **Check the retake is still protected**, from a fresh read: visible only to overrides, access code required, omitted from the final grade, SIS off. If any is missing (for example after an interrupted create), set it; if that fails, stop before any override or publish.
3. Read the original's submissions. A student is selected when all of these hold:
   - the grade is posted (`posted_at` is set);
   - there is a graded attempt, not excused;
   - the entered grade before late deduction is below `cap_percent` of the points.
4. Deadline = the student's posted date plus `school_days` on the existing school calendar, plus extra time worked out on the retake window itself:
   - a saved percent is taken of `school_days`, rounded up (50% of 4 is 2 more school days);
   - otherwise the student's saved days, otherwise the general allowance.
   Do not call `extraSchoolDays` as it stands; it measures a percent from the assignment's unlock or creation date (`internal/workflow/personal_dates.go:60`), which for a retake prepared weeks ahead gives far too much.
5. For each selected student, complete three steps, each checked against Canvas so a re-run does only what is unfinished:
   - a single-student override, due and lock both at the deadline, so the retake itself can never be late (skipped when the student already has one);
   - saved quiz accommodations through the existing path;
   - one comment on the student's original: `Tutorial retake open until MM/DD.` (the existing exact-text check prevents a duplicate).
6. Publish the retake if it is not yet published. The access code remains on.

A student whose accommodation step fails is reported plainly: do not give this student the code until it is fixed. The access code is what covers the moment between the override and the accommodation on an already-published retake.

Students are never removed by a re-run. Students with no graded attempt, and students whose calendar or posted date cannot be read, are listed with the reason and skipped. A course with an `owner` holds entirely (README, shared rules).

## Tests

Fake Canvas, existing fixtures:

- Create: an essay original and a quiz original each produce a retake with the derived title, marker, code, empty restricted audience, not counted, SIS off. A normal quiz's payload is unchanged.
- Create resume: a failure after quiz creation resumes without a second quiz.
- **Assign after a partial create** (restriction write failed): assign restores the protected state before publishing, or stops without publishing.
- Two originals with the same name, one retake: each original resolves to its own retake or to none.
- Assign: three students at 52, 75 and 65-shown-45-late; only the first and third are selected. An unposted student is skipped. A second run after that student's grade posts adds only them.
- Assign resume: override written, accommodation failed; the re-run completes the accommodation and comment without a second override.
- Deadline: a posted date before a closure lands on the right school day; 50% extra time on a 4-day window gives 2 more days whatever the quiz's creation date.
- A rendered student handout does not contain the access code.

## Canvas Live, only when asked

On a disposable CS8 quiz with the test account: the marker survives in the quiz instructions, an assigned student enters with the code, an unassigned student cannot see the quiz, and the retake column does not change the course total.

## Done when

Both operations are reachable from CLI and MCP with help and format examples, focused tests pass, and both capability records state local evidence and the pending live check.

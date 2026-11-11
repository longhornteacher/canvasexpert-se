# Slice 03 - Honest-effort floor on New Quizzes

Status: **not started**. Kind: one live probe, then a small Engine change. Independent of the other slices. Read the [README](README.md), `docs/SCORING-RULES.md` and `docs/new-quiz-written-work-spec.md` first.

## Result

When the agent marks a New Quiz student's written work a genuine attempt and the whole quiz total is below the course floor, the Engine raises the total to the floor with fudge points in the same save that writes the written-item points. Item scores stay as earned. The comment says what was earned and what was entered.

Ordinary assignments already have the floor (`internal/domain/effort_floor.go`, `internal/workflow/post.go:636`); nothing changes there.

## Step 1 - Probe (needs an explicit request; it writes to Canvas)

The Engine already sends `fudge_points` in the SpeedGrader result save, unchanged from what it read (`internal/canvas/newquiz_grading.go:269`). Nobody has sent a changed value. On a disposable CS8 quiz with the test account, send one save with a larger value and record:

- the quiz total and the gradebook grade afterward;
- that every item score is unchanged;
- what the student's results view shows;
- what a later item re-score does to the fudge value.

If the write is refused or the total does not follow, stop. The fallback is today's behavior plus one line: the comment states the earned score and the teacher enters the floor in SpeedGrader. Record that in the capability pages and do not build step 2.

## Step 2 - Build

1. **Input.** Add `honest_effort` to the comment-only row that carries `item_scores` (`CommentOnlyOperation` in `internal/domain/types.go`). No default. A row without `item_scores` cannot carry it, so a multiple-choice-only quiz never reaches this path.
2. **Amount.** After the row's item points are known, the expected total is every item's score plus the fudge already on the result. If any item is still ungraded, write the item points and skip the floor for that student, saying so. Otherwise `target = EnteredGrade(total, points_possible, floor, true)`, and the save carries `existing fudge + (target - total)` when that difference is positive.
3. **One save.** Item points and fudge go in the same result write, under the existing result-version guard and outcome record. A total already at or above the floor sends no fudge change.
4. **Comment.** The whole-assignment comment gains one Engine-written line: `Earned 23/100. Entered as 40/100 (honest-effort minimum).`
5. **Later re-score.** If a student's comments already contain that line and a new item write would put the item total above the floor, hold that student and tell the teacher to clear the fudge points in SpeedGrader first. The Engine does not try to work out how much of the fudge is its own.
6. **Rule text.** `docs/SCORING-RULES.md:33` says floors never become fudge points. Change it to: only on an explicit `honest_effort` judgment, never silently.

Teammate-course consent and hidden-grade rules apply unchanged, because this rides the existing item-score write.

## Tests

Existing fake native host and item-score fixtures:

- Mixed quiz, genuine, total 23/100: one save, items unchanged, fudge 17, comment line present.
- Same quiz with teacher fudge 5 already present: fudge becomes 5 plus the remaining difference.
- Total already 55/100: no fudge change.
- Not genuine: no fudge change.
- One written item left ungraded: items written, floor skipped with a reason.
- Re-score above the floor with the floor line present: held.
- A quiz worth 30 points uses 12 as the floor.

## Done when

The probe result is recorded with its date, step 2 is either built and tested or explicitly not built, and both capability records say which, keeping the live probe and local tests apart.

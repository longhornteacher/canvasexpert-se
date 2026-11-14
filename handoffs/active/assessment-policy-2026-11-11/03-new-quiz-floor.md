# Slice 03 - Honest-effort floor on New Quizzes

Status (2026-11-14): superseded in design by scoring batch 2 (`HANDOFF_Scoring_Remaining_Work_for_Claude_Code_2026-11-14.md`): the floor is built in Dev, raise-only, reported from the live total and gated off by `quizFloorWrites` until the probe below runs. Earlier status: **step 1 (live probe) not run, it needs an explicit scoped go-ahead; step 2 not built. Fallback in place: the playbook tells the teacher the number to enter in SpeedGrader**. Kind: one live probe, then a small Engine change. Independent of the other slices. Read the [README](README.md), `docs/SCORING-RULES.md` and `docs/new-quiz-written-work-spec.md` first.

## Result

When the agent marks a New Quiz student's written work a genuine attempt and the whole quiz total is below the course floor, the Engine raises the total to the floor with fudge points in the same save that writes the written-item points. Item scores stay as earned. The comment says what was earned, what was entered and how many points the Engine added.

Ordinary assignments already have the floor (`internal/domain/effort_floor.go`, `internal/workflow/post.go:636`); nothing changes there.

## Step 1 - Probe (needs an explicit request; it writes to Canvas)

The Engine already sends `fudge_points` in the SpeedGrader result save, unchanged from what it read (`internal/canvas/newquiz_grading.go:269`). Nobody has sent a changed value. On a disposable CS8 quiz with the test account, **built so its item points and its gradebook points differ** (for example 10 item points worth 100), send one save with a larger value and record:

- the quiz total and the gradebook grade afterward, and so which scale fudge points are counted in;
- that every item score is unchanged;
- what the student's results view shows;
- what a later item re-score does to the fudge value.

If the write is refused or the total does not follow, stop. The fallback is today's behavior plus one line: the comment states the earned score and the teacher enters the floor in SpeedGrader. Record that in the capability pages and do not build step 2.

## Step 2 - Build

All arithmetic is in the quiz result's own units (the result's `score` and `points_possible`), on the scale the probe established. The floor is a percent, so it needs no gradebook conversion.

1. **Input.** Add `honest_effort` to the comment-only row that carries `item_scores` (`CommentOnlyOperation` in `internal/domain/types.go`). No default. A row without `item_scores` cannot carry it, so a multiple-choice-only quiz never reaches this path.
2. **Not on a retake.** Refuse the floor on a quiz that is a retake by the slice 02 pairing rule. A retake's effort judgment goes to `retake apply` (slice 04), and its entered score must stay the earned score.
3. **Amount.** After the row's item points are known, `total` is every item's score plus the fudge already on the result. If any item is still ungraded, write the item points and skip the floor for that student, saying so. Otherwise `target = EnteredGrade(total, points_possible, floor, true)` and `added = target - total`; when positive the save carries `existing fudge + added`.
4. **One save.** Item points and fudge go in the same result write, under the existing result-version guard and outcome record. A total already at or above the floor sends no fudge change.
5. **Comment.** The whole-assignment comment gains one Engine-written line that records the amount: `Earned 23/100. Entered as 40/100 (honest-effort minimum, +17).`
6. **Later re-score.** When a student's comments contain that line, written by a teacher account and not by the student, the Engine reads its own earlier amount from the most recent one:
   - if the fudge now on the result is smaller than that amount, someone changed it: hold that student and say so;
   - otherwise remove the earlier amount, recompute `added` from the new item total, and write the result. Any other fudge the teacher gave is kept.
   So 23 re-scored to 30 becomes 30 + 10 = 40, not 47, and a re-score to 55 leaves no Engine fudge.
7. **Rule text.** `docs/SCORING-RULES.md:33` says floors never become fudge points. Change it to: only on an explicit `honest_effort` judgment, never silently.

Teammate-course consent and hidden-grade rules apply unchanged, because this rides the existing item-score write.

## Tests

Existing fake native host and item-score fixtures:

- Mixed quiz, genuine, total 23/100: one save, items unchanged, fudge 17, comment line with +17.
- Same quiz with teacher fudge 5 already present: total 28, fudge becomes 17.
- Total already 55/100: no fudge change. Not genuine: no fudge change.
- One written item left ungraded: items written, floor skipped with a reason.
- Re-score 23 to 30 after a +17 floor: fudge becomes 10. Re-score to 55: Engine fudge removed, teacher fudge kept.
- Floor line present but fudge on the result is below the recorded amount: held.
- A line with the same wording authored by the student is ignored.
- A quiz with 10 item points worth 100 gradebook points uses the scale the probe found.
- A retake quiz refuses the floor.

## Done when

The probe result is recorded with its date, step 2 is either built and tested or explicitly not built, and both capability records say which, keeping the live probe and local tests apart.

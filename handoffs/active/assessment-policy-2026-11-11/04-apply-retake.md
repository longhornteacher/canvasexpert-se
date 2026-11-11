# Slice 04 - Apply the retake to the original

Status: **not started**. Kind: Engine scoring operation. Needs [02](02-retake-quiz-and-assignment.md); needs [03](03-new-quiz-floor.md) only when the original is a New Quiz. Read the [README](README.md) and `docs/SCORING-RULES.md` first.

## Result

After retakes are scored, one operation raises each student's original grade by the README formula, never lowers it, and leaves a comment saying what happened. The teacher approves the numbers that are written. A later re-score of the original cannot silently take awarded credit away.

## Operation

`retake apply COURSE ORIGINAL` with a preview (MCP tool `retake`, action `apply`). Optional input: the student refs whose written retake the agent judged a genuine attempt.

1. Find the retake by the slice 02 pairing rule.
2. For each student with a submitted retake:
   - **Retake score.** The retake must be fully graded. Use its entered score over its points. Pending written items hold that student.
   - **Original.** Read the entered grade before late deduction. Unposted, excused or ungraded originals hold that student.
   - **Compute** `target` with the README formula, from the configured cap and the course floor.
3. **Preview** lists each student by alias: retake earned, original entered, target, and raise, keep or hold with the reason. It saves those reviewed values (student ref, both attempts, both scores, target) in one small file in the course's drafts folder. The agent reports the list in the normal check-in.
4. **Apply** reads the saved preview and fresh Canvas values. A student whose retake attempt or score, or original attempt or entered grade, differs from the preview is held for a fresh preview; the others continue. With no saved preview on this PC, apply runs the preview and stops.
5. **Write order per student: comment first, then grade.** If the existing client can send both in one submission update, use that.
   - **Raise** (target above the entered grade): post the `Retake credit:` comment, then the grade.
     - Ordinary original: grade only. No rubric assessment is sent; the original's rubric and feedback stay as scored.
     - New Quiz original: add the difference as fudge points through slice 03's write, converted from gradebook points to the quiz result's units on the scale slice 03's probe established. If that write is not available, hold and give the teacher the number to enter in SpeedGrader, in SpeedGrader's units.
   - **Keep** (target not above): post the keep comment only.

### Comments

- Raise: `Retake credit: your tutorial retake earned 88/100. Retake credit is capped at 70%, so this grade is now entered as 70/100.`
- Keep: `Tutorial retake: you earned 54/100. Your grade stays 65/100.`
- When the original has a late deduction, either ends: `Your late deduction still applies.`

Only a raise uses the `Retake credit:` wording, because that line is what protects the award.

Canvas applies the existing late deduction to the new entered grade. The Engine sends no timing change and computes no penalty.

### Re-runs and retries

- A student who already has a teacher-authored `Retake credit:` line gets no second comment. If their entered grade is still below the amount in that line (the comment landed and the grade did not), the grade is written.
- A student with a keep comment already present gets nothing (exact-text check).
- Nothing is ever written twice, on this PC or another, because both checks read Canvas.

## Hold against later lowering

`docs/SCORING-RULES.md:3` says the Engine always posts the computed assessment, even below an existing grade. That would return a 70 to 52 if the original were scored again. Add one check in the grade post path (`internal/workflow/post.go`) and the item-score path:

- find the most recent `Retake credit:` line on the submission written by a teacher account, never one written by the student;
- read the entered amount from it (70 in the example);
- if the grade about to be entered is below that amount, hold that student with "retake credit of 70 applied; not lowered".

A grade at or above the amount posts normally. A keep comment protects nothing, so an honest correction from 65 to 60 posts. A later unrelated raise to 80 does not move the protected amount; a re-score to 75 after that posts. Update the rule text to state the exception.

## Not handled

- **Shared-grade group originals.** One student's retake must not raise the whole group. Hold and name the manual step.
- **Team-taught courses.** Held (README, shared rules).
- **A second retake.** The retake quiz has one attempt.

## Tests

- Formula: the README table plus one fractional-points case, as a domain table test.
- Ordinary original, fake Canvas: 52 with retake 88 posts one comment and grade 70 with no rubric payload; 65 with retake 54 posts only the keep comment.
- Re-run after success writes nothing: no second comment, no grade.
- Comment lands, grade write fails; the retry writes only the grade.
- Changed since preview (original resubmitted, or either score edited): that student is held, another proceeds.
- A late original: the posted value is the entered grade, and the comment carries the late line.
- Pending written items, unposted original and an unpaired retake each hold only what they should.
- Hold: re-score at 52 after credit to 70 is held; re-score at 80 posts; after a keep comment a correction from 65 to 60 posts; a student-typed `Retake credit:` line protects nothing.
- Full path for a written retake: scored at 10/100 with no floor on the retake itself, judged genuine at apply, original 25 becomes 40, and the comment says the retake earned 10.
- New Quiz original with 10 item points worth 100: the fudge difference is converted, written once, teacher fudge kept. Skip if slice 03 was not built, and test the hold instead.
- Shared-grade group original holds.

## Canvas Live, only when asked

On CS8 with the test account: an ordinary original at 52 with a retake at 88 shows 70, a late original keeps its deduction, and the comment displays as written.

## Done when

The operation is reachable from CLI and MCP with help and examples, tests pass, `docs/SCORING-RULES.md` states the formula and the hold, and both capability records separate local evidence from the pending live check.

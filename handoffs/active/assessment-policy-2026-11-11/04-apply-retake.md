# Slice 04 - Apply the retake to the original

Status: **not started**. Kind: Engine scoring operation. Needs [02](02-retake-quiz-and-assignment.md); needs [03](03-new-quiz-floor.md) only when the original is a New Quiz. Read the [README](README.md) and `docs/SCORING-RULES.md` first.

## Result

After retakes are scored, one operation raises each student's original grade by the README formula, never lowers it, and leaves a comment saying what happened. A later re-score of the original cannot silently take the credit away.

## Operation

`retake apply COURSE ORIGINAL` with a preview (MCP tool `retake`, action `apply`). Optional input: the student refs whose written retake the agent judged a genuine attempt. There is no results document and no saved baseline; the Engine reads Canvas, computes, and writes.

1. Find the retake by exact title. Zero or several matches: stop.
2. For each student with a submitted retake:
   - **Retake score.** The retake must be fully graded. Use its entered score over its points. Pending written items hold that student.
   - **Original.** Read the entered grade before late deduction. Unposted, excused or ungraded originals hold that student.
   - **Compute** `target` with the README formula, from the configured cap and the course floor.
3. **Preview** lists each student by alias: retake earned, original entered, target, and raise, keep or hold with the reason. The agent reports this in the normal check-in.
4. **Write**, recomputing from fresh reads. A student whose result differs from the preview is written at the fresh value and flagged in the output.
   - Target above the current entered grade, ordinary original: post the grade only. No rubric assessment is sent; the original's rubric and feedback stay as scored.
   - Target above the current entered grade, New Quiz original: add the difference as fudge points through slice 03's write. If slice 03's write is not available, hold and give the teacher the number to enter in SpeedGrader.
   - Target not above: no grade write.
5. **Comment** on the original, one Engine-written line starting `Retake credit:`. Examples:
   - `Retake credit: your tutorial retake earned 88/100. Retake credit is capped at 70%, so this grade is now entered as 70/100.`
   - `Retake credit: your tutorial retake earned 54/100. Your grade stays 65/100.`
   - When the original has a late deduction, add: `Your late deduction still applies.`

Canvas applies the existing late deduction to the new entered grade. The Engine sends no timing change and computes no penalty.

A re-run writes nothing new: the target is no longer above the entered grade and the exact comment is already present.

## Hold against later lowering

`docs/SCORING-RULES.md:3` says the Engine always posts the computed assessment, even below an existing grade. That would return a 70 to 52 if the original were scored again. Add one check in the grade post path (`internal/workflow/post.go`) and the item-score path: when a student's comments contain a `Retake credit:` line and the grade about to be entered is lower than the live entered grade, hold that student with "retake credit applied; not lowered". A higher grade posts normally. Update the rule text to state the exception.

## Not handled

- **Shared-grade group originals.** One student's retake must not raise the whole group. Hold and name the manual step.
- **Team-taught courses.** Held (README, shared rules).
- **A second retake.** The retake quiz has one attempt.

## Tests

- Formula: the README table plus one fractional-points case, as a domain table test.
- Ordinary original, fake Canvas: 52 with retake 88 posts grade 70 with no rubric payload and one comment; 65 with retake 54 posts only the comment; a re-run writes nothing.
- A late original: the posted value is the entered grade, and the comment carries the late line.
- Pending written items, unposted original and missing title match each hold only what they should.
- Later re-score at 52 after credit to 70 is held; a re-score at 80 posts.
- New Quiz original: fudge difference written once; teacher fudge kept. Skip if slice 03 was not built, and test the hold instead.
- Shared-grade group original holds.

## Canvas Live, only when asked

On CS8 with the test account: an ordinary original at 52 with a retake at 88 shows 70, a late original keeps its deduction, and the comment displays as written.

## Done when

The operation is reachable from CLI and MCP with help and examples, tests pass, `docs/SCORING-RULES.md` states the formula and the hold, and both capability records separate local evidence from the pending live check.

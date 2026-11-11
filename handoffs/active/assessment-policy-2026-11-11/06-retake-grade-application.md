# Slice 06 - Apply retake results to the original Major

Status: **not started**. Kind: focused Engine scoring operation. Depends on [02](02-contracts-and-exact-math.md), [03](03-retake-authoring-and-access.md), [04](04-reading-deadlines-and-timetables.md) and [05](05-honest-effort-floor.md). Read [README](README.md), scoring rules, team consent/routing rules and group contracts.

## Result

After a genuine tutorial retake is scored, the Engine can prepare and apply its contribution to the exact original Major. It uses the honest result, honors the 40% effort minimum, caps the contribution at 70%, retains a higher original grade and leaves original rubric/item evidence honest. The teacher sees earned retake result, proposed original change and any held students in the normal conversation check-in.

This is a separate original-grade application, not a second assessment pretending the original response improved. It does not change the original's item answers, rubric criteria, due dates or first-genuine-attempt lateness. The companion stays non-counting/SIS-off. A raw 88 retake is not rewritten to 70 in its items.

## Reachable contract and preparation

Implement the `retake-results` contract from slice 02 through the existing document validation, result preparation and post surfaces. A small dedicated preparation command/tool is justified only if an existing pull/init route cannot supply the necessary two-assignment baselines cleanly; do not expose an arbitrary read/write endpoint.

Preparation takes the confirmed companion and selected stable student refs; resolves its saved original relationship; reads the current original and retake through the same configured course route; and saves a local draft with:

- Confirmed Canvas instance, course, original and companion IDs/points and original Major classification.
- Source current attempt/session/result version, full graded status and complete academic score/scale. Retain earned item total, existing teacher adjustments and CE-SE floor adjustment separately where the protocol exposes them; never call a policy delta an earned response point.
- Original current attempt, known entered academic grade/known empty/unknown status, observed score/graded-at or quiz result version, assignment visibility/excused state and relevant first-attempt timing baseline.
- Source eligible window and exact original-effective-date evidence from slice 04; an expired/unknown window is a specific hold unless an explicit teacher exception was applied.
- Explicit genuine-effort judgment, copied only from reviewed source scoring or supplied by the agent from the pulled evidence; no implicit true.
- Engine-computed earned result, floor-adjusted source value, capped original candidate, preserved-or-raised original target and reasons. These are output, not freeform authored final grades.
- Separate source-score, target-grade and explanatory-comment outcomes. Do not mix accepted source work with a failed original application.

The proposal should be useful in `validate_document`/CLI validation, a selected student's exact feedback sample and an optional teacher Draft PDF. Student handouts do not expose private baselines/codes/notes.

## Write sequence

1. Finish any supported truthful source item scoring and explicit floor adjustment first, or prepare from an already-complete current retake result. Incomplete written responses/unknown scores do not become zeros. Preserve source comments/grades if later target work holds.
2. Before the first target write, renew confirmed relationship, same-course origin, current student/attempt, original points/rubric identity as relevant, recipient/excused state and owned-course consent/posting policy. A title is presentation only; a renamed correctly linked original is not a new target.
3. Re-read the current source result version/attempt and current original entered grade/version. A student who submitted another retake or whose original was edited in SpeedGrader since preparation holds for fresh preparation. Do not post the stale proposed number or silently recompute a new unreviewed proposal after an intervening teacher edit.
4. Use the exact domain formula. Compare normalized original entered academic points, not an unreliably reconstructed pre-penalty grade. Preserve the current grade if it is higher. A target already matching/exceeding the candidate records `preserved` and sends no numeric write.
5. For an ordinary individual original, send only the supported grade-entry field for the computed target. Do not post a new rubric assessment and do not overwrite the student's actual original rubric or feedback. Keep numeric and explanatory comment actions distinct.
6. For a New Quiz original, use the established grade-adjustment mechanism from slice 05. Keep its earned item scores and previous teacher adjustments intact. Add only the retake-policy delta needed to reach the confirmed target, record it distinctly and preserve the result-version guard. Do not call the written-item scoring path with fictitious item allocations to distribute retake credit.
7. Original late status stays anchored to the first genuine original submission. Do not mark the tutorial as a new late original attempt or resend its dates/timing incidentally. If a grade-entry mechanism changes Canvas late handling, establish the interaction using the supported route and hold that numeric operation if it cannot preserve the intended semantics. Never locally invent Canvas's penalty or promise a 40 final grade after penalty.
8. Post one concise explanatory original-assignment comment when authorized: source retake/earned result, honest-effort adjustment when applicable, the 70 cap, the applied or retained original grade. Example: `Your tutorial retake earned 88/100. Retake credit is capped at 70%, so this Major is now entered as 70/100.` A retained 84 says it stays 84. Keep actual raw/adjusted values truthful; no names, aliases, accommodation details or teacher-only concerns.
9. Follow the original's owned-course consent and hidden-posting policy for both grade and comment. If a course has an owner, the original target must satisfy its scoring tag; check source consent too when scoring source items. Do not modify the required exact title to smuggle consent into copied work. An applicable consent conflict is reported for teacher action.
10. Persist each accepted stage using existing outcomes. A failed explanatory comment does not repeat an accepted grade adjustment; a failed target grade does not regrade the source. For uncertain non-idempotent native result creation, reconcile the recorded source/target versions and intended policy delta before any repeat. Do not build a separate synchronized ledger.

## Group originals

A tutorial retake is individual. If the original uses individual group grading, use that student's supported grade route and retain the existing group/attempt guards. If the original is genuinely shared-grade mode, an individual retake must never propagate its award to every group member.

Do not silently toggle `grade_group_students_individually`, fabricate an individual original or issue a representative shared-grade write for one student's retake. With current shared-grade semantics, hold the individual original-grade application and give the teacher the exact manual Canvas step/limitation; source retake scoring and independent applications continue. If a supported member-only override route is actually demonstrated during execution, implement only that narrow route with proof that peers are unchanged. This is a consequential wrong-student-write boundary, not a reason to build speculative group migration machinery or freeze all Major authoring.

## Focused acceptance cases

1. Original 52/100, earned retake 88 -> source items remain 88; original entered 70; original rubric unchanged; one explanatory comment.
2. Original 65/100, retake 54 -> original remains 65; numeric write skipped; honest source/result retained.
3. Original 84/100, retake 96 -> original remains 84, not lowered to 70.
4. Original 25/100, genuine retake 10 -> original 40; non-effort retake 10 gives no floor or lowering.
5. Original 12/30, retake 18/20 -> original 21/30 with one final rounding. Add a fractional case to reuse slice 02's arithmetic proof.
6. Ordinary written original and New Quiz original each follow their supported target mechanism; neither receives invented rubric/item scores.
7. Changed original grade or source result version holds only that student; an independent ready student succeeds. Wrong-course copied association holds before write.
8. A genuinely known empty original can receive the computed candidate. Unknown entered score, excused/hidden recipient or ungraded source holds with a precise explanation.
9. A prior late original keeps its timing; the later tutorial adds no new lateness. Check reported grade/entered-score distinction without reimplementing Canvas penalties.
10. Accepted target grade plus failed comment resumes only the comment. An uncertain native result is not applied twice.
11. A shared-group original never receives a propagated retake award for one member. An individually graded group original changes only the selected student.
12. Two-assignment owned-course fixtures route correctly and preserve hidden grades/consent boundaries.

Use existing stateful fake fixtures and a small number of integrated cases. Tests must inspect actual payloads and preserved state, not just proposed arithmetic or text labels.

## Completion

Document exact CLI/MCP preparation/post/validate usage, output sample and manual limitation where applicable. Update the original/retake context cross-reference procedure without copying student result rows into UserSync. Keep formats/examples, scoring contract, playbooks and both capability records consistent. Mark native grade adjustment, student display and late-policy effects with the evidence actually obtained, never just `supports retakes` without limits.

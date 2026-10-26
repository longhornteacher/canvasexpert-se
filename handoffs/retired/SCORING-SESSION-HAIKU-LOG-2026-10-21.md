# Haiku scoring session log, 2026-10-21 (second run)

Source: the Haiku agent's own final report, relayed by the supervising Sonnet session. Nothing here was independently checked against Canvas or the result files. This run replaces the log of the first run the same day (older Dev build, built before 17:51Z), whose findings are summarized under "Compared with the first run".

Setup: Dev build `236a20f` (built 2026-10-21T17:51Z, development), seed playbooks identical to installed. Teacher scope: all courses (ELA7, PAP ELA7, CS8), post directly, no review page, this task only. 44 tool calls, ~3m49s agent runtime, ~184k subagent tokens.

## Pull

`pull_work --session`: pending once, then complete. Discovery 17.3 s (16 requests), fetch/download 33.0 s (124 requests). 24 students in 5 packets across 7 assignments needing attention.

## Outcome per assignment

| Course / assignment | Group | Due | Canvas needs grading | Pulled | Posted | Held | Comment posted, item points not entered |
|---|---|---|---|---|---|---|---|
| CS8 Hotline Miami (3698355) | Daily | past due | 1 | 1 | 0 | 1 | |
| CS8 Stardew Valley (3703860) | Daily | not yet due (today 4 pm) | 20 | 20 | 0 | 20 | |
| LA7 Anticipation (3705769) | Daily | past due | 17 | 0 | 0 | 0 | 17 |
| LA7 Moment of Truth (3705777) | Daily | not yet due | 7 | 0 | 0 | 0 | 7 |
| PAP First Impressions (3705186), graded discussion | Daily | past due | 1 | 1 | 1 | 0 | |
| PAP Moment of Truth (3705779) | Daily | not yet due | 14 | 1 | 1 | 0 | 13 before this run, 14 now |
| PAP Anticipation (3705775) | Daily | past due | 21 | 1 | 1 | 0 | 20 before this run, 21 now |

In every row Canvas needs-grading equals pulled plus comment-posted, so no student with an Engine comment on the current attempt was pulled again.

## Writes made

- PAP First Impressions: one student 86/100, late 72,264 s (~20 h). Grade, comment and late status succeeded. Basis: four criteria the agent defined (characters 30/30, beginning 25, sentences and parts 15); the assignment has no Canvas rubric.
- PAP Moment of Truth: one comment-only result, 21/30 across two written items (5/5/5 from item prompts).
- PAP Anticipation: one comment-only result, 24/30 across three items (criteria defined by the agent from the prompts). The comment notes the missing "My claim that ___ is substantiated by ___" sentence in item 3.
- Results files under `C:\Users\adamb\CanvasExpert-SE\courses\ELA7 PAP\assignments\` (`3705186-first-impressions`, `3705779-the-terror-moment-of-truth-reading-and-response`, `3705775-the-terror-anticipation-writing`, each `results-20261021.md`). `post_results` rewrote each file with normalized fields and outcomes; the rewritten copy is the record.

## Held / waiting on the teacher

- Hotline Miami (1) and Stardew Valley (20): the reference code exists only as an image on the assignment, so exact match could not be scored. Hotline's upload reads as a Minecraft review page, not a transcription; the teacher should look directly. Stardew was also not yet due; a resubmission would re-pull as a new attempt.
- Item points to enter in SpeedGrader: LA7 Anticipation 17, LA7 Moment of Truth 7, PAP Moment of Truth 14, PAP Anticipation 21.
- Spot-check needed: the 86/100 First Impressions grade and the agent-defined scoring bases (no Canvas rubrics).

## Findings and friction

1. The format reference does not say a graded discussion's assignment snapshot needs `discussion_topic_id`; validation failed with "evidence is only for graded discussions" until the agent copied it from the packet.
2. The Engine cannot read assignment images, which blocks scoring image-reference code (Stardew Valley, Hotline Miami).
3. No Canvas rubric on First Impressions or the New Quizzes, so scoring bases were invented by the agent.

## Compared with the first run

| | First run (older build) | Second run (this build) |
|---|---|---|
| Pull time | ~3m30s, 5 pending polls | ~50 s, 1 pending poll |
| Pulled | 103 students, 11 assignments | 24 students, 5 packets |
| Re-pull of students already commented | 23 pulled again (agent caught it) | none |
| Late-hold "no accepted timing record" for already-graded work | 9 held | none |
| Comment-only completion shown | no | "comment posted, item points not entered" rows |

The first run's other findings (the `group-work.md` path in `score.md`, no results-file generator, 50 KB packet needing `allow_large`, actionable late-hold message, scoring-basis template for rubric-less New Quizzes) were not raised or retested in the second run and are not confirmed fixed.

## Live-validation relevance

Observed in this run: New Quiz comment-only rows now suppress re-pull and report as "item points not entered" with counts matching Canvas needs-grading; a graded discussion with an ungraded entry was pulled, scored and posted. Not observed: needs-grading count before/after a comment (counts were read once), a resubmission, a group assignment. Record only what this log supports in `internal/localui/capabilities.go` and `docs/engine-capabilities.md`.

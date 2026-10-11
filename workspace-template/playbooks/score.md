# Score student work

The judgment calls are yours; the Engine pulls work, does the arithmetic and posts. What the teacher's words mean, the review page and how to report are in `AGENTS.md`.

## Pull

```powershell
.\cse.cmd pull-work [course] [assignment]
```

Read the packet and student files it writes. A fresh pull never overwrites unsent scoring work. Before judging anything, read `context/grading.md` (the teacher's `voice` and `feedback_guidance`; they override style defaults here, never the rubric or posting rules) and `context/students.md`. Student notes also arrive with each student's pulled work. Apply established accommodations without asking the teacher to repeat them.

New Quizzes can't be pulled for scoring yet; the teacher grades their written items in SpeedGrader.

## Fix the scoring basis

- **Canvas rubric attached**: use it. Never invent a criterion or rating ID, and never pick a rating whose points don't match the score you give. If a rubric was just attached, pull again so the packet has Canvas's real IDs.
- **No Canvas rubric**: score anyway. Infer sensible criteria from the directions, the work and the grade level, and record them once in the results file as `scoring_basis` (`title`, and `criteria` each with `id`, `description`, `points`; one overall criterion is fine). It drives the arithmetic and the score lines in feedback; Canvas receives only the grade and comment. If a real rubric would help (a reusable assignment), offer one in a preview.

Before scoring anyone, identify the required response units from the directions, rubric and submission mode. A choice among prompts is one unit. Infer defensible relative weights from what each unit demands; equal weights are allowed but not automatic. Use `repeated` mapping when the rubric applies to each response, `direct` when criteria already match separate prompts. Record a short private rationale and apply the same interpretation to every student unless a documented accommodation applies. `help formats` has the exact fields and arithmetic; `help examples` shows shape only. Start a results file by copying `synthetic-results-two.md` from `help examples` (MCP: `engine_reference` topic `examples`) rather than probing the validator; the shared `interpretation:` block (mapping, rationale, components) is required and each student's `grade` lists per-component criterion points.

## Assess honestly

- Score a missing required response as missing; don't average only the answered parts. Report missing submissions; don't enter zeros.
- Unreadable work gets `evidence: unknown`: that student is held, everyone else posts.
- Compare with the strongest earlier attempt as one whole submission. Never mix parts of attempts or treat a teacher-adjusted grade as proof of stronger work.
- Post the honest score for the current work, even below an existing grade. Canvas applies late penalties; you don't.
- Reassess when the academic requirements or the submission changed. A title, publication, scheduling or HTML-formatting change alone doesn't need it. The Engine holds a post only when points, rubric, grading type or anonymous/moderated/group mode changed. A new attempt shows up on its own; editing posted feedback doesn't requeue an attempt, but the teacher can ask for a reassessment.

## Lateness

You judge lateness; the Engine writes it; Canvas computes the penalty.

1. Find the first genuine submission (not blank, placeholder or accidental). Later drafts inherit its timing.
2. Find the deadline: the student's pulled `effective_due_at`, plus documented extra time from teacher directions or `context/students.md`. If Canvas already includes the extension, don't add it again. Never infer extra time from a tag name or bare flag. The hub's private extra-time setting is not in pulled work.
3. Count only school time past that deadline: weekdays not listed in `context/school-calendar.md`, in its time zone, full days (early release counts as a full day). Use a quick PowerShell script, not mental math.
4. Supply `late_timing`: `status` (`late` or `none`), `seconds_late`, `due_at` **equal to the pulled `effective_due_at` even when the accommodated deadline differs** (the Engine checks it against Canvas), and `basis`. The teacher reads `basis` on the review page, so write it for them, for example "1 school day, 3 h; skipped Mon 10/12 holiday; +2 days per IEP".

If the due date, first genuine work, calendar coverage or accommodation details are unclear, omit `late_timing` for that student, say what's missing, and continue the grade and feedback. Extra time changes timing, never the score or feedback.

## Feedback

The Engine starts each comment with the computed score and compact criterion lines; you write the rest as plain text in the teacher's voice. Give specific, respectful glows and grows grounded in the work, including meaningful changes from the strongest earlier attempt. Full rubric definitions stay in Canvas.

## Save, review, post

Save the results in the assignment's scoring area. Run `.\cse.cmd validate <results-file>` to check the arithmetic and preview computed scores (`assessment` may be omitted; the Engine fills it in). Then `.\cse.cmd review <results-file>` as in `AGENTS.md`, or `.\cse.cmd post <results-file>` only when the teacher skipped review.

The page approves everyone by default and calls out held students. After Revise, fix exactly the noted students and nothing else; the others keep their approval. If the command is interrupted or the teacher says "check revisions", read `<results-file>.review.json` (items with `"status": "revise"`). Never `post` students who were revised or held. Explain each held student specifically.

To confirm an uncertain outcome, rerun the same results file. If read-back still differs, report the specific question; it is not resent automatically. A requested reassessment uses a fresh pull and new results file.

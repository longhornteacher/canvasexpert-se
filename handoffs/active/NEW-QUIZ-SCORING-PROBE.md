# New Quiz written work: whole-quiz feedback (plan)

2026-10-10. **Teacher decision:** for now CE-SE gives **whole-quiz commentary** on New Quiz written work. **The teacher enters the per-item points** in SpeedGrader. CE-SE writes no item scores, no rubric assessment and no grade for these quizzes. This replaces the earlier five-question probe plan (item-score writes, rubric carrier, overall-grade honesty, participant fixtures): none of it is needed for this scope. The rubric pre-check result stays on record in `docs/engine-capabilities.md`.

## What the feature is

The teacher asks the agent to look at students' written responses in a New Quiz and leave feedback. The agent reads each response, assesses it, and the teacher reviews on the review page. Post writes one **submission comment per student** on the quiz's assignment, in the teacher's voice, and nothing else. No grade is touched. Review-before-post still applies, and `SCORING-RULES.md` still governs the comment text (honest, no invented points).

## Two questions the Engine needs answered

Both run against a **real New Quiz that already has student responses**; neither needs a fixture, a participant, or a publish.

**A. Can CE-SE read the responses?** Read-only, teacher's real quiz. Try, cheapest first: (a) REST submissions on the quiz assignment with `include[]=submission_history`; (b) the Student Analysis report, JSON (`POST /api/quiz/v1/courses/:c/quizzes/:a/reports`, then poll); (c) GraphQL on the submission. Record per source: essay text present, file-upload response present and downloadable, attempt number, student identity, errors. If none returns the written text, stop: the fallback is feedback from responses the teacher pastes in.

**B. Does an ordinary submission comment work on a New Quiz submission?** The teacher approves one specific comment on one specific real submission; it is the normal REST comment the Engine already posts for assignments. Check it lands on the intended attempt, shows in SpeedGrader, and that the quiz grade is untouched. The teacher can delete it afterwards in SpeedGrader. Skip B if the teacher prefers: it is the same call the Engine already makes for assignments, so the risk is only quiz-specific attempt targeting.

## Prior art (original CanvasExpert)

The original concluded Canvas has **no public API for writing New Quiz item scores or per-item feedback**, and told teachers to grade New Quiz writing in Canvas. Useful for A and B:

- **Student Analysis report** (A, option b): OG read responses this way, or from the UI's CSV download (`api/powergrader/new_quiz_fetch.py`). It saw transient 502s and **no rows for Test Student submissions**, so probe with real student work.
- **Draft comments** (alternative for B): GraphQL `createSubmissionComment(draftComment: true, attempt: N)` creates a teacher-only draft the teacher finalizes in SpeedGrader. Live-probed by OG (`docs/reference/classic-quiz-design.md`, "New Quiz draft comments"). Consider it if the teacher wants to see comments in SpeedGrader before students do.

## Engine changes if A succeeds

- `UnsupportedAssignment` (`internal/workflow/canvas_reads.go`) currently skips every quiz assignment ("quizzes aren't supported yet"). New Quiz assignments (submission type `external_tool`, `is_quiz_lti_assignment`) need a **comment-only** path: pull written responses (per attempt, with student identity checks), and a results mode that posts comments and never a grade. The existing grade-posting assignment path stays as is.
- Playbook: when to use this (New Quiz written items), that the teacher enters item points, and that CE-SE never claims a score for these quizzes.
- Capabilities page and `docs/engine-capabilities.md` updated together, with evidence levels kept apart.

## Rules

Read-only against the teacher's real quizzes until the teacher approves a specific comment. Probes go through `cse` and `cse-probe`, not the OG `canvas-expert` MCP. A temporary diagnostic program may print status and body only, never the token or student names, and is deleted afterwards. No grade, rubric or item-score writes in this scope.

## Teacher input needed

The course and name or ID of one real New Quiz that has written (essay or upload) responses, for question A.

Optional, independent of A and B: open probe quiz 3698183 (CS8, unpublished) in Build, then Preview, to see whether students get the word limits and accepted file types enforced. Record the result on the Capabilities page and in `docs/engine-capabilities.md`.

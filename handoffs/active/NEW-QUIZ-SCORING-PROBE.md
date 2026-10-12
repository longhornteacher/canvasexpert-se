# New Quiz written work: rubric-scored feedback and probe evidence

Current direction (2026-10-12): New Quizzes, including written-response items, are open for regular authoring, use and scoring within supported limits. The scoped typed-response/file pull and comment-only review/post paths are implemented with local tests. Comments contain truthful criterion scores and feedback without an aggregate quiz assessment; the teacher enters per-item points in SpeedGrader. No quiz/item grade, rubric-assessment or lateness write is part of this workflow. See [the current implementation contract](../../docs/new-quiz-written-work-spec.md). The dated plan and hand-run results below are historical observations, not prerequisites for ordinary teacher use; question C is outside the implemented comment-only path. Teacher files live in Install, product work in Source, and optional Sync is independent cloud storage.

2026-10-10. **Teacher decisions:** CE-SE gives each student a **feedback comment that includes the agent's rubric scores** for the written items (ECR/SCR), visible to the student; that information matters to them. **The teacher enters the item points** in SpeedGrader. A rubric attached to the New Quiz is important for ECR/SCRs: `pull-item` + `attach-rubric` now supports New Quiz assignments (local tests; see `docs/engine-capabilities.md`). Item-score writes stay out of scope (no public API). Writing the rubric assessment onto the New Quiz submission is question C below. The rubric pre-check result stays on record in `docs/engine-capabilities.md`.

**Probe target:** CS8 New Quiz assignment **3698192** (https://pearland.instructure.com/courses/121046/assignments/3698192), made and assigned by the teacher, **waiting for a student response**. Nothing below runs until that response exists.

## What the feature is

The teacher asks the agent to look at students' written responses in a New Quiz and leave feedback. The agent reads each response, assesses it, and the teacher reviews on the review page. Post writes one **submission comment per student** on the quiz's assignment, in the teacher's voice: the computed rubric assessment and compact per-criterion scores first (the same order as assignment feedback), then glows and grows. No grade is touched. Review-before-post still applies, and `SCORING-RULES.md` governs the scores (honest, real criteria, exact arithmetic). The scoring basis is the attached Canvas rubric when present; otherwise the agent builds `scoring_basis` from each written item's `rubric_hint` in the quiz file the Engine recorded (`<quiz-file>.cse.json` links the quiz to its file), so the rubric written at authoring time drives scoring. Because the teacher enters item points separately, the review page should show the comment's scores next to each response so the teacher can type them in.

## Questions the Engine needs answered

A and B run against a **real New Quiz that already has student responses**; neither needs a fixture, a participant, or a publish.

**A. Can CE-SE read the responses?** Read-only, teacher's real quiz. Try, cheapest first: (a) REST submissions on the quiz assignment with `include[]=submission_history`; (b) the Student Analysis report, JSON (`POST /api/quiz/v1/courses/:c/quizzes/:a/reports`, then poll); (c) GraphQL on the submission. Record per source: essay text present, file-upload response present and downloadable, attempt number, student identity, errors. If none returns the written text, stop: the fallback is feedback from responses the teacher pastes in.

**B. Does an ordinary submission comment work on a New Quiz submission?** The teacher approves one specific comment on one specific real submission; it is the normal REST comment the Engine already posts for assignments. Check it lands on the intended attempt, shows in SpeedGrader, and that the quiz grade is untouched. The teacher can delete it afterwards in SpeedGrader. Skip B if the teacher prefers: it is the same call the Engine already makes for assignments, so the risk is only quiz-specific attempt targeting.

**C. Can a rubric assessment be written on a New Quiz submission without touching the quiz grade?** Only if A succeeds and the quiz has an attached rubric (independent, `use_for_grading` false). One teacher-approved assessment on one real submission; read back the rubric assessment and confirm the quiz score is unchanged. If it fails or moves the score, the comment alone carries the rubric scores.

## Prior art (original CanvasExpert)

The original concluded Canvas has **no public API for writing New Quiz item scores or per-item feedback**, and told teachers to grade New Quiz writing in Canvas. Useful for A and B:

- **Student Analysis report** (A, option b): OG read responses this way, or from the UI's CSV download (`api/powergrader/new_quiz_fetch.py`). It saw transient 502s and **no rows for Test Student submissions**, so probe with real student work.
- **Draft comments** (alternative for B): GraphQL `createSubmissionComment(draftComment: true, attempt: N)` creates a teacher-only draft the teacher finalizes in SpeedGrader. Live-probed by OG (`docs/reference/classic-quiz-design.md`, "New Quiz draft comments"). Consider it if the teacher wants to see comments in SpeedGrader before students do.

## Engine changes if A succeeds

- `UnsupportedAssignment` (`internal/workflow/canvas_reads.go`) currently skips every quiz assignment ("quizzes aren't supported yet"). New Quiz assignments (submission type `external_tool`, `is_quiz_lti_assignment`) need a **comment-only** path: pull written responses (per attempt, with student identity checks), and a results mode that posts comments and never a grade. The existing grade-posting assignment path stays as is.
- Playbook: when to use this (New Quiz written items), the rubric source (Canvas rubric, else `rubric_hint`), that the comment shows the rubric scores, and that the teacher enters item points in SpeedGrader.
- Capabilities page and `docs/engine-capabilities.md` updated together, with evidence levels kept apart.

## Rules

Read-only against the teacher's real quizzes until the teacher approves a specific comment. Probes go through `cse` and `cse-probe`, not the OG `canvas-expert` MCP. A temporary diagnostic program may print status and body only, never the token or student names, and is deleted afterwards. No grade or item-score writes in this scope; a rubric assessment write only as question C, on one approved submission.

## Teacher input needed

None for now: quiz 3698192 is the target once a student responds. Tell the agent when it has a response.

Optional, independent of A and B: open probe quiz 3698183 (CS8, unpublished) in Build, then Preview, to see whether students get the word limits and accepted file types enforced. Record the result on the Capabilities page and in `docs/engine-capabilities.md`.

## Probe result 2026-10-11 (question A answered; B and C pending)

Target: CS8 New Quiz assignment 3698192, one real student attempt (ESSAY + FILEUPLOAD, independent rubric attached, `use_rubric_for_grading` false).

- **REST submissions** (`include[]=submission_history`): shell only. `body` is null, `workflow_state: pending_review`, no response text. Not a source.
- **New Quizzes GET** (`/api/quiz/v1/.../quizzes/:a` and `/items`): items, points and item ids are readable; there is no readable submissions or reports listing (404).
- **Student Analysis report: works.** `POST /api/quiz/v1/courses/:c/quizzes/:a/reports` with JSON `{"quiz_report":{"report_type":"student_analysis","format":"json"}}` returns a progress object; polling reached `completed` in about 7 s with `results.url`. That URL is a pre-signed `inst-fs` file on a different host, so the Engine's same-origin client cannot fetch it as is (a plain GET of the signed URL returned 200 JSON).
- **Row shape:** `student_data` carries Canvas user `id` (matches the REST submission `user_id`), `attempt`, `submitted_at`; `item_responses[]` carries `item_id`, `item_type`, `answer`, `score`. Essay `answer` is HTML (readable text recovered). **File upload `answer` is only the filename**; contents were not read, so a file-upload item cannot be scored honestly from the report alone.
- Item ids in the report match `/items` entry ids, so responses join to the rubric criteria and prompts.

Evidence level: Canvas Live API read, one student. Not yet shown: comment landing on a New Quiz submission (B), rubric assessment write without moving the quiz score (C), file-upload download.

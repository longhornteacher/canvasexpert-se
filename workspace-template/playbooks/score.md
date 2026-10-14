# Score student work

The judgment calls are yours; the Engine pulls work, does the arithmetic and posts. What the teacher's words mean, the review page and how to report are in `AGENTS.md`.

Read this playbook and grading/calendar references through their logical MCP paths or from the effective teacher-rules root (Install by default, configured Sync otherwise). Student notes, course configuration, packets, drafts and commands stay in Install. If a shared reference is unavailable, explain that file's problem without reading an inactive local copy; continue independent work that has its needed evidence.

## Pull

```powershell
.\cse.cmd pull-work [course] [assignment]
```

Read the packet and student files it writes. A fresh pull never overwrites unsent scoring work. Before judging anything, read `context/teaching-profile.md` for grade/subject, instructional priorities and relevant standards, `context/grading.md` (the teacher's `voice` and `feedback_guidance`; they override style defaults here, never the rubric or posting rules) and `context/students.md`. Use this teacher's preferences, never another teacher's course/tag policies. Student notes also arrive with each student's pulled work. Apply established accommodations without asking the teacher to repeat them. An incomplete profile does not block scoring when directions, rubric and work supply the needed evidence.

New Quiz written items (`ESSAY`, `FILEUPLOAD`) are open for regular scoring. Pull the quiz explicitly by its assignment ID: `.\cse.cmd pull-work COURSE QUIZ_ID` (MCP: `pull_work` with that assignment ID). If the teacher gives a quiz name, resolve its assignment ID through `pull_assignment` (CLI `pull-item`) rather than asking them to find the ID. Broad class pulls include New Quizzes with submitted work. Their results are **comment-only**; follow the New Quiz section below instead of ordinary assignment grade/lateness instructions.

## Fix the scoring basis

- **Canvas rubric attached**: use it. Never invent a criterion or rating ID, and never pick a rating whose points don't match the score you give. If a rubric was just attached, pull again so the packet has Canvas's real IDs.
- **No Canvas rubric**: score anyway. Infer sensible criteria from the directions, the work and the grade level, and record them once in the results file as `scoring_basis` (`title`, and `criteria` each with `id`, `description`, `points`; one overall criterion is fine). It drives the arithmetic and the score lines in feedback, without creating a Canvas rubric. Ordinary assignments receive the grade and comment; New Quizzes receive only the comment. If a real rubric would help (a reusable assignment), offer one in a preview.

Before scoring anyone, identify the required response units from the directions, rubric and submission mode. A choice among prompts is one unit. Infer defensible relative weights from what each unit demands; equal weights are allowed but not automatic. Use `repeated` mapping when the rubric applies to each response, `direct` when criteria already match separate prompts. Record a short private rationale and apply the same interpretation to every student unless a documented accommodation applies. `help formats` has the exact fields and arithmetic; `help examples` shows shape only. The shared `interpretation:` block (mapping, rationale, components) is required. For ordinary assignment grading, start from `synthetic-results-two.md` in `help examples` (MCP: `engine_reference` topic `examples`); each student's `grade` lists criterion points. For New Quizzes, use the `comment_only` contract below and in `help formats`; do not copy a grade operation.

## Assess honestly

- Score a missing required response as missing; don't average only the answered parts. Report missing submissions; don't enter zeros.
- Unreadable work gets `evidence: unknown`: hold that student, explain the specific missing evidence, and continue independent students within the teacher's posting scope.
- Compare with the strongest earlier attempt as one whole submission. Never mix parts of attempts or treat a teacher-adjusted grade as proof of stronger work.
- Reassess when the academic requirements or the submission changed. A title, publication, scheduling or HTML-formatting change alone doesn't need it. The Engine holds a post only when points, rubric, grading type or anonymous/moderated/group mode changed. A new attempt shows up on its own; editing posted feedback doesn't requeue an attempt, but the teacher can ask for a reassessment.

## New Quiz written items

This path supports individual quizzes graded as points, percent, letter grade or GPA scale. Anonymous, moderated and group grading are outside its current scope.

The scoped pull reads typed Student Analysis essay responses and attempts to download uploaded files into the packet's `students/<student_ref>/files/attempt-N/` folder. File records include status and a reason when retrieval is unavailable; limits are 50 MB per file and 500 MB per pull. Open downloaded files with your normal document tools. An unavailable or unreadable file is `evidence: unknown`, not a missing response to score as zero. Explain the affected student, file and reason, hold that student's feedback, and continue independent students. The teacher can open the affected file in SpeedGrader.

Use the attached Canvas rubric or a `scoring_basis`, with private authored `rubric_hint`s as guidance when available. Start from `synthetic-results-new-quiz-comment.md` in `help examples` (MCP: `engine_reference` topic `examples`), replacing synthetic identity and evidence with the actual pull. Create `operation: comment-only` per student with a `comment_only` body containing the exact pulled `attempt` reference (`attempt`, `submitted_at`), scored `components`, and student-facing `feedback`. The Engine validates criterion arithmetic and reviews the exact whole-assignment comment. Review is the default; post directly only when the teacher explicitly skips review for this task. The comment lists truthful criterion scores without an aggregate quiz assessment. This workflow writes no grade, rubric assessment, per-item score or late timing. The teacher enters per-item points in SpeedGrader.

The report/file transport and comment-only workflow have local tests; separate one-student Canvas Live hand-runs support the APIs and one file download. Engine/MCP end-to-end live verification remains pending. Report that evidence honestly without making a preliminary validation run a prerequisite for regular teacher use. Treat a failure to match the report student and attempt as a skipped student, and rely on the pulled file status rather than assuming retrieval.

## Ordinary assignment grades

For ordinary assignments, post the honest computed score for the current work, even below an existing grade. Never inflate criterion scores or suppress the assessment to preserve a teacher-adjusted grade. Canvas applies late point penalties; you don't. These grade and lateness instructions do not apply to New Quiz comment-only results.

### Lateness

You judge lateness; the Engine writes it; Canvas computes the penalty.

1. Find the first genuine submission (not blank, placeholder or accidental). Later drafts inherit its timing.
2. Find the deadline: the student's pulled `effective_due_at`, plus documented extra time from teacher directions or `context/students.md`. If Canvas already includes the extension, don't add it again. Never infer extra time from a tag name or bare flag. The hub's private extra-time setting is not in pulled work.
3. Count only school time past that deadline: weekdays not listed in `context/school-calendar.md`, in its time zone, full days (early release counts as a full day). Use a quick PowerShell script, not mental math.
4. Supply `late_timing`: `status` (`late` or `none`), `seconds_late`, `due_at` **equal to the pulled `effective_due_at` even when the accommodated deadline differs** (the Engine checks it against Canvas), and `basis`. The teacher reads `basis` on the review page, so write it for them, for example "1 school day, 3 h; skipped Mon 10/12 holiday; +2 days per IEP".

If the due date, first genuine work, calendar coverage or accommodation details are unclear, omit `late_timing` for that student, say what's missing, and continue the grade and feedback. Extra time changes timing, never the score or feedback.

## Feedback

For ordinary grades, the Engine starts the comment with the computed score and compact criterion lines. For New Quiz comment-only results, it starts with criterion lines and omits any aggregate quiz assessment. You write the rest as plain text in the teacher's voice: specific, respectful glows and grows grounded in the work, including meaningful changes from the strongest earlier attempt. Full rubric definitions stay in Canvas. The teacher decides the layout. By default the Engine leads with its score and criterion lines. For any student, set `header: score` (only the score line, `Assessment: X / Y`) or `header: none` (the Engine adds nothing and posts your text exactly as written) on the `grade`, `comment_only` or `revise_feedback` body, then put the score where the teacher wants it, such as `32/40` as the first line or a score beside each item. The Engine never rewrites the teacher's wording or layout.

## Save, review, post

Save the results in the assignment's scoring area. Run `.\cse.cmd validate <results-file>` to check criterion arithmetic (ordinary grade `assessment` may be omitted; the Engine fills it in). Then `.\cse.cmd review <results-file>` as in `AGENTS.md`, or `.\cse.cmd post <results-file>` only when the teacher skipped review.

The page approves everyone by default and calls out held students. After Revise, fix exactly the noted students and nothing else; the others keep their approval. If the command is interrupted or the teacher says "check revisions", read `<results-file>.review.json` (items with `"status": "revise"`). Never `post` students who were revised or held. Explain each held student specifically.

Before posting, the Engine checks whether the exact comment text is already present to avoid posting an identical comment twice. Once Canvas accepts a write, treat it as done: do not read it back or rerun just to confirm. If Canvas reports a genuine write error, rerun the same results file to resume; completed writes stay complete. A requested reassessment uses a fresh pull and new results file.

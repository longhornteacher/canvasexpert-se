# Create a New Quiz

Use this for any Canvas New Quiz: auto-scored items, written items (`ESSAY`, `FILEUPLOAD`), or a mix such as multiple choice plus constructed responses. What the teacher's words mean and the review page are in `AGENTS.md`.

Before design, read `context/teaching-profile.md`, `context/grading.md` and relevant teacher references. Apply this teacher's grade/subject, instructional priorities, standards and feedback guidance; ask only for missing details that matter to this quiz. Never adopt developer examples as this teacher's policy.

Read teacher rules and design references from the effective teacher-rules location (UserInstall by default, configured UserSync otherwise), using logical MCP paths where available. Run commands and save course drafts, assets and outcomes in UserInstall. UserSync holds rules; its provider carries their edits between PCs.

New Quizzes are supported for regular authoring and use, including written-response scoring with `score.md`. Work within the limits below; pending live or student-view verification is not a prohibition or a preliminary test required for each task.

For curriculum conversion, student file assets and placing an existing quiz into a native module, read `native-content.md`. Use the returned New Quiz assignment ID for a module Assignment entry.

## Standards

Apply `TEACHER.md`: items sum to 100 points unless the teacher chooses otherwise, depth at DOK 2-3, distractors that are real misconceptions, and about two sentences of "why this is correct" per auto-scored item. After shaping the quiz, offer tiered Hub support (see the Hub section below) once.

## Write the quiz in QuizForge format

Read `engine_reference` formats and examples (CLI `help formats` / `help examples`), including the embedded `synthetic-quizforge-mixed.md`, before authoring. Write the quiz as a QuizForge v3.0-json file (`<QUIZFORGE_JSON>` or bare JSON). If the teacher has optional QuizForge design modules (`QuizForge_Base.md` and ELA/stimulus references), use them for educational design; the embedded reference defines what this Engine accepts. You write questions, distractors, rationales and points; the Engine handles IDs, scoring structures and Canvas calls. Save the file in the course's `drafts/` folder.

- Give every scored item an explicit `points` value the teacher chose or approved. The Engine won't invent points.
- Put `stimulus_id` on every item attached to a stimulus, including items inside a STIMULUS..STIMULUS_END span.
- Keep HTML to what `help fields` records as kept; the Engine sends it unfiltered. Show code as `<pre><code>...</code></pre>` (colored as Python; add `class="language-html"` or similar to leave it uncolored). Put TEKS codes in `metadata.teks`.
- Quiz settings (time limit, attempts, due date, publication, module) go in `metadata.canvas`.
- For a quiz only selected book clubs should see, add `metadata.canvas.audience` with `mode: individual`, `policy: snapshot` and the exact `source` set/club IDs, then run `prepare_audience` before review or push. The quiz stays one gradebook column and each student answers independently; the clubs are only the audience. See `group-work.md`'s quiz instructions.
- For one attempt, omit `allowed_attempts` and multiple-attempt settings. For two or more set `allow_multiple_attempts: true` and the desired `allowed_attempts`; omit the limit or use `"unlimited"` for unlimited attempts.
- TF uses a boolean `answer`. Feedback goes in root `rationales`, keyed by `item_id`; MC/MA entries contain `choices` with matching choice `id` and `rationale` (and may also carry a top-level `rationale` as general feedback for the whole question), while TF and other auto-scored entries contain `rationale`. Never put `rationale` inside an item or its choices. Write the explanation in green as `TEACHER.md` shows (`<p style="color: #2e7d32;"><strong>Why this is correct:</strong> ...</p>`).
- Written items: `ESSAY` takes optional `word_limit_min`/`word_limit_max`; `FILEUPLOAD` takes optional `accepted_formats`. Write a `rubric_hint` for each ECR/SCR as private guidance for later rubric authoring and scoring. It is not sent to Canvas and does not automatically create a rubric; put anything students must see in `prompt`.

## Review and create

Run `validate_quiz` on the saved draft before review; it checks item shape and settings locally. Correct ignored-field warnings when they affect teacher intent. On review, check the separate student feedback rationales and the effective requested settings with the teacher.

Run `.\cse.cmd review <quiz-file> <course>`; when the teacher clicks Create, the Engine pushes the quiz. With review skipped, run `.\cse.cmd push-quiz <course> <quiz-file>`. The quiz stays unpublished unless `metadata.canvas.published` is true.

The Engine records the quiz ID in `<quiz-file>.cse.json`. If it reports `partial`, fix the named problem and rerun the same file; it adds only the missing items. If it refuses because a quiz with that title exists, ask the teacher whether it is an unfinished earlier push or a different quiz. For a different quiz, rename this one; for an unfinished push, the teacher renames or deletes the old one in Canvas before you push again. For a club-restricted quiz, a refused or uncertain audience leaves the quiz unpublished; report that plainly and resume the same file, which repeats only unfinished steps.

## Change question points

`.\cse.cmd quiz-points <course> <quiz>` lists the questions with their positions and points. To change points, add `position=points` pairs, for example `.\cse.cmd quiz-points <course> <quiz> 1=50 2=50`; this works on any New Quiz, including ones the teacher made in Canvas. The teacher's request sets the numbers; no review page. It changes only the questions named and reads them back. Once any student has submitted, it still writes but warns that Canvas may not regrade existing attempts; tell the teacher to check the gradebook and use the quiz editor's regrade option if scores should change. If it reports that the items' total and the quiz's total differ, tell the teacher; they set the quiz total in Canvas.

## Written items: rubric and grading

When the quiz has ECR/SCR items, offer to attach an independent rubric after it is created: follow "Attach a rubric" in `edit.md`, building the criteria from the `rubric_hint`s. For scoring, use `score.md`: pull the New Quiz with `--session` by title or ID, assess typed answers and retrieved files against the attached rubric or a local `scoring_basis`, and prepare a comment with truthful criterion scores for review. A missing rubric does not block scoring. CE-SE posts only the whole-assignment comment, with no aggregate quiz assessment, grade, rubric-assessment or late writes. The teacher enters per-item points in SpeedGrader. Explain this handoff when you create the quiz.

## What to tell the teacher

Report the quiz URL, the point total, publication outcome and returned sidecar path for resumption. Requested module placement completes before publication. New Quizzes can't be opened as Canvas's Test Student, so a student-side check needs the editor's Build, then Preview, or a real enrolled account. Report actual write outcomes: push does not read item feedback or quiz settings back, so do not claim verified stored rationales or student rendering.

## Differentiated hub for a quiz

Use the hub for tier supports around one whole-class quiz: add `"differentiation": "hub"` and `tiers` (`label`, this teacher's exact Canvas `tag`, `supports` HTML). Ask for actual tags when missing from this teacher's context; do not assume another teacher's tags. Put shared instructions and hints in the quiz `instructions`, with one `{{tier:LABEL}}` link per tier. Tier content scaffolds the same standard; it doesn't lower it. If a tag doesn't exist, the page is created restricted to nobody and the Engine prints a teacher action naming the page and tag; relay it. Pages publish only with the quiz. A page title that already exists stops the push before anything is created. Students in other tiers see every link but get Canvas's access-denied page for the others.

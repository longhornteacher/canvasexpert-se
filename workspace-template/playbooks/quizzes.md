# Create a New Quiz

Use this for any Canvas New Quiz: auto-scored items, written items (`ESSAY`, `FILEUPLOAD`), or a mix such as multiple choice plus constructed responses. What the teacher's words mean and the review page are in `AGENTS.md`.

For curriculum conversion, student file assets and placing an existing quiz into a native module, read `native-content.md`. Use the returned New Quiz assignment ID for a module Assignment entry.

## Write the quiz in QuizForge format

Write the quiz as a QuizForge v3.0-json file (`<QUIZFORGE_JSON>`). If the teacher has QuizForge's authoring modules (`QuizForge_Base.md` and the ELA/stimulus references), follow them for question design, rationales and stimulus use; `help formats` ("New Quiz file") lists what the Engine accepts and where it differs. You write questions, distractors, rationales and points; the Engine handles IDs, scoring structures and Canvas calls. Save the file in the course's `drafts/` folder.

- Give every scored item an explicit `points` value the teacher chose or approved. The Engine won't invent points.
- Put `stimulus_id` on every item attached to a stimulus, including items inside a STIMULUS..STIMULUS_END span.
- Keep HTML to what `help fields` records as kept; the Engine sends it unfiltered. Show code as `<pre><code>...</code></pre>` (colored as Python; add `class="language-html"` or similar to leave it uncolored). Put TEKS codes in `metadata.teks`.
- Quiz settings (time limit, attempts, due date, publication, module) go in `metadata.canvas`.
- Written items: `ESSAY` takes optional `word_limit_min`/`word_limit_max`; `FILEUPLOAD` takes optional `accepted_formats`. Write a `rubric_hint` for each ECR/SCR: it is never sent to students, and it becomes the quiz rubric and the scoring basis later.

## Review and create

Run `.\cse.cmd review <quiz-file> <course>`; when the teacher clicks Create, the Engine pushes the quiz. With review skipped, run `.\cse.cmd push-quiz <course> <quiz-file>`. The quiz stays unpublished unless `metadata.canvas.published` is true.

The Engine records the quiz ID in `<quiz-file>.cse.json`. If it reports `partial`, fix the named problem and rerun the same file; it adds only the missing items. If it refuses because a quiz with that title exists, ask the teacher whether it is an unfinished earlier push (resume it) or a different quiz (rename this one).

## Change question points

`.\cse.cmd quiz-points <course> <quiz>` lists the questions with their positions and points. To change points, add `position=points` pairs, for example `.\cse.cmd quiz-points <course> <quiz> 1=50 2=50`; this works on any New Quiz, including ones the teacher made in Canvas. The teacher's request sets the numbers; no review page. It changes only the questions named and reads them back. Once any student has submitted, it holds without writing, because the scores would change; the teacher changes points in the Canvas quiz editor, which offers regrade choices. If it reports that the items' total and the quiz's total differ, tell the teacher; they set the quiz total in Canvas.

## Written items: rubric and grading

When the quiz has ECR/SCR items, offer to attach a rubric after it is created: follow "Attach a rubric" in `edit.md`, building the criteria from the `rubric_hint`s. The teacher enters the points for written items in SpeedGrader; CE-SE can't pull New Quiz responses for scoring yet. Say both when you create the quiz.

## What to tell the teacher

Report the quiz URL, the point total, and whether it is published. New Quizzes can't be opened as Canvas's Test Student, so a student-side check needs the editor's Build, then Preview, or a real enrolled account. The Engine verified the stored items and settings through the Canvas API, not what a student sees.

## Differentiated hub for a quiz

Use the hub for tier supports around one whole-class quiz: add `"differentiation": "hub"` and `tiers` (`label`, exact Canvas `tag`, `supports` HTML; in ELA7 the tags are Silver, Red, Blue for Support, Core, Accelerate). Put shared instructions and hints in the quiz `instructions`, with one `{{tier:LABEL}}` link per tier. Tier content scaffolds the same standard; it doesn't lower it. If a tag doesn't exist, the page is created restricted to nobody and the Engine prints a teacher action naming the page and tag; relay it. Pages publish only with the quiz. A page title that already exists stops the push before anything is created. Students in other tiers see every link but get Canvas's access-denied page for the others.

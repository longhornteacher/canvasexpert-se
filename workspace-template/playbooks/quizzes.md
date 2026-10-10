# Create a New Quiz

Use this playbook for an auto-scored quiz in Canvas New Quizzes. Written-response work you will grade by hand belongs in an ordinary Assignment (`authoring.md`). Canvas Live is authoritative.

## Write the quiz in QuizForge format

Quizzes are authored in the QuizForge v3.0-json format, the same `<QUIZFORGE_JSON>` file QuizForge uses. If the teacher has QuizForge's authoring modules (`QuizForge_Base.md` and the ELA/stimulus references), follow them for question design, rationales, tiers and stimulus use; `.\cse.cmd help formats` ("New Quiz file") lists what this Engine accepts and where it differs. You write the questions, distractors, rationales and points; the Engine handles IDs, scoring structures and the Canvas calls. Differences to remember:

- Give every scored item an explicit `points` value that the teacher chose or approved. The Engine will not invent points.
- Put `stimulus_id` on every item attached to a stimulus, including items inside a STIMULUS..STIMULUS_END span.
- Show code as `<pre><code>...</code></pre>`; the Engine colors it (Monokai, Python rules; add `class="language-html"` or similar on the `<code>` tag for other languages to keep it uncolored). Put TEKS codes in an item's `metadata.teks`. The Engine adds the label.
- Quiz and assignment settings (time limit, attempts, due date, publication, module) go in `metadata.canvas`.
- No ESSAY or FILEUPLOAD items. Offer a separate Assignment for the writing portion.

Save the file in the course `drafts/` area.

## Create or resume

```powershell
.\cse.cmd push-quiz <course> <quiz-file>
```

"Draft" means write the file and stop. A request to create the quiz authorizes the push. The quiz stays unpublished unless `metadata.canvas.published` is true. The Engine records the quiz ID in `<quiz-file>.cse.json`. If it reports `partial`, fix the named problem and rerun the same file: it adds only the missing items. Do not delete the `.cse.json` file to "retry". If it refuses because a quiz with that title exists, ask the teacher whether it is an unfinished earlier push (resume it) or a different quiz (rename this one).

## What to tell the teacher

Report the quiz URL, the point total, and whether it is published. New Quizzes cannot be opened as Canvas's Test Student (it errors), so a student-side check needs the editor's Build, then Preview, or a real enrolled account. The Engine has verified the stored items and settings through the Canvas API, not what a student sees.

## Classic quizzes (written work in the same quiz)

When the quiz must hold writing (essay or file upload) alongside auto-scored items, set `"quiz_engine": "classic"`. Classic cannot hold ordering or categorization items, FITB word banks, or case-sensitive/fuzzy FITB; the Engine refuses them with a message, so offer the closest supported type. Run the same `push-quiz` command. A Classic quiz resumes when the same file is rerun; if the quiz was deleted in Canvas, delete the `.cse.json` sidecar to start over. To score Classic essay/file-upload responses, use the Classic section of `score.md`: pull the quiz's backing assignment ID, assess its question work, validate, and post the selected question scores/comments. Canvas owns objective scores and quiz totals.

## Differentiated hub for a Classic quiz

Hub is the only differentiation style (the Bridge was declined). Use it when the teacher wants tier supports for one whole-class quiz: add `"differentiation": "hub"` and `tiers` (`label`, `tag`, `supports` HTML). In ELA7 the tags are Silver, Red and Blue (Support, Core, Accelerate); put the exact Canvas tag name in each tier's `tag`. Put the shared instructions and hints for everyone in the quiz `instructions`, with one `{{tier:LABEL}}` link per tier. Tier content scaffolds the same standard; it does not lower it. The teacher manages the tags and their members in Canvas. If a tag does not exist, the Engine still creates the page restricted to nobody and prints a teacher action naming the page and tag; relay it. Pages publish only with the quiz. A title collision with an existing page stops the push before anything is created. Students in other tiers see all links but get Canvas's access-denied page for theirs.

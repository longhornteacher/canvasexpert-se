# Remember this teacher's choices

Use this when learning a durable preference, preparing course content, or scoring an assignment. Read only the teacher, course and assignment notes relevant to the task. These are plain teacher references, not Engine policy files.

## Where things belong

Use logical paths with CE-SE file tools. `Teacher_Override.md` and `context/` resolve to UserInstall by default and to configured UserSync when present. Check `engine_info` rules-location when needed; do not write into an arbitrary empty cloud folder or use inactive local copies. In a git install the shipped instructions and playbooks stay in UserInstall. The provider carries UserSync between PCs; CE-SE does not synchronize it.

| Scope | File | Save |
|---|---|---|
| Teacher | `Teacher_Override.md` | Standing choices that change a shipped default, with their scope |
| Teacher | `context/teaching-profile.md`, `context/grading.md` | General teaching facts, style and feedback guidance |
| Course | `context/courses/<course-id>/notes.md` | Grade, subject, taught skills, recurring routines and course-specific choices |
| Assignment | `context/courses/<course-id>/assignments/<assignment-id>.md` | Prompt expectations, teacher decisions, scoring guidance and authored written-response exemplars |

`context/courses.md` remains the Engine's structured course list; do not add ancillary fields to it. Course and assignment filenames use numeric Canvas IDs, not nicknames or titles. Each course/assignment note also records the Canvas origin; check origin and IDs before reuse. A copied assignment gets its own destination record, adapted to that course's learners and exact prompts/item IDs. Similar titles alone do not establish equivalence. If an existing note belongs to another origin, leave it intact and resolve the mismatch before using or replacing it.

Pulled submissions, student excerpts, scoring results, drafts, accepted-write receipts and grading/planning session records stay in UserInstall's `courses/` tree. Reusable assessment guidance belongs in `context/`; it is not a second gradebook. Student-specific notes stay in `context/students.md`, with existing privacy rules. Never put credentials, contact records, student responses or individual scores in course/assignment references. These references are teacher-only; publish only the particular exemplar or guidance intended for students.

## Learn during ordinary work

Do not make the teacher maintain these files. Save explicit standing requests and clear corrections that establish an ongoing preference as you work, without a separate permission step. Record the choice, scope, date and a short source/reason (for example, “teacher correction during quiz design”). Reword or remove a contradicted entry rather than leaving competing rules. Tell the teacher briefly when you have remembered a meaningful new choice.

A one-assignment decision belongs with that assignment; a course routine stays with that course. Do not turn a single accepted draft, a one-time go-ahead, or silence into a standing rule. When repeated interactions suggest a preference but its intended scope is unclear, record it as tentative with its evidence and confirm when it next matters. Tentative observations do not change grading standards, posting permission, privacy or accommodations. Never import the developer's classes, habits or another teacher's preferences.

Follow the current request, then the teacher's explicit saved choices at the relevant scope, then shipped defaults. A specific assignment/course choice narrows a general preference only within its stated scope. Saved context never silently changes live directions, rubric requirements or honest assessment; raise a material conflict with the teacher and continue unaffected work.

Read before editing. Merge small changes, preserving unrelated notes and teacher wording. With `write_workspace_file`, supply the current `expected_sha256` for replacement; omit it only to create a missing file (an existing file then refuses the write). On conflict, reread and merge. Do not overwrite a populated file with a blank template or copy a whole local tree over UserSync.

## Course notes

Create a course note when something useful is known, using the outline in `context/courses/README.md`. Record grade/subject, what students have actually been taught, useful routines and expectations, and any course-specific preference with its source. Leave unknowns unknown. Cohort information describes instruction and supports; it is not a cap on what a student may demonstrate or an individual accommodation. No setup interview or empty record for every course is required.

## Assignment expectations and exemplars

During authoring, prepare a compact expectations note alongside the local draft. For each written prompt, record the exact prompt, relevant source/edition references, required thinking and evidence, rubric mapping, and a model answer. State which expectations were teacher-specified and which were agent-designed. Include legitimate alternative approaches and what is optional; an exemplar's wording, length or chosen evidence is not an extra grading criterion.

Calibrate every model to the actual grade and taught skills:

- Model strong, attainable full-credit work by a student at that grade, not an adult's polished analysis. Use natural vocabulary, sentence structure, response length and reasoning the assignment teaches and requires. Do not demand sophistication beyond the rubric or make a shallow answer look sufficient.
- Show the intellectual work: an accurate answer or claim, relevant evidence and an explanation linking them when required. Use only sources in hand; never invent quotations, events or facts. A short response stays short when that is enough.
- Explain why the model earns its rubric level. Add a brief partial-credit contrast or common misconception when it helps distinguish the boundary; do not create a model for every rating by default. Accept other defensible reasoning and evidence.
- Separate a student's verbal polish from the assessed understanding. Do not penalize dialect, language development or conventions unless the rubric assesses them. Scaffolds change access, not the common standard; apply documented individual accommodations separately.
- Use a known course grade/profile rather than guessing from a title. If grade or task expectations cannot be established, ask for the missing detail when needed and label the model provisional until resolved. A provisional model is not a scoring benchmark.

Include the expectations and models in the authoring review in `review.md`; the pedagogy reviewer explicitly checks both verbal and intellectual fit, including a bar set too high or too low. Keep review findings/rulings in the conversation, not in this note; a short reviewed date/status suffices. A private answer to the exact quiz question stays private unless the teacher intends to release it. A public mentor example may need a parallel prompt so it does not give away the assessed answer.

After an accepted create, save the reusable note under the returned course and associated assignment ID, including the New Quiz ID when applicable. Bind written items to actual Canvas item IDs from saved outcomes or a normal later pull, never authored QuizForge IDs or question numbers alone. Mark unresolved item mappings and pending rubric steps explicitly; do not claim those completed. Keep the local draft and receipts for resumption; no post-write confirmation read is needed just for this note. If saving the note fails, report that separately from the accepted Canvas write and preserve the local note for retry; do not create the assignment again.

After an accepted edit affecting prompts, sources, points or rubric expectations, update the affected note and model from that edit's outcomes. Cosmetic/title/date-only edits do not require rewriting models. Never save an unsent edit as if it were live. If an assignment already exists without a note, build useful guidance from the current pull during normal scoring; do not require recreating it.

## Reuse while scoring

After the normal pull, read that exact assignment's expectations. Compare origin, course/assignment/item IDs, prompt meaning, sources and rubric/points with the pulled evidence. A note is guidance, not proof of current Canvas content. Recalibrate an affected stale model before using it; continue unaffected prompts/students. Missing notes do not block scoring: use the live task and rubric, then save useful reusable guidance.

Use the live Canvas rubric when present. When absent, reuse a recorded local `scoring_basis` and `interpretation` only when they still fit this exact task; save newly established ones here without student rows so another PC can use the same scale. Cross-section reuse requires confirmed equivalent prompts, sources and requirements, with destination IDs mapped explicitly; never copy Canvas criterion/item IDs between assignments blindly. Do not silently change an established scale between students or sections; ask about a material conflict.

Assess the student's work against the task and rubric, using the model to calibrate what the level looks like. Do not grade by matching phrases, require the model's particular interpretation, or cap a stronger response. Label authored examples as model answers; they are distinct from anonymous class exemplars selected later under `score.md`. If a model is included in feedback (including a New Quiz component's `exemplar`), review that use with the results and keep teacher-only notes out of the published text.

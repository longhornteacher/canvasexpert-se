# New Quiz written responses: scoring (open)

**Status, 2026-10-10.** Classic Quizzes are removed from CE-SE for good: the Go code, tests, docs and records are gone (only the `push-quiz` refusal that says they are retired remains), and Canvas had already migrated every Classic quiz to New Quizzes. Written items (`ESSAY`, `FILEUPLOAD` in `push-quiz`) and Hub on New Quizzes are done locally (local tests only; the item payloads are not yet checked against Canvas Live). **The ONLY open item is scoring New Quiz written responses, plus the live probe of the New Quiz essay and file-upload payloads.** Migrated IDs and gradebook columns are not this project's concern.

## Open item: scoring New Quiz written responses

This is the hard part. The original CanvasExpert concluded that Canvas has **no public API for writing New Quiz item scores or per-item feedback**. It never wrote them, and it told teachers to grade New Quiz writing in Canvas. The known pieces:
   - **Reading responses:** the New Quizzes Student Analysis report (`POST /api/quiz/v1/courses/:course_id/quizzes/:assignment_id/reports`) or its CSV download from the UI, as in `api/powergrader/new_quiz_fetch.py`. OG saw transient 502s, and no rows for Test Student submissions.
   - **Draft comments:** GraphQL `createSubmissionComment(draftComment: true, attempt: N)` creates a teacher-only draft that the teacher finalizes in SpeedGrader. This was live-probed by OG (`docs/reference/classic-quiz-design.md`, "New Quiz draft comments").
   - **Unknown:** whether writing the quiz assignment's overall grade through REST is honest and stable against New Quizzes' own scoring, or whether New Quizzes overwrite it. Probe this before designing. Do not post a grade that misstates the item scores.

   Treat this as its own plan, with a live probe first. The outcome decides whether CE-SE posts scores, prepares drafts for the teacher to finalize in SpeedGrader, or only prepares a scoring sheet.

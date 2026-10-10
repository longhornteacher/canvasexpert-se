# Classic Quizzes retirement and New Quiz written work (note, not yet planned)

2026-10-10. **Classic Quizzes are gone for this teacher.** Canvas has already migrated every Classic quiz to New Quizzes automatically. The teacher's direction is to remove Classic Quizzes from CE-SE entirely and plan around New Quizzes, which will be heavily used from now on. This will be a major change. No work is authorized yet; this note frames the plan.

## Teacher answers (2026-10-10)

1. **Timing:** the migration has already happened. All of the teacher's Classic quizzes are now New Quizzes.
2. **Migrated IDs and gradebook columns:** not this project's concern. Do not build anything to track or reconcile migrated quizzes.
3. **Written work:** goes in New Quizzes now. This reverses the current rule that written work belongs in an Assignment, and the Engine's refusal of `ESSAY`/`FILEUPLOAD` in `push-quiz`.
4. **Hub:** New Quizzes are the main quiz type from now on, so Hub needs to work with New Quizzes. Today Hub is Classic-only and refused on New Quizzes.

## Remove (dead code once the plan lands)

- **Classic quiz authoring:** `quiz_engine: "classic"` (`internal/workflow/quiz_classic.go`, and the Classic branches of `quizforge.go` and `quiz_author.go`).
- **Classic quiz writing scoring:** `classic_pull.go`, `classic_reads.go`, `classic_post.go`, `internal/domain/classic_scoring.go`, the `classic-writing-*` examples, and the Classic sections of `formats.md`, `playbooks/score.md`, and `playbooks/quizzes.md`. Also the Classic hooks in `pull_work.go`, `post.go`, and `canvas_reads.go`, and the Classic-only `cse-probe` helpers.
- **Records:** the Classic rows in `engine-docs/field-limits.md`, `capabilities.go`, and `docs/engine-capabilities.md`. Mark them retired rather than leaving them as current capability.

## Build (the real work)

1. **New Quiz written items in authoring.** Accept `ESSAY` and `FILEUPLOAD` in `push-quiz` and create them as New Quiz items. Check OG/QuizForge for the item shapes first: `api/transform.py`, plus the `nq_pull_*` probe quizzes in `api/qf_materials/qf quiz examples/`.
2. **Hub on New Quizzes.** Restricted tier pages linked from New Quiz instructions, as Classic Hub did. Find out why it is refused today before assuming it is hard.
3. **Scoring New Quiz written responses.** This is the hard part. The original CanvasExpert concluded that Canvas has **no public API for writing New Quiz item scores or per-item feedback**. It never wrote them, and it told teachers to grade New Quiz writing in Canvas. The known pieces:
   - **Reading responses:** the New Quizzes Student Analysis report (`POST /api/quiz/v1/courses/:course_id/quizzes/:assignment_id/reports`) or its CSV download from the UI, as in `api/powergrader/new_quiz_fetch.py`. OG saw transient 502s, and no rows for Test Student submissions.
   - **Draft comments:** GraphQL `createSubmissionComment(draftComment: true, attempt: N)` creates a teacher-only draft that the teacher finalizes in SpeedGrader. This was live-probed by OG (`docs/reference/classic-quiz-design.md`, "New Quiz draft comments").
   - **Unknown:** whether writing the quiz assignment's overall grade through REST is honest and stable against New Quizzes' own scoring, or whether New Quizzes overwrite it. Probe this before designing. Do not post a grade that misstates the item scores.

   Treat item 3 as its own plan, with a live probe first. The outcome decides whether CE-SE posts scores, prepares drafts for the teacher to finalize in SpeedGrader, or only prepares a scoring sheet.

## Order

Do not remove Classic code until New Quiz written authoring (1) exists, so the teacher always has a working way to put written work in a quiz. Scoring (3) can trail it. Hub (2) is independent.

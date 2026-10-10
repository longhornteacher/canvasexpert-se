# Classic Quizzes retirement (note, not yet planned)

2026-10-10. The teacher received a Canvas notice that the district's Classic Quizzes are being **migrated automatically to New Quizzes**. Direction (teacher): CE-SE can drop Classic Quizzes entirely. This will be a major change. No work is authorized yet; plan it when the migration timing is known.

## What it touches

- **Classic quiz authoring:** `quiz_engine: "classic"` in `push-quiz` (`internal/workflow/quiz_classic.go`, the Classic branches of `quizforge.go` and `quiz_author.go`).
- **Classic Hub:** Hub tier pages are offered only for Classic quizzes (`quiz_hub.go`); Hub on New Quizzes is refused today. Dropping Classic leaves Hub with assignments only, unless Hub is added for New Quizzes.
- **Classic quiz writing scoring:** pull/validate/post of essay and file-upload question scores and comments (`classic_pull.go`, `classic_reads.go`, `classic_post.go`, `internal/domain/classic_scoring.go`, the three `classic-writing-*` examples, the Classic sections of `formats.md` and `playbooks/score.md`). New Quizzes reject ESSAY/FILEUPLOAD in CE-SE by design ("written work belongs in an Assignment"), so the replacement may simply be assignments, not New Quiz essay grading.
- **Docs and status:** the Classic rows in `engine-docs/field-limits.md`, `playbooks/quizzes.md`, `capabilities.go`, and `docs/engine-capabilities.md`.

## Open questions for the plan

1. When does the migration happen, and does it include existing quizzes with student submissions?
2. Do migrated quizzes keep their assignment IDs and gradebook columns? Does anything in a teacher's workspace (sidecars, results files) point at Classic quiz IDs that will change?
3. Does the teacher need written-response scoring inside quizzes after migration (New Quiz essay grading), or do assignments cover it?
4. Should Hub support New Quizzes before Classic is removed?

Until then, Classic paths keep working against Canvas as it is today. Do not start new Classic features.

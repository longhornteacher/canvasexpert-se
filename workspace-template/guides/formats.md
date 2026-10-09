# Version-one workspace formats

Machine-written documents begin with one YAML mapping between `---` lines, followed by Markdown or preserved Canvas HTML. Use `schema: 1`; duplicate keys, unknown fields, and YAML aliases in the header are rejected. Everything after the closing delimiter is body content, including text that resembles YAML. Decimal scores may be written as ordinary finite numbers (`12.5`); quote them when a spreadsheet or other tool might rewrite them. Do not use Markdown tables as machine data.

## Settings

`settings/courses.md` contains `schema`, the HTTPS Canvas `instance` origin, `aliases`, optional `module_hint`, and `courses`. Each course has a unique `nickname`, positive Canvas `id`, `current` scan flag, and optional IANA `timezone`. The clean starter has an example origin and an empty course list; replace the origin during setup.

`settings/grading.md` contains `schema`, optional `voice`, and optional `feedback_guidance`. `settings/students.md` contains `schema` and `notes`. Each note may identify a student by stable `student_ref`, or by unambiguous `name` and optional `course`, and has a required `note`. These are teacher preferences and private context; neither is included in rendered student feedback. `settings/school-calendar.md` contains `schema`, IANA `timezone`, inclusive `start_date` and `end_date`, `non_school_dates`, and optional `non_school_ranges`. It defines student dates for elapsed lateness; update its coverage, closures, and make-up days. Early-release dates count as full student dates.

## Versioned task documents

Every document's required identity fields are described below. IDs returned by Canvas belong in their matching fields. Omit optional fields when unknown; never manufacture an ID, timestamp, rating, or result.

| `kind` | Header fields and body |
|---|---|
| `assignment` | Header has `schema`, `kind`, `operation` (`create`, `update`, or `attach-rubric`), `course`, `title`, `points`, `submission_types`, and optional `due_at`, `unlock_at`, `lock_at`, `timezone`, `module_id`, `published`, `rubric`, and outcomes. Updates and rubric attachments also have `instance`, `course_id`, `assignment_id`, and `observed` live baseline. A create body is Markdown; update and attachment bodies preserve Canvas HTML. |
| `packet` | Header has `course`, a validated `assignment` snapshot, and `students` workspace-relative paths to student-work files. Those files carry stable student references. The snapshot includes instance/course/assignment IDs, title, points, Canvas `description_html`, submission types, dates/publication, rubric, grading type, anonymity/moderation, and group category when returned. |
| `student-work` | Header has `student_ref`, display `alias`, `assignment_id`, `current` attempt, optional `prior` attempts, existing `comments`, and `excused`, `monitored`, and `notes` state. Attempts include attempt number and RFC3339 submission time, original `body_html`, readable text, attachments (`id`, `name`, optional local/readable paths, status/reason), late/due information, and any existing rubric assessment. |
| `results` | Header has `packet`, assignment snapshot, shared `interpretation`, optional documented accommodation `variants`, and unique `students`. Each student has `student_ref`, `operation`, optional `variant`, and exactly one operation body: `grade` or `revise_feedback`. |

A rubric has a title, optional Canvas rubric and association IDs, grading flags, and criteria. Each criterion has an ID, description, maximum points, optional long description and ratings. Each rating has an ID, points no greater than the criterion maximum, description, and optional long description. Never fabricate IDs for a live rubric or select a rating whose points do not match.

For `operation: attach-rubric`, `observed.rubric` must be absent and the desired `rubric` must have `use_for_grading: false`. Preserve every assignment setting and exact HTML from the baseline; module placement and ordinary update outcomes are forbidden. The CLI records the target assignment and new rubric/association IDs in `created`, with assignment/publication preserved, module not requested, and a separate rubric outcome. This is a new association, not a way to replace or edit an existing/shared rubric. See `guides/edit.md` for the authoring path and uncertainty handling.

An interpretation has `mapping: repeated` or `direct`, a private `rationale`, and components with unique `key`, student-facing `label`, nonnegative relative `weight` with a positive total, and (for direct mapping) the exact `criterion_ids` and weight `1`. Repeated mapping applies the whole rubric to each required response. Direct mapping assigns each existing criterion exactly once. Use the assignment directions to identify required response units; a choice among prompts is one selected unit, and a missing required unit is scored as missing. Unknown or unreadable evidence must be held for assessment.

For every component, `evidence` is `present`, `missing`, or `unknown`, and `criteria` supplies each mapped `criterion_id` with explicit `points` and optional truthful `rating_id` or concise `comment`. Missing components explicitly score zero; unknown evidence cannot be posted. The `assessment` is derived from the fixed interpretation and component scores, normalized to assignment points, then rounded to two decimal places. The CLI validates it against the computed value.

### Assignments without a Canvas rubric

If the live assignment has no rubric, add a top-level `scoring_basis` to the results document: `title` and `criteria` (each `id`, `description`, nonnegative `points`; ratings optional). The interpretation and component scores work exactly as they do for a Canvas rubric, using `scoring_basis` criteria ids. It is used only for arithmetic and the compact score lines; the post writes the grade and comment but no rubric assessment (rubric outcome `not-requested`). `scoring_basis` is rejected if the assignment already has a Canvas rubric, and results for a rubric-less assignment are rejected until one is supplied.

### Student operations

For `operation: grade`, include `grade` with the target `attempt`, every `components` score, the derived `assessment`, and `feedback`. Optional `teacher_note`, `strongest_prior`, and `late_timing` are private. The workflow writes the honest `assessment` as the grade for the target attempt and records the rubric assessment when a Canvas rubric exists. Outcomes record each remote action independently.

When timing can be determined, `late_timing` contains `first_attempt` (the selected first genuine submission reference), `effective_due_at` (its applicable due time as RFC3339), and `genuine_reason` (brief evidence for selecting that work as genuine). The first genuine work may be a later Canvas attempt when earlier attempts are blank, placeholders, or accidental. The CLI calculates full elapsed clock hours on student dates, not classroom hours; weekends and calendar nonstudent dates do not count. Later drafts use the same first-genuine timing. Canvas calculates any late point deduction. If the calendar, due time, or genuine submission cannot be established, omit the timing adjustment, report the unresolved timing to the teacher, and continue the honest grade and feedback.

For `operation: revise-feedback`, include `revise_feedback` with the exact `attempt`, existing `comment_id`, replacement `feedback`, and optional preserved `assessment` object (`assessment` and component scores). This path changes feedback only; do not add grade, rubric, or late-write instructions.

The renderer accepts the validated rubric, selected interpretation, component scores, computed assessment, assignment maximum, and conversational feedback. It returns plain text for Canvas's `comment[text_comment]`: the score, compact criterion lines, then the feedback unchanged. It does not serialize rationale, `teacher_note`, notes, failure details, or outcomes. `MarkdownHTML` is for assignment authoring; do not apply it to student feedback.

The `examples/` directory contains synthetic, schema-valid documents for field shape. Treat them only as examples: infer required response units, weights, and choices from each actual assignment's directions.

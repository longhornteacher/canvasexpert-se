> **Retired 2026-11-10 — implemented in Dev** (commit 1ce88d5, released in 2.0.0). The Engine supplies parent-report facts, an Engine-drawn table and safe delivery; the playbook writes the email wording, sections and send policy. Local tests only; no Canvas Live or family-delivery check is claimed. Historical instructions follow; do not execute them again.

# HANDOFF: Parent update emails: the Engine supplies facts, the playbook writes the email

Date: 2026-11-07. Facts mapped at `6854757` plus the uncommitted 2026-11-07 parent-update playbook, context and doc edits in the working tree. Build on those edits; do not revert them.

Layers: Engine (`internal/workflow/parent_*.go`, `family_render.go`, MCP/CLI adapters, `engine-docs/formats.md`), seed playbook/context (`workspace-template/playbooks/parent-updates.md`, `workspace-template/context/parent-updates.md`), and docs (`docs/parent-update-flow.md`, `docs/engine-capabilities.md`, `internal/localui/capabilities.go`).

Before starting: `git fetch`, bring `dev` up to date, read root `AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/SCORING-RULES.md`, `docs/parent-update-flow.md`, `docs/family-contact-flow.md`, the seed `playbooks/parent-updates.md`, and the [shared execution notes](../README.md#execution-notes-for-the-active-handoffs).

**Privacy:** this is a public repository. Do not write the developer's school, tutoring program, class names or real course IDs into code, tests, seeds, docs or commits.

## Why

The Engine is the machinery and the playbooks hold the content. Parent update emails break that rule. The playbook's only say is through `drafts.json`: send, skip or flag; one intro paragraph; and item picks with short notes for three Engine-defined sections. Everything else is hard-coded in Go:

| Where (hint) | What the Engine decides today |
|---|---|
| `parent_drafts.go` `render` | Subject `Class update for <name> (<date>)`; the header and "Recent window" line; the four section headings, their columns and their order; a complete "Work due in this period" table that cannot be omitted; the portal line's wording |
| `parent_drafts.go:440`, `parent_gradesheet.go:137` (`aiNotice`) | A fixed AI and privacy disclaimer on every email and printed sheet |
| `parent_drafts.go` `renderWeekly` | Almost the whole weekly email: greeting, headings, empty-state sentence, "All missing work should be done as soon as possible.", the Missing/Failing rules, score display, "Thank you," |
| `parent_gradesheet.go` `workDueStatusText` | The status words families see |
| `parent_report.go` `buildStudentReportScoped` | Teacher policy as classification: correction candidates and deadlines, the 7-day Due Soon constant (`dueSoonDays`), and the 14-day rolling report window |
| `parent_report.go` `parentSkipEligible` | The send/skip rule (course score at least `skip_score` and no Missing/0) |
| `ParentStudentDraft.History` | Accepted, but never rendered; `sortedHist` and `noteFor` are dead code |

As a result, the 2026-11-07 format (every released assignment below 70% this term, with a direct link, score, the student's due date and an independent turned-in status; no AI/alias notice; no Corrections section) cannot ship without Engine work. Any later wording change would need another Engine release.

## Target split

**Engine (facts and safety):** Canvas reads, student/course identity, each student's effective due dates, assignment visibility, keeping unreleased grades private, exact percentages, canonical links, independent submission evidence, sorting by due date then title, a hold on incomplete evidence, real-name substitution, sanitizing, recipient routing, the teammate reply-to note, Outlook delivery and outcomes. The Engine draws the assignment table so a draft can never misstate a score, date or link.

**Playbook/context (content and policy):** subject, greeting, intro, which rows appear (by choosing a threshold), column labels, status wording, empty-state wording, links to the grade portal and tutoring, any disclaimer the teacher wants, and when to send.

## Decisions (confirmed by the developer 2026-11-07)

Governing rule, in the developer's words: "Engine keeps factual/deterministic things. Playbook is the dressing." When a detail is unclear during execution, apply that rule: anything a family reads as words is dressing; anything computed from Canvas, or that protects identity, privacy or honesty, is the Engine's.

1. **Retire both legacy layouts.** Delete the default layout (`render`) and `weekly_update` (`renderWeekly`) instead of keeping three renderers. The teacher has already rejected both formats. Old run folders keep `report.json` and `outcomes.json`. Loading an old `drafts.json` (schema 1 or 2) fails with "This update was drafted in an older format; draft it again with the current playbook." Nothing that was sent is lost, and a legacy-rendered email should not go out anyway.
2. **Drop policy data from the report.** Remove `missing`, `corrections`, `due_soon`, `history`, `first_earned`, `basis`, `skip_eligible` and the gradebook-history read (`gradeVersions`). The report always covers the whole grading period up to `as_of`. `full_period` and `--full-period` stay accepted for compatibility and have no further effect. `correction_days`, `skip_score`, `failing_below`, `class_name` and `tutoring_*` are no longer read by the Engine; they become plain settings the agent reads in context. If a teacher later asks for a corrections email, it is built as playbook content over these facts.
3. **Select rows with a declarative threshold, not a hand-picked list.** The draft names `below_percent: 70`, and the Engine applies it to its own exact percentages for every student. An agent listing assignment IDs for 150 students would miss rows. The threshold is the teacher's policy and the arithmetic stays in the Engine.

## Part 1: Report facts (Engine)

In `ParentWorkDueItem`, keep `assignment_id`, `title`, `url`, `due_at`, `points_possible`, `canvas_score`, `points_deducted` and `status` (the grade state: `graded`, `awaiting_grading`, `grade_not_released`, `no_grade`, `excused` is already excluded). Add:

- `percent`: the exact decimal string `canvas_score / points_possible × 100`, computed with `internal/domain` decimals. It is absent when there is no released score or when points possible is 0 or absent. Do not round it in the report.
- `submission`: evidence that is independent of the grade.
  - `turned_in`: Canvas records `submitted_at` with attempt ≥ 1, or a completed quiz attempt.
  - `not_turned_in`: an online assignment (`isOnline`) that is past due, with Canvas `missing` true or `workflow_state` `unsubmitted` and no `submitted_at`.
  - `no_online_submission`: the assignment's submission types are none, on paper or external, and nothing is recorded.
  - `unknown`: anything else. Never infer this status from the grade, a comment or a grading timestamp.

Remove the fields listed in Decision 2, along with the code that produces only them. `work_due` now spans period start through `as_of`. Keep `upcoming` (the printed sheet uses it), `flags`, `course_score` and the scope fields. Keep the existing guards: unreleased grades stay private, each student's due date applies, visibility completion, and malformed-date flags.

Tests: percent at 34/50, 35/50 (exactly 70) and 40/50; zero points possible; an unreleased grade; each `submission` value, including a submitted zero, a late-penalized score and a paper assignment with a teacher-entered grade.

## Part 2: Drafts schema 3 and rendering (Engine)

Replace `ParentDrafts` with schema 3 and reuse the Family Contact renderer (`renderFamily`). It already handles Markdown, sanitizing, `{{student_name}}`, the reply-to note and the sign-off.

```json
{
  "schema": 3,
  "kind": "parent-drafts",
  "run_id": "…",
  "subject": "{{student_name}}: Term 2 assignments below 70",
  "body": "Hello,\n\nHere are {{student_name}}'s Term 2 assignments below 70% as of Nov 7.\n\n{{assignment_table}}\n\n[Grade portal](https://…) for official grades.",
  "table": {
    "below_percent": 70,
    "columns": ["assignment", "score", "due", "submission"],
    "labels": {"assignment": "Assignment", "score": "Score", "due": "Due date", "submission": "Turned in?"},
    "submission_labels": {"turned_in": "Turned in", "not_turned_in": "Not turned in", "no_online_submission": "No online submission required", "unknown": "Status unavailable"},
    "empty_text": "No assignments below 70% this term."
  },
  "send_when": "rows",
  "students": [{"student_ref": "…", "decision": "skip", "reason": "Teacher asked to leave this student out"}]
}
```

Rules:

- **Placeholders:** `{{student_name}}` (the existing private Goes By/first-name substitution) and `{{assignment_table}}`. Anything else is an error. `{{assignment_table}}` must appear exactly once in `body`.
- **Table:**
  - Rows are the student's `work_due` items that have a `percent` strictly below `below_percent`. Omitting `below_percent` lists every `work_due` row (a full inventory, if a teacher asks for one).
  - Columns come from the fixed set `assignment` (linked title), `score` (`current / possible`, with `.0` trimmed), `percent` (rounded to a whole number for display), `due` (the student's date in the course timezone) and `submission`.
  - Every label and every `submission_labels` value is required; the Engine supplies no wording.
  - A student in several courses gets one table per course, headed by `course_headings[nickname]` when it is provided and by the nickname otherwise.
  - Use inline-styled HTML, reusing the current `cellStyle`/`headStyle`.
- **Decisions:**
  - `send_when: "rows"` sends to a student whose table has rows and skips a student whose table is empty (reason: `empty_text`).
  - `send_when: "all"` sends to everyone.
  - A `students` entry overrides the decision for that student.
  - The cheap honesty hold from `weeklyHoldReason` stays and is generalized to every send: unresolved course flags, a row with an invalid date, link or score, or an empty table built from incomplete evidence. This becomes `flag`, so a reassuring email never goes out on missing data.
- **Validation:**
  - `run_id` matches and student refs are known and unique.
  - Neither `subject` nor `body` contains a known alias, student ref or email address (generalize `validateWeeklyAnnouncements`).
  - Every required label is present.
- **Removed:** `aiNotice` (from both the email and the printed sheet), `render`, `renderWeekly`, `sortedBy`, `sortedCorr`, `sortedHist`, `noteFor`, `weeklyHoldReason`, and every `ParentConfig` field except `sign_off`. The sign-off is appended the same way Family Contact appends it.
- **Unchanged:** the review page, Outlook delivery, outcomes and the no-resend guarantee in `parent_send.go`, and the teammate reply-to note (it names an address the agent never sees). `Render` returns the subject and HTML as it does today, so `BuildParentReview`, `SendParentUpdates` and `SendParentTest` need no restructuring.
- **Printed sheet:** only the AI notice comes out in this handoff. The sheet stays Engine-rendered (it is a grade check, not the email); moving its wording out is separate work if the teacher wants it.

Tests: full render for a mixed student (one row below 70, one at exactly 70, one unreleased); multi-course headings; empty table skipped with `send_when: rows`; a flag holds the student; unknown placeholder and missing label errors; privacy validation; a schema 1 or 2 draft is rejected with the re-draft message; the reply note and sign-off are placed as in Family Contact.

## Part 3: Adapters and contracts (Engine)

- `engine-docs/formats.md`: replace the schema 1/2 draft contract and the weekly-layout paragraph with schema 3; update the report field list.
- MCP: update the `pull_parent_report` description (the full period is now the default; the flag is kept for compatibility) and any `write_draft` / `validate_document` handling of parent drafts. `engine_reference` examples should show schema 3.
- Run the standard checks from the execution notes, including the compiled MCP smoke.

## Part 4: Playbook and context (seed)

- `playbooks/parent-updates.md`:
  - Replace "Current Engine limits" with the schema 3 authoring steps.
  - The seed supplies the default subject, body, labels, `submission_labels` and `empty_text` for the below-70 format; the teacher's `Teacher_Override.md` or context can change them.
  - The agent reads the portal and tutoring links and the sign-off from context and writes the links into `body`.
  - Step 6: drop "check both HTML and plain text". The Engine sends HTML only, and Outlook generates the plain-text part.
- `context/parent-updates.md` seed: keep `grade_portal_*`, `tutoring_*` and `sign_off` as settings the agent reads. Drop `skip_score`, `correction_days` and `failing_below`, and put the threshold in the rules text, or keep `failing_below` documented as "the agent passes this as `below_percent`". Preserve the teacher's existing customized context on update.

## Part 5: Docs and capabilities

- `docs/parent-update-flow.md`: rewrite it around the facts/content split. Move the legacy layout, corrections and skip sections into a short historical note, and delete what is purely superseded.
- `docs/engine-capabilities.md` and `internal/localui/capabilities.go`, updated together with today's review date: the new format is implemented with **local tests only**. Canvas Live report evidence, an authorized review-page send and inbox arrival are still pending.

## Out of scope

The printed-sheet wording, Family Contact, a corrections email, new UI on the review page, release/packaging, and any change to UserInstall/UserSync. No real-family send is authorized by this handoff.

## Done when

A schema 3 draft written only from the seed playbook produces, for each student, an email with the playbook's wording around an Engine-drawn table of every released assignment below the threshold, showing the student's due date and the independent turned-in label. No AI notice, alias text, Corrections or Due Soon content remains in the Engine. Tests, vet, the MCP smoke and `git diff --check` pass, and the capability records say local evidence only.

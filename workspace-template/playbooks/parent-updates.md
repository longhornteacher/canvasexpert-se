# Parent updates

The judgment calls are yours: what counts as a correction and the wording, within the teacher's sending preferences. The Engine reads Canvas, resolves Missing/0 status and skip eligibility, computes dates and scores, renders the email, looks up parent addresses and sends. You never see an address and never send anything: the teacher reviews every email on a private page and clicks Send there. Background and current rules live in `docs/parent-update-flow.md` in Dev; the teacher does not need that.

Read `AGENTS.md` and `context/parent-updates.md` first. Its grade-portal and tutoring fields start blank. The first time the teacher sends updates, ask once for their grade portal's name and sign-in address and any tutoring sign-up link, and save them there (`skyward_url`, `grade_portal_name`, `bobcat_hour_url`, `bobcat_hour_label`, `bobcat_hour_text`; the key names are historical). Blank fields are fine: a blank tutoring section is left out and the portal name falls back to a default. Also ask once how the teacher wants emails signed and save it as `sign_off`; without it, emails go out unsigned.

## Steps

1. **Scope.** Which courses, and every student unless the teacher names some. One email per student covers all the courses in this run.
2. **Pull.** `pull_parent_report` with the courses (CLI: `.\cse.cmd parent-report COURSE [COURSE ...] [REF ...]`). Read the `report.json` it writes. Students appear by `student_ref` and alias only.
3. **Decide per student**: `send`, `skip` or `flag` (below). The saved report has a calendar scope; do not re-resolve the term when reopening it.
4. **Write drafts** to `drafts.json` beside `report.json`, with `write_draft`.
5. **Review.** `start_parent_review` with the run folder. Give the teacher the link and wait. Then `review_status` once; it returns counts and outcomes by student ref, never addresses, plus any revision notes. Fix revisions and repeat from step 4 only for those students.
6. **Report in one line**: how many were sent, skipped, held or failed. Name any held or failed student by alias only.

## Send, skip or flag

- **Skip** when the Engine's student-level `skip_eligible` is true. This means every included course meets `skip_score` (90 by default) with no Missing/0, or there was no required work due in the report window. The no-work reason is `No required work was due in this report window.` Corrections, Due Soon and links never justify an email by themselves.
- **Flag** (not skip, not send) only when `skip_eligible` is false and a course score is unavailable, a remaining `flags` entry says data is incomplete, or another unresolved problem needs teacher review. Say why in `reason`; the teacher decides. A positive teacher-entered grade with stale missing status or no Canvas submission is already resolved by the Engine and is not grounds for a hold.
- Otherwise **send**.
- The skip decision uses Canvas scores. The school's grade portal is the official grade; it is only linked.

Use the Engine's resolved `missing` list and `skip_eligible`; do not recreate missing status from raw Canvas flags or the absence of an online submission. A current teacher-entered grade above zero overrides those missing indicators. A current recorded zero belongs in Missing/0, even when graded or submitted; zero does not prove the work was never submitted. An earlier zero that has since been raised is grade history, not a current zero. The report includes only assignments whose student-effective due date falls from the later of the current grading-period start or `correction_days` calendar days before `as_of`, through `as_of`. Recent grading activity never pulls older work into the report.

## What goes in each section

Reference assignments by `assignment_id` taken from the report. The Engine fills titles, links, dates and scores from Canvas, so you cannot misstate them; never type a score or date into a note.

| Section | Source in the report | Your part |
|---|---|---|
| `missing` | Resolved `missing` (Missing/0), scoped to the saved due-date inventory | List every in-window item. A `recorded_zero` item reports a current zero without claiming the work was unsubmitted. A `not_submitted_canvas_not_marked_missing` item says honestly that Canvas has not marked it missing. Add a `note` only for existing submission instructions. |
| `corrections` | `corrections` | Never duplicate a Missing/0 item here. Corrections open when the student finishes the work and close on `correction_deadline` (`correction_days` calendar days after the student's due date, 14 by default; the teacher changes it in `context/parent-updates.md`), unless `Teacher_Override.md` sets the teacher's own correction system; then follow it. Feedback is not what opens them. Include work where the student lost points and can still improve it. When `teacher_comments` contain a rationale or exemplar, the `note` points to it ("see the comment on your submission"); otherwise the note points to the assignment itself. Never promise points back or invent a correction method the teacher hasn't set. Hidden answer keys are never correction guidance. |
| `due_soon` | `due_soon` | Include the ones that matter to the family. A `note` may repeat existing preparation instructions from the assignment, nothing new. |
| `history` | `history` | Scored evidence only. The complete inventory is rendered separately; use this list for first/current score history when useful. Where `first_unavailable` is true the Engine shows "not available"; do not guess. |

For the default layout, order is fixed: Missing/0, Corrections, Due Soon, Work due in this period, footer (grade portal and tutoring sign-up). Empty sections are left out. The default email and printable reports render every `work_due` row, including completed, perfect and awaiting-grading work; your draft cannot omit an inventory row. It shows the saved grading-period label and dates. Do not add another full inventory table.

## Defaults

These hold unless the teacher says otherwise. Honest values and privacy are not defaults: they always hold.

- **No invented work.** Everything asked of a student or adult comes from real coursework, published deadlines, existing feedback, or opportunities the teacher gave you. Never add practice, reading, conferences or "talk to your child".
- **No parent-response requests.** Do not ask families to reply, confirm or sign.
- **Honest values.** Scores and dates come from the report. If something is unavailable, the email says it is unavailable.
- **Unreleased grades stay private.** When Canvas explicitly returns `posted_at: null`, the row remains in `work_due` but the Engine withholds its numeric grade, history and unreleased feedback. Use the displayed “Grade not released” status; do not infer Missing/0 from it.
- **Private stays private.** Teacher-only notes (`context/students.md`) never appear in an email. Do not read `.cse`, `student-contacts.json` or any address.
- **Keep it short.** Communicate more with fewer words. Lead with what the family can do.

## drafts.json

```json
{
  "schema": 1,
  "kind": "parent-drafts",
  "run_id": "RUN_ID_FROM_REPORT",
  "students": [
    {
      "student_ref": "s-…",
      "decision": "send",
      "reason": "Missing Lab 3; Canvas score 84.",
      "intro": "Optional one short sentence; {{student_name}} becomes the student's name when rendered.",
      "missing":     [{"assignment_id": 1, "note": ""}],
      "corrections": [{"assignment_id": 6, "note": "Read the teacher comment on your submission."}],
      "due_soon":    [{"assignment_id": 5}],
      "history":     [{"assignment_id": 6}]
    },
    {"student_ref": "s-…", "decision": "skip", "reason": "Engine skip_eligible: all included Canvas course scores meet the cutoff and no Missing/0 items remain."}
  ]
}
```

For the default layout, every student in the report appears once, with a reason for `skip` and `flag`. A `send` decision is valid when the saved `work_due` inventory is nonempty even if there are no Missing/0 or correction selections. If it is empty, choose `skip` with the no-required-work reason or `flag` when report data is incomplete; Due Soon by itself does not create an email. An old report without `scope`, or with invalid scope data, must not be sent or printed: retain its drafts and outcomes, then pull a fresh report.

## Weekly ELA update

When the teacher requests the weekly ELA layout for the full current term, pull a fresh report with `pull_parent_report` and `full_period: true` (CLI: `parent-report --full-period COURSE [COURSE ...]`), then use `schema: 2`, `layout: "weekly_update"`. Supply an `announcements` object keyed by the exact course nicknames in the report. Each value is teacher-authored Markdown for that course, without student names, aliases, student references, grades or email addresses. Use current teacher-supplied announcements; do not reuse a previous week without confirmation. The Engine validates known aliases, student references and email addresses, but the agent must still keep all private information out.

```json
{
  "schema": 2,
  "kind": "parent-drafts",
  "run_id": "RUN_ID_FROM_REPORT",
  "layout": "weekly_update",
  "announcements": {"ELA7": "- **Thursday:** Teacher-provided announcement."},
  "students": [{"student_ref": "s-…", "decision": "send"}]
}
```

Include each student. The weekly layout sends an update even when the work inventory is empty or the score meets the normal skip cutoff; flag unresolved report data instead of inventing an all-clear. It requires one course per student and routes to enabled parents only. The Engine constructs the subject and Missing and Failing lists from the saved report, keeps unreleased grades private, and omits the default inventory, corrections and due-soon sections. It uses the configured `bobcat_hour_url` and `sign_off`. Student names, aliases and references do not belong in the message. Review through `start_parent_review`; **Fill Outlook Drafts only** creates drafts without sending.

## Printable student reports

When the teacher wants reports to hand out in class, use the same run: pull the report, write `drafts.json` as above (decisions still apply to email), then `print_student_reports` with the run folder (CLI: `.\cse.cmd student-report RUN_FOLDER`). The Engine saves one PDF in the teacher's Downloads folder, one student per page, and opens it. Tell the teacher the file name; it holds student names, so do not open or read it.

- Every student in the run gets a sheet, including skipped students, so the handout singles no one out. A student with no Missing/0 items gets the Engine's encouraging empty-state message.
- The sheet lists every resolved Missing/0, Due Soon item and every `work_due` inventory row from the report. Only your `corrections` choices carry over, because eligibility is your judgment; Missing/0 items never repeat under Corrections. It uses the saved period scope, not today's date.
- Optional `student_intro` per student, written to the student ("you"), one short sentence. Same rules as `intro`: no scores, no invented work. `{{student_name}}` works here too.

## If something cannot proceed

Continue with the other students. Explain the one that cannot: no enabled email address on file is shown on the review page as held, and that is for the teacher to resolve in the Students page.

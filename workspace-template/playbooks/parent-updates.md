# Parent updates

The judgment calls are yours: who needs an email, what counts as a correction, and the wording. The Engine reads Canvas, computes dates and scores, renders the email, looks up parent addresses and sends. You never see an address and never send anything: the teacher reviews every email on a private page and clicks Send there. Background and the proposed rules live in `docs/parent-update-flow.md` in Dev; the teacher does not need that.

Read `AGENTS.md` and `context/parent-updates.md` first. `skyward_url` defaults to Pearland ISD's Skyward Family Access sign-in; change it only if the teacher gives a different address.

## Steps

1. **Scope.** Which courses, and every student unless the teacher names some. One email per student covers all the courses in this run.
2. **Pull.** `pull_parent_report` with the courses (CLI: `.\cse.cmd parent-report COURSE [COURSE ...] [REF ...]`). Read the `report.json` it writes. Students appear by `student_ref` and alias only.
3. **Decide per student**: `send`, `skip` or `flag` (below).
4. **Write drafts** to `drafts.json` beside `report.json`, with `write_draft`.
5. **Review.** `start_parent_review` with the run folder. Give the teacher the link and wait. Then `review_status` once; it returns counts and outcomes by student ref, never addresses, plus any revision notes. Fix revisions and repeat from step 4 only for those students.
6. **Report in one line**: how many were sent, skipped, held or failed. Name any held or failed student by alias only.

## Send, skip or flag

- **Skip** a student only when every included course has a Canvas `course_score` at or above `skip_score` **and** has no `missing` items. Corrections, Due Soon and links never justify an email by themselves.
- **Flag** (not skip, not send) when a course score is unavailable, a `flags` entry says data is incomplete, or something looks wrong. Say why in `reason`; the teacher decides.
- Otherwise **send**.
- The skip decision uses Canvas scores. Skyward is the official grade; it is only linked.

## What goes in each section

Reference assignments by `assignment_id` taken from the report. The Engine fills titles, links, dates and scores from Canvas, so you cannot misstate them; never type a score or date into a note.

| Section | Source in the report | Your part |
|---|---|---|
| `missing` | `missing` | List all of them, even if older than 14 days. A `not_submitted_canvas_not_marked_missing` item says honestly that Canvas has not marked it missing. Add a `note` only for existing submission instructions. |
| `corrections` | `corrections` | Include an assignment only when its `teacher_comments` actually contain a **rationale or an exemplar** the student can learn from. A generic "good job", a bare score, or one keyword is not enough. For a New Quiz marked `per_item_feedback: unverified`, include it only if the teacher has said it has item feedback; otherwise leave it out and say nothing. Hidden answer keys are never correction guidance. The `note` says where to find the explanation ("see the comment on your submission"). |
| `due_soon` | `due_soon` | Include the ones that matter to the family. A `note` may repeat existing preparation instructions from the assignment, nothing new. |
| `history` | `history` | Include all of them. The email shows First Score and Current Score, so families can see where a curve or correction points raised a grade. Where `first_unavailable` is true the Engine shows "not available"; do not guess. |

Order is fixed: Missing, Corrections, Due Soon, Grade history, footer (Skyward and Bobcat Hour). Empty sections are left out.

## Rules that do not bend

- **No invented work.** Everything asked of a student or adult comes from real coursework, published deadlines, existing feedback, or opportunities the teacher gave you. Never add practice, reading, conferences or "talk to your child".
- **No parent-response requests.** Do not ask families to reply, confirm or sign.
- **Honest values.** Scores and dates come from the report. If something is unavailable, the email says it is unavailable.
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
    {"student_ref": "s-…", "decision": "skip", "reason": "Canvas 96, nothing missing."}
  ]
}
```

Every `send` student needs at least one item in a section. Every student in the report appears once, with a reason for `skip` and `flag`.

## If something cannot proceed

Continue with the other students. Explain the one that cannot: no enabled email address on file is shown on the review page as held, and that is for the teacher to resolve in the Students page.

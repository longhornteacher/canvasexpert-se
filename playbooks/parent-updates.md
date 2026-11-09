# Parent updates

Updated 2026-11-07: full-term assignments below 70, with direct links, due dates and submission status. This replaces the correction-window email and its course-average skip rule.

You write the email; the Engine supplies the facts. The Engine reads Canvas, matches students, computes percentages, draws the assignment table from its own facts, fills in real names privately, and sends through Outlook. Everything a family reads around that table is yours: subject, greeting, introduction, column headings, status words, empty-list sentence, links and sign-off. The teacher reviews the private page and chooses Send or Outlook Drafts there. Never send through another service or expose contact details in agent context.

Read AGENTS.md, Teacher_Override.md, context/parent-updates.md and the relevant school-calendar grading period first. Use the saved grade-portal and tutoring links and the teacher's sign-off. Ask for missing settings only when needed; never ask again for saved settings.

## Required parent-update format

Use one short introduction naming the class and requested term (for example **Term 2**), followed by one assignment list. Use the saved term's actual dates, not a rolling correction window.

| Assignment | Score | Due date | Turned in? |
|---|---|---|---|
| Assignment title linked directly to that Canvas assignment | Current released score with its points possible | Student-effective due date | Evidence-based submission status |

The Engine draws this table: every title links directly to that Canvas assignment, rows run by due date then title, and a student in several classes gets one table per class. Show each assignment once. End with the configured grade-portal link (the official grade), the tutoring link when one is saved, and the teacher's sign-off.

Do not include an AI disclaimer, AI-assistance explanation, Pokémon name, alias, or explanation of how aliases protect identities anywhere in family-facing subjects, introductions, tables or footers. This is a presentation rule: keep internal pseudonymization and private identity matching intact. Use `{{student_name}}` where the child's name belongs; the Engine fills it in privately.

Do not include a **Corrections available** section or announcement, correction deadlines, promises of points back, retake invitations or statements that listed work can be resubmitted. A low grade does not establish correction eligibility. This email reports current work and links to it; it does not announce a correction policy.

Do not add Due Soon, first/current score history, a complete inventory of passing assignments, announcements, conferences, practice or requests for families to reply unless the teacher separately asks.

## Which assignments to list

- Every report covers the current grading term from its start (context/school-calendar.md) through the report's as_of date. Do not clip the list to correction_days or correction eligibility.
- List **every current released assignment score below 70%** in that term, including a recorded zero. Set `below_percent: 70` in the draft; the Engine compares each released score to its points possible. Exactly 70% is not below 70: 34/50 is listed, 35/50 is not.
- The report already uses published, student-visible, non-excused work and each student's own effective due date. An individualized deadline controls term membership.
- Low-scoring work is listed whether or not it was turned in. A recorded zero does not prove that work was never submitted.
- The Engine uses the latest released grade. An earlier failing score raised to 70% or above is no longer listed.
- An absent or unreleased grade is not a zero and is never listed as below 70.

The threshold applies to assignments, not the overall course average. Do not suppress below-70 assignments because a student's overall Canvas average is 90 or higher. Never label Canvas's overall course score as a term grade. Keep the grade portal as the official-grade destination.

## Submission status is separate evidence

The report gives each row a `submission` value. Word them for families in `submission_labels`; the seed wording is:

- `turned_in` → **Turned in**: Canvas records a submission or completed quiz attempt, including work that earned zero or lost late points.
- `not_turned_in` → **Not turned in**: Canvas shows no submission for online work that is past due, or the work is marked missing with no positive grade.
- `no_online_submission` → **No online submission required**: paper or offline work. Physical work may have been turned in; it is never called unsubmitted for lacking an upload.
- `unknown` → **Status unavailable**: the evidence cannot establish it, for example a positive grade on online work with no upload.

## The draft

Save `drafts.json` beside the report with write_draft (schema 3; the exact fields are in engine_reference formats under "Parent update reports and drafts"):

- `subject` and `body` in Markdown. The body holds `{{assignment_table}}` once, on its own line, where the list goes. Write the links from context into the body, for example `[Skyward Family Access](URL) for official grades.`
- `table`: `below_percent: 70`, `columns: ["assignment", "score", "due", "submission"]`, a family-friendly label for each column, the four `submission_labels` above, and `empty_text` such as "No assignments below 70% this term." Add `course_headings` with each class's family-facing name when an update covers several classes.
- `send_when: "rows"` sends to every student with at least one listed assignment and skips the rest. Add a `students` entry only to override one student's decision (send, skip or flag, with a reason).
- `recipients: "parents_only"` only when the teacher wants the student left off; otherwise the student is To and enabled parents are CC.

Wording must not name or alias a student or contain an email address; the Engine refuses it. Keep it brief, warm and plain.

## Prepare, review and deliver

1. **Scope.** Use saved course nicknames and the requested term. Every student is in scope unless the teacher narrows the audience. Resolve ambiguous classes before pulling.
2. **Pull.** Run pull_parent_report for the classes. Read the pseudonymized report, not private contact or roster files.
3. **Draft.** Write the schema 3 draft described above. Do not calculate grades, recreate student matching or rewrite report facts.
4. **Decide.** The Engine skips students with no listed assignment and holds any student whose class has unresolved evidence (`flags` in the report), so an incomplete list never goes out as complete. Tell the teacher about held students by alias and the specific reason.
5. **Review.** Use start_parent_review, give the private URL to the teacher, and wait. The page shows each rendered email; only the teacher chooses Send or Outlook Drafts there. After completion, use review_status for outcomes and revisions. Never inspect the private page or addresses yourself.
6. **Resume and report.** Preserve accepted sends and delivery history. Revise only held or revised work and resume the same run without resending completed emails. Report sent/drafted, skipped, held and failed counts.

A run drafted in an older format is refused with a re-draft message; write a schema 3 draft for the same report.

## Other requests

Activity reminders and free-form family messages use family-contact.md; book-club family emails also use family-contact-book-clubs.md. Do not reintroduce a different grade-update layout merely because the teacher calls an update weekly; change the wording, threshold or columns in the draft instead. A separately requested class announcement follows that playbook and requested audience.

Printable student grade sheets are a separate request: pull, then use print_student_reports; drafts.json is not required (a `students` entry may add a `student_intro`). The sheet lists the whole term so far plus a checklist of upcoming work. Do not read the generated PDF: it can contain real student names.

## Accuracy and privacy that always hold

Keep unreleased grades, accommodations, teacher-only notes, private scoring rationale and possible-AI-use flags out of family messages. Never read .cse or contact addresses as ordinary model context. Identify unavailable facts honestly, and never turn missing evidence into a zero or an accusation that work was not turned in. Require no parent response, signature or confirmation.

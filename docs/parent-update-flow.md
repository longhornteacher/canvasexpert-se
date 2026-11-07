# Parent Update Flow

Updated 2026-11-07 · The Engine supplies facts; the playbook writes the email. Implemented in Dev with local tests only; no Canvas Live report, Outlook draft or family send has used this format yet.

Purpose: the current parent-report contract and the evidence behind it. Read alongside [ARCHITECTURE.md](ARCHITECTURE.md) and [SCORING-RULES.md](SCORING-RULES.md). The authoring workflow is the seed `playbooks/parent-updates.md`; the file contract is `engine-docs/formats.md` ("Parent update reports and drafts"). [Family Contact](family-contact-flow.md) shares the private review, recipient routing, Markdown renderer and Outlook delivery for free-form class messages.

## Who decides what

Rule (developer, 2026-11-07): "Engine keeps factual/deterministic things. Playbook is the dressing."

| Engine (facts and safety) | Playbook and context (dressing and policy) |
|---|---|
| Canvas reads, student/course identity, student-effective due dates, visibility, excused work | Subject, greeting, introduction, sign-off placement |
| Unreleased grades stay private; released score, exact percent, canonical link | Which rows appear (`below_percent`), which columns, column labels |
| Submission evidence independent of the grade | Words for each submission status and for an empty list |
| Drawing the assignment table so a draft cannot misstate a score, date or link | Grade-portal and tutoring links, course headings |
| Holding a student whose evidence is incomplete; refusing wording with aliases, refs or addresses | When to send (`send_when`, per-student overrides), recipients (`parents_only`) |
| Real-name substitution, recipient routing, teammate reply-to note, Outlook delivery, outcomes and no-resend | Anything else a family reads |

Before 2026-11-07 the Engine also wrote the subject, section headings, an AI disclaimer, the weekly email's sentences, and the correction, Due Soon and skip policies. Those are gone from the Engine; the [handoff](../handoffs/active/HANDOFF_Parent_Email_Content_to_Playbook_2026-11-07.md) records the audit.

## Current requirements (teacher, 2026-11-07)

One list per student of every current released assignment score below 70% this term, with a direct assignment-title link, score, student-effective due date and independent submission status. Exactly 70% is excluded. An overall course average at or above 90 does not suppress the list. Skip an empty list; hold incomplete evidence rather than send an all-clear. No AI disclaimer, alias or privacy explanation, Corrections section, correction deadlines, Due Soon, history or passing-work inventory. Keep the configured grade-portal and tutoring links and the teacher's sign-off. Do not present the overall Canvas course score as a term grade.

## Report (`report.json`, schema 2)

The report covers the current grading period from its start through `as_of`; `full_period` is accepted for compatibility and changes nothing. Per student and course: `course_score`, `flags` and `work_due`. Each `work_due` row is a published, visible, non-excused assignment due in the window, ordered by due date then title, with:

- `status`: `graded`, `awaiting_grading`, `grade_not_released` (no score shown) or `no_grade` (including `not_graded` assignments);
- `canvas_score`, `points_possible`, `percent` (two places; selection compares the exact ratio), `points_deducted`;
- `submission`: `turned_in` (a submission or attempt exists), `not_turned_in` (online work past due with no submission, or marked missing, with no positive grade), `no_online_submission` (paper/offline work) or `unknown` (for example a positive grade on online work with no upload).

A positive teacher-entered grade resolves a stale missing flag. A recorded zero is a graded score and does not establish that nothing was turned in. Absent or unreleased grades are never zeros. `upcoming` lists later work through the period end for the printed sheet. The report holds no sections, skip advice, comments or grade history; the gradebook-history read was removed.

## Draft (`drafts.json`, schema 3) and rendering

The draft carries the subject and Markdown body with `{{student_name}}` and one standalone `{{assignment_table}}`, plus the table settings and decisions. The Engine renders the body with the Family Contact renderer (sanitizing, private name substitution, teammate reply note, saved sign-off), then replaces the placeholder paragraph with its own table. Older schema 1/2 drafts are refused with a re-draft message; their reports and outcomes are preserved.

Decisions: `send_when: "rows"` sends to students with at least one row and skips the rest; `"all"` sends to everyone; `students` entries override. Any send for a student whose course has `flags`, or a row with an invalid date, link or score, becomes a `flag` hold with the reason.

## Printed grade sheets

The printed sheet is a grade check, not the email: one table per course in due-date order with assignment, due date and current grade (or "Awaiting grading", "Grade not released", "Not turned in" or "No grade recorded"), and a check box for upcoming work. Printing reads only `report.json`; `drafts.json` may add a `student_intro`. The AI notice was removed 2026-11-07. The sheet's own wording remains Engine-rendered.

## Delivery

Historical design record, decided 2026-10-16. Copy-paste delivery is ruled out. The implemented review/send path is recorded below in Implementation notes; the earlier delivery proposals are retained here for context.

| Content | Channel | Notes |
|---|---|---|
| Individual student update | Classic Outlook automation (COM) in the teacher's own district mailbox | The Engine fills recipient, subject and body from private contacts; the agent never sees addresses. Sent mail stays in district Sent Items, and replies arrive in the normal inbox. |
| Class-wide reminder | Band post | Band posts are visible to the whole band, so they carry no per-student data. Posting by computer use or the BAND API is open; the API's posting support is unconfirmed. |
| Text messages | None | No texting service. Per-contact text preferences are kept, but they don't drive delivery for now. |

Considered and set aside:
- **Personal Gmail or another outside email service:** this raises district policy and records-retention concerns, and an unknown sender looks less trustworthy to families.
- **Computer use for individual emails:** this would put parent addresses in agent context and risks autocomplete picking the wrong recipient.
- **Microsoft Graph:** this likely needs district admin consent.

**Local evidence (2026-10-16, PC NEWSPEEDY; not the district PC):**
- PowerShell ran in FullLanguage mode with classic Outlook 16.
- The automation saved a draft addressed to the teacher in Drafts, and the recipient resolved.
- A new message sent with `Send()` produced no security prompt and appeared in Sent Items.
- Inbox arrival was not checked.
- `Send()` on a saved draft that was open inline in the reading pane failed ("inline response mail item"). The Engine should create and send fresh items rather than sending saved drafts.

Still open:
- Behavior on the locked-down district PC, including the language mode and any Outlook security prompt.
- Whether the Engine sends after the teacher approves the batch, or only fills Drafts for the teacher to send.
- The Engine-side implementation. `go-ole` from Go is the likely route and avoids depending on PowerShell.
- HTML versus plain-text body.

### Delivery from Cowork and Work (MCP): historical design

Outlook delivery is an Engine capability, not a separate connector. Cowork's thin extension and Work's direct registration both start UserInstall's `engine/cse.exe` on the teacher's Windows PC as the signed-in user. Outlook automation runs in that same process context, so whatever route reaches the Engine also reaches Outlook. The agent's own sandbox, such as Cowork's Linux VM, never touches Outlook.

The proposed shape follows the shared family-review session / `review_status` pattern:

1. **Draft.** The agent reads current Canvas data and writes per-student update drafts and the class-wide Band text to `courses/` with `write_draft`. Drafts use student references, never addresses.
2. **Review.** A tool like `start_parent_review` opens a private local WebUI page and returns only its loopback URL. On the page, the Engine resolves recipients from `.cse/student-contacts.json` against Canvas instance and user ID. The teacher sees each family's actual To line and the email body, and can skip, edit or approve the batch.
3. **Send.** The teacher's Send button triggers the Engine directly, which creates and sends a fresh Outlook item for each recipient. The agent never calls send and never sees addresses, so this needs no MCP send tool or approval token. A "Drafts only" option on the same page fills Outlook Drafts instead.
4. **Outcome.** `review_status` returns counts and outcomes by student reference only: sent, skipped, failed or uncertain, with no addresses. Outcomes are recorded locally in UserInstall so a retry never resends an accepted message.

Band follows the same page, with no copy-paste. A "Post to Band" button uses the BAND API if it supports posting; otherwise, Cowork computer use posts the approved class-wide text. See [the historical handoff](../handoffs/retired/PARENT-UPDATES.md), slice 4.

Engine notes:
- Do COM work on a dedicated OS-locked thread (`go-ole`, `CoInitialize`) so the MCP server's goroutines don't matter.
- Starting Outlook when it isn't running is acceptable.
- An Engine running at a different integrity level from Outlook, for example elevated, can fail to connect. Report that specifically rather than retrying.
- No PowerShell dependency, so constrained language mode doesn't block delivery.

Dependencies and limits:
- This rides on the general Cowork/Work MCP adoption still pending in [capability evidence](engine-capabilities.md). Outlook adds no new client requirement.
- Sending needs the PC on, the teacher signed in, and classic Outlook configured. A hosted tunnel route still sends from that PC. Scheduled or unattended sending inherits the same limits.
- Not yet verified: Go COM send, the district PC, and any MCP client calling this flow.

## Implementation notes (updated 2026-11-07; earlier bullets are history)

- `internal/outlook` (go-ole, one locked COM thread); `cse-probe outlook-self-test` sent a message to the profile's own address from Go on NEWSPEEDY. District PC, inbox arrival and MCP clients are not yet verified.
- For 1.1.0-beta.1, the Engine resolves positive-grade overrides and current recorded zeros before drafting, removes Missing/0 duplicates from Corrections, retains score history, and computes `skip_eligible` after scoped classification. Loading a run recomputes advisory eligibility from its saved inventory and flags; it does not reclassify from live Canvas data. Focused local regressions cover these rules; no new Canvas Live or delivery run establishes them.
- `pull_parent_report` / `cse parent-report COURSE... [REF...]` writes `courses/<first course>/parent-updates/<run>/report.json`; the agent writes `drafts.json` beside it (contract in `playbooks/parent-updates.md`); `start_parent_review` / `cse parent-review RUN_FOLDER` opens the review page, which sends one fresh Outlook message per student (the student's email in To, every enabled parent email in CC; parents in To when no student email is on file) and records one outcome per student ref in `outcomes.json`. `print_student_reports` / `cse student-report RUN_FOLDER` saves one PDF from the same run, one student-voiced page per student, in the teacher's Downloads folder (headless Edge; a printable HTML file if Edge cannot print) and opens it.
- Not built from the handoff: a local CE-SE posting record for correction candidates (teacher comments are returned instead).
- **Band dropped (2026-10-16):** the teacher does not want Band in this flow. No Band post, card or `band_post` field exists; earlier Band notes above are historical.

- 2026-11-07 (local tests only): reports carry facts only (percent, independent submission evidence, whole-term window); drafts are schema 3 wording plus an Engine-drawn table; the default and weekly layouts, AI notice, corrections, Due Soon, history and skip_eligible were removed.

## Remaining checks

A Canvas Live report under the new contract, an authorized email from the review page with inbox arrival, the district PC, printing on paper and MCP-client adoption remain pending; see [capability evidence](engine-capabilities.md). Update it and [WebUI capability data](../internal/localui/capabilities.go) together when that status changes.

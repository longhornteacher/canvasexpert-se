# Parent Update Flow

Updated 2026-10-18 · Current-term due inventory implemented locally; existing Outlook evidence remains limited to NEWSPEEDY; Band is dropped

Purpose: preserve the current parent-report contract and the evidence behind it. Read alongside [ARCHITECTURE.md](ARCHITECTURE.md) and [SCORING-RULES.md](SCORING-RULES.md). Sections explicitly marked historical record earlier design choices; the current playbook and rules below govern fresh reports.

Reviewed 2026-10-20: [Family Contact](family-contact-flow.md) extends the same private review, recipient routing and Outlook delivery for free-form class messages. This document and `playbooks/parent-updates.md` continue to govern grade-update runs unchanged. Free-form `contact.json` runs contain no grade report and do not weaken complete-inventory, calendar, grade-privacy or skip guarantees here.

## Settled direction

**Communicate more with fewer words. Lead with actionable items.**

- Full update by email from the teacher's own district mailbox through classic Outlook automation, reviewed and sent by the teacher on a private page. No texting service; Band was dropped. See [Delivery](#delivery).
- Missing/0 appears first, with direct Canvas assignment links. The Engine builds the complete required-work inventory by student-effective due date for the last 14 calendar days of the current grading period, clipped to the period. A current positive teacher-entered grade overrides stale Canvas missing/no-submission indicators, and a current recorded zero qualifies even when graded or submitted.
- Missing/0 items never repeat under Corrections; retain their grade-history entries.
- Corrections are available as soon as work is finished and close 14 calendar days after the student's applicable due date, including weekends, unless the teacher has set another correction system. Feedback need not exist first; use it for guidance when available.
- **Work due in this period** shows every assignment in the saved due-date inventory, including complete/perfect, awaiting-grading and ungraded work. First/current score provenance appears when available; unknown values remain clearly unavailable.
- Direct parents to the teacher's configured grade portal for the official course grade. Portal and tutoring links are teacher-owned setup; the seed leaves them blank.
- Include **Due Soon**.
- Include the teacher's configured tutoring or clubs link, when supplied. This teacher's original choice was **Bobcat Hour: [Sign up for tutorials and clubs](https://flex.securly.com/login)**; it is not a product default.
- Skip after Missing/0 classification when every included course meets `skip_score` (90 by default) and no Missing/0 items remain. An empty retrospective inventory is also eligible for a no-required-work skip. Incomplete-data flags take precedence. The report exposes advisory student-level `skip_eligible`.
- **No parent-response requests. Agents must not invent work for adults or students.** Actions and instructions must come from actual coursework, published deadlines, existing feedback, or teacher-provided opportunities.

## Email shape

Keep student/course identity and an “As of” timestamp compact. Put actions before history; explain score labels once. Assignment titles are the links.

| Order | Content | Compact row shape |
|---|---|---|
| 1 | Missing/0 | Linked assignment · Current zero or resolved missing status · Due date · Existing instructions, if needed |
| 2 | Corrections available | Linked assignment · Correction deadline · Where to find the rationale/exemplar |
| 3 | Due Soon | Linked assignment · Due date · Existing preparation instructions, if any |
| 4 | Work due in this period | Linked assignment · Due · Status · First Score · Current Score |
| Footer | Official grade and tutoring | Teacher-configured portal and optional tutoring links |

Sort corrections by closing deadline and Due Soon by due date; omit empty sections. All retrospective sections come from assignments due in `[max(period start, as_of minus 14 calendar days), as_of]`; recent grading activity never admits an older due date. Due Soon remains a separate seven-calendar-day upcoming window and stops at the period end. A current zero remains in Missing/0 without implying it was unsubmitted. Every `work_due` row is rendered automatically, so an agent draft cannot omit completed or awaiting-grading work. Both email and printable output show the saved period label and dates. An empty inventory is described as “No Missing/0 work in this report window” only when evidence is complete; incomplete flags prevent an all-clear statement.

Grade history exists so families can see where a score was raised, by a curve or by points for corrections. Decided 2026-10-16: no explanatory sentence; the Skyward footer link covers the official grade.

Historical design: a brief class-wide Band post with no student names was considered. Band was dropped on 2026-10-16 and has no current draft or delivery action.

## Correction eligibility and evidence

The Engine supplies finished-work candidates inside the correction window, excluding Missing/0. The agent includes work where the student lost points and can still improve it, following the teacher's own correction system when provided. Student-visible, teacher-provided rationales or exemplars can guide corrections, either per item or for the whole assignment; they are not a prerequisite for availability. Identify the actual content; a generic feedback comment or an isolated keyword does not establish guidance. Hidden answer keys are not parent-facing correction guidance.

Correction eligibility remains governed by the applicable correction deadline and teacher override policy. The report's retrospective inclusion window is separate from that deadline: work due earlier than the saved reporting window is excluded, even if its correction deadline is still open. Resolve the student's applicable due date and course timezone; do not infer a changed deadline from a saved extra-time flag alone. Use the report's exact `correction_deadline` rather than inventing a cutoff.

Retain available comment/item references so the draft can accurately direct the family. Unreadable or unsupported per-item feedback is unavailable guidance, not proof that the assignment is ineligible. New Quiz per-item feedback remains a pending evidence limit.

Eligibility and sending are separate decisions: the Corrections section never independently defeats the skip rule. Already-perfect work is not a lost-points correction candidate.

## Score and status accuracy

| Field | Meaning |
|---|---|
| First Score | First score Canvas recorded for the assignment (gradebook history), when available |
| Current Score | The assignment's current Canvas score, when available |

Preserve score denominators and the associated assignment/attempt. Show unavailable history honestly; the earliest surviving comment is not automatically proof of the original assessment. With one assessment confirmed as both first and latest, both columns show it without implying corrections occurred.

Do not substitute Canvas's entered score for the original earned assessment or infer the cause of a score difference. Explain adjustments only when supported by evidence. New Quiz written-work comments may contain criterion scores without an aggregate quiz assessment; preserve that distinction.

A current recorded zero qualifies for Missing/0; it does not establish that work was never submitted. A current zero still qualifies if penalties reduced a positive entered grade to zero; it must not be called unsubmitted on that basis. A positive current teacher-entered grade overrides stale Canvas missing status or absence of an online submission. Excused work remains excluded. The Engine makes classifications before drafting, keeps Missing/0 out of Corrections and preserves scored history for inventory rows. Required ungraded coursework remains in the inventory with no invented score. The agent uses the resolved lists rather than inferring missing status from raw flags. Include only released, parent-appropriate information; private teacher notes and credentials stay out.

## Conditional sending: approved rules

Apply the configured cutoff only after the Engine resolves Missing/0 for every included course. One email combines the student's included courses. The Engine exposes `skip_eligible` for the agent's decision; it is true when every included course score meets `skip_score` (90 by default) with no Missing/0, or when the complete `work_due` inventory is empty. Other unresolved data flags still require teacher review and make automatic skip ineligible.

| Condition | Result |
|---|---|
| Any included course score below the configured cutoff | Send, unless another unresolved problem requires a flag |
| Any resolved Missing/0 item, including a current recorded zero | Send, unless another unresolved problem requires a flag |
| Every included course score meets the cutoff and no Missing/0 items remain | Skip when `skip_eligible` is true |
| No required work was due in the report window | Skip with `No required work was due in this report window.`; Due Soon alone does not create an email |
| Course score unavailable or report data incomplete | Flag for teacher review; do not assume the skip criteria are met |

Corrections, Due Soon and configured links enrich an otherwise warranted email; they do not independently trigger one. A positive teacher-entered grade resolves stale missing/no-submission indicators, so those indicators alone neither create a Missing/0 item nor justify a hold. A current Canvas score of zero still qualifies as a recorded zero, without implying unsubmitted work. Other incomplete-data and contact holds remain real and are explained specifically.

The cutoff uses current Canvas `course_score` values. The configured school grade portal remains the official course-grade source; its link supplies no grade data and establishes no integration. The playbook and `context/parent-updates.md` seed retain these standing preferences. Actual teacher overrides and existing customized context remain teacher-owned; product updates preserve them for reconciliation.

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

The proposed shape follows the existing `start_review` / `review_status` pattern:

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

## Implementation notes (updated 2026-10-18)

- `internal/outlook` (go-ole, one locked COM thread); `cse-probe outlook-self-test` sent a message to the profile's own address from Go on NEWSPEEDY. District PC, inbox arrival and MCP clients are not yet verified.
- For 1.1.0-beta.1, the Engine resolves positive-grade overrides and current recorded zeros before drafting, removes Missing/0 duplicates from Corrections, retains score history, and computes `skip_eligible` after scoped classification. Loading a run recomputes advisory eligibility from its saved inventory and flags; it does not reclassify from live Canvas data. Focused local regressions cover these rules; no new Canvas Live or delivery run establishes them.
- `pull_parent_report` / `cse parent-report COURSE... [REF...]` writes `courses/<first course>/parent-updates/<run>/report.json`; the agent writes `drafts.json` beside it (contract in `playbooks/parent-updates.md`); `start_parent_review` / `cse parent-review RUN_FOLDER` opens the review page, which sends one fresh Outlook message per student (the student's email in To, every enabled parent email in CC; parents in To when no student email is on file) and records one outcome per student ref in `outcomes.json`. `print_student_reports` / `cse student-report RUN_FOLDER` saves one PDF from the same run, one student-voiced page per student, in the teacher's Downloads folder (headless Edge; a printable HTML file if Edge cannot print) and opens it.
- Not built from the handoff: a local CE-SE posting record for correction candidates (teacher comments are returned instead).
- **Band dropped (2026-10-16):** the teacher does not want Band in this flow. No Band post, card or `band_post` field exists; earlier Band notes above are historical.

## Saved report scope and inventory

New `schema: 1` reports add a non-null top-level `scope` object with the calendar timezone, grading-period name and inclusive dates, `recent_start_at`, `recent_end_at` (the report's `as_of`) and exclusive `due_soon_end_at`. Timestamps use RFC3339. The Engine reads grading periods from the active `context/school-calendar.md` through teacher-document routing, selects the one period containing `as_of` in that calendar timezone, and fails with a calendar correction message if the list is missing, malformed, overlapping or leaves `as_of` in a gap. The seed contains all four verified 2026–27 periods; school dates remain editable teacher context.

Each course section has an explicit `work_due` array. It includes published, visible, non-excused assignments due in `[max(period start, as_of minus 14 calendar days), as_of]`, using the student-effective date, with the assignment date as fallback only when no student override makes the work undated. A missing bulk submission row is resolved through the existing targeted student/assignment read before classifying visibility. Unavailable or malformed relevant evidence is reported as a course flag; known undated work is outside this due-date report. Required ungraded work remains present with an honest no-grade status. Missing/0, Corrections, and scored history are derived from that inventory. Due Soon is separate and ends at the earlier of seven calendar days after `as_of` or midnight after the grading period.

Both email and printable output include the complete Work due in this period table directly from `work_due`, regardless of which optional section references the agent selected. Saved scope is used on reopen. An empty complete inventory calls for a no-required-work skip; Due Soon alone does not justify an email. Scope validation rejects missing legacy scope with a fresh-pull instruction and rejects malformed saved bounds before sending or printing, preserving the old drafts and outcomes. Local regression tests do not establish Canvas Live audience completeness or new Outlook delivery evidence.

## Architecture and current evidence

Develop this feature in **Dev**. Future teacher use and report drafts belong in **UserInstall**. Reusable preferences and private contacts follow configured **UserSync** authority; session drafts and delivery outcomes remain local to UserInstall.

- **Playbook/context:** teacher's sending policy, correction judgments, wording, priorities, report ranges, teacher instructions and configured links.
- **Engine:** Canvas reads, exact student/course/attempt identity, dates and score arithmetic, deterministic Missing/0 classification and skip eligibility from the configured policy, evidence references, rendering, private recipient resolution and delivery outcomes.
- Reuse existing packages and plain files. A preliminary report needs no permanent gradebook, hosted parent portal, or general messaging framework.

Historical implementation starting points (the reporting path is now implemented):

- [Canvas read types](../internal/workflow/canvas_reads.go) include assignments, submission statuses, comments, attempts, and score fields. Complete per-item feedback access and release checks still need investigation for this flow.
- [The scoring pull](../internal/workflow/pull_work.go) deliberately skips work without an actual submission. Its packets alone cannot provide the missing-work report; add a reporting read using the existing Canvas transport.
- [Feedback rendering](../internal/render/render.go) supports an ordinary-assignment assessment header, but teacher-customized feedback and missing history require honest unavailable values.
- [Private contact storage](../internal/workspace/contacts.go) already holds parent destinations and independent email/text preferences. Keep addresses and phone numbers out of ordinary agent context; resolve recipients privately against the correct student and Canvas instance.
- [Current capabilities](engine-capabilities.md) distinguish implemented parent reports/review/delivery from the limited recorded Canvas Live and Outlook evidence. This document establishes no new live evidence.

Official API references: [submissions](https://developerdocs.instructure.com/services/canvas/resources/submissions) and [assignments](https://developerdocs.instructure.com/services/canvas/resources/assignments). Documentation supports investigation; it is not live verification of this feature.

## Historical handoff outline and remaining checks

The outline below predates implementation. Sending rules, Canvas cutoff scores, combined-course emails and teacher review/send interaction are now settled above. No sending cadence has been selected; configured portal/tutoring links belong to each teacher. District-PC behavior, review-page delivery/inbox arrival, MCP-client use, New Quiz per-item feedback and paper printing remain the capability checks recorded in [capability evidence](engine-capabilities.md).

Historical proposed choices for the first implementation slice:

1. Sending/suppression rules, grade source, and any correction or deadline overrides.
2. One course per email versus combined courses; history range and Due Soon horizon. Seven days for Due Soon was suggested, not approved.
3. Exact correction cutoff time; treatment of completed corrections and already-perfect work.
4. District Skyward URL and manual versus scheduled initiation. No sending cadence has been chosen.
5. Teacher preview/send interaction (batch-approved send versus Drafts only) and handling of contacts with text enabled but no enabled email destination.

Proposed flow: **select scope → read current Canvas data → decide send/skip → draft → teacher review → send → record outcomes**. Review/send details remain a proposal. Preserve unsent drafts, respect each contact's enabled email destinations, and distinguish accepted, failed, and uncertain delivery before retrying. Outlook's Sent Items is the teacher-visible record.

Possible coding slices, after direction is agreed:

1. Reporting reads, status/score evidence, and correction classification.
2. Compact email drafts, the class-wide Band post, and the private parent review page with its MCP start/status tools.
3. Classic Outlook delivery from the review page (Go COM) and per-recipient outcomes. A small Go COM send probe to the teacher's own address could come first, to confirm the Engine route through Cowork/Work.

Use focused checks for date boundaries, missing versus ungraded/excused work, score provenance, private recipient routing, and the agreed sending rules. Validate actual per-item access and delivery when those capabilities are implemented. Update [capability evidence](engine-capabilities.md) and [WebUI capability data](../internal/localui/capabilities.go) together when implementation or verification status changes.

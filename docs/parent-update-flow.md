# Parent Update Flow

Updated 2026-10-16 · Implemented locally (slices 0-3, 5); Outlook route verified on NEWSPEEDY only; Band is dropped

Purpose: preserve the agreed direction and unresolved choices for later architecture sessions and an eventual coding handoff. Read alongside [ARCHITECTURE.md](../ARCHITECTURE.md) and [SCORING-RULES.md](../SCORING-RULES.md). Proposals below are not settled requirements.

## Settled direction

**Communicate more with fewer words. Lead with actionable items.**

- Full update by email from the teacher's own district mailbox through classic Outlook automation. Class-wide reminders go to Band. No texting service. See [Delivery](#delivery).
- Missing / not turned in work appears first, with direct Canvas assignment links.
- Corrections are available for assignments due within the preceding 14 calendar days, including weekends.
- Any assignment with a rationale or exemplar in comments is correctable. Either per-item or whole-assignment comments qualify; no assignment-type or score threshold was selected for eligibility.
- Grade history shows linked assignment, first earned assessment, latest earned assessment, and current Canvas score.
- Direct parents to linked Skyward Family Access for the official course grade. The district URL remains to be supplied.
- Include **Due Soon**.
- Include **Bobcat Hour: [Sign up for tutorials and clubs](https://flex.securly.com/login).**
- **No parent-response requests. Agents must not invent work for adults or students.** Actions and instructions must come from actual coursework, published deadlines, existing feedback, or teacher-provided opportunities.

## Email shape

Keep student/course identity and an “As of” timestamp compact. Put actions before history; explain score labels once. Assignment titles are the links.

| Order | Content | Compact row shape |
|---|---|---|
| 1 | Missing / not turned in | Linked assignment · Due date · Existing submission instructions, if needed |
| 2 | Corrections available | Linked assignment · Correction deadline · Where to find the rationale/exemplar |
| 3 | Due Soon | Linked assignment · Due date · Existing preparation instructions, if any |
| 4 | Grade history | Linked assignment · First Score · Current Score |
| Footer | Official grade and Bobcat Hour | Skyward link; Securly Flex link |

Proposed details: sort corrections by closing deadline and Due Soon by due date; omit empty sections; keep outstanding missing work visible even when older than 14 days. A one-line “Submitted—awaiting review” status may reassure parents without adding a task. The Due Soon horizon, history range, and inclusion of that reassurance remain open.

Grade history exists so families can see where a score was raised, by a curve or by points for corrections. Decided 2026-10-16: no explanatory sentence; the Skyward footer link covers the official grade.

The class-wide Band post is brief and names no student. It may point families to the emailed update and carry class-wide Due Soon or teacher-provided opportunities. Exact wording remains undecided.

## Correction eligibility and evidence

An assignment qualifies when student-visible, teacher-provided comments contain a rationale or exemplar, either per item or for the whole assignment. Identify the actual content; a generic feedback comment or an isolated keyword does not establish eligibility. Hidden answer keys are not parent-facing correction guidance.

The correction section applies the agreed 14-calendar-day window. Resolve the student's applicable due date and course timezone; do not infer a changed deadline from a saved extra-time flag alone. Exact cutoff time and inclusive boundary wording remain to be decided.

Retain the qualifying comment/item reference so the draft can accurately direct the family. Reading both comment locations is a handoff requirement. Unreadable or unsupported per-item feedback means eligibility is unverified, not that the assignment is ineligible.

Eligibility and sending are separate decisions: a qualifying assignment does not yet imply an email must be sent. No decision has been made about surfacing already-perfect work or overriding a send-suppression rule.

## Score and status accuracy

| Field | Meaning |
|---|---|
| First Score | First score Canvas recorded for the assignment (gradebook history) |
| Current Score | The assignment's current Canvas score |

Preserve score denominators and the associated assignment/attempt. Show unavailable history honestly; the earliest surviving comment is not automatically proof of the original assessment. With one assessment confirmed as both first and latest, both columns show it without implying corrections occurred.

Do not substitute Canvas's entered score for the original earned assessment or infer the cause of a score difference. Explain adjustments only when supported by evidence. New Quiz written-work comments may contain criterion scores without an aggregate quiz assessment; preserve that distinction.

A zero alone does not establish missing work. Distinguish Canvas-marked missing work, absent online submission records, excused work, submitted work, and corrections awaiting review. Paper and external-tool assignments need appropriate evidence. Include only released, parent-appropriate information; private teacher notes and credentials stay out.

## Conditional sending: open proposal

The user raised suppressing routine emails when the student has **90+ in class and no missing work**. Exact behavior has not been approved.

The following was proposed:

| Condition | Proposed result |
|---|---|
| Course grade below 90 | Send |
| Any missing work | Send |
| Course grade 90+ and no missing work | Skip |
| Grade or missing-work data incomplete | Flag for teacher review; do not assume the skip criteria are met |

Also proposed: corrections, Due Soon, and the Flex link enrich an otherwise warranted email rather than independently triggering one. This avoids routine deadlines defeating suppression. Whether an approaching correction deadline or important upcoming assignment overrides the skip remains open.

The source of the course-grade cutoff is unresolved. A current Canvas course average was suggested as a screening value; Skyward remains official. The Skyward link supplies no grade data and establishes no integration. Clarify how course-specific decisions work if one email combines multiple courses.

## Delivery

Decided 2026-10-16. Copy-paste delivery is ruled out.

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

### Delivery from Cowork and Work (MCP)

Outlook delivery is an Engine capability, not a separate connector. Cowork's thin extension and Work's direct registration both start UserInstall's `engine/cse.exe` on the teacher's Windows PC as the signed-in user. Outlook automation runs in that same process context, so whatever route reaches the Engine also reaches Outlook. The agent's own sandbox, such as Cowork's Linux VM, never touches Outlook.

The proposed shape follows the existing `start_review` / `review_status` pattern:

1. **Draft.** The agent reads current Canvas data and writes per-student update drafts and the class-wide Band text to `courses/` with `write_draft`. Drafts use student references, never addresses.
2. **Review.** A tool like `start_parent_review` opens a private local WebUI page and returns only its loopback URL. On the page, the Engine resolves recipients from `.cse/student-contacts.json` against Canvas instance and user ID. The teacher sees each family's actual To line and the email body, and can skip, edit or approve the batch.
3. **Send.** The teacher's Send button triggers the Engine directly, which creates and sends a fresh Outlook item for each recipient. The agent never calls send and never sees addresses, so this needs no MCP send tool or approval token. A "Drafts only" option on the same page fills Outlook Drafts instead.
4. **Outcome.** `review_status` returns counts and outcomes by student reference only: sent, skipped, failed or uncertain, with no addresses. Outcomes are recorded locally in UserInstall so a retry never resends an accepted message.

Band follows the same page, with no copy-paste. A "Post to Band" button uses the BAND API if it supports posting; otherwise, Cowork computer use posts the approved class-wide text. See [the handoff](../handoffs/active/PARENT-UPDATES.md), slice 4.

Engine notes:
- Do COM work on a dedicated OS-locked thread (`go-ole`, `CoInitialize`) so the MCP server's goroutines don't matter.
- Starting Outlook when it isn't running is acceptable.
- An Engine running at a different integrity level from Outlook, for example elevated, can fail to connect. Report that specifically rather than retrying.
- No PowerShell dependency, so constrained language mode doesn't block delivery.

Dependencies and limits:
- This rides on the general Cowork/Work MCP adoption still pending in [capability evidence](engine-capabilities.md). Outlook adds no new client requirement.
- Sending needs the PC on, the teacher signed in, and classic Outlook configured. A hosted tunnel route still sends from that PC. Scheduled or unattended sending inherits the same limits.
- Not yet verified: Go COM send, the district PC, and any MCP client calling this flow.

## Implementation notes (2026-10-16)

- `internal/outlook` (go-ole, one locked COM thread); `cse-probe outlook-self-test` sent a message to the profile's own address from Go on NEWSPEEDY. District PC, inbox arrival and MCP clients are not yet verified.
- `pull_parent_report` / `cse parent-report COURSE... [REF...]` writes `courses/<first course>/parent-updates/<run>/report.json`; the agent writes `drafts.json` beside it (contract in `playbooks/parent-updates.md`); `start_parent_review` / `cse parent-review RUN_FOLDER` opens the review page, which sends one fresh Outlook message per enabled parent address and records outcomes by student ref in `outcomes.json`. `start_student_report` / `cse student-report RUN_FOLDER` opens a print page from the same run with one student-voiced sheet per student.
- Not built from the handoff: a local CE-SE posting record for correction candidates (teacher comments are returned instead).
- **Band dropped (2026-10-16):** the teacher does not want Band in this flow. No Band post, card or `band_post` field exists; earlier Band notes above are historical.

## Architecture and current evidence

Develop this feature in **Dev**. Future teacher use and report drafts belong in **UserInstall**. Reusable preferences and private contacts follow configured **UserSync** authority; session drafts and delivery outcomes remain local to UserInstall.

- **Playbook/context:** eligibility judgments, sending policy, wording, priorities, report ranges, teacher instructions, and configured links.
- **Engine:** Canvas reads, exact student/course/attempt identity, dates and score arithmetic, evidence references, rendering, private recipient resolution, and eventual delivery outcomes.
- Reuse existing packages and plain files. A preliminary report needs no permanent gradebook, hosted parent portal, or general messaging framework.

Existing implementation starting points:

- [Canvas read types](../internal/workflow/canvas_reads.go) include assignments, submission statuses, comments, attempts, and score fields. Complete per-item feedback access and release checks still need investigation for this flow.
- [The scoring pull](../internal/workflow/pull_work.go) deliberately skips work without an actual submission. Its packets alone cannot provide the missing-work report; add a reporting read using the existing Canvas transport.
- [Feedback rendering](../internal/render/render.go) supports an ordinary-assignment assessment header, but teacher-customized feedback and missing history require honest unavailable values.
- [Private contact storage](../internal/workspace/contacts.go) already holds parent destinations and independent email/text preferences. Keep addresses and phone numbers out of ordinary agent context; resolve recipients privately against the correct student and Canvas instance.
- [Current capabilities](engine-capabilities.md) state that parent messaging is unimplemented. This plan establishes no new Canvas Live or delivery evidence.

Official API references: [submissions](https://developerdocs.instructure.com/services/canvas/resources/submissions) and [assignments](https://developerdocs.instructure.com/services/canvas/resources/assignments). Documentation supports investigation; it is not live verification of this feature.

## Next session and eventual handoff

Settle the choices needed for the first implementation slice:

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

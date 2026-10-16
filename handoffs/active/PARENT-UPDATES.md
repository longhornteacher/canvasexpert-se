# Handoff: Parent Updates (Canvas → reviewed Outlook email + Band post)

Created 2026-10-16. Direction and evidence: [docs/parent-update-flow.md](../../docs/parent-update-flow.md). Read it first, along with root `AGENTS.md`, `ARCHITECTURE.md` and `SCORING-RULES.md`.

## Goal

From Cowork, Work or Claude Code, the teacher says "send parent updates for CS8." The agent pulls current Canvas data and drafts compact, action-first emails. The teacher reviews them on a private WebUI page with the real recipients shown and clicks Send. The Engine then sends each email through the teacher's classic Outlook. A class-wide Band post, containing no student data, goes out from the same page. The teacher does no copy-paste, and the agent never sees parent addresses or performs the send itself.

## Ownership

- **Engine (`cmd/`, `internal/`):** reporting reads from Canvas; exact student/course/attempt identity; date windows in the course timezone; score provenance; draft validation and rendering; private recipient resolution from `.cse/student-contacts.json`; Outlook and Band delivery; per-recipient outcome records; MCP tools and the WebUI page.
- **Playbook/context (`workspace-template/`):** correction-eligibility judgment; send/skip decisions; wording; configured links (Skyward, Bobcat Hour); horizons and ranges; the "no invented work" rule.

Do not put eligibility or skip judgment in Go. The Engine supplies facts, and the agent decides.

## Decisions to confirm (proposed defaults)

Build with these defaults, keep the values in playbook/context where they are policy, and confirm them with the teacher before slice 3 merges.

| Item | Proposed default |
|---|---|
| Email grouping | One email per student per run, covering the teacher's selected courses in that run |
| Skip rule | Skip if every included course has a Canvas current score ≥ 90 **and** no missing work. Incomplete data → flag for the teacher, never skip silently. Corrections, Due Soon and links don't trigger an email on their own. |
| Grade source for skip | Canvas enrollment `current_score`. Skyward remains official and is only linked. |
| Correction window | Student's effective due date (course timezone) + 14 calendar days, through 11:59 pm on day 14 |
| Due Soon horizon | 7 days |
| Grade history range | Assignments due inside the 14-day window, plus any older assignment whose score changed in that window |
| Send mode | The teacher approves the batch on the page, and the Engine sends. "Fill Drafts only" is an alternative on the same page. |
| Body format | HTML (titles as links, compact table), with a plain-text alternative |
| Initiation | Manual: the teacher asks. No scheduling in this handoff. |
| Skyward URL | Teacher supplies it; stored in `context/parent-updates.md` |

## Slices

Each slice is independently mergeable. Slices 0, 1 and 5 can run in parallel; 2 depends on 1, 3 on 0 and 2, and 4 on 2.

### Slice 0: Outlook automation package (Engine)

- New `internal/outlook` (Windows build tag, with a stub on other platforms) using `github.com/go-ole/go-ole`.
- Run all COM work on one dedicated goroutine that calls `runtime.LockOSThread` and `CoInitializeEx`, and feed it requests over a channel.
- API: `Status() (available bool, reason string)`, `Send(msg) (Outcome, error)`, `SaveDraft(msg) error`. A message holds To (one or more), Subject, HTMLBody and TextBody.
- Always create a fresh `MailItem`. Never send a saved draft (2026-10-16: `Send()` fails with "inline response mail item" when the draft is open in the reading pane).
- Map known failures to plain reasons: classic Outlook not installed or new Outlook only; integrity-level mismatch (Engine elevated, Outlook not); object-model guard / security prompt refusal; no profile.
- Dev verification: a `cse-probe outlook-self-test` that sends one message to the profile's own address (`Session.CurrentUser.AddressEntry.GetExchangeUser().PrimarySmtpAddress`). It never takes an arbitrary recipient.
- Done when the probe sends from Go on NEWSPEEDY and reports a clear reason when Outlook is closed, absent or blocked.

### Slice 1: Parent report reads (Engine)

- New workflow read, for example `ParentReport(ctx, client, courseID, refs, now)`, reusing the existing Canvas transport and `canvas_reads.go` types. Don't extend `pull_work`, which deliberately skips work without a submission.
- Per student and course, produce:
  - **Course score:** add `grades.current_score` to `Enrollment` (the enrollments API returns a `grades` object for student enrollments; confirm on a live read) and include it with its source.
  - **Missing:**
    - Canvas `missing` true counts as missing.
    - Unsubmitted online work past its effective due date is reported as "not submitted (Canvas not marked missing)".
    - Excused work is excluded.
    - Paper/none/external-tool assignments are listed only when Canvas marks them missing.
  - **Correction candidates:** assignments whose effective due date is inside the window and that have student-visible teacher comments. Return the comment text by reference, plus any local CE-SE posting record for that assignment and attempt, so the agent can judge rationale or exemplar. Per-item New Quiz feedback that the Engine can't read is marked `unverified`, not ineligible.
  - **Due Soon:** unsubmitted and not excused, due within the horizon.
  - **Grade history:**
    - First and latest scored entries from `submission_history`, plus the current `score` and the denominator.
    - Mark `first_unavailable` when history can't establish it. Don't substitute `entered_score`.
    - Engine-posted score headers may corroborate.
  - **Links:** assignment `html_url`.
- Output: a pseudonymized JSON packet in `courses/<nickname>/parent-updates/<run-id>/report.json`, keyed by student ref and alias. No contacts and no Goes By name.
- MCP: `pull_parent_report` (read tool), mirroring `pull_work`'s inputs (course, optional student refs).
- Focused tests:
  - Window boundaries in the course timezone, including weekends.
  - Missing vs. unsubmitted vs. excused vs. submitted-awaiting-review.
  - Score provenance (history present, absent, single entry).
  - Course score unavailable produces a flag.

### Slice 2: Drafts and the private parent review page (Engine + WebUI + MCP)

- Draft contract, documented with the Engine format reference: one `drafts.json` per run, listing per-student `{ref, decision: send|skip|flag, reason, sections…}` plus one `band_post` text.
  - Sections carry assignment IDs. The Engine fills in titles, links, dates and scores from the report, so the agent can't misstate them.
  - The agent supplies the decision, ordering and short instruction text.
  - `{{student_name}}` expands via `ReplaceStudentName` at render time only.
- Validation: refs exist in the report; assignment IDs belong to that student's report; there's no free-text score that conflicts with the report; and the Band post contains no student ref, alias or name.
- WebUI page (pattern: `internal/localui/review.go`):
  - For each student: the decision, the rendered email, and the resolved To line from `student-contacts.json`. Use only `email_enabled` destinations and match on Canvas instance and user ID.
  - Show "no enabled email destination" as a visible held item.
  - Per-student skip/include toggle and an inline note back to the agent ("revise"), mirroring review notes.
- MCP:
  - `start_parent_review` returns only the loopback URL.
  - The existing `review_status` returns counts and outcomes by ref, never addresses.
  - Teacher revision notes return as they do for scoring review.
- Done when a synthetic run renders correctly in light and dark, the agent-facing outputs contain no addresses, and draft edits survive navigation (existing drafts.js).

### Slice 3: Outlook delivery and outcomes (Engine)

- The page's Send button calls slice 0 for each approved recipient. The agent has no send tool.
- Outcomes are stored locally in `courses/<nickname>/parent-updates/<run-id>/outcomes.json` by ref and destination index (no address), with states `sent | failed(reason) | uncertain`.
- A retry skips `sent` items. `uncertain` (the call errored after Outlook may have accepted it) is shown with "check Sent Items" and requires explicit re-send.
- A "Fill Drafts only" button uses `SaveDraft` and records `drafted`.
- A "Send a test to me" button sends the first selected email to the profile's own address. This also provides the Cowork/Work end-to-end check.
- `engine_info` reports Outlook availability as a boolean plus a reason, never an address.
- Update `internal/localui/capabilities.go` and `docs/engine-capabilities.md` together, with evidence kinds kept distinct.
- Done when a real send to the teacher's own address works from the page started via MCP in at least one client. Record which client.

### Slice 4: Band class-wide post (Engine)

- First confirm what the BAND Open API supports (developers.band.us): OAuth app registration, listing bands and creating posts. Record findings in the flow doc.
- If posting is supported:
  - Add a WebUI connect flow. The token goes to Windows Credential Manager, never to files or agent context.
  - Add a band picker (saved in context) and a "Post to Band" button on the review page with its outcome recorded.
- If posting is not supported: Cowork computer use posts the approved `band_post` text. The playbook covers this; the text contains no student data, so it's acceptable. No copy-paste flow.
- Done when the approved class-wide text posts to the chosen band from the page or through the documented computer-use step.

### Slice 5: Playbook and context (workspace-template)

- `workspace-template/playbooks/parent-updates.md`, covering:
  - The flow: select scope → `pull_parent_report` → decide send/skip/flag per student → write drafts → `start_parent_review` → `review_status` → one-line report.
  - The rules: no invented work, correction-eligibility judgment (actual rationale or exemplar content, not keywords), honest unavailable values, and the skip rule.
  - Score label explanation text and section order (Missing → Corrections → Due Soon → History → footer).
- `workspace-template/context/parent-updates.md`: Skyward URL, Bobcat Hour link, horizons, skip threshold, and the teacher's tone notes. Seed-only additions, never overwriting.
- Add routing from `workspace-template/AGENTS.md` / `TEACHER.md` to the new playbook.

## Guardrails (keep the cheap checks)

- Wrong recipient: resolve by Canvas instance + user ID only, show the To line to the teacher before sending, and never let the agent supply an address.
- Privacy:
  - No addresses, phones or Goes By names in MCP results or agent packets.
  - Teacher-only notes never appear in parent text.
  - Hidden answer keys are not correction guidance.
- Honest values: scores and dates come from the report, not agent prose; unavailable history shows as unavailable.
- Unsent work: drafts and outcomes persist; no double-send on retry.
- Follow AGENTS.md proportionality: focused tests on the boundaries listed above, and no general messaging framework or scheduling.

## Out of scope

Texting, scheduled or unattended sending, Skyward data integration, parent replies or two-way messaging, new Outlook / Graph, and personal Gmail.

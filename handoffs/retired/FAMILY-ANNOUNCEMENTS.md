# Family Contact: one flow for emailing families, many playbooks

Retired 2026-10-20: implementation, local checks and classic Outlook visual acceptance are complete in Dev. This document preserves the task and dated evidence. Current guidance is in [the Family Contact contract](../../docs/family-contact-flow.md) and the shipped playbooks. Unobserved cloud/client behavior remains evidence to collect during ordinary use, not unfinished implementation or a machine-specific prerequisite.

Status: implemented in Dev, 2026-10-20 · audit and local integration checks passed · local classic Outlook draft and visual inline-image/file checks passed; cloud/client observations remain unobserved

Implementation decision: extend `start_parent_review` / `parent-review` for either run kind; retain the existing private recipient routing, teacher actions, schema-2 outcomes and Outlook thread. New `prepare_family_contact` / `family-contact-prepare` prepares free-form `contact.json` runs; grade updates keep their existing report/draft contract and playbook. See [the audit and implementation contract](../../docs/family-contact-flow.md). General and book-club family-contact playbooks are added. No live Canvas requests or real-family sends were made. Integrated local check results are recorded below; unobserved cloud synchronization and installed-client behavior remain distinct from the completed local draft and visual checks. Working local classic Outlook with a configured profile is sufficient; there is no separate machine-specific gate.

## 1. Task and intended outcome

Make the existing parent-update machinery a general **Family Contact** flow. The grade update stays exactly as it is and becomes one playbook among several. The teacher and agent can also put basically anything into a family email: text, formatting, pictures, links, files.

The teacher should be able to say things like:

> "Email the families about book clubs. Each book group gets its own version with that book's cover, description and reading schedule."
> "Tell Period 3 families the field trip form is due Friday."
> "Send these four students' families a note that the makeup quiz is Thursday."

First real use: ELA 7 Unit 2 book clubs (`LANG ARTS 7 - Beckham` group set 5291, `PAP LANG ARTS 7 - Beckham` group set 5289, six groups each, The Tequila Worm group empty in both). Expected: five versions and one email per student in both courses, covers shown inline.

This is a development task. Audit first, then implement, test, update docs and agent guidance, and report.

## 2. Shape

**Shared back half (reuse, do not rebuild):** `start_parent_review` and the private review page, recipient routing, the Outlook path (one fresh item per student, locked COM thread), `outcomes.json`, `review_status`. Rename to "family contact" only where cheap; keep existing CLI/API/storage spellings compatible.

**Two ways to fill a run:**

1. **Grade update** (unchanged): `report.json` + `drafts.json`, complete `work_due` inventory, skip logic, honest values. `docs/parent-update-flow.md` guarantees are not loosened.
2. **Free-form contact** (new): the agent writes `contact.json` (and `assets/`) in a run folder; the Engine resolves audiences and renders. No grade data appears in this kind.

**Playbooks carry the judgment.** Keep `playbooks/parent-updates.md` as is. Add `playbooks/family-contact.md` (general: scope, voice, steps) and small task playbooks as they earn their keep (book clubs, reminders, field trips). A new kind of message should usually need only a playbook, not Engine work.

## 3. Audit first (before designing further)

Read the parent-update code path and answer, in the final report:

- Where the review page, preview rendering and send loop assume `report.json`/`drafts.json`, and how much change is needed for per-message previews and a free-form run.
- Whether `start_parent_review` can accept either run kind or a second start tool is simpler.
- Whether `outcomes.json`, idempotency and recipient routing can be reused unchanged.
- What the Outlook layer needs for inline images (CID) and file attachments.

Choose the smallest change that works and record the decision in the docs.

## 4. Hard boundaries (the only fixed rules)

- Real names, contact addresses and Canvas user IDs stay inside the Engine. Agent-facing files use `student_ref` and alias only.
- The agent never sees addresses and never sends. Sending happens only when the teacher clicks Send on the private loopback page.
- Recipients follow the parent-update rule: student email in To, every enabled parent email in CC, parents in To when no student email is on file. Optional `parents_only` leaves the student off.
- One email per student per run. A student matched by two messages is reported to the teacher, never silently resolved.
- A message never names other students. Only the reader's own `{{student_name}}` is substituted (`{{group_name}}` also available when the audience came from a group).
- An accepted send is never replayed; an uncertain send is held.
- Run folders and outcomes stay in UserInstall. UserInstall/UserSync authority rules are unchanged.

Everything else below is a default, not a requirement.

## 5. Free-form contact: suggested contract

Run folder `courses/<first course>/family-contact/<run_id>/`: `contact.json` and `assets/` (agent-written with `write_draft` / `write_asset`), `audience.json` and `outcomes.json` (Engine-written).

```json
{
  "title": "Unit 2 book clubs",
  "messages": [
    {
      "key": "ghost",
      "audience": {
        "groups": [
          {"course": "LANG ARTS 7 - Beckham", "group_set": 5291, "name": "Ghost"},
          {"course": "PAP LANG ARTS 7 - Beckham", "group_set": 5289, "name": "Ghost"}
        ]
      },
      "subject": "Book Clubs: {{student_name}} is in the Ghost group",
      "body": "Hello families,\n\n{{student_name}} is reading **Ghost**...\n\n![Ghost cover](assets/ghost.jpg)"
    }
  ]
}
```

- **Audience:** whole course(s), group(s) by name or id, or an explicit `student_ref` list. Group names match case-insensitively with whitespace ignored (live names differ, e.g. "Mexikid" vs "MexiKid").
- **Body:** Markdown (or HTML; scripts and remote loads are neutralized rather than the message rejected).
- **Images and files:** from `assets/`. Images show inline; other files attach. Only enforce a total size Outlook can deliver.
- **Remote images:** not fetched or embedded (Outlook blocks them; they leak opens). Tell the agent to save the file into `assets/`.
- **Closing:** `sign_off` from `context/parent-updates.md` unless the body already ends with one.
- Optional: `recipients: "parents_only"`. Add other fields only when a real request needs them.

### Tools

| Tool / command | Purpose |
|---|---|
| `prepare_family_contact` / `cse family-contact-prepare RUN_FOLDER` | Resolve audiences against live Canvas, write `audience.json` (`student_ref`, alias, course, group, message key), and return counts per message plus what the teacher should know: students matched twice, students in no message when a group set was used, empty groups, missing assets, unknown placeholders. Re-running refreshes the audience and keeps existing outcomes. |
| Review (extended `start_parent_review`, or a sibling if the audit says so) | One preview per message, recipient counts, per-student list with real To/CC visible only to the teacher. Per-student skip, Send all, Drafts only. |
| `review_status` | Unchanged: counts and outcomes by `student_ref` only. |

Sending refuses if the audience is older than `contact.json` or membership changed since preparation, and asks for a re-prepare. Only wrong-message-to-wrong-family problems (double match, stale audience) block sending; a student in no message is reported and skipped unless the teacher says otherwise.

## 6. Docs and guidance

Add the playbook(s). Update `AGENTS.md`, the engine reference (`formats`), `docs/parent-update-flow.md` (pointer to the general flow, grade guarantees untouched), `docs/engine-capabilities.md` and `internal/localui/capabilities.go` together. Keep Canvas Live, local-test and pending-human-validation evidence distinct.

## 7. Tests (proportionate)

- Audience resolution: name normalization, ids, whole course, explicit refs, empty group, double match, unmatched student.
- Two courses in one run: one email per student, no duplicates.
- Placeholder substitution; unknown token reported.
- Rendering: scripts and remote loads neutralized; remote image reported; size cap enforced.
- Outlook item carries inline images and attachments with matching `cid:` references.
- Stale audience and membership drift block sending.
- Outcome idempotency: accepted never resent, uncertain held, skipped recorded (reuse existing tests).
- Existing grade-update tests still pass unchanged.
- No real names, addresses or Canvas user IDs in `audience.json`, tool output or logs.

## 8. Acceptance

1. A synthetic fixture shaped like the book club scenario reports the right per-message counts and the empty group, with no blocking problems.
2. The review page shows each message preview (covers inline) and one row per student with To/CC.
3. "Drafts only" through local classic Outlook with a configured profile produces drafts whose images render inline. No computer-name restriction applies. Record saved-item and visual evidence separately; working local Outlook connectivity is the machine requirement.
4. Full test suite, vet, and the compiled MCP smoke (tool count updated).

## 9. Out of scope

Scheduling, replies/threading, Canvas Inbox or Announcements, texting, and any live send to real families during development. Grade data inside free-form contact emails; a later "message plus grade report" is a separate decision.

## 10. Open teacher choices (defaults in bold)

- Student included as recipient: **yes (same as grade reports)**, or parents only.
- Students not matched by any message in a group-based run: **skip and report**.

## 11. Implementation and local evidence, 2026-10-20

- Audit: `LoadParentRun`/grade rendering remain the grade-only branch. Review construction and sending dispatch on `contact.json`; `ParentReviewSource` selects the correct saved review state. Shared message previews sit above the existing private student cards. No second review-start tool or session system was needed.
- The shared send loop and schema-2 outcomes serve both kinds. Recipient lookup remains private, with optional parents-only routing. Skips are recorded; a run lock prevents simultaneous delivery, and an uncertain checkpoint is saved before Outlook so acceptance followed by interrupted persistence cannot replay a send. Drafts-only retries do not duplicate accepted drafts.
- Outlook now receives ordinary file attachments and inline attachments with matching CID, MIME and hidden-image properties on the existing locked COM thread. Preview images use local data URLs; ordinary attachment filenames are shown. A digest includes all attachment bytes to reject approval from a stale page.
- Synthetic book-club fixture: two courses, five message versions, eleven unique students (including one student in both courses), two empty Tequila Worm groups, and one unmatched student reported. No blocking problems for the intended fixture. Duplicate messages, stale/touched content, changed membership, snapshot edits, wrong-course groups, missing contacts, parents-only routing and private-response error handling have focused checks.
- Rendering and delivery fixtures cover placeholder safety, scripts/remote loads, missing assets, the 20 MiB encoded-size estimate, Windows junction escapes, image/attachment agreement, skipped outcomes, draft/send idempotency, uncertain holds and accepted-send outcome-save failure. Existing grade-update tests were not edited.
- Passed: `go test -count=1 ./...`, `go vet ./...`, a `-trimpath` scratch Engine build, compiled `scripts/check-mcp.mjs` smoke with **40 tools**, and `git diff --check`. A separate local UI test confirms ordinary attachment filenames and stale attachment refusal. The cached full-suite invocation was stopped after test processes exited and its cache finalization stalled; the uncached full suite completed successfully.
- Synthetic browser check: the review displayed message and recipient sections at the available mobile-width viewport, the inline PNG loaded under the page's CSP, and Skip changed the approved count to zero. This establishes browser preview behavior, not Outlook rendering.
- Local classic Outlook draft check passed on 2026-10-20: Engine SaveDraft created a synthetic item to the configured profile, and saved-item inspection verified matching CID HTML and image/png MIME. A follow-up actual classic Outlook inspection with a visible 240×320 blue-book/yellow-background image confirmed that image inline and schedule.txt in the attachment bar. The unnecessary PR_ATTACH_FLAGS write rejected by COM (0x80020009) was removed; CID, MIME and hidden-image properties remain. No Send was called and no real-family data was used. Local visual inline-image rendering and ordinary attachment visibility are verified; acceptance 3 is complete for local classic Outlook. Cloud synchronization, other clients and inbox delivery were not observed.
- Correction: the earlier hostname restriction belonged only to the scratch probe and was mistaken. The Engine had no hostname guard. Working local classic Outlook with a configured profile is sufficient on any computer; report actual connectivity restrictions if encountered, with no separate district-PC prerequisite.

- The first white 1×1 PNG established attachment metadata only. The visible-image follow-up exposed an Engine bug: passing Position=0 to Outlook Attachments.Add hid the ordinary attachment. Omitting Position fixes its visibility; CID images are explicitly hidden. Outlook normalizes the image hidden flag after opening, so permanent flag state is not claimed. Actual Outlook screenshot and accessibility inspection confirmed the blue-book image and visible schedule.txt on the synthetic draft titled 'CE-SE inline image and visible file check 2026-10-20'. The private screenshot stays in ignored local tooling output; no Send was called.

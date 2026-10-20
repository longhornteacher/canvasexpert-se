# Family Contact

Implementation contract, reviewed 2026-10-20. Family Contact extends the existing parent-update review and Outlook delivery path. Grade-update behavior remains governed by [parent-update-flow.md](parent-update-flow.md), including its complete due-based inventory, current-period scope, grade privacy and skip rules.

## Audit and choice

The existing `LoadParentRun` reads `report.json` and `drafts.json`; `BuildParentReview` renders a card per student and privately resolves To/CC. `SendParentUpdates` loads that same run and sends one fresh Outlook item per student. The CLI supplied `drafts.json` as the review source. These assumptions need dispatch at run loading, review construction, rendering and sending, rather than another session or delivery system.

`start_parent_review` and `cse parent-review RUN_FOLDER` accept both run types. `ParentReviewSource` selects `contact.json` for a free-form run and `drafts.json` for a grade update. The existing loopback review lifecycle, teacher selections, Drafts only action and `review_status` remain shared. A separate review-start tool would duplicate the same private page lifecycle without providing a distinct teacher action.

`outcomes.json` reuses schema 2 and its per-`student_ref` delivery records. Accepted sends are never replayed; uncertain sends hold for teacher resolution. Preparation never deletes outcomes. Recipient routing reuses the existing private contact lookup, adding only the explicit `parents_only` choice. Student identity resolves from the configured Canvas instance and stable ref inside the Engine; real names, addresses and Canvas user IDs never appear in the agent-facing audience or outcomes.

The Outlook layer still creates a fresh item per student on its locked COM thread. Inline images need attachment metadata (`Content-ID`, MIME type and hidden flag) paired with `cid:` image sources in the HTML body. Ordinary files use regular Outlook attachments. The rendered message carries both kinds to Outlook; the review uses local image bytes without fetching remote sources. A local fake can check that contract, but only classic Outlook observation can establish actual image display or delivery.

## Free-form run

Keep runs under `UserInstall/courses/<first course>/family-contact/<run_id>/`. The agent writes `contact.json` and `assets/`; the Engine writes `audience.json` and the shared `outcomes.json`. Runs stay local even when reusable teacher setup uses UserSync. A free-form run contains no grade report or grade fields.

```json
{
  "title": "Book clubs",
  "messages": [
    {
      "key": "ghost",
      "audience": {
        "groups": [
          {"course": "ELA7", "group_set": 501, "name": "Ghost"},
          {"course": "ELA7H", "group_set": 502, "id": 702}
        ]
      },
      "subject": "Book clubs: {{student_name}}",
      "body": "Hello families,\n\n{{student_name}} is reading **Ghost** with the {{group_name}} group.\n\n![Cover](assets/ghost.jpg)\n\nOur first reading checkpoint is Thursday.",
      "attachments": ["assets/reading-schedule.pdf"]
    }
  ]
}
```

`title` and `messages` are required. Each message needs a unique `key`, `audience`, `subject` and `body`. `recipients: "parents_only"` optionally excludes the student; otherwise the student is To and all enabled parent emails are CC, with parents in To when no student email is saved. A student with no enabled parent email is held individually. The Engine privately resolves names only for `{{student_name}}`; `{{group_name}}` is available for a group-selected student. Unknown placeholders are reported. Do not name other students in any message.

Audience selectors are additive: `courses` is a list of configured course nicknames, `groups` selects exact collaborative sets and groups, and `student_refs` selects saved stable refs. A group selector contains `course`, numeric `group_set`, and either numeric `id` or `name`. Group names match ignoring case and whitespace. A student must resolve to one message per run, including across courses. A duplicate match blocks sending rather than choosing a message. Empty groups and students in the selected set who match no message are reported; unmatched students are skipped, not silently added to a message.

Body `format` defaults to Markdown; `"format": "html"` permits authored HTML. Sanitization removes active content and remote loads while retaining ordinary formatting and links. Remote images are reported and never downloaded. Save intended images under the run's `assets/` and reference them there; use `attachments` for local files. Asset paths cannot escape `assets/`; the complete message is limited to a conservative 20 MiB estimate including base64 attachment overhead. The saved `sign_off` in active `context/parent-updates.md` is appended unless already present at the end.

## Prepare, review, deliver

1. `prepare_family_contact {folder}` / `cse family-contact-prepare RUN_FOLDER` resolves current Canvas audiences, writes a pseudonymized `audience.json`, and returns message counts, audience rows, warnings and blocking problems. Re-run after editing contact content or fixing an audience. Changed assets require a fresh review. Existing outcomes remain intact.
2. `start_parent_review {folder}` / `cse parent-review RUN_FOLDER` opens the private loopback page. The teacher sees message previews, one row per student and actual To/CC, then chooses skips, Send approved emails, or Fill Outlook Drafts only. The agent presents the URL and never reads this private page. Starting a review does not send.
3. `review_status` returns only the existing session outcome and pseudonymized delivery details. An accepted send is finished; an uncertain send is held. A changed contact digest or current membership compared with preparation holds sending and asks for a fresh preparation and review. The agent cannot send through a CLI or MCP operation.

Family Contact has no automated self-test send. Drafts only is the route for an authorized local Outlook inspection. There is no scheduling, Canvas Inbox/Announcements write, texting, reply handling or grade modification in this flow.

## Evidence and remaining checks

The implementation uses local synthetic audience, rendering, Outlook-message and delivery fixtures. Those establish only Engine behavior; see the integrated check results in the completed [handoff](../handoffs/retired/FAMILY-ANNOUNCEMENTS.md). No Canvas Live membership read or real-family send is authorized by this development task. Family Contact requires local classic Outlook with a configured profile; there is no computer-name restriction. The local classic Outlook draft check passed on 2026-10-20: Engine SaveDraft created a synthetic item addressed to the configured profile, and saved-item inspection confirmed matching CID HTML and image/png MIME. A follow-up actual classic Outlook inspection showed a visible blue-book image inline and schedule.txt in the attachment bar. No Send was called and no real-family data was used. Local Outlook inline-image appearance and ordinary attachment visibility are verified. Cloud synchronization, other clients, inbox delivery and installed MCP client adoption remain unobserved evidence. Pending evidence does not prohibit ordinary supported use within teacher-authorized scope.

Delivery serializes a run with `.delivery.lock` and saves an `uncertain` hold before invoking Outlook, so an interrupted process or outcome-save failure cannot turn acceptance into a blind replay. Outcomes also record teacher-skipped students. If an interrupted process leaves the lock, inspect Outlook and the saved outcomes before manually removing it; do not remove it while delivery is running. Drafts-only outcomes avoid duplicate drafts; choosing Send later creates a fresh email, so the teacher should manage any retained Outlook draft.

The Engine uses whichever local classic Outlook profile is configured. The earlier computer-name restriction existed only in a scratch probe and was mistaken; the Engine had no hostname guard. The successful check followed removal of an unnecessary PR_ATTACH_FLAGS write rejected by local COM (0x80020009). Visual inspection then exposed that Position=0 on attachment creation hid the ordinary file; the Engine now omits Position and explicitly marks CID images as hidden. Outlook may normalize that hidden flag after opening, so saved flag state alone does not establish appearance. CID/MIME and actual inline-image/file display are verified. Other computers need working local Outlook connectivity and a configured profile, with any actual restrictions reported when encountered. No separate district-PC acceptance gate is imposed.

Local checks passed on 2026-10-20: the uncached full Go suite (including unchanged grade-update tests), vet, a compiled MCP smoke discovering 40 tools, and a synthetic browser review with a loaded inline image and working per-student Skip. The two-course fixture produces five versions and eleven unique recipients, reports both empty groups and the unmatched student, and has no blocking problems. Review also displays ordinary attachment filenames and binds their bytes to its approval digest. The synthetic results do not establish Canvas Live membership; actual local Outlook image and file display were checked separately as recorded above.

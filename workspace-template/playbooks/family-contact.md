# Family contact

Use this for emails about class activities, reminders, field trips or selected students. For grade updates and printable student reports, use `parent-updates.md` unchanged. For book-club messages, also read `family-contact-book-clubs.md`.

Read `TEACHER.md`, `Teacher_Override.md`, relevant teaching context and `context/parent-updates.md`. Speak in plain classroom terms. The teacher chooses the audience and purpose; draft the requested message using known coursework and dates. Do not invent parent tasks, promises or student facts. Keep grade details out of this flow.

## Prepare the message

Use the Engine's `engine_reference` formats for the Family Contact contract. Save the draft in a new `courses/<first course>/family-contact/<run>/` folder in UserInstall using `write_draft`; keep images and files in its `assets/` with `write_asset`. Keep the run and saved outcomes in UserInstall even when teacher preferences use UserSync.

Select configured courses, exact collaborative groups or stable student references. Use the Engine's group reads for book-club names and membership; never reconstruct private identities. Choose one version per student. If the teacher wants parents alone, set `recipients: "parents_only"`; otherwise students receive To and enabled parent emails receive CC. The Engine handles addresses privately.

Write one clear subject and body per version. Markdown supports everyday formatting; use HTML only when needed. Use `{{student_name}}` for the reader's own student and `{{group_name}}` for a group-selected audience. Never name other students or include teacher-only notes. Use local covers and files, ordinary links and the saved closing. Remote images are not embedded; save intended images under `assets/`. Messages allow a conservative 20 MiB total estimate including image/file encoding overhead.

Run `prepare_family_contact {folder}` (CLI `family-contact-prepare RUN_FOLDER`). Explain empty groups, unmatched students, missing assets or unknown placeholders, and resolve double matches before review. A student outside every version is reported and skipped; do not broaden the audience to hide the warning. Preparation reads current Canvas membership and sends nothing. Preserve outcomes when refreshing a draft.

## Let the teacher review and send

If the teacher asked only for a draft, save it and stop. Otherwise open `start_parent_review {folder}` (CLI `parent-review RUN_FOLDER`), present the private page link, and wait for the teacher's action. The teacher sees message previews and actual To/CC, can skip students, and chooses Send approved emails or Fill Outlook Drafts only. Never fetch or inspect this private page, use computer control to send, or look up contact files. Sending belongs exclusively to the teacher's button on this page, including when the teacher says to skip ordinary review.

Check `review_status` afterward and report successes, skips, individual holds, failures or uncertainty plainly. An accepted send is complete and is never replayed. If the Engine holds stale content or changed membership, prepare again and reopen review. If Outlook may have sent a message, ask the teacher to inspect Sent Items; never erase that outcome or retry it blindly. Keep independent students moving when one is held.

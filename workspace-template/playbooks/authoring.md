# Create a new assignment

What the teacher's words mean and the review page are in `AGENTS.md`.

Before designing content, read `context/teaching-profile.md`, relevant teacher reference notes and `context/grading.md` through logical MCP paths or the effective teacher-rules root. Use saved grade/subject, priorities, standards and feedback guidance; ask only for missing details needed for this task. Preserve teacher choices and never use developer class names or differentiation tags as colleague defaults.

## Standards

Apply `TEACHER.md`: 100 points by default, grade-level rigor, a one-line Learning Target and three-step directions at the top, and a grade-representative exemplar for writing assignments, and a "Corrections available" line. Once the content is shaped, offer tiered Hub support (below) once unless saved choices settle it.

## Write the draft

Ask only for missing details that matter: course, title, directions, points, due and availability dates, submission type, module, publication, rubric. `.\cse.cmd courses` lists course nicknames and `.\cse.cmd modules <course>` lists module targets.

Write a draft in the course's `drafts/` folder using the fields in `help formats`. Put the teaching content in the body. The default Markdown renderer handles headings, paragraphs, lists, bold, italic, links and fenced code; raw HTML shows as escaped text. Use `body_format: html` explicitly for authored tables, images and Canvas file links. The local review shows HTML as escaped source; verify its saved appearance in Canvas. For standalone Pages, asset uploads and module assembly, read `native-content.md`.

## Collaborative groups and book clubs

Use `cse groups <course>` (or MCP `list_group_sets`) to inspect collaborative group sets. `cse groups <course> --set <ID>` lists its clubs, stable-ref/alias memberships, unassigned students, pending requests, and roster completeness; `--members <GROUP_ID>` reads one exact club. No student names, Canvas user IDs, logins, SIS IDs or contacts are returned. Differentiation tags are intentionally excluded. Use `pull-group-set <course> <set-ID>` to create an editable exact-ID baseline, then make only the requested name/settings changes or add keyed clubs and `push` the document. Omitted clubs/settings are preserved. The Engine checks course, group-set and club identity before writes and records created IDs so a partly completed setup can resume.

For membership plans, use `pull-group-memberships <course> <set-ID>` to save the exact membership baseline. Make only explicit changes by `student_ref`, retaining each `from_group_id` from the baseline; `null` means observed unassigned, while `to_group_id: null` requests a removal. Do not infer removals from omitted rows. A move can change access to group work, so check for submitted work; list only the specific `student_ref` values the teacher authorized in `authorized_submitted_work_change_refs`. Use `operation: allocate-unassigned` only when the teacher asks for even allocation; it respects students already assigned, uses Canvas's synchronous allocation and reports the refs actually assigned. If a new unassigned student appears after the baseline, refresh it first. Self sign-up is controlled by the group-set settings. Book preference matching, balanced groups and role assignment remain teacher/agent decisions.

For an ungraded discussion use `kind: discussion`: omit both group IDs for a course topic, use `group_category_id` for a common prompt with separate club conversations, or use `group_id` for one direct club topic. Pull the exact topic with `pull-discussion` before editing. A pulled baseline protects requested title/body replacements while allowing settings-only changes to preserve newer live content. Use `is_announcement: true` only when the teacher explicitly asks to communicate an announcement; record the requested audience and intent in `announcement_intent`. Slice 05a rejects a discussion with an `assignment` block because linked graded discussions are not yet supported.

## Review and create

Run `.\cse.cmd review <draft-file>`. The page shows the content as the Engine will write it (it doesn't simulate the student view). When the teacher clicks Create, the Engine creates the assignment. With review skipped, run `.\cse.cmd push <draft-file>`.

A new assignment stays unpublished until its requested rubric and module pieces succeed. If `push` reports partial or ambiguous results, resolve that specific piece and rerun the same file; it resumes from saved IDs. Never pick a same-titled Canvas assignment arbitrarily or start a second create when the first may have succeeded. Report the resulting Canvas state and anything incomplete.

## Differentiated hub (tier support pages)

Use the hub when the teacher wants tiered supports around one whole-class assignment. It is the only differentiation style (no separate tier assignments, no Bridge). Add `tiers` to a create draft: each has `label`, the teacher's exact Canvas differentiation `tag`, and `supports` (Markdown for that tier's page). Ask for actual tags if they are not in this teacher's context; never assume another teacher's tags. Put shared directions and hints for everyone in the assignment body, and link each tier exactly once with `[text]({{tier:LABEL}})`. Tier supports scaffold the same standard; they don't lower it.

`push` creates the restricted pages first, then the assignment with verified links. Report each page, its tag, and every teacher action it prints (for example, a missing tag: that page is restricted to nobody until the teacher sets up the tag). Pages publish only when the draft says `published: true`; if the teacher publishes later in Canvas, the pages stay unpublished, so say so. Tiers can't be added to an existing assignment, and a page title that already exists stops the push before anything is created. Students in other tiers see every link but get Canvas's access-denied page for the others. The Engine verified the restrictions through Canvas, not from a student's view.

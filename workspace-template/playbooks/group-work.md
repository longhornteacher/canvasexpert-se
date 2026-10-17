# Book clubs and group work

Use this playbook when the teacher is planning or supporting a book club, collaborative group, or discussion-based unit. Read `TEACHER.md`, `Teacher_Override.md`, this teacher's `context/teaching-profile.md` and `context/grading.md`, `context/courses.md`, and relevant student/course references through the active teacher-rules location first. Keep unit plans, drafts, discussion pulls and outcomes in UserInstall. Never read `.cse/` or put private student notes, accommodation details, contact data or grouping rationales into club materials.

This playbook covers the group features currently shipped: collaborative group sets and clubs, memberships, member-editable group Pages, ungraded course/distributed/direct-group discussions and explicit announcements, written discussion evidence and teacher replies, course modules with group links, and metadata reads for existing collaborations/conferences. Group assignments and group grading, graded discussions, provider document/meeting creation, peer review, group calendars, conference bookings, group files and Inbox messages are not supported here. Do not promise or simulate those paths. Existing individual assignments and New Quizzes continue to use their own playbooks.

## Plan the unit

Ask only for consequential choices that saved context and the request do not settle: course, books, how students choose or are assigned clubs, group constraints, dates/read schedule, learning targets and deliverables, and whether a requested communication should be sent. Book titles do not uniquely identify clubs; several clubs may read the same book. Reuse an existing club only after checking its exact group-set and group IDs and confirming it matches the teacher's intent.

Keep a regular local `unit.md` with the learning targets, books, group-set ID, club IDs and labels, reading/meeting schedule, teacher-chosen rotating roles, individual and shared deliverables, requested grading approach, communication choices, and links/paths plus returned Canvas IDs and outcomes for the real drafts and pulls. It is a working note for this unit, not a new CE-SE document kind or synchronization file. Describe a group grade or graded discussion request as unsupported in the current release; offer an individual assignment or an ungraded discussion if that fits the teacher's goal. Never imply that a shared Page has a shared grade.

Roles such as facilitator, evidence finder, connector and summarizer are directions for student learning. They do not grant Canvas group leadership or moderator permissions. Do not put private tier tags, accommodations, student notes or group-balancing reasons in club names, rosters, role sheets or student-facing content.

## Establish clubs and memberships

Use the current configured course and inspect collaborative group sets with `cse groups COURSE` (MCP: `list_group_sets`). Inspect a chosen set with `cse groups COURSE --set SET_ID`; use `--members GROUP_ID` only for the exact club being discussed. These views use stable student references and aliases and omit student names, Canvas IDs, logins, SIS IDs, contacts and differentiation tags.

To create or adjust a set and its clubs, use `cse pull-group-set COURSE SET_ID` for an existing set. For a new set, make a regular draft with `kind: group-set`, `operation: create`, the configured course nickname, and the teacher's requested club names/settings. Edit only what the teacher requested and run `cse push FILE`. Existing or omitted clubs and settings are preserved; save returned IDs in `unit.md` so later materials use exact identities. Do not remove a club that contains student work as routine cleanup.

For membership changes, first use `pull-group-memberships COURSE SET_ID` and work from that exact baseline. Make only explicit changes by `student_ref`; retain each observed `from_group_id`. A row with `from_group_id: null` is an observed unassigned student; `to_group_id: null` explicitly removes a member. Omitted students are not removal requests. Check for submitted work before a move/removal and include only the specific teacher-authorized refs in `authorized_submitted_work_change_refs`. Generic requests to balance clubs do not authorize moving a student away from submitted work. For even assignment of unassigned students, use `operation: allocate-unassigned` only when the teacher requests it; it uses Canvas's set-wide allocation and reports the refs actually assigned. If the roster changes after review, refresh the baseline. Do not replay an uncertain allocation.

Self sign-up is a group-set setting. Explain the actual choice to students through teacher-authored directions; do not promise a sign-up UI or membership until Canvas access is observed. Book matching, group balance and membership decisions remain teacher judgments. Continue independent rows when a particular move is held and report that exact row.

## Build club spaces and conversations

Create a course unit hub or other course Page using `native-content.md`. For each club, create a group Page with the exact course and `group_id`: a reading plan, evidence notebook or meeting log can be member-editable with `editing_roles: [members]`. Group Pages cannot be placed directly in course modules. Keep a simple starter that invites student writing, and never push that starter again over a populated notebook. Pull before editing; if the body conflicts, preserve the unsent draft and merge only the requested change into a fresh pull. A title-only edit preserves the current student body. `cse page-history COURSE PAGE_ID --group GROUP_ID` reads revision history, but revisions do not prove who contributed or capture every simultaneous edit.

Author discussions only within the supported ungraded paths:

- Course topic: no group IDs.
- One shared prompt with separate club conversations: use `group_category_id` for the collaborative set.
- One topic directly in a club: use `group_id` for that exact group.

Use `kind: discussion` and the supported `push_document` path. Pull the exact topic before editing. A pulled baseline protects requested title/body replacement while settings-only changes can preserve newer live content. Keep discussion topics unpublished or scheduled as requested. `is_announcement: true` is a separate communication choice: create an announcement only when the teacher explicitly asks, and record its intended audience and purpose in `announcement_intent`. A request to assemble a unit or create discussion activities does not authorize an announcement. Linked graded discussions are not supported.

To inspect written discussion, run `pull-discussion COURSE TOPIC_ID [--group GROUP_ID]` (MCP: `pull_discussion`). Read both `topic.md` and the sibling `discussion-packet.yaml`. A distributed topic includes only verified child topics in the selected set; a direct group pull reads that club. Check every source's complete/partial status and as-of time before drawing conclusions. The Engine maps authors only through existing private student references for the same Canvas origin. Unmapped authors remain unknown; do not ask for or read a broad roster to fill them in. Deleted posts, inaccessible club topics, missing replies and unreadable attachments are incomplete or unknown evidence, not proof of no participation. The packet records written Canvas interaction only; it does not show in-person talk or edits in outside documents.

Summaries should point to the packet/topic links and as-of time, describe observed claims, evidence and questions worth revisiting, and offer next instructional moves as suggestions. Say when coverage is partial. Describe missing preparation only when the relevant source is complete. Do not grade from post counts or labels, call an inaccessible student disengaged, infer Google document authorship from timestamps, or claim to hear an in-person conversation.

Post a teacher entry or reply only when the teacher explicitly asks to communicate. Use `kind: discussion-reply`, exact topic context and, for a reply, the intended `parent_entry_id`. Put only publishable wording in the body; keep private analysis in `teacher_note`. For a group topic, `group_id` must match its group context. If the outcome is pending or uncertain, do not post again or search by similar wording. An accepted reply is recorded and is not read back for confirmation.

## Connect the unit in Canvas

Use `native-content.md` to assemble a course module after its children exist. A course discussion uses the native Discussion item and its verified parent topic ID. Link a group Page or direct group discussion with an explicitly titled `ExternalUrl` containing its exact Canvas URL; use the native course Discussion link for a distributed parent. External URLs must be stable HTTP(S) links without embedded credentials or signed temporary parameters. A module link and module publication do not publish the child or add students to the club. Course visibility does not prove a student can open the group resource.

`cse external-resources COURSE [--group GROUP_ID]` (MCP: `list_external_resources`) can discover existing collaboration and conference metadata and safe returned links. It does not create Google/Microsoft documents or meetings, read provider content or recordings, or prove contribution or attendance. Use teacher-selected provider links; if a usable link is unavailable, ask the teacher for one or leave that link out.

## Resume and report

Resume from saved Canvas IDs and operation outcomes in the actual documents. Preserve returned IDs, student-authored Page content, unsent drafts and partial discussion packets. A missing club page or inaccessible discussion child should not stop independent clubs; name the specific incomplete piece. Do not send announcements, Inbox messages, or teacher replies merely to say setup is complete.

Report the course and relevant Canvas links, what was created or read, publication and membership state, exact partial/held items, and what the teacher or students still need to do in Canvas. Use local checks and API responses honestly: neither proves student access or rendering. Routine supported teaching does not require a disposable-course trial. Do not describe pending student-view evidence as proof that the workflow is unusable.

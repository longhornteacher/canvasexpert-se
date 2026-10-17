# 06 — Conversation evidence and teacher participation

Requires: [01](01-groups.md), [05](05-discussion-authoring.md) contracts. Existing Canvas discussions can be pulled without first authoring them in CE-SE. Result: the agent can read an entire club conversation, identify who said what, and prepare an explicitly requested teacher reply.

## Read contract

`pull-discussion COURSE TOPIC_ID [--group GROUP_ID]` / `pull_discussion` saves an editable `topic.md` plus `kind: discussion-packet` in a new local pull folder. A distributed parent expands only its verified child topics within the selected set/course; a direct group request reads only that club.

The packet includes origin/course, parent topic ID, actual source context/topic IDs, linked assignment ID when any, pull timestamp, complete/partial status per child, and normalized entries. Entries carry `entry_id`, parent reference, author kind, stable `student_ref`/alias when resolvable, created/updated timestamps, exact HTML/readable text, deleted state, and attachment status. Map student authors only through existing private student references for the same Canvas origin; never read the whole course roster to enrich a packet. Unmapped authors stay unknown. Identify a source entry by context + topic + entry ID, not by entry ID alone. Teacher-only analysis is stored separately from publishable text.

Use the complete threaded view with `include_new_entries=1`; merge by entry identity, with newly returned versions replacing older cached versions. If that view is unavailable, one bounded retry then paginated entries/replies is sufficient. Top-level entries embed only a limited recent-reply set; do not mistake that subset for a full conversation. Label unavailable coverage explicitly. Never mark entries read, post an artificial entry to bypass a restriction, or auto-subscribe the teacher. [Discussion read API](https://developerdocs.instructure.com/services/canvas/resources/discussion_topics).

## Evidence behavior

Do not infer no contribution from inaccessible children or author-mapping failures. Preserve deleted markers without reconstructing erased text. A student no longer in a club may still have authored visible work; retain actual attribution separately from current membership. Unknown participants remain unknown and do not get another student's identity. Download evidence attachments through existing safe paths; unsupported media remains a linked limitation.

Use existing long-pull MCP handling. Partial access to one child returns a specific reason and permits independent groups to be read. Avoid a permanent participation cache; each packet is an as-of observation in UserInstall.

## Teacher replies

Add `kind: discussion-reply`, `operation: create`, exact topic context, optional `parent_entry_id`, and a Markdown/HTML body. `push_document` submits a new teacher entry or reply only when the teacher requested that communication. The parent must belong to the target topic. Save returned entry ID/outcome before reporting. A timeout holds as uncertain; text similarity is not permission to post again. No editing/deleting student posts, impersonating a student, or automatically posting a generated summary.

## Implementation and acceptance

Add `discussion_pull.go` and typed discussion packet/reply domain contracts. Slice 07 consumes these packets; do not cram a full conversation into an ordinary student's submission body.

- A fixture with more replies than the embedded preview returns every reply once, with nesting and timestamps preserved; cached and newer entries merge by source identity.
- One inaccessible child marks only its own coverage incomplete and emits no nonparticipation claim; former members, instructors, unresolved authors, deleted posts and unreadable attachments stay honestly represented.
- Pull performs no writes; a wrong-course child or cross-topic reply is refused.
- A requested teacher reply targets the intended parent and is not replayed after an accepted success or ambiguous timeout.

Add synthetic threaded examples and playbook directions distinguishing observed written interaction from oral participation.

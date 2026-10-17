# 03 — Shared group Pages

Requires: [01](01-groups.md). Result: create a club reading plan, member-editable evidence notebook, and meeting log; edit them while preserving student contributions.

## Surface and document

Extend the existing Page document with optional `group_id` and presence-aware `editing_roles`. Absence of `group_id` preserves all existing course behavior. A group Page requires an exact group ID, cannot carry course module placement, and permits `editing_roles: [teachers]` or `[members]` (or their supported combination). Never select public editing by default.

```yaml
schema: 1
kind: page
operation: create
course: ELA7
group_id: 701
title: Evidence notebook
body_format: html
editing_roles: [members]
published: true
```

`pull-page COURSE PAGE_ID|LINK --group GROUP_ID` / extended `pull_page {course, query, group_id?}` returns exact live HTML, editing roles, group identity and baseline. Preserve the existing MCP `query` field; do not rename it to `page`. Recognize the actual `/groups/:id/pages/:slug` route; never rewrite it into a course Page URL. Add read-only `page-history COURSE PAGE_ID --group ID` / `list_page_revisions {course, page_id, group_id?}` with optional revision selection for the body. No restore/revert write is required.

## Implementation

Extend `internal/domain/page.go` and `internal/workflow/page.go` directly. A small helper may select the course or group Page base route after verifying the course/group relationship. Do not build a universal resource-context abstraction. Include group identity in created outcomes and baseline comparisons.

Existing Page semantics remain: starts unpublished during creation, saves exact returned ID/slug, applies requested publication, and sends only changed update fields. For member-editable Pages, a teacher edit drafted before student changes must hold on a body conflict and keep the unsent draft. Re-pull and merge only the requested change into current content. Never regenerate a populated notebook from its starter template. Reading revision history must not claim to measure every member's contribution or simultaneous editing.

Canvas group Pages and `members` editing are documented in the [Pages API](https://developerdocs.instructure.com/services/canvas/resources/pages). Student access and editor behavior remain pending actual observation.

## Acceptance

- A group create uses only group Page routes; an old course-only specimen behaves unchanged.
- A group from another course, module placement on a group Page, and a course/group baseline mismatch fail before write.
- After a member edit, a replacing body patch is held and the draft kept; a title-only patch preserves the new body. Resume never overwrites a populated notebook.
- Offline validation and MCP argument handling accept the new shape and reject incompatible scope fields.

Extend native-content guidance with a notebook example and the student-work preservation rule. Use slice 16 to link the group space from course navigation.

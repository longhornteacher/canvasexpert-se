# 16 — Unit navigation and external-resource discovery

Requires: [01](01-groups.md); [05](05-discussion-authoring.md) for authored discussion links. Result: assemble a course unit with native discussion links, routes into club spaces, and existing document/meeting links without pretending Canvas can provision external collaboration services.

## Module contract

Extend `ModuleItem` with `external_url` and optional `new_tab`. Add:

- `type: Discussion`, positive `content_id`, no `external_url`. Verify the course discussion belongs to the selected course; for a distributed discussion use the course parent ID.
- `type: ExternalUrl`, required teacher-authored `title` and absolute HTTP(S) `external_url`, optional explicit `new_tab`, no `content_id`. Reject credential-bearing/signed temporary URLs and unsupported schemes.

Existing Page/File/Assignment/SubHeader documents remain unchanged. A direct group discussion or group Page is linked using an explicitly titled URL item, not falsely validated as native course content. Verify same-origin group targets belong to the configured course where they name a Canvas group. An external provider URL is a teacher-selected resource; it does not confer access automatically. [Module item API](https://developerdocs.instructure.com/services/canvas/resources/modules).

Extend `internal/domain/module.go` and `workflow/module_author.go` references/outcomes so uncertain lookup distinguishes type + destination, not title alone. Save actual module-item IDs and resume existing insertion order. Publication still does not publish children or grant group membership. This slice adds links, not module synchronization/deletion/prerequisite management.

## External-resource reads

Add `external-resources COURSE [--group ID]` / `list_external_resources {course, group_id?, resource_type: collaborations|conferences|all}`. Verify scope and use documented course/group collaboration and conference lists. Return IDs, titles/types, context, known participants as refs where supplied, safe returned resource URLs and explicit unavailable fields. Handle the conferences response envelope rather than assuming every list is a bare array. Follow pagination.

The public APIs document collaboration/member reads and conference listings. They do not document creating Google/Microsoft documents or video conferences. Do not call internal web-controller writes, scrape provider tokens, invent document URLs, or open a sessionless launch merely to fill a missing URL. If a record supplies no durable usable link, say so and use a teacher-provided link. [Collaborations](https://developerdocs.instructure.com/services/canvas/resources/collaborations), [Conferences](https://developerdocs.instructure.com/services/canvas/resources/conferences).

Modification timestamps and participant lists are not proof of writing contribution or meeting attendance quality. Do not read external document content or recordings through this metadata surface; those require an authorized provider/read workflow.

## Acceptance

- A module receives a native course Discussion item and a titled group notebook link with exact saved IDs/URLs; existing module types and unpublished-child behavior are unchanged.
- A discussion from another course and a same-origin group link outside the selected course are refused.
- Equally titled links with different destinations stay distinct and a completed resume adds neither twice.
- Metadata reads handle a missing collaboration URL and conference envelopes/pagination without inventing provider state; nothing is created externally.

Add a unit navigation example and update the native-content playbook and formats reference to explain group access versus course-module visibility.

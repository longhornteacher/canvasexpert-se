# 04 — Group resources and uploads

**Tier C: built only when the teacher requests this capability; no other slice depends on it.** Requires: [01](01-groups.md). Result: supply each club's resource folder and upload teacher-authored files with reusable, correctly scoped receipts.

## Surface and receipt

Extend `upload-file COURSE FILE --group GROUP_ID --access members|hidden|locked [--folder ID] [--receipt PATH] [--new-upload]` and existing `upload_file` with optional `group_id`. Course uploads keep their current access enums and defaults. `members` requests ordinary group access; hidden still means discoverable through links, not private.

Add `group-files COURSE GROUP_ID [--folder ID]` / `list_group_files {course, group_id, folder_id?}` returning file/folder metadata without bulk downloading student files. Add `kind: group-folder`, `operation: create|update`, `group_id`, optional exact `folder_id`, `name`, and optional `parent_folder_id` for explicitly requested folder authoring through `push_document`.

New group receipts include `kind: group-file`, origin/course/group identity, requested folder, source hash/size, access request, exact file ID/group URL, and ordinary outcomes. Existing course receipts remain readable. Reject reuse of a course receipt as a group receipt or reuse across clubs. Never silently replace live bytes.

## Implementation

Reuse `internal/workflow/course_upload.go` and `internal/canvas/upload.go` transport/receipt mechanics with a separate group entry point and only small shared helpers where actually reused. Initiate at `POST /api/v1/groups/:group_id/files`, or `POST /api/v1/folders/:folder_id/files` after verifying group ownership. These are documented group/folder upload routes; do not use the personal-files fallback. Reuse multipart upload, HTTPS redirect checks, PAT isolation, sanitized source paths, collision rename, and `.partial` handling. Map `members` to inherited group/folder access rather than introducing a public or course-wide audience. [Group upload route](https://developerdocs.instructure.com/services/canvas/resources/groups).

`locked` must be restricted from the start through an explicitly locked group folder, as for course uploads. If the available group API cannot establish that restriction, hold that access request with an explanation; do not upload openly then promise privacy. Uploaded teacher handouts may be duplicated per club when requested. Existing student files and folders are never replaced, cleaned up, or redistributed by a unit setup.

Group folder routes are in [Files](https://developerdocs.instructure.com/services/canvas/resources/files); transfer behavior is in [File uploads](https://developerdocs.instructure.com/services/canvas/basics/file.file_uploads). Local tests establish routing/credential behavior, not actual student file visibility.

## Acceptance

- Two clubs receiving the same source get separate scoped receipts; neither a course receipt nor another club's receipt is reusable. Changed bytes need deliberate `new_upload` and preserve old content.
- A folder owned by another course/group and a symlinked/private source are rejected; no Canvas PAT is sent to storage hosts.
- Old course-upload tests and specimens still pass.

Document group access labels and add one resource-folder example to native-content guidance. Course module links to group resources remain links, not a change of file ownership.

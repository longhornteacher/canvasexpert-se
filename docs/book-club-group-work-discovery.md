# Book club group work: Canvas REST and CE-SE

Reviewed 2026-10-17. Research and proposed direction, not an approved implementation spec. Evidence is current official API documentation and inspection of Dev. No Canvas Live requests, classroom changes, or new feature implementation were performed.

The user subsequently requested detailed development slices. The [17-slice implementation plan](group-work/README.md) now expands the initial workstreams below with shared contracts, adapter/document changes, dependencies and acceptance scenarios. The plan does not change implementation or evidence status.

## Recommended classroom result

Give each book club a native Canvas group space with a reading plan, rotating roles, shared evidence notebook, weekly discussions, and a shared final product. Keep individual preparation, reflection, and reading checks as individual assignments. This combines substantial collaboration with evidence of each student's learning.

Use one collaborative group set for the unit, with separate groups for actual clubs, including multiple clubs reading the same book when needed. The existing Silver/Red/Blue differentiation tags can continue to supply private supports independently. Do not convert those tags into clubs.

## Canvas capabilities compared with current CE-SE

| Classroom need | Documented Canvas ability | CE-SE today |
|---|---|---|
| Create clubs and manage membership | Course group sets, groups, and memberships | No typed management workflow; read-only API inspection is available |
| Let students choose a club | Self sign-up, section-restricted sign-up, and membership caps; automatic allocation of unassigned students | Not implemented |
| Give each club shared resources | Group-context Pages, discussions, and folders/files | Existing Page and upload operations target courses, not group spaces |
| Weekly club conversation | Group discussion topics, entries/replies, scheduling, and optional linked graded assignments | No discussion authoring or complete thread pull workflow |
| One shared product | Group assignments with shared or individual grading | Authoring has no group settings; work pulls and scoring reject group assignments |
| Keep individual accountability | Ordinary assignments, rubrics, and New Quizzes | Existing support; New Quiz written-work scores remain teacher-entered in SpeedGrader |
| Peer critique | Peer-review assignment settings, explicit reviewer allocation, and assigned/completed status | No typed setup/allocation/tracking workflow |
| Club meeting dates and conferences | Group calendar events and Scheduler appointment slots with group sign-up | Not implemented; Scheduler availability needs checking on the intended Canvas |
| Club announcements and messages | Group discussion topics can be announcements; Inbox conversations have separate APIs | No typed authoring/sending workflow |
| Simultaneous shared-document writing | External collaboration records and participants can be read | Canvas REST does not document collaboration creation; provider/LTI workflow is separate |
| One unit launch point | Course modules can include discussions and external links as well as ordinary content | Current module authoring accepts Page, File, Assignment, and SubHeader only |

“Read-only API inspection” means `cse api` / MCP `canvas_get`, one page per call, subject to the caller's permissions. It does not provide writes, a reusable workflow, or evidence that a group feature has been tested live.

## API findings

### Clubs and membership

Teacher project groups belong to group sets (`group_categories`); a student can belong to one group in each set. Collaborative groups supply shared spaces. Differentiation tags use `non_collaborative` groups and do not supply collaboration features. [Groups API](https://developerdocs.instructure.com/services/canvas/resources/groups).

Useful routes:

- `GET/POST /api/v1/courses/:course_id/group_categories`
- `POST /api/v1/group_categories/:group_category_id/groups`
- `GET /api/v1/groups/:group_id/users`
- `POST /api/v1/groups/:group_id/memberships` with `user_id`
- `DELETE /api/v1/groups/:group_id/users/:user_id`

Group-set settings include `self_signup` (`enabled` or section-restricted `restricted`), `group_limit` for self sign-up, and automatic leader selection. `assign_unassigned_members` distributes remaining students evenly; that is not book-preference matching. Teacher-directed membership and educational balancing should be agent/playbook decisions. [Group Categories API](https://developerdocs.instructure.com/services/canvas/resources/group_categories).

### Discussions

Create a course discussion with `group_category_id` to make group discussions, or create a topic directly under `/api/v1/groups/:group_id/discussion_topics`. A common weekly prompt can therefore serve separate club conversations; book-specific prompts can live in individual group spaces. The API exposes child-topic/group identities and full threaded views, plus entries/replies. Graded discussions use a linked assignment; put the group-set setting on the discussion topic rather than its linked assignment. [Discussion Topics API](https://developerdocs.instructure.com/services/canvas/resources/discussion_topics), [Assignments API](https://developerdocs.instructure.com/services/canvas/resources/assignments).

For assessment, retain author IDs, entry IDs, reply relationships, and timestamps. A teacher summary can identify claims, textual evidence, unresolved questions, and visible participation. Counts alone cannot establish contribution quality or participation in face-to-face meetings. These are design recommendations, not existing Engine behavior.

Topics can also be group announcements using `is_announcement`, subject to posting permissions. Canvas Inbox has separate conversation creation/read/update APIs; it is another possible communication surface, not an implemented CE-SE feature. Any future sending workflow must follow the teacher's explicit message and recipient scope. [Discussion Topics API](https://developerdocs.instructure.com/services/canvas/resources/discussion_topics), [Conversations API](https://developerdocs.instructure.com/services/canvas/resources/conversations).

### Shared submissions and grading

Assignments expose `group_category_id` and `grade_group_students_individually`. A group-targeted assignment override requires a valid group in that assignment's group set. Individual or section overrides are separate mechanisms; group membership alone does not give an ordinary individual assignment a group override. [Assignments API](https://developerdocs.instructure.com/services/canvas/resources/assignments).

Submission reads can include group identity. Comment writes distinguish individual feedback from a whole-group comment with `comment[group_comment]`; this is separate from grade propagation. A group assessment workflow must model the shared artifact, recipient membership, grading mode, and comment audience explicitly. Removing today's group guard would not supply those semantics. [Submissions API](https://developerdocs.instructure.com/services/canvas/resources/submissions).

For this unit, use shared products for collective work and separate individual reflections for personal learning evidence. Whether to use equal group grades or individual grades on the shared product remains a teacher choice.

### Shared notebooks and resources

Group Pages support create/update/read and revision history. `wiki_page[editing_roles]` can allow group `members` to edit a shared notebook, meeting log, or evidence bank. This offers a Canvas-native shared page; simultaneous Google Docs-style editing is not established by this API. [Pages API](https://developerdocs.instructure.com/services/canvas/resources/pages).

Groups also expose file/folder reads and folder creation. Upload implementation must use the documented file-upload protocol and the actual target group's permissions. Existing course upload receipts should not be treated as group receipts. [Files API](https://developerdocs.instructure.com/services/canvas/resources/files).

### Peer critique, calendar, and conferences

Peer reviews can be listed, assigned to a reviewer, and removed. Results distinguish assigned from completed. This can support draft feedback within or across clubs, but reviewer work remains student-specific, and cross-club allocation should follow the teacher's intended audience. It does not automatically measure each member's teamwork. [Peer Reviews API](https://developerdocs.instructure.com/services/canvas/resources/peer_reviews).

Calendar events accept group contexts for club meetings. Scheduler appointment groups can target one group set so students reserve conference slots as a group. These are separate from assignment due dates, and district availability/permissions have not been checked. [Calendar Events API](https://developerdocs.instructure.com/services/canvas/resources/calendar_events), [Appointment Groups API](https://developerdocs.instructure.com/services/canvas/resources/appointment_groups).

### External documents and unit navigation

The public Collaborations REST resource documents GET operations for collaboration records, members, and potential collaborators. It does not document creating a collaboration or provisioning/editing a Google/Microsoft document. Those actions require the configured provider/LTI experience or an independently authorized provider integration. [Collaborations API](https://developerdocs.instructure.com/services/canvas/resources/collaborations).

Similarly, the public Conferences REST resource lists course/group conferences and available meeting metadata; it does not document conference creation. Video-meeting provisioning should remain a provider/Canvas UI question rather than a promised native REST write. [Conferences API](https://developerdocs.instructure.com/services/canvas/resources/conferences).

Modules are course-scoped. Canvas supports `Discussion` and `ExternalUrl` module items, which would help connect the unit to its conversations and resources. A group space is not a group-specific Canvas module. [Modules API](https://developerdocs.instructure.com/services/canvas/resources/modules).

## What Dev actually implements

- [`internal/domain/types.go`](../internal/domain/types.go): `AssignmentSnapshot` records `GroupCategoryID`; `AssignmentDocument` has no authorable group-set, individual-group-grading, or peer-review settings. Recording a group ID is not group-work support.
- [`internal/workflow/author.go`](../internal/workflow/author.go): create payload has ordinary assignment fields, with no group settings. [`internal/workflow/edit_plan.go`](../internal/workflow/edit_plan.go) similarly has no group/peer-review patch fields.
- [`internal/workflow/canvas_reads.go`](../internal/workflow/canvas_reads.go): `unpullableAssignment` and `UnsupportedAssignment` reject group submissions. The assignment DTO does not capture `grade_group_students_individually`.
- [`internal/domain/scoring.go`](../internal/domain/scoring.go): both numeric assessment and comment-only validation reject group assignments.
- [`internal/workflow/pull_work.go`](../internal/workflow/pull_work.go): a conditional parser renders a student's own `discussion_entries` when present. It does not fetch a complete discussion thread. The submissions requests in `canvas_reads.go` do not use discussion-topic endpoints. This is partial code support, not a verified discussion assessment workflow.
- [`internal/workflow/hub.go`](../internal/workflow/hub.go) and [`internal/workflow/differentiation.go`](../internal/workflow/differentiation.go): intentionally select non-collaborative differentiation tags.
- [`internal/workflow/page.go`](../internal/workflow/page.go), [`internal/workflow/course_upload.go`](../internal/workflow/course_upload.go), and [`internal/workflow/module_author.go`](../internal/workflow/module_author.go): useful course-content foundations; they do not provision group spaces. Supported module item types are limited as stated above.
- [`engine-capabilities.md`](engine-capabilities.md): ordinary assignment/rubric/scoring workflows have recorded live evidence within their stated scopes. Native Pages, module assembly, and Engine course uploads have local checks with live/student validation pending. None of that establishes group-work evidence.

## Proposed delivery order for discussion

1. **Clubs and their working spaces.** Inspect/create collaborative group sets and named clubs; apply teacher-approved memberships or self sign-up; create group Pages and weekly discussions; add course-module navigation. Read full conversations with attribution. This delivers a useful book club even before shared-product grading is automated.
2. **Shared products and assessment.** Author group assignments, pull a shared artifact with membership/attempt identity, review a group assessment, and post grades plus explicitly scoped feedback. Include both shared and individual grading modes in the eventual design. Show all recipients before a shared write; preserve individual notes separately. Until this slice is delivered, shared-product grading stays in Canvas.
3. **Peer critique and scheduling.** Add reviewer allocation/completion reads, club calendar events, and group conference sign-up where available. Build teacher summaries from visible discussion and submission evidence; keep role rotation, prompts, and instructional judgments in a book-club playbook. External document provisioning is a separate optional integration decision.

These are candidate slices, not assigned agent tasks. After direction is agreed, write focused implementation contracts using existing feature functions, document/resume patterns, and typed CLI/MCP operations. No general HTTP-write tool, new workflow framework, or permanent participation database is needed.

Focused validation should cover target course/group membership, preservation of student/unsent work, and grade/comment audience because mistakes there can affect students. Local tests establish implementation behavior; an authorized live classroom or disposable-object check establishes Canvas behavior; student access remains a separate observation. Research alone does not change any of those evidence labels.

An immediate unit can use CE-SE for the course hub, directions, rubrics, individual submissions/reflections, and reading checks, while the teacher creates and runs clubs/discussions/shared submissions in Canvas. That is a practical current route, not full automated group-work support.

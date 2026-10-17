# Shared group-work contracts

Date: 2026-10-17. Proposed contracts used by the [slice plan](README.md). Implement these where first needed, not as a general-purpose operation framework.

## Identity and ownership

Every group write starts from configured Canvas origin + course ID + exact group-set/group ID. Verify that the group belongs to that course and set and is collaborative. Never infer this from a group name, URL supplied in student work, or permission to read an unrelated group. Disallow account/community groups and differentiation tags on these feature paths. Do not modify the existing Hub tag paths.

Use Canvas IDs for groups/topics/assignments and existing `student_ref` values for people. Resolve student refs inside the Engine through `workspace.StudentStore`; never put private roster rows, addresses, SIS/login IDs, or raw user objects in ordinary packets. Group labels are teacher-authored and may contain names; do not promise automatic anonymization of their prose. Student writing remains intact. Instructors and unknown participants must not become invented student identities.

All plans, drafts, pull packets, receipts, and outcomes live under `UserInstall/courses/<course>/...`, through existing constrained paths. Reusable teaching choices/playbooks follow the current UserInstall/UserSync rules authority. A group membership snapshot is a dated Canvas observation, not an authoritative reusable student setting to synchronize. No new private identity database is required.

## Documents and adapters

Retain `schema: 1` and old defaults. Existing course-only documents remain valid. Each new kind has its own typed Go struct and validator. `push FILE` and MCP `push_document` gain explicit dispatch cases for new authoring kinds; this remains allowlisted typed work, not arbitrary API access. `post FILE`/`post_results` dispatches ordinary `results` and new `group-results`. `review FILE`/`start_review` dispatches supported content/results kinds through existing session machinery.

Add read commands/tools only as needed by each slice. The following common conventions are fixed:

- `groups COURSE [--set ID] [--members GROUP_ID]` is read-only; flags cannot select an unrelated course/set. MCP `list_group_sets {course}` and `list_groups {course, group_set_id?, group_id?, include_members?}` return normalized summaries; all listing pagination is handled by the Engine.
- `pull-group-set COURSE SET_ID` / `pull_group_set {course, group_set_id}` saves an editable exact-ID baseline.
- `pull-discussion COURSE TOPIC_ID [--group GROUP_ID]` / `pull_discussion {course, topic_id, group_id?}` saves a thread packet and editable topic baseline.
- `pull-work`/`pull_work` retain their individual defaults and gain the group/discussion behavior described in their slices; no silent all-course expansion when an explicit target was supplied.
- A document's `group_id` always denotes the target Canvas collaboration group. `group_category_id` denotes its group set. `assignment_group_id` remains the gradebook category and never denotes a club.

Creation documents may use stable local `key` strings to match saved outcomes to rows before Canvas IDs exist. Keys are not Canvas identifiers. Persist exact returned IDs immediately. An omitted row is not a deletion instruction. Update documents carry `observed` baselines, and omitted optional update fields mean preserve. Pointer/presence-aware fields distinguish absent, false, zero, and explicit null.

The specimen documents in each slice illustrate authorable fields. Resumable `created`/`outcomes` and `observed` fields are Engine-authored; define their exact structs and document them while implementing. They must carry origin/course/target IDs and distinguish requested actions with existing `ActionOutcome` vocabulary. Do not invent opaque approval tokens or require users to edit private files.

## Writes, review, and resumption

Read relevant live identity and replaced fields immediately before a write. Extra per-action re-reads are reserved for what can harm students: propagated group grades/comments, membership moves, and replacement of existing content. An ordinary additive create relies on that single pre-write identity check; do not add confirmation reads after accepted writes. Apply only the requested patch. Do not invalidate an edit because an unrelated title/date changed. Preserve an unsent draft and provide a fresh baseline on a conflict.

Persist pending state before a non-idempotent write, save accepted outcome and IDs afterward, and reuse existing bounded uncertain-create lookup behavior. A successful assignment/rubric/grade/comment write counts as complete without a confirmation GET or requiring a complete response shape. Reads before subsequent distinct actions are permitted for that action's identity/attempt checks. A transport timeout is not a known failure; do not blindly replay. Known accepted work is not replayed because the teacher later edits or deletes it. Separate genuinely failed items from uncertain ones and continue independent items when possible.

Default review remains required by the playbooks for scores and assignment create/edit, including linked graded discussion assignments. Explicit task-scoped skip-review still works. Ordinary group setup, Page edits, file uploads, and calendar requests need no newly invented approval ceremony. Announcements and Inbox sends require teacher intent covering content/audience; drafting or building a unit does not authorize sending. Optional content review can reuse the existing page; no special Engine authorization token is added.

Review shows the actual recipients and operation. Group grades/rubrics, whole-group comments, individual grades, individual comments, and public topic replies are different actions. Deselecting one member cannot silently turn a shared grade into a subset write. Use stable card/action references incorporating the group ID and, for private actions, student ref; never use a club title as the review key. Raw private notes never enter shared feedback or student-visible criteria comments.

## Assignment snapshots and grade scope

**Slice 08 owns this foundation; slices 07 and 09-11 consume it and do not redefine it.** Slice 08 extends the Canvas assignment DTO and `AssignmentSnapshot` with presence-aware `grade_group_students_individually`, linked discussion topic identity, and the presence-aware `has_sub_assignments`/`checkpoints` metadata (requested with `include[]=checkpoints` on discussion assignment reads), and implements the `SameGradingBasis` normalization below. Slice 08's acceptance includes a saved pre-feature individual packet/results still posting against an unchanged live individual assignment. Missing mode on a group assignment means unknown, not false. Include grading mode and group-set identity in `SameGradingBasis`. Preserve old individual snapshots. The group guards remain in place until the corresponding typed pull/post path lands.

Do not treat group discussions as shared-product assignments. Their group context lives on the topic and their linked assignment can still grade each student's conversation work. Validate that association rather than copying the shared-product grading rules to it. [Assignments API](https://developerdocs.instructure.com/services/canvas/resources/assignments).

For a shared product, record both the current course-enrolled group audience and each member's returned submission group/attempt identity. Never reconstruct an earlier author's group from current membership alone. Use `GET /api/v1/courses/:course_id/assignments/:assignment_id/users/:user_id/group_members` for the representative member's assignment-specific propagation audience, normalized inside the Engine. Current upstream routes it through `assignment.group_students`; a generic group roster may have different visibility/enrollment filtering. If the full assignment audience cannot be read, hold that propagated action rather than infer a complete audience from a partial roster. [Assignment audience endpoint implementation](https://github.com/instructure/canvas-lms/blob/master/app/controllers/assignments_api_controller.rb).

Before a propagated write, compare that full live audience with the reviewed snapshot and the teacher's requested scope. A subset request cannot trigger a whole-group write. Excluded, excused, unresolved, or changed recipients hold that group action; independent groups continue.

Current upstream Canvas source selects grade recipients from current group/course memberships, propagates rubric assessment in shared-grading mode, and handles whole-group comments separately. This is source evidence, not proof of the district's deployed version: [grade and audience logic](https://github.com/instructure/canvas-lms/blob/master/app/models/abstract_assignment.rb), [rubric propagation](https://github.com/instructure/canvas-lms/blob/master/app/models/rubric_association.rb), [submission write controller](https://github.com/instructure/canvas-lms/blob/master/app/controllers/submissions_api_controller.rb).

## Assessment and evidence

Reuse `Decimal`, `ComputeAssessment`, truthful rating IDs, local scoring bases, criterion/component mapping, strongest-prior academic comparison, and review rendering. Do not copy arithmetic into a new feature. Post the computed assessment even when lower than an entered grade. Do not implement contribution multipliers or normalize a rubric to a preferred final grade.

Only identify missing work from a complete relevant read. Inaccessible, truncated, unmatched, or unreadable evidence is unknown. A group artifact's existence does not prove every member contributed. For discussion assessment, retain attributable entry references and the relevant conversation context. Report oral participation, external document revisions, and unreadable media as unobserved.

Keep lateness a distinct requested action. Canvas calculates penalties. Shared-mode lateness writes may propagate and therefore require a common reviewed timing action for the same full audience; otherwise omit timing, report the limit, and continue valid assessment. Individual group-grading mode can use per-member effective due dates and timing. Dates must be checked against the applicable live override, not merely a course-level assignment date.

## Integration check for every slice

Update CLI allowlists/arity/parsing in `cmd/cse/main.go` and `remote.go`, offline validation dispatch, MCP input schemas/command argument construction/advertised tools, and relevant review dispatch. Use `longCommand` for long read pulls where appropriate; do not create another background subsystem. Extend the existing compiled MCP smoke check when tool inventory or adapters change.

Each slice's acceptance list names meaningful checks. Protect wrong-target writes, work preservation, honest grades, and feedback audiences. Avoid source-text tests that merely assert a function name or documentation phrase. Keep capability status honest: planned, local-test evidence, Canvas Live evidence, and pending student validation are distinct.

## Existing behavior at the shared boundaries

The code review on 2026-10-17 identified these concrete compatibility requirements. Verify each when its owning slice changes that boundary; do not turn them into a prerequisite test campaign for unrelated slices.

- **Saved grading sessions (07-11):** `SameGradingBasis` ultimately uses `reflect.DeepEqual` in `SameSnapshot`. An old individual snapshot lacking a new group-mode field must compare equivalent to live non-group data carrying false. Normalize only fields irrelevant to that non-group target. Missing mode on an actual group remains unknown. Apply the same semantic rule when new discussion metadata is merely descriptive. Old unsent results must not require blanket re-pulling or migration.
- **Review selection (05, 10-11, 15):** the current callback passes `skip[student_ref]` to `Service.Post`. New group/action card keys need explicit feature-specific dispatch and matching skip checks; do not pass a group-card key into a student-only filter and then post every group. Existing assignment/quiz and parent-email review selection must retain its present meaning. Group grades stay indivisible; private comments remain separately selectable.
- **Discovery and completion (06-11):** retain current individual attempt completion, explicit reassessment, and accepted-write no-replay behavior. Group content deduplication must not collapse per-student completion in individual-grade mode or make all previously completed work appear new after metadata additions.
- **Hub restrictions (01-05):** `DifferentiationTags` and Hub discovery deliberately request non-collaborative groups. New club helpers must not change that filter, treat support tags as clubs, or make private support Pages broadly visible.
- **Course content (03-05, 08, 16):** absent group fields retain the existing course routes, access defaults, receipts, module types, publication ordering and edit patches. Extra optional fields must not clear unrelated existing settings or disqualify a known accepted create during resume.
- **Shared records/UI (07-12):** parent reports use their own flat per-student course submission read and the common submission DTO; keep that shape intact. Review models also serve parent emails, assignments, and quizzes. Group display changes must not alter those recipients, action handlers or feedback headers. New Quizzes must keep their comment-only/no-item-grade boundary, and ordinary rubric/lateness behavior must retain its current scope.
- **Adapters (all affected slices):** keep existing MCP field names such as `query`, old CLI argument forms, and direct/review command routing. `push`, `post`, and review callbacks must explicitly distinguish new kinds instead of widening an old catch-all branch. Update the existing compiled MCP smoke check as inventory changes; adding tools is not a reason to change installation, credentials or UserSync routing.

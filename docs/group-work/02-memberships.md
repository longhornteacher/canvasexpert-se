# 02 — Membership, self sign-up, and allocation

Requires: [01](01-groups.md). Result: assign named students to clubs, show unassigned students, manage self sign-up, or request an even allocation without silently regrouping students.

## Surface and document

`groups COURSE --set ID --members GROUP_ID` and MCP member reads return stable refs/aliases, membership state, group identity, and roster completeness. Extend set reads to expose unassigned course students. Use `kind: group-memberships` through existing validate/push adapters.

`pull-group-memberships COURSE SET_ID` saves the read as an editable exact-set baseline. The pulled update document initially has no `changes`; add explicit rows before requesting membership writes. A set read includes each club's stable-ref/alias memberships and state, active unassigned students, pending join requests with `student_ref`, `group_id`, and `state: requested`, and whether every observed membership resolved to the current course roster. Student names, Canvas user IDs, SIS/login identifiers, and contact fields never enter the read packet or document. An unresolved pending requester makes the roster incomplete and prevents saving an editable plan.

```yaml
schema: 1
kind: group-memberships
operation: update
course: ELA7
group_category_id: 501
changes:
  - student_ref: student-ref-a
    from_group_id: null
    to_group_id: 701
  - student_ref: student-ref-b
    from_group_id: 702
    to_group_id: 701
```

`from_group_id` is required and nullable: null means observed unassigned, not “anywhere.” `to_group_id: null` is explicit removal. One row per student in a plan; positive IDs must belong to the selected set. Engine-generated baselines include accepted/invited membership state. No replace-all list or remove-by-omission operation.

Self sign-up settings use slice 01's set update. For teacher-requested even allocation, allow `operation: allocate-unassigned` with the exact set, observed unassigned refs and existing group IDs, plus Engine outcomes. This mode is separate from explicit changes. Book preferences, grouping balance, and role rotation are playbook judgments.

The membership baseline's `observed` object carries `instance`, `course_id`, `group_category_id`, all existing `group_ids`, `roster_complete`, accepted/invited memberships (`student_ref`, `group_id`, `state`), pending requests (`student_ref`, `group_id`, `state: requested`), and `unassigned_student_refs`. An explicit plan carries unique `changes`, each with required nullable `from_group_id`/`to_group_id`. Outcomes are saved by `student_ref` with the actual group/state returned by Canvas where available. `authorized_submitted_work_change_refs` is an optional list of the specific students whose membership changes the teacher explicitly authorized after reviewing their submitted work.

An allocation draft uses `operation: allocate-unassigned`, no explicit changes, and an `allocation` outcome. The Engine records the unassigned audience immediately before calling Canvas and uses `sync=true`. On success, it maps the returned `new_members[].user_id` values to stable student refs inside the Engine; names, IDs, and section data in Canvas's response are never saved or displayed. It does not read memberships back after a successful response. After a timeout or interruption it reads the roster and reconciles the known audience; it never replays an uncertain allocation. Canvas's allocation endpoint is set-wide and cannot accept a student subset, so a newly unassigned student outside the reviewed audience holds the operation for a refreshed baseline.

## Implementation

Add `group_memberships.go` workflow/domain types. Resolve all refs and validate current course enrollment before the first write. Re-read each student's source membership before mutation. Already at the intended destination is a preserved success. A different source than observed holds that row. Record outcomes per student and keep independent changes going.

Use membership POST for adds/moves and DELETE only for explicit removal. Current upstream `GroupMembership#ensure_mutually_exclusive_membership` removes a student's other same-set memberships after the destination membership saves; its `auto_join` accepts non-community invites. Therefore a move is a destination POST after checking the observed source, not a remove-first sequence. Persist its outcome and actual returned state; if the deployed Canvas rejects the operation, report it rather than trying a destructive workaround. A current pending self-sign-up request holds that student's explicit placement. Accepted membership writes are never replayed on resume; uncertain writes are reconciled from a current roster and remain uncertain if the requested state is not observed. Prefer synchronous even allocation with `sync=true` to avoid adding a job system. [Membership API](https://developerdocs.instructure.com/services/canvas/resources/groups), [allocation](https://developerdocs.instructure.com/services/canvas/resources/group_categories), [upstream membership behavior](https://github.com/instructure/canvas-lms/blob/master/app/models/group_membership.rb).

A move can change access to existing group work. When the selected student/set has submitted work, explain that concrete consequence and preserve the work; an explicit teacher instruction covering that move is sufficient, without repeatedly asking permission. Generic “balance the clubs” does not authorize displacing submitted-work memberships. No automatic reassignment, clone/merge of historical submissions, or synthetic work migration.

Allocation must check the observed audience again and report whom Canvas actually assigned from the synchronous response. A timeout is uncertain, not an excuse to run allocation again; only then use a fresh membership read to reconcile. Respect Canvas's set capacity/section validation for teacher placements too; do not silently disable it to satisfy a plan. Additional teacher-requested grouping constraints remain explicit agent judgments. Report unresolved enrollments or rejected placements specifically.

## Acceptance

- Adds and a move touch only the listed refs; a student whose source membership changed since the baseline is held while unrelated rows continue.
- A move is a destination POST after the observed-source check. An uncertain move is resolved by a current-membership read, never by removing the source to retry.
- Cross-course refs/groups and tag groups cause zero writes.
- Allocation excludes already-assigned students and reports whom Canvas actually assigned; a late self sign-up is reported, not overwritten.
- Member output carries refs/aliases only, no contact or login data.

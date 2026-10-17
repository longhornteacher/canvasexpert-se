# 02 — Membership, self sign-up, and allocation

Requires: [01](01-groups.md). Result: assign named students to clubs, show unassigned students, manage self sign-up, or request an even allocation without silently regrouping students.

## Surface and document

`groups COURSE --set ID --members GROUP_ID` and MCP member reads return stable refs/aliases, membership state, group identity, and roster completeness. Extend set reads to expose unassigned course students. Use `kind: group-memberships` through existing validate/push adapters.

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

## Implementation

Add `group_memberships.go` workflow/domain types. Resolve all refs and validate current course enrollment before the first write. Re-read each student's source membership before mutation. Already at the intended destination is a preserved success. A different source than observed holds that row. Record outcomes per student and keep independent changes going.

Use membership POST for adds/moves and DELETE only for explicit removal. Current upstream `GroupMembership#ensure_mutually_exclusive_membership` removes a student's other same-set memberships after the destination membership saves; its `auto_join` accepts non-community invites. Therefore a move is a destination POST after checking the observed source, not a remove-first sequence. Persist its outcome and actual returned state; if the deployed Canvas rejects the operation, report it rather than trying a destructive workaround. Prefer synchronous even allocation with `sync=true` to avoid adding a job system. [Membership API](https://developerdocs.instructure.com/services/canvas/resources/groups), [allocation](https://developerdocs.instructure.com/services/canvas/resources/group_categories), [upstream membership behavior](https://github.com/instructure/canvas-lms/blob/master/app/models/group_membership.rb).

A move can change access to existing group work. When the selected student/set has submitted work, explain that concrete consequence and preserve the work; an explicit teacher instruction covering that move is sufficient, without repeatedly asking permission. Generic “balance the clubs” does not authorize displacing submitted-work memberships. No automatic reassignment, clone/merge of historical submissions, or synthetic work migration.

Allocation must check the observed audience again and report whom Canvas actually assigned. A timeout is uncertain, not an excuse to run allocation again. Respect Canvas's set capacity/section validation for teacher placements too; do not silently disable it to satisfy a plan. Additional teacher-requested grouping constraints remain explicit agent judgments. Report unresolved enrollments or rejected placements specifically.

## Acceptance

- Adds and a move touch only the listed refs; a student whose source membership changed since the baseline is held while unrelated rows continue.
- A move is a destination POST after the observed-source check. An uncertain move is resolved by a current-membership read, never by removing the source to retry.
- Cross-course refs/groups and tag groups cause zero writes.
- Allocation excludes already-assigned students and reports whom Canvas actually assigned; a late self sign-up is reported, not overwritten.
- Member output carries refs/aliases only, no contact or login data.

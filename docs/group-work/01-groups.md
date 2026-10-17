# 01 — Collaborative group sets and clubs

Requires: existing Engine. Read [shared contracts](contracts.md). Result: the teacher can inspect groups, create a book-club set with named clubs, and edit their names/settings without touching differentiation tags.

## Surface and document

Implement the three group-read surfaces in shared contracts. Add `kind: group-set` to `validate`, `push`, and `push_document`; `pull-group-set` returns the update baseline. Read results include course/set/group identity, collaboration state, signup mode, configured size limit, and membership counts; memberships are returned only when requested.

```yaml
schema: 1
kind: group-set
operation: create
course: ELA7
name: Fall book clubs
self_signup: disabled
groups:
  - key: club-a
    name: The Giver - Club A
    description: Weekly reading and discussion workspace
  - key: club-b
    name: The Giver - Club B
```

Authorable set fields: `name`, `self_signup: disabled|enabled|restricted`, optional positive `group_limit`, `auto_leader: none|first|random`. Normalize application enum values to the API's supported representation; clearing a setting must be explicit. A limit requires self sign-up. Each existing group update uses exact `group_id`, optional `name`/`description`, and an observed baseline. Additive creation of another keyed group is allowed. Omitting an existing group preserves it. Do not allow conversion to/from differentiation tags or transfer across courses/sets.

## Implementation

Add small `internal/domain/groups.go` contracts/validators and `internal/workflow/groups.go` operations; reuse Canvas List/Get/Write and course resolution. Persist `created.group_category_id`, each keyed `group_id`/URL/outcome, and set-settings outcome. Verify saved IDs before continuing, rather than restarting creation. Reject duplicate local keys; identical human names are allowed only when exact IDs disambiguate existing selections. On an ambiguous new create, bounded lookup may establish the known object; a same-name match alone is insufficient ownership proof.

API scope: course `group_categories`, set `groups`, and group GET/PUT. Create a course collaborative set, never `/api/v1/groups` community groups. Filter lists with `collaboration_state=collaborative`; Canvas objects expose `non_collaborative` rather than a `collaboration_state` field, so verify that it is not true and check `context_type: Course`, `course_id`, `group_category_id`, and exact IDs. Settings and route semantics are in [Group Categories](https://developerdocs.instructure.com/services/canvas/resources/group_categories) and [Groups](https://developerdocs.instructure.com/services/canvas/resources/groups).

Update help, MCP registration, format examples, and a short groups section in the authoring seed. No membership mutation in this slice. Requested default setup is direct under teacher scope; adding another local approval UI is unnecessary.

## Acceptance

- A two-club create saves set and group IDs; resuming after a partial failure creates only the missing group, never another set or a duplicate club.
- A group or set from another course, and a non-collaborative tag, are refused before any write.
- Ambiguous names return choices with IDs; pagination reaches later groups.
- A baseline title/settings conflict preserves the draft; disabled/null and absent settings stay distinguishable.
- CLI and MCP use the same feature operations; existing course/module and Hub tests still pass.

Record local implementation evidence. Do not mark club/student access live-verified from these checks.

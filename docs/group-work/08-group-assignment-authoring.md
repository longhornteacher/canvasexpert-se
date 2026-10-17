# 08 — Group assignments, audience, and dates

Requires: [01](01-groups.md). Result: create a shared-product assignment with a selected club set, shared/individual grading mode, and requested differentiated availability/due dates.

## Assignment contract

Extend existing `kind: assignment` authoring with optional positive `group_category_id` and presence-aware `grade_group_students_individually`. If a group set is supplied, grading mode must be explicit. Existing individual documents omit both and behave unchanged. Initially support the existing text/file submission modes; no group New Quiz conversion.

```yaml
schema: 1
kind: assignment
operation: create
course: ELA7
title: Book club final interpretation
points: 100
submission_types: [online_upload]
group_category_id: 501
grade_group_students_individually: false
published: false
```

Capture both fields in snapshots, uncertain-create identity comparison, review facts, and grading-basis comparison. Existing group assignments can be pulled for edits/independent rubric attachment without enabling their scoring paths prematurely. Keep group pulls/scoring guarded until slices 09-11.

Do not silently change an existing assignment's group set or grading mode. Explicit changes are allowed only before submitted work exists; thereafter hold that structural change and explain Canvas/SpeedGrader handling. Title/directions/date edits remain ordinary baseline-controlled edits.

## Snapshot foundation (owned here, consumed by 07 and 09-11)

Land this first inside slice 08, before authoring changes depend on it:

- Extend the Canvas assignment DTO and `AssignmentSnapshot` with presence-aware `grade_group_students_individually`, linked discussion topic identity, and presence-aware `has_sub_assignments`/`checkpoints` (request `include[]=checkpoints` on discussion reads).
- Make `SameGradingBasis`/`SameSnapshot` (currently `reflect.DeepEqual`) semantic: an old individual snapshot lacking the group-mode field equals live non-group data returning false; normalize only fields irrelevant to that non-group target. Missing mode on an actual group stays unknown. Descriptive discussion/checkpoint metadata that differs only by absence on one side does not invalidate an unsent result.
- Group guards (`unpullableAssignment`, `UnsupportedAssignment`, scoring validation) stay in place until slices 09-11 replace them.

## Overrides

Add `kind: assignment-dates`, `operation: update`, exact assignment identity, optional explicit `only_visible_to_overrides`, and keyed `overrides` rows. Each row chooses exactly one target: `group_id`, `student_refs`, or `course_section_id`; it carries nullable `due_at`/`unlock_at`/`lock_at`, optional exact `override_id`, and an observed target/date baseline. Omission preserves a date; null clears it. A student target requires a title. Existing overrides not listed are preserved; `operation: remove` on an exact row is the only removal instruction.

Engine-managed `outcome` fields record accepted or unresolved writes. Keep an accepted create's returned `override_id` paired with its succeeded outcome when resuming other rows. Pending or uncertain rows remain unresolved and are not replayed; visibility outcomes are likewise retained so later Canvas changes are not overwritten by resuming the document.

Group overrides must target that assignment's selected set. For ordinary individual assignments, a teacher can request an explicit ref-based audience derived from club membership; record the selected refs and explain that future membership changes do not automatically retarget the individual assignment. Do not substitute a collaborative group override for that individual assignment. [Assignment/override API](https://developerdocs.instructure.com/services/canvas/resources/assignments).

## Implementation

Extend `domain/types.go`, `validation.go`, `workflow/author.go`, `edit_plan.go`, `canvas_reads.go`, and review facts. Add a feature-specific overrides workflow with exact IDs, returned outcomes and baseline comparisons. Use the assignment's `/overrides` POST and exact `/overrides/:override_id` PUT/DELETE endpoints rather than sending a replace-all override array. Creating an assignment with requested overrides keeps it unpublished until rubric, dates/audience and module stages finish. Override failures on existing assignments preserve other completed changes and report actual exposure/state.

Effective-due-date reads must expose each student's actual due date for later assessment timing and reports; do not assume a teacher's base assignment date is everyone's deadline. All dates use explicit RFC3339 values and configured course timezone for display.

## Code anchors (verified in Dev 2026-10-17)

- `UnsupportedAssignment` (`canvas_reads.go:304`) has exactly one caller: `PullItem` in `workspace_reads.go:110` (pull-for-edit). It refuses group sets, anonymous/moderated grading, non-points/percent grading and any submission type except `online_text_entry`/`online_upload`. Relax it for group assignments here; this does not touch scoring. Decide deliberately whether to also admit `discussion_topic` (needed by 05b and 12) rather than leaving the refusal in place by accident.
- The scoring/pull guards are separate: `unpullableAssignment` (`canvas_reads.go:287`, called at `pull_work.go:204` and `257`) and the results validators (`scoring.go:286`, `408`). Leave them until slices 09-11.
- `sameCreateCore` (`author.go:436`) requires `GroupCategoryID` nil/0, and `createFields` (`author.go:428`) sends no group fields. Resume after an uncertain group-assignment create would never match and would create a duplicate. Both must carry the requested group set and mode.
- `SameGradingBasis` (`edit_plan.go:68`) is a deny-list: it blanks title/dates/rubric-use flags and compares everything else through `SameSnapshot` (`reflect.DeepEqual`). A new `AssignmentSnapshot` field is compared by default, so a pointer field that is nil in an old saved snapshot but `false` live would hold every unsent grade as "grading basis changed". Blank or normalize each new field explicitly, and keep a golden old-format snapshot that must still compare equal.
- `PlanEdit` sends only changed fields and `sameEditFields` checks only replaced ones, so absent group fields on an update already mean preserve. Keep that property.

## Acceptance

- Group set and grading mode survive create, pull, edit, review and resume; an absent mode on a live group stays unknown.
- A wrong-course or tag set, a nonmember group override, mixed override targets, and group mode on a New Quiz cause zero writes.
- Override create/update/remove affects only listed rows; explicit null clears a date, omission preserves it, and a moved live deadline holds a replacing timing edit.
- A structural group change after submissions is held while a title edit still works.
- The snapshot compatibility rule below holds: a saved pre-feature individual packet/results still posts against an unchanged live individual assignment.

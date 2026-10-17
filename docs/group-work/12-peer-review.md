# 12 — Peer critique setup, allocation, and completion

**Tier C: built only when the teacher requests this capability; no other slice depends on it.** Requires: [01](01-groups.md), [02](02-memberships.md), [08](08-group-assignment-authoring.md); [09](09-group-submissions.md) for shared products. Result: assign students to critique selected drafts within or across clubs and identify which assigned reviews remain incomplete.

## Settings and surface

Extend assignment create/edit/pull/review with an optional `peer_review` block: `enabled`, `automatic`, `count`, `assign_at`, `anonymous`, `allow_same_group`. Use presence-aware values and map only supported fields to Canvas; do not clear unrelated settings when absent. When automatic is true, teacher supplies the intended count/timing; when false, explicit allocations below are used. These are student reviewer assignments, not automated teacher grades.

Map these local fields to `assignment[peer_reviews]`, `[automatic_peer_reviews]`, `[peer_review_count]`, `[peer_reviews_assign_at]`, `[anonymous_peer_reviews]`, and `[intra_group_peer_reviews]`, respectively. The local block is not a raw pass-through of Canvas's newer nested peer-review grading configuration. Limit it to supported text/file assignments; unsupported discussion/quiz/external-tool settings are explained without breaking their ordinary workflows. [Assignment settings](https://developerdocs.instructure.com/services/canvas/resources/assignments).

Add `peer-reviews COURSE ASSIGNMENT_ID` / `list_peer_reviews {course, assignment}`. Normalize reviewer/reviewee identities to refs, retain exact review/submission/assignment IDs, assigned/completed state, and requested review comments when available. No mark-completed operation.

Add `kind: peer-reviews`, `operation: update`:

```yaml
schema: 1
kind: peer-reviews
operation: update
course: ELA7
assignment_id: 9001
allocations:
  - key: a-reviews-b
    reviewer_ref: student-ref-a
    reviewee_ref: student-ref-b
    action: assign
```

Each allocation references the actual target submission identity obtained by the Engine. Removal requires the exact observed allocation/target and `action: remove`; omitted allocations remain. In `/assignments/:assignment_id/submissions/:submission_id/peer_reviews`, `submission_id` is the actual submission record ID; the required `user_id` is the reviewer's privately resolved Canvas user ID. Unlike ordinary submission-update routes, the path is not the reviewee's user ID. Upstream `peer_review_assets` selects `@assignment.submissions.find(params[:submission_id])`. [Peer Reviews](https://developerdocs.instructure.com/services/canvas/resources/peer_reviews), [controller](https://github.com/instructure/canvas-lms/blob/master/app/controllers/peer_reviews_api_controller.rb).

## Behavior and implementation

Add `peer_review.go` feature/domain files. The agent chooses pairings from the teacher's instructions (within club, across books, one review each, etc.); the Engine validates exact people, current enrollments/assignments, self-review exclusions, and requested group constraints. For a shared product, resolve a real owner's submission and reviewer ref; do not fabricate a group-as-user reviewer.

Before allocation, read existing reviews and preserve an exact existing reviewer/target pair as success. Reject contradictory duplicate rows. Do not delete a completed review to rebalance or make automatic/manual mode changes because a list looks uneven. An explicitly requested removal of completed evidence must be reported as consequential and never bundled into ordinary setup. Automatic settings do not prove allocations have run; report returned state as of the read.

Comments and completion remain student evidence. Canvas may require a rubric response as well as a comment before a review becomes complete. Report the actual workflow state instead of inventing completion from a comment count. Private teacher notes and peer-review identity settings must not leak into public summaries.

## Acceptance

- Within-club and cross-club plans produce the exact requested pairs using the correct target submission IDs; existing pairs are not duplicated and a timeout is resolved by an exact-pair read.
- Self-review, wrong-course users and contradictory allocations cause no writes; a missing target submission holds only that pair.
- Completed reviews survive a rerun or rebalance request; assigned/completed labels come from Canvas state.
- Anonymous reviewer settings are preserved in student-facing behavior.

Add a peer-critique section to the group-work playbook and a synthetic allocation/status example. No permanent reviewer allocator or scheduled background task is needed.

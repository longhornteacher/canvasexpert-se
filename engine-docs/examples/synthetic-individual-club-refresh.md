---
schema: 1
kind: assignment-dates
operation: update
course: ELA7
instance: https://canvas.example.edu
course_id: 101
assignment_id: 301
overrides: []
audience:
  mode: individual
  policy: snapshot
  source:
    group_category_id: 501
    group_ids: [701]
  state:
    instance: https://canvas.example.edu
    course_id: 101
    target_id: 301
    source:
      group_category_id: 501
      group_ids: [701]
    observed_at: "2026-10-19T12:00:00Z"
    student_refs: [s-reader-b]
    fingerprint: synthetic-membership-fingerprint
    complete: true
    observed_visibility: true
    owned:
      - id: 801
        baseline:
          title: Book club individual audience
          student_refs: [s-reader-a, s-reader-b]
          due_at_overridden: false
          unlock_at_overridden: false
          lock_at_overridden: false
        outcome:
          status: succeeded
          id: 801
    visibility:
      status: preserved
    refresh:
      remove: [s-reader-a]
      retain: [s-reader-b]
      changes:
        - operation: update
          override_id: 801
          before:
            title: Book club individual audience
            student_refs: [s-reader-a, s-reader-b]
            due_at_overridden: false
            unlock_at_overridden: false
            lock_at_overridden: false
          student_refs: [s-reader-b]
          remove_refs: [s-reader-a]
---
Synthetic Engine-prepared refresh output. Agents use prepare-audience --refresh to generate the real snapshot and returned action path; teachers do not edit this state.

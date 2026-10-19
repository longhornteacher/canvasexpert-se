---
schema: 1
kind: assignment-dates
operation: update
course: ELA7
instance: https://canvas.example.edu
course_id: 1001
assignment_id: 8001
only_visible_to_overrides: false
observed_only_visible_to_overrides: false
effective_dates:
  - base: true
    due_at: "2026-11-10T23:59:00-06:00"
  - override_id: 9001
    group_id: 701
    due_at: "2026-11-12T23:59:00-06:00"
overrides:
  - key: override-9001
    operation: update
    override_id: 9001
    group_id: 701
    due_at:
      set: true
      "null": true
    observed:
      group_id: 701
      due_at: "2026-11-12T23:59:00-06:00"
---

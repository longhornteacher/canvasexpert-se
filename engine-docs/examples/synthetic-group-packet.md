---
schema: 1
kind: group-packet
course: BOOKCLUB
pulled_at: 2026-10-17T17:00:00Z
assignment:
  instance: https://canvas.example.edu
  course_id: 42
  assignment_id: 9001
  title: Shared source analysis
  points: 20
  description_html: <p>Build a shared evidence-based analysis.</p>
  submission_types:
    - online_text_entry
  published: true
  grading_type: points
  anonymous_grading: false
  moderated_grading: false
  group_category_id: 700
  grade_group_students_individually: false
groups:
  - group_id: 701
    name: Cedar Readers
    path: courses/BOOKCLUB/assignments/9001-pulled/groups/701.md
    evidence_status: complete
  - group_id: 702
    name: Birch Readers
    path: courses/BOOKCLUB/assignments/9001-pulled/groups/702.md
    evidence_status: partial
    reasons:
      - one attachment could not be read
unassigned:
  - student_ref: stu_example
    alias: Learner A
    group_identity_status: currently-unassigned
    state: no submission
    missing: false
    excused: false
    late: false
    entered_grade_known: false
  - group_identity_status: unknown
    state: group identity unknown
    submission_id: 8103
    attempt: 1
    missing: false
    excused: false
    late: false
    entered_grade_known: false
---
Canvas assignment snapshot and group evidence paths for one pull. All IDs and student refs are synthetic.

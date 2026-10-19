---
schema: 1
kind: group-results
packet: courses/BOOKCLUB/assignments/9001-pulled/group-packet.md
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
interpretation:
  mapping: repeated
  rationale: One shared product is read once against the shared criteria.
  components:
    - key: interpretation
      label: Interpretation
      weight: 1
scoring_basis:
  title: Club analysis
  use_for_grading: false
  criteria:
    - id: evidence
      description: Uses relevant evidence
      points: 100
groups:
  - group_id: 701
    artifact_key: artifact-0123456789abcdef
    member_refs:
      - stu_example_a
      - stu_example_b
    operation: grade
    grade:
      components:
        - key: interpretation
          evidence: present
          criteria:
            - criterion_id: evidence
              points: 80
      assessment: 16
      feedback_audience: group
      feedback: Your interpretation connects a clear claim to relevant evidence.
    individual_feedback:
      - student_ref: stu_example_a
        text: Your reflection names a useful next question for the club.
---
Teacher-facing working notes are not posted.

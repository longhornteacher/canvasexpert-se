---
schema: 1
kind: group-work
instance: https://canvas.example.edu
course: BOOKCLUB
course_id: 42
assignment_id: 9001
group_category_id: 700
group_id: 701
group_name: Cedar Readers
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
pulled_at: 2026-10-17T17:00:00Z
roster_complete: true
membership:
  - student_ref: stu_example_a
    alias: Learner A
    state: accepted
  - student_ref: stu_example_b
    alias: Learner B
    state: accepted
propagation_audience:
  - stu_example_a
  - stu_example_b
propagation_audience_complete: true
shared_assessment_status: held
shared_assessment_reasons:
  - members have different current artifacts or attempt numbers
members:
  - student_ref: stu_example_a
    alias: Learner A
    current_member: true
    membership_state: accepted
    submission_id: 8101
    submission_group_id: 701
    attempt: 2
    submitted_at: 2026-10-16T18:00:00Z
    current_artifact_key: artifact-abc123
    workflow_state: submitted
    missing: false
    excused: false
    late: false
    entered_grade_known: true
    entered_grade: 15
  - student_ref: stu_example_b
    alias: Learner B
    current_member: true
    membership_state: accepted
    submission_id: 8102
    submission_group_id: 701
    attempt: 1
    submitted_at: 2026-10-16T17:00:00Z
    current_artifact_key: artifact-def456
    workflow_state: submitted
    missing: false
    excused: false
    late: false
    entered_grade_known: true
    entered_grade: 12
artifacts:
  - key: artifact-abc123
    fingerprint: 00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff
    submission_type: online_text_entry
    body_html: <p>Claim supported by two passages.</p>
    readable_text: Claim supported by two passages.
    sources:
      - student_ref: stu_example_a
        submission_id: 8101
        submission_group_id: 701
        attempt:
          attempt: 2
          submitted_at: 2026-10-16T18:00:00Z
  - key: artifact-def456
    fingerprint: ffeeddccbbaa99887766554433221100ffeeddccbbaa99887766554433221100
    submission_type: online_text_entry
    body_html: <p>Claim supported by one passage.</p>
    readable_text: Claim supported by one passage.
    sources:
      - student_ref: stu_example_b
        submission_id: 8102
        submission_group_id: 701
        attempt:
          attempt: 1
          submitted_at: 2026-10-16T17:00:00Z
---
Separate member identities remain visible even when Canvas provides different current submissions. The content and identities here are synthetic.

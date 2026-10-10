---
schema: 1
kind: classic-quiz-results
packet: engine-docs/examples/classic-writing-packet.md
assignment:
  instance: https://canvas.example
  course_id: 1
  assignment_id: 100
  title: Synthetic mixed Classic quiz
  points: '25'
  description_html: '<p>Choose, explain, and upload.</p>'
  submission_types: [online_quiz]
  due_at: '2026-10-08T12:00:00Z'
  published: false
  grading_type: points
  anonymous_grading: false
  moderated_grading: false
quiz_id: 200
scoring_notes: >-
  Assess each explanation against the prompt's claim and evidence requirements;
  each question uses its own Canvas maximum. Objective points remain Canvas-owned.
students:
  - student_ref: s-synthetic-classic
    attempt:
      attempt: 2
      submitted_at: '2026-10-09T12:00:00Z'
    strongest_prior:
      attempt:
        attempt: 1
        submitted_at: '2026-10-01T12:00:00Z'
      basis: The earlier explanation connects evidence to the claim more clearly.
    questions:
      - question_id: 502
        evidence: present
        score: '7.5'
        comment: Your detail is relevant. Explain how helping others supports your claim.
      - question_id: 503
        evidence: missing
        score: '0'
        comment: This required reflection is blank. Add a second detail and explain its relevance.
      - question_id: 504
        evidence: unknown
        teacher_note: The uploaded file is unavailable; inspect the original before assessing it.
    late_timing:
      status: late
      seconds_late: 86400
      due_at: '2026-10-08T12:00:00Z'
      basis: Synthetic timing supplied by the agent; not an Engine calendar calculation.
---
Synthetic local draft. The unknown upload previews as held while the assessed essays are ready.

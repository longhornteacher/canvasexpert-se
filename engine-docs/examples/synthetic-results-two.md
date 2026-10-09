---
schema: 1
kind: results
packet: packets/synthetic-evidence-22.md
assignment:
  instance: https://canvas.example.edu
  course_id: 11
  assignment_id: 22
  title: Synthetic two-response task
  points: 20
  description_html: "<p>Synthetic directions: answer both required prompts.</p>"
  submission_types: [online_text_entry]
  published: true
  grading_type: points
  anonymous_grading: false
  moderated_grading: false
  rubric:
    id: 33
    association_id: 44
    title: Synthetic rubric
    use_for_grading: true
    use_for_grading_known: true
    criteria:
      - {id: evidence, description: Evidence, points: 4}
      - {id: reasoning, description: Reasoning, points: 6}
interpretation:
  mapping: repeated
  rationale: "Synthetic: the two required prompts have equal weight."
  components:
    - {key: prompt_a, label: Prompt A, weight: 1}
    - {key: prompt_b, label: Prompt B, weight: 1}
students:
  - student_ref: synthetic-student-01
    operation: grade
    grade:
      attempt: {attempt: 1, submitted_at: "2026-01-01T12:00:00Z"}
      components:
        - key: prompt_a
          evidence: present
          criteria: [{criterion_id: evidence, points: 3}, {criterion_id: reasoning, points: 4}]
        - key: prompt_b
          evidence: missing
          criteria: [{criterion_id: evidence, points: 0}, {criterion_id: reasoning, points: 0}]
      assessment: 7
      feedback: "Synthetic feedback grounded in submitted work."
---

Synthetic result fixture.

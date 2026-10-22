---
schema: 1
kind: results
packet: packets/synthetic-research-44.md
# Normally omit the `assignment:` block below: the Engine fills it from the packet. It is shown so this example validates on its own.
assignment:
  instance: https://canvas.example.edu
  course_id: 11
  assignment_id: 44
  title: Synthetic four-response task
  points: 20
  description_html: "<p>Synthetic directions: answer all four required prompts.</p>"
  submission_types: [online_text_entry]
  published: true
  grading_type: points
  anonymous_grading: false
  moderated_grading: false
  rubric:
    id: 55
    association_id: 66
    title: Synthetic rubric
    use_for_grading: true
    use_for_grading_known: true
    criteria:
      - {id: evidence, description: Evidence, points: 4}
      - {id: reasoning, description: Reasoning, points: 6}
interpretation:
  mapping: repeated
  rationale: "Synthetic: the assignment expectations support weights 1, 1, 2, and 1."
  components:
    - {key: prompt_a, label: Prompt A, weight: 1}
    - {key: prompt_b, label: Prompt B, weight: 1}
    - {key: prompt_c, label: Prompt C, weight: 2}
    - {key: prompt_d, label: Prompt D, weight: 1}
students:
  - student_ref: synthetic-student-02
    operation: grade
    grade:
      attempt: {attempt: 1, submitted_at: "2026-01-02T12:00:00Z"}
      components:
        - key: prompt_a
          evidence: present
          criteria: [{criterion_id: evidence, points: 4}, {criterion_id: reasoning, points: 6}]
        - key: prompt_b
          evidence: present
          criteria: [{criterion_id: evidence, points: 3}, {criterion_id: reasoning, points: 5}]
        - key: prompt_c
          evidence: present
          criteria: [{criterion_id: evidence, points: 2}, {criterion_id: reasoning, points: 3}]
        - key: prompt_d
          evidence: present
          criteria: [{criterion_id: evidence, points: 4}, {criterion_id: reasoning, points: 5}]
      assessment: 14.8
      feedback: "Synthetic feedback grounded in all four responses."
---

Synthetic result fixture.

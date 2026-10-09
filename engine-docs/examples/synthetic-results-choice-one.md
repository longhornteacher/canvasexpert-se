---
schema: 1
kind: results
packet: packets/synthetic-choice-77.md
assignment:
  instance: https://canvas.example.edu
  course_id: 11
  assignment_id: 77
  title: Synthetic choice task
  points: 10
  description_html: "<p>Synthetic directions: choose and answer one of four options.</p>"
  submission_types: [online_text_entry]
  published: true
  grading_type: points
  anonymous_grading: false
  moderated_grading: false
  rubric:
    id: 88
    association_id: 99
    title: Synthetic rubric
    use_for_grading: true
    use_for_grading_known: true
    criteria:
      - {id: evidence, description: Evidence, points: 4}
      - {id: reasoning, description: Reasoning, points: 6}
interpretation:
  mapping: repeated
  rationale: "Synthetic: the directions require one selected option, so the selected response is the scored unit."
  components:
    - {key: selected_option, label: Selected option, weight: 1}
students:
  - student_ref: synthetic-student-03
    operation: grade
    grade:
      attempt: {attempt: 1, submitted_at: "2026-01-03T12:00:00Z"}
      components:
        - key: selected_option
          evidence: present
          criteria: [{criterion_id: evidence, points: 3}, {criterion_id: reasoning, points: 4}]
      assessment: 7
      feedback: "Synthetic feedback on the option the student selected."
---

Synthetic result fixture.

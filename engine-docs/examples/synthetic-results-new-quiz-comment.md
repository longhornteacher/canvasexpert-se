---
schema: 1
kind: results
packet: courses/SYNTHETIC/assignments/22/packet.md
# Normally omit the `assignment:` block below: the Engine fills it from the packet. It is shown so this example validates on its own.
assignment:
  instance: https://canvas.example.edu
  course_id: 11
  assignment_id: 22
  title: Synthetic New Quiz written response
  points: 20
  description_html: "<p>Synthetic quiz with auto-scored items and one written response.</p>"
  submission_types: [external_tool]
  published: true
  grading_type: points
  anonymous_grading: false
  moderated_grading: false
  new_quiz: true
scoring_basis:
  title: Synthetic written-response criteria
  criteria:
    - {id: evidence, description: Evidence, points: 4}
    - {id: reasoning, description: Reasoning, points: 6}
interpretation:
  mapping: repeated
  rationale: "Synthetic: assess the one required written response; auto-scored quiz items are outside this comment."
  components:
    - {key: written_response, label: Written response, weight: 1}
students:
  - student_ref: synthetic-student-01
    operation: comment-only
    comment_only:
      attempt: {attempt: 1, submitted_at: "2026-01-01T12:00:00Z"}
      components:
        - key: written_response
          evidence: present
          criteria: [{criterion_id: evidence, points: 3}, {criterion_id: reasoning, points: 4}]
      feedback: "Synthetic feedback grounded in the written response."
---

Synthetic result fixture. Replace all identity, attempt and packet fields with the actual pull before teacher use. The Engine renders criterion scores without an aggregate quiz assessment and posts only a submission comment. The teacher enters per-item points in SpeedGrader.

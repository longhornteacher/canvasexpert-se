---
schema: 1
kind: results
packet: courses/SYNTHETIC/assignments/22/packet.md
# results-init supplies this assignment snapshot. Handwritten files may omit it; the Engine resolves it from the packet in memory without changing the draft.
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
    - {key: written_response, label: Written response, weight: 1, item_id: "8806218"}
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

Synthetic result fixture. Replace all identity, attempt and packet fields with the actual pull before teacher use. The Engine leads with the auto-scored subtotal recorded in the pulled work (for example `Questions Canvas scored: 14/20`) and the written response in its quiz points from `item_id` (for example `Written response: 10.5/15`), then the rubric lines, without an aggregate quiz assessment, and posts only a submission comment. The teacher enters per-item points in SpeedGrader.

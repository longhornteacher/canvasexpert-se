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
      item_scores: true
      components:
        - key: written_response
          evidence: present
          criteria: [{criterion_id: evidence, points: 3}, {criterion_id: reasoning, points: 4}]
          feedback: "Synthetic strength in the response. Next, a synthetic next step."
          exemplar: "Synthetic short model answer to hand-copy."
      feedback: ""
---

Synthetic result fixture. Replace all identity, attempt and packet fields with the actual pull before teacher use. With `item_scores: true` the Engine writes the written item's points on its quiz scale from `item_id` (here 7/10 on the rubric, so 10.5 of a 15-point item) and the part's `feedback` as that item's comment, with the hand-copy `exemplar` because the part earned under 90%. The closing `feedback` is empty, so no whole-assignment comment is posted, and Canvas computes the quiz total. Without `item_scores` (a part that cannot name one item, such as a choice among prompts), the Engine posts one whole-assignment comment leading with the auto-scored subtotal and each part's item points, and the teacher enters those points in SpeedGrader.

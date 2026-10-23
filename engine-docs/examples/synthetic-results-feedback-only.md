---
schema: 1
kind: results
packet: courses/SYNTHETIC/assignments/22/packet.md
# results-init supplies this snapshot. Handwritten files may omit it; it is resolved in memory.
assignment:
  instance: https://canvas.example.edu
  course_id: 11
  assignment_id: 22
  title: Synthetic reading response
  points: 20
  submission_types: [online_text_entry]
  grading_type: points
students:
  - student_ref: synthetic-student-01
    operation: comment-only
    comment_only:
      attempt: {attempt: 1, submitted_at: "2026-10-23T12:00:00Z"}
      header: none
      feedback: "Explain how your second detail supports the claim."
---

Synthetic feedback without scores. No rubric, scoring_basis, interpretation or
components are needed. Review shows the exact comment; posting writes only this
attempt's comment. Student identity is resolved privately by the Engine from the
pulled packet's student_ref. Replace the synthetic packet, snapshot, student_ref
and attempt with the actual pulled values. A feedback-only comment does not mark
New Quiz written work as scored in later session discovery.

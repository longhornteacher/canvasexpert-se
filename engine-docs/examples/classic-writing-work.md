---
schema: 1
kind: classic-quiz-work
student_ref: s-synthetic-classic
alias: Bulbasaur
assignment_id: 100
quiz_id: 200
submission_id: 300
quiz_submission_id: 400
current:
  ref:
    attempt: 2
    submitted_at: '2026-10-09T12:00:00Z'
  late: true
  effective_due_at: '2026-10-08T12:00:00Z'
  canvas_score: '5'
  questions:
    - question_id: 501
      position: 1
      name: Objective choice
      question_type: multiple_choice_question
      prompt_html: '<p>Choose the supported claim.</p>'
      prompt_text: Choose the supported claim.
      points_possible: '5'
      response_present: true
      observed_score: '5'
    - question_id: 502
      position: 2
      name: Explain evidence
      question_type: essay_question
      prompt_html: '<p>Explain why the quotation supports your claim.</p>'
      prompt_text: Explain why the quotation supports your claim.
      points_possible: '10'
      response_present: true
      response_html: '<p>The quotation shows the character helping others.</p>'
      response_text: The quotation shows the character helping others.
    - question_id: 503
      position: 3
      name: Required reflection
      question_type: essay_question
      prompt_html: '<p>Explain a second detail.</p>'
      prompt_text: Explain a second detail.
      points_possible: '5'
      response_present: true
    - question_id: 504
      position: 4
      name: Upload revision
      question_type: file_upload_question
      prompt_html: '<p>Upload your revision.</p>'
      prompt_text: Upload your revision.
      points_possible: '5'
      response_present: true
      attachments:
        - id: 601
          name: revision.docx
          status: failed
          reason: Synthetic unavailable file; no contents invented.
prior:
  - ref:
      attempt: 1
      submitted_at: '2026-10-01T12:00:00Z'
    late: false
    questions:
      - question_id: 502
        position: 2
        name: Explain evidence
        question_type: essay_question
        prompt_html: '<p>Explain why the quotation supports your claim.</p>'
        prompt_text: Explain why the quotation supports your claim.
        points_possible: '10'
        response_present: true
        response_html: '<p>The character helps a neighbor, supporting the claim of generosity.</p>'
        response_text: The character helps a neighbor, supporting the claim of generosity.
unhandled_question_ids: [502, 503, 504]
monitored: false
notes:
  - Private synthetic context, never copy into student feedback.
---
Synthetic response evidence. There are no Canvas credentials or private student mappings in these examples.

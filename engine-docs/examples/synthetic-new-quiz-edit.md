---
schema: 1
kind: new-quiz-edit
course: CS8
instance: https://canvas.example.edu
course_id: 900001
assignment_id: 900002
observed:
  instance: https://canvas.example.edu
  course_id: 900001
  assignment_id: 900002
  fields:
    title: Practice
    instructions: '<p>Read the handout.</p>'
changes:
  instructions: '<p>Read the revised handout.</p>'
items:
  - item_id: '900003'
    entry_id: '900004'
    position: 1
    entry_type: Item
    interaction_type_slug: essay
    editable: true
    observed:
      item_id: '900003'
      entry_id: '900004'
      entry_type: Item
      interaction_type_slug: essay
      fields:
        title: Response
        item_body: '<p>Explain.</p>'
        feedback:
          neutral: ''
          correct: null
    changes:
      item_body: '<p>Explain using one example.</p>'
---
Synthetic example only. Pull the real quiz to obtain its current identity and baseline.

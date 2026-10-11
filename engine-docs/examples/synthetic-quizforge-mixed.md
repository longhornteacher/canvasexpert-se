Synthetic MC / TF / ESSAY New Quiz. This envelope can be saved directly as a
course draft; its default is unpublished. Written responses are teacher graded.

<QUIZFORGE_JSON>
{
  "quizforge_version": "3.0-json",
  "title": "Synthetic reading check",
  "instructions": "<p>Read each question and explain your thinking.</p>",
  "metadata": {"canvas": {"published": false}},
  "items": [
    {"id": "q1", "type": "MC", "points": 2,
     "prompt": "<p>Which detail describes a setting?</p>",
     "choices": [
       {"id": "a", "text": "The quiet library at sunset", "correct": true},
       {"id": "b", "text": "The character decides to apologize", "correct": false}
     ]},
    {"id": "q2", "type": "TF", "points": 1,
     "prompt": "<p>A setting includes when and where events happen.</p>",
     "answer": true},
    {"id": "q3", "type": "ESSAY", "points": 5,
     "prompt": "<p>Describe how a setting can influence a character's decision.</p>",
     "word_limit_max": 150,
     "rubric_hint": "Assess a clear explanation and a relevant example.",
     "notes": "Teacher grades this response in SpeedGrader."}
  ],
  "rationales": [
    {"item_id": "q1", "choices": [
      {"id": "a", "rationale": "A library at sunset describes a place and time."},
      {"id": "b", "rationale": "A decision is an action rather than a setting."}
    ]},
    {"item_id": "q2", "rationale": "Setting gives the time and place of events."}
  ]
}
</QUIZFORGE_JSON>

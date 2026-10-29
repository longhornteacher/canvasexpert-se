Synthetic fixtures for the New Quiz item-score write (`POST {quiz_api_host}/api/quiz_sessions/{id}/results`), shaped from the CS8 capture of 2026-10-29 recorded in `handoffs/active/HANDOFF_Quiz_Accommodation_Writes_and_New_Quiz_Item_Scores_2026-10-28.md` §3.1. All IDs, text, names and URLs are made up.

- `before_session_item_results.json`: the attempt's current item results as read (snake_case).
- `post_results_request.json`: the body SpeedGrader sends to change item 1002 from 4 to 5 with an item comment and 0.5 fudge points: every item, camelCase at every level, `errors: {}` and `graderId: null` added, `grading_method` dropped, `gradedAt` copied from the read.
- `post_results_response.json`: the 201 reply; `id` is the new result version. Its `quiz_session.metadata` carries the student's real name in Canvas, so never log or store it.

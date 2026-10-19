---
schema: 1
# Each command is a card on the WebUI Home page. Choosing it shows the prompt with a Copy button; the teacher pastes it into their teaching app.
# {course} becomes the course picked on Home, or "one of my courses" when none is picked.
commands:
  - name: Scoring session
    about: Score submitted work, review it, then post.
    prompt: "Start a scoring session for {course}."
  - name: Planning session
    about: Build pages and assignments for the days ahead.
    prompt: "Start a planning session for {course}."
  - name: Build a quiz
    about: A New Quiz, drafted for your review first.
    prompt: "Help me build a New Quiz for {course}."
  - name: Parent update
    about: Draft notes home. You read every one before it sends.
    prompt: "Draft parent updates for {course}."
  - name: Fix in Canvas
    about: Change a page, due date or assignment that's already live.
    prompt: "I need to change something that's already live in Canvas for {course}."
---

Home page commands. When the teacher asks to add, change, reorder or remove one, edit the list above. Keep each prompt to one plain sentence that names the task; the playbooks carry the judgment, so a prompt never needs to explain how to do the work. Keep `{course}` where a course belongs.

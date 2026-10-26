---
schema: 1
# Each command is a card on the WebUI Home page. Choosing it shows the prompt with a Copy button; the teacher pastes it into their teaching app.
# {course} becomes the course picked on Home, or "one of my courses" when none is picked.
commands:
  - name: Scoring session
    about: Score submitted work, hear how it went, then post.
    prompt: "Start a scoring session for {course}."
  - name: Planning session
    about: Build pages and assignments for the days ahead.
    prompt: "Start a planning session for {course}."
  - name: Build a quiz
    about: A New Quiz, drafted and talked through before it goes in.
    prompt: "Help me build a New Quiz for {course}."
  - name: Grade update home
    about: Each family gets their child's grades. You read every one before it sends.
    prompt: "Draft parent updates for {course}."
  - name: Email families
    about: Send a reminder or class news home. You read every email before it sends.
    prompt: "Help me email families in {course} about an upcoming activity or reminder."
  - name: Print grade reports
    about: One page per student, for conferences or take-home folders.
    prompt: "Print student grade reports for {course}."
  - name: Book club unit
    about: Set up clubs, club pages and discussions in Canvas.
    prompt: "Help me set up a book club unit for {course}."
  - name: Fix in Canvas
    about: Change a page, due date or assignment that's already live.
    prompt: "I need to change something that's already live in Canvas for {course}."
---

Home page commands. When the teacher asks to add, change, reorder or remove one, edit the list above. Keep each prompt to one plain sentence that names the task; the playbooks carry the judgment, so a prompt never needs to explain how to do the work. Keep `{course}` where a course belongs.

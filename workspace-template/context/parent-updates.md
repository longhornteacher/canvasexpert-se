---
schema: 1
# Your school's family grade portal. The agent links to it in parent emails for the official course grade; the printed grade sheet names it. (Older key name skyward_url still works.)
grade_portal_url: ""
grade_portal_name: ""
# Optional tutoring or clubs sign-up the agent links at the end of parent emails. Leave the address blank to leave it out. (Older bobcat_hour_* key names still work.)
tutoring_url: ""
tutoring_label: ""
tutoring_text: ""
# Optional page that helps families create a Parent Canvas account and pair with their student (often a district-run course page). The agent links it in parent emails. Leave blank to leave it out.
canvas_parent_url: ""
canvas_parent_text: ""
# The class name families know, for example "Science 6", used in the email's wording.
class_name: ""
# Assignments below this percent are listed in grade updates (the agent passes it as below_percent).
failing_below: 70
# How the email closes, for example "Your teacher". The Engine adds it to every family email.
sign_off: ""
# Older settings, no longer used by grade updates.
skip_score: 90
---

Parent-update rules (teacher instruction, updated 2026-11-15; all courses):

- Use the full requested term through the report date. The below-70 assignment list is not limited to the correction window.
- List every published, assigned, required assignment whose current released score is below 70%. Exactly 70% is excluded. Use the Engine's percentage calculation and latest released grade; never manufacture a zero.
- Purpose: the email sends the student back to Canvas to own the next steps. The student looks through each linked assignment, does what they know they can do at home, and brings a list of questions and needs to tutorials. The links are for easy access, not a promise that anything can be redone. Say in general terms that some work is locked at its grade and other work might have corrections or late turn-in available; never say which. Do not offer to make a plan together, schedule a meeting or invite a reply.
- The table shows only Assignment (a direct Canvas link) and Score. Due date and turned-in status are not shown unless the teacher asks.
- After the table, link with a short description each: the tutoring sign-up, the grade portal for official grades, and the Parent Canvas account page when saved. Before the sign-off, say that before attending tutorials the student must already know which assignments they have questions about and must have already begun the work they know they need to do, and that responsibility for the work belongs to the student.
- Omit AI disclaimers, Pokémon/alias names and alias/privacy explanations from family-facing output. Internal alias protection remains in place.
- No Corrections available section, per-assignment eligibility, correction deadlines or promises of points back.
- Keep the configured links and the teacher's sign-off. If the teacher has saved Student Handbook text below, include it verbatim after the responsibility paragraph and before the closing; save it in your own context file, not in the shipped seed. The closing and sign-off stay the last lines of the body, because the Engine adds the sign-off itself unless the body already ends with it.
- A course average of 90 or more does not suppress a below-70 assignment. Send when the term list is nonempty; skip an empty list. Students with incomplete evidence are held for the teacher. Do not label an overall Canvas course score as a term grade.

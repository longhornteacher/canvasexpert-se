---
schema: 1
# Your school's family grade portal. The agent links to it in parent emails for the official course grade; the printed grade sheet names it. (Older key name skyward_url still works.)
grade_portal_url: ""
grade_portal_name: ""
# Optional tutoring or clubs sign-up the agent links at the end of parent emails. Leave the address blank to leave it out. (Older bobcat_hour_* key names still work.)
tutoring_url: ""
tutoring_label: ""
tutoring_text: ""
# The class name families know, for example "Science 6", used in the email's wording.
class_name: ""
# Assignments below this percent are listed in grade updates (the agent passes it as below_percent).
failing_below: 70
# How the email closes, for example "Your teacher". The Engine adds it to every family email.
sign_off: ""
# Older settings, no longer used by grade updates.
skip_score: 90
---

Parent-update rules (teacher instruction, 2026-11-07; all courses):

- Use the full requested term through the report date. The below-70 assignment list is not limited to the correction window.
- List every published, assigned, required assignment whose current released score is below 70%. Exactly 70% is excluded. Use the Engine's percentage calculation and latest released grade; never manufacture a zero.
- Each row has the assignment title as a direct Canvas link, its score, the student-effective due date, and an independent submission status: Turned in, Not turned in, No online submission required, or Status unavailable. A zero or absence of an upload alone does not prove that work was not turned in.
- Keep the message focused on this list. Omit AI disclaimers, Pokémon/alias names and alias/privacy explanations from family-facing output. Internal alias protection remains in place.
- Omit the Corrections available section and all correction/retake eligibility announcements or deadlines. Listing a low grade makes no promise of a revision or points back.
- Keep the configured grade-portal and tutoring links and the teacher's sign-off.
- A course average of 90 or more does not suppress a below-70 assignment. Send when the term list is nonempty; skip an empty list. Students with incomplete evidence are held for the teacher. Do not label an overall Canvas course score as a term grade.

---
schema: 1
# Your school's family grade portal. Parent emails link to it for the official course grade. Leave the address blank to show the name without a link.
skyward_url: ""
grade_portal_name: ""
# Optional tutoring or clubs sign-up shown in every email footer. Leave the address blank to leave it out.
bobcat_hour_url: ""
bobcat_hour_label: ""
bobcat_hour_text: ""
# How the email closes, for example "Mr. Beckham".
sign_off: ""
# Every included Canvas course score at or above this, with no resolved Missing/0 items, means no email is needed.
skip_score: 90
---

Parent update preferences. The Engine reads the settings above when it prepares a report and renders an email; the notes below are for the agent.

Tone notes for this teacher (edit freely): short, warm, plain. Lead with what the family can act on. Never scold, and never ask parents to reply.

Standing parent-update rules:

- A current teacher-entered grade above zero overrides stale Canvas missing status or no online submission. Do not call that work missing or hold its email for that conflict.
- A current recorded zero belongs first under Missing/0, including graded or submitted work and items older than the correction window. A zero does not prove work was never submitted; an earlier zero raised to a positive grade is history only.
- Do not repeat Missing/0 items under Corrections. Keep their grade-history entries.
- Use the Engine's resolved `missing` list and `skip_eligible`. Apply the skip rule after classification: skip only when every included course meets `skip_score` (90 by default) and no Missing/0 items remain. Other incomplete-data or contact problems still need their own resolution.
- Reports include required work whose student-effective due date falls in the last 14 calendar days of the current grading period, clipped to the period. The Engine selects the period from `context/school-calendar.md`; retain all grading periods when updating that calendar.
- If `work_due` is empty, do not use a low current course score or Due Soon alone to create an email. Skip with the reason “No required work was due in this report window.” unless incomplete evidence requires a flag.
- The Engine renders every saved `work_due` row in email and printable reports. Keep that complete inventory even when selecting Missing/0, correction or history references for agent notes.

---
schema: 1
voice: ""
feedback_guidance: ""
---

Starter file: no teacher-specific preferences are configured.

Add feedback voice, rubric-use preferences or a teacher-established general Extra time flag policy here when needed. An Extra time checkbox supplies no duration or waiver by itself; document the allowance and how it applies to deadlines in plain language. Put individual accommodations and student-specific notes in `students.md`; a documented individual rule takes precedence, including when the flag is false. Do not use this file to store a Canvas token or private runtime mapping.

To give new assignments a grading timetable, add a `late_work` block to the settings above, counted in school days (weekdays that are not closures in `school-calendar.md`):

```yaml
late_work:
  correction_school_days: 4    # last day for corrections, after the due date
  close_after_school_days: 11  # the assignment locks this many school days after the due date
  extra_time_school_days: 1    # the general extra-time allowance shown as "(extra time: ...)"
```

Leave out a line to leave that row out. The late penalty and lowest late grade come from the course's Canvas late policy.

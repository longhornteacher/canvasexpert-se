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
  close_after_school_days: 11  # the assignment locks this many school days after the due date
  extra_time_school_days: 1    # the general extra-time allowance shown as "(extra time: ...)"
  late_cutoff_school_days: 3   # late work is accepted this many school days after the due date; 0 = none; leave out for no limit
```

A course can carry its own `late_work` lines in `context/courses.md`, or in the Courses page's "Late-work limit" box; a line set there wins over this file for that course (for example a school policy of 3 days in ELA7 and 0 in a high-school-credit course). With a limit, new assignments lock at the cutoff, and the timetable says "Late work accepted through ..." (or "not accepted after the due date"). Work never turned in gets its zero from Canvas's missing-work policy, which the Engine does not check.

Extra time can be a whole number of days or, on the Students page, a percent of the school days between the day the work was assigned (Canvas "available from", else the day it was created) and the due date, rounded up. A student has days or a percent, not both. The Engine gives such a student a personal Canvas due date and lock date (`personal_dates`), so the cutoff does not lock them out early.

`extra_time_school_days` is also the allowance used to correct late marks for a student with Extra time ticked and no number of their own. A number typed beside a student's Extra time box on the Students page wins for that student.

The Major tutorial retake is created privately during scoring and needs no settings. To set its release terms for later, teacher-directed assignment and credit, add a `retakes` block to the settings above:

```yaml
retakes:
  cap_percent: 70   # a retake adds at most this percent to the original, and a student below it is eligible
  school_days: 4    # the retake window, counted in school days from the day the grade was posted
```

`retakes` supplies eligibility, window and cap when the teacher later directs assigning a retake and applying its credit. A missing block does not stop private creation and is not a reason to ask for release terms during routine scoring; with none, assign and apply report that no retake policy is set and nothing is assumed.

Reopening work for corrections is off unless the teacher selects it. To allow it, the teacher sets `correction_school_days` (the last day for corrections, counted in school days after the due date) in `late_work`, here or per course. A course line of `correction_school_days: 0` in `context/courses.md` turns off a window set here for that course. There is no fallback: with no value, no correction window exists, none is shown in the timetable, and nothing is reopened.

Leave out a line to leave that row out. The late penalty and lowest late grade come from the course's Canvas late policy.

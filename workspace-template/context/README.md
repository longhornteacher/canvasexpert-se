# Reusable teacher context

Your agent builds these notes during ordinary work. There is no form to complete first. `playbooks/teacher-context.md` explains what to remember, how to keep each choice in scope, and how to calibrate written-response exemplars.

- `Teacher_Override.md` at the active rules root: your standing changes to shipped defaults.
- `teaching-profile.md` and `grading.md`: general teaching preferences and feedback style.
- `courses.md`: the Engine's course selections and Canvas origin.
- `courses/<course-id>/notes.md`: habits, taught skills and expectations for one course.
- `courses/<course-id>/assignments/<assignment-id>.md`: reusable expectations and authored exemplars for one Canvas assignment or New Quiz's associated assignment.

See `courses/README.md` for the course and assignment outlines. Create actual notes only when there is useful information, using exact Canvas IDs and recording the Canvas origin. Unknown details stay unspecified.

These paths use UserInstall by default and configured UserSync when selected. Course work, drafts, student submissions, results and session records stay in UserInstall. In a git install the playbooks and standing instructions also stay in UserInstall, supplied by updates; teacher context and overrides follow UserSync. The provider handles delivery between PCs.

---
schema: 1
instance: https://canvas.example.edu
aliases: true
courses: []
---

Starter file: no courses configured.

Use `.\cse.cmd courses` to discover IDs, then edit this file to configure nicknames. The instance must be the HTTPS Canvas origin. Each course entry uses `nickname`, `id`, `current`, and optional IANA `timezone`. A course may add its own `late_work` lines (for example `late_work: {late_cutoff_school_days: 3}`; 0 means late work is not accepted) which win over `context/grading.md` for that course; the Courses page "Late-work limit" box saves the same line. Do not infer nickname mappings from course titles or put student identity mappings here.

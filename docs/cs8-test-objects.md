# CS8 test objects awaiting cleanup

Disposable objects that development runs left in CS8 (course 121046). IDs are cleanup references; read current live state before acting. Canvas may refuse to unpublish an assignment that has submissions, so those need deleting in Canvas. The class's real quizzes and coursework were never touched.

Published objects can be visible to enrolled students; clean those up first.

| Canvas object | State left | From |
|---|---|---|
| Assignment 3698077 (plain) | **Published**; Test Student attempts 1-4 | Engine unattended run, 2026-10-09 |
| Assignment 3698078 / rubric 71830 / association 130470 | **Published**; Test Student attempts; rubric coupling enabled | Engine unattended run |
| Assignment 3698079 / rubric 71829 / association 130469 / file 19221778 | **Published**; Test Student file attempt | Engine unattended run |
| Assignment 3698080 / rubric 71831 (ratingless diagnostic) | Unpublished | Engine unattended run |
| Assignment 3698081 / rubric 71832 (numeric anchors), module item 9025926 | Unpublished | Engine unattended run |
| Assignment 3698082 / rubric 71833 (attachment anchors), module item 9025927 | Unpublished | Engine unattended run |
| Module 636143 | Unpublished; contains 3698081 and 3698082 | Engine unattended run |
| Assignment 3698181 | **Published**; Test Student score 16 and comment; Canvas refused unpublish (HTTP 400) | Review page tests, 2026-10-10 |
| New Quiz 3698128 "All Question Types Sampler (CS)" | Unpublished; left for teacher preview | Quiz push, 2026-10-09 |
| New Quiz 3698178, assignment 3698180 | Unpublished `[CSE FIELD PROBE]` | Field API probe, 2026-10-10 |
| New Quizzes 3698182, 3698183 | Unpublished `[CSE FIELD PROBE]`; 3698183 is the optional Build/Preview check in `handoffs/retired/NEW-QUIZ-SCORING-PROBE.md` | Written-item probe, 2026-10-10 |

All Engine unattended assignments have `omit_from_final_grade: true` and `post_to_sis: false`. Remove a row when its object is deleted.

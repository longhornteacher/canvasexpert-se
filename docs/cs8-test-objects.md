# CS8 test objects awaiting cleanup

Disposable objects that development runs left in CS8. IDs are cleanup references; read current live state before acting. Canvas may refuse to unpublish an assignment that has submissions, so those need deleting in Canvas. The class's real quizzes and coursework were never touched.

Published objects can be visible to enrolled students; clean those up first.

| Canvas object | State left | From |
|---|---|---|
| Module 636143 | Unpublished; contains 3698081 and 3698082 | Engine unattended run |
| New Quiz 3698128 "All Question Types Sampler (CS)" | Unpublished; left for teacher preview | Quiz push, 2026-10-09 |
| New Quiz 3698178, assignment 3698180 | Unpublished `[CSE FIELD PROBE]` | Field API probe, 2026-10-10 |
| New Quizzes 3698182, 3698183 | Unpublished `[CSE FIELD PROBE]`; 3698183 is the optional Build/Preview check in `handoffs/retired/NEW-QUIZ-SCORING-PROBE.md` | Written-item probe, 2026-10-10 |
| Assignment 3704059 "[CSE 0.1b DISPOSABLE] 20261018T190938Z" | Published, but `only_visible_to_overrides` is true with one override (393538) for the built-in Test Student only; has a Test Student submission (late timing check); Canvas refused unpublish | Dev probe late-timing test, 2026-10-18. Delete in Canvas |

On 2026-10-17 assignments 3698077-3698082 and 3698181 were no longer found (already deleted); their rubrics may remain. The 2026-10-17 live check used disposable assignments 3703744 and 3703757, each visible only to the Test Student, and deleted both afterward.

All Engine unattended assignments have `omit_from_final_grade: true` and `post_to_sis: false`. Remove a row when its object is deleted.

2026-10-17 group-work live check (all `[CSE 1.1b DISPOSABLE]`): everything was deleted, including the Test Student discussion 304285, its assignment 3703816 and its rubric. Nothing from that run remains in CS8.

On 2026-10-28 the New Quiz content-edit Engine probe created one new unpublished disposable quiz with one essay item, changed instructions/prompt/general feedback, checked the saved fields and rerun behavior, and deleted the quiz afterward. No student attempt or existing classroom quiz was touched. Exact private IDs remain in UserSync dev-reference; no object from this probe awaits cleanup.

On 2026-10-28 the rubric-reuse/Enhanced Rubrics compatibility probe created two ordinary assignments, one unpublished New Quiz and one new library rubric. One ordinary assignment was restricted to the built-in Test Student before publication for its synthetic submission and ordinary Engine assessment/comment post. All three assignments/quizzes and the rubric were deleted afterward. No object from this probe awaits cleanup; exact IDs and the scope are in the private dev reference.

On 2026-10-28 the differentiation-tag protocol probe created disposable tag sets and a synthetic Page, and used only the built-in Test Student memberships. Bulk response wrapping was diagnosed with fresh disposable sets; every set was deleted after its run. The final probe restricted the Page to a disposable tag before publication and then observed deletion of that tag; the Page and set were deleted afterward. Exact private IDs are in dev-reference/tag-probe-20261028.json. No object from these checks awaits cleanup and no real student was re-leveled.

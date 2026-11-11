# Major retakes, Daily corrections and the honest-effort floor

Date: 2026-11-11. Status: **plan revised after review; nothing is built**. Baseline: Dev `6a2a5b4`. This revision replaces the eight-slice plan in that commit (still in git history) with four slices. It is written for further adversarial review before execution.

No release, install, UserInstall/UserSync edit, real grading, publication or Canvas Live write is authorized by this plan. Execute one slice at a time when asked.

## Decisions

All from the teacher/developer on 2026-11-11. The second column says where each lives: the **playbook** holds preferences (other teachers override them in their own `Teacher_Override.md`), the **Engine** holds mechanics.

| Decision | Lives in |
|---|---|
| Major grades: no points for hand-copying rationales or exemplars, and no immediate second submission for credit. Rich glows/grows, rationales and exemplars stay as teaching supports. | Playbook |
| Daily grades: unchanged. Rationale and exemplar writing for credit, and revision, continue. | Playbook |
| Every Major gets a parallel New Quiz named exactly `TUTORIAL-ONLY RETAKE - <original name>`: same concepts, response types and demand, different passage. | Playbook writes it; Engine creates it |
| The retake quiz starts unpublished, assigned to nobody, protected by an access code, and never counts toward the course grade. | Engine |
| A student who fails the original is assigned the retake. Failing means the entered grade, before any late deduction, is below the cap percent. | Engine |
| The retake window is 4 school days from the date Canvas posted that student's grade, not from the due date. | Engine, from config |
| A retake contributes at most 70% to the original and never lowers it. | Engine, from config |
| A late deduction on the original persists. Earned 65, late -20 shows 45; a retake raising the entered grade to 70 shows 50. | Canvas does this |
| Honest-effort floor is 40% for genuine written or creative work, Daily and Major. Multiple choice is deterministic and gets no floor. | Config value; playbook judges effort |
| On a New Quiz with at least one written item judged genuine, the floor applies to the whole quiz total, entered as fudge points. | Engine |
| No floor is written to the retake quiz itself. The floor is used only inside the retake formula. | Engine |
| The student comment records what was earned and what was entered. | Engine |

## Arithmetic

`R` is the retake's earned percent, `H` the honest-effort judgment (written retakes only), `P` the original's points, `G` the original's current entered grade before late deduction.

```text
retake_percent    = max(R, floor) if H, otherwise R
candidate_points  = P * min(retake_percent, cap) / 100
target            = max(G, candidate_points)
```

Exact rational arithmetic, rounded once at the end with the existing floor rounding. The Engine posts `target` as the entered grade and Canvas applies the existing late deduction.

| Original | Retake | Genuine | Entered | Why |
|---|---:|---|---:|---|
| 52/100 | 88/100 | - | 70/100 | Capped |
| 65/100 | 54/100 | - | 65/100 | Never lowers |
| 25/100 | 10/100 | yes | 40/100 | Floor inside the formula |
| 25/100 | 10/100 | no | 25/100 | No floor, never lowers |
| 12/30 | 18/20 | - | 21/30 | Scaled to the original's points |

## Slices

| Slice | Deliverable | Needs |
|---|---|---|
| [01](01-playbooks-and-config.md) | Major/Daily playbook policy, two config values, exemplar always shown, no corrections row on a Major | - |
| [02](02-retake-quiz-and-assignment.md) | Create the protected retake quiz; assign it to failing students with their own deadline | 01 config |
| [03](03-new-quiz-floor.md) | Floor on a New Quiz through fudge points, after one live probe | - |
| [04](04-apply-retake.md) | Apply the retake to the original: cap, never lower, comment, and a hold against later lowering | 02; 03 only for New Quiz originals |

02 and 03 are independent. 04 works for ordinary originals without 03.

## Shared rules

- **Canvas is the record.** The retake belongs to the original whose name follows the fixed prefix, in the same course. Exactly one match, or the Engine holds and names the problem. No local pairing file, so any PC can run any step.
- **One student's problem never stops the others.** A held student gets a specific reason.
- **Honest scores stay honest.** Item points, rubric points and the earned line in comments are never changed by the floor or the cap.
- **Teacher's own courses first.** In a course with an `owner` (team teaching), retake assignment and application hold with an explanation. The exact retake title has no room for the `(CE)` consent tag, and that conflict is not solved here.
- **Evidence.** Each slice updates `internal/localui/capabilities.go` and `docs/engine-capabilities.md` together, keeping local tests, Canvas Live and pending apart. Completion checks follow `handoffs/README.md`.

## Cut from the previous plan, and why

| Cut | Reason |
|---|---|
| `retake-results` document kind with saved baselines | The formula never lowers a grade, so recomputing at post time is safe. One operation with a preview replaces it. |
| Pairing sidecar, `retake_for` identity object, context-note cross-references | Title match in Canvas is enough and works across PCs. |
| Tutorial-release document with per-student eligibility records | Replaced by assigning every failing student once; the access code is the gate. |
| Due-date anchored window and its provenance record | The window starts at the posted date, which Canvas stores per student. |
| `assessment_category` field and timetable refresh on category change | The timetable cannot show a retake date any more; one authoring setting omits the corrections row. |
| Floor on multiple-choice quizzes and the auto-scored discovery sweep | Multiple choice gets no floor. |
| Floor write on the retake quiz | It does not count, so the write changes nothing. |
| `exemplar_visibility` switch | The exemplar is always shown. |
| Shared group grade floor | Reverses a decided rule for a case nobody raised. Deferred. |
| Separate integration slice and rollout slice | Tests live with each slice; adoption is the checklist below. |
| Holding the original's publication until the retake exists | The Major is needed on test day; the retake is not. |

## Adoption checklist for this teacher (only when asked)

Not a slice. Edits go through the Engine's authority-aware tools, after re-reading the current files.

1. `Teacher_Override.md`: remove the response-type correction rules and the "major extra attempts outside school" permission; the seed playbook now carries the policy. Keep every unrelated preference.
2. `context/grading.md`: add the retake cap and window values; replace the 2-3 sentence feedback limit with rich, specific feedback in the same warm voice.
3. `context/courses.md`: effort floor 40 in ELA7, PAP ELA7 and CS8. Collab is untouched.
4. No sweep of existing grades, no retake quizzes for past Majors unless the teacher asks for one.

## Choices open to challenge

These are the planner's, not the teacher's. Reviewers should attack them first.

1. Pairing by exact title alone. A renamed original breaks the link; the Engine holds and says so.
2. A student with no graded attempt on the original is not assigned the retake. They are listed for the teacher.
3. Team-taught courses are held in this version.
4. The hold against later lowering reads an Engine-written comment line (slice 04).
5. A floored New Quiz that is later re-scored above the floor holds for the teacher instead of recalculating fudge (slice 03).
6. Students with saved extra time get the existing extra-time rule applied to the 4-day window (slice 02).
7. The exemplar is shown at every score for every teacher (slice 01).
8. `personal_dates` stops reopening a Major original for corrections (slice 01).

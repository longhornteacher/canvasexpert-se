# Slice 01 - Playbooks, config and exemplars

Status: **not started**. Kind: seed playbooks plus three small Engine changes. Read the [README](README.md) first. This slice edits Dev seeds, not the teacher's live files.

## Result

An agent authoring or scoring a Major no longer offers points for copying or an immediate re-upload; it points students to the tutorial retake. Dailies are unchanged. The Engine has the two retake numbers as config, always shows the exemplar, and can leave the corrections row off a Major.

## Playbook changes

Seed files under `workspace-template/`. Put the rule once in `TEACHER.md` (Corrections) and have the task playbooks point to it.

1. **`TEACHER.md`**: Corrections splits by grade category. Major: tutorial retake only, capped contribution, never lowers, window counted from the day the grade is posted. Daily: the current text. Category comes from the teacher's words or the Canvas assignment group; ask only when it is unknown.
2. **`playbooks/quizzes.md`**: a Major quiz's instructions drop "No retakes. You can earn points back" and say feedback prepares for the tutorial retake. A Daily quiz keeps the current offer.
3. **`playbooks/authoring.md`**: every Major gets a retake quiz (procedure in slice 02). The retake's passage may be shared ahead as an ordinary Page or file with no questions, key or code. This is existing tooling; nothing is built for it.
4. **`playbooks/score.md`**:
   - Honest effort is judged for written or creative work only. A multiple-choice score is what it is.
   - After Major grades are posted, run the retake assignment (slice 02). After retakes are scored, run the retake application (slice 04).
   - Feedback is as rich as the work needs, not a sentence count.
5. **`playbooks/review.md`**: for a Major, review both forms together: each key against its own passage, and comparable demand.
6. **`setup.md` and the `context/grading.md` seed**: describe the two new config values.

No test searches playbook prose.

## Engine changes

1. **Config.** Add a `retakes` block to `GradingConfig` (`internal/domain/types.go`): `cap_percent` and `school_days`, validated like the existing percent and school-day fields. No block means retake operations report that no retake policy is set. No default is invented for other teachers.
2. **Exemplar.** In `internal/render/render.go`, show a part's exemplar at every score and label it `Exemplar`. Remove the below-threshold condition and the `Hand-copy exemplar` wording in both `partText` and the item-comment path. Whether copying earns Daily points is the playbook's sentence, not the Engine's.
3. **Corrections row.** One optional authoring setting, on assignment drafts and QuizForge settings, that omits `Last day for corrections` from the timetable (`internal/workflow/timetable.go:184`). The Engine prints no retake policy text; the agent writes that sentence in the assignment's own instructions.
4. **`personal_dates`.** It reopens on-time students through the correction window (`internal/workflow/personal_dates.go:317`). It must not do that for an assignment that has a retake quiz by the slice 02 pairing rule, because a reopened Major invites the second submission the policy removes.

## Tests

- Config: valid block, out-of-range values, absent block.
- Render: exemplar present at 20% and 95% through the real comment and Draft PDF paths; update the existing threshold tests.
- Timetable: the setting removes the row and leaves the others unchanged.
- `personal_dates`: an assignment with a retake quiz is not reopened; one without still is.

## Done when

Seeds read as one policy, the four Engine changes pass focused tests, formats/help show the new config and setting, and both capability records are updated with local evidence only.

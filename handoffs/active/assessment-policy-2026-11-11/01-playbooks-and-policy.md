# Slice 01 - Playbooks and policy consistency

Status: **not started**. Kind: playbook judgment and policy documentation; renderer work belongs to slice 05. Read [README](README.md) first. This slice changes Dev seeds/contracts, not the teacher's live UserSync instructions.

## Teacher-visible result

A teacher can ask for a Major or Daily without receiving contradictory correction offers. Major authoring includes the tutorial companion and optional advance passage reading. Scoring provides useful explanations, rich glows/grows and exemplars; it never offers Major points for copying them. Daily feedback can still invite formative correction or revision.

## Source files and current contradictions

- `workspace-template/TEACHER.md`: `Corrections`, exemplar/rationale standards and assignment defaults.
- `workspace-template/playbooks/quizzes.md`: one-attempt/no-retake default, rationale-copy offer, MC+Why, draft/review/create, saved models, publication and accommodations.
- `authoring.md`: original assignment directions, rubric/model creation, hub tiers and timetable instructions.
- `score.md`: discovery, earlier attempts, whole-assignment/item feedback, class/model exemplars, item-score posting, dates and teacher check-in.
- `edit.md`: category changes and substantive prompt changes that affect companion alignment.
- `native-content.md`: passage-only Page/file and native module assembly.
- `teacher-context.md`: durable standing policy, assignment expectations, paired Canvas IDs and provenance.
- `review.md`: fact, logic and pedagogy checks for parallel assessments and student-facing offers.
- `setup.md` and `context/grading.md`/`context/courses.md` seeds: explain the existing course effort-floor field and the four-day distinction without populating another teacher's course facts.
- `group-work.md`, `team.md`, `stad.md`, `parent-updates.md`, family/inbox/install playbooks: review references for consistency; edit only relevant policy routing. STAD awards are not gradebook credit. Parent updates retain their no-retake-promise rule.
- `docs/SCORING-RULES.md`, relevant paragraphs of `docs/ARCHITECTURE.md`, `docs/playbook-ownership-and-updates.md`, Engine formats/examples and both capability records must eventually describe the same supported behavior. The latter records move with the implementing slice, not a premature feature claim.

## Implementation steps

1. Put the canonical instructional rule in `TEACHER.md`; add a focused `playbooks/retakes.md` if the procedure is too long for the existing authoring/quiz/scoring sections. This is a task playbook, not a new Engine policy framework. Have affected task playbooks link to that one procedure instead of repeating the whole policy.
2. Resolve category before making an offer. Use the teacher's explicit classification and the course's exact assignment-group facts. Names may vary by course. A quiz or short response is not automatically Major; a long essay is not automatically eligible for ordinary resubmission. Ask only when the grade category materially remains unknown.
3. Distinguish a second attempt on the same quiz from a separate tutorial companion. Original Majors keep one attempt by default. Their instructions must not say the blanket `No retakes` or promise points for rationale transcription. State that feedback prepares students for the separate tutorial retake and explain its 70% contribution cap/four-school-day window in plain words.
4. Major written assignments follow the same rule as MC/mixed quizzes. Do not invite an immediate revised upload for credit on the original. Keep a meaningful original exemplar available at the intended instructional release; the retake's exact model/key remains private until the retake's intended feedback release.
5. Require paired authoring: a map of original concept/item to retake counterpart; same assessed skill, format, demand and rubric scale; new passage content; close stem wording with necessary evidence-specific changes. A simple noun swap is insufficient when it breaks the key or raises difficulty. Non-reading subjects use fresh equivalent material, such as a new code sample or data set, rather than importing ELA passages.
6. Prepare separate passage-only reading when practical. The reading resource may be visible before tutorial release; the quiz remains unpublished/unassigned. Describe the reason when advance reading is impractical instead of blocking a useful retake.
7. Daily instructions retain rationale/exemplar writing for credit and feedback-based resubmission. Preserve any established teacher amount/turn-in method. Where none exists, do not invent a percentage, automatic full restoration or a 70 cap. The new rule explicitly supersedes this teacher's older response-length ban for Dailies.
8. Rich feedback means accurate, specific glows, priority grows and concrete next steps proportional to the work; not a rigid sentence count. Preserve the teacher's plain, warm voice, score header preference and privacy requirements. The original two-sentence-per-choice rationale standard remains intact; it is separate from the length of scoring feedback.
9. Every promised exemplar must have a delivery path regardless of the student's score. Prefer a strong anonymous student example when legitimately available; otherwise an explicitly labelled agent-authored model. Do not skip it because the last student has not submitted or nobody supplied a suitable response. Existing follow-up comment-only results are a valid class delivery path. Never call an AI model student work.
10. Preserve the existing Daily never-lower preference. Assess a weaker revision honestly and use the supported comment-only path when retaining the higher grade; do not inflate criteria. Major original-grade replacement uses slice 06's separate operation, not fake original scoring.
11. Tell scoring agents to flag genuine effort from evidence; it is not inferred from a nonzero grade, a submitted timestamp or the existence of a group member. Missing evidence holds the judgment for that student without blocking independent work.
12. Make the four-day anchor and accommodations explicit. Preserve the existing eleven-day original close policy, extra-time allowance and first-genuine-attempt timing unless separately changed. A late-work lock on the original must not accidentally close the separate tutorial quiz before its legitimate retake window.
13. Update the content-review instructions: compare both forms, verify every key/rationale against its own passage, check comparable intellectual demand, ensure all Major offers are tutorial-only, and inspect the passage-only resource for leaked questions/key/code. Use the existing author/fact/logic/pedagogy/arbiter workflow for actual student content; no new review engine.
14. Save reusable original/companion origin and IDs and reviewed alignment expectations in the original assignment's existing context note. Cross-reference from the companion's note. Keep drafts, response excerpts, result rows and action receipts in UserInstall. Store the code in a teacher-only local authoring record, never a student Page or general shared course note.

## Acceptance examples

- A 100-point Major MC quiz gets a separate protected New Quiz and explains tutorial preparation; it does not offer copying points.
- A Major essay gets the same companion requirement. Its feedback teaches revision skills without soliciting an immediate graded second upload.
- A Daily 1-2 sentence response can be revised for credit despite the old response-length restriction.
- A Daily MC quiz can offer handwritten rationale corrections under the existing credit arrangement.
- A student earning 95% still has access to the promised exemplar; a student earning 20% does not see `Hand-copy for points` on a Major.
- The companion's private answers and access code never appear in the advance reading Page, student handout, family report or student feedback.
- An unrelated parent-update task neither advertises retakes nor creates companions.

## Verification and completion record

Review every active seed playbook with `rg` for `retake`, `correction`, `rationale`, `resubmission`, `second attempt`, `exemplar`, `hand-copy`, `extra credit`, `major` and `daily`. Classify hits in context; do not rewrite legitimate MC+Why assessed reasoning or STAD celebration points as prohibited transcription credit. Historical handoffs are evidence, not active instructions to rewrite.

Do not add source-text tests for prose. Run `git diff --check` and verify local Markdown links. If a new playbook is added, ensure the embedded seed/update mechanism actually includes it and add it to route/reference help where needed. Capability records must label the policy as planned/manual until the relevant Engine slices land.

Completion entry: changed files, consistency findings resolved, checks run, and any delivery path still awaiting an Engine slice.

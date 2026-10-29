# Review before anyone sees it

Use this for content students will see: assignments, quizzes with their answer keys and rationales, Pages, tier supports, exemplars, rubrics, discussion prompts and scoring feedback. Run it on finished work before the check-in or report, and before any write when the request already included one. Family messages, Inbox replies, discussion replies and parent updates are not reviewed this way yet. The goal is the most reliable product for the fewest tokens: every fact proven against a source, every explanation sound, every teaching choice fit for this teacher and these students, and each helper reading only what its job needs.

## Roles

Five roles, each with a fixed job. Only the author writes.

| Role | Receives | Returns | Never |
|---|---|---|---|
| **Author** (you) | the request, sources, teacher context | the product, and a response to every finding | skips the review or writes to Canvas before it ends |
| **Fact-checker** | the product and its sources | each provable item: found, with the exact source text and location, or not found | edits, judges quality |
| **Logic reviewer** | the product and the task (directions, prompt, rubric, interpretation) | findings, each with the passage, its reason and a suggested fix | edits, rescores, sees your reasoning |
| **Pedagogy reviewer** | the product, `TEACHER.md`, `Teacher_Override.md`, `context/teaching-profile.md`, `context/grading.md`, relevant course/assignment notes, the grade, course and tier | findings, each with the passage, its reason and a suggested fix | edits, rescores, sees your reasoning |
| **Arbiter** | the findings, your responses, the passages they concern and the standards they cite; possible-AI-use flags with their checked evidence; any fact the fact-checker and you still disagree on | a ruling on each | edits, re-reads the whole product, re-checks settled facts |

Run each role as a separate helper agent when your app can start one: one helper per role for each product or assignment, never one per student. Give the fact-checker the cheapest capable model; it only has to find and quote. Give the reviewers a model at least as strong as yours, and the arbiter the strongest available. Hand each helper file paths and its brief, not a summary of the work: a summary is exactly what the review exists to check. Helpers follow `AGENTS.md`: aliases only, no `.cse/` or contact files, and no student work sent to the web or any outside service.

When your app can't start helpers, play each role yourself in turn. Say which role you are starting ("Fact-checking: product and sources only"), set your own reasoning aside, and read only what that role receives. It is weaker than independent agents, so be strict.

## 1. Fact-check (provable items)

Run this first, so the reviewers never weigh an argument built on a quote that doesn't exist. Provable items include:

- quotations, and the page, chapter, line or paragraph numbers attached to them
- characters, events, settings and who said or did what
- vocabulary and key terms as the text uses them, and definitions given as fact
- dates, titles, authors, and names of assignments, units and books
- the keyed answer of a factual or computed item: recompute math, re-run code, find the fact
- in scoring: every quotation from or claim about a student's own work ("your second paragraph says…"), and every quotation from the source text in feedback or an exemplar
- the evidence behind every possible-AI-use flag in `score.md`: the suspect phrase is quoted exactly, matching wording really matches, and a quote called invented is absent from the whole source. Without the whole source, absence isn't proven: drop that signal and keep the flag only if another one still qualifies

Check against sources in hand only: the assigned text in the workspace or `context/`, pulled packets and student files, pulled Canvas content, Engine reports, the teacher's own words in this conversation and the unit notes. Memory is not a source, even for a famous book. Search a long source for each item rather than reading it whole when your tools allow. Page numbers follow the teacher's edition; when the edition is unknown, use chapter or section instead.

Fix or cut every item not found. An item with no source in hand is cut or rewritten without the specific claim (a chapter instead of a page, a paraphrase instead of a quotation). Nothing unproven ships. If you are sure an item exists, give its location and the fact-checker confirms it there; only a disagreement that survives that goes to the arbiter.

## 2. Logic review (arguable items)

Does each argument, explanation and piece of commentary hold up? Look for:

- an explanation that doesn't follow from its evidence, or evidence that doesn't support the claim it's attached to
- a keyed answer that isn't clearly the best choice, a distractor that is defensibly correct, or two choices that are both right
- a rationale that is wrong, circular or explains a different answer
- an exemplar or model answer whose reasoning a student shouldn't copy
- scoring: an interpretation or scoring basis that misreads the prompt or rubric, feedback whose reasons don't match the score given, or similar work scored differently
- directions or a Check yourself answer that contradicts the task, the text or itself

## 3. Pedagogy review (teaching decisions)

Do the teaching choices fit this teacher, this grade and these students? Judge against the teacher's saved standards and the current request, not general taste:

- alignment: the Learning Target, task, items and rubric measure the same skill at the intended rigor (DOK 2-3 by default)
- wording: `TEACHER.md` budgets, sentence length for the grade, academic words bolded and defined once
- quiz design: distractors are real misconceptions, choices survive shuffling, the longest choice doesn't give the answer away
- tiers scaffold the same standard and never lower it; Green supports actually support emergent bilingual students
- feedback is specific, actionable and in the teacher's saved voice and `feedback_guidance`
- authored exemplars (including private models saved for later scoring) show strong, attainable work at the actual grade in both language and thinking, using taught skills; they neither demand adult polish nor excuse shallow thinking, and add no hidden rubric requirements (see `teacher-context.md`)
- nothing private (accommodations, tier reasons, teacher notes, flags) shows in student-facing text

The teacher's explicit choices are settled: flag a likely problem with one, but treat it as the teacher's decision, not a defect.

## 4. Respond and arbitrate

Answer every logic and pedagogy finding: accept or reject, with a one-line reason. Don't defend work by habit or agree to look cooperative; both mislead the arbiter.

The arbiter rules on every logic and pedagogy finding, accepted or rejected, on every possible-AI-use flag, and on any fact still disputed. It works from the passages and evidence it is handed and opens a source only when a ruling depends on it. Facts the fact-checker found, or that you fixed or cut, never reach it.

- **Uphold** a finding when it shows a real error in logic, a factual problem the fact-checker missed, or a departure from the teacher's standards. It may name a better fix than the one suggested.
- **Dismiss** a finding when it is a matter of taste, when your reason holds, or when it would override the teacher's explicit request.
- **Uphold** a possible-AI-use flag when its checked evidence meets `score.md`'s threshold; dismiss it otherwise. Wording shared by several students that may have come from the teacher (a sentence stem, a frame, a model sentence) is not a flag on each student: make it one question for the teacher.
- **Refer to the teacher** a finding in material the teacher wrote or supplied (archived curriculum, a pasted prompt, their own key), rather than ruling it changed.

Upheld rulings are binding. Apply them, then send only new or changed provable items back to the fact-checker; fix or cut anything not found. That ends the review: one round of review, one ruling and one final fact-check, with no second round.

## Scale by output

| Output | Fact-check | Logic | Pedagogy |
|---|---|---|---|
| Assignment, rubric, Page, tier supports, discussion prompt | full | full | full |
| New Quiz: items, key, rationales, instructions | full | full | full |
| Edit of existing content | changed passages | changed passages | changed passages |
| Scoring | every quotation, claim about a student's work, exemplar and flag's evidence | interpretation, scoring basis, and every student's scores and feedback | interpretation, scoring basis, and every student's feedback |

In scoring, the reviewers read the results file against the rubric and interpretation in one pass per assignment, not the packets: the fact-checker has already proven what the feedback says about each student's work. They open a student's work only when a finding depends on it. An upheld finding that reflects a pattern ("feedback misreads criterion 2") is applied to every student it affects, and their changed feedback goes through the final fact-check. A change to the shared interpretation or scoring basis applies to everyone, as `score.md` says. A date, points or settings change with no new wording has nothing to review.

## Report

The teacher sees only the arbiter's decisions that need them: upheld possible-AI-use flags (reported as `score.md` describes), questions about wording that may be the teacher's own, and findings in the teacher's own material. Leave out the review itself: fixes, cuts, dismissed findings and counts. Keep findings and rulings in the conversation, out of drafts, results files and student notes, and show them only when the teacher asks.

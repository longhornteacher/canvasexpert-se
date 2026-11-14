> **Retired 2026-11-14.** All five slices are committed: c8b5a17, 612629d, 23202b2, 9631f84 and 5dc068c. This is historical execution text, not work to repeat. Remaining live/client evidence and scope limits are recorded in [pending observations](../../active/PENDING_OBSERVATIONS.md). Retirement does not claim those checks passed.

# Slice 4: review sized to the change

Playbook text only. Files: `workspace-template/playbooks/review.md`, `workspace-template/playbooks/teacher-context.md`, `workspace-template/AGENTS.md`, root `AGENTS.md`.

Why: every authoring review starts four helper agents, even for a one-word fix. Scoring already uses two helpers and calls the arbiter only for disputes. The developer decided:

| Change | Review |
|---|---|
| A correction that changes no meaning (typo, spelling, formatting, a date, points, a setting) | No helpers |
| Scoring feedback; edits to existing content; Pages; Daily assignments; discussion prompts; tier supports | Fact-checker and one combined reviewer; arbiter only for a flag, a rejected finding or a disputed fact |
| A new quiz, a Major (and its tutorial retake), a rubric | All four helpers, as now |

The task playbooks (`edit.md`, `authoring.md`, `quizzes.md`, `native-content.md`, `group-work.md`, `stad.md`) only say "run the review in `review.md`". Do not edit them.

## Step 1: `review.md`, the paragraph that starts "**Scoring uses two helpers, and the arbiter only when needed.**"

**Replace that whole paragraph with:**

> **Three sizes of review.** Use the smallest that fits; when unsure, use the larger.
>
> - **No helpers**: a correction that changes no meaning, such as a typo, spelling, formatting, a date, points or a setting. Validate locally and look it over yourself. A change inside a quotation, a source passage, an answer key or a rationale is not this size, because it changes a provable item.
> - **Two helpers**: scoring feedback, and authored work not listed under four helpers (edits to existing content, Pages, Daily assignments, discussion prompts, tier supports). The fact-checker (cheapest capable model) runs as above, and one reviewer (a mid-tier model, such as a Sonnet-class model) does both the logic and the pedagogy review in a single pass. You answer its findings as in step 4. The arbiter runs only when something is left to rule on: a possible-AI-use flag, a finding you rejected, or a fact you and the fact-checker still disagree on. Give it the strongest available model, since a flag is the highest-stakes call in scoring. When you accepted every finding and there is no flag or disputed fact, apply the findings, run the final fact-check on what changed, and the review is done.
> - **Four helpers**: a new quiz, a Major and its tutorial retake, and a rubric. Separate logic and pedagogy reviewers run, and the arbiter rules on every finding.

## Step 2: `review.md`, section "4. Respond and arbitrate"

**Find:**

> In scoring it runs only when there is a flag, a rejected finding or a disputed fact, and rules on just those; accepted findings are applied without it.

**Replace with:**

> In a two-helper review it runs only when there is a flag, a rejected finding or a disputed fact, and rules on just those; accepted findings are applied without it.

## Step 3: `review.md`, the table under "## Scale by output"

**Replace the whole table** (header through the Scoring row) with:

```markdown
| Output | Helpers | Fact-check | Logic | Pedagogy |
|---|---|---|---|---|
| New Quiz (items, key, rationales, instructions), Major and its tutorial retake, rubric | four | full | full | full |
| New Page, Daily assignment, tier supports, discussion prompt | two (one reviewer covers both columns) | full | full | full |
| Edit of existing content | two (one reviewer covers both columns) | changed passages | changed passages | changed passages |
| Correction that changes no meaning; date, points or settings change | none | none | none | none |
| Scoring | two (one reviewer covers both columns) | every quotation, claim about a student's work, exemplar and flag's evidence | interpretation, scoring basis, and every student's scores and feedback | interpretation, scoring basis, and every student's feedback |
```

Leave the paragraph after the table as it is.

## Step 4: `teacher-context.md`, about line 64

**Find:** `the pedagogy reviewer explicitly checks` **Replace with:** `the reviewer explicitly checks`

## Step 5: `workspace-template/AGENTS.md`, about line 31

**Find:**

> run the review in `playbooks/review.md`: a fact-checker, logic and pedagogy reviewers, and an arbiter whose rulings you apply (scoring uses a fact-checker and one combined reviewer, and the arbiter only for a flag or a disputed finding).

**Replace with:**

> run the review in `playbooks/review.md`, which sets its size: no helpers for a correction that changes no meaning; a fact-checker and one combined reviewer for scoring, edits, Pages and Daily work, with the arbiter only for a flag or a disputed finding; and a fact-checker, logic and pedagogy reviewers and an arbiter for new quizzes, Majors and rubrics.

## Step 6: root `AGENTS.md`, the "Student-facing content" paragraph

**Find:**

> (author, fact-checker, logic and pedagogy reviewers, arbiter; scoring uses a fact-checker and one combined reviewer, with the arbiter only for flags or disputes)

**Replace with:**

> (author, fact-checker, logic and pedagogy reviewers and arbiter for new quizzes, Majors and rubrics; a fact-checker and one combined reviewer for scoring, edits, Pages and Daily work, with the arbiter only for flags or disputes; no helpers for corrections that change no meaning)

## Done when

All six edits are in and the common checks pass. Read `review.md` top to bottom once: nothing left in it should say that scoring is the only two-helper case, or that every authored product gets four helpers. If a sentence still says so, fix that sentence only and name it in the commit message.

Commit: `playbooks: review uses zero, two or four helpers by size of change`

# Teacher mode: classroom standards

Read this when the teacher asks for classroom work: an assignment, a page, a quiz, scoring, or class setup. Act as an experienced classroom teacher's assistant, not a software engineer. Talk in plain classroom language about learning targets, grade level and student work. Do not mention code, terminals, JSON or diffs unless the teacher asks. Then read the task playbook.

These are defaults. A teacher's own preference in `context/teaching-profile.md`, `context/grading.md` or this request always wins; save a new preference there once they state it.

## Points: 100 by default

Assignments and quizzes total 100 points unless the teacher says otherwise. Rubrics balance to 100 (4 criteria at 25, 5 at 20). Quiz items are weighted evenly or deliberately to sum to 100 (10 items at 10). Percentages are what students, parents and the gradebook read at a glance. If a request does not divide evenly, propose the nearest clean split and say so; the Engine never invents points, so state the values you choose.

## Rigor: strong students at grade level

Pitch text complexity, vocabulary and question depth for proficient-to-advanced students at the stated grade. Do not talk down (an 8th grader is not a 3rd grader) and do not inflate into college jargon. Aim for DOK 2 (apply, interpret in context) and DOK 3 (evaluate a claim, justify a choice). Quiz distractors must be real, predictable misconceptions: no jokes, trick wording, obvious fakes or "all of the above".

## Layout: landscape laptops, not phones

Students use 11-14 inch Chromebooks and laptops in landscape. Do not build narrow phone-width containers. Where a page uses HTML (`body_format: html`), let it fill the width (`max-width: 100%` or about `1100px`) and put agendas, vocabulary and resource links in two or three flexible columns (`display: flex; gap: 16px; flex-wrap: wrap;`). Default Markdown bodies are plain and already full width; use HTML only when the layout earns it. `cse help fields` says what Canvas keeps.

## Exemplars and quiz rationales

- **Writing assignments** include a grade-representative exemplar (a mentor text showing what top work looks like) in the directions or rubric overview.
- **Every quiz item** gets a root `rationales` entry (see `playbooks/quizzes.md`) of about two sentences saying why the correct answer is correct, shown in green so a student reviewing an attempt gets positive reinforcement:

  ```html
  <p style="color: #2e7d32;"><strong>Why this is correct:</strong> two sentences naming the idea and the evidence.</p>
  ```

## Student-facing structure

- Open every page and assignment with one goal line: **Learning Target:** *I can [verb] [skill] so that [purpose].*
- Give directions as three scannable steps rather than paragraphs: **1. Read and examine** (the text, prompt or video), **2. Plan and draft** (expectations, length, criteria), **3. Check the rubric and submit** (self-check first).

## Offer tiered support (Hub)

When the teacher asks for a substantial assignment, project or quiz, offer once, in their words:

> "Want to tier this? We keep one assignment for the whole class and add private support pages: sentence starters and organizers for students who need more, and a challenge for those ready for it."

If yes, follow the Hub section of `playbooks/authoring.md` or `playbooks/quizzes.md`. Typical tiers: **Emerging** (sentence frames, organizers, word banks, chunked steps), **On-level** (guiding questions, outline, self-check list), **Extension** (DOK 3-4 task, counter-perspective, creative constraint). Supports scaffold the same standard and never lower it. Ask for the teacher's exact Canvas differentiation tags (or read them from `context/`); never assume or invent tags.

## Ask when needed, not up front

Do not interview the teacher about style or grading during setup. On the first page build, ask once whether they like pages clean and academic or warm and engaging; on the first scoring session, ask about rubric scale and feedback style. Save answers to `context/teaching-profile.md` and `context/grading.md`.

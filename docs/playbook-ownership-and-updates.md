# Playbook ownership and updates

Working design decisions from the teacher/developer conversation, 2026-10-17.
This document records agreed direction and remaining design questions. Except
where a conflict below is marked shipped, it does not claim that the shipped
instructions or Engine already implement it. Read alongside ARCHITECTURE.md
and SCORING-RULES.md; implementation changes to those contracts remain
separate work.

## Product intent

CE-SE ships a teaching approach as well as machinery. Playbooks should show
teachers useful possibilities for agent-assisted planning, assessment and
feedback. A pedagogical default is not automatically a personal preference.
The teacher's word governs agent behavior outside the Engine's mechanical
functions. Teachers can override shipped teaching and workflow defaults,
including review, and those decisions must survive new installations and
product updates. An agent preference cannot change what the Engine
mechanically supports or computes.

Ownership follows the source of a decision, not its file location. A teacher
may customize a playbook directly or state a preference in conversation.
Neither choice makes that decision disposable during a product update.

## Confirmed shipped defaults

- Assignments and quizzes total 100 points unless overridden.
- Pitch rigor at proficient-to-advanced students at the stated grade level.
- Use a Learning Target and three-step directions.
- Offer tiered support.
- Include a mentor exemplar when creating a writing assignment.
- Include about two sentences of rationale for every auto-scored question
  during planning/creation. This requirement does not apply to written items.
- For ordinary assignment feedback, lead with an overall computed assessment
  and per-item scores: "Overall: X/100", then "[Item label]: Y/Z" for each
  assessed response. Preserve the relevant denominators when an assignment
  uses another scale. The complete default feedback order is overall
  assessment score, item score(s), compact rubric breakdown, improvement tips,
  then exemplars. The teacher identified later parent-update comparisons as a
  reason to expose both score levels.
- When a late deduction applies, the overall line shows the late-adjusted
  score with the deduction named, for example
  "Overall: 75/100 (10 late points deducted)". The earned assessment stays
  recoverable from the line, and the comment matches what a parent later sees
  in the gradebook. Canvas's late policy computes the deduction, so where the
  Engine obtains that value is open (see Remaining design work).
- For New Quizzes, include an overall score when Canvas supplies the complete
  quiz result for the matching attempt, including auto-scored items. Identify
  it as the Canvas overall result, distinct from the agent's written-response
  assessments. Otherwise omit the overall line and continue per-item feedback;
  do not equate assessed written work with the full quiz result. This display
  rule does not authorize writing quiz or item grades.
- During scoring, the agent selects the best two student exemplars for each
  written prompt. Excerpts are always anonymous; there is no named option, and
  the choice needs no separate teacher approval. Every student in a receiving
  class gets the same pair for that prompt. Automatically consider matching
  work across saved current courses; the source course need not be the
  receiving course. For example, an ELA7 class can receive ELA7 PAP exemplars.
  Individual feedback posts as usual; the exemplars follow in a second comment
  once the matching set is scored, so independent feedback never waits for
  the whole set.
- Use school-day lateness, excluding weekends and calendar breaks;
  early-release days count as full days.
- Corrections are available as soon as a student finishes the assignment.
  The official correction deadline is 14 calendar days from that student's
  applicable due date, not 14 days from feedback publication. Availability
  does not wait for agent scoring or for feedback to be posted. Turning work
  in and doing corrections are separate; lateness rules govern the first, not
  the correction window. This is a shipped default only, and teachers replace
  it with their own correction systems.
- In planning/creation, say "Corrections available" in the assignment itself
  and direct students to feedback for rationales/exemplars. This does not
  prescribe a correction method, promise a point increase, or require the
  teacher to establish a formula before the assignment can be created.
  Teachers develop their own correction norms; remember them when expressed.
- Parent updates skip students at 90+ with no missing work in the included
  courses; never request parent replies; never invent extra-credit
  opportunities; include only existing coursework and teacher-authorized
  opportunities.

The school calendar, bell schedule and timezone remain in shipped starter
context; they are the expensive content to recreate. Strip identifying
information (school name) rather than removing that content.

The bell schedule source is the 2026-2027 school bell schedule PDF the teacher
supplied on 2026-10-17. It has four day types:

| Period | Bobcat Hour (Mon-Thu) | Homeroom | Homeroom First | House Days (Homeroom Last) |
|---|---|---|---|---|
| Homeroom | | 10:23-10:53 (after 2nd) | 8:35-9:05 | 2:30-3:55 |
| 1st | 8:35-9:25 | 8:35-9:25 | 9:09-9:59 | 8:35-9:15 |
| 2nd | 9:29-10:21 (announcements 9:30) | 9:29-10:19 | 10:03-10:53 | 9:20-10:00 |
| 3rd | 10:25-11:15 | 10:57-11:47 | 10:57-11:47 | 10:05-10:45 |
| 4th | 11:19-12:09 | 11:51-1:13 (A 11:51-12:21, B 12:43-1:13) | 11:51-1:13 (A 11:51-12:26, B 12:38-1:13) | 11:35-12:55 (A 11:35-12:10, B 12:20-12:55) |
| Bobcat Hour | 12:11-1:13 | | | |
| 5th | 1:17-2:07 | 1:17-2:07 | 1:17-2:07 | 10:50-11:30 (before 4th) |
| 6th | 2:11-3:01 | 2:11-3:01 | 2:11-3:01 | 1:00-1:40 |
| 7th | 3:05-3:55 | 3:05-3:55 | 3:05-3:55 | 1:45-2:25 |

The PDF lists 5th before 4th on House Days, and the times confirm that order.
It does not say which schedule Friday uses or which days are Homeroom,
Homeroom First or House Days; the school calendar or the teacher supplies
that.

## Teacher-owned setup

- The Skyward address and the Flex/Bobcat Hour link are this teacher's setup,
  not shipped defaults. They are two URLs, cheap for any teacher to supply.
- Other teachers' corresponding links, course choices, preferences and notes
  belong to their own setup.
- Standing overrides, course-specific differences and task-specific exceptions
  retain their stated scope.
- Correction methods and point-recovery norms develop with each teacher;
  the shipped feedback practices do not establish those terms by themselves.

The earlier "spelling never counts for more than 10%" example was illustrative,
not a requested shipped scoring policy or a confirmed personal preference.

Without UserSync, reusable setup stays in UserInstall. With UserSync configured,
it is authoritative and each UserInstall accesses it directly. Session work
stays in UserInstall. Product updates do not change this ownership boundary.

## Agent maintenance

Updating guidance is a normal part of teacher-agent conversation. The teacher
does not need to request a file edit or know the storage layout.

- Teacher decisions live in a separate teacher overlay that the agent reads
  after the shipped playbooks; where they differ, the overlay wins. Saving a
  preference writes the overlay, not the shipped playbook text.
- Save explicit standing preferences as they are expressed. For example,
  "Never offer extra credit without my explicit permission" becomes a
  remembered standing instruction without a separate file-edit approval.
- Reconcile conflicting reusable guidance so saving a preference is effective,
  rather than leaving another active instruction that contradicts it.
- Preserve meaningful scope, exceptions and stated reasons. Do not generalize
  "for this assignment" into a permanent default.
- Honor explicitly stated standing workflow preferences, including "Whenever
  I explicitly request posting, skip review." If the teacher says skip review,
  skip review. Persist the teacher's actual authorization and scope rather
  than inferring standing permission from a one-task skip.
- Perform routine maintenance and incorporate compatible product updates in
  the background. Acknowledge a newly stated preference naturally.
- Discuss new teaching choices or genuine conflicts when relevant to the
  teacher's work. Keep the existing teacher decision while a conflict remains
  unresolved; avoid an installation-time questionnaire about every change.

## Update direction

With the overlay, a product update replaces shipped playbooks and leaves the
overlay alone. Three-way reconciliation is needed only where a teacher edited
a shipped playbook directly, or where a new shipped default meets an overlay
entry on the same topic.

Reconciliation runs during "install" or "update", in the background. It must
work for a teacher who only unzips, points the agent and talks: no merge
prompts, no file vocabulary, and a short plain note only when a new teaching
choice is worth mentioning.

Use the previous shipped component, the current teacher component and the new
shipped component to inform agent reconciliation. Include relevant teacher
instructions stored elsewhere, not only edits to the component being updated.
The new product version contributes proposed improvements; it does not acquire
authority over teacher choices.

Retain each shipped baseline as a plain readable folder in UserInstall,
`shipped/<version>/`, outside `.cse/` so agents can read it. The current
updater only compares active files with the current Engine seed and can
archive/replace whole files; archives of teacher files do not by themselves
supply the original shipped baseline. When a baseline is unavailable, do not
guess that unexplained differences are obsolete or safe to discard.

## Current conflicts to resolve in implementation

1. Setup and Settings use whole-file replacement. This preserves old bytes in
   an inactive archive but does not keep teacher decisions active.
2. **Shipped 2026-10-17.** TEACHER.md now requires rationales for every
   auto-scored quiz item and says written items take none.
3. The creation-time mentor exemplar does not implement the distinct scoring
   practice of selecting two student exemplars per written prompt.
4. Pearland/Flex defaults exist in both shipped context and Engine fallbacks;
   rendering also assumes Skyward/Bobcat Hour. Separating the seed alone will
   not separate this teacher's preferences.
5. **Shipped 2026-10-17.** score.md now leads New Quiz comments with per-item
   criterion lines and allows a labeled Canvas quiz result only when the pull
   carries the complete result for that attempt. The review found an Engine
   defect behind this conflict: the default comment header ignored the New
   Quiz omission and printed `Assessment: X / Y`, the written portion scaled
   to the whole quiz. The renderer and the revise-feedback path now omit it
   (local tests only; no Canvas Live run).
6. Parent-update guidance uses feedback evidence to identify corrections.
   Align that with the planning declaration, availability on completion and
   teacher correction norms. Existing feedback helps students correct work;
   its publication is not what makes corrections available, and it must not
   invent a point-recovery promise.
7. First-task questions and repeated tier-support offers should consult saved
   choices rather than restarting settled conversations on a new installation.
8. Parent history currently carries assignment-level first/latest/current
   Canvas scores, not per-item assessment history. Adding score lines to
   feedback does not by itself implement per-item comparisons. Prefer the
   structured scoring results for CE-SE-authored assessments; establish how
   the parent-update workflow obtains the relevant values before claiming
   support. Keep earned assessments distinct from adjusted Canvas grades.
9. **Shipped 2026-10-17.** AGENTS.md, score.md, the install safety promise and
   the `post_results` tool description now separate a one-task skip from a
   saved standing preference, which the agent records with its scope in
   `context/teaching-profile.md` until the overlay exists. Parent-update
   "Rules that do not bend" became "Defaults", with honest values and privacy
   kept as non-negotiable.

## Remaining design work

- Name the overlay file, its location under the active rules authority, and
  how it replaces today's scattered `context/teaching-profile.md` and
  `context/grading.md` preference saves.
- Build `shipped/<version>/` retention in the updater and the background
  install/update reconciliation pass.
- Decide where the late-adjusted overall comes from. Canvas computes the
  deduction from the course late policy after the Engine writes late timing;
  the comment is written in the same post. Options: read the course late
  policy and compute the same deduction, or name the deduction from the
  teacher's late timing only. Do not add a post-write read-back.
- Define selection guidance for matching prompts across courses and for fewer
  than two usable student examples, without blocking independent scoring or
  fabricating student work. Define when a matching set counts as scored for
  the exemplar comment.
- Complete the shipped-versus-personal inventory for other scoring and
  feedback choices before writing a coding handoff.
- Determine where the existing New Quiz pull can supply the complete Canvas
  result for the matching attempt. Preserve per-item-only feedback when that
  value is unavailable; this must not block ordinary scoring.
- Add the bell schedule to the school-calendar seed without the school name.

## Done looks like

A colleague with saved preferences (for example a standing review skip, a
different correction window and their own Skyward link) unzips a newer CE-SE,
points their agent at it and starts working. Every saved preference still
applies, new shipped improvements apply where they don't conflict, and the
teacher was not asked to merge anything.

No coding agents have been dispatched, no teacher setup has been migrated, and
no Canvas writes or live verification are part of this design review.

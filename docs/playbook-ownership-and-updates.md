# Playbook ownership and updates

Working design decisions from the teacher/developer conversation, 2026-10-17,
with the git-install rule (2026-10-21) first. This document records agreed
direction and what has been built. Items marked
shipped or built are implemented; the rest is direction only. Read alongside ARCHITECTURE.md
and SCORING-RULES.md; implementation changes to those contracts remain
separate work.

## Current rule: git installs (decided 2026-10-21)

Git installs win over ZIP installs. In a git install git owns the shipped playbooks, `TEACHER.md` and the root `AGENTS.md`/`CLAUDE.md`, and an update replaces them. A teacher's choices live in `Teacher_Override.md`, `context/` and their own root guides, which git never tracks. An agent never edits a shipped file in a git install; a preference goes in `Teacher_Override.md`. An edit that is found at update time is converted into an override once, the old file is kept under `playbook-old/<date>/`, and the file is reset to the shipped text. The `shipped/` copies and the `kept` merge belong to ZIP installs only. UserSync carries `context/` and `Teacher_Override.md`, not playbooks. See [the migration plan](../handoffs/retired/GIT-INSTALL-MIGRATION.md).

Add shipped playbooks under `workspace-template/playbooks/`; the package script includes matching Markdown playbooks in the flattened UserInstall tree, and the Engine seeds them from the same template. `inbox.md` follows this standard path and remains teacher-editable in ZIP installs and git-owned in git installs.

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
- When the work is late, the overall line shows the earned assessment, before
  Canvas's late deduction, marked "(late)": "Overall: 85/100 (late)". The
  Engine does not compute or name the deduction; Canvas's gradebook shows the
  adjusted grade. A family may notice the gap; that follows from the late
  submission and is not worth extra machinery.
- New Quiz feedback never has an overall line, only per-item lines. The
  comment goes out before the teacher enters written-item points, so Canvas
  has no complete quiz result yet, and the written portion is never presented
  as the quiz score. (Decided 2026-10-17; this replaces showing a Canvas
  result when available.)
- During scoring, the agent selects the best two student exemplars for each
  written prompt. Excerpts are always anonymous; there is no named option, and
  the choice needs no separate teacher approval. Every student in a receiving
  class gets the same pair for that prompt. Automatically consider matching
  work across saved current courses; the source course need not be the
  receiving course. For example, an ELA7 class can receive ELA7 PAP exemplars.
  Individual feedback posts as usual; the exemplars follow in a second,
  comment-only post when the last student who answered that prompt is scored,
  so independent feedback never waits. The follow-up never touches a grade.
- Use school-day lateness, excluding weekends and calendar breaks;
  early-release days count as full days.
- Superseded 2026-11-13 (see
  [the handoff](../handoffs/active/HANDOFF_Student_Uploads_and_Grading_Offers_2026-11-13.md),
  section 2): the earlier default of corrections open for 14 calendar days,
  advertised as "Corrections available" in every assignment, is gone. With no
  explicit teacher correction policy, a new assignment, quiz or feedback comment
  offers no further submission, extra credit or correction points. Feedback
  still explains how to improve, and rationales and exemplars stay. A teacher's
  own correction system, from `Teacher_Override.md` or `context/grading.md`,
  governs; remember it when expressed. A teacher-set `correction_school_days`
  window puts a corrections row in the Engine timetable and lets `personal_dates`
  reopen on-time work, and it never mentions points.
- During Major scoring the agent writes the tutorial retake from what students
  missed and creates it privately: unpublished, assigned to nobody, omitted from
  the final grade. `retake assign` and `retake apply` run only on explicit
  teacher direction. Implemented in Dev with local tests only; no Canvas Live
  check of this behavior.
- Parent updates use Engine-resolved Missing/0 classifications: a positive
  teacher-entered grade overrides stale missing/no-submission indicators;
  a current recorded zero belongs in Missing/0 without implying it was
  unsubmitted. Keep grade history, but never repeat Missing/0 items under
  Corrections. Skip only after classification, when every included course
  meets `skip_score` (90 by default) and no Missing/0 items remain. Never
  request parent replies or invent extra-credit opportunities; include only
  existing coursework and teacher-authorized opportunities. Superseded
  2026-11-07 and 2026-11-13: the current parent-report contract is
  [parent-update-flow.md](parent-update-flow.md), which has no Corrections
  section, correction deadlines, Missing/0 inventory or `skip_score`; read that
  rather than this bullet.

- Parent reports are scoped by the active grading period in
  `context/school-calendar.md`. The Engine inventories all required visible
  work due within the saved retrospective window, resolves student-specific
  effective dates, and limits Missing/0, Corrections and History to that
  inventory. Email and print render every inventory row even when the agent's
  draft selects none. An empty window is an explicit skip. Do not disclose
  grades whose `posted_at` is explicitly null. (Superseded 2026-11-07: the
  report now lists below-70% released scores with submission status and has no
  Corrections or History list; see [parent-update-flow.md](parent-update-flow.md).)
- Lateness is separate from score arithmetic. The Engine counts school time
  from the first genuine submission using the calendar; the agent records its
  reasoning and accommodation basis. `late-only` changes only Canvas lateness
  fields on an existing grade, and review remains the normal posting gate.

The school calendar, bell schedule and timezone remain in shipped starter
context under generic names (`context/school-calendar.md`); they are the
expensive content to recreate. The bell-schedule table there is an example with
example times for the teacher to replace.

## Teacher-owned setup

- The grade-portal address and the tutoring sign-up link are each teacher's
  setup, not shipped defaults. They are two URLs, cheap for any teacher to supply.
- Other teachers' corresponding links, course choices, preferences and notes
  belong to their own setup.
- Standing overrides, course-specific differences and task-specific exceptions
  retain their stated scope.
- Correction methods and point-recovery norms develop with each teacher;
  the shipped feedback practices do not establish those terms by themselves.
  Shipped text offers none until the teacher chooses one (2026-11-13).

The earlier "spelling never counts for more than 10%" example was illustrative,
not a requested shipped scoring policy or a confirmed personal preference.

Without UserSync, reusable setup stays in UserInstall. With UserSync configured,
it is authoritative and each UserInstall accesses it directly. Session work
stays in UserInstall. Product updates do not change this ownership boundary.

## Agent maintenance

Updating guidance is a normal part of teacher-agent conversation. The teacher
does not need to request a file edit or know the storage layout.

- Teacher decisions live in `Teacher_Override.md`, a separate overlay that
  the agent reads after the shipped playbooks; where they differ, the overlay
  wins. It sits beside `TEACHER.md` under the active rules authority
  (UserInstall, or UserSync when configured), so it follows the teacher across
  PCs. Saving a preference writes the overlay, not the shipped playbook text.
- `Teacher_Override.md` holds only overrides of shipped defaults (for example
  a standing review skip, a correction window, no tiered-support
  offers). `context/teaching-profile.md` and `context/grading.md` stay as
  setup facts the shipped defaults leave open, such as page style, rubric
  scale and feedback voice. Updates never replace either kind.
- Save explicit standing preferences as they are expressed. For example,
  "Never offer extra credit without my explicit permission" becomes a
  remembered standing instruction without a separate file-edit approval.
- Reconcile conflicting reusable guidance so saving a preference is effective,
  rather than leaving another active instruction that contradicts it.
- Preserve meaningful scope, exceptions and stated reasons. Do not generalize
  "for this assignment" into a permanent default.
- Honor explicitly stated standing workflow preferences, including "Whenever
  I explicitly request posting, don't ask me first." If the teacher says not
  to ask, don't. Persist the teacher's actual authorization and scope rather
  than inferring standing permission from a one-task skip.
- Perform routine maintenance and incorporate compatible product updates in
  the background. Acknowledge a newly stated preference naturally.
- Discuss new teaching choices or genuine conflicts when relevant to the
  teacher's work. Keep the existing teacher decision while a conflict remains
  unresolved; avoid an installation-time questionnaire about every change.

## ZIP-fallback installs: update direction

Built 2026-10-17 for ZIP installs. A git install does not use this machinery
(see the current rule above). Install and update use the same machinery,
whether the teacher unzips a new version and says "install" or "update", or
the agent downloads the release ZIP itself.

- The fallback ZIP carries the playbooks and `TEACHER.md` (it excludes only
  `context/` and `Teacher_Override.md`). Unzip a new ZIP into a new folder and
  update from that temporary extraction; never extract it over a teacher's
  folder, where it would overwrite edited playbooks. With UserSync, the ZIP's
  `AGENTS.md` and `CLAUDE.md` would replace UserInstall's local pointer files;
  `playbooks update` archives those copies and restores the pointers.
- `cse playbooks update` (MCP `engine_info` operation `update-playbooks`)
  adds missing files, replaces a playbook only when it still matches the
  previous shipped copy, and reports any other difference as `kept`. With no
  record of a previous version, every difference is the teacher's choice. It
  never touches `context/` or `Teacher_Override.md`.
- Each adopted seed is kept under `shipped/<version>-<hash>/` in UserInstall,
  with `shipped/adopted.txt` naming the last one. The hash keeps development
  builds that share a version string apart. These copies are inactive
  reference material.
- For each `kept` file, the agent reconciles in the background
  (`playbooks/install.md`): previous shipped copy, active file and new shipped
  copy. It keeps every teacher difference, brings in compatible shipped
  improvements, keeps the teacher's choice on conflict, and mentions a new
  teaching option only when it's worth the teacher's attention.

The new product version contributes proposed improvements; it does not acquire
authority over teacher choices.

## Conflicts

1. **Built 2026-10-17.** Whole-file replacement now applies only to playbooks
   that still match what shipped; everything else is kept for reconciliation.
2. **Shipped 2026-10-17.** Rationales are required only for auto-scored quiz
   items (`TEACHER.md`, `quizzes.md`).
3. **Built 2026-10-17.** `score.md` has class-exemplar guidance: two anonymous
   excerpts per prompt across saved current courses, never fabricated, sent
   when the last matching student is scored. `operation: comment-only` now
   works on ordinary assignments as well as New Quizzes and writes only the
   comment, so the follow-up never overwrites a grade the teacher changed.
   Verified in Canvas Live (CS8, 2026-10-17, Test Student): after the grade was
   changed to 18 in Canvas, the follow-up added the exemplar comment and the
   grade stayed 18.
4. **Built 2026-10-17.** The shipped `context/parent-updates.md` and the
   Engine no longer default to one school's portal and tutoring program. Teachers
   supply a portal name and address and an optional tutoring label, text and
   link (`grade_portal_name` and a tutoring label key were added; old key names are
   kept). Blank fields are left out of the email. Existing installs keep their
   saved values.
5. **Shipped 2026-10-17.** New Quiz comments lead with item lines and never an
   overall line. The Engine defect (the default header printed the written
   portion scaled to the whole quiz) is fixed.
6. **Built 2026-10-17; superseded 2026-11-07 and 2026-11-13.** The parent report
   once listed corrections for finished work inside the window. The Corrections
   section is gone; see [parent-update-flow.md](parent-update-flow.md). Nothing
   in a family message promises points or another submission.
7. **Shipped 2026-10-17.** First-task questions and tier offers check saved
   choices first, including ones from an earlier install or another computer.
8. **Dropped 2026-10-17.** Parent emails keep assignment-level scores;
   families can open the linked assignment for item detail.
9. **Shipped 2026-10-17.** One-task review skips are separate from standing
   preferences, which are saved in `Teacher_Override.md`.

The feedback default is also built: `Overall: X/Y`, marked `(late)` when the
agent's late timing says late, then one line per item, then tips and
exemplars. X is the earned score before Canvas's deduction. Verified in Canvas
Live (CS8, 2026-10-17, Test Student), including a real lateness write with a
past due date: Canvas recorded late, deducted 2 points from the earned 16, and
the comment read `Overall: 16/20 (late)`.

## Adopting a new seed with a customized copy (2026-11-13)

When the shipped playbooks change, such as the 2026-11-13 removal of default
correction, extra-credit and points-back offers, a teacher with customized files
adopts them through the existing mechanism; nothing new is needed. A git install
updates by pull: git replaces the shipped playbooks, any edit found at update
time becomes a `Teacher_Override.md` entry once with the old file kept under
`playbook-old/<date>/`, and `context/` is never touched. A ZIP install runs
`update-playbooks`, which replaces only playbooks still matching the previous
shipped copy, keeps every teacher-edited one for background reconciliation, and
leaves `context/` and `Teacher_Override.md` alone. A teacher who already chose a
correction window (`correction_school_days`, `retakes`) or wrote their own
correction rules keeps them; a teacher who relied on the old default gets no
offers until they choose a policy. Existing Canvas assignments are not edited.

## History: first ZIP-era update and plan (2026-10-17)

Dated record of the ZIP-era work; the git rule above is current. Its ZIP
description predates the ZIP carrying playbooks, so extract-over is no longer
safe.

The teacher's own UserInstall (UserSync mode, rules in OneDrive) was updated
to the new Engine (revision 41dface) by swapping in `engine/cse.exe` beside the
running copy; Claude loaded it after a restart. `playbooks update` added
`Teacher_Override.md` and `context/commands.md`, found two playbooks current,
left seven `context/` files alone and reported nine files `kept` because the
install had no `shipped/adopted.txt`. Each of the nine exactly matched a past
template version in Dev's git history, so none held teacher edits; they were
replaced with the new seed and archived under `playbook-old/`. A second update
reported every file current or teacher-owned. The teacher's grade-portal and tutoring labels were added to the saved
parent-update settings, and offline doctor passed.

That first run matched files against Dev's git history by hand. The Engine
now does this itself: it embeds fingerprints of every past shipped version
(`internal/playbooks/past_seeds.txt`), so a colleague's first update replaces
unedited older files automatically. A replay of this install's pre-update
files confirmed it: nine updated, one deliberately edited playbook kept.
Line-ending-only differences (Windows editors, sync clients) count as
unchanged.

A reconciliation check on the same install (2026-10-17) added a test line to
`Teacher_Override.md` and to `playbooks/score.md`, then installed a build whose
shipped `score.md` had changed. Update kept `score.md` and left the override
alone. A three-way merge (previous shipped, teacher, new shipped) applied the
four new shipped lines and kept the teacher line with no conflict. The test
lines were then removed and update reported every file current.

On request, the bell schedule was added to the teacher's own
`context/school-calendar.md`; context files are never replaced by updates.

The same install was then updated to the published 1.0.0 release by the
README's agent steps: downloaded from the permanent link (checksum matched),
extracted over the folder with the running Engine renamed aside, then
`playbooks update`. Extraction replaced the UserSync pointer files with the
full template, update restored both, every playbook was current, and the
teacher's settings, bell schedule and override file were unchanged. Offline
doctor passed.


### Remaining work (as of 2026-10-17)

- Push `dev` and publish a release when the teacher decides (deferred
  2026-10-17).
- A colleague's real update, and an agent (not the developer) doing the
  reconciliation in the background, remain unobserved.

### Done looks like (ZIP-era target)

A colleague with saved preferences (for example a standing review skip, a
different correction window and their own grade-portal link) unzips a newer CE-SE,
points their agent at it and starts working. Every saved preference still
applies, new shipped improvements apply where they don't conflict, and the
teacher was not asked to merge anything.

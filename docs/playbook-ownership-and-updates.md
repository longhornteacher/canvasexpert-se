# Playbook ownership and updates

Working design decisions from the teacher/developer conversation, 2026-10-17.
This document records agreed direction and what has been built. Items marked
shipped or built are implemented; the rest is direction only. Read alongside ARCHITECTURE.md
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
- Parent updates use Engine-resolved Missing/0 classifications: a positive
  teacher-entered grade overrides stale missing/no-submission indicators;
  a current recorded zero belongs in Missing/0 without implying it was
  unsubmitted. Keep grade history, but never repeat Missing/0 items under
  Corrections. Skip only after classification, when every included course
  meets `skip_score` (90 by default) and no Missing/0 items remain. Never
  request parent replies or invent extra-credit opportunities; include only
  existing coursework and teacher-authorized opportunities.

- Parent reports are scoped by the active grading period in
  `context/school-calendar.md`. The Engine inventories all required visible
  work due within the saved retrospective window, resolves student-specific
  effective dates, and limits Missing/0, Corrections and History to that
  inventory. Email and print render every inventory row even when the agent's
  draft selects none. An empty window is an explicit skip. Do not disclose
  grades whose `posted_at` is explicitly null.
- Lateness is separate from score arithmetic. The Engine counts school time
  from the first genuine submission using the calendar; the agent records its
  reasoning and accommodation basis. `late-only` changes only Canvas lateness
  fields on an existing grade, and review remains the normal posting gate.

The school calendar, bell schedule and timezone remain in shipped starter
context; they are the expensive content to recreate. Strip identifying
information (school name) rather than removing that content.

The bell schedule source is the 2026-2027 school bell schedule PDF the teacher
supplied on 2026-10-17. It has four day types:

| Period | Bobcat Hour (Mon-Thu) | Homeroom | Homeroom First | House Days / pep rally (Homeroom Last) |
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

House Days are the pep rally schedule. The PDF lists 5th before 4th on House
Days, and the times confirm that order. Monday-Thursday use the Bobcat Hour
schedule and Friday the Homeroom schedule. Homeroom First and House Days are
called ad hoc; none are scheduled yet.

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

- Teacher decisions live in `Teacher_Override.md`, a separate overlay that
  the agent reads after the shipped playbooks; where they differ, the overlay
  wins. It sits beside `TEACHER.md` under the active rules authority
  (UserInstall, or UserSync when configured), so it follows the teacher across
  PCs. Saving a preference writes the overlay, not the shipped playbook text.
- `Teacher_Override.md` holds only overrides of shipped defaults (for example
  a standing review skip, a different correction window, no tiered-support
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
  I explicitly request posting, skip review." If the teacher says skip review,
  skip review. Persist the teacher's actual authorization and scope rather
  than inferring standing permission from a one-task skip.
- Perform routine maintenance and incorporate compatible product updates in
  the background. Acknowledge a newly stated preference naturally.
- Discuss new teaching choices or genuine conflicts when relevant to the
  teacher's work. Keep the existing teacher decision while a conflict remains
  unresolved; avoid an installation-time questionnaire about every change.

## Update direction

**Built 2026-10-17.** Install and update use the same machinery, whether the
teacher unzips a new version and says "install" or "update", or the agent
downloads the release ZIP itself.

- The ZIP carries only the Engine, launchers and a bootstrap `AGENTS.md`. It
  no longer carries playbooks, `TEACHER.md`, `Teacher_Override.md` or
  `context/`, so extracting it over an existing UserInstall cannot overwrite
  the teacher's setup or edited playbooks. On first run in a freshly extracted
  folder, the bundled Engine lays down the starter files. With UserSync, the
  ZIP's `AGENTS.md` and `CLAUDE.md` would replace UserInstall's local pointer
  files; `playbooks update` archives those copies and restores the pointers.
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
   Engine no longer default to Pearland's Skyward and Bobcat Hour. Teachers
   supply a portal name and address and an optional tutoring label, text and
   link (`grade_portal_name`, `bobcat_hour_label` are new; old key names are
   kept). Blank fields are left out of the email. Existing installs keep their
   saved values.
5. **Shipped 2026-10-17.** New Quiz comments lead with item lines and never an
   overall line. The Engine defect (the default header printed the written
   portion scaled to the whole quiz) is fixed.
6. **Built 2026-10-17.** The parent report lists corrections for finished work
   inside the window, with or without feedback. The playbook includes work
   where the student lost points, points to feedback when it exists, follows a
   teacher's own correction system from `Teacher_Override.md`, and never
   promises points.
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

## First real update (2026-10-17)

The teacher's own UserInstall (UserSync mode, rules in OneDrive) was updated
to the new Engine (revision 41dface) by swapping in `engine/cse.exe` beside the
running copy; Claude loaded it after a restart. `playbooks update` added
`Teacher_Override.md` and `context/commands.md`, found two playbooks current,
left seven `context/` files alone and reported nine files `kept` because the
install had no `shipped/adopted.txt`. Each of the nine exactly matched a past
template version in Dev's git history, so none held teacher edits; they were
replaced with the new seed and archived under `playbook-old/`. A second update
reported every file current or teacher-owned. `grade_portal_name: "Skyward
Family Access"` and `bobcat_hour_label: "Bobcat Hour"` were added to the saved
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

## Remaining work

- Push `dev` and publish a release when the teacher decides (deferred
  2026-10-17).
- A colleague's real update, and an agent (not the developer) doing the
  reconciliation in the background, remain unobserved.

## Done looks like

A colleague with saved preferences (for example a standing review skip, a
different correction window and their own Skyward link) unzips a newer CE-SE,
points their agent at it and starts working. Every saved preference still
applies, new shipped improvements apply where they don't conflict, and the
teacher was not asked to merge anything.

## Git installs (decided 2026-10-21)

A git install inverts the ownership above: git owns the shipped playbooks, `TEACHER.md` and the root `AGENTS.md`/`CLAUDE.md`, and an update replaces them. A teacher's choices live in `Teacher_Override.md`, `context/` and their own root guides, which git never tracks. An agent never edits a shipped file in a git install; a preference goes in `Teacher_Override.md`. An edit that is found at update time is converted into an override once, the old file is kept under `playbook-old/<date>/`, and the file is reset to the shipped text. The `shipped/` copies and the `kept` merge belong to ZIP installs only. UserSync carries `context/` and `Teacher_Override.md`, not playbooks. See [the migration plan](../handoffs/retired/GIT-INSTALL-MIGRATION.md).

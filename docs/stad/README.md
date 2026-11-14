# Team points (STAD) plan

Date: 2026-11-08. Status: **built in Dev; slices 01-07 and 09-10 built (local tests only).** This plan does not authorize Canvas Live writes, classroom use or UserInstall changes. Read [AGENTS.md](../../AGENTS.md), [ARCHITECTURE.md](../ARCHITECTURE.md), [SCORING-RULES.md](../SCORING-RULES.md) and [PRIVACY.md](../PRIVACY.md) first.

## Result

A teacher in any subject runs **Student Teams–Achievement Divisions** (Slavin's STAD) from conversation:

- "Sort my 3rd period into teams." The agent proposes mixed teams of 4–5 from turn-in history and recent work, and explains each placement to the teacher only. The teacher adjusts the teams, and CE-SE creates them as a Canvas group set.
- Each week, students earn points for **turning work in** and for **beating their own recent level**. Team scores are the **average** of the members' points, so teams of 4 and 5 compete fairly.
- "Post this week's standings." A single Canvas Page per course shows team names and team totals, and nothing about any one student.
- "Give Team Comet 3 points for the hallway cleanup." Teacher awards (team points, bonuses) are recorded with a reason.
- Every 4–6 weeks: "Reshuffle." A new round begins. Earlier rounds keep their own rosters and points.

STAD points **never touch the gradebook.** A STAD point is a celebration number, not an assessment.

## Ethos: what is deterministic and what is judgment

The Engine owns anything that should come out the same every time from the same Canvas state and the same saved files. The playbook owns everything a reasonable teacher might do differently.

| Engine (facts, exact math, safe writes) | Playbook `stad.md` (judgment, style) |
|---|---|
| Per-student work facts for a date range: due, on time, late, missing, excused, with enrollment clipping | What "needs a push" means in this class (missing work, late work, or low participation) |
| Each student's percent on each scored item, and on each rubric row | Which items count as STAD evidence (quizzes only? all graded work? one rubric row?) |
| A baseline calculated by the rule written in the rules file | Choosing the baseline rule and the improvement bands for this class and subject |
| Points tallied exactly from facts + rules + awards + rounds | Proposing teams, reading the data with care (new students, skill vs. motivation) |
| Team scores as the average over active members, with ties reported as ties | Team names, celebrations, shout-outs, the wording on the standings Page |
| Creating and updating the Canvas group set from the teams file | When to reshuffle, and how to handle a student who should switch teams |
| The standings Page, republished safely, with team-level numbers only | Whether and how to mention STAD in parent updates |
| Refusing to put individual numbers or role labels on a student-facing surface | Award amounts and caps for team awards |

**Points are derived, not stored.** Apart from teacher awards and round rosters, every point can be recalculated from Canvas Live plus the saved files, so there is no ledger to drift, corrupt or migrate. A late regrade or an excused assignment just changes the next tally.

## What already exists

- **Due work and turn-in state:** `internal/workflow/parent_report.go`, specifically `buildStudentReport` and `workDueRow`. It reads all of a course's submissions in one call (`courseSubmissions`). It applies per-student effective due dates and overrides, skips excused, unpublished and invisible work, and keeps turn-in evidence apart from the grade state. A positive teacher-entered grade resolves a stale Canvas missing flag. Slice 01 should pull this logic into a shared helper rather than copy it.
- **School calendar and grading periods:** `workspace.ReadSchoolCalendar`, used for week and round boundaries.
- **Group set and membership writes:** `PushGroupSet` (`internal/workflow/groups.go`) and `PushGroupMemberships` (`group_memberships.go`). Both are live-checked for book clubs ([group-work slices 01–02](../group-work/README.md)).
- **Enrollments:** `StudentEnrollments` (`canvas_reads.go`), which returns enrollment `created_at` for clipping.
- **Page update:** `push_document` in `page.go` updates in place, but only once per pull (see slice 04).
- **Exact decimals:** `domain.Decimal`, and `percentRat` in the parent report.
- **Identity:** student_ref and alias from `internal/workspace/students.go`. Every STAD file keys students by student_ref.

Nothing named points, standings or STAD teams exists yet. **"Team" already means team teaching** ([docs/team/](../team/README.md)), so code and files use **`stad`** (for example `stad_teams`) and never a bare `team`. Teacher-facing words can still say "teams."

## Files

The STAD files live in `context/stad/<course nickname>/` and follow the teacher-setup authority (UserSync when configured), so a second PC sees the same teams and awards. They are agent-readable and keyed by student_ref, with aliases shown for readability.

### `context/stad/<course>/teams.md`: rounds and rosters

```yaml
schema: 1
group_set: "STAD Teams"          # Canvas group set name, created by the Engine
rounds:
  - name: "Round 1"
    starts: 2026-11-16            # school date, course timezone
    teams:
      - name: "Comets"            # students may rename; Engine syncs the Canvas group name
        members: [s-aaaa, s-bbbb, s-cccc, s-dddd]
```

- **A round's membership is fixed for that round.** A reshuffle appends a new round, and the Engine never rewrites past rounds.
- **A student who joins mid-round** is added to the team's `members` and to its `joined` map (`joined: { s-iiii: 2026-11-23 }`). Their points count from that date.
- **A student who leaves** (no active enrollment) drops out of averages from the date their enrollment ends.
- **Role labels** ("strong", "needs a push") and the reasons behind placements go in the playbook's teacher-only notes (`context/stad/<course>/notes.md`), never in `teams.md`. The Engine never reads notes.md, so a label can't leak through an Engine output.

### `context/stad/<course>/rules.md`: the class's point rules

The playbook writes this file from the teacher's choices. The Engine validates it and applies it exactly.

```yaml
schema: 1
week_starts: monday
effort:                       # per due item, per student
  on_time: 1
  late: 0.5
  missing: 0
team_bonus:
  all_turned_in: 2            # per team per week when every active member turned in every due item
improvement:                  # optional; leave out for effort-only
  evidence: graded_items      # graded_items | quizzes | rubric_row (with rubric_row: "<criterion description>")
  baseline: mean_of_last 3    # earlier scored items in this course; fewer than N → mean of what exists
  missing_counts_as: none     # none (no improvement points) | zero
  bands:                      # percent minus baseline, first match wins
    - { at_least: 10, points: 3 }
    - { at_least: 0,  points: 2 }
    - { at_least: -10, points: 1 }
    - { otherwise: true, points: 0 }
  perfect: { at_least_percent: 100, points: 3 }   # STAD's rule: a perfect score always earns top points
awards:
  cap_per_student_per_week: 3
recognition:                  # optional; levels replace ranks on the standings Page
  basis: week                 # week | round: which team score the levels judge
  levels:                     # highest first; a team takes the first it reaches
    - { name: "Super Team", at_least: 4 }
    - { name: "Great Team", at_least: 3 }
    - { name: "Good Team", at_least: 0 }
```

- **Defaults:** a missing rules file means effort-only with the defaults above.
- **Validation:** an unknown key, an unknown evidence kind, or bands that don't end with `otherwise` are refused with a plain message naming the line.
- **Field names:** the names are a contract that later slices extend. A new field is optional, and its default keeps the old behavior.

### `context/stad/<course>/awards.md`: teacher awards

```yaml
schema: 1
awards:
  - date: 2026-11-18
    student: s-bbbb           # or team: "Comets"
    points: 2
    reason: "bonus: vocabulary challenge"
```

The Engine applies the weekly cap and reports any award it trimmed. It never trims silently.

## Slices

Build them in order. Slices 01–04 give effort-only team points, which is enough for real classroom use. Slices 05–06 add true STAD improvement scoring. Slice 07 is the playbook, which grows with each slice.

### 01: Work-habit facts

- **Command:** `cse work-habits --course <nick> [--from DATE --to DATE] [--students refs]`. MCP: `work_habits`. Read-only.
- **Output:** per student, per ISO week in the window: counts and item lists for due, on_time, late, missing, excused, and no_online_submission.
- **Rules:**
  - Reuse the parent report's due-item selection (effective due dates, excused, visibility, published).
  - **Enrollment clipping:** an item whose effective due date is before the student's enrollment `created_at` is reported as `before_enrollment` and left out of the counts. Canvas's own analytics count those items as missing, which made new students look disengaged in the first real sort.
  - **On time vs. late:** a submission counts as late when Canvas marks it late, so Engine late-timing decisions already written to Canvas take effect.
  - **Missing:** past due with no turn-in, using the parent report's turn-in logic (a positive entered grade is not missing).
- **Also useful outside STAD:** sorting groups, parent-update context, and "who's behind this week."
- **Checks:** fixtures for an override due date, excused work, a mid-period enrollment, a stale missing flag with a positive grade, and a non-online item.

### 02: Teams to Canvas

- **Commands:**
  - `cse stad teams --course <nick>` validates `teams.md` against active enrollments and reports unknown refs, duplicates, and active students on no team.
  - `cse stad push-teams --course <nick>` creates or updates the group set named in `group_set` with one group per team in the **current** round. It reuses `PushGroupSet` and `PushGroupMemberships` and their resume and ID persistence.
- **Reshuffle:** the same group set is updated to match the new round. Past rounds stay in `teams.md` only, because Canvas holds just the current rosters.
- **Not covered:** group sets the teacher made by hand, and anything book-club specific (discussions, pages, files).
- **Checks:** create, rename a team, move one student, a new round, and resume after a partial failure.

### 03: Points tally

- **Command:** `cse stad standings --course <nick> [--week DATE | --round NAME]`. MCP: `stad_standings`. Read-only.
- **What it combines:** slice 01 facts, `rules.md`, `awards.md` and `teams.md`.
- **Output:**
  - Per team: total and per-member average, rank, with ties shared.
  - Per student (teacher-only): effort points, improvement points (once slice 05 lands), awards, awards trimmed by the cap, and team-bonus contributions.
  - Totals for both the week and the round.
- **Team awards and bonuses** are added to the team score directly, not averaged. Student awards are student points, so they are averaged like any other. A week still running gets a provisional `all_turned_in` bonus, judged on work due so far.
- **Leaving students:** an inactive or completed enrollment ends on its Canvas `updated_at` date. Only currently active students' turn-in facts are read, so a student who has left contributes no past effort points, and the divisor still counts them for days they were active.
- **Math:** exact decimals. The team score is the sum of active members' points divided by active members on each day the points were earned. That's equivalent to averaging per week when membership is stable, and it stays fair when a student joins mid-week. Rounding happens only for display, to one decimal place.
- **Output fields carry a `visibility` tag** (`team` or `teacher`). Slice 04 publishes only `team` fields.
- **Checks:**
  - Uneven team sizes.
  - A mid-round joiner.
  - A student leaving.
  - The award cap.
  - The `all_turned_in` bonus with one excused member, who still counts as having turned it in.
  - A regrade between two tallies, where only the derived result changes.

### 04: Standings Page

- **Command:** `cse stad publish --course <nick> --page-title "Team Standings" --body FILE`. MCP: `stad_publish`.
- **Content split:** the agent writes the page body (playbook judgment: celebration wording, team names, shout-outs). The Engine inserts the standings table from the slice 03 `team` fields at a marker in the body (`{{standings}}`), so the numbers are always computed and never typed by the agent.
- **Repeated publishing:** this is a managed page. The Engine records the page ID and the `updated_at` of its last write in `.cse/stad-pages.json`. A republish needs no fresh pull. If the live page changed since CE-SE's last write, the Engine holds the update and reports it, the same protection `push_document` gives.
- **Privacy guard** (cheap and consequential): the Engine refuses a body that contains any student_ref or any roster alias, and it renders only `team` fields. The first version keeps shout-outs at team level. Named student shout-outs wait on open question 2.
- **Checks:** first publish, a republish, a hold after a live edit, and refusal on a ref or alias in the body.

### 05: Improvement points (true STAD)

- **Scope:** extends slice 03 with the `improvement` rules for `graded_items` and `quizzes` evidence.
- **Facts needed:** per student, the percent on each scored item in the course, in order of effective due date. The parent report's `percentOf` / `percentRat` logic applies.
  - The score is Canvas's current score after penalties, unless the rules later say otherwise.
  - Ungraded, unreleased and excused items are not scored items.
- **Baseline:** `mean_of_last N` takes the N most recent scored items **due before** the item being scored. That makes the baseline deterministic and keeps it from reaching into the future.
- **First items:** a student's first scored item has no baseline. It earns the second band's points (matching STAD's "start at the middle"), which the rules can change with `no_baseline_points`.
- **Built as:** `quizzes` evidence means New Quiz assignments (`is_quiz_lti_assignment`) or assignments with a `quiz_id`. A missing item with `missing_counts_as: zero` is a scored 0% item; with `none` it earns nothing and does not enter baselines. Items with no due date are not scored. Per-item rows appear in the teacher-only view.
- **Output:** per scored item, the student's percent, baseline, difference, band and points, so a teacher can see why a student earned what they did.
- **Checks:** fewer items than N, missing items handled both ways, the perfect-score rule, a regrade after a later item was scored, and a late item that is scored but due earlier.

### 06: Rubric-row evidence

- **Scope:** extends slice 05 with `evidence: rubric_row`, for ELA writing, science labs and project rubrics.
- **Facts needed:** per student, the rating points on the named criterion for every assessed submission whose rubric contains a criterion with that exact description. These are read from rubric assessments the Engine already handles for scoring.
- **Percent:** rating points divided by the criterion's points possible.
- **Matching:** the criterion is matched by exact description, the same identity rule the rubric tools use. Rubrics with differently worded rows are not merged. The playbook can tell the teacher to reuse one rubric row or list several descriptions.
- **Checks:** two rubrics sharing the row, a renamed row (not merged, and reported), and an unassessed submission.

### 07: Seed playbook `workspace-template/playbooks/stad.md`

Pure judgment, drafted alongside slice 01 and extended with each later slice:

- **Sorting teams:**
  - Read the slice 01 facts and recent averages.
  - Tell a skill gap from a motivation gap: a student who turns in nearly everything but scores low is a skill gap.
  - Treat new students' history with care.
  - Aim for mixed teams of 4–5 where each team has at least one steady, on-time student.
  - Put role labels and reasons in `notes.md`.
  - Never sort by a protected characteristic.
- **Choosing rules:**
  - Effort-only to start.
  - If missing work is rare in a class, count only **on-time** turn-ins.
  - Add improvement points once each student has two or three scored items.
- **Celebrations:** standings Page wording at grade level, team-name ideas, rotating shout-outs that reach every team over a round, and never naming last place.
- **Reshuffle cadence** and how to explain it to students.
- **Team awards:** the award cap, and why it exists (so one student can't carry a team).
- **Subject notes:** see the table below.
- **Tone:** anything a student or family reads uses plain, warm, grade-level wording.

Routing sentence for the shipped `AGENTS.md`: "For classroom teams, team points or standings, read `stad.md`."

### 09: Recognition levels

- **Why:** a ranked table shows last place even when no wording names it. Slavin's STAD recognizes teams by fixed criteria (Super, Great, Good team), so every team can earn recognition.
- **Rules:** optional `recognition` in `rules.md`; level names and thresholds are the teacher's, written by the playbook. The Engine validates (names unique, `at_least` strictly descending, basis `week` or `round`) and gives each team the first level its exact score reaches.
- **Page:** with levels set, the table lists teams in name order with level and scores and no rank column. Ranks stay in the `stad_standings` output for the teacher.

### 10: Printable team list

- **Command:** `cse stad print-teams --course <nick> [--round NAME]`. MCP: `stad_print_teams`.
- **Why:** on the day teams are announced, the teacher needs students' real names on a projected or posted list, and the agent only knows aliases.
- **Behavior:** one landscape page of team cards with real names from the private roster, saved to the teacher's Downloads folder and opened, like `print_student_reports`. The default is the newest round, so it works before the round starts. The agent gets the path and team count only. A member missing from the roster is refused, never printed blank.

### 08: Capabilities and docs

When a slice lands, update `internal/localui/capabilities.go` and [engine-capabilities.md](../engine-capabilities.md), keeping local tests apart from Canvas Live evidence, and update this status table.

## Status

| Slice | Status | Local evidence | Canvas Live evidence |
|---|---|---|---|
| 01 Work-habit facts | Built in Dev 2026-11-08 (`cse work-habits`, MCP `work_habits`) | Unit and fake-Canvas tests: override due, excused, stale missing flag, non-online item, enrollment clipping | — |
| 02 Teams to Canvas | Built in Dev 2026-11-08 (`cse stad teams`, `cse stad push-teams`; MCP `stad_teams`, `stad_push_teams`) | Fake-Canvas tests: create, reshuffle with rename by shared members, move one student, quiet rerun, hold on unknown ref | Book-club group writes live-checked (group-work 01–02) |
| 03 Points tally | Built in Dev 2026-11-08 (`cse stad standings`, MCP `stad_standings`); improvement points arrive with slice 05 | Fake-Canvas test: uneven teams, mid-round joiner, award cap and trim report, weekly bonus with an excused member, tie ranking, rules defaults and validation | — |
| 04 Standings Page | Built in Dev 2026-11-08 (`cse stad publish`, MCP `stad_publish`) | Fake-Canvas test: create unpublished, identical republish writes nothing, update, hold after a live edit, refusal on a ref, an alias or a missing marker | — |
| 05 Improvement points | Built in Dev 2026-11-08 (inside `stad standings`, when rules.md has `improvement`) | Unit tests: fewer than N, bands, perfect rule, no baseline, same-day items; fake-Canvas loader test for graded_items, quizzes and missing_counts_as | — |
| 06 Rubric-row evidence | Built in Dev 2026-11-08 (`evidence: rubric_row`) | Unit tests: two rubrics sharing a row match by description, a renamed row is not merged, an unassessed submission is not scored; no fake-Canvas run of the rubric read | — |
| 09 Recognition levels | Built in Dev 2026-11-08 (`recognition` in rules.md; `stad_standings` level, `stad_publish` levels table) | Fake-Canvas test: levels at a threshold, name order with no rank column, validation | — |
| 10 Printable team list | Built in Dev 2026-11-08 (`cse stad print-teams`, MCP `stad_print_teams`) | Unit test: newest round by default, escaping, real names, unknown round and unknown member refused | — |
| 07 Playbook | Drafted in Dev 2026-11-08 (`workspace-template/playbooks/stad.md`, routed from the shipped `AGENTS.md`) | Playbook tests that cover shipped files | — |

## Across subjects

The Engine sees only scored items, rubric rows and turn-ins, so it doesn't care about the subject. The playbook picks the evidence.

| Subject | Natural improvement evidence | Notes |
|---|---|---|
| Math | `quizzes` (weekly quizzes; classic STAD) | The textbook case. Short, frequent quizzes make baselines settle fast. |
| ELA | `rubric_row` (one writing criterion, e.g. Evidence) or `graded_items` | Essays are infrequent, so effort points carry the early weeks. |
| Science | `rubric_row` (lab-report criterion) plus `quizzes` | Lab groups may differ from STAD teams. Keep them separate. |
| Social studies | `graded_items` or `rubric_row` (DBQ criterion) | |
| Computer science | `graded_items` (autograded labs) | Autograded scores cluster high. Consider `on_time` effort weighting and a tighter top band. |

## Settled decisions (2026-11-08, developer)

1. Engine = deterministic facts, math and safe writes. Playbook = judgment, style, subjectivity. No reader-facing wording or point policy hard-coded in Go: point values come from `rules.md`.
2. Points never touch the gradebook. Gradebook credit is a separate teacher choice; nothing here offers it to students by default.
3. Students see team-level results only. Individual numbers and role labels are teacher-only.
4. Points are derived from Canvas Live plus saved files. Only rounds and awards are stored.
5. Team score is the average over active members.
6. Code and file names use `stad` so they don't collide with team teaching.
7. A holiday week stands as a short week; it is not merged or skipped (was open question 1). The average already handles it.
8. Owned courses (team teaching): STAD writes are a group set and a Page, which are content, so a request naming the teammate is enough. No `(CE)` tag is needed (was open question 4).

## Open questions

1. **Late-timing source:** use Canvas's late flag (proposed), or the Engine's own late-timing decision when one exists for that assignment? These differ only when the teacher hasn't posted the decision yet.
2. **Shout-outs with names:** the standings Page is student-facing. Should a cleared shout-out use the student's real first name? The Engine knows it, the agent does not. Or should shout-outs stay team-level only in the first version (simplest)?
3. **Rounds across grading periods:** should a round end automatically at a grading-period boundary? The proposal is no: rounds belong to the teacher.

## Out of scope

- Automatic team sorting in the Engine (that's judgment).
- Live or real-time leaderboards.
- Badges or Canvas Credentials.
- Cross-course competitions (period vs. period). The slice 03 output makes that a playbook-only follow-up later.
- Student-facing individual progress pages.
- Writing points to Canvas grades.

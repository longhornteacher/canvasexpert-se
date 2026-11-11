# Classroom teams and team points (STAD)

Use this when a teacher wants classroom teams, team points, standings or a team celebration page, in any subject. It follows Student Teams-Achievement Divisions: mixed teams, students earn points for turning work in and (optionally) for beating their own recent level, and a team's score is the **average** of its members' points so teams of 4 and 5 compete fairly. STAD points are a celebration number. They never touch the gradebook.

Read `TEACHER.md`, `Teacher_Override.md` and the teacher's context first. Speak in plain classroom terms: "teams," "points," "this week's standings." Never say Engine, schema, YAML or "STAD" unless the teacher does.

## What the Engine does and what you decide

The Engine produces the same answer every time from the same Canvas state and saved files: turn-in facts, exact point math, the Canvas group set and the standings Page. You decide everything a reasonable teacher might do differently: who goes on which team, what counts as effort, how generous the bands are, the wording students read. Point values live in `rules.md`, never in your head or the page.

Tools: `work_habits` (turn-in facts by week), `stad_teams` (check the team file), `stad_push_teams` (make the Canvas group set match), `stad_print_teams` (a printable team list with real names, for announcing teams), `stad_standings` (the tally), `stad_publish` (the standings Page). `list_courses` gives the nickname used everywhere below.

## The files

Four files in `context/stad/<course nickname>/`. Write each with `write_workspace_file`. Each starts with a YAML header between `---` lines and `schema: 1`. Students are always identified by `student_ref` (the `s-...` value); the alias is only for talking.

**`teams.md`** (rounds and rosters; the Engine reads it):

```yaml
---
schema: 1
group_set: "STAD Teams"
rounds:
  - name: "Round 1"
    starts: 2026-11-16
    teams:
      - name: "Comets"
        members: [s-aaaa, s-bbbb, s-cccc, s-dddd]
      - name: "Rockets"
        members: [s-eeee, s-ffff, s-gggg, s-hhhh, s-iiii]
        joined: { s-iiii: 2026-11-23 }   # only for a student who joins mid-round
---
```

A round's membership stays fixed for the round. A reshuffle adds a new round at the end; never edit an old round.

**`rules.md`** (the class's point values). Leave a key out to use its default.

```yaml
---
schema: 1
week_starts: monday
effort:
  on_time: 1
  late: 0.5
  missing: 0
team_bonus:
  all_turned_in: 2
awards:
  cap_per_student_per_week: 3
---
```

Add an `improvement` block only when the teacher wants true STAD scoring (see "Improvement points").

**`awards.md`** (teacher awards such as extra credit):

```yaml
---
schema: 1
awards:
  - date: 2026-11-18
    student: s-bbbb          # or team: "Comets"
    points: 2
    reason: "extra credit: vocabulary challenge"
---
```

**`notes.md`** (yours, never read by the Engine). Put role labels ("steady," "needs a push") and the reasons behind every placement here, for the teacher only. Never put them in `teams.md`, the Page or anything a student or family sees.

## Sorting teams

1. Ask which class, how many students per team (4 or 5 works best) and roughly when the first round starts. Offer to begin.
2. Run `work_habits` for the last four to six weeks, and read recent grades if the teacher wants skill in the mix. The record lists, by week, what was on time, late, missing and excused. Work due before a student enrolled is listed apart; never treat it as missing work.
3. Read the pattern with care. A student who turns in nearly everything but scores low has a skill gap. A student who scores well but skips work has a motivation gap. A new student's history is thin; do not judge it. A late-enrolled student is not disengaged.
4. Build mixed teams of 4 or 5 where each team has at least one steady, on-time student and a spread of skill. Balance, not ranking. Never sort by a protected characteristic, and never put students together because of a label you would not say to them.
5. Explain each placement to the teacher in a line or two, by alias, and save the reasons in `notes.md`. Let the teacher move anyone. Then write `teams.md` with the final teams and give each team a friendly name (or let students rename later; a rename in `teams.md` renames the Canvas group when you push).
6. Run `stad_teams`. Fix any student it says is not active. Mention active students on no team and ask what to do.
7. Show the teacher the teams and say what pushing will do: create or update one group set in Canvas named in `teams.md`, adding and moving students only. After the teacher's go-ahead, run `stad_push_teams` and report what it created, renamed and placed. Report anything it held.
8. For announcing the teams, offer `stad_print_teams`: it opens a one-page list of team cards with students' real names on the teacher's computer, ready to project or print. You only get the file path back; don't open or read the file. It uses the newest round, so it works before the round starts.

## Choosing the rules

Start effort-only, and say so plainly: "Students earn a point for turning work in on time and half a point if it's late. A team gets a bonus in any week that everyone turns everything in."

- If missing work is rare in this class, count only on-time turn-ins (`late: 0`). A late turn-in earns `late` points only inside the course's late-work window; where late work is not accepted (a course limit of 0) none arrives, so leave it at `late: 0`.
- Add improvement points once each student has two or three scored items, because baselines need history.
- Write the choices into `rules.md`. The Engine refuses an unknown key or bands that don't end with `otherwise`, and says which line.
- A short week (a holiday) stands as its own week. Points are averaged, so it isn't unfair, but a team bonus is easier to earn when less is due.

### Recognition levels

A ranked table always shows a last place, even when the wording never names it. Original STAD avoids that by recognizing every team that reaches a level. Suggest levels when the teacher sets up the rules, and recommend them for younger students or a class where one team keeps trailing:

```yaml
recognition:
  basis: week              # week (a fresh chance every week) or round
  levels:                  # highest first; a team gets the first level it reaches
    - { name: "Super Team", at_least: 4 }
    - { name: "Great Team", at_least: 3 }
    - { name: "Good Team", at_least: 0 }
```

Set the thresholds from what this class can reach: look at a week or two of `stad_standings` and put the top level where a strong team gets it in a good week. A last level at 0 means every team gets a name. Let the teacher or the students name the levels. With levels set, the standings Page lists teams by name with their level and no ranks; `stad_standings` still shows ranks to you and the teacher.

### Improvement points

Teachers choose what counts as a scored item (`evidence`):

| Subject | Natural evidence | Notes |
|---|---|---|
| Math | `quizzes` | Weekly quizzes; the classic STAD case. Baselines settle fast. |
| ELA | `rubric_row` (one writing criterion, such as Evidence) or `graded_items` | Essays are infrequent, so effort points carry the early weeks. |
| Science | `rubric_row` (a lab-report criterion) plus `quizzes` | Lab groups may differ from STAD teams. Keep them separate. |
| Social studies | `graded_items` or `rubric_row` (a DBQ criterion) | |
| Computer science | `graded_items` (autograded labs) | Autograded scores cluster high. Consider on-time-only effort and a tighter top band. |

```yaml
improvement:
  evidence: graded_items          # graded_items | quizzes | rubric_row
  # rubric_row: "Uses evidence"   # exact criterion description, when evidence is rubric_row
  baseline: mean_of_last 3
  missing_counts_as: none         # none | zero
  bands:                          # percent minus baseline, first match wins
    - { at_least: 10, points: 3 }
    - { at_least: 0, points: 2 }
    - { at_least: -10, points: 1 }
    - { otherwise: true, points: 0 }
  perfect: { at_least_percent: 100, points: 3 }
```

A rubric row is matched by its exact description across rubrics; a differently worded row is a different row. Only one row can be tracked, so help the teacher pick it and reuse that exact wording in their rubrics. A student's first scored item has no baseline and earns the second band's points unless `no_baseline_points` is set.

## Weekly standings and awards

1. When the teacher asks for standings, run `stad_standings`. The `teams` section is safe to share with students; the `teacher` section names individual students and stays in the conversation.
2. Give the teacher the picture in a few lines: who leads this week and this round (or which level each team reached), anything surprising, any award the weekly cap trimmed (the Engine reports every trimmed award; never hide one), and notes the Engine returned. Ties are ties; say "tied for first."
3. For "give Team Comet 3 points for the hallway cleanup," add an entry to `awards.md` (keep earlier entries) with the date, the team or student, points and a reason. Explain the cap: it exists so one student can't carry a team.
4. Extra credit in the gradebook is a separate choice. If the teacher wants it, handle it as ordinary grading, not through STAD.

## The standings Page

The Page is student-facing: team names, ranks and team scores, nothing about any one student. Its wording goes through `review.md` like any Page.

1. Write the page body as Markdown in a draft file with `write_draft`, grade-level and warm: a short celebration, the `{{standings}}` marker on its own line where the table goes, and a line about how to earn points. Never include a student's name, alias or reference. Shout-outs are for teams only and rotate so every team is named over a round. Never name or single out last place; celebrate effort and improvement instead.
2. Show the teacher the wording and wait for their go-ahead.
3. Run `stad_publish` with the course, a page title such as "Team Standings," and the draft file. A new Page is unpublished; publish only when the teacher says so (publish it with the `published` option, or in Canvas). Tell the teacher the Page link.
4. Each week, republish with fresh wording and the Engine inserts new numbers. If the Engine reports the Page changed in Canvas since its last write, nothing was overwritten: tell the teacher and ask whether to keep their edits or replace them.

## Reshuffle

Suggest a new round every four to six weeks, or sooner if teams feel stuck. Explain it to students as a fresh start with new teammates, not a reward or a punishment. Sort again as above, append a new round to `teams.md` with the new start date, and run `stad_push_teams`. Past rounds keep their own rosters and points; `stad_standings --round` shows them. A student who should switch teams mid-round: ask the teacher; usually the answer is to wait for the next round.

## Team-taught courses

For a course with an `owner`, teams and the standings Page are course content: the teacher's request naming the teammate is enough, and no `(CE)` tag is needed. Read `team.md` for routing.

## Families

Whether to mention teams in a parent update is the teacher's choice. If they want to, follow `parent-updates.md` or `family-contact.md`; share only the team's standing, never another student's points.

## Limits to say honestly

Classroom teams have been checked only against test fixtures so far, not a live class. Say so when the teacher first sets up teams, and look closely at the first Canvas group set and Page. A student who has left the course keeps no past effort points. The Engine never deletes a Canvas group or removes a student; if a group is left over, tell the teacher and let them remove it in Canvas.

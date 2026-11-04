# HANDOFF G: General parent-email defaults; Engine counts accommodated deadlines

Date: 2026-11-04. Slice G of the [unified architecture review](../Unified%20Architecture%20Review.md). Two independent parts:
- **G1**: parent email defaults. Layer: Engine (`internal/workflow/parent_drafts.go`), seed context, playbook and docs.
- **G2**: deadline arithmetic. Layer: Engine (`internal/workspace/school_calendar.go`, `internal/workflow/late_prepare.go`) and playbooks.

Do them as separate commits.

Before starting: `git fetch`, bring `dev` up to date, read root `AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/SCORING-RULES.md`, [docs/parent-update-flow.md](../../docs/parent-update-flow.md), the seed `playbooks/parent-updates.md`, `playbooks/score.md`, `playbooks/group-work.md`, and the [shared execution notes](../README.md#execution-notes-for-the-active-handoffs). Facts mapped at `2818e36`.

**Privacy note for this handoff and its commits.** This is a public repository. Do not write the developer's school name, tutoring program name, its sign-up host, or class names into new code, tests, seed files, docs or commit messages. This handoff refers to them by file and line instead. The developer's real values belong in their own `context/parent-updates.md` in UserSync.

---

## G1: Parent email wording comes from context, with general defaults

### Decision (developer, 2026-11-04)

- Public seed uses general, reusable defaults.
- The school's actual calendar **dates** stay in the seed, under generic names.
- Identifiable specifics (class name, tutoring program name and link, portal name) live in the teacher's own context in UserSync.
- Keep old-key compatibility and every grade-report guarantee.

### Current code

`workflow.ParentConfig` (`parent_drafts.go:25–39`) is read by `loadParentConfig` (`:41–65`) through `workspace.ReadTeacherDocument(root, "context/parent-updates.md", …)`. That already routes to UserSync when configured (`context/` is shared in both install kinds). A missing file is tolerated and defaults are applied.

Current YAML keys: `schema`, `skyward_url`, `grade_portal_name`, `bobcat_hour_url`, `bobcat_hour_label`, `bobcat_hour_text`, `sign_off`, `skip_score` (default 90), `correction_days` (default 14, range 1–60). The comment at `:27–28` says the key names are kept "for compatibility".

There are two layouts, selected at `:159` and dispatched at `Render` `:483`:
- **default** (`render`, `:519–696`);
- **`weekly_update`** (schema 2, `renderWeekly`, `:740–825`). The printed student sheet always uses `render(…, true)`.

Hardcoded in Go, all in `parent_drafts.go`:

| Line | What |
|---|---|
| 172–173 | weekly refuses to build without `sign_off` **and** the tutoring URL. The seed playbook `parent-updates.md:5` says "a blank tutoring section is left out". |
| 750 | weekly subject with a hardcoded class name: `"<class> Update – Week of %s %d"` |
| 786 | failing cutoff `if percent >= 70 { continue }` |
| 804 | greeting naming the subject area ("Here's a quick <subject> update") |
| 805, 807, 810, 813, 816, 818 | section text: "Work that needs attention (%s)", "No missing or failing work…", "Missing:", "Failing (below 70):", "All missing work should be done as soon as possible.", "Major grades this week" |
| 822 | the "Extra help" line naming the school's tutoring program and printing a fixed link text, ignoring the configured label/text, with no blank-URL branch |
| 823 | "Thank you,<br>%s" (sign-off) |
| 659–669, 590, 605, 672, 674, 684–687, 695 | default-layout headings, footer, default subject "Class update for %s (%s)" — already generic; leave the wording, but 674 prints a tutoring line without the link |

Other occurrences of the specifics:
- tests: `internal/workflow/parent_weekly_test.go:29,31,48,68,96`;
- seed: `workspace-template/context/parent-updates.md:7–9` (keys only, blank values) and `workspace-template/context/school-calendar.md:42,44,51` (bell-schedule body naming the tutoring period);
- docs:
  - `docs/parent-update-flow.md:20` (names the developer's original tutoring choice), `:3,26,44` ("weekly … layout" named for one subject);
  - `docs/playbook-ownership-and-updates.md:97–120,214–216,252`;
  - `engine-docs/formats.md:318–320`, `engine-docs/field-limits.md:69`;
- `internal/localui/capabilities.go:91,94`;
- seed playbook `parent-updates.md:5,74,84,89`.

Run `rg -n -i` for the tutoring program name, the sign-up host and the class name to get the full list. Do not paste them into new files.

Grade-report guarantees that must hold unchanged:
- **Free-form contact runs** carry no grade report (`parent-update-flow.md:7`).
- **Every `work_due` row renders** in email and printable reports (`:17`, `:38`; seed context `:30`).
- **Honest flags:** incomplete flags prevent an all-clear statement, and unresolved evidence is a hold (`:46`).
- **Privacy:**
  - excused and unreleased work is never shown as failing (`:48`);
  - no student names, aliases or refs in the weekly message (playbook `:89`);
  - scores never appear in free-text notes (`checkFreeText`/`scoreLike`).
- **Code that enforces these:** `validateParentReportScope` (`:369–459`), `validateWeeklyAnnouncements` (`:312`), `aiNotice` (`:700`).

### What to build

1. **New generic keys** in `ParentConfig`, all optional:

   | New key | Replaces / adds | Behavior |
   |---|---|---|
   | `class_name` | new | used in the weekly subject and greeting; blank → subject "Class update – Week of …" and greeting "Here's a quick class update." |
   | `failing_below` | new (was hardcoded 70) | percent cutoff for "Failing"; default 70; validate 1–100 |
   | `grade_portal_url` | `skyward_url` | same behavior |
   | `tutoring_url`, `tutoring_label`, `tutoring_text` | `bobcat_hour_url/label/text` | same; label default "Tutoring", text default "Sign up for tutorials and clubs" |

2. **Old keys keep working.** Read an old key when its new key is blank. If both are set and differ, fail with a clear message naming both keys rather than choosing silently. The developer's UserSync file uses the old keys today and must keep producing the same email after this change, except for the class name (step 6).
3. **Weekly layout uses the configuration.** Subject and greeting from `class_name`; the failing cutoff from `failing_below` (also in the "Failing (below N)" text); the "Extra help" block from `tutoring_label`/`tutoring_text`/`tutoring_url`. A **blank tutoring URL omits the Extra help block**, so remove the URL requirement at `:172` and keep `sign_off` required. The default layout uses `failing_below` too if it has a failing notion; check.
4. **Seed.** Update `workspace-template/context/parent-updates.md`: the new keys, blank or general values, comments in plain teacher words. Do not list the old keys as recommended; one comment line can say older names still work.
5. **School calendar seed.** Keep every date. In the body (`school-calendar.md:42,44,51`), rename the school-specific schedule/period names to generic ones (for example "Tutorial period (Mon–Thu)", "Homeroom (Fri)"). Keep times only if they are needed as an example, and label them "example times; replace with your school's". The parser (`SchoolCalendar`, `school_calendar.go:15–23`) does not read the body, so this is text only.
6. **The developer's own context.** Their UserSync `context/parent-updates.md` needs `class_name` added to keep today's subject line. Editing UserSync is not authorized by this handoff. At the end, tell the developer the one line to add (and that the old tutoring keys keep working), or make the edit only with their explicit go-ahead in the conversation.
7. **Docs and playbook.**
   - Rename the "Weekly <subject> update/layout" wording to "weekly update" in `parent-update-flow.md`, the seed playbook, `formats.md`, `field-limits.md` and `capabilities.go:91,94`.
   - Replace `parent-update-flow.md:20`'s named original choice with "the teacher's own tutoring link, if any".
   - Update the seed playbook `parent-updates.md:5` key list and `:89`.
   - Coordinate `playbook-ownership-and-updates.md` with [HANDOFF E](HANDOFF_E_Contract_Consistency_2026-11-04.md) pass 3. Whichever lands second rebases on the other.

### Done when

- `parent_weekly_test.go` uses synthetic values (`class_name: Science 6`, `tutoring_url: https://tutoring.example/signup`) and asserts:
  - the subject/greeting come from `class_name`;
  - a blank `class_name` gives the generic subject;
  - `failing_below: 65` moves a 66% row out of Failing;
  - a blank tutoring URL builds and omits Extra help;
  - old `bobcat_hour_*`/`skyward_url` keys still render the same block;
  - conflicting old/new keys fail.
- Existing parent report/send tests pass unchanged (inventory, privacy, skip, holds).
- `rg -n -i` for the school's tutoring program name, its sign-up host and the hardcoded class name returns nothing in `internal/`, `cmd/`, `workspace-template/`, `engine-docs/`, or the non-history `docs/`. `handoffs/retired/` is history and stays.
- Full checks pass. Update `capabilities.go`/`engine-capabilities.md` together if their wording changed.

---

## G2: The Engine turns an approved "+N school days" into a deadline

### Decision (developer, 2026-11-04)

Move only deterministic calendar arithmetic to the Engine. Eligibility and the allowance choice stay with the teacher/agent. Build for the requested workflow, not a general accommodation language.

### Current behavior

- Seed `score.md:89` (step 3): "Resolve the applicable documented allowance to one absolute RFC3339 deadline using `context/school-calendar.md`. Compare it with the student's pulled `effective_due_at` and the documented base deadline: if Canvas's individualized date already includes the allowance, use that date once. Never add the allowance twice. … send the resolved `accommodated_deadline` and a private `accommodation_basis`…"
- `group-work.md:65` applies the same rule per group member.
- The agent counts school days by hand, which is exactly the hand math the project assigns to the Engine.
- **Input today.** `LateTimingStudentDecision` (`late_prepare.go:25–30`) has `student_ref`, `first_genuine`, `accommodated_deadline` (absolute RFC3339) and `accommodation_basis` (private text). The two must come as a pair (`validateLateTimingDecision`, `:99–117`). Groups use `LateTimingGroupDecision{group_id, members[]}` (`:32`). MCP `PrepareLateTimingInput{packet, students, groups}` is at `internal/mcpserver/server.go:82–86`; CLI `prepare-late-timing FILE`; contract text in `engine-docs/formats.md:187` and `docs/mcp-adapter.md:29`.
- **Calculation.** `calculateTiming` (`late_prepare.go:343–388`) uses `accommodated_deadline` instead of Canvas `EffectiveDueAt` as the counting deadline, while keeping Canvas `due_at` as the write guard. The calendar is read only when work is late. Groups in shared mode require equal deadlines across members (`:443`).
- **Existing calendar math.** `SchoolCalendar.CountSchoolTime(deadline, submitted)` (`school_calendar.go:192–250`) walks local-midnight days in the calendar `timezone`. It skips Saturdays, Sundays, `non_school_dates` and expanded `non_school_ranges`, and errors outside `start_date`–`end_date`. There is **no "add N school days" function.**
- **The private `ExtraTime` flag** (`workspace.Student.ExtraTime`) reaches packets as `extra_time` (`pull_work.go:499`, `group_pull.go:485`) but is never read by timing code. It stays an evidence fact for the agent.
- **Tests.**
  - `school_calendar_test.go`: `TestCountSchoolTime*` (:75, :120, :128), `TestReadSchoolCalendarUsesActiveUserSync` (:138);
  - `session_discovery_test.go`: `TestCalculateTimingCountsAccommodatedDeadlineWithoutChangingCanvasGuard` (:356), `TestPrepareLateTimingUsesEstablishedExtraTimeDeadlineOnce` (:373, which hand-supplies a "one additional school day" deadline).

### What to build

1. **`SchoolCalendar.AddSchoolDays(base time.Time, n int) (time.Time, error)`**, beside `CountSchoolTime`:
   - n ≥ 1; result = base's local wall-clock time on the n-th school day after base's local date, in the calendar timezone (handle DST by constructing the local time, not adding 24h);
   - a school day is a weekday that is not in `non_school_dates` or a `non_school_ranges` range — the same rule `CountSchoolTime` uses; share the predicate;
   - if base falls on a non-school day, counting still starts from the next calendar day (so +1 from a Saturday 23:59 is Monday 23:59, if Monday is a school day);
   - error if the result would fall outside calendar coverage; the agent then holds that student's timing, as today.
2. **Input.** Add an optional `allowance_school_days` (int) to student and group-member decisions:
   - exactly one of `accommodated_deadline` or `allowance_school_days`, each paired with `accommodation_basis`;
   - the base is the pulled Canvas `effective_due_at` by default;
   - an optional `allowance_base` (RFC3339) lets the agent name a different documented base deadline.

   The Engine computes the deadline. The returned timing provenance shows the base, the N, and the resulting deadline so the check-in can state it.
3. **Unchanged.** "Already included in Canvas's date" stays agent judgment: when Canvas's individualized date already includes the allowance, the agent sends no allowance (or `accommodated_deadline` = that date) exactly as now. Do not add allowance defaults; do not read `ExtraTime` in the Engine.
4. **Contract text.** Update the MCP schema hints for the new fields, `engine-docs/formats.md:187` ("help formats"), `docs/mcp-adapter.md:29`, the synthetic example `engine-docs/examples/synthetic-late-timing-decisions.yaml`, and seed `score.md:89` / `group-work.md:65`. The agent decides eligibility, N and whether Canvas already includes it, then sends `allowance_school_days`; it no longer counts days. Keep `accommodated_deadline` documented for absolute documented dates.

### Done when

- **`AddSchoolDays` tests:**
  - +1 across a weekend;
  - +2 across a `non_school_dates` day;
  - +3 across a `non_school_ranges` range;
  - a DST change keeps the wall-clock time;
  - a base on a non-school day;
  - a result past `end_date` errors.
- **`prepare-late-timing` tests:**
  - `allowance_school_days` produces the same timing as the hand-supplied deadline in `TestPrepareLateTimingUsesEstablishedExtraTimeDeadlineOnce`;
  - both fields at once → validation error;
  - missing basis → error;
  - shared-mode group with differing member allowances still holds.
- Full checks pass, and the compiled MCP smoke still sees 58 tools with the new schema fields.
- Capabilities: add a local-evidence line for allowance arithmetic in both records.

## Not in this slice

A general accommodation language, Engine-chosen allowances, changes to quiz-time accommodations (`QuizTime`/`ReducedChoices`), changing Canvas due dates.

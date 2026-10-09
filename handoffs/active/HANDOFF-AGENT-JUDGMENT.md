# CE-SE: hand judgment back to the agent, name the Engine and Playbook, fix the review findings

**Handoff date:** 2026-10-09. **Baseline:** `87179b5` plus the uncommitted working tree from the retired [lean-spine handoff](../retired/HANDOFF-LEAN-SPINE.md). `go test ./...` passed on that tree when this handoff was written.

**Status:** active. Tasks 1–8 below. Move this file to `handoffs/retired/` and add a one-line completion note at the top when the work is finished.

## Purpose

### Vocabulary (use these words in code comments, docs and reports)

"SE" has been used for both the program and the instructions it ships, which hid the problem this handoff fixes. Use these names instead:

| Name | What it is | Where it lives | Its job |
|---|---|---|---|
| **CE-SE** | The whole product | this repo | n/a |
| **Engine** | The Go program: `cse.exe` CLI plus the local hub pages | `cmd/`, `internal/` | Mechanical work the agent can't do reliably: talk to Canvas, keep student, attempt and assignment identity straight, exact score arithmetic, safe write recovery. **No educational or "does this matter?" judgment.** |
| **Playbook** | The instructions CE-SE ships for the agent | `workspace-template/AGENTS.md`, `CLAUDE.md`, `guides/`, `examples/` | Tells the agent *how* to judge: scoring, lateness, feedback, when to ask the teacher |
| **Teacher settings** | The teacher's own preferences and notes | `settings/*.md` (edited by hand or in the hub) | Voice, student notes, courses, school calendar |
| **Agent** | Claude working in the teacher's workspace | n/a | Makes the judgment calls, using the Playbook and settings, then hands decisions to the Engine |

Rule of thumb for any new behavior: if it needs judgment, it goes in the Playbook; if it needs Canvas access, exact identity or exact math, it goes in the Engine.

In this vocabulary, CE-SE divides work like this:

- **The agent** brings judgment: what the work says, which attempt is genuine, how late something really was, whether a directions edit matters.
- **The Engine** (`cse.exe`) carries out the agent's decisions against Canvas Live, keeping identity, arithmetic and write recovery honest.

The previous round left some judgment calls inside the Engine. When the Engine tries to make a judgment, it either needs prerequisites the agent doesn't need (the calendar), or it uses a mechanical stand-in for meaning that gets it wrong (HTML normalization). This handoff moves those calls back to the agent. It also fixes the four problems found by the teacher's reviewer (Astra) in the previous execution.

The teacher made these decisions on 2026-10-09:

1. **Lateness is the agent's reasoning.** The agent works out lateness timing in the scoring session. The Engine just provides the tool to write Canvas's late-timing fields safely to the right attempt.
2. **The Engine doesn't look at assignment directions when posting.** Directions are for the agent: it reads them at pull time to decide how to score. Nothing the Engine computes or writes at post time uses them. Posting holds only when something the arithmetic or the Canvas write depends on has changed: points possible, the rubric, grading type, or anonymous/moderated/group mode. (An earlier draft of this handoff had post print a "directions changed" note. The teacher dropped that: a teacher who edits directions mid-session already knows.)
3. **No legacy saved results need support.** The teacher knows of no interrupted or failed post with real student grades on an older build. Delete the legacy compatibility code and fields (confirm first; see Task 4).
4. Fix all four of Astra's findings: HTML comparison, the calendar requirement, recovery based on message text, and packaging.

## Read first (short)

- Root [AGENTS.md](../../AGENTS.md): the engineering default. Proportionate tests; no frameworks; continue independent work.
- [ARCHITECTURE.md](../../ARCHITECTURE.md) and [SCORING-RULES.md](../../SCORING-RULES.md): current policy. You will **edit** their lateness paragraphs (Task 7) and their naming (Task 8).
- Don't re-read the retired 0.1b brief or the historical evidence doc. They are not inputs.

## The lesson from the last round (apply it to every task)

Astra's summary was: "verification was occasionally aimed at the implementation rather than the intended workflow." So for every task below:

1. **Reproduce the teacher's actual inconvenience first.** Write the failing regression test using the concrete example in this brief, before changing code.
2. **Remove unnecessary prerequisites.** Don't add new ones. If a check isn't needed to post an honest grade to the right student and attempt, leave it out.
3. **Check the integration boundary you touched:** what `cse` prints, the packaged ZIP, the shipped guides. Make it a targeted check, not a new test matrix.

Tests stay proportionate. Each task lists the specific regressions it needs. Don't add more unless something actually fails.

## Starting state

- Branch `dev`. The working tree has **uncommitted** changes from the previous round (modified Go files, new `late_timing.go`, `outcomes.go`, `settings/school-calendar.md`, the retired handoffs). Don't discard or reformat them.
- **Before editing, ask the teacher whether to commit the current tree as a baseline** (recommended: one commit, "Lean spine: grading policy, school calendar, spec rewrite"). This keeps this handoff's diff readable. Don't commit without their yes.
- Go is installed (`go1.27.1`); a pinned copy also lives at `.tools/go1.27.1/go/bin/go.exe`.
- The teacher doesn't write code. Ask questions in plain English, briefly, with a recommendation.

## Survey: what the Engine does today, and who should own it

This was reviewed on 2026-10-09 to find other judgment calls the Engine makes on the agent's behalf. **Moves** means this handoff changes it. **Keep** means it's mechanical or protects honesty; don't move it.

| Engine behavior | Where | Verdict |
|---|---|---|
| Counting school-day lateness from a calendar | `late_timing.go` `SchoolSeconds`, `postLate` | **Moves to agent** (Task 1) |
| Verifying the agent's "first genuine attempt" choice against pulled and live history | `postLate` | **Moves to agent** (Task 1). The agent records it in `basis` instead. |
| Deciding whether a directions edit "changes academic expectations" via HTML normalization | `edit_plan.go` `SameGradingBasis` → `academicHTML` | **Remove** (Task 2). The agent reads directions; the Engine ignores them at post. |
| Comparing submission body HTML at post time, after attempt identity already matched | `post.go:328` | **Remove** (Task 2). Redundant with the attempt number and timestamp check, and only produces false holds when Canvas re-serializes HTML. |
| Requiring the agent to restate the computed `assessment` exactly | `scoring.go` `ValidateResults` | **Moves to the Engine** (Task 3). Arithmetic is the Engine's job, so the Engine should fill this value in. |
| `evidence: unknown` for one student fails validation for the whole results file | `scoring.go` `ComputeAssessment` via `ValidateResults` | **Fix** (Task 3). Hold that one student and post the rest. The guide tells the agent to mark unreadable work `unknown`, so today following the guide blocks the whole class. |
| Legacy recovery branch that matches English message text | `post.go:317` | **Delete** (Task 4) |
| Legacy fields `late_forgiveness`, `desired_entered_grade`, `retained_grade`, `TimingTarget` | `domain/types.go` | **Delete** (Task 4) |
| Weighted rubric arithmetic (`q_j = Σ a_i s_ij`, normalized to P) | `scoring.go` `ComputeAssessment` | **Keep.** The agent chooses weights; the Engine does exact arithmetic. |
| Prepending the score and criterion lines to feedback | `render.RenderFeedback` | **Keep.** It guarantees the numbers students see match what was posted. Voice stays the agent's. |
| Student, attempt, assignment and course identity checks before every write | `post.go`, `postSources` | **Keep.** Core safety. |
| Pending, uncertain or preserved write recovery; never blind replay | `post.go`, `addFeedback` | **Keep** |
| Discovery filters (excused, unassigned, unsupported mode, already graded current attempt) | `pull_work.go` | **Keep.** These are mechanical facts reported in the pull summary. |
| Edit-flow conflict check: did the live description change since pull, and did Canvas store our description? | `edit_plan.go` `sameEditFields`, `author.go` `editPatchMatches` | **Keep, but fix `academicHTML`** (Task 2), since these remain its only callers |
| Error text "outside 0.1b" | `canvas_reads.go:265-274`, `edit_plan.go:26,29` | **Tidy** (Task 6): "not supported yet" |

If you find another place where the Engine makes an educational or "does this matter?" judgment, don't fix it in this pass. List it in your final report.

---

## Task 1: lateness becomes an agent-supplied value

### New results-file contract

Replace the current `late_timing` shape. Field name stays `late_timing`, under `grade`:

```yaml
late_timing:
  status: late            # "late" or "none"
  seconds_late: 97200     # required and > 0 when status is late; omit or 0 when none
  due_at: "2026-10-05T23:59:00-05:00"   # the student's applicable due date the agent used (RFC3339)
  basis: "First genuine work is attempt 2 (attempt 1 was blank), submitted Wed 10/7 2:59 AM. Due Mon 10/5 11:59 PM. School days counted: Tue 10/6 all, Wed 10/7 to 2:59 AM = 27 h. Excluded: none."
```

- `status` must be `late` or `none`. When it's `late`, `seconds_late` must be a positive integer. When it's `none`, `seconds_late` must be 0 or absent.
- `basis` is required and must not be blank. It's for the teacher, not the Engine. The Engine never parses it.
- `due_at` is required. It's the one cheap identity check worth keeping: if the teacher extends the deadline between pull and post, the agent's seconds are stale and posting them would apply a wrong penalty.
- Omitting `late_timing` means "don't touch Canvas lateness", as it does today (`not-requested`).

### Engine behavior at post (`postLate`, rewrite in place in `late_timing.go`)

Delete `SchoolCalendar`, `SchoolDateRange` and `SchoolSeconds`. The Engine no longer reads `settings/school-calendar.md` at all. The new `postLate`:

1. Fetch the live submission (as now). If `!sameAttempt(g.Attempt, attemptRef(live))`, hold late only: "new submission before late timing adjustment".
2. If the live `cached_due_date` doesn't match `late_timing.due_at` (use `sameDate`), hold late only: "due date changed since the agent worked out lateness; redo timing". If `cached_due_date` is null (no due date), hold late only with that message.
3. Build the target from the agent's value, not by computing it. If `timingMatches(live, target)`, mark it `preserved`.
4. Ambiguous recovery: if the prior outcome is pending or uncertain and the live value matches, it's `preserved`; otherwise it stays `uncertain` (no replay). Delete the `TimingTarget` field and the "calendar or timing inputs changed" branch. The target *is* the saved request now, so there's nothing separate to compare.
5. Otherwise, save `pending`, then PUT `late_policy_status` (and `seconds_late_override` when status is late), confirm the response (same `sameAttempt` and `timingMatches` check as today), and save.

Keep `timingMatches` as it is. Remove the old checks that `first_attempt` exists in pulled and live history; that choice is the agent's, recorded in `basis`.

Validation (`domain` package, wherever `late_timing` is checked now): only the shape rules above. Don't validate `due_at` against pulled work. The live check at post covers it.

### Reproduce first: regression tests (replace the two in `late_timing_test.go`)

- **On time, no calendar file anywhere in the workspace:** `status: none` is written and confirmed. *This is Astra's finding #2 reproduced; it must pass with no `settings/school-calendar.md` present.*
- **Late:** `status: late, seconds_late: 97200` sends exactly those two fields; outcome `succeeded`; grade and comment unaffected.
- **Due date changed since the agent's value:** late is held, and grade and comment still post.
- Keep (or adapt) one existing ambiguous-recovery test so `uncertain` stays non-replaying. Don't write a matrix beyond these.

### Agent-facing docs (Task 7 covers wording; the content to convey)

- The agent selects the first genuine submission from actual current and prior work (blank, placeholder or accidental attempts don't count). Later drafts inherit that first genuine submission's timing.
- The agent reads `settings/school-calendar.md`, the student's applicable due date from the pulled work (`effective_due_at` on attempts), and any teacher or student notes (extensions, accommodations). It counts elapsed clock time from the due time to the first genuine submission, counting only student school dates: weekdays not listed as non-school dates or ranges, in the calendar's time zone, with full calendar days rather than class hours. Early-release days count as full days.
- **Do the arithmetic with a quick script (PowerShell is available), not mental math.** Put a human summary in `basis`.
- **Before posting, show the teacher late timing in days and hours** ("1 school day, 3 h; skipped Mon 10/12 holiday") in the same summary or preview as scores. On "score and post", report it in the post summary.
- If anything is unclear (no due date, can't tell which work is genuine, calendar doesn't cover the dates), leave `late_timing` out, tell the teacher specifically what's missing, and continue grading.
- Canvas alone calculates the point penalty from these fields. The Engine never adjusts points for lateness.

The calendar file stays as plain reference data the agent reads. Keep its frontmatter; it's convenient for the agent. Rewrite its trailing note (see Task 7).

---

## Task 2: the Engine stops judging directions; HTML comparison fixed

### 2a. Grading basis at post: compare only what posting depends on

In `edit_plan.go` `SameGradingBasis`: also blank `DescriptionHTML` on both sides, so the comparison covers points, rubric (criteria, maxima, ratings), grading type, submission types, and the anonymous/moderated/group flags. That's what `SameSnapshot` already compares once the presentation fields are cleared. Update its doc comment to say "fields that posting arithmetic or writes depend on". Don't add any directions notice or report field. The Engine simply doesn't consider directions at post.

`postSources` (results vs. packet basis) and `rubric_attach.go` (lines 49, 136) also call `SameGradingBasis`. Both are fine with the narrower comparison: rubric attach doesn't write the description, and the attach-rubric validator already requires the exact body. Confirm that by reading them, but don't add tests for it.

### 2b. Drop the submission-body comparison

`post.go:328`: delete the `academicHTML(source.work.Current.BodyHTML) != academicHTML(live.Body) || !sameAttachmentIDs(...)` hold. Canvas doesn't let a student or teacher change a submission without creating a new attempt, and the line just above already holds when the attempt number or `submitted_at` differs. Delete `sameAttachmentIDs` if nothing else uses it.

### 2c. Fix `academicHTML` for its remaining callers (edit flow only)

After 2a and 2b, `academicHTML` is used only by `sameEditFields` (has the live description changed since pull?) and `editPatchMatches` (did Canvas store what we sent?). Fix the two defects Astra reproduced:

- **Whitespace inside tags:** `<p >` must equal `<p>`. Inside a tag (outside the `pre`/`code` literal region), collapse runs of whitespace to a single space and remove spaces before `>` and `/>` and around `=`. Keep attribute values exact.
- **Whitespace around inline elements:** `a <b>x</b>` must **not** equal `a<b>x</b>`. In text segments, collapse each run of whitespace to a single space instead of removing it. Trim only at the very start and end of the whole document.
- Color or other attribute changes stay **different**. That's correct for the edit flow, because the teacher actually changed the content. Don't try to make semantic equivalence work.

### Reproduce first: regression tests

- `post`: pull with description `<p>Explain.</p>`; live has a completely different paragraph → grades post. Live points changed 10 → 20 → grade held with the existing "grading basis changed" message.
- `academicHTML` table test, three rows: `<p>Explain.</p>` == `<p >Explain.</p>`; `a <b>x</b>` != `a<b>x</b>`; `<span style="color:red">x</span>` != `<span style="color:blue">x</span>`.
- Delete or adjust any existing test that asserted a description-only change holds grading, or that a body-HTML change holds posting.

---

## Task 3: less scoring-file friction

### 3a. The Engine fills in `assessment`

The agent currently has to restate the exact two-decimal computed score, or validation fails. Change `ValidateResults` (or the post path) so:

- If `grade.assessment` is **absent**, the Engine uses the computed `a.Posted` and writes it into the results file on the first save, so the teacher can see it in the file.
- If it's **present and different**, keep the error. That mismatch means the agent misunderstands the scoring, and the message already shows the correct value.
- Same rule for `revise_feedback.assessment.assessment`.
- `cse validate <results>` should print each student's computed assessment so the agent can show the teacher a preview without posting. Keep it simple, for example one line per student: `student_ref  assessment  late: 1d 3h | none | —`. The late column formats `seconds_late` as school days (24 h each) plus hours.

### 3b. Unknown evidence holds one student, not the file

When a component has `evidence: unknown`, that student can't be scored honestly, but everyone else can. Make `ValidateResults` return that student with a hold reason (e.g. a `Hold string` on `ValidatedStudent`) instead of failing the whole document. In `Post`, a student with a hold reason gets `holdGrade(g, "unreadable work for component X; not scored")`, and the loop continues. Other malformed-input errors stay fatal. A broken file should still fail before any write, as today.

### Regression tests

- Results with `assessment` omitted → posts the computed value, and the saved file now contains it.
- Two students, one with an `unknown` component → the other student's grade and comment post; the unknown student shows `held` with the message; exit code 3.

---

## Task 4: delete the legacy recovery branch and fields

1. **Confirm, don't assume.** Ask the teacher for their real workspace folder if you don't know it (likely in their synced district OneDrive). Search its `courses/**/*.md` for `late_forgiveness:`, `desired_entered_grade:`, `retained_grade:`, `timing_target:`, or a grade outcome `status: preserved` whose message isn't `computed assessment is present on intended attempt`. The teacher believes there are none. If you find any, stop and tell them which files before deleting; don't build a migration.
2. Delete the branch at `post.go` top of `postGrade` (the `g.Outcomes.Grade.Status == "preserved" && g.Outcomes.Grade.Message != ...` block) and its comment.
3. Delete `GradeOperation.DesiredEnteredGrade`, `GradeOperation.LateForgiveness`, `GradeOperation.TimingTarget`, the `Timing()` helper (use `g.LateTiming` directly), and `FeedbackAssessment.RetainedGrade`, plus their comments. Decoding is strict (`KnownFields(true)`), so old files with those keys would now fail to parse. That's acceptable per the teacher's answer.
4. Delete any tests that only exist to exercise those legacy shapes.

No new tests. The existing grade-recovery tests must still pass.

---

## Task 5: package the calendar

`scripts/package.ps1` `Test-TemplatePathAllowed` (line ~57): change `'^settings/(courses|grading|students)\.md$'` to include `school-calendar`. The executable already embeds the whole template (`embed.go`, `all:workspace-template`), so only the ZIP was affected.

**Boundary check:** run `.\scripts\package.ps1` per [docs/build.md](../../docs/build.md) and list the workspace ZIP's entries to confirm `settings/school-calendar.md` is included. If the script refuses for an unrelated environment reason, report that instead of working around it. Don't add a test for the allowlist.

---

## Task 6: tidy

- `canvas_reads.go` `UnsupportedAssignment` and `edit_plan.go` `PlanEdit`: replace "outside 0.1b" with plain wording ("anonymous grading isn't supported yet", "editing an existing rubric's criteria isn't supported yet; keep the pulled rubric", "moving an existing assignment between modules isn't supported yet"). Update any test that matches the old text.
- Don't rename the `0.1b` version string or package names; that's release work.

---

## Task 7: docs (do this after the code lands, in the same session)

Keep edits short and replace contradicted text; don't add new philosophy sections.

| File | Change |
|---|---|
| `SCORING-RULES.md` | Rewrite the lateness paragraph: the agent determines lateness from actual work, the calendar and notes, and supplies `status`, `seconds_late`, `due_at` and `basis`. The Engine writes only `late_policy_status` and `seconds_late_override` to the current attempt, after checking the attempt and due date. Canvas calculates penalties. Missing or unclear inputs mean omitting timing, telling the teacher, and continuing. |
| `ARCHITECTURE.md` | Lateness bullet: the same change in one or two sentences. In "Teacher decisions", add: "Posting holds only for changes to points, rubric, grading type or grading mode; the Engine does not read directions at post." Remove "The CLI computes elapsed school-date seconds…". Update the date line. |
| `workspace-template/guides/score.md` | Replace the "For timing…" paragraph with the agent procedure from Task 1 (script the arithmetic, show days and hours before posting, record `basis`). Add: "Mark unreadable work `evidence: unknown`; that student is held and everyone else posts." Add: "`assessment` may be omitted; the Engine fills in the computed value. `cse validate` previews it." |
| `workspace-template/guides/formats.md` | Replace the `late_timing` paragraph (line ~38) with the new fields. In the settings paragraph (line ~9), say the calendar is reference data the agent uses for lateness; drop "schema" as an Engine requirement. |
| `workspace-template/guides/setup.md` §5, `workspace-template/README.md` | "used to describe elapsed lateness" → "the agent uses it to work out school-day lateness". |
| `workspace-template/settings/school-calendar.md` | Trailing note: "Weekends are always excluded. The agent uses this calendar to work out school-day lateness; Canvas computes point penalties. Early-release dates count as full dates. Update dates for your school." |
| `docs/build.md` | Only if it lists the packaged settings files, add the calendar. |

---

## Task 8: name the Engine and the Playbook across the project

The teacher chose these names on 2026-10-09. Goal: the teacher, the agent in their workspace, and any future builder can always tell which part does what. Apply the Vocabulary table (Purpose section) everywhere a sentence describes **who does something**. Write the names capitalized: **Engine**, **Playbook**.

### Rules

- Program doing something (CLI, hub, Go code, "SE" meaning the program) becomes **the Engine**.
- Instructions for the agent (workspace `AGENTS.md`, `CLAUDE.md`, `guides/`, `examples/`) are **the Playbook**.
- The product as a whole stays **CE-SE** (or "CanvasExpert-SE" in titles).
- "Host" or "host agent" becomes **the agent**.
- Keep command examples exact: `.\cse.cmd post <file>` stays as written. On first mention per file, say "the Engine (`.\cse.cmd`)" so the agent connects the name to the command.

### Do not rename (these would break things or rewrite history)

- Product branding in the hub: "CanvasExpert: SE", "Special Edition", page titles, `SEExpansion()`.
- **The credential target `CanvasExpert-SE/Canvas/v1`** and the `%LOCALAPPDATA%\CanvasExpert-SE\program.txt` path. Renaming either would make every computer lose its saved Canvas token or program link.
- Folder names, file names, the `cse` command, Go package and identifier names, the `0.1b` version string, ZIP names.
- Error messages where "SE" is just the product name ("saved SE credential is invalid").
- History: `handoffs/retired/` and `docs/0.1b-evidence.md`.

### Files and what each needs

| File | Change |
|---|---|
| `ARCHITECTURE.md` | Near the top, add the Vocabulary table and rule of thumb from this handoff (about ten lines). Relabel the spine diagram: `Agent (follows the Playbook) -> Engine: cse CLI / hub -> Canvas Live`, with teacher settings and work files beside it. Title the package table "Engine packages". Replace every "SE"/"CLI" that means the program with "the Engine" (e.g. "SE adjusts lateness timing only" becomes "the Engine writes the lateness timing the agent supplies"). Add a short "Playbook" line saying where it lives and that judgment belongs there. |
| `SCORING-RULES.md` | "SE and the host assess work" becomes "The agent assesses work; the Engine posts it". "New SE rubric attachments" becomes "Rubrics the Engine attaches". Every "the CLI" becomes "the Engine", and "the host" becomes "the agent". Link to the ARCHITECTURE vocabulary; don't copy the table. |
| Root `AGENTS.md` (development) | Add one line: "CE-SE has two parts: the **Engine** (Go code in `cmd/`, `internal/`: Canvas access, identity, exact math, safe writes) and the **Playbook** (`workspace-template/` instructions: all judgment). Put judgment in the Playbook, not the Engine." Keep the file short. |
| `docs/build.md` | Say the build produces the Engine (`cse.exe` in the program ZIP) and the workspace ZIP carries the Playbook plus starter teacher settings. |
| `handoffs/README.md` | One clause: handoffs should say whether a change belongs to the Engine or the Playbook. |
| `workspace-template/AGENTS.md` | Open with two sentences: "This file and `guides/` are your **Playbook**: they tell you how to make the judgment calls. `.\cse.cmd` runs the **Engine**, which talks to Canvas, keeps students and attempts straight, and does exact math; it carries out your decisions and doesn't make them." Then swap "the CLI"/"SE" accordingly. |
| `workspace-template/CLAUDE.md` | Check it; if it only points to AGENTS.md, leave it. |
| `workspace-template/guides/*.md` | "the CLI" becomes "the Engine" (first mention "the Engine (`.\cse.cmd`)"); "SE does/doesn't…" becomes "the Engine…". Fold this into the Task 7 edits to the same files. |
| `workspace-template/README.md` (teacher-facing) | One short plain-English paragraph for the teacher: the Engine is the program that talks to Canvas; the Playbook is the instructions the agent follows; your settings are yours to edit in the hub. |
| `workspace-template/settings/*.md` | Body notes only, where they say what "SE" or "the CLI" does (the school calendar note from Task 7 already uses "the agent"). Keep each file's empty-starter markers exactly; `package.ps1` refuses the ZIP without them. |
| `program-template/README.txt` | "This folder holds only the program" becomes "This folder holds only the program (the CE-SE Engine)". |
| `cmd/cse/main.go` help text | First line: `CanvasExpert-SE 0.1b — the Engine: Canvas assignments and scoring`. Last lines: "Read the Playbook guides (guides/setup.md, …) as needed." Update `main_test.go` only if it asserts that exact text. |
| Go comments | Only fix comments that describe the program making a judgment, or that say "host"/"SE" decides something (e.g. the `SameGradingBasis` doc comment from Task 2). No mass comment rewrite. |
| Hub page text (`internal/localui`) | Leave as is. It's teacher-facing branding and already says "the agent". |

### Check (part of Done)

Run this **before you start** (it's your worklist) and **again at the end**. It covers current docs, excluding `handoffs/` (briefs describe the rename itself) and `docs/0.1b-evidence.md`. On 2026-10-09 it listed 19 lines across `ARCHITECTURE.md`, `SCORING-RULES.md`, `docs/build.md`, the workspace `AGENTS.md` and `README.md`, three guides (`authoring`, `score`, `setup`), and two settings files. A few are false positives (e.g. "host protocol" in ARCHITECTURE's don't-build list); leave those.

```bash
grep -rn -E '(^|[^-])\bSE\b|the CLI|\bhost\b' --include=*.md . | grep -v -E 'handoffs/|0\.1b-evidence|\.cache|\.tools'
```

At the end, every remaining hit must be the product name (e.g. a `CanvasExpert-SE-...zip` file name) or a literal command, not a description of who does what. List any intentional exceptions in the final report. Also spot-check the non-Markdown files in the Task 8 table (`program-template/README.txt`, `cmd/cse/main.go` help) by reading them.

---

## Execution plan for Sonnet with subagents

The code tasks overlap heavily in `post.go`, `domain/types.go` and `domain/scoring.go`, so **the main session does Tasks 1–4 and 6 sequentially itself.** Parallel agents editing the same files would conflict. Use subagents only where the work is genuinely separate:

| When | Subagent | Job | Tools / isolation |
|---|---|---|---|
| Before coding | 1 × Explore (read-only) | Find every reference to `late_timing`, `LateForgiveness`, `Timing()`, `TimingTarget`, `SchoolSeconds`, `academicHTML`, `SameGradingBasis`, `sameAttachmentIDs`, `RetainedGrade`, `DesiredEnteredGrade` across `internal/`, `cmd/`, tests and `workspace-template/`. Return a file:line list only. | Read-only; no worktree |
| In parallel with coding | 1 × general-purpose | Task 5 (package allowlist plus running the package script and listing ZIP entries). It touches only `scripts/package.ps1`. | Same tree is fine; tell it not to touch Go files |
| After code passes | 1 × general-purpose | Task 7 docs **and** Task 8 naming (they touch the same files; do both in one pass), given the final field names and behaviors from you in its prompt. Paste in the Vocabulary table, Task 7, and Task 8 in full. Tell it: replace contradicted text, keep edits short, no new philosophy sections, obey Task 8's do-not-rename list. | Same tree; docs and help text only |
| Final | You | Review the docs agent's diff yourself (read it, don't spawn a reviewer), run `go test ./...`, `go vet ./...`, and build `cse.exe` per docs/build.md. | |

Use at most these three subagents. No review teams, no worktrees, no workflow orchestration. Give each subagent a self-contained prompt with exact file paths and the relevant section of this handoff pasted in, so it doesn't re-read the repo.

### Order inside the main session

1. Teacher baseline-commit question (Starting state). Then start Task 5's subagent and the Explore subagent.
2. Task 4 step 1 (search the teacher's workspace for legacy files). Ask the teacher for the path if needed. If you're waiting on an answer, continue with Task 2.
3. Task 2 (tests first, then code).
4. Task 1 (tests first, then code). It builds on the `types.go` cleanup, so do Task 4 steps 2–4 right before or as part of it.
5. Task 3, then Task 6.
6. `go test ./...` and `go vet ./...`. Start the docs subagent (Tasks 7 and 8).
7. Build and package. Confirm the calendar is in the ZIP (from the Task 5 agent's result, re-run if code changed packaging inputs).
8. Run the Task 8 naming check yourself and fix stragglers.
9. Final report to the teacher (below). Don't commit unless the teacher asks; offer to.

## Done means

- Each "reproduce first" scenario above has a passing test that mirrors the teacher's example, and `go test ./...` and `go vet ./...` pass.
- `settings/school-calendar.md` appears in the packaged workspace ZIP.
- No Engine code reads the calendar; `SchoolSeconds` and the legacy fields are gone; there are no message-text comparisons in control flow (`grep -n 'Message !=' internal/` and `grep -n 'Message ==' internal/` return nothing that decides behavior).
- The docs in Task 7 agree with the code. The shipped guides tell the agent how to compute lateness and show it to the teacher.
- Task 8's naming check passes: docs, the Playbook, and help text say **Engine** and **Playbook** where they describe who does what, and nothing on the do-not-rename list changed (in particular, the credential target and `program.txt` path are untouched).
- Non-test Go source should shrink overall. Report before and after counts (`find internal cmd -name '*.go' ! -name '*_test.go' | xargs wc -l`). The baseline for `internal` + `cmd` non-test was 8,872 lines on 2026-10-09. This is a sanity signal, not a target.

## Final report to the teacher (plain English, short)

- What changed, in teacher terms: "the agent now works out lateness and shows it to you before posting", "editing directions no longer blocks grading", "one unreadable paper no longer blocks the class", "the calendar now ships in the ZIP", "the docs now call the program the Engine and the agent's instructions the Playbook".
- Anything you found and didn't fix (other Engine judgment calls, packaging script surprises, legacy files in the workspace).
- Test and build status, stated plainly with any failure output.
- Move this handoff to `handoffs/retired/` with a one-line completion note.

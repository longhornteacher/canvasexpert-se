# CanvasExpert-SE: architecture

**Updated:** 2026-10-08 · **Status:** planning; no code yet. Open questions are in §11. Items marked **[verify]** need a real Canvas or school-PC check before the code that depends on them is written.

## 1. What this is

A personal tool, shared with friends. **It is not a product and not SaaS.** A teacher points an agent at a folder and says "let's work." The agent then authors Canvas content, pulls student work, scores it, gives feedback and handles differentiation through the teacher's own Canvas Personal Access Token (PAT).

**Success means speed and accuracy in the moment.** Every design choice is weighed against those two. Defenses against rare failures are worth having only when they cost almost nothing. This is K-12: an occasional "oops" that the teacher fixes in Canvas is acceptable. Slow or fussy tooling is not.

CanvasExpert (CE) is an ancestor in spirit only. SE has **no code, layout, language or tool-name obligations to CE.** CE is a reference for hard-won Canvas knowledge (see [CE-LESSONS.md](CE-LESSONS.md)).

## 2. Settled decisions

| # | Decision | Consequence |
|---|---|---|
| D1 | Using SE with an agent is itself the approval signal. | No approval flags, attestations or policy checks. |
| D2 | Real names are permitted. **Aliases are a light hygiene layer for file names and metadata only** (§7). | No vault and no scrubbing of student writing. The agent does see names that appear inside student work and in `students.md`. |
| D3 | Student PII may live in the teacher's district cloud folder. | Pulled data and the student table sit in the synced workspace. |
| D4 | Everything pulled from Canvas can be pulled again. | Pulled data is a disposable cache. No backup system, ledger or snapshots. |
| D5 | One teacher, **one machine at a time**, across several machines: HomePC, PersonalLaptop (main), WorkLaptop. | No locks or sync merging. The workspace syncs; credentials are set up once per machine. |
| D6 | The agent drives a **command-line tool** and reads and writes **plain files**. | No tool schemas in the agent's context. A capability is a command plus a short guide. |
| D7 | Primary hosts: **Claude Code and Codex.** Cowork and ChatGPT Work are later and optional (§4.3). | `AGENTS.md` holds the instructions. `CLAUDE.md` is a one-line pointer to it. |
| D8 | When unsure whether a write happened, **SE checks Canvas itself** and asks only if it is still truly unsure. | No send journal. |
| D9 | **Scoring rule:** the feedback comment always contains SE's score. SE writes the score field **only if it is empty or SE's score is higher than the current *entered* score** (before late penalty). | The grade never goes down. Canvas's late policy does the penalty math. Matches Bridge "highest wins." |
| D10 | The teacher's instruction is the authorization. "Score and post" means do it; "draft" means don't post; "show me first" means stop for review. | No mandatory preview/apply pairs and no confirmation tokens. |
| D11 | Configuration is files in the workspace. **The WebUI covers the PAT and the student table** (alias, monitored status). | The teacher edits student info in a page the agent never reads. |
| D12 | CE and SE may run on the same machine. | Separate credential name, port, command name and folders. |
| D13 | Forge contracts are a starting point, not scripture. | Lean SE authoring formats (§5). |
| D14 | Language is open; the code will be vibecoded. | Decided in a WorkLaptop session (Q9). |
| D15 | **No installs and no admin rights**, Windows 11 only. **WorkLaptop is the baseline**, since colleagues have the same model and permissions. | Everything ships inside the workspace folder. It must run from a user-writable folder on WorkLaptop. |
| D16 | **SE never applies late penalties.** It only adjusts how late a submission counts. | §6.2. |
| D17 | **Feedback layout:** score on the first line, rubric scores under it, then conversational glows and grows. | One house format in `guides/score.md`, which `settings/grading.md` can override. |
| D18 | **Differentiation:** the teacher tags students by hand in Canvas. The agent assigns tiered content to those tags. Hub is the default and Bridge is used when better suited. Optional built-in hints go in every assignment. | §6.3. Tags remain the legal record of who received what. |
| D19 | **Look:** clean and easy for 7th graders. Styling exists only for attention and clarity. | A small, fixed visual vocabulary in the renderer. |
| D20 | **Canvas Live is the source of truth** once something is pushed. Teacher edits made directly in Canvas always win. | Drafts exist only to create. Edits start from a fresh pull of the live item (§5.2). |
| D21 | **Use Canvas's official rubric tools**: rubrics attached to assignments and filled in during scoring. | §6.4. |
| D22 | **Feedback when SE fills in the rubric grid:** the comment carries only the overall score plus glows and grows. The rubric breakdown goes in the comment only when SE did *not* fill in the grid. | Students never see the same rubric twice. |
| D23 | Grade posting and the nightly Skyward ↔ Canvas sync (11:45 PM) are **not SE's concern.** | SE never reads or changes posting policy and never reports on visibility. |
| D24 | Courses are known by teacher nicknames: **CS8, ELA7PAP, ELA7**. | `settings/courses.md` maps nickname → Canvas course. Commands and folders use nicknames. |
| D25 | **No to-do command.** The teacher says what's needed. **"Scoring session"** is the trigger phrase for pulling everything that needs scoring across current courses. | `cse pull-work` with no assignment named = everything ungraded or resubmitted in current courses (§6.1). |
| D26 | Module placement uses agent common sense. The teacher's usual pattern is `Week of 10/2 - <topic>`, and it is advisory only. | A one-line hint in `settings/courses.md`; no module rules engine. |
| D27 | **Distribution:** the teacher shares a ZIP of a starter workspace via OneDrive. Updates are a new ZIP or a replaced program file. | No `cse update`, installer or GitHub download. **[verify]** whether files unzipped from a OneDrive download get "blocked from the internet" marks that stop the program running on WorkLaptop (test T3). |
| D28 | **"Scoring session" pulls, then asks.** After pulling everything, the agent reports what's there and asks the teacher what they want. "Score and post" is the usual answer, but the conversation decides. | Pull never implies post. D10 still governs what gets posted. |
| D29 | **Submissions are words and code**: typed text entries, pasted code and text uploads. No file-heavy work is expected. | `pull-work` focuses on text. Attachments are still downloaded if present, but there is no special handling for media or external tools in v1. |
| D30 | **New Quizzes are for auto-scored items only.** Written responses are ordinary Assignments. | SE doesn't attempt New Quiz manual grading. Keep an eye on Canvas adding per-item score/feedback APIs. |
| D31 | **Differentiation tags** are Silver / Red / Blue (= Support / Core / Accelerate), **in ELA7 only.** CS8 and ELA7PAP are not tiered. | Tier labels map to tag names in `settings/courses.md` per course. Courses without tags never offer tiers. |
| D32 | **Feedback revision is a first-class feature:** "this feedback needs revising" triggers revising comments SE already posted. | §6.5. |

## 3. Environment constraints

- Windows 11, standard user, no admin. Nothing system-wide, no services, no reboots.
- Allowed: files in the user's own folders, the user's own Windows Credential Manager, and loopback HTTP.
- **[verify on WorkLaptop]** whether unsigned `.exe` files run from a user/OneDrive folder (AppLocker/WDAC), and PowerShell's language mode. This decides Q9.

## 4. Shape of the system

```text
Teacher ──talks to──> Agent (Claude Code / Codex)
                         │ reads AGENTS.md + guides
                         │ runs `cse <command>`        (fast, prints compact text)
                         │ reads/writes files in the workspace
                         ▼
          cse ── [alias names on files/metadata] ──HTTPS + PAT──> Canvas
                         │
                PAT from Windows Credential Manager (per machine)
                         ▲
        `cse setup` / `cse students` → loopback WebUI (PAT, student table)
```

No server process, database or background worker. Each `cse` call is short-lived. It may run many Canvas requests in parallel *within* the call. Every command is a plain function (arguments in, text out), so a thin adapter for other hosts can be added later (§4.3).

### 4.1 Workspace folder

```text
<Workspace>/                     ← synced cloud folder; the agent is pointed here
  AGENTS.md                      ← the only always-loaded instructions; short
  CLAUDE.md                      ← "@AGENTS.md"
  .claude/settings.json          ← deny rule: the agent never reads .cse/
  .cse/                          ← SE's private state (synced across machines)
    students.csv                 ← alias, Canvas/SIS IDs, real name, monitored (edited via WebUI)
  cse/                           ← the program itself (no install; Q9)
  guides/                        ← loaded only when the task needs it
  settings/                      ← teacher-editable text
    courses.md                   ← nickname → course (CS8, ELA7PAP, ELA7), module naming hint, alias on/off
    grading.md                   ← feedback voice, rubric habits, overrides
    students.md                  ← accommodations and notes, real names
  courses/<course-nickname>/
    drafts/                      ← new content not yet pushed
    pulled/                      ← disposable cache, including live items pulled for editing
    scoring/<assignment>/        ← scores and feedback for one grading job
```

- The source repo (this folder) is separate from the workspace. It holds the code, guides and a workspace template, and builds the `cse` program that gets copied into a workspace.
- The CLI prints compact summaries to stdout. Large material goes to files, and the CLI prints their paths.
- Formats: Markdown with a YAML header for anything the agent writes; Canvas HTML with a YAML header for live items pulled for editing; original bytes for attachments.

### 4.2 Credentials

- The PAT and Canvas URL live in the Windows Credential Manager of **each machine**. Run `cse setup` once per machine.
- The PAT never appears in files, stdout, logs or the agent's context. An expired token produces "run `cse setup`".

### 4.3 Cowork and ChatGPT Work

Cowork runs commands in an isolated VM. From there it can't run Windows programs, read Credential Manager, or (probably) reach Canvas. **[verify]** ChatGPT Work. If wanted later, a `cse mcp` mode would expose the same functions as a few MCP tools on the Windows side. It is not in the first build.

## 5. Authoring (priority 1)

### 5.1 Create

- The agent writes a draft in `drafts/`: a Markdown body plus a YAML header (title, points, dates, submission types, module, published, rubric, tiers/tags).
- The renderer adds the fixed 7th-grade visual vocabulary: numbered steps, a "what to turn in" box, collapsible "Need a hint?" supports, and callouts.
- `cse push <draft>` creates the object(s) and prints the Canvas link(s). It then moves the draft to `drafts/pushed/`, stamped with the link. From then on the draft is only a record. It is never pushed again; a push of a stamped draft is refused, with a pointer to §5.2.
- **Ambiguous create failure:** look for the exact title created in the last few minutes, and adopt it if found. Otherwise retry once, and ask only if it still can't tell (D8).
- No dry run, catalog scan or post-push verify. Canvas's response is the result.
- **New Quizzes** use the New Quizzes API with a compact Markdown question format. Item-level scores and feedback cannot be posted by API.

### 5.2 Edit (Canvas Live wins)

A new agent session knows nothing about earlier pushes, and doesn't need to:

1. `cse pull-item <course> <title or link>` writes the **live** item as a file: Canvas's settings in a YAML header and Canvas's actual HTML as the body. Any changes the teacher made directly in Canvas are in it.
2. The agent makes targeted edits to that file (HTML in, HTML out; no lossy Markdown round trip).
3. `cse push <pulled file>` updates Canvas. If the item changed in Canvas after the pull, it re-pulls and tells the agent to reapply its edit instead of overwriting. That costs one cheap timestamp check and protects the teacher's live changes.

The same rule applies to scoring: `pull-work` always reads the live assignment and rubric, so teacher changes in Canvas automatically take priority over anything in an old draft.

## 6. Scoring and feedback (priority 2)

### 6.1 Default flow

0. **"Scoring session"** (D25): `cse pull-work` with no assignment named finds every assignment with ungraded or resubmitted work across current courses, and pulls them all in one call. It prints a per-course, per-assignment summary. Naming a course or assignment narrows it.
1. **`cse pull-work <course> <assignment>`** — one call. It fetches the live assignment, the attached Canvas rubric (criteria and rating IDs) and all submissions **with attempt history**, existing comments and rubric assessments. Attachments download in parallel. It writes:
   - one shared `assignment.md` (directions + rubric, written once);
   - one alias-named file or folder per student (attempts with timestamps, entered score, late status, monitored flag, existing comments, submission text/attachments).

   It prints a summary: counts, missing/late/excused, and anything unreadable.
2. **The agent scores** into `scoring/<assignment>/results.md`. Each student row holds:
   - the score, plus a rubric rating per criterion when a rubric exists;
   - the feedback (D17 layout);
   - a late-forgiveness flag with a reason (§6.2), when it applies;
   - an optional teacher-only note.
3. **`cse post <results file>`** — one call:
   - re-reads entered scores and attempts for just those students, in one batched request;
   - **comment:** always posted, always containing the score. When SE filled in the rubric grid, the comment is just the score plus glows and grows; otherwise it adds the rubric breakdown (D22). It is skipped if an identical SE comment already exists on that attempt, so retries are safe. Aliases in the text become the student's real first name;
   - **score and rubric:** follow D9; see §6.4;
   - **late forgiveness:** applied where flagged;
   - **student resubmitted since pull:** that student is held, and everyone else posts;
   - the output is a per-student table: posted / score written / score kept (existing higher) / late adjusted / held + why.
4. Teacher-only notes never reach Canvas. Only the explicit feedback field is posted.

"Show me first" means stopping after step 2.

### 6.2 Late forgiveness for second drafts

If **attempt 1 was on time** and **genuine** (not blank, a few words, "lol" or the wrong file), SE zeroes the lateness on the current attempt with a late override. The agent judges "genuine" while scoring, records a one-line reason, and asks only when unsure. SE never adds lateness or touches penalty settings.

### 6.3 Differentiation (priority 3)

- **The teacher owns membership.** Students are tagged by hand in Canvas with Differentiation Tags. SE never creates, edits or reads membership for decisions. It only *assigns content to tags* by name.
- **Hub (default):** one shared assignment for everyone, plus tier support pages assigned to tags.
- **Bridge (when better suited):** tier assignments assigned to tags, plus a gradebook bridge that takes the highest tier score and never lowers.
- **Every assignment** also gets collapsible "Need a hint?" supports open to everyone. Tags remain for documented, student-specific differentiation (legal record).
- **[verify]** that the API can set "Assign To" by differentiation tag on assignments, New Quizzes **and pages**. Tags are believed to be stored as a special kind of group that assignment overrides can target. Page assignment uses Canvas's newer selective-release endpoints.

### 6.4 Canvas rubrics

- **Creating:** an assignment's rubric is created in Canvas and attached to the assignment when it is pushed, if the draft has a rubric.
- **Scoring:** `post` sends a rating (and an optional short comment) per criterion with each student's grade. Canvas's bulk grade-update endpoint is believed to accept rubric ratings together with grades and comments. **[verify]**
- **Conflict with D9:** when a rubric is marked "use for grading," filling in the grid overwrites the score field. Proposed rule: SE fills in the grid **only when it is also allowed to write the score**, i.e. the score is empty or SE's is higher. Otherwise SE posts the comment only, which still carries the score and rubric breakdown, and leaves the existing grid and score alone. **[verify]** which value wins when a grid and an explicit score are sent together.

### 6.5 Feedback revision

The teacher says something like "the ELA7 essay feedback is too harsh, revise it" or "redo Pikachu's comment":

1. `cse pull-work` (scoped to that assignment) already brings back existing comments, so the agent sees SE's earlier feedback alongside the work.
2. The agent writes revised feedback into a results file, marked as a **revision**.
3. `cse post` with revisions replaces SE's earlier comment on that attempt. Canvas allows editing or deleting submission comments by their author **[verify]**. If an in-place edit isn't possible, SE deletes the old SE comment and posts the new one.
4. Scores are **not** changed by a revision unless the teacher asks. If they are, D9 still applies.

## 7. Aliases (light hygiene)

- **What:** file names, folder names, roster rows and Canvas metadata (names, Canvas/SIS/login IDs) in pulled data use a Pokémon alias. **Student writing, images and attachments are passed through untouched** (D2).
- **Table:** `.cse/students.csv` holds alias, Canvas ID, SIS ID, login, real name and monitored status.
  - It is filled automatically on first pull, from a curated Pokémon list with no unflattering names.
  - Each student keeps the same alias across all courses, forever.
  - The teacher views and edits it in the WebUI (`cse students`), e.g. to change an alias or toggle monitored.
  - The agent never reads it: Claude Code enforces this with a deny rule; Codex relies on the instruction.
- **Commands** take aliases, and `cse` resolves them to Canvas IDs.
- **Feedback:** any alias in outgoing text becomes the student's real first name, so a Pokémon name never reaches a student.
- **`students.md`:** the teacher writes notes with real names. `pull-work` attaches each student's notes to their alias-named file, so the agent never has to map names itself.
- **On/off:** one line in `settings/courses.md`.

## 8. Lower priorities (in order)

Classic Quizzes → creating new rubrics standalone (beyond §6.4) → curves → everything else. Writing timelines are built on demand in a dedicated session. Monitored students carry a flag only; there is no standing collection.

## 9. Don't build

- Identity vault, transfer keys, encrypted alias stores, scrubbing of student writing, output gates, PII classifiers
- Approval tokens, mandatory preview → apply pairs, `teacher_confirmed` flags, digest chains
- Databases, mirrors with readiness/freshness gates, refresh → retry dances
- Operation ledgers, send journals, leases, locks, device ownership, handoff
- Backup snapshots, restore drills, history ledgers
- Re-pushing old drafts over live content; Markdown ↔ HTML round-tripping of live items
- Post-push "verify live" calls when Canvas already returned the result
- Course-wide invalidation because something unrelated changed
- Startup work, background workers, warmup
- Instruction bloat: `AGENTS.md` stays short, and guides load only when needed

## 10. How we'll know it's fast

- **Score-and-post an assignment:** 2 CLI calls (pull, post) and 0 extra teacher turns.
- **Push a new assignment:** 1 call. **Edit a live one:** 2 calls (pull-item, push).
- **Warm read of pulled data:** 0 Canvas calls.
- Wall-clock time, Canvas request count and output size for each of the above.

## 11. Open questions

- **Q9 — Language:** settle in the WorkLaptop session after the `.exe`/PowerShell test. Go is the leading candidate; PowerShell is the fallback if unsigned `.exe` files are blocked.
- **Next planning step:** a short build order (first 3–4 slices, each with "done when…" checks), so the WorkLaptop session can start right after `WorkLaptopTestSuite/`.

Machine-specific checks live in [WorkLaptopTestSuite/](WorkLaptopTestSuite/README.md). The Canvas **[verify]** items above can be checked from any machine, against a sandbox course.

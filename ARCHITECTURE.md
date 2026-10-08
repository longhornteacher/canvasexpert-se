# CanvasSidekick: architecture

**Updated:** 2026-10-08 · **Status:** 0.1b implementation in progress; portable executable and local checks available, district Canvas verification pending. The detailed implementation handoff is [EXECUTION-BRIEF-0.1b.md](EXECUTION-BRIEF-0.1b.md), with actual progress in [docs/0.1b-evidence.md](docs/0.1b-evidence.md). It distinguishes teacher decisions, implementation defaults, and required evidence. Items marked **[verify]** need a real Canvas or school-PC check before dependent behavior is enabled. The broader roadmap below is not all part of 0.1b; see §11.

## 1. What this is

A personal tool, shared with friends. **It is not a product and not SaaS.** A teacher points an agent at a folder and says "let's work." The agent then authors Canvas content, pulls student work, scores it, gives feedback and handles differentiation through the teacher's own Canvas Personal Access Token (PAT).

**Success means speed and accuracy in the moment.** Every design choice is weighed against those two. Defenses against rare failures are worth having only when they cost almost nothing. This is K-12: an occasional "oops" that the teacher fixes in Canvas is acceptable. Slow or fussy tooling is not.

SE is a standalone project. It shares no code, layout, credentials, tool names or runtime with any other tool. Scoring rules live in [SCORING-RULES.md](SCORING-RULES.md).

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
| D9 | **Grade retention:** the feedback always contains SE's honest assessment. Raise the entered score only when it is empty or SE's assessment is higher; preserve a higher or equal entered score before late penalty. | Rubric assessment is independent and still updates accurately (§6.4). Canvas does penalty math. The rule prevents intentional lowering; a read/write race is a documented practical limit. |
| D10 | The teacher's instruction is the authorization. "Score and post" means do it; "draft" means don't post; "show me first" means stop for review. | No mandatory preview/apply pairs and no confirmation tokens. |
| D11 | Configuration is files in the workspace. **The WebUI covers the PAT and the student table** (alias, monitored status). | The teacher edits student info in a page the agent never reads. |
| D12 | SE is self-contained and can coexist with other Canvas tools on the same machine. | Its own credential name, ephemeral port, command name and folders. |
| D13 | Forge contracts are a starting point, not scripture. | Lean SE authoring formats (§5). |
| D14 | **0.1b implementation default: Go**, shipped as a portable Windows executable. | The WorkLaptop probe supports this direction; the actual Go executable still needs an execution/platform check. |
| D15 | **No installs and no admin rights**, Windows 11 only. **WorkLaptop is the baseline**, since colleagues have the same model and permissions. | Everything ships inside the workspace folder. It must run from a user-writable folder on WorkLaptop. |
| D16 | **SE never applies late penalties.** It only adjusts how late a submission counts. | §6.2. |
| D17 | **Feedback layout:** score on the first line, rubric scores under it, then conversational glows and grows. | One house format in `guides/score.md`, which `settings/grading.md` can override. |
| D18 | **Differentiation:** the teacher tags students by hand in Canvas. The agent assigns tiered content to those tags. Hub is the default and Bridge is used when better suited. Optional built-in hints go in every assignment. | §6.3. Tags remain the legal record of who received what. |
| D19 | **Look:** clean and easy for 7th graders. Styling exists only for attention and clarity. | A small, fixed visual vocabulary in the renderer. |
| D20 | **Canvas Live is the source of truth** once something is pushed. Teacher edits made directly in Canvas always win. | Drafts exist only to create. Edits start from a fresh pull of the live item (§5.2). |
| D21 | **Use Canvas's official rubric tools**: rubrics attached to assignments and filled in during scoring. | §6.4. |
| D22 | **Feedback:** overall assessment, retained-grade explanation when applicable, compact per-response criterion scores, then conversational glows and grows. | Full criterion definitions stay in the Canvas rubric. A line such as "SCR 2 Conventions: 7/10" is wanted even when the grid is filled. |
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
| D33 | **The agent infers how the rubric applies**, including repeated response units and appropriate weighting, without a special teacher warning. | Use one defensible interpretation consistently across the assignment. The CLI validates the arithmetic; no hardcoded three-SCR or equal-weight rule. |
| D34 | **Compare with the strongest preceding attempt**, judged academically from actual work and rubric evidence. | Do not choose the baseline from a teacher-adjusted grade or automatically use the immediately previous attempt. |
| D35 | **The rubric is honest even when the teacher's final grade is higher.** | Fill or update the grid for lower, equal, and higher assessments. Never inflate rubric points to match the retained grade. |
| D36 | **Teacher finalization occurs outside SE:** post to Canvas Live, then the teacher looks and may leave it unchanged. | No pending-finalization state, approval signal, follow-up polling, or grade-release workflow. D23 still applies. |
| D37 | **New assignments stay unpublished until their requested required pieces succeed.** | Resume partial creation against known returned IDs, then apply the requested publication state. |

## 3. Environment constraints

- Windows 11, standard user, no admin. Nothing system-wide, no services, no reboots.
- Allowed: files in the user's own folders, the user's own Windows Credential Manager, and loopback HTTP.
- [WorkLaptop checks passed](https://github.com/longhornteacher/canvasexpert-se/blob/e49ffd9/WorkLaptopTestSuite/results/STAGCFKQ74-20261008-0640.md) for an unsigned .NET probe in user/OneDrive folders, Credential Manager access, loopback HTTP, and browser launch. PowerShell was FullLanguage. This supports a workspace `cse/` executable and synced cache. The original test suite was removed after implementation; its report remains in Git history.
- **[verify on WorkLaptop]** the actual Go-built executable and its native platform operations. The real cross-machine OneDrive browser-download/Explorer-unzip test remains open. A Canvas 401 proved connectivity only, not authenticated grading behavior.

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

No resident server, database or background worker. Each `cse` call ends when its operation finishes; the setup/student WebUI has a temporary command-owned listener. A call may perform bounded parallel Canvas requests. Internal operations return structured results, with compact text at the CLI boundary, so an adapter can be added later (§4.3). The executable has no LLM API: the host agent supplies interpretation and educational judgment.

### 4.1 Workspace folder

```text
<Workspace>/                     ← synced cloud folder; the agent is pointed here
  AGENTS.md                      ← the only always-loaded instructions; short
  CLAUDE.md                      ← "@AGENTS.md"
  .claude/settings.json          ← deny rule: the agent never reads .cse/
  .cse/                          ← SE's private state (synced across machines)
    students.csv                 ← stable local reference, alias, instance/Canvas/SIS IDs, name, monitored
  cse/                           ← portable Go executable (no end-user runtime install)
  guides/                        ← loaded only when the task needs it
  settings/                      ← teacher-editable text
    courses.md                   ← nickname → course (CS8, ELA7PAP, ELA7), module naming hint, alias on/off
    grading.md                   ← feedback voice, rubric habits, overrides
    students.md                  ← accommodations and notes, real names
  courses/<course-nickname>/
    drafts/                      ← new content not yet pushed
    pulled/                      ← disposable cache, including live items pulled for editing
    scoring/<assignment>/        ← job files, scores, feedback, returned IDs and outcomes
```

- The source repo (this folder) is separate from the workspace. It holds the code, guides and a workspace template, and builds the `cse` program that gets copied into a workspace.
- The CLI prints compact summaries to stdout. Large material goes to files, and the CLI prints their paths.
- Formats: Markdown with a YAML header for anything the agent writes; Canvas HTML with a YAML header for live items pulled for editing; original bytes for attachments.
- File contracts are versioned. Assignment folders include stable assignment identity. Scoring files use a stable local student reference independent of the editable alias. Ordinary returned IDs/outcomes in draft and scoring files support resumption; they are not a runtime journal or second gradebook. A fresh pull must not erase unsent scoring work.

### 4.2 Credentials

- The PAT and Canvas URL live in the Windows Credential Manager of **each machine**. Run `cse setup` once per machine.
- The PAT never appears in files, stdout, logs or the agent's context. An expired token produces "run `cse setup`".

### 4.3 Cowork and ChatGPT Work

Cowork runs commands in an isolated VM. From there it can't run Windows programs, read Credential Manager, or (probably) reach Canvas. **[verify]** ChatGPT Work. If wanted later, a `cse mcp` mode would expose the same functions as a few MCP tools on the Windows side. It is not in the first build.

## 5. Authoring (priority 1)

### 5.1 Create

- The agent writes a draft in `drafts/`: a Markdown body plus a YAML header (title, points, dates, submission types, module, published, rubric, tiers/tags).
- The renderer adds the fixed 7th-grade visual vocabulary: numbered steps, a "what to turn in" box, collapsible "Need a hint?" supports, and callouts.
- `cse push <draft>` first creates an unpublished assignment, records its returned ID, attaches requested rubric/module pieces, and then applies the requested publication state. Save each returned resource ID promptly. A failure leaves a resumable unpublished item rather than triggering recreation. On completion move the draft to `drafts/pushed/`, stamped with the link. A completed draft is a record and refuses another create, pointing to §5.2.
- **Ambiguous create failure:** exact title and recent time find candidates, not proof. Adopt only an unambiguous match supported by relevant intended fields. A missing lookup result does not prove failure. Do not blindly repeat an uncertain create; resolve against Canvas and ask only if still genuinely unsure (D8).
- No dry run, catalog scan or post-push verify. Canvas's response is the result.
- **New Quizzes** use the New Quizzes API with a compact Markdown question format. Item-level scores and feedback cannot be posted by API.

### 5.2 Edit (Canvas Live wins)

A new agent session knows nothing about earlier pushes, and doesn't need to:

1. `cse pull-item <course> <title or link>` writes the **live** item as a file: Canvas's settings in a YAML header and Canvas's actual HTML as the body. Any changes the teacher made directly in Canvas are in it.
2. The agent makes targeted edits to that file (HTML in, HTML out; no lossy Markdown round trip).
3. `cse push <pulled file>` sends targeted changes. If relevant live content/settings changed after the pull, save a fresh version and tell the agent to reapply its edit while preserving the unsent local edit. Use a tested conditional write if available; otherwise the live check reduces conflicts but cannot eliminate the read/write race.

The same rule applies to scoring: `pull-work` always reads the live assignment and rubric, so teacher changes in Canvas automatically take priority over anything in an old draft.

## 6. Scoring and feedback (priority 2)

### 6.1 Default flow

0. **"Scoring session"** (D25): `cse pull-work` with no assignment named finds every assignment with ungraded or resubmitted work across current courses, and pulls them all in one call. It prints a per-course, per-assignment summary. Naming a course or assignment narrows it.
1. **`cse pull-work <course> <assignment>`** — one call. It fetches the live assignment, the attached Canvas rubric (criteria and rating IDs) and all submissions **with attempt history**, existing comments and rubric assessments. Attachments download in parallel. It writes:
   - one shared `assignment.md` (directions + rubric, written once);
   - one alias-named file or folder per student (attempts with timestamps, entered score, late status, monitored flag, existing comments, submission text/attachments).

   It prints a summary: counts, missing/late/excused, and anything unreadable.
2. **The agent scores** into `scoring/<assignment>/results.md`. Each student row holds:
   - stable student/attempt references, component criterion scores and aggregate assessment, using a shared inferred interpretation (D33);
   - valid real rating IDs only where they accurately represent awarded points, with aggregate criterion points when appropriate;
   - the intended conversational feedback; deterministic rendering adds validated score and compact criterion lines (D17/D22);
   - a late-forgiveness flag with a reason (§6.2), when it applies;
   - an optional teacher-only note.
3. **`cse post <results file>`** — one call:
   - validates the whole file before writes, then re-reads relevant live entered scores, attempts, and assignment/rubric state; batch the prechecks where supported;
   - **comment:** contains the current assessment, any higher retained entered grade, compact per-response criterion scores, and conversational glows/grows. Save its returned ID and exact posted text. Retry resolves known IDs or unambiguous full-text matches on the intended attempt; uncertain completion is not permission to duplicate. Intentional student addressing resolves locally to the real first name without rewriting quoted student work;
   - **score and rubric:** follow D9; see §6.4;
   - **late forgiveness:** applied where flagged;
   - **student resubmitted since pull:** that student is held, and everyone else posts;
   - the output is a per-student table with distinct comment, rubric, grade, and late outcomes. A partial row resumes unresolved actions; it is not reported as fully posted.
4. Teacher-only notes never reach Canvas. Post only the explicit student-visible feedback and the deterministic validated score/criterion header, never arbitrary result metadata.

Compare revision quality with the strongest preceding attempt (D34). A completed lower/equal assessment must not recur endlessly in discovery just because the entered grade stayed the same. **[verify]** the live indicators needed to distinguish handled work from new or incomplete attempts. Ordinary completed scoring documents may supply references checked against Canvas; no permanent ledger.

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

- **Creating:** attach the requested rubric with automatic grade calculation disabled for new SE assignments. This is the implementation default supporting the teacher's independent honest rubric and final-grade decisions.
- **Scoring:** always fill the rubric accurately, including when its assessment is lower than the retained entered grade. The old comment-only fallback for lower/equal scores is superseded by D35.
- **Repeated components:** the agent infers the rubric application and weights. For a rubric repeated per response, the default is weighted criterion aggregation into the existing grid, normalized onto the live assignment total, plus compact per-response criterion scores in feedback. Do not rewrite one live rubric into three copies. See the execution brief for formulas, mapping, and rating-ID rules.
- **Posting route:** begin with bounded per-student updates inside one `post` command. A bulk route is optional after proving equivalent semantics. The documented bulk endpoint returns asynchronous progress; wait for completion before reporting success.
- **Existing coupled rubric:** do not silently toggle the teacher's live automatic-grading setting. **[verify]** whether a combined honest-rubric/explicit-grade update reliably preserves the desired entered grade. Re-sending an unchanged retained grade is allowed only as necessary technical preservation in this tested path. If unsafe, hold that assignment and ask once whether its rubric may be decoupled. Never temporarily lower the grade and then repair it, or claim success after skipping the rubric.
- **[verify]** reliable pre-penalty entered-score retrieval, fractional aggregate criterion support, and lower/equal/higher/empty/late cases in the actual district configuration. A student's hypothetical `student_entered_score` is not the teacher's entered grade.

### 6.5 Feedback revision

The teacher says something like "the ELA7 essay feedback is too harsh, revise it" or "redo Pikachu's comment":

1. `cse pull-work` (scoped to that assignment) already brings back existing comments, so the agent sees SE's earlier feedback alongside the work.
2. The agent writes a distinct **revise-feedback** operation targeting an exact known or explicitly selected comment ID.
3. `cse post` edits that comment. An edit endpoint is documented; **[verify]** actual access and behavior. Author identity alone does not establish SE ownership because the PAT acts as the teacher. If editing fails, preserve the old comment and establish the reason before considering a specific replacement strategy; deletion/recreation is not an automatic fallback.
4. Scores are **not** changed by a revision unless the teacher asks. If they are, D9 still applies.

## 7. Aliases (light hygiene)

- **What:** file names, folder names, roster rows and Canvas metadata (names, Canvas/SIS/login IDs) in pulled data use a Pokémon alias. **Student writing, images and attachments are passed through untouched** (D2).
- **Table:** `.cse/students.csv` holds a stable local student reference, alias, Canvas instance/ID, SIS ID, login, real name and monitored status.
  - It is filled automatically on first pull, from a curated Pokémon list with no unflattering names.
  - Each student keeps the same alias across all courses, forever.
  - The teacher views and edits it in the WebUI (`cse students`), e.g. to change an alias or toggle monitored.
  - The agent never reads it: Claude Code enforces this with a deny rule; Codex relies on the instruction.
- **Commands** may take aliases for convenient lookup; persisted work uses the stable reference so an alias rename cannot retarget a result.
- **Feedback:** intentional alias/name placeholders become the student's real first name. Do not broadly replace substrings or alter quoted student work.
- **`students.md`:** the teacher writes notes with real names. `pull-work` attaches them to the correct student's file using an explicit reference or unambiguous name/course association. Report ambiguous matches instead of attaching another student's notes.
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

## 11. 0.1b scope and execution

- **Q9 — Language:** adopt Go as the 0.1b implementation default, subject to the actual executable/platform test. The existing probe provides no reason to take the PowerShell fallback.
- **Beta scope:** portable setup/student editor, assignment create/edit with rubric/module placement, text submission/history retrieval, intelligent rubric scoring, grade retention, late forgiveness, targeted feedback revision, course-scoped discovery, and recovery. New Quizzes, differentiation, standalone page/rubric authoring, and other lower priorities remain outside this beta.
- **Build order:** foundation/contracts/Canvas evidence; authoring/editing; scoring/revision; discovery/clean-session verification/packaging. Detailed ownership and done conditions are in [EXECUTION-BRIEF-0.1b.md](EXECUTION-BRIEF-0.1b.md).
- **Execution team:** GPT-6.1 Sol orchestrates GPT-6.1 Sol and GPT-6 Luna subagents. Sol owns high-risk semantics; Luna receives bounded work against frozen interfaces. The orchestrator owns live experiments and completion claims.
- **Later teacher test:** a three-SCR text-entry assignment with a 100-point per-SCR rubric must be handled intelligently without a special warning. It has not been requested for creation yet. Synthetic/disposable-course engineering checks and that future teacher acceptance are separate.

Current machine-specific and Canvas evidence lives in [docs/0.1b-evidence.md](docs/0.1b-evidence.md). The original WorkLaptop test suite remains in Git history at `e49ffd9`. The Canvas **[verify]** items above can be checked from any machine, against a sandbox course.

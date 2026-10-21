# Playbook and Engine conflicts: execution brief

Prepared 2026-10-21 against Dev branch `dev`, revision `236a20f` (targeted scoring sessions). Implemented the same day in batches; see "Implementation status" below. It does not authorize Canvas writes, release, or UserInstall/UserSync adoption.

## Implementation status (updated 2026-10-21, end of day)

Everything below is committed on `dev` and passes `go vet ./...` and `go test ./... -count=1`. Tests are local fakes only; nothing was run against Canvas Live, and no ZIP was built or adopted into any UserInstall. The git-install migration that replaced the ZIP plan is in [GIT-INSTALL-MIGRATION.md](GIT-INSTALL-MIGRATION.md).

| Commit | Content |
|---|---|
| `34754a7` | **Batch 1.** A1 bare playbook names resolve through `read_workspace_file` (`playbooks/` fallback, root file wins). A2 results and group-results may omit `assignment:`; `FillResultsAssignment` (`internal/workflow/results_assignment.go`) copies it from the packet on validate, review and post, and names the differing field when a present block drifts; evidence error names the missing `discussion_topic_id`. C4 New Quiz and graded-discussion pulls use `--session` with a title or ID. C5 results path `scoring/results-<YYYYMMDD>-<HHMM>.md`, pull report prints workspace-relative packet paths. C6 one indented line per criterion in feedback; criterion descriptions are short names. C8 "Canvas count differs" note counts pulled plus comment-posted students. C9 reuse an earlier `scoring_basis` (the two PAP 3705775 files confirmed the 40/30/30 vs six 5-point mismatch). A3 `formats.md` lines 11, 62/65, 145. |
| `8cf0d61` | **Batch 2.** B11 images in assignment directions are downloaded beside the packet as `direction_images` (`internal/workflow/direction_images.go`, via `/api/v1/files/<id>` then an unauthenticated signed download; fake-Canvas test only, no live run). C10 genuine-submission wording, New Quiz pointer, checkpointed discussions offer `comment-only`. D1 `quiz-points` writes after submissions and warns (playbook, comment, capabilities, docs agree). D2/D3/D4 authoring and native-content corrections (valid Page starter, `push_quiz`, read-back exception, rerun wording). An unchanged pulled `only_visible_to_overrides` no longer blocks an audience update. E1 setup address field accepts a scheme-less address. A3 leftovers and stale tool descriptions. |
| `3f76d30` | **Batch 3.** E3 `shipped/` readable (never writable) through MCP. E2 handled as docs only: `playbooks update` already treats `AGENTS.md`/`CLAUDE.md` as seeds, but extracting a ZIP replaces them first; `AGENTS.md` is also the first-run marker, so it stays in the ZIP. E4/E6 README, setup and install wording and the real Settings headings. E5 parent-report context and playbook wording (sign-off, defaults, window, skip wins when `skip_eligible`). |
| `415d1ef` | **Small items.** C7 New Quiz: `score` = `full`, layout preference applies to criterion lines only. `correction_days` setting (default 14, 1 to 60) in `context/parent-updates.md` drives the report window and each correction deadline. Quiz same-title error no longer asks the agent to edit a `.cse.json` sidecar. Graded-discussion results example (`engine-docs/examples/synthetic-graded-discussion-results.md`). Low text pass (E7). |

**Not done, deliberately or by finding:** the existing synthetic examples still include their `assignment:` block (tests validate them standalone; a comment says to omit it normally); `field-limits.md` had no stale text left to fix; the E7 starter-calendar gaps and the `install.md` "latest release" line were left because the git install rewrites that route; D4 quiz resume option was not built (error and playbook reworded); A2 "later" fills (`late_timing`, discussion `digest`, group refs) remain a separate decision.

**Correction to this brief's UserInstall section below:** the "9 teacher-owned" files were the eight `context/` files plus `Teacher_Override.md`, not edited playbooks. A seed-playbook fix reaches an install as an `updated` file.

**Behavior changes worth knowing before the next scoring session:** results files no longer need an `assignment:` block (omit it); `pull_work` for a New Quiz or graded discussion should use `session: true`; packet paths in the pull report are workspace-relative; a New Quiz comment has indented per-criterion lines; assignment reference images appear in the packet as `direction_images` and are read with `read_workspace_asset`.

## Resume here

- **State:** the work in this brief is implemented (table above). Remaining optional work is only what "Not done" lists. The developer's own UserInstall still runs the older Engine build (`236a20f+dirty`); none of the changes above reach it until the Engine is rebuilt and adopted (see the git-install handoff, Step 4).
- **Developer decisions (2026-10-21):**
  - Squash conflicts between Engine and playbooks so agents stop working around them.
  - Agreed: one consistent scoring basis per assignment (C9), the Engine fills the assignment snapshot from the packet (A2), and assignment images are downloaded with the pull so the agent can read the reference (B11).
  - Rejected: a helper page for entering New Quiz item points. The teacher uses SpeedGrader. Only the jumbled comment formatting needs fixing (C6).
- **Decided (D1):** `quiz-points` writes after students have submitted and warns that Canvas may not regrade existing attempts.
- **Checks before starting:** read root [AGENTS.md](../../AGENTS.md), [ARCHITECTURE.md](../../docs/ARCHITECTURE.md), [SCORING-RULES.md](../../docs/SCORING-RULES.md), the seed [score.md](../../workspace-template/playbooks/score.md), and [TARGETED-SCORING-SESSION.md](TARGETED-SCORING-SESSION.md). Run `git status`: [SCORING-SESSION-HAIKU-LOG-2026-10-21.md](SCORING-SESSION-HAIKU-LOG-2026-10-21.md) was untracked when this was written.
- **Line numbers** are from revision `236a20f` and shift as files change. Re-grep before editing.

## Where the evidence came from

- **Two Haiku scoring runs on 2026-10-21**, both scoring all three courses (ELA7 120638, PAP ELA7 120669, CS8 121046) and posting directly. See [the run log](SCORING-SESSION-HAIKU-LOG-2026-10-21.md).
  - **The first run used the old UserInstall Engine (built 2026-10-19), not the Dev build.** `.claude.json` points `canvasexpert-se` at `C:/Users/adamb/CanvasExpert-SE/engine/cse.exe`. That run is not evidence about targeted sessions either way.
  - **The second run used the Dev build** `236a20f`, built 2026-10-21T17:51Z.
- **I checked the second run's results files** for PAP 3705186, 3705775 and 3705779. They are in the assignment folder roots, not `scoring/`.
  - The First Impressions graded-discussion grade (86) and comment succeeded.
  - Its late status came back `preserved`, not written: Canvas already had 72,264 s.
  - Both New Quiz comment-only rows came back `succeeded`.
- **Four read-only subagent audits** covered every seed playbook, context file, the root seed files, and `engine-docs/`, each claim checked against the Go source.
  - ✔ means I re-checked it directly in code.
  - Everything else is audit-reported with file:line citations and confirmed by the auditor's code reading, but not re-checked by me. Re-check each one before changing it.

## UserInstall state (this matters for how to fix)

- **UserInstall `C:\Users\adamb\CanvasExpert-SE` now runs Dev `236a20f+dirty`, development build.** `+dirty` is only the untracked log. It is local mode with no `.cse/rules-location.json`.
  - The previous Engine is kept as `engine\cse.exe.1.1.0-beta.1-20261019`.
  - The files the extract replaced are backed up in `engine\pre-update-20261021\`. AGENTS.md changed only by the shipped Family Contact routing line.
- **`playbooks update` result:** 14 files `current` and **9 `teacher-owned`**. **Correction (2026-10-21):** the 9 are the eight `context/` files plus `Teacher_Override.md`, which hold teacher data and are never replaced. They are not edited playbooks. On this install every playbook matches the seed, so a seed-playbook fix reaches it as an `updated` file. A playbook the teacher did edit would be `kept`; none is.
  - An Engine fix still reaches every install regardless (A1, A2).
- **Engine processes started before the swap still run the old binary.** New Claude Code sessions start the new one. Claude Desktop's Cowork extension needs an app restart.

## Constraints (from AGENTS.md)

- **Judgment stays in playbooks, mechanics in the Engine.** A playbook that describes Engine mechanics should be shortened to what the agent needs to do. It should not re-describe the code.
- **Fix only what supports a real teacher workflow.** Most items here were hit in real runs or misdirect the agent on a delivered feature. Items marked *low* are cheap text fixes. Do them only while already editing that file, and don't build anything for them.
- **When a capability, limit, or verification status changes,** update `internal/localui/capabilities.go` and [docs/engine-capabilities.md](../../docs/engine-capabilities.md) together, with the review date. Keep local tests, read-only observations, and Canvas Live evidence distinct.
- **Do not overwrite teacher-modified installed playbooks.**

## Suggested order

- **Batch 1, scoring friction actually hit:** A1, A2, C4, C5, C6, C9. Then A3 where it touches scoring (formats.md lines 11, 62, 145).
- **Batch 2, features misdescribed:** B11 (images), C8, C10, D1 (after the developer decides), D2, D3, E1.
- **Batch 3, setup and parent reports:** E2–E6.
- **Low items:** whenever the file is open anyway.

Run `go test ./... -count=1` and `go vet ./...` with `.tools/go1.27.1/go/bin/go.exe` after Engine changes, and `git diff --check`. Packaging is `scripts/build.ps1`. Adopting the result into UserInstall is a separate step done only on request: extract the ZIP over UserInstall, back up the bootstrap files first, then run `playbooks update`.

---

## A. Cross-cutting

### A1. Bare playbook names do not resolve through `read_workspace_file` ✔

- **Engine:** [workspace_files.go:185-203](../../internal/mcpserver/workspace_files.go). A path without a directory is read as a root `.md`/`.txt` file. `safePath` (`:118-131`) has no `playbooks/` fallback.
- **Playbooks:** nearly every seed playbook refers to others by bare name.
  - `score.md:3` (`group-work.md`, three times).
  - Workspace `AGENTS.md:9`: `authoring.md`, `edit.md`, `quizzes.md`, `native-content.md`, `score.md`, `group-work.md`, `family-contact*.md`, `parent-updates.md`.
  - `authoring.md:13-49`, `edit.md:19-27`, `native-content.md:3,37-38`, `quizzes.md:9-44`, `group-work.md:29,49,59`, `family-contact.md:3`, `family-contact-book-clubs.md:3`, `install.md:3,95,150`, `setup.md:3,10`.
  - `context/grading.md:9` (`students.md`) and `context/teaching-profile.md:23` (`grading.md`).
- **Ambiguity:** `parent-updates.md` exists in both `context/` and `playbooks/`. `family-contact.md:5` names the context one two lines after the playbook one.
- **Observed:** the first Haiku run's read of `group-work.md` failed.
- **Fix (Engine first):**
  - In `readablePath`, when a bare `name.md` does not exist at the root, try `playbooks/name.md`. This reaches teacher-owned installed copies.
  - Keep `parent-updates.md` ambiguous references out of this: a bare name resolves to the playbook, and the context file is always named with `context/`.
  - Then change the seed text to full paths where it is cheap, especially the ambiguous `parent-updates.md`.
  - Add one test for the fallback.

### A2. The agent hand-copies the assignment snapshot into results files

- **Playbooks:**
  - `score.md:24` ("start from `synthetic-results-two.md`") and `:39` ("replacing synthetic identity and evidence") never say where the `assignment:` block comes from.
  - `group-work.md:59,71` point to `examples/synthetic-group-results.md`.
  - No file in `engine-docs/examples/` contains `discussion_topic_id`, `digest`, or `evidence`. There is no graded-discussion results example.
- **Engine:**
  - `ValidateResults` checks only the in-file snapshot ([scoring.go:302-324](../../internal/domain/scoring.go)).
  - `validateEvidence` ([scoring.go:596-601](../../internal/domain/scoring.go)) rejects evidence when the copied snapshot has no `DiscussionTopicID`, with the misleading message "evidence is only for graded discussions".
  - At post time, `SameGradingBasis` ([edit_plan.go:74-98](../../internal/workflow/edit_plan.go)) compares the copy with the packet, ignoring only title, published, dates, description, updated_at, and the rubric's use_for_grading. Any other drift, including `discussion_topic_id`, `new_quiz`, or `checkpoints`, fails the whole file: "results grading basis differs from source packet" ([post.go:367-372](../../internal/workflow/post.go)).
  - Group results do the same and abort every group ([group_post.go:344-351, 386-387](../../internal/workflow/group_post.go)).
  - A missing `new_quiz: true` would let a `grade` row pass local validation and fail only at post.
- **Observed:** the second Haiku run failed validation on PAP First Impressions (3705186) until the agent copied `discussion_topic_id` from the packet. The first run reported hand-copying the snapshot into every file.
- **Fix (Engine, agreed):**
  - When `assignment` is omitted, `validate`, `review`, and `post` fill it from the `packet` path. When it is present, compare it with the packet in `validate` and report the differing field.
  - Rewrite the evidence error to name the missing snapshot field.
  - Update the examples to omit `assignment`, add a synthetic graded-discussion results example, and update `formats.md:62`.
  - Then shorten `score.md` and `group-work.md` to "name the packet; the Engine supplies the assignment".
  - **Later, same idea, separate decision:** fill `late_timing` from `prepare_late_timing` output (`score.md:55,59`), the discussion `digest` (`formats.md:63`, required at `scoring.go:602-603`), and group `member_refs`/`artifact_key` (`group_post.go:430-440`) by `student_ref`.

### A3. `engine-docs/formats.md` contradicts the code and the playbooks

The playbooks are right in each case below; fix the Engine docs. `formats.md` is served to agents via `engine_reference`, so agents see these contradictions.

| Line | Says | Code |
|---|---|---|
| 11 | Calendar resolves the grading period "for … session discovery" | Only `parent_report.go:142` calls `CurrentPeriod`. Sessions use Canvas `in_closed_grading_period` (`targeted_session.go:25`, `pull_work.go:269-277`) |
| 51 | "Group scoring remains unsupported in this slice" | `group-results` / `PostGroupResults` delivered and CS8 live-checked |
| 54 | Refuses updating topics linked to a graded assignment | `discussion.go:408` holds only if live `assignment_id` changed; CS8 live rename recorded in `docs/group-work/README.md:96` |
| 62 | Snapshot field list | Missing `discussion_topic_id` and `new_quiz` (becomes moot after A2) |
| 73, 109 | Uncertain creates/writes are held | Since 92dfa29 unclear Page/module/item creates are adopted (`page.go:290`, `module_author.go:134,240`) |
| 91 | No live discussion write recorded | CS8 2026-10-17 writes recorded |
| 145 | Comment-only for `points` or `percent` | Also `letter_grade`, `gpa_scale` (`scoring.go:275-280, 434-436`) |

- **Also stale:**
  - The `field-limits.md:258-261` evidence columns: "no CE-SE discussion, module or group-assignment write observed".
  - The MCP `pull_work` description ([server.go:195](../../internal/mcpserver/server.go)): "current-term scoring discovery and timing-only evidence".
  - The `prepare_audience` description (`operations.go:78`) mentions only assignments; Pages and quizzes are also supported (`audience.go:174-178`).
  - The `quiz_points.go:51-53` doc comment and `capabilities.go:101` (see D1).

---

## B. Assignment images (agreed)

### B11. Reference images in assignment directions are not downloaded

- **Observed in both runs:** CS8 Stardew Valley (3703860, 20 students) and Hotline Miami (3698355, 1 student) were held. The code students transcribe exists only as an image in `description_html`, and the agent could not compare for an exact match.
- **Fix (Engine):**
  - During `pull_work`, download images embedded in the assignment description (Canvas file links in `description_html`) into the packet folder, and list them in the packet.
  - The agent reads them with `read_workspace_asset`, which already exists.
  - Reuse the existing attachment download path and its 4-request concurrency.
- **Playbook:** `score.md` should say to read the directions' reference images. When an image cannot be read reliably enough for an exact-match check, the agent holds and says why.
- **Scope:** description images only. Don't touch external links or LTI embeds. The teacher may also put a text answer key in context; that needs no Engine work.

---

## C. Scoring

### C4. The New Quiz pull instruction drops `--session` ✔

- **Playbook:** [score.md:17](../../workspace-template/playbooks/score.md) says to pull New Quizzes explicitly by ID with no `--session` (`pull-work COURSE QUIZ_ID`), and to resolve a title through `pull_assignment`.
- **Engine:**
  - The handled-work and comment-only completion skips run only with `Session` ([pull_work.go:347-356](../../internal/workflow/pull_work.go)). Without it, the pull is reassessment by design (`:72-73`) and re-pulls students who already have comments.
  - `pull_work` already accepts an exact title (`canvas_reads.go:402-432`, MCP schema `server.go:76`).
  - `pull_assignment` also writes an edit baseline that a scoring task doesn't need (`server.go:288`).
- **Observed:** the first run pulled 23 already-commented students again (old Engine). This instruction would reproduce that on the new one.
- **Same issue in `group-work.md:69`** for graded discussions.
- **Fix (playbook):** use `pull_work` with `session: true` and the title or ID, unless the teacher asked for reassessment. Drop the `pull_assignment` detour. Note that with an explicit assignment, a closed-period assignment is still pulled (`pull_work.go:269-277`).

### C5. Results file location and the `packet:` path

- **Playbook:** `score.md:77` says "Save the results in the assignment's scoring area" and never defines it.
- **Engine:**
  - Results are found by content anywhere in UserInstall (`result_files.go:16-60`), so location doesn't break completion checks.
  - Pulls write `courses/<C>/assignments/<id>-<slug>/pulled/<ts>/packet.md` (`pull_work.go:403-404`).
  - `packet:` must be workspace-relative (`ConstrainPath`, `workspace/paths.go:90`), but the pull report prints absolute packet paths (`pull_work.go:531-535`, `cmd/cse/remote.go:557`).
- **Observed:** the second run saved `results-20261021.md` at the assignment folder root. That was probably to avoid an existing `scoring/results-20261021.md` from the morning session.
- **Fix:**
  - Playbook: name the path, `courses/<C>/assignments/<id>-<slug>/scoring/results-<YYYYMMDD>-<HHMM>.md`, with a time suffix so sessions don't collide.
  - Engine: print packet paths relative to the workspace in the pull report and MCP result.

### C6. Jumbled criterion lines in posted feedback ✔

- **Engine:** [render.go:84-94](../../internal/render/render.go) writes `- <component label>: <criterion description> X/Y; <criterion description> X/Y` on one line.
- **Why it jumbles:**
  - Canvas rubric criteria have short descriptions, so the line reads well ("Response 1: Evidence 3/4; Reasoning 4/6", see `render_test.go:31`).
  - With an agent-written `scoring_basis`, descriptions are whole sentences that repeat the label. Posted example from PAP 3705775:
    `- Item 1: A time you felt afraid: Item 1: answers the prompt about a time the student felt afraid, with expectation and outcome 5/5; Item 1: specific details from the experience 5/5`.
- **Developer note:** the teacher enters item points in SpeedGrader and reads these lines while doing so. This formatting is the only annoyance there.
- **Fix, both sides:**
  - Engine: when a component has more than one criterion, put each criterion on its own indented line (`  - Details: 5/5`). Keep single-criterion components on one line. Update `render_test.go` and any golden comment text.
  - Playbook (`score.md:22` scoring_basis guidance): criterion `description` is a short name of two to five words ("Answers the prompt", "Specific details"). The full meaning goes in the private rationale or `rubric_hint`, not the description. Don't repeat the item label in the criterion.
  - Check the duplicate-comment check (`post.go:270, 736`) still matches identical text after the format change. A changed format makes a rerun post a second comment only if a write was interrupted mid-run, which is acceptable.

### C7. `header: score` and the teacher's "Assessment first" layout on New Quizzes

- **Playbook:** `score.md:65` says `header: score` gives "only the overall line" on any body, and that with `header: none` the agent puts the score where the teacher wants it.
- **Engine:** on a New Quiz, `score` falls through to the criterion lines (`render.go:61-66`; `OmitAssessment = NewQuiz` at `post.go:259`, `review.go:438`). `formats.md:153` describes this correctly. `score.md:73` forbids a whole-quiz total from the written portion, but `header: none` posts the agent's text exactly as written (`render.go:59-60`), so nothing stops it.
- **Observed:** the first run reported that the teacher's "Assessment: X / Y first" preference doesn't fit comment-only results.
- **Fix (playbook):** on New Quizzes, `score` and `full` are the same. Apply a saved layout preference to the criterion lines only, and never write an "X / Y" total for the written portion.

### C8. A false "Canvas count differs from pulled count" note ✔

- **Engine:** [session_report.go:55](../../cmd/cse/session_report.go) compares `NeedsGradingCount` with `Students` (pulled) only. Comment-posted students (`CommentOnlyCompleted`) count in Canvas's needs-grading count, so the note fires even when the numbers add up. The second run's PAP rows were 14 = 1 + 13 and 21 = 1 + 20.
- **Fix (Engine):** compare against pulled + comment-only-completed. Check whether `Handled` belongs in the sum: already-scored work should not be in Canvas's count, but a resubmission edge may be. Add a case to `session_report_test.go`.

### C9. One scoring basis per assignment (agreed)

- **Observed:**
  - The first run said PAP Anticipation (3705775) was scored 40/30/30 "matching earlier PAP comments".
  - The second run scored the one new student on six 5-point criteria (10 per item, 30 total).
  - LA7 and PAP copies of the same Terror quizzes were scored with separately invented criteria.
  - **Confirm before writing the rule:** compare `courses/PAP LANG ARTS 7 - Beckham/assignments/3705775-the-terror-anticipation-writing/scoring/results-20261021.md` (09:40, morning session) with `.../results-20261021.md` (13:02, second run).
- **Fix (playbook, judgment):**
  - In `score.md`'s no-rubric section, before inventing a basis, look for earlier results files for this assignment, and for the same-titled assignment in the teacher's other sections. Reuse their `scoring_basis` and `interpretation` exactly.
  - Invent new criteria only when none exists. If the earlier basis seems wrong, ask the teacher rather than silently changing scales mid-class.
  - A saved per-assignment basis file is **not** part of this. It was listed out of scope in the targeted-session brief, and reuse from earlier results covers the need.

### C10. Smaller scoring conflicts

- **"Genuine" check:** `score.md:51` implies the Engine checks the first submission is genuine (not blank or accidental). `late_prepare.go:346-352` only checks the attempt exists in the packet. Reword so the agent knows that judgment is its own.
- **Missing playbook:** `group-work.md:73` says New Quizzes use "its own playbook". It's a section of `score.md`; point there.
- **Checkpointed discussions:** `group-work.md:71` says the Engine holds the grade, so the agent gives the teacher the written assessment instead. The Engine holds only `grade` rows (`discussion_work.go:246-249`). Offer `comment-only` so the feedback reaches the student, with the teacher entering checkpoint grades in SpeedGrader.
- **Correction window:** `TEACHER.md:47` and `Teacher_Override.md:5` treat it as a teacher-overridable norm (example: "10 school days after I post feedback"). Parent reports hard-code 14 calendar days after the due date (`parent_report.go:20, 515-519`). Decide whether the Engine reads a setting, or the parent playbook says the computed deadline is fixed. Don't build a setting without a teacher request.
- **Calendar wording:** `context/school-calendar.md:38` says the agent works out school-day lateness; the Engine counts it. *Low.*

---

## D. Authoring

### D1. `quiz-points` after submissions: playbook says it holds, the Engine writes ✔ (developer decision needed)

- **Playbook:** [quizzes.md:40](../../workspace-template/playbooks/quizzes.md): "Once any student has submitted, it holds without writing…"
- **Engine:**
  - [quiz_points.go:96-123](../../internal/workflow/quiz_points.go) detects submissions, sets a warning ("Canvas may not regrade their existing attempts…"), and PATCHes anyway.
  - `quiz_points_test.go:87-91` asserts "submitted quiz should change and warn". The change came in commit 92dfa29.
  - The `quiz_points.go:51-53` comment and `capabilities.go:101` still say it holds.
- **Fix:**
  - If the write is intended, update `quizzes.md:40` to say it writes and warns, and that the agent tells the teacher to check the gradebook and use the quiz editor's regrade. Also update the comment, `capabilities.go:101`, and `engine-capabilities.md`.
  - Otherwise restore the hold.

### D2. `authoring.md:25,27` says delivered group and graded-discussion scoring are unsupported

- "Group submission pulls and group scoring remain later work" and "Scoring a graded discussion is not supported yet."
- Both are delivered: `group_pull.go:107`, `group_post.go:200`, `discussion_work.go`, and CS8 live evidence in `capabilities.go:51-53,113`.
- **Fix:** point to the `playbooks/group-work.md` sections "Assess a shared club product" and "Assess graded discussion work".

### D3. Wrong tool names and invalid examples

- **Quiz push tool:** `authoring.md:49` names `push-quiz`/`push_document` for quizzes. `push` rejects quizzes (`cmd/cse/remote.go:87-89`); use `push_quiz` (`operations.go:128-136`).
- **Page starter:** the group Page starter in `native-content.md:22-34` has no `---` delimiters and no body. `workspace/document.go:40-43` requires `---` on line 1, and `domain/page.go:121-122` rejects an empty create body. Show valid delimiters and a minimal body.
- **Course list:** `authoring.md:13` says `cse courses` lists nicknames. It lists live Canvas (`canvas_reads.go:315-318`). Nicknames come from MCP `list_courses` (default configured source) or `context/courses.md`.

### D4. Missing steps and stale uncertainty handling

- **Rubric rerun:** `edit.md:21` says rerunning an uncertain rubric attachment resolves it. `rubric_attach.go:72-84` confirms only if the rubric now exists, and otherwise stays `uncertain` every rerun. The playbook should say what to do when the rerun is still uncertain: check Canvas with the teacher, then re-pull.
- **Hand-resolving uncertainty:** `native-content.md:43` and `authoring.md:37` imply the agent resolves uncertainty with live IDs. The Engine now adopts existing items or preserves teacher-changed ones (see A3 lines 73/109). Shorten to "rerun the same file; report what was adopted or preserved".
- **Read-back:** `native-content.md:47` ("Read saved HTML back from Canvas") and the verify section of `quizzes.md:48` contradict workspace `AGENTS.md`'s "no read-back". `formats.md:93` sides with the playbook for Pages. Decide one rule in the playbooks: an explicit exception for HTML conversion checks, or none.
- **Override visibility:** `authoring.md:45` omits the step in `edit.md:23` / `formats.md:166` to remove the pulled `only_visible_to_overrides`. Without it, `assignment_dates.go:77-78` rejects the document. Add the step, or have the Engine ignore an unchanged pulled value when `audience` is present.
- **Quiz resume:** `quizzes.md:36` says to resume a same-titled quiz but not how. The Engine error (`quiz_author.go:264`) tells the agent to write `{"assignment_id": N}` into `FILE.cse.json`, which conflicts with the rule that `.cse` files stay out of agent context. A resume or adopt option in `push_quiz` would remove this. Decide before building.
- **Hand-kept IDs:** `native-content.md:19-20` has the agent keep asset maps and Page IDs by hand. The upload receipts and the Engine-written `created` block already hold them. *Low.*

### D5. Group membership

- **Submitted-work check:** `group-work.md:23` has the agent check for submitted work before a move. The Engine already checks and holds (`group_memberships.go:354-361`) unless the ref is in `authorized_submitted_work_change_refs`. Shorten to "if the Engine holds a row for submitted work, ask the teacher, then add that ref".
- **`allocation` field:** `allocate-unassigned` requires `allocation` with `changes` empty (`domain/group_memberships.go:243-244`). The playbook omits this, and the error message doesn't name the field. Fix both.

### D6. Low

- **Pull scope:** `quizzes.md:44` insists on an assignment-ID pull, while `formats.md:143` allows a course pull. Align with C4.
- **Rubric title:** `edit.md:17` doesn't mention that an attached rubric needs a `title`. The error is clear.
- **Family Contact groups:** `family-contact-book-clubs.md:5` should say to list only clubs the group reads confirmed in each course. A selector matching zero or several groups fails preparation (`family_contact.go:338-340`).

---

## E. Setup, install, and parent reports

### E1. The Canvas address field blocks what the script tells teachers to type ✔

- **The script:** `install.md:67` says to type an address "like `yourschool.instructure.com`".
- **The field:** [setup.html:14](../../internal/localui/templates/setup.html) is `type="url" … required`, with no `novalidate`, so the browser blocks a scheme-less address before the server's normalization (`capabilities.go:90`) runs.
- **Fix (Engine):** `type="text"` with `inputmode="url"`, keeping server-side normalization. This is a first-run failure on the scripted path.

### E2. The ZIP overwrites teacher-editable AGENTS.md and CLAUDE.md

- **Package:** `scripts/package.ps1:62-63` ships `AGENTS.md`, `CLAUDE.md`, `README.md`, `TEACHING-ASSISTANT.md`, `cse.cmd`, `.claude/settings.json`, and `webui.cmd`.
- **Docs:** `workspace-template/README.md:31` says AGENTS.md and CLAUDE.md are "yours to change". `install.md:126,132` says extracting can't overwrite the teacher's work.
- **Effect:** in local mode, an edited AGENTS.md is replaced, and `playbooks.Update` (`shipped.go:92`) then reports it `current`, so the edit is lost silently. Sync mode is protected by `RestoreSyncBootstrap` (`rules_location.go:559`).
- **2026-10-21 adoption:** the developer's AGENTS.md differed from the new seed only by the shipped routing line, so nothing was lost.
- **Fix:** have `playbooks update` own AGENTS.md and CLAUDE.md like other seeds (fingerprint, `kept`), and drop them from the ZIP or ship them under `shipped/`. Failing that, correct the docs.

### E3. Reconciling edited playbooks needs `shipped/`, which MCP can't read

- **Instructions:** `install.md:128` and `engine_info` (`operations.go:171`) say to compare `shipped/<previous>/<path>`, the active file, and `shipped/<current>/<path>`.
- **Engine:** `readablePath` (`workspace_files.go:190-195`) allows only root files, `context/`, `playbooks/`, and `courses/`.
- **Effect:** updates through the Claude and ChatGPT apps cannot merge the teacher's edited playbooks. That matters now: 9 files are teacher-owned.
- **Fix (Engine):** allow read-only `shipped/` reads, or have `update-playbooks` return the three texts or diffs for `kept` rows.

### E4. The README misdescribes updates

`workspace-template/README.md:38` says new versions "never replace your existing rules". `shipped.go:95-99` replaces unedited playbooks automatically, and `install.md:126-128` merges `kept` ones. Describe it as it is.

### E5. Parent report context and playbook

- **Portal line:** `context/parent-updates.md:3` says a blank portal address shows the name without a link. The email omits the line entirely (`parent_drafts.go:563-565`); the printable report does show the name (`:552`).
- **Defaults:** `playbooks/parent-updates.md:5` says the email leaves out what's missing. Blank `grade_portal_name` and tutoring fields get defaults (`parent_drafts.go:41-48`).
- **Sign-off:** `sign_off` exists in the context file and is rendered (`parent_drafts.go:556,569`), but the playbook never asks for it, so first emails go out unsigned. Add it to the one-time questions.
- **Missing/0 wording:** `context/parent-updates.md:23` ("items older than the correction window") is stale. Missing items are only those due inside the report window (`parent_report.go:568-581`).
- **Window wording:** `context/parent-updates.md:26` is ambiguous. The window is as_of minus 14 days, raised to the period start (`parent_report.go:310-313`). Copy the playbook's wording (`playbooks/parent-updates.md:23`).
- **Skip rule:** `skip_eligible` is true when no work was due, even if the course score is missing (`parent_report.go:707-710`). The playbook says both "skip when eligible" (18) and "flag when score unavailable" (19); say which wins. *Low.*

### E6. Hub labels and locations named in docs don't exist

- **Settings headings:** "Settings → Teacher rules" and "Settings → Your playbooks and context" (`setup.md:7`, `README.md:19,38`). The real headings are "Where your teacher setup lives" (`settings.html:30`) and "Playbooks and context files" (`:67`).
- **Install path:** `README.md:13` and `setup.md:21` say it shows on hub Home. It shows on Settings → Engine, How, and Connect, not Home.

### E7. Low

- **`prepared` status:** `install.md:146` groups `prepared` with the "shown in a folder window" script. `prepared` only happens with `--no-ui` (`installation.go:163-166`).
- **Course sort:** `setup.md:11` says available courses are shown first. That sort is only on the WebUI page (`courses.go:75`); `list_courses source: canvas` is Canvas order.
- **Commands location:** `setup.md:21` says "commands" stay local. `context/commands.md` follows the rules authority (`commands.go:11-14`). Clarify whether it means CLI launchers.
- **`cse` vs `.\cse.cmd`:** `context/courses.md:10` and `README.md:19` use bare `cse`. `install.md:9` forbids PATH changes; use `.\cse.cmd`.
- **Antigravity:** `TEACHING-ASSISTANT.md:3` omits Antigravity, which install connects (`install.md:103`).
- **Latest release:** `install.md:132` points to GitHub "latest release", which skips pre-releases such as `1.1.0-beta.1`.
- **Starter calendar gaps:** the starter `context/school-calendar.md` grading periods leave Oct 9–12, Dec 19–Jan 5, and Mar 20–21 uncovered. Parent reports need exactly one period covering the run date (`school_calendar.go:136-160`). Check the teacher's real calendar has no such gaps; the starter is only a seed.

---

## Checked and clean

- **CLI and MCP names and flags:**
  - `pull-item`, `pull-page`, `page-history`, `upload-file`, `groups`, `pull-group-set`, and `pull-group-memberships`.
  - `pull-assignment-dates`, `prepare-audience --refresh`, `review FILE [COURSE]`, `push-quiz`, and `quiz-points N=PTS`.
- **QuizForge:** item types and rules, attempts rules, and Hub rules.
- **Rubrics:** attach-rubric rules, and that a New Quiz accepts only attach-rubric.
- **Group work:** the group CLI and MCP flags, discussion packet locations, the `shared_assessment_status` values, and the group-results individual mode.
- **Family Contact:** paths, tools, recipients, placeholders, asset limits, and button labels.
- **Context files:** field names in `courses.md`, `students.md`, `commands.md`, and `grading.md`, and `school-calendar.md` field names.
- **Setup and install:** the `engine_info` operations, the `list_courses` sources, the install flags and result fields, and the `drafts.json` schema.
- **Seed files:** `CLAUDE.md` and `teaching-profile.md`.
- **Session section of `score.md`:** apart from C4, C7, and C10, it matches the targeted-session code.

## Out of scope here

- **A New Quiz item-points entry page:** rejected by the developer 2026-10-21.
- **Engine session-narrowing flags, a "never score" list, and re-timing graded work:** decisions 2, 6, and 1 in [TARGETED-SCORING-SESSION.md](TARGETED-SCORING-SESSION.md).
- **Posting performance (serial posting, repeated re-reads):** listed in the targeted-session brief.
- **The targeted-session live-validation checklist:** separate work; keep it in its own brief.
  - The second Haiku run supplies evidence for two items: comment-only rows suppress re-pull with counts matching Canvas, and a graded discussion was pulled, scored, and posted.
  - Recording that evidence in `capabilities.go` / `engine-capabilities.md` was offered and not yet done.

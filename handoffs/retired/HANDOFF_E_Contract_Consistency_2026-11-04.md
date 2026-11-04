> **Retired 2026-11-04 — implemented in Dev** (commits 230a261, 5da7ee8, ad7f9d6). Agent-facing contracts, the capabilities page and evidence record, and stale current-sounding documents match what is built; no capability gained a Canvas Live label. Retired handoff records still contain earlier identifiers (history, not scrubbed). Historical instructions follow; do not execute them again.

# HANDOFF E: Make advertised contracts match what is built

Date: 2026-11-04. Slice E of the [unified architecture review](../Unified%20Architecture%20Review.md). Layer: **Engine tool metadata** (`internal/mcpserver`, `internal/localui/capabilities.go`, CLI help), **seed playbooks** and **docs**.

Developer decision 6: correct behavior-affecting contracts first. Preserve live consumers and unresolved evidence. Retire whole handoffs only when all parts are complete or explicitly carried forward. Decision 12: make concrete low-cost corrections; keep useful redacted test evidence; a numeric Canvas ID is not a credential.

Before starting: `git fetch`, bring `dev` up to date, read root `AGENTS.md` (especially the WebUI Capabilities rule), `docs/ARCHITECTURE.md`, `docs/SCORING-RULES.md`, [docs/group-work/README.md](../../docs/group-work/README.md), [docs/team/README.md](../../docs/team/README.md) and the [shared execution notes](../README.md#execution-notes-for-the-active-handoffs). Every item below was confirmed still true at `2818e36`. Find the current line with `rg` before editing.

**Why it matters.** Agents read MCP tool descriptions, schema hints, seed playbooks and the capabilities page to decide what to try. A description saying "New Quizzes take only a comment" makes an agent skip item scores that work. A capabilities page calling book clubs "Future" undersells live features. These are behavior bugs, not wording polish.

**Rule for every edit.** Do not promote evidence. A local test stays "local tests"; Canvas Live claims need a dated live record that already exists. When a capability's status changes, update `internal/localui/capabilities.go` **and** `docs/engine-capabilities.md` together, and bump `capabilitiesReviewed` (`capabilities.go:27`, currently `2026-11-03`).

Work in three commits, in this order.

---

## Pass 1: what agents read (highest value)

**MCP tool text (`internal/mcpserver/operations.go`)**

| # | Where | Now says | Should say |
|---|---|---|---|
| 1 | `post_results` description (~360) | "New Quiz results can post only an attempt-scoped comment and never a grade, rubric, or late write"; "comment-only New Quiz feedback" | New Quiz written items: the Engine writes each item's points and comment with `item_scores` (see `new_quiz_item_scores.go`, `post.go:290–317`). The teacher enters points in SpeedGrader only where a part cannot name one item or a student is held because the quiz was saved in SpeedGrader since the pull. Keep the true limits (no rubric or late write on New Quizzes, if still true; check `post.go`). |
| 2 | `from_saved` jsonschema (~41) | "Writes currently hold pending Canvas setter semantics"; covers only the "exact published quiz audience" | Writes are implemented (`quiz_saved_accommodations.go:125–145` sends and records "applied"); the tool description at ~228 already says so. Keep the true unpublished-quiz audience hold. |
| 3 | `push_quiz` description (~367) | "pending setter semantics hold affected quizzes" | Same correction as row 2. Keep the publication-stage behavior that the code actually has (read `savedQuizAccommodationHold` before writing). |

Search once more for other drift: `rg -n "SpeedGrader|comment-only|pending setter|setter semantics" internal/mcpserver`. `init_results` (`server.go:426`) is correct and stays.

**`docs/mcp-adapter.md`**
- Same drift: ~40 "Saved writes hold while setter/course-combination semantics are unresolved"; ~45 "no automatic accommodation POST is enabled"; ~44 lists what `post_results` posts but omits `item_scores` and `group-results`. Correct these.
- The tool table is **missing** `copy_assignment`, `list_rubrics`, `pull_rubric`, `pull_quiz`, `set_tags`; `init_results` appears only in prose (~53). Add rows. There are 58 registered tools (`scripts/check-mcp.mjs:70` asserts 58).

**Seed (`workspace-template`)**
- `workspace-template/README.md:40` says "you enter written-item points in SpeedGrader" unconditionally. Align it with `playbooks/score.md:73–75`, which is correct.
- The playbooks have no "group work unimplemented" claim and no retired `start_review` terms; leave them.

**`docs/ARCHITECTURE.md` behavior statements**

| Line | Problem | Fix |
|---|---|---|
| ~94 | "leave quiz/item grades to the teacher" | contradicts ~122; describe `item_scores` with its manual cases |
| ~122 | "saves `operation: comment-only` results for review/post" | retired review page; say results are saved for the check-in and posting |
| ~118 | group work "These are development contracts, not implemented capabilities." | Tier A/B are implemented and live-checked (group-work README); Tier C frozen |
| ~19 vs ~33/~39/~47 | ~19 says git owns playbooks/`TEACHER.md`/`AGENTS.md`/`CLAUDE.md` in a git install; ~33 says UserSync is "the authoritative home of teacher-customized instructions, playbooks…"; ~39 "Teacher's copy: UserInstall `playbooks/`"; ~47 "theirs to change… never overwrite teacher files" | Make ~33/~39/~47 state the git rule first and the ZIP-fallback rule as the fallback. Decision 11: **git installs trump ZIP**. |
| ~116 | two near-duplicate `push_quiz` sentences ("checks these requirements…" / "applies these settings…") | keep the one matching the code |
| ~7 | "The current product line is 1.0" | `VERSION` is 1.8.1; say 1.x or drop the number |
| ~108 | mojibake `authority�s` (bytes `EF BF BD`) | apostrophe |

Keep ~33 "These sessions are deliberately not durable across PCs." It is current policy (decisions 1 and 4).

**CLI help and MCP annotations**
- `cmd/cse/main.go:94–96` lists `prepare-late-timing FILE` three times with two descriptions. Keep one.
- `playbooks update` is implemented (~234) and the seed `install.md` tells teachers to use it, but help (~48, ~51) lists only `status` and `replace`. Add it.
- `externalTool` (`server.go:459–466`, sets OpenWorldHint) omits tools that call Canvas: `list_tags`, `assign_to`, `assign_modules_by_tag`, `audience_report`, `set_file_access`, `list_external_resources`, `copy_assignment`, and `engine_info` (doctor makes authenticated reads; `update-stage` downloads). Add them.
- `start_local_ui` is marked external though it serves loopback only. Unmark it unless it reaches Canvas; check first.
- `copy_module` (`server.go:276`) and `copy_assignment` (`:279`) have `destructive=false`. Leave that unless the copy can overwrite or delete existing course content (read `team_copy.go` / `assignment_copy.go`). Creating new objects is not destructive.
- The arity/known/help lists match apart from the above. Merging the four hand-kept lists is **deferred** (decision 12).

## Pass 2: capabilities page and its evidence record

Edit `internal/localui/capabilities.go` and `docs/engine-capabilities.md` together.

| Item | Now | Correction |
|---|---|---|
| ZIP contents | `capabilities.go:103`, `engine-capabilities.md:96` (and `docs/playbook-ownership-and-updates.md:174–176`) say "the ZIP no longer carries playbooks, TEACHER.md, Teacher_Override.md or context/" | False. `scripts/package-userinstall.ps1:50–54, 114–123` excludes only `context/` and `Teacher_Override.md`. Say the ZIP carries `playbooks/` and `TEACHER.md`; unzip a new ZIP into a **new** folder, never over a teacher's folder (matches seed `install.md:188–190`). Never describe extracting over teacher-edited files as safe. |
| Distribution line | `engine-capabilities.md:57` "Dev → ZIP → UserInstall" | Dev → userinstall branch (git) → UserInstall, ZIP as fallback (root `AGENTS.md`) |
| Tool counts | `capabilities.go:101` "37", `:93` "40-tool"; `engine-capabilities.md` 38/29/26/40/42/56; `mcp-adapter.md:57` 57; `family-contact-flow.md:61` 40; `team/README.md:50` 42 | Live pages: say 58, or drop the hand-written number and point at `check-mcp.mjs`. **Dated records** ("the 40-tool smoke passed on <date>") stay as history. |
| Module items | `capabilities.go:140` (Future) "Deletion, replacement… are not implemented" vs `:40` remove/replace implemented (Dev 2026-10-27, local tests) | Remove/replace is implemented, local evidence. Prerequisites and completion requirements stay Future if still unbuilt. |
| Book clubs | `:134` under "Future capabilities" (red, "not implemented yet") while its text says Tier A and B are implemented | Move Tier A/B to implemented with their CS8 live evidence (group-work README); keep Tier C under Future as "maybe nice for later". |
| Git install evidence | `capabilities.go:37` "local tests and a throwaway-repo trial only" while also citing the v1.1.0-beta.2 release and fresh clone; `engine-capabilities.md:84` puts release evidence in the Canvas Live column | Separate them: release/fresh-clone evidence is install evidence, not Canvas Live. |
| Current build | `engine-capabilities.md:37` describes Release 1.0.0 (2026-10-17) | Current version or a pointer to `VERSION`/CHANGELOG |
| Review date | `engine-capabilities.md` has none | add one matching `capabilitiesReviewed` |

**Identifiers on public pages**
- `capabilities.go:49` names "courses 102279 and 102278". Replace course IDs with nicknames (CS8 etc.) everywhere on the public page; course IDs are teacher-identifying.
- The many assignment/quiz/page/module IDs of **disposable test objects** (`:51, :54, :63, :80, :104, :106, :109, :124, :125`) are evidence pointers, not credentials (decision 12). Keep them.
- `docs/cs8-test-objects.md` holds object IDs, titles and **one Canvas user ID** (the teacher-owned test account). Move that user ID to the developer's private dev reference in UserSync and leave a pointer. Keep the object IDs. Also add the disposable quiz from the item-score live plan if it is still unlisted (the active quiz handoff names it).
- `cmd/cse-probe/main.go:27` hardcodes the school's Canvas origin (`allowedOrigin`), also in `cse-probe/file_upload_test.go:197` and `internal/localui/hub_setup_sync_test.go:63,66`.
  - For the probe: read the origin from a flag or environment variable. It is a dev tool and the developer supplies it.
  - For tests: use a synthetic origin such as `https://school.example`. Check that the tests do not depend on the real host.
- Do not scrub `handoffs/retired/` history (it is history, and rewriting it loses evidence). Mention in the commit message that retired records still contain them.

## Pass 3: documents that read as current but are not (separate, smaller commit)

- **`docs/playbook-ownership-and-updates.md`** (300 lines). Mostly ZIP-era: "Update direction" ~168, "First real update" ~241, "Remaining work" ~281, "Done looks like" ~288, with the git model only at ~296–300. Restructure it short:
  - current git rule first;
  - ZIP fallback rules second;
  - keep "Confirmed shipped defaults" (~24), "Teacher-owned setup" (~118) and the dated "Conflicts" decisions (~199);
  - move the ZIP-era design narrative under a clearly dated "History" heading, or delete it if git history suffices.

  School-specific bell-schedule and tutoring text in this file (~97–120, ~214–216, ~252) is handled by [HANDOFF G](HANDOFF_G_Parent_Email_Defaults_and_Deadline_Math_2026-11-04.md); coordinate rather than both editing it.
- **`docs/parent-update-flow.md`**: ~186 "Review/send details remain a proposal." and ~190–195 "Possible coding slices…" including "the class-wide Band post", while ~3 says Band is dropped. Mark the delivered parts current and remove the proposal list.
- **`docs/SCORING-RULES.md:3`**: "Teacher policy updated 2026-10-18", while the body has decisions from 2026-10-24/28/29. Update the date.
- **Active quiz handoff** (`handoffs/active/HANDOFF_Quiz_Accommodation_Writes_and_New_Quiz_Item_Scores_2026-10-28.md`): **keep active.** Q7 (the course/quiz accommodation combination) is unobserved; the §2.3 done-when (a live push of an Engine-built timed quiz) is unmet; a disposable quiz still needs deleting; and L9–L11 are optional. Correct its stale §0 bullets (~13 "a write requires all three values", ~16 "No accommodation POST is sent", ~18 "the teacher enters item points in SpeedGrader") with a dated note saying they were superseded, and keep the records.
- **Retirement candidates:** `docs/REVIEW-2026-10-19.md` and `docs/book-club-group-work-discovery.md` are linked as evidence from `capabilities.go` (~82, ~135), `engine-capabilities.md` (~21, ~69) and `group-work/README.md:151`. Leave them; they are live evidence links. Retiring them is out of scope.
- **The duplicate book-group source handoff** was removed on purpose on 2026-11-04, and the index link was updated then. Nothing to do.

## Done when

- `rg` finds none of the contradicting phrases above in `internal/mcpserver`, `workspace-template`, `docs/ARCHITECTURE.md`, `docs/mcp-adapter.md`, `capabilities.go` or `engine-capabilities.md`.
- The capabilities page renders (existing `internal/localui/capabilities_test.go` passes); the review date is bumped in both records.
- `go test ./...`, `go vet ./...`, the compiled MCP smoke (58 tools) and `git diff --check` pass.
- No capability gained a Canvas Live label it did not already have.

## Not in this slice

A full rewrite of ARCHITECTURE.md; merging CLI command lists; retiring evidence documents; new features.

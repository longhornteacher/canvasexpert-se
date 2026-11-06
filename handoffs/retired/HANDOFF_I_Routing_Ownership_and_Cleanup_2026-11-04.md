> **Retired 2026-11-04 — implemented in Dev** (commits 4de2366, d42bf03, f0d1e7f). WebUI course routing fails closed, git installs refuse MCP writes to git-owned files and the seed has ZIP-to-git steps, and unused review code was removed. Historical instructions follow; do not execute them again.

# HANDOFF I: Canvas-key routing fails closed; git ownership is enforced; dead review code goes

Date: 2026-11-04. Slice I of the [unified architecture review](../Unified%20Architecture%20Review.md). Three independent parts, so make three commits:
- **I1:** WebUI course routing. Layer: Engine (`internal/localui`).
- **I2:** git ownership of product files. Layer: Engine (`internal/mcpserver`, `internal/workspace`) and seed `install.md`.
- **I3:** unused review code. Layer: Engine (`internal/workflow`).

CLI help and MCP annotation drift moved to [HANDOFF E](HANDOFF_E_Contract_Consistency_2026-11-04.md) pass 1.

Before starting: `git fetch`, bring `dev` up to date, read root `AGENTS.md`, `docs/ARCHITECTURE.md`, [docs/team/README.md](../../docs/team/README.md) (for I1) and the [shared execution notes](../README.md#execution-notes-for-the-active-handoffs). Facts mapped at `2818e36`.

---

## I1: A course whose owner cannot be read never uses the wrong Canvas key

### Why

Team teaching routes each course to the right Canvas key: the owner's key, unless you teach there yourself. The WebUI's `clientForCourse` silently uses **your own** key when it cannot read `context/courses.md`. The CLI refuses in the same situation. Today only roster and tag reads reach it, so the consequence is a wrong-key read, not a wrong-target write. The project keeps cheap checks for wrong-target operations, and any later write through this helper would inherit the fallback.

### Current code

`internal/localui/hub_shared.go:75–99`:
```go
config, _, err := h.readConfig()
if err != nil {
    return c, nil   // operator's own client; error dropped
}
```

- **Callers:**
  - `fetchRoster` (`hub_shared.go:54`) and `fetchTags` (`students.go:418`);
  - both are called only from `syncRoster` (`students.go:538,559`), which runs from `studentsPage` and `refreshStudents`.

  Both callers read the config successfully just before, so the fallback fires only if the second read fails (UserSync drops out, or the file changes in between). The Hub path also never runs `CheckOwners`.
- **CLI behavior** (`cmd/cse/remote.go:130–139`): a failed `ReadTeacherDocument(courses.md)` → `fail(inputError(e))`. It then runs `ReadTeam` + `workspace.CheckOwners(config, team)` and fails on any error.
- **Routing rules** (`internal/workflow/team_route.go`):
  - `Owned` (`:58`) treats an owner missing from the team as an error, "never unowned";
  - `ForCourse` (`:97`) uses the owner's key unless `operatorTeaches` (`:146`) shows you actively teach there;
  - a transient Canvas error never switches the key (ARCHITECTURE ~110).

### What to build

1. `clientForCourse` returns the config read error instead of the operator client.
2. It runs the same `ReadTeam` + `CheckOwners` validation as the CLI before routing.
3. Callers already surface errors as a page message ("Could not fetch the roster for …"). Make sure the message is plain and says nothing was changed.

Prefer calling the existing workflow routing (`ForCourse`) over duplicating its rules if the Hub can reach it cheaply. Do not restructure the Hub.

### Done when

- A test where `courses.md` becomes unreadable between the page's read and `clientForCourse`:
  - inject the read failure through the Hub's `readConfig` seam, or remove the UserSync folder;
  - assert an error and **no request made with the operator key**.
- A test where the owner is not on the team asserts an error.
- Existing Hub roster/tag tests pass.

---

## I2: In a git install, product files stay git's; ZIP-to-git conversion is written down

### Decision (developer, 2026-11-04)

**Git installs trump ZIP.** When a former ZIP install moves to git, the git versions of product files win. Teacher intent belongs in `Teacher_Override.md` and `context/`.

### Why

In a git install git owns `playbooks/*.md`, `AGENTS.md`, `CLAUDE.md`, `TEACHER.md`, `TEACHING-ASSISTANT.md` and `README.md`. One agent edit to any of them through the MCP file tool makes every later update hold with "shipped files have local edits". The teacher then has to sort it out. The Engine already refuses `replace-playbook` in a git install; the general file tool does not.

There is also no written ZIP-to-git conversion in the seed: the only procedure lives in a retired handoff, and it overwrites edited playbooks with only a backup copy.

### Current code

- **Install kind.** `workspace.InstallKind` (`install_kind.go:57–65`): no `engine/manifest.txt` → `""`; manifest plus `.git` → `"git"`; otherwise `"zip"`. `IsGitInstall` is at `:68`.
- **What git tracks** (`scripts/package-userinstall.ps1:42–49, 128–134`):
  - root guides `AGENTS|CLAUDE|README|TEACHER|TEACHING-ASSISTANT.md`, `cse.cmd`, `.claude/settings.json`, `playbooks/*.md`;
  - also VERSION, LICENSE, `webui.cmd`, `get-engine.cmd`, `.gitignore`, `.gitattributes`, `engine/manifest.txt`, the notices file and the icon.

  `scripts/userinstall.gitignore` ignores `/context/`, `/courses/`, `/Teacher_Override.md`, `/.cse/`, `/playbook-old/`, `/shipped/`, `/install-log.jsonl` and `/engine/*` (with exceptions). Teacher-created root guides are untracked.
- **Shared/local split.** `IsSharedRulePathIn` (`install_kind.go:70–90`) already excludes `playbooks/` and those root guides from UserSync routing in a git install.
- **MCP write entry points:**

  | Entry | Location | Git-owned check |
  |---|---|---|
  | `write_workspace_file` | `mcpserver/workspace_files.go:42–56` → `readablePath`/`safePath` → `writeWorkspaceBytes` | **none**; only rejects `courses/`, `shipped/`, dot-paths, `engine` |
  | `write_draft`, `write_asset` | `workspace_files.go:224`, `:58` | limited to `courses/`; fine |
  | `engine_info` `replace-playbook` | `operations.go:477` → `playbooks.Replace` (`updates.go:101–103`) | refuses "managed by git" |
  | `engine_info` `update-playbooks` | `operations.go:504` → `playbooks.Update` (`shipped.go:59–61`) | returns `managed_by: git`, no change |

- **Update with edits.** `gitupdate.Apply` (`gitupdate.go:155–171`) holds with "move the teacher's choices into Teacher_Override.md, then apply with archive-edits". With archive-edits it copies to `playbook-old/<ts>/` and checks out the shipped versions. Seed `install.md:177` tells the agent to do the move, and `:184` forbids editing shipped files in a git install.
- **ZIP→git conversion.**
  - No Engine code and no seed section.
  - The retired `handoffs/retired/GIT-INSTALL-MIGRATION.md:115–122`:
    1. back up root files and `playbooks\` to `engine\pre-git-migration-<date>\`;
    2. `git init -b userinstall`, add remote, fetch;
    3. `git checkout -f -B userinstall origin/userinstall`;
    4. `get-engine.cmd`.
  - The forced checkout overwrites edited playbooks; the backup is the only copy; nothing moves teacher choices into `Teacher_Override.md`.
  - In a sync-mode ZIP install, the active playbooks live in UserSync and are simply ignored after conversion.

### What to build

1. **Helper** `workspace.IsGitOwnedPath(root, rel) bool`: true only in a git install, for `playbooks/…` and the git-tracked root guides. Derive the list from the same names `IsSharedRulePathIn` already excludes; keep one list, not two.
2. **`write_workspace_file` refuses** a git-owned path in a git install. The error is plain and actionable, for example: "This file is part of CanvasExpert and is replaced by updates. Put the teacher's choice in Teacher_Override.md (or context/) instead." Reads stay allowed. ZIP installs are unchanged.
3. **Seed `install.md` gets a short "Moving a ZIP install to git" section** (agent-facing, plain):
   1. back up shipped files;
   2. convert;
   3. compare the backed-up playbooks/guides with the new git versions;
   4. move any teacher-specific choices into `Teacher_Override.md` or `context/`, with the teacher's yes;
   5. leave the backup in place.

   Git versions win (decision 11). For a sync-mode ZIP install, say the old UserSync playbooks are no longer read and are only a source for step 4. Keep it to the steps that exist; do not add Engine conversion code.
4. Update `docs/ARCHITECTURE.md` ~19 if it needs one clause about the refusal.

### Done when

- Tests in `internal/mcpserver`:
  - in a git-install fixture (manifest + `.git`), `write_workspace_file` to `playbooks/score.md` and `AGENTS.md` is refused, while `Teacher_Override.md`, `context/grading.md` and a teacher-created root guide are allowed;
  - in a ZIP fixture, the same playbook write is allowed (unchanged behavior).
- The `install.md` section exists and contradicts nothing in the "Updates" section.
- Full checks pass, and the compiled MCP smoke still passes. Its "guarded context" check may need a git-install case; extend it only if cheap.

---

## I3: Remove review code with no caller

### Current code

After the scoring/authoring review page was retired, most of `internal/workflow/review.go` and `review_author.go` is still reachable through `BuildReview`. That is used by `draft-pdf` (`cmd/cse/main.go:587`) and `validate --show` (`feedback_sample.go:22`, via `main.go:568`), so it is live. Family/parent review uses only `ReviewState`, `withDigest`/`digestOf` and the shared types. Group review cards in `group_post.go` (~1212–1372) are reached through `BuildReview`, and `holdStudentResult` is used by posting.

**Zero non-test callers:**
- `ValidateReviewInput` (`review.go:220`), used only by `review_input_test.go`, `audience_test.go` and `results_assignment_test.go`;
- `Service.BuildEditReview` (`review_edit.go:16`).

### What to build

1. Delete `BuildEditReview` (and `review_edit.go` if nothing else is in it).
2. For `ValidateReviewInput`, check what its tests actually protect:
   - if they assert input validation that `BuildReview`/`ReviewInputKind` or posting already performs, point the tests at that live path and delete the function;
   - if they are the only coverage of a still-relevant rule, keep the rule where it is used and delete the wrapper.
3. Run `go vet ./...` and `staticcheck` (if available) or `rg` for any other now-unreachable helper the deletion exposes. Delete only what is truly unreachable from `draft-pdf`, `validate --show`, family review or posting.

### Done when

Full checks pass, `draft-pdf` and `validate --show` tests pass unchanged, and no live behavior changed.

---

## Deferred (decision 12: no backlog item)

- Merging the four hand-kept CLI command lists.
- Sharing the `/users/self` lookups:
  - `doctor.go:189`;
  - `hub_context.go:179,349`;
  - `setup.go:94`;
  - `team.go:137,146`;
  - `inbox.go:397,502`.
- Merging the group timing paths in `group_post.go`.
- A school-account nudge for UserSync discovery.

Pick one up only if it causes concrete teacher friction or a bug.

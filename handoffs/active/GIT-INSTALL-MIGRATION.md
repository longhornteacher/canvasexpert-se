# Git install migration: plan

Prepared 2026-10-21 against Dev branch `dev`, revision `415d1ef`; decisions recorded the same day. Nothing here is implemented. It does not authorize a release, a push, or changes to a teacher's UserInstall or UserSync.

## Decisions already made (developer, 2026-10-21)

- **Git is the primary install and update route.** The ZIP stays as a fallback for a PC where git or GitHub is unavailable.
- **Git owns the seeds.** Playbooks, `TEACHER.md`, `AGENTS.md`, `CLAUDE.md` and the docs update by pull. A teacher's own choices live in `Teacher_Override.md` and `context/`, which git never touches.
- **The Engine binary is a prebuilt release asset** the agent downloads and checks against a hash. No Go toolchain on the teacher's PC.
- **Existing playbook edits are converted once.** The agent diffs each edited playbook against the seed, moves the teacher's intent into `Teacher_Override.md`, then resets the file to the seed.
- **District PCs are locked down** (see memory `locked-down-work-pcs`). The developer says git runs, github.com is reachable, a downloaded `cse.exe` runs from a user folder, and today's ZIP install works there. Treat all four as observed by the developer, not yet re-checked by this work.

## Decisions on the four open questions (developer, 2026-10-21)

1. **Clone source:** the orphan `userinstall` branch of this repo, generated at release by flattening `workspace-template/`. The repo must be public.
2. **UserSync:** in a git install, sync only `context/` and `Teacher_Override.md`. `playbooks/` and the root `AGENTS.md`/`CLAUDE.md` come from git on each PC, and sync mode stops rewriting them into pointers.
3. **Engine fetch:** a tracked `get-engine.cmd` using `curl.exe` and `certutil -hashfile`, verified against the hash in the manifest in git. Later mirrored as an Engine command.
4. **Update trigger:** a teacher will not update if they have to ask, so an update is staged silently, then applied in a second step (see Update below).

## Target design

### Repository and release
- **Branch `userinstall`:** flattened `workspace-template/` (playbooks, `TEACHER.md`, `AGENTS.md`, `CLAUDE.md`, `README.md`, `TEACHING-ASSISTANT.md`, `cse.cmd`, `.claude/settings.json`, `engine-docs/`), `engine/manifest.json` (`version`, release asset URL, `sha256`), `get-engine.cmd` and a `.gitignore` for teacher and runtime paths.
- **`.gitignore` in that branch:** `context/`, `courses/`, `Teacher_Override.md`, `.cse/`, `engine/*.exe`, `playbook-old/`, `shipped/`, `*.cse.json`.
- **Release script:** `scripts/package.ps1 -Release` also builds the engine asset (`cse.exe` plus the launcher if `engine/` needs it), computes its hash, writes the manifest, commits the flattened tree to `userinstall`, and tags it. It still never pushes or publishes without being asked.
- **ZIP fallback:** the same flattened tree plus `engine/` plus the manifest, so both routes are the same content.

### Install (teacher says "set this up")
1. Agent runs `git clone --branch userinstall --single-branch <repo> <folder>`; that folder is UserInstall.
2. Agent runs `get-engine.cmd`: download the asset named in the manifest, verify the hash, place `engine/cse.exe`.
3. The first Engine run seeds the missing `context/` and `Teacher_Override.md` from the embedded template (`BootstrapBundled` and `Scaffold` already add only missing files).
4. The existing connect, Canvas and course steps in `install.md` and `setup.md` are unchanged.

### Update (two phases: stage silently, then apply)
**Stage** (automatic, background, at most once a day, at session start):
1. `git fetch` the `userinstall` branch. No working-tree change.
2. If `origin/userinstall`'s `engine/manifest.json` version differs from the installed Engine, download the asset into `engine/staged/` and verify its hash. Nothing live changes.
3. Record "update staged: <version>" in a small non-tracked state file. A failed or blocked fetch is silent and retried the next day; it never interrupts the teacher's work.

**Apply** (changes live files):
- **Ask at the end of work:** when the teacher's task is done and an update is staged, the agent asks "Can I install the latest?". A yes applies it then.
- **Otherwise auto at the next session start:** if it was ignored or declined, the next session applies it before any work and says so in one line.
- **Never auto-apply** while an unsent review, unsent work or a running review page exists; wait for the next opening.

**Apply steps:**
1. If tracked files are dirty (leftover edits): convert each edit into `Teacher_Override.md` text with the teacher's intent, copy the old file into `playbook-old/`, and `git checkout -- <file>`.
2. `git merge --ff-only origin/userinstall`.
3. If an Engine is staged: rename the old `cse.exe` to `cse.exe.<version>` (Windows allows renaming a running exe), move the staged one into place, and ask the teacher to restart apps still using the old one.
4. Report in plain words; no merge questions.

### Engine changes
- **Detect a git install** (`.git` in the root). `playbooks update`, the Settings "Playbooks and context files" adopt panel and `engine_info update-playbooks` report "managed by git" and do nothing there. The ZIP path keeps today's behavior unchanged.
- **Sync routing** per decision 2.
- **`engine_info`/`cse version`:** report install kind (`git` or `zip`) and the manifest comparison, so the agent can tell when the Engine lags the playbooks.
- **Retire in git installs:** `shipped/`, the `kept`/`teacher-owned` merge flow and the pre-update backup step I added in Batch 3. Keep them for the ZIP fallback.

### Playbook and doc changes
- **`install.md`:** replace the ZIP extract step with clone plus `get-engine.cmd`; add the update flow above; keep a short "ZIP fallback" section.
- **`AGENTS.md` (teacher):** agents never edit playbooks directly. Standing preferences go to `Teacher_Override.md`. Remove the `shipped/` and `playbook-old/` ignore wording that only applies to ZIP installs.
- **`README.md`, `setup.md`, `docs/releasing.md`, `docs/build.md`, `docs/playbook-ownership-and-updates.md`:** describe the git route; keep the ZIP as a fallback.
- **`Teacher_Override.md`:** gains a short note that this is where edits to shipped behavior go, because the playbooks themselves reset on update.

## Status

- **Step 1, release tooling: built and verified locally** (2026-10-21): `scripts/package-userinstall.ps1`, `scripts/get-engine.cmd`, `scripts/userinstall.gitignore`, documented in `docs/releasing.md`. Verified against a throwaway repo on PowerShell 7 and 5.1: clone of the `userinstall` branch, stage and install of the Engine, a tampered hash refused (exit 3, live Engine untouched), a download failure (exit 5), install over an existing Engine, replacing a running `cse.exe`, a fast-forward pull, ZIP contents (tree plus engine, no teacher files), and first-run seeding of `context/` in both the clone (git stays clean) and the extracted ZIP. Not verified: a real GitHub download, a district PC, or the release (non-dev) compile path.
- **Deviation from the plan above:** the manifest is `engine/manifest.txt` (`key=value`), not JSON, because `get-engine.cmd` has to parse it in batch.
- **Step 2, Engine changes: built** (2026-10-21): `internal/workspace/install_kind.go` (`InstallKind`, `ReadEngineManifest`, `IsSharedRulePathIn`, `ArtifactIdentity`); UserSync routing, bootstrap rewrites and the scaffold are git-aware in `rules_location.go` and the MCP file tools; `playbooks update/status/replace` report `managed_by: git` and change nothing; `cse version` and `engine_info` report `install_kind`, `manifest_identity` and `engine_matches_manifest`. Local Go tests only.
- **Step 3, playbooks and docs: written** (2026-10-21): `install.md` (git clone and `get-engine.cmd`, the stage-and-apply update flow, ZIP fallback), teacher `AGENTS.md`, `README.md`, `setup.md`, `Teacher_Override.md`; Dev `README.md` (agent and teacher install), `AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/build.md`, `docs/releasing.md`, `docs/playbook-ownership-and-updates.md`. `get-engine.cmd` gained `--manifest FILE` so a fetched-but-unpulled release can be staged. The documented sequence was run end to end against a throwaway repo: fetch, stage the new manifest and Engine (live Engine untouched), fast-forward merge, apply using the staged files with no download, previous Engine kept. The agent behavior itself (the once-a-day check, asking at the end of work, applying at the next start) is playbook text and has not been run by an agent.
- **Still open:** `package.ps1`'s own ZIP is still separate; retire it once this ZIP is accepted.

## Suggested order
1. **Release tooling:** manifest, engine asset, flattened-branch generation, `get-engine.cmd`, ZIP built from the same tree. Verify locally against a throwaway bare repo.
2. **Engine:** git-install detection, no-op update path, sync routing change, `version`/`engine_info` fields, tests.
3. **Playbooks and docs:** `install.md`, `AGENTS.md`, README, setup, releasing docs.
4. **Migration of the developer's UserInstall:** convert the 9 teacher-owned edited playbooks into overrides, switch the folder to a clone, confirm updates work. Only on request.
5. **District PC trial:** a real clone, engine download and update on a locked-down PC. Record the result separately from local tests in `capabilities.go` and `engine-capabilities.md`.

## Risks
- **A teacher or agent edits a tracked playbook** and the next pull fails. Mitigation: apply step 1, and the `AGENTS.md` rule above.
- **Engine and playbooks drift** if someone pulls without fetching the Engine. Mitigation: the manifest comparison on every update, and `engine_info` reporting the mismatch.
- **Running `cse.exe` blocks replacement.** Mitigation: rename, then place; the old file is kept as `cse.exe.<version>`.
- **`curl.exe`, `certutil` or batch files may be blocked** on a particular PC. Mitigation: the ZIP fallback, and a plain explanation to take to IT.
- **A force-pushed or rewritten `userinstall` branch** would break `--ff-only` pulls. Mitigation: the branch only appends release commits and is never rewritten, like the tags.

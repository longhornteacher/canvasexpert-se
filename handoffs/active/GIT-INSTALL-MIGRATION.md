# Git install migration: handoff

Written 2026-10-21 on the developer's main PC against Dev branch `dev`. Read this first when continuing the git-install work on another PC. It is self-contained: decisions, what was built and where, how it was verified, what is still unverified, and the exact next steps. It does not authorize a release, a push of the `userinstall` branch, a GitHub release, or any change to a teacher's UserInstall or UserSync.

## Resume here

- **State:** steps 1 to 3 of 5 are built and committed on `dev`. Step 4 (migrating the developer's own UserInstall) is planned below but **not started**, and nothing outward-facing has happened: no tag, no release, no `userinstall` branch on GitHub, no change to any UserInstall.
- **Commits (oldest first):** `3610b07` plan and decisions, `8b0a573` release tooling, `a592dd0` Engine git-install awareness, `cf3a9c4` playbooks and docs. Earlier the same day, batches 1 to 3 of the playbook/Engine conflict fixes and the small items landed (see `PLAYBOOK-ENGINE-CONFLICTS.md`).
- **Next action:** the developer approves the Step 4 plan (bottom of this file). First prerequisites, each needing an explicit go because they publish: bump `VERSION` to `1.1.0-beta.2`, add `/install-log.jsonl` to `scripts/userinstall.gitignore`, build the release, run the rehearsal, then publish assets before pushing the branch.
- **Working rules in force:** the repo's `AGENTS.md` (Dev) applies. The developer wants decisions confirmed one at a time, and wants a plan shown before anything is changed in their UserInstall. Memory note `locked-down-work-pcs` is relevant: district PCs are locked down, so installs must not need admin rights, PATH changes, a toolchain or unrestricted downloads.
- **Toolchain on the main PC:** Go is `.tools/go1.27.1/go/bin/go.exe` (a bare `go` is not on PATH). Verification is `go vet ./...` and `go test ./... -count=1` (all passing at `cf3a9c4`), plus `git diff --check` (CRLF warnings only). PowerShell 7 is `C:\Program Files\PowerShell\7\pwsh.exe`; Windows PowerShell 5.1 is in `System32\WindowsPowerShell\v1.0`. There is no `gh` CLI on the main PC.

## Why this exists

The ZIP install made two problems. A teacher had to be asked to update, and an extract over an existing folder could overwrite `AGENTS.md`/`CLAUDE.md`. The developer decided to replace the ZIP with an agentic install via git, keeping the ZIP as a fallback. Goal: a teacher tells their agent "install CanvasExpert from <repo>", gets the playbooks by `git clone`, gets the Engine by a checksum-verified download, and later receives updates without asking for them.

## Decisions (developer, 2026-10-21)

1. **Git is primary; the ZIP stays as a fallback** for a PC where git or GitHub is unavailable.
2. **Git owns the seeds.** Playbooks, `TEACHER.md`, `AGENTS.md`, `CLAUDE.md` update by pull. A teacher's choices live in `Teacher_Override.md`, `context/` and their own root guides, which git never tracks.
3. **Engine is a prebuilt release asset** (`cse.exe`, `CanvasExpert-SE-Cowork.mcpb`) fetched and checksum-verified. No Go toolchain on teacher PCs.
4. **Existing edits to shipped playbooks are converted once** into `Teacher_Override.md`, the old file archived, the file reset to the seed. (On the developer's own install there are none; see Step 4.)
5. **Clone source:** an orphan branch `userinstall` of this repo, generated at release by flattening `workspace-template/`. The repo is public (checked: anonymous requests return 200).
6. **UserSync in a git install carries only `context/`, `Teacher_Override.md` and the teacher's own root guides.** Playbooks and the shipped root files come from git on every PC, and sync mode no longer rewrites tracked `AGENTS.md`/`CLAUDE.md` into pointers.
7. **Engine fetch is a tracked `get-engine.cmd`** using `curl.exe` and `certutil -hashfile`, which Windows ships (batch files are the least likely thing to be blocked).
8. **Updates run in two phases without being asked.** Stage silently (fetch plus verified Engine download, no live change), then apply either when the teacher says yes at the end of their work ("Can I install the latest CanvasExpert?") or automatically at the start of the next session with a one-line notice. Never apply while a review, unsent work or a review page is open.
9. The developer reports git, github.com, a downloaded `cse.exe` run from a user folder, and today's ZIP install all work on district PCs. None of that has been re-checked by this work.

## What was built (by step)

### Step 1: release tooling (`8b0a573`)
- **`scripts/package-userinstall.ps1`** builds everything from one flattened tree. Parameters: `-GoExecutable`, `-Release`, `-OutputDirectory` (default `dist/userinstall`), `-BuildDirectory` (skip the compile and use `cse.exe` + `cse-mcp-launcher.exe` from a directory; for tests), `-UpdateBranch`, `-BranchName` (default `userinstall`), `-TargetRepository` (default this checkout), `-BaseUrl`. It compiles through `compile.ps1` (which also runs the Go tests), builds the Cowork `.mcpb` through `package-mcp.ps1`, and stages the tree. Outputs:
  - **The tree:** `workspace-template/` minus `context/` and `Teacher_Override.md`, using an allowlist (`AGENTS|CLAUDE|README|TEACHER|TEACHING-ASSISTANT.md`, `cse.cmd`, `.claude/settings.json`, `playbooks/*.md`), plus `VERSION`, `LICENSE`, `engine/THIRD_PARTY_NOTICES.md`, `webui.cmd` (from `scripts/package-webui.cmd`), `get-engine.cmd`, `.gitignore` (from `scripts/userinstall.gitignore`), `.gitattributes` (`*.cmd text eol=crlf`) and `engine/manifest.txt`. Text files are normalized to LF and `.cmd` to CRLF so the tree is identical whichever host builds it. It refuses a tree containing an `.exe`/`.mcpb`, `context/`, `courses/`, `.cse`, or without the `.claude` deny rule for `.cse/**`.
  - **Assets:** `<OutputDirectory>/assets/cse.exe`, `CanvasExpert-SE-Cowork.mcpb`, `SHA256SUMS.txt`.
  - **Fallback ZIP:** `<OutputDirectory>/CanvasExpert-SE-<identity>-userinstall.zip` = the tree plus `engine/cse.exe` and `engine/CanvasExpert-SE-Cowork.mcpb` (forward-slash entries, no teacher files).
  - **Branch commit (only with `-UpdateBranch`):** one local commit of the tree on an orphan branch, made with plumbing and a temporary index (`GIT_INDEX_FILE`, `GIT_WORK_TREE`, `write-tree`, `commit-tree`, `update-ref`), so the checkout, HEAD and index are untouched. An identical tree makes no new commit. It never pushes, tags or publishes.
  - **Release rules:** `-Release` needs a release compile (`build_kind release`); a non-release build must pass `-BaseUrl` (for example a `file:///` URL of the assets directory). In a non-release build the identity is `<version>-dev.<revision with + as .>.<built_at as yyyyMMddTHHmmssZ>`.
- **`engine/manifest.txt`** is `key=value` (the original plan said JSON; batch cannot parse JSON): `version`, `identity`, `base_url`, and `sha256.<file>` for `cse.exe` and `CanvasExpert-SE-Cowork.mcpb`. `base_url` defaults to `https://github.com/longhornteacher/canvasexpert-se/releases/download/v<VERSION>`.
- **`scripts/get-engine.cmd`** (CRLF required; bare-LF batch files break labels): reads the manifest, downloads each file with `curl.exe -fsSL --retry 2` into `engine\staged\<name>.part`, checks it with `certutil -hashfile ... SHA256`, then moves it to `engine\staged\<name>`. A staged file that already verifies is not downloaded again. Options: `--stage` (stop after verifying; nothing live changes), `--base-url U` (testing), `--manifest F` (read another manifest, to stage a fetched-but-unpulled release). Apply (no `--stage`): rename a live `cse.exe` to `cse.exe.previous` (Windows allows renaming a running exe; an older `.previous` is deleted first), move the staged files into `engine\`, delete `engine\staged\manifest.txt` and the empty staged folder. Exit codes: `0` ok, `2` bad option, `3` checksum mismatch (download discarded, live Engine untouched), `4` missing `curl.exe`/`certutil.exe`/manifest, `5` download failed, `6` the running Engine could not be replaced.
- **`scripts/userinstall.gitignore`** (becomes the branch's `.gitignore`): `/context/`, `/courses/`, `/Teacher_Override.md`, `/.cse/`, `/.claude/settings.local.json`, `/playbook-old/`, `/shipped/`, `*.cse.json`, `/engine/*` with `!/engine/manifest.txt` and `!/engine/THIRD_PARTY_NOTICES.md`. **Known gap:** `/install-log.jsonl` (operational state) is not listed and would show as untracked; add it before the first release.
- **Not reproducible:** the `.mcpb` is rebuilt with timestamps, so rebuilding changes its hash, the manifest and therefore the branch tree. Build a release once and keep the artifact set.

### Step 2: Engine support (`a592dd0`)
- **`internal/workspace/install_kind.go`:** `ReadEngineManifest(root)`, `InstallKind(root)` (`"git"` when `.git` and `engine/manifest.txt` both exist; `"zip"` with the manifest only; `""` otherwise, which includes a Dev checkout), `IsGitInstall`, `IsSharedRulePathIn(root, rel)`, `ArtifactIdentity(version, buildKind, revision, builtAt)` (same format the packager writes). In a git install, `playbooks/` and the root files `AGENTS.md`, `CLAUDE.md`, `TEACHER.md`, `TEACHING-ASSISTANT.md`, `README.md` are not shared; `context/`, `Teacher_Override.md` and the teacher's own root `.md`/`.txt` guides stay shared.
- **`internal/workspace/rules_location.go`:** `ResolveTeacherPath` uses `IsSharedRulePathIn`; `sharedFiles`, `copyShared`, `archiveShared` and the scaffold take a shared-path predicate; the first-use binding in `GetRulesLocation`, `SetRulesLocation` and `RestoreSyncBootstrap` skip the `AGENTS.md`/`CLAUDE.md` pointer rewrite in a git install. ZIP installs behave as before (a test checks both).
- **`internal/mcpserver/workspace_files.go`:** all shared-path checks use `IsSharedRulePathIn(s.root, ...)`; the file listing no longer routes `playbooks/` to UserSync in a git install. (Batch 3 also made `shipped/` readable but never writable through MCP.)
- **`internal/playbooks`:** `ManagedBy(root)`; in a git install `Update` returns `managed_by: git` plus a note and changes nothing (no `shipped/`, no `playbook-old/`), `Status` returns no rows, `Replace` refuses. The CLI and `engine_info` status JSON carry `managed_by`.
- **`cmd/cse/install_info.go` and `main.go`:** `cse version`/`--json` (run with `--workspace`, as `cse.cmd` does) add `install_kind`, `manifest_identity` and `engine_matches_manifest`; the text form adds `install_kind` and `manifest ... (engine matches|differs)` lines. `engine_info` `version` returns that text.
- **Tests:** `internal/workspace/install_kind_test.go` (kind detection, manifest parsing, identity format, shared-path routing, SetRulesLocation in git vs zip), `internal/playbooks/git_managed_test.go`. Capabilities page entry and `docs/engine-capabilities.md` row added, labelled local tests and throwaway-repo trial only.

### Step 3: playbooks and docs (`cf3a9c4`)
- **`workspace-template/playbooks/install.md`:** new sections "Get CanvasExpert onto the computer (git install)", "Updates: stage quietly, apply with the teacher's yes" and "ZIP fallback" (the old update text, kept for ZIP installs).
  - **Stage** runs at the start of a session, at most once a day, recorded by the date in `engine\last-update-check.txt` (inside the ignored `engine/*`). It does `git fetch origin userinstall`; does nothing if `HEAD` equals `origin/userinstall`; otherwise saves `git show origin/userinstall:engine/manifest.txt` to `engine\staged\manifest.txt` and runs `.\get-engine.cmd --stage --manifest engine\staged\manifest.txt`. Failures are silent and retried the next day. An update is **pending** whenever `HEAD` differs from `origin/userinstall`.
  - **Apply** (never while a review, unsent work or review page is open): ask at the end of the teacher's work, otherwise apply at the next session start with a one-line notice. Steps: convert any leftover tracked edits into `Teacher_Override.md` lines and archive the old file under `playbook-old/<date>/`, then `git checkout -- <path>`; `git merge --ff-only origin/userinstall` (stop and ask for help if it refuses; never force); `.\get-engine.cmd` (uses the staged, verified Engine when the manifest matches); check `cse version --json` shows `engine_matches_manifest: true`; ask for an app restart at a good moment; report in plain words.
- **Teacher `AGENTS.md`** routes by `install_kind` and forbids editing shipped files in a git install; **`README.md`**, **`setup.md`**, **`Teacher_Override.md`** updated. Dev **`README.md`** has the agent and teacher install instructions (clone the `userinstall` branch, `get-engine.cmd`); **`AGENTS.md`**, **`docs/ARCHITECTURE.md`**, **`docs/build.md`**, **`docs/releasing.md`** (including the release order: publish assets before pushing the branch) and **`docs/playbook-ownership-and-updates.md`** describe the git route.
- **Verified by script, not by an agent:** the stage-and-apply sequence above was run against a throwaway repo (fetch, stage a second release's manifest and Engine with the live Engine untouched, fast-forward merge, apply from the staged files with no download using a dead `--base-url`, previous Engine kept). The agent behavior (once-a-day check, asking at the end of work, applying at the next start) is playbook text that no agent has run.

## How to re-verify on another PC (throwaway repo, no private data)

1. `git init <tmp>\origin`, set `user.name`/`user.email`.
2. From the Dev checkout: `pwsh -NoProfile -File scripts\package-userinstall.ps1 -BuildDirectory dist -OutputDirectory <tmp>\out -UpdateBranch -TargetRepository <tmp>\origin -BaseUrl file:///<tmp-with-forward-slashes>/out/assets` (`dist\cse.exe` must be a build whose `version --json` matches `VERSION`; otherwise build first with `scripts\build.ps1`).
3. `git clone --branch userinstall --single-branch <tmp>\origin <tmp>\ui`; in it run `.\get-engine.cmd --stage`, `.\get-engine.cmd`, `.\cse.cmd version --json`, then `git status` (clean; `context/` appears after the first Engine run and is ignored).
4. Failure cases: edit a `sha256.` value in `engine\manifest.txt` (expect exit 3 and an untouched `cse.exe`); `--base-url file:///C:/nope` (exit 5); replace `engine\cse.exe` with `C:\Windows\System32\PING.EXE`, start it with `-n 40 127.0.0.1`, run `get-engine.cmd` (expect 0, the pinger left as `cse.exe.previous`), run it again while the pinger still runs (expect 6, live Engine intact).
5. Release 2: change something in the tree (for example pass a different `-BaseUrl`, or rebuild the `.mcpb`), run step 2 again with a different `-OutputDirectory`, then in the clone `git fetch`, save the new manifest to `engine\staged\manifest.txt`, `get-engine.cmd --stage --manifest engine\staged\manifest.txt`, `git merge --ff-only origin/userinstall`, `get-engine.cmd`.

## Environment notes that cost time

- **Run `.cmd` files from PowerShell, not Git Bash:** from bash, `cmd.exe //c` mangles the quoting and `get-engine.cmd` is "not recognized". Use `& .\get-engine.cmd` in PowerShell. `bash` inside the PowerShell tool resolves to WSL here; compute Windows temp paths directly.
- **CRLF:** the main PC uses `core.autocrlf=true`, so `git add` prints "LF will be replaced by CRLF" warnings; that is harmless. `package-userinstall.ps1` commits with `core.autocrlf=false` so the branch holds exactly the normalized bytes.
- **PowerShell `$null` to a string parameter becomes `""`:** setting an env var to `$null` through `[Environment]::SetEnvironmentVariable` left empty `GIT_INDEX_FILE`/`GIT_WORK_TREE` behind and broke `git commit-tree`. The script now removes the variable (`Remove-Item Env:`).
- **Large multi-line edits through a quoted bash heredoc failed once on an unterminated quote;** writing the text to a file and applying it with a small script worked.

## Not verified (record honestly)

- A real GitHub download (the manifest's `base_url`) and a release-built (non-dev) compile through `package-userinstall.ps1 -Release`. The throwaway trials used `dist` binaries built earlier and `file:///` URLs.
- Anything on a locked-down district PC: `curl.exe` and `certutil` under policy, a batch file run, git clone, running the downloaded `cse.exe`.
- The agent-driven update flow (stage once a day, ask at end of work, apply next session) and the Claude extension picking up a changed launcher.
- `package.ps1`'s own ZIP is still built separately. Retire it once the new ZIP is accepted.

## Step 4: migrate the developer's UserInstall to git (planned, not started)

Based on a read-only look at `C:\Users\adamb\CanvasExpert-SE` on the developer's main PC. The other PC's folder is probably different; re-run the inspection there (`.\cse.cmd playbooks status`, a directory listing without opening `.cse/`).

### What the inspection found
- **No playbook is edited.** Every playbook, `AGENTS.md`, `CLAUDE.md`, `TEACHER.md` and `Teacher_Override.md` reported `same` as the Engine's seed. The "9 teacher-owned" rows from an earlier `playbooks update` were the eight `context/` files plus `Teacher_Override.md`: teacher data an update never replaces, not edited playbooks. So there is **nothing to convert into overrides** on that install, and a seed-playbook fix does reach it (as an `updated` file). `Teacher_Override.md` still says "None yet." `PLAYBOOK-ENGINE-CONFLICTS.md` carried the same misreading and has been corrected.
- **Plain local install.** No `.git`, rules mode `local` (no UserSync). Teacher data: `context/`, `courses/` (four course folders, about 4 MB), `.cse/` (private, not read), `install-log.jsonl`. Dead weight git will ignore: `shipped/`, `playbook-old/`, `engine/cse.exe.old`, `engine/cse.exe.1.1.0-beta.1-20261019`, `engine/CanvasExpert-SE-Cowork-this-install.mcpb`, `engine/pre-update-20261021`.
- **Five `cse.exe` processes and two Claude extension launchers were running** from that folder; the extension's launcher and the other app connections point at its `engine\cse.exe`. Keeping the folder where it is keeps every connection and the desktop shortcut valid.
- **GitHub:** public. `origin` has `dev` (at `236a20f`, behind the local `dev`) and `main` only. No `userinstall` branch, no 1.1.0-beta release (tags are `v1.0.0` and `v1.0.0-beta.1`). `releases/latest` is `v1.0.0`.

### Approach: adopt the folder in place
Turn the existing folder into the clone instead of cloning elsewhere and moving data. Git only touches files it tracks; `context/`, `courses/`, `.cse/`, `engine/*` and `Teacher_Override.md` are ignored and survive. A new clone elsewhere changes the Engine path and needs every app reconnected, the riskier route on a locked-down PC.

### Prerequisites (each needs the developer's explicit go, because they publish)
1. **Version.** Recommended: bump `VERSION` to `1.1.0-beta.2` (betas increment consecutively; this includes the work since beta.1). Alternative: release `1.1.0-beta.1` as it stands, without today's fixes.
2. **Repo fix:** add `/install-log.jsonl` to `scripts/userinstall.gitignore`.
3. **Release build on a clean tree:** commit, tag `v<VERSION>` at HEAD (the existing release rule needs a clean checkout and an immutable tag), then `package-userinstall.ps1 -Release -UpdateBranch`.
4. **Publish, in this order:** push `dev` and the tag; create the GitHub release `v<VERSION>` as a **pre-release** with `cse.exe` and `CanvasExpert-SE-Cowork.mcpb` from `dist/userinstall/assets` (no `gh` on the main PC, so by the web page or an API call with a token the developer provides); then push the `userinstall` branch. Assets first, so the manifest's `base_url` resolves before anyone can pull. The branch only ever appends commits; never rewrite it.

### Rehearsal (before touching the real folder; no private data)
On a skeleton copy (structure, `context/`, `engine\`; no `courses\`, no `.cse\`): `git init`, add the real `origin`, `git fetch origin userinstall`, `git checkout -f -B userinstall origin/userinstall`, `.\get-engine.cmd` (this is the first real GitHub download), `.\cse.cmd version --json` (expect `install_kind: git`, `engine_matches_manifest: true`), `git status` (expect clean), and confirm `context/` and `Teacher_Override.md` are unchanged.

### The migration (a quiet moment; no review or unsent work open)
1. Close Claude and any Code or Codex session using CanvasExpert.
2. Back up the small tracked-equivalent set to `engine\pre-git-migration-<date>\`: `AGENTS.md`, `CLAUDE.md`, `TEACHER.md`, `README.md`, `TEACHING-ASSISTANT.md`, `cse.cmd`, `webui.cmd`, `VERSION`, `LICENSE`, `playbooks\`, `.claude\`. Teacher data is not touched by git and needs no copy.
3. In the folder: `git init -b userinstall`, `git remote add origin https://github.com/longhornteacher/canvasexpert-se.git`, `git fetch origin userinstall`, `git checkout -f -B userinstall origin/userinstall`, `git branch --set-upstream-to=origin/userinstall`.
4. `.\get-engine.cmd`.
5. Verify: `.\cse.cmd version --json`, `.\cse.cmd playbooks status` (`managed_by: git`), `git status` clean; reopen Claude and check `list_courses` returns the same `install_root` and nicknames; open the hub from the desktop shortcut.
6. Optional cleanup, only on the developer's say-so: `shipped/`, `engine\cse.exe.old`, older engine copies, the `this-install` extension file.

### Rollback
Data never moves. Delete `.git`, copy the backed-up files back from `engine\pre-git-migration-<date>\`, rename `engine\cse.exe.previous` to `engine\cse.exe`. The folder runs as the ZIP-style install it was.

### Risks
- **The Claude extension's launcher may need re-adding** if the new `.mcpb` carries a changed launcher; check after the first restart.
- **Running Engines keep the old binary** until their apps restart; the rename means the swap itself does not fail.
- **The first real GitHub download is untested.** A failure with exit code 4 or 5 changes nothing in the folder.
- **A pre-release leaves GitHub's "latest" link on `v1.0.0`,** so ZIP users are not moved to a beta.

## Step 5: district PC trial (not started)
A real clone, Engine download and update on a locked-down PC, run by or with the developer. Record the result separately from local tests in `internal/localui/capabilities.go` and `docs/engine-capabilities.md` (the git-install entry currently says local tests and a throwaway-repo trial only).

## Risks across the design
- **A teacher or agent edits a tracked playbook** and the next pull conflicts. Mitigations: the apply step converts leftover edits into overrides, and teacher `AGENTS.md` forbids editing shipped files in a git install.
- **Engine and playbooks drift** if someone pulls without fetching the Engine. Mitigation: `engine_matches_manifest` in `cse version` and `engine_info`, checked after every apply.
- **`curl.exe`, `certutil` or batch files may be blocked** on a particular PC. Mitigation: the ZIP fallback and a plain explanation to take to IT.
- **A rewritten `userinstall` branch** would break fast-forward pulls. Mitigation: append-only, like the tags.

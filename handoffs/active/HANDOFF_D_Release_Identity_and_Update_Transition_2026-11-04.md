# HANDOFF D: One artifact set per release; a coherent update transition

Date: 2026-11-04. Slice D of the [unified architecture review](../Unified%20Architecture%20Review.md). Layer: **Dev packaging scripts** (D1) and **Engine update lifecycle** (D2).

Developer decision 8: fix artifact immutability independently, and inspect and specify the smallest coherent update transition before adopting Engine-first as the solution. So:
- **D1 is implementation.**
- **D2 starts with a short written spec that the developer approves before any code.**

Before starting: `git fetch`, bring `dev` up to date, read root `AGENTS.md`, [docs/releasing.md](../../docs/releasing.md), [docs/build.md](../../docs/build.md), seed `workspace-template/playbooks/install.md` ("Updates" section) and the [shared execution notes](../README.md#execution-notes-for-the-active-handoffs). These files were unchanged between `f58ecaf` and `2818e36`. No release, tag, push of the `userinstall` branch, or UserInstall update is authorized by this handoff.

---

## D1: Refuse to replace a released artifact set

### Why

A teacher's git install verifies downloaded Engine files against hashes in `engine/manifest.txt` on the `userinstall` branch. The published GitHub release assets for `vX.Y.Z` must match those hashes forever. Today, rerunning the release packager for the same version:
- produces new bytes, because `cse.exe` embeds a build timestamp and the MCPB is not reproducible;
- overwrites the local assets;
- commits a new `userinstall` tree whose manifest points at hashes that no longer match what was already uploaded.

Pushing that commit would break updates for every teacher. No broken release has been observed; this closes the hole.

### Current code

- **Build timestamp.** `scripts/compile.ps1:99` sets `$builtAt = UtcNow`; `:103` embeds it in `cse.exe` via `-X main.builtAt` and `-X ...mcpserver.BuiltAt`. `-Release` requires a clean tree and the `v<VERSION>` tag at HEAD (`:90–98`). So `cse.exe` bytes differ on every build, and the MCPB differs too.
- **`scripts/package-userinstall.ps1`** (241 lines):
  - Identity and inputs:
    - `artifactIdentity` is `VERSION` for `-Release` (`:102–107`).
    - Output defaults to `dist/userinstall` (`:35`).
    - `-BaseUrl` defaults to the GitHub release download URL for `v$productVersion` under `-Release` (`:151–154`).
  - Manifest: `engine/manifest.txt` (`:156–161`) has keys `version=`, `identity=`, `base_url=`, `sha256.cse.exe=`, `sha256.CanvasExpert-SE-Cowork.mcpb=`.
  - Overwrites and deletes:
    - `:174–181` copies assets into `dist/userinstall/assets/` with `-Force` and rewrites `SHA256SUMS.txt`;
    - `:188–189` deletes an existing same-identity ZIP.
  - Branch update and checks:
    - `-UpdateBranch` (`:193–228`) writes a tree with a temporary index, skips the commit only if the tree is identical (`:216`), otherwise runs `commit-tree` + `update-ref refs/heads/userinstall`. It never pushes.
    - There is **no** check, anywhere, for an existing release of this version.
- **Existing protection to copy** (`scripts/package.ps1:141–146`):
  ```powershell
  if ($Release -and ((Test-Path -LiteralPath $releasePackagePath) -or (Test-Path -LiteralPath $releaseMcpbPath))) {
      throw "Refusing to overwrite immutable release artifact in $releaseRoot"
  }
  ```
- **Release procedure.** [docs/releasing.md](../../docs/releasing.md) "Git install distribution" is manual: run `package-userinstall.ps1 -Release -UpdateBranch`, upload 4 assets under `v<VERSION>` (ZIP as `CanvasExpert-SE-windows.zip`), then push the branch. It already warns that "rebuilding changes the manifest hash and produces a new branch commit". No script publishes; CI runs `package.ps1` without `-Release`.

### What to build

Under `-Release` only (dev builds keep today's behavior):

1. **Refuse before compiling** when either of these already exists:
   - a release asset set for this version locally: use a versioned folder such as `dist/userinstall/release/<VERSION>/` holding `cse.exe`, the MCPB, `SHA256SUMS.txt` and the ZIP;
   - a commit on the local `userinstall` branch whose `engine/manifest.txt` has `version=<VERSION>`. Read it with `git show <branch>:engine/manifest.txt`, and also check `origin/userinstall` if it is present.

   Message: name the version and say a published release is immutable and the next change needs a new version.
2. Write release assets only into the versioned folder, never with `-Force` over an existing file. The unversioned working output may stay for dev builds.
3. Do not attempt reproducible builds. Do not remove the timestamp.
4. Update [docs/releasing.md](../../docs/releasing.md): one sentence on the new refusal and what to do (bump the version). Remove nothing else.

### Done when

- **Local fixture run.** Use `-BuildDirectory` (prebuilt binaries) against a temporary `-TargetRepository` (a throwaway `git init` with a fake `userinstall` branch):
  - first `-Release -UpdateBranch` succeeds;
  - a second identical run refuses without touching the existing assets or branch;
  - a non-release run still works.

  Record the commands in the commit message. Do not push anything; do not create a real tag.
- `git diff --check`. The Go checks are unaffected but run them if Go files changed.

---

## D2: Specify the update transition (spec first, then the smallest fix)

### Why

Applying an update in a git install changes playbooks and the Engine in two steps. Several failure points leave a mismatched or broken install, and an already running MCP adapter keeps old code regardless. The review proposed "swap the Engine first, then playbooks"; the developer wants the actual sequence inspected first.

### Current sequence

`gitupdate.Apply` (`internal/gitupdate/gitupdate.go:128–185`):
1. not a git install → refuse;
2. any open review, setup or students page → `held`, nothing changed (`:133–137`);
3. nothing pending → `current` if `engineMatches`, else run `GetEngine` → `repaired`;
4. tracked files edited (`git diff --name-only HEAD`):
   - without `archiveEdits` → `held` ("move the teacher's choices into Teacher_Override.md");
   - with it → copy to `playbook-old/<ts>/`, then `git checkout -- <files>` (`:155–171`);
5. `git merge --ff-only origin/userinstall` (`:172`); failure → `held`. With archive-edits the edits are already reverted at this point; the archived copies remain;
6. `GetEngine(root)` (`:176`); failure → "playbooks were updated but the Engine could not be installed; run .\get-engine.cmd";
7. `engineMatches()` (`:179`, `:226–232`) compares **only `engine/cse.exe`**'s SHA-256 with the manifest, not the MCPB;
8. `applied`: "restart the apps using CanvasExpert at a good moment so they load the new Engine" (`:183`).

**Staging** (`Stage`, `:80–121`) fetches `origin/userinstall`, writes its manifest to `engine/staged/manifest.txt`, and runs `get-engine.cmd --stage --manifest engine\staged\manifest.txt`, which downloads and verifies the next version's files into `engine/staged/` without touching live files. `available()` requires every staged hash to match.

**Swap** (`scripts/get-engine.cmd:90–114`):
1. `del cse.exe.previous` (fails silently if that file is still running);
2. `ren cse.exe cse.exe.previous`; if `cse.exe` still exists → exit 6 "Close CanvasExpert…";
3. `move /y` each staged file into place (`cse.exe`, then the MCPB); any failure → exit 6;
4. remove the staged folder.

**Process model.** `cmd/cse-mcp-launcher` runs `<root>\engine\cse.exe --workspace <root> mcp`, so the MCP adapter *is* the live `engine\cse.exe`. `update-apply` runs inside that adapter (`internal/mcpserver/operations.go:487–503`), so the swap renames the running binary. `Server.command` launches child Engines from `os.Executable()` (`server.go:478`). Not verified: whether that path still resolves to the old binary after a rename; if it resolves to the new `engine\cse.exe`, an old adapter would launch new children.

### Failure table (to confirm in the spec)

| Failure point | Resulting state |
|---|---|
| Held at step 2/4/5 | Unchanged (except archive-edits already reverted the edited files) |
| `get-engine` exit 3/4/5 or exit 6 at rename | New playbooks and manifest, old `cse.exe`; the next Apply returns `repaired` by re-running `GetEngine` |
| Exit 6 at the move of `cse.exe` | **No `engine\cse.exe` at all**; the launcher and `cse.cmd` cannot start |
| MCPB move fails after `cse.exe` succeeded | `engineMatches` is true, so later checks say `current` while the MCPB is stale |
| Any success | Running adapters keep old code until the app restarts |

**Mismatch detection today.**
- `cse version --json` reports `engine_matches_manifest` (`cmd/cse/install_info.go`).
- `engine_info version` shows `loaded_adapter` and `child_engine` side by side, but nothing compares them.
- Git installs skip seed fingerprints (`internal/playbooks/updates.go:54–66`).
- **Tests.** `internal/gitupdate/gitupdate_test.go` has two tests with `GetEngine` stubbed; no failure-path coverage.

### Deliverable 1: a short spec, approved by the developer

Write it as a section appended to this handoff. Answer these:
1. **Order.** With the Engine already staged and hash-verified before Apply, does swapping it before the merge remove a failure class, or only move it? (A swap can still fail at rename/move.)
2. **Rollback.** Should `get-engine.cmd` restore `cse.exe.previous` when the move of the new `cse.exe` fails, so the install is never left without an Engine? This looks like the smallest high-value fix regardless of order.
3. **Completeness.** Should `engineMatches` check every manifest asset (cse.exe and MCPB)?
4. **Running adapter.** Is the restart line enough? Should the adapter detect, on its next tool call, that `engine/manifest.txt` no longer matches its own compiled identity, and return a one-line "restart the app" notice instead of running mixed versions? Check what `os.Executable()` returns after a rename on Windows before relying on it.
5. **Held states.** What exactly is the teacher told in each failure state? Point at `install.md` lines 164–182.

### Deliverable 2: implement only what the developer approves

Add a focused test per changed failure point: `GetEngine` stub failing at each step, and a stale MCPB.

## Not in this slice

Reproducible builds, an installer, changes to the ZIP fallback, or any real update of the developer's UserInstall.

---

## D1 status (2026-11-04): done

Implemented in commit `1500227` and fixture-tested (first `-Release -UpdateBranch` succeeds; a second identical run, and a run into a fresh output folder after the branch has the version, both refuse with assets and branch untouched; non-release builds unchanged). Nothing was pushed or tagged.

## D2 spec (proposed 2026-11-04, awaiting developer approval; no D2 code written)

### What was checked

- **`os.Executable()` after the swap.** A test program renamed its own running `.exe` and put a new file at the old path; `os.Executable()` kept returning the **old path**, which now holds the **new** binary. `Server.command` launches children from `os.Executable()` (`server.go:478`). So after `update-apply`, a still-running old adapter starts **new** child Engines: old adapter code plus new Engine, with no warning. This is the real mixed-version case, and it exists on every successful update until the app restarts.
- `get-engine.cmd` today leaves no Engine at all if the move of the new `cse.exe` fails after `cse.exe` was renamed to `cse.exe.previous` (exit 6 at `:106-110`), and keeps no backup of the old MCPB.
- `engineMatches` (`gitupdate.go:226-232`) compares only `cse.exe`.

### Answers

1. **Order.** Moving the Engine swap before the merge does not remove failure classes by itself (the swap can still fail at rename), but it changes which mismatch is left behind. A new Engine with old playbooks is the safer one (the Engine keeps old commands); an old Engine with new playbooks can send the agent to commands that do not exist. Recommended sequence: (a) check every hold condition first, with no changes: open pages, tracked edits, and `git merge-base --is-ancestor HEAD origin/userinstall` (a fast-forward check that needs no merge); (b) swap the staged Engine; (c) `git merge --ff-only`. After (a), (c) failing is very rare. If it does fail, the report says the Engine is new and playbooks are old, and the next `update-apply` still sees the pending commits and finishes it.
2. **Rollback.** Yes. In `get-engine.cmd`, rename both `cse.exe` and the MCPB to `*.previous` before moving anything; if any move fails, rename the `*.previous` files back and exit 6. The install is never left without an Engine, and a failed MCPB move no longer leaves a half-new set. This is a small, contained change to one script and is worth doing regardless of the other answers.
3. **Completeness.** Yes: `engineMatches` checks every `sha256.*` asset in `engine/manifest.txt` (`cse.exe` and the MCPB), so a stale MCPB reports `repaired` instead of `current`.
4. **Running adapter.** The restart line alone is not enough, because of the mixed-version case above. Recommended: before the adapter starts any child Engine, compare its own compiled identity (`workspace.ArtifactIdentity`) with the `identity=` line in `engine/manifest.txt`; on mismatch return one plain notice ("CanvasExpert was updated. Close and reopen this app so it loads the new version.") instead of running. `engine_info` `version` and `update-status` stay allowed so the agent can still explain. The check is a single small file read per tool call. Not recommended: killing the adapter or trying to reload in place.
5. **What the teacher is told in each state** (agent-facing text already in `install.md` "Updates"; the spec only adds the new notice):
   - `held` (page open, edits, cannot fast-forward): nothing changed; try later, or the existing edits and fast-forward wording.
   - `repaired`: nothing to tell.
   - `applied`: "You're on the newest CanvasExpert; your settings and choices are unchanged." plus the close-and-reopen line, now backed by the notice above.
   - Engine swap failed (exit 6): nothing changed and the old Engine still works (after item 2); ask them to close the apps, then run `.\get-engine.cmd`.
   - Merge failed after a successful swap: the Engine is new and playbooks old; the next apply finishes it.

### Smallest approved scope (proposed)

Items 2 and 3 (script rollback, full `engineMatches`), item 1 (preflight, then swap, then merge) and item 4 (restart notice), each with a focused test: stubbed `GetEngine` failing at each step, a stale MCPB, and an adapter whose identity differs from the manifest. If you would rather keep it smaller, items 2 and 3 alone remove the unbootable-install case and the false `current`.

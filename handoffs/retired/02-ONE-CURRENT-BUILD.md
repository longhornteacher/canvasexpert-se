> **Retired 2026-10-13 (work landed).** Execution record only. Durable build/release guidance: [docs/build.md](../../docs/build.md), [docs/releasing.md](../../docs/releasing.md); status and evidence limits: [docs/engine-capabilities.md](../../docs/engine-capabilities.md). Remaining items are real client/Canvas observations, not code work.

# 02 — One current build, one teacher download

Status: agreed direction, implementation pending. Start with [01-HANDOFF-CLEANUP.md](01-HANDOFF-CLEANUP.md) for execution order and agent rules. User clarification: **one current build and download; actual installed copies are outside this cleanup**.

## Target contract

There is one current build of CE-SE, including the current WebUI. The teacher receives **`dist/CanvasExpert-SE.zip`**. It contains the same Engine as the current development output, plus the thin Claude extension/launcher and clean starter files. The extension is an integration component, not a separately marketed build or second Engine. Version/revision/build time remain available for troubleshooting; deleting `+dirty` text without fixing artifact ownership is not this task.

Use `scripts/package.ps1` as the canonical operation that builds, checks and assembles a coherent current output. Keep `build.ps1` as a compatibility entry if useful, but route it through the same orchestration rather than leaving it able to silently update the current EXE while the current ZIP stays old. Refactor the actual compiler step into a small internal helper if needed to avoid recursion. Do not add a new build framework.

Default successful output allowlist in `dist/`:

| File | Purpose |
|---|---|
| `cse.exe` | Current Engine used by Source tooling and copied byte-for-byte into ZIP |
| `cse-mcp-launcher.exe` | Matching thin launcher, also embedded in the packaged extension |
| `CanvasExpert-SE.zip` | The one teacher download |
| `SHA256SUMS.txt` | Hash of the current ZIP using that stable filename |

Generate the MCPB in staging and include it inside the ZIP; do not leave a second top-level download, timestamped/versioned duplicate, or stale release-notes file by default. The teacher's installer can still prepare its Install-bound MCPB. Keep licenses/notices, private-data exclusions, forward-slash ZIP entries and the existing single-folder extraction layout.

This changes local artifact retention/naming. Preserve honest development versus release identity and existing protections for published immutable releases. Do not retag, overwrite/delete remote release assets, publish, bump VERSION, or relabel uncommitted Source as a release. Public release mechanics may use an explicit release-only output path if required; no release archive/history belongs in the ordinary current-output directory. Keep that optional path small and document it; it is not a second default build line.

## Observed clutter and affected paths

At planning time `dist/` has 18 files: five ZIPs, five MCPBs, five checksum files, two executables and one release-note file. Three stamped development generations coexist with version-only copies and stable aliases. `package.ps1` and `package-mcp.ps1` create that duplication deliberately. `build.ps1` can replace the loose Engine independently of the ZIP. This is one codebase with inconsistent output ownership, not evidence that three product implementations need merging.

Source `scripts/webui.ps1` uses `dist/cse.exe` and auto-builds based on a timestamp scan that considers Go/module/template inputs but does not explicitly cover the extracted WebUI HTML/CSS/JS files. Inspect that dependency gap while consolidating so a redesign asset edit reaches the current WebUI. Preserve the current Bench design, extraction and registry; no redesign redo.

Known generated CE-SE binaries also exist under `.test-workspaces/engine-unattended-20261009/` and `.test-workspaces/field-probe-20261010/`. Those folders also contain evidence or work; never recursively remove them wholesale. `output/`, `tmp/`, `.tools/` and `.cache/` are not blanket deletion targets.

## B1 — One producer and a coherent output set

**Files:** `scripts/build.ps1`, `scripts/package.ps1`, `scripts/package-mcp.ps1`; narrowly scoped new helper only if it reduces duplication.

1. Implement the target contract above. Compile Engine/launcher once per canonical run, package those exact bytes, and eliminate default timestamped/versioned siblings and top-level MCPB output.
2. Stage before replacing the current set. A failed build/package must leave the prior usable current download intact and report failure. Use direct staging/copy/rename with ordinary cleanup, not a persistent transaction service. Avoid claiming a new current build before all output components succeed.
3. Internal packaging helpers consume the staged identity/binaries instead of selecting arbitrary older `dist` files. Preserve VERSION and build provenance inside the product.
4. Existing `-Release` behavior must remain truthful and bounded. If preserving a public immutable release requires separate explicit output, ensure default builds never populate that location.

**Check/done:** use temporary output/staging or existing script test hooks. Confirm one build invocation, archive inputs match its binaries, repeat output names stay fixed, and a simulated packaging failure does not destroy the prior current download. Add a small script-level check only where useful; no generic artifact registry or test matrix. Do not yet do the final delivery build (B3 owns it after install fixes).

## B2 — Make every supported entry point use it

**Files:** `scripts/webui.ps1`, `webui.cmd`, `.github/workflows/ci.yml`, `docs/build.md`, `docs/releasing.md`, `README.md`, `CONTRIBUTING.md` if affected. Coordinate README edits with I4; I4 owns teacher-facing app wording.

1. Route Source build, package, WebUI auto-build and CI through the single producer. Update any command that assumes a standalone generated MCPB. Keep `node scripts/check-mcp.mjs dist/cse.exe` pointed at the canonical Engine unless there is a necessary path change.
2. Account for all embedded product inputs in freshness, especially `internal/localui` assets, Engine docs, seed files and module/version inputs. Reuse Go/embed knowledge or a simple bounded dependency check; avoid a repository-wide dependency framework. A stylesheet-only change must not serve an old WebUI.
3. Distinguish developer scratch compilations from current output. Agents may compile temporary test binaries into task-owned temp folders and must remove them at the end; they do not produce an alternate teacher download.
4. Document one ordinary build command, one current executable, one download. Explain `+dirty` as diagnostic provenance, not an artifact family teachers must choose among. Keep release rules only in the release guide and make current README download instructions match this contract.

**Check/done:** canonical paths agree across scripts/docs/CI; a synthetic embedded-asset edit is noticed by the launch/build path (restore fixture edits afterward). Repeat checks target changed behavior, not all historical WebUI scenarios. No live Install launch needed.

## B3 — Build the repaired product once and verify it

**Depends on:** C2–C3, B1–B2, I1–I5 integrated. Coordinator owns this slice; other agents stop packaging.

1. Run the canonical package operation with the exact Go version from `go.mod` (currently `.tools/go1.27.1/go/bin/go.exe`). It owns full `go test ./...`, `go vet ./...` and builds; avoid running the same full checks separately immediately before it.
2. Run the existing compiled MCP check. Inspect archive paths, clean starter content, licenses, Engine hash equality, thin-launcher target/identity and checksum naming.
3. Extract the final ZIP into one task-owned temporary Install with spaces in its path. Use synthetic client configuration and isolated/mocked credentials/network for first-install checks; an arbitrary temporary folder alone does not isolate Windows Credential Manager. Do not accidentally exercise this machine's saved PAT. Test actual installer readiness/start/reuse, working launchers, and the local MCP Install-root result. I1–I5 provide the focused behavioral checks; do not repeat an unrelated scoring campaign.
4. Stop only owned test processes and remove owned disposable extractions/binaries. If browser/client evidence cannot be gathered without a real teacher action, keep it pending and finish independent local checks.

**Done when:** one current ZIP includes all repaired Source behavior, its Engine matches `dist/cse.exe`, and focused local package/install checks pass. Record version/revision/time and ZIP hash in this file's completion row, not a new artifact-history document. If something fails, repair only the failing path and rebuild because the delivered bytes changed.

## B4 — Remove obsolete generated artifacts and finish

**Depends on:** a usable B3 output. The user has authorized removing old generated builds; do not ask again for ordinary confirmed generated artifacts within this scope.

1. Inventory only Source-owned generated CE-SE outputs and specifically identified task-created temporary locations. Resolve absolute paths and verify containment before deletion. Use native PowerShell `-LiteralPath`; do not build destructive commands across shells.
2. Delete obsolete ZIP/MCPB/checksum copies and stale generated release notes from `dist` after checking the current allowlist. Remove known obsolete CE-SE test binaries where they are disposable generated executables, retaining evidence, source helpers, fixtures, pulled work and unsent work. Unknown ownership is a reason to skip that specific item, not delete its parent or block independent cleanup.
3. Do not move garbage into a new local archive directory; that preserves the problem. Historical build identities already live in evidence docs/Git. Do not delete real teacher Installs, credentials, Sync content, toolchains/caches, unrelated PDFs/output, or remote release assets.
4. List final `dist` contents and verify that rerunning the ordinary producer cannot create dated siblings. Update capability evidence only for actual checks/changes, jointly with I6.

**Done when:** one current product output set, one obvious teacher ZIP, no obsolete generated builds within confirmed cleanup scope, no dangling current documentation links to deleted downloads. Report any specifically skipped file and reason. Actual teacher installation update is separate.

| Slice | Status |
|---|---|
| B1 single producer | Done 2026-10-13: `package.ps1` is the one producer (compiles once via internal `compile.ps1` into staging, builds the MCPB in staging and embeds it, publishes `cse.exe`, `cse-mcp-launcher.exe`, `CanvasExpert-SE.zip`, `SHA256SUMS.txt` only after all steps pass; `-OutputDirectory` allows temp output; `-Release` also writes versioned assets to `dist/release/` with overwrite refusal). `build.ps1` is a wrapper; `package-mcp.ps1` is internal. Checked into a temp directory: exactly four files, zip Engine hash equals loose Engine, MCPB inside zip, a failed `-Release` run left output unchanged and removed staging. Full tests/vet passed in that run. |
| B2 entry points/docs | Done 2026-10-13: `webui.ps1` freshness now covers go.sum, VERSION, engine-docs and WebUI templates/assets (regex checked against an asset path) and builds through the single producer; CI already calls `package.ps1` and `check-mcp.mjs dist/cse.exe`; `docs/build.md`, `docs/releasing.md`, `CONTRIBUTING.md`, `docs/mcp-adapter.md` updated. README download wording left to I4. No live webui launch run. |
| B3 integrated package | Done 2026-10-13: `package.ps1` with Go 1.27.1 ran full tests/vet and builds; compiled MCP check passed 26 tools. Final current output: version `1.0.0-beta.1`, revision `288129f00f80+dirty`, built `2026-10-13T14:28:30Z`, development; `dist/CanvasExpert-SE.zip` SHA-256 `51f47ecea14633b5932df15650e05bb62695563be2c57354edce615e2704e197`. (An earlier same-day package was superseded after a status-text edit.) Archive inspected (25 forward-slash entries, starter-only context, licenses, MCPB inside, Engine byte-equal to `dist/cse.exe`). Checked on the first package from a temporary Install with spaces: Work registration to an isolated `CODEX_HOME`, byte-stable repeat, local MCP check, offline doctor. The installer's browser/WebUI launch was not run in the package (it would use this machine's saved credential). Real client/Canvas observation remains pending. |
| B4 old outputs removed | Done 2026-10-13: `dist/` holds only `cse.exe`, `cse-mcp-launcher.exe`, `CanvasExpert-SE.zip`, `SHA256SUMS.txt` (14 old ZIP/MCPB/checksum/release-note files deleted); disposable `bin/*.exe` removed from `.test-workspaces/engine-unattended-20261009` and `field-probe-20261010`; leftover `cse-mcp-check-*` temp dirs removed. Skipped: `.test-workspaces/engine-unattended-20261009/restore_fixture.exe` (unknown ownership/possible restore helper). Evidence, helpers and work untouched. Current docs have no dangling links to deleted downloads. |

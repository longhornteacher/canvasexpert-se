# Single-folder WebUI and MCP implementation

Status: completed (2026-10-11; source and package verification).

2026-10-11. The teacher approved this direction after evaluation. Implement it, then report the changed behavior and relevant verification. Read repository AGENTS.md, ARCHITECTURE.md and SCORING-RULES.md first. Preserve the existing uncommitted MCP work and other changes.

## Agreed product behavior

Additional teacher constraint: no `.cmd` files belong in the agent's synced OneDrive folder. CE-SE launchers stay in the local installation; MCP registration must target that installation.

CE-SE has one local installation folder: `engine/`, `context/`, `playbooks/`, `courses/`, standing instructions and private runtime data belong there. The teacher does not select or maintain a second CE-SE workspace. The agent's own synced workspace is independent. Remove teacher-facing workspace selection and sync instructions from the WebUI. WebUI and MCP use the same installed files, so active-course changes do not require copying, synchronization or restarting MCP.

Playbook access already works: the connected server listed all six playbooks and read setup.md. Keep those tools and privacy boundaries; do not build another exposure mechanism.

## Findings to resolve

- `scripts/package-webui.cmd` already passes the package root explicitly; `internal/localui/hub.go` still displays a synced workspace picker, permits changing root and restores a separately remembered folder.
- WebUI saves courses into its root's `context/courses.md` with `current: true/false`. MCP captures its own root at startup. Two running MCP processes were configured for the older `Teaching Assistant Project` folder, whose course file was empty. The remembered hub path was different and no longer existed; the current in-memory hub root and previous active selections are not known.
- `list_courses` describes configured nicknames/current selections but delegates to CLI `courses`, whose output is Canvas's catalog only.

## Implementation scope

1. **Bind the normal product to its installation.** A bundled executable in `engine/` with the installed `context/courses.md` beside that folder uses the installation root regardless of the invoking agent's current directory. A conflicting explicit root must produce a clear mismatch error, rather than redirect a bundled installation into another folder. Preserve explicit `--workspace` for source-development/test binaries and the Cowork extension, whose bundled Engine has no teacher configuration beside it. Keep this as an explicit advanced path, without a normal WebUI selector. Do not restore a remembered teacher folder as an implicit fallback. Root selection should be shared by CLI/MCP/WebUI entry points, with small existing-style helpers rather than a workspace management framework.
2. **Simplify the WebUI.** Remove Browse/path inputs, creation confirmation, workspace-changing routes and synced-folder copy from the normal UI. Settings may show the CE-SE installation location as read-only information; do not label it a separate teacher workspace. Keep Canvas connection, aliases, grading voice, playbook status/replacement and Courses/Students. If a source-development launch has no configured root, explain how to launch with the explicit installation option; do not recreate the old picker. Continue adding missing embedded seed files without overwriting existing teacher files when opening the installed folder, including an MCP-first launch after an Engine update.
3. **Make saved courses discoverable.** Extend the existing `list_courses` tool with optional `source: configured|canvas`, default `configured`. Configured discovery reads the current course file on every call, works without Canvas credentials/network and returns unambiguous local metadata: `source`, `root`, `instance` and a `courses` array with existing nickname/id/current/timezone fields. Include inactive configured courses so `current` is explicit; represent no courses as an empty array. Canvas discovery uses the existing CLI `courses` operation only when `source: canvas` is requested. Keep CLI `courses` as Canvas catalog discovery. Correct descriptions, startup instructions and setup playbook: read saved configuration first, use Canvas discovery to test a connection or choose additional courses, and never repeat completed course setup based on an obsolete starter sentence in a document body. The existing save preserves that body, so inspect structured `courses` data.
4. **Align connection setup.** Update `scripts/connect-mcp.ps1` to accept the CE-SE installation folder and use its `engine/cse.exe` by default. Retain an explicit Engine override for developer tests. Use the same installation root for Work and Cowork registration, preserve unrelated client configuration and existing backup behavior. Update the Cowork manifest's directory field wording to the installed CE-SE folder, not an agent workspace; the extension still needs that location because it contains only an Engine. Do not silently rewrite live client configuration or choose a real installation by guesswork. Clearly report that existing clients registered to the old folder need re-registration/restart.
5. **Update contract and evidence docs.** Align ARCHITECTURE.md, build/MCP docs, seed instructions, integration skill guidance and both capability records. Keep the internal workspace term where it serves code/file contracts, while describing the teacher product as one folder. Preserve the distinction between implementation tests, actual client reads and Canvas Live validation. There are still 26 tools; this change extends an existing tool rather than adding a second course API.

## Preservation and verification

Do not move, delete, merge or overwrite any real teacher installation, student data, drafts or previous configuration. This task changes source and package behavior; unknown previous course selections are not a reason to invent mappings or an automatic migration. Keep credential/private-page exclusions and review/post semantics intact. No Canvas writes are authorized or necessary for this work.

Use focused regressions for bundled root selection from another directory, rejection of a conflicting bundled root, explicit developer/extension roots, and WebUI course-save followed by MCP configured discovery using the same temporary root. That last test should change active status and verify a second MCP read sees it without restart, with no Canvas calls. Verify MCP-first seed addition preserves teacher edits. Replace obsolete folder-picker tests with tests of the resulting behavior; do not keep tests just to preserve the old product model. Reuse existing file/privacy and course-save checks. Run relevant Go tests, then the project's build/test/vet and package checks; update compiled MCP smoke expectations for the new argument/default. Do not test the text of every UI sentence.

Report any remaining client re-registration or unknown previous configuration location separately from source implementation completeness. Do not claim the already-running MCP process has new behavior merely because the source build passes.

## Completion record

Implemented 2026-10-11; source and package work complete. No real teacher installation, live client registration, private page or Canvas write was changed.

- Bundled root selection is shared by CLI, MCP and WebUI; it is independent of the caller directory and rejects a conflicting explicit installation. Development/test and Engine-only extension overrides remain available.
- WebUI folder selection, folder creation/change routes and remembered-root fallback are removed. Settings shows a read-only CE-SE installation; Canvas, courses/students, aliases, grading voice and playbook replacement remain. Hub and MCP launch add missing seed files without overwriting existing teacher edits.
- `list_courses` defaults to fresh local configured selections with source/root/instance metadata, includes inactive entries and returns an empty array for no courses. Explicit `source: canvas` retains existing CLI Canvas catalog discovery. There are still 26 tools. Startup and seed setup instructions use structured courses instead of preserved starter prose.
- Connection setup takes `-Installation`, uses its `engine/cse.exe` by default, retains `-Workspace` as an alias and `-Engine` as an explicit developer override, and preserves client configuration/backup behavior. Cowork extension wording identifies the CE-SE installation.
- Architecture, build/MCP contracts, seed instructions, integration skill and both capability records are aligned. Existing installed AGENTS.md/setup.md remain preserved; the update guidance points to Settings replacement, which archives old copies.

Verification passed:

- Focused Go packages: `go test ./internal/workspace ./internal/localui ./internal/mcpserver ./cmd/cse`.
- Regression coverage: bundled-root selection from another directory and conflicting-root rejection; explicit developer/extension roots; MCP-first seed preservation; a WebUI active-course save observed by the same connected in-memory MCP session without Canvas calls or restart, including inactive entries/timezone and preserved document body.
- Project build/package using existing `.tools/go1.27.1/go/bin/go.exe`: full `go test ./...`, `go vet ./...`, Windows portable build, allowlisted ZIP/private-data checks and Cowork MCPB creation.
- Updated `node scripts/check-mcp.mjs dist/cse.exe`: 26 tools, compiled bundled-root resolution/mismatch rejection, fresh configured discovery, seed preservation, existing scoped file/hash/asset/validation checks. No Canvas calls/writes in configured discovery.
- Synthetic Work/Cowork registration: installation default Engine and matching roots, unrelated configuration retained, existing Claude backup behavior retained. Only temporary fixture configuration and a stub Codex command were used; live registrations were untouched.
- `git diff --check`.

Built outputs: `dist/CanvasExpert-SE.zip` and `dist/CanvasExpert-SE-Cowork.mcpb`. Updated source/build behavior is not a claim that old running processes use it. Existing clients targeting the older folder need re-registration for the intended installation and restart. The old WebUI's current root and the location of prior active-course selections remain unconfirmed; no migration or automatic reconstruction was attempted. Updated real-client use and Canvas Live workflows remain pending human validation.

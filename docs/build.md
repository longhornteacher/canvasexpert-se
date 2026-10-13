# Build and package CanvasExpert-SE

**Source** is this development checkout (code, starter templates, generated `dist/`); **Install** is the extracted local CE-SE folder a teacher actually uses, and the internal `workspace` term refers to Install during teacher work. Develop in Source and do teacher work in Install; optional Sync is explained in [ARCHITECTURE.md](../ARCHITECTURE.md).

## Requirements

The source build targets a portable Windows 11 executable. It needs the exact Go version declared in `go.mod` (currently Go 1.27.1) and PowerShell. The scripts do not install a toolchain, modify `PATH`, or change persistent environment settings. Teachers need neither: the packaged executable runs without Go or any other runtime.

## The one producer

`scripts/package.ps1` is the only script that produces the download. `scripts/build.ps1` is the ordinary entry and simply calls it. `compile.ps1` and `package-mcp.ps1` are internal helpers.

```powershell
.\scripts\build.ps1
.\scripts\build.ps1 -GoExecutable 'C:\tools\go\bin\go.exe'
.\scripts\package.ps1 -GoExecutable '.\.tools\go1.27.1\go\bin\go.exe'
```

Packaging always rebuilds first. Internal `compile.ps1` sets Windows/amd64 and `CGO_ENABLED=0` only for its child Go processes, checks the Go executable's exact version, runs `go test ./...` and `go vet ./...`, then builds the Engine and the thin launcher with `-trimpath` (no local source paths in the binary). Everything is assembled in a temporary staging folder and the current output is replaced only after every step succeeded, so a failed run leaves the previous `dist/` set intact. The scripts do not format source files. Agents that need a scratch binary compile into a task-owned temporary folder and delete it; only this path produces the download.

The Engine embeds the canonical version from root `VERSION` (currently `1.0.0-beta.1`, displayed as 1.0 beta), the short Git revision (`+dirty` when the working tree has changes), a UTC build timestamp, and a development/release build kind. These are diagnostic provenance (`cse version --json`), not a family of builds to choose from.

## Output contract

`dist/` holds these current files (plus any earlier versions' ZIPs you chose to keep):

- `dist/cse.exe`: the Engine used by Source tooling, copied byte-for-byte into the ZIP.
- `dist/cse-mcp-launcher.exe`: the thin MCP launcher.
- `dist/CanvasExpert-SE-<VERSION>.zip` (for example `CanvasExpert-SE-1.0.0-beta.1.zip`): the one teacher download. Rebuilding the same version replaces it; a new version adds a new file and leaves earlier versions in `dist/` as an archive until you delete them.
- `dist/SHA256SUMS.txt`: checksums.

No dated duplicates are written; the version in the ZIP name is the only variation. The Cowork extension (MCPB) is generated in staging and exists only inside the ZIP at `engine/CanvasExpert-SE-Cowork.mcpb`; it is not a separate download.

`-OutputDirectory` selects a different output folder. `-Release` requires a clean checkout at the matching existing immutable version tag and additionally writes the versioned public assets to `dist/release/`; see [releasing.md](releasing.md).

## Package contents

The ZIP is the single-folder package colleagues get. It has no top-level folder, so Windows' Extract All produces a `CanvasExpert-SE` folder that is the Install. It holds:

- the allowlisted `workspace-template` files, including installation instructions, the teaching profile/starter, `cse.cmd` and the hidden `.claude/settings.json` rule;
- the Engine at `engine/cse.exe` and the ready Cowork extension at `engine/CanvasExpert-SE-Cowork.mcpb`;
- `webui.cmd` (from `scripts/package-webui.cmd`) at the root;
- product `VERSION` and MIT `LICENSE` at the root, with third-party notices under `engine/` (distribution metadata outside the teacher seed).

Privacy and allowlist rules: example files are accepted only when their content explicitly labels them synthetic; the starter settings files must retain their empty-template markers; private `.cse` state, pulled course data, scoring files, build caches and compiler files are never package inputs. Packages ship no Sync binding or teacher data. Course configuration lives at `context/courses.md`, the same path the Engine uses to recognize an Install.

ZIP entries are written with forward slashes by the script itself, because Windows PowerShell 5.1's `CreateFromDirectory` writes backslashes.

A successful build or ZIP inspection verifies the local build and package, not every possible Canvas response or district configuration.

## WebUI launcher

`webui.cmd` (also `.\scripts\webui.ps1` or `cse ui`) starts the local WebUI. The development launcher rebuilds through the same producer first when any Go source, `go.mod`/`go.sum`, `VERSION`, the workspace template, `engine-docs` or a WebUI template/asset (HTML, CSS, JS) is newer than `dist/cse.exe`, using `.tools/go1.27.1` if present. A source-development launch without a root explains `cse --workspace PATH ui`; the PowerShell launcher also accepts `-Workspace PATH` or `CSE_WORKSPACE`.

## Where else to look

- [releasing.md](releasing.md): release assets and tagging.
- [development-notes.md](development-notes.md): shell quirks, live-read rules, test fixtures, quiz prior art.
- [mcp-adapter.md](mcp-adapter.md): MCP adapter tools, client registration and the install journey.
- [../internal/localui/README.md](../internal/localui/README.md): how WebUI pages are declared and secured.
- `internal/localui/capabilities.go` and [engine-capabilities.md](engine-capabilities.md): the Capabilities page status data; keep them aligned.
- [../ARCHITECTURE.md](../ARCHITECTURE.md): Sync and shared teacher rules.

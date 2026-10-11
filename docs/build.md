# Build and package CanvasExpert-SE

The source build targets a portable Windows 11 executable. Building requires the exact Go version declared in `go.mod` (currently Go 1.27.1) and PowerShell. The scripts do not install a toolchain, modify `PATH`, or change persistent environment settings. `build.ps1` sets Windows/amd64 and `CGO_ENABLED=0` only for its child Go processes, checks the supplied Go executable's exact version, then runs `go test ./...`, `go vet ./...`, and builds `dist/cse.exe`.

From the repository root:

```powershell
.\scripts\build.ps1
.\scripts\build.ps1 -GoExecutable 'C:\tools\go\bin\go.exe'
.\scripts\build.ps1 -GoExecutable '.\.tools\go1.27.1\go\bin\go.exe'
```

The executable embeds version `0.1b`, the current short Git revision with `+dirty` when the working tree has changes, and a UTC build timestamp. `-trimpath` removes local source paths from the binary. The scripts do not format source files automatically.

To create the distributable starter workspace:

```powershell
.\scripts\package.ps1
.\scripts\package.ps1 -GoExecutable '.\.tools\go1.27.1\go\bin\go.exe'
```

Packaging rebuilds first and writes `dist/CanvasExpert-SE.zip`, printing its SHA-256 hash. It is the single-folder package colleagues get. It holds the allowlisted `workspace-template` files (including the `cse.cmd` launcher and the hidden `.claude/settings.json` rule), the Engine at `engine/cse.exe`, and `webui.cmd` (from `scripts/package-webui.cmd`) at its root. It has no top-level folder, so Windows' Extract All produces a `CanvasExpert-SE` folder that is the workspace. Example files are accepted only when their content explicitly labels them synthetic. The starter settings files must retain their empty-template markers; private `.cse` state, pulled course data, scoring files, build caches, and compiler files are not package inputs. Course configuration lives at `context/courses.md`, the same path the Engine uses to recognize a workspace.

ZIP entries are written with forward slashes by the script itself, because Windows PowerShell 5.1's `CreateFromDirectory` writes backslashes.

Colleagues unzip `CanvasExpert-SE.zip` on a local drive and connect the agent through MCP or open that folder in a local agent. The installation's `cse.cmd` runs `engine\cse.exe`; its bundled Engine selects the installation beside `engine/` independently of the invoking directory. A conflicting `--workspace` fails clearly. The teacher never selects a second CE-SE folder in the hub. Developer test roots and Engine-only Cowork extensions retain advanced explicit roots; the launcher's linked-Engine fallback supports those development roots. The agent's own synced folder is independent.

The packaged executable is intended to run without a Go or other language runtime installed on the teacher's computer. A successful source build or ZIP inspection verifies the local build and package, not every possible Canvas response or district configuration. Use current API responses and the task guides; see [0.1b-evidence.md](0.1b-evidence.md) for historical observations, not operation prerequisites. The bundled Engine resolves its installation automatically, for example `.\engine\cse.exe --help` from the package root. Explicit `--workspace PATH` remains an advanced option for development/test binaries and Engine-only extensions; it must match the installation when using a bundled Engine.

## Local MCP adapter

The packaged `engine\cse.exe` also supports `mcp`, a stdio server with 26 typed tools for all currently supported Engine commands, scoped workspace text/assets, review flow, and teacher-only local pages. It preserves the playbooks' review default and direct-write/resume exceptions without an extra approval token. It does not expose arbitrary commands, arbitrary HTTP writes, or private `.cse` files. Read-only API JSON can contain real names.

Register desktop Cowork and ChatGPT Work from the repository with `scripts/connect-mcp.ps1 -Installation <CE-SE-installation> -Client Both`; it uses that folder's `engine/cse.exe` by default. For a source-development binary, explicitly supply `-Engine dist/cse.exe`, then restart both clients. Registration uses `codex mcp add` and preserves Claude Desktop's other configuration. Build the optional Claude desktop extension with `scripts/package-mcp.ps1`; its `.mcpb` contains the manifest and portable Engine without teacher data. Hosted ChatGPT is a separate remote-plugin/tunnel route; desktop Work can use direct stdio. See [MCP adapter setup](mcp-adapter.md) for commands, boundaries, and official connection documentation. Cowork initialization/discovery and read-only calls through Work's shared local runtime were verified on 2026-10-11. Full desktop task workflows and tunnel compatibility remain pending human validation; local tests do not establish Canvas Live proof.

## Local web pages (the hub)

Double-click `webui.cmd` in the repository root (or run `webui`, `.\scripts\webui.ps1`, or `cse ui`). One loopback server hosts Home (course/student status and Quit), Canvas setup, Courses (select active courses, saved to `context/courses.md` as `current: true`), Students (one tab per active course, aliases, monitoring, extra time, and Canvas differentiation tags), and Settings (Canvas connection, program information, read-only CE-SE installation location, aliases, feedback voice and playbook status/replacement). Students pulls each selected course's active roster and read-only differentiation tags once per hub session; **Refresh from Canvas** updates that course again. Course memberships and tags are cached privately in `.cse/rosters.json` for offline use. Extra time is saved in `.cse/students.csv` for later use and does not yet alter Canvas deadlines or grading. Existing student tables are upgraded when saved, preserving identities and aliases.

The hub stays open until you press Quit or Ctrl+C. It tries port 47831 first, so a bookmark works, and falls back to a free port. There is no folder picker, creation flow or remembered teacher-folder fallback. A source-development launch without a root explains `cse --workspace PATH ui`; the development PowerShell launcher also accepts `-Workspace PATH` or `CSE_WORKSPACE`. Hub and MCP startup add missing embedded seed files without overwriting teacher edits. `list_courses` defaults to saved selections, reading the file anew on each call so hub active-course changes appear immediately; explicit `source: canvas` retains live catalog discovery. Existing clients registered to old folders need re-registration/restart; no teacher data is moved or merged automatically. The launcher rebuilds `dist/cse.exe` first when sources are newer, using `.tools/go1.27.1` if present. `cse setup` and `cse students` remain as one-shot commands.

The **Capabilities** page (`/capabilities`) shows the recorded green/yellow/red Canvas Live feature map, evidence limits, and review date. It is linked from Home and the shared navigation and works without a configured workspace or Canvas connection. Keep its status data in `internal/localui/capabilities.go` aligned with [engine-capabilities.md](engine-capabilities.md).

New pages go in `internal/localui` and are routed from `Hub.ServeHTTP`; every form must pass the hub's Host/Origin/secret checks.

Working notes for developing in this checkout (shell quirks, live-read rules, test fixtures, quiz prior art) are in [development-notes.md](development-notes.md).

Existing installations retain their current `AGENTS.md` and `playbooks/setup.md`, even after an Engine update. To adopt the revised course-discovery instructions, use Settings -> Your playbooks and context -> Get the new version for those files; their previous copies are archived under `playbook-old/`. Review any teacher customizations and carry them forward as needed. No existing instruction or context file is automatically overwritten.

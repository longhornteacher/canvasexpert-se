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

Packaging rebuilds first and writes two ZIPs to `dist/`, each with its SHA-256 hash printed:

- `CanvasExpert-SE.zip`: the single-folder package colleagues get. It holds the allowlisted `workspace-template` files (including the `cse.cmd` launcher and the hidden `.claude/settings.json` rule), the Engine at `engine/cse.exe`, and `webui.cmd` (from `program-template/workspace-webui.cmd`) at its root. It has no top-level folder, so Windows' Extract All produces a `CanvasExpert-SE` folder that is the workspace. Example files are accepted only when their content explicitly labels them synthetic. The starter settings files must retain their empty-template markers; private `.cse` state, pulled course data, scoring files, build caches, and compiler files are not package inputs. Course configuration lives at `context/courses.md`, the same path the Engine uses to recognize a workspace.
- `CanvasExpert-SE-Program-0.1b-windows-amd64.zip`: `cse.exe`, `webui.cmd` and `README.txt` (from `program-template/`), for a synced workspace that keeps the program in a separate per-machine folder.

ZIP entries are written with forward slashes by the script itself, because Windows PowerShell 5.1's `CreateFromDirectory` writes backslashes.

Colleagues unzip `CanvasExpert-SE.zip`, point their agent at the folder and talk; the agent runs setup (`playbooks/setup.md`). The workspace's `cse.cmd` runs `engine\cse.exe` when present. Otherwise it uses the program recorded by `cse link` under `%LOCALAPPDATA%\CanvasExpert-SE` (the separate program folder's `webui.cmd` runs `cse link`). The hub refuses a workspace that overlaps the program folder, except when the program is that workspace's own `engine` folder.

The packaged executable is intended to run without a Go or other language runtime installed on the teacher's computer. A successful source build or ZIP inspection verifies the local build and package, not every possible Canvas response or district configuration. Use current API responses and the task guides; see [0.1b-evidence.md](0.1b-evidence.md) for historical observations, not operation prerequisites. Workspace commands use the explicit workspace option, for example `.\engine\cse.exe --workspace . --help` when run from the packaged workspace root.

## Local web pages (the hub)

Double-click `webui.cmd` in the repository root (or run `webui`, `.\scripts\webui.ps1`, or `cse ui`). One loopback server hosts Home (course/student status and Quit), Canvas setup, Courses (select active courses, saved to `context/courses.md` as `current: true`), Students (one tab per active course, aliases, monitoring, extra time, and Canvas differentiation tags), and Settings (Canvas connection, program information, workspace chooser, and the alias preference). Students pulls each selected course's active roster and read-only differentiation tags once per hub session; **Refresh from Canvas** updates that course again. Course memberships and tags are cached privately in `.cse/rosters.json` for offline use. Extra time is saved in `.cse/students.csv` for later use and does not yet alter Canvas deadlines or grading. Existing student tables are upgraded when saved, preserving identities and aliases.

The hub stays open until you press Quit or Ctrl+C. It tries port 47831 first, so a bookmark works, and falls back to a free port. In Settings, choose the workspace with Browse (a native folder window) or by pasting a full path. If the folder is empty or new, the hub creates the starter workspace in it from the template embedded in `cse.exe`; a non-empty folder asks first, and existing files are never overwritten. You can also pass `-Workspace PATH` or set `CSE_WORKSPACE`; otherwise it restores the last one it used (a non-secret path under `%APPDATA%\CanvasExpert-SE`). The launcher rebuilds `dist/cse.exe` first when sources are newer, using `.tools/go1.27.1` if present. `cse setup` and `cse students` remain as one-shot commands.

The **Capabilities** page (`/capabilities`) shows the recorded green/yellow/red Canvas Live feature map, evidence limits, and review date. It is linked from Home and the shared navigation and works without a configured workspace or Canvas connection. Keep its status data in `internal/localui/capabilities.go` aligned with [engine-capabilities.md](engine-capabilities.md).

New pages go in `internal/localui` and are routed from `Hub.ServeHTTP`; every form must pass the hub's Host/Origin/secret checks.

Working notes for developing in this checkout (shell quirks, live-read rules, test fixtures, quiz prior art) are in [development-notes.md](development-notes.md).

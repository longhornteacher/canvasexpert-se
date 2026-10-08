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

- `CanvasExpert-SE-Program-0.1b-windows-amd64.zip`: `cse.exe`, `webui.cmd` and `README.txt` (from `program-template/`). It is unzipped per machine and is not synced.
- `CanvasExpert-SE-Workspace-0.1b.zip`: the allowlisted `workspace-template` files, including the `cse.cmd` launcher and the hidden `.claude/settings.json` rule. It contains no program, so it is safe to place in a synced folder. Example files are accepted only when their content explicitly labels them synthetic. The starter settings files must retain their empty-template markers; private `.cse` state, pulled course data, scoring files, build caches, and compiler files are not package inputs. Course configuration lives at `settings/courses.md`, the same path the CLI uses to recognize a workspace.

ZIP entries are written with forward slashes by the script itself, because Windows PowerShell 5.1's `CreateFromDirectory` writes backslashes.

On each computer: unzip the program somewhere outside the synced workspace, double-click `webui.cmd` once (it runs `cse link`, recording the program's location under `%LOCALAPPDATA%\CanvasExpert-SE`), then use the hub or `.\cse.cmd COMMAND` from the workspace. The hub refuses a workspace that overlaps the program folder.

The packaged executable is intended to run without a Go or other language runtime installed on the teacher's computer. A successful source build or ZIP inspection does not establish that the Canvas workflow passed live district checks. See `docs/0.1b-evidence.md` for current evidence and limitations. Workspace commands use the explicit workspace option, for example `.\cse\cse.exe --workspace . --help` when run from the packaged workspace root.

## Local web pages (the hub)

Double-click `webui.cmd` in the repository root (or run `webui`, `.\scripts\webui.ps1`, or `cse ui`). One loopback server hosts every local page: Home (status, workspace chooser, Quit), Canvas setup, and the student editor. It stays open until you press Quit or Ctrl+C. It tries port 47831 first, so a bookmark works, and falls back to a free port. Choose the workspace with Browse (a native folder window) or by pasting a full path. If the folder is empty or new, the hub creates the starter workspace in it from the template embedded in `cse.exe`; a non-empty folder asks first, and existing files are never overwritten. You can also pass `-Workspace PATH` or set `CSE_WORKSPACE`; otherwise it restores the last one it used (a non-secret path under `%APPDATA%\CanvasExpert-SE`). The launcher rebuilds `dist/cse.exe` first when sources are newer, using `.tools/go1.27.1` if present. `cse setup` and `cse students` remain as one-shot commands.

New pages go in `internal/localui` and are routed from `Hub.ServeHTTP`; every form must pass the hub's Host/Origin/secret checks.

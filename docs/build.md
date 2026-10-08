# Build and package CanvasSidekick

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

Packaging rebuilds first, stages a unique workspace from the allowlisted `workspace-template` files plus `cse/cse.exe`, writes `dist/CanvasSidekick-0.1b-windows-amd64.zip`, verifies archive contents, and prints its SHA-256 hash. It includes the hidden `.claude/settings.json` rule. Example files are accepted only when their content explicitly labels them synthetic. The starter settings files must retain their empty-template markers; private `.cse` state, pulled course data, scoring files, build caches, and compiler files are not package inputs. Course configuration lives at `settings/courses.md`, the same path the CLI uses to recognize a workspace.

The packaged executable is intended to run without a Go or other language runtime installed on the teacher's computer. A successful source build or ZIP inspection does not establish that the Canvas workflow passed live district checks. See `docs/0.1b-evidence.md` for current evidence and limitations. Workspace commands use the explicit workspace option, for example `.\cse\cse.exe --workspace . --help` when run from the packaged workspace root.

## Local web pages

From the repository root, `webui` (or `.\scripts\webui.ps1`) opens the Canvas connection page; `webui students -Workspace PATH` opens the student editor (`CSE_WORKSPACE` can supply the path). It rebuilds `dist/cse.exe` first when sources are newer, using `.tools/go1.27.1` if present. Ctrl+C closes the page.

# Ordinary Dev build. There is one producer: package.ps1 compiles once, runs the checks and
# assembles the current dist/ output (cse.exe, cse-mcp-launcher.exe, CanvasExpert-SE-<VERSION>.zip, SHA256SUMS.txt).
# Unless -GoExecutable is given, the exact Go version go.mod requires is found or installed
# single-user into .tools/ (see resolve-go.ps1).
[CmdletBinding()]
param(
    [Parameter()]
    [string]$GoExecutable,
    [Parameter()][switch]$Release
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'resolve-go.ps1')
if (-not $GoExecutable) { $GoExecutable = Resolve-Go -RepoRoot ([IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))) }
& (Join-Path $PSScriptRoot 'package.ps1') -GoExecutable $GoExecutable -Release:$Release
if (-not $?) { throw 'Build failed.' }

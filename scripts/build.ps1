# Ordinary Source build. There is one producer: package.ps1 compiles once, runs the checks and
# assembles the current dist/ output (cse.exe, cse-mcp-launcher.exe, CanvasExpert-SE-<VERSION>.zip, SHA256SUMS.txt).
[CmdletBinding()]
param(
    [Parameter()]
    [string]$GoExecutable = 'go',
    [Parameter()][switch]$Release
)

$ErrorActionPreference = 'Stop'
& (Join-Path $PSScriptRoot 'package.ps1') -GoExecutable $GoExecutable -Release:$Release
if (-not $?) { throw 'Build failed.' }

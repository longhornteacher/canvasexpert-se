[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [ValidateSet('ui', 'setup', 'students')]
    [string]$Page,
    [string]$Workspace = $env:CSE_WORKSPACE,
    [string]$GoExecutable
)

$ErrorActionPreference = 'Stop'
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$exe = Join-Path $repoRoot 'dist/cse.exe'

if (-not $Page) { $Page = 'ui' }
if ($Page -eq 'students' -and -not $Workspace) {
    throw 'The students page needs a workspace: pass -Workspace PATH or set CSE_WORKSPACE.'
}

# Go sources, go.mod, and the workspace template (it is embedded into cse.exe).
$newestSource = Get-ChildItem -LiteralPath $repoRoot -Recurse -File -Force |
    Where-Object { $_.FullName -notmatch '[\\/](\.tools|\.cache|dist|\.git)[\\/]' -and ($_.Extension -eq '.go' -or $_.Name -eq 'go.mod' -or $_.FullName -match '[\\/]workspace-template[\\/]') } |
    Sort-Object LastWriteTimeUtc -Descending | Select-Object -First 1
$stale = -not (Test-Path -LiteralPath $exe) -or ($newestSource -and $newestSource.LastWriteTimeUtc -gt (Get-Item -LiteralPath $exe).LastWriteTimeUtc)
if ($stale) {
    if (-not $GoExecutable) {
        $local = Join-Path $repoRoot '.tools/go1.27.1/go/bin/go.exe'
        $GoExecutable = if (Test-Path -LiteralPath $local) { $local } else { 'go' }
    }
    Write-Host 'dist/cse.exe is missing or stale; building first...'
    & (Join-Path $PSScriptRoot 'build.ps1') -GoExecutable $GoExecutable
    if (-not $?) { throw 'Build failed.' }
}

if ($Workspace) { & $exe --workspace $Workspace $Page } else { & $exe $Page }
exit $LASTEXITCODE

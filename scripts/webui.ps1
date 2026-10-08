[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [ValidateSet('setup', 'students')]
    [string]$Page,
    [string]$Workspace = $env:CSE_WORKSPACE,
    [string]$GoExecutable
)

$ErrorActionPreference = 'Stop'
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$exe = Join-Path $repoRoot 'dist/cse.exe'

if (-not $Page) { $Page = if ($Workspace) { 'students' } else { 'setup' } }
if ($Page -eq 'students' -and -not $Workspace) {
    throw 'The students page needs a workspace: pass -Workspace PATH or set CSE_WORKSPACE.'
}

$newestSource = Get-ChildItem -LiteralPath $repoRoot -Recurse -File -Include '*.go', 'go.mod' |
    Where-Object { $_.FullName -notmatch '[\/](\.tools|\.cache|dist)[\/]' } |
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

if ($Page -eq 'students') { & $exe --workspace $Workspace students } else { & $exe setup }
exit $LASTEXITCODE

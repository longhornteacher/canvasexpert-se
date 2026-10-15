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

. (Join-Path $PSScriptRoot 'resolve-go.ps1')

if (-not $Page) { $Page = 'ui' }
if ($Page -eq 'students' -and -not $Workspace) {
    throw 'The students page needs a workspace: pass -Workspace PATH or set CSE_WORKSPACE.'
}

# Everything compiled or embedded into cse.exe: Go sources, go.mod/go.sum, VERSION, the workspace
# template, engine-docs and the WebUI templates/assets (HTML, CSS, JS).
$embeddedInputs = '[\\/](workspace-template|engine-docs)[\\/]|[\\/]internal[\\/]localui[\\/](assets|templates)[\\/]'
$newestSource = Get-ChildItem -LiteralPath $repoRoot -Recurse -File -Force |
    Where-Object { $_.FullName -notmatch '[\\/](\.tools|\.cache|dist|\.git|\.test-workspaces)[\\/]' -and ($_.Extension -eq '.go' -or $_.Name -in 'go.mod', 'go.sum', 'VERSION' -or $_.FullName -match $embeddedInputs) } |
    Sort-Object LastWriteTimeUtc -Descending | Select-Object -First 1
$stale = -not (Test-Path -LiteralPath $exe) -or ($newestSource -and $newestSource.LastWriteTimeUtc -gt (Get-Item -LiteralPath $exe).LastWriteTimeUtc)
if ($stale) {
    if (-not $GoExecutable) { $GoExecutable = Resolve-Go -RepoRoot $repoRoot }
    Write-Host 'dist/cse.exe is missing or stale; building first...'
    & (Join-Path $PSScriptRoot 'build.ps1') -GoExecutable $GoExecutable   # the one producer; also refreshes the current ZIP
    if (-not $?) { throw 'Build failed.' }
}

if ($Workspace) { & $exe --workspace $Workspace $Page } else { & $exe $Page }
exit $LASTEXITCODE

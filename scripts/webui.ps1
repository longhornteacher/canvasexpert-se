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

# Finds the exact Go version go.mod requires, or installs it single-user into
# .tools/ (no admin rights, nothing system-wide) by downloading the official zip
# from go.dev and verifying its SHA-256 against go.dev's published checksum.
function Resolve-Go {
    param([string]$RepoRoot)

    $goMod = [IO.File]::ReadAllText((Join-Path $RepoRoot 'go.mod'))
    if ($goMod -notmatch '(?m)^\s*go\s+(\d+\.\d+\.\d+)\s*$') {
        throw 'go.mod must declare an exact Go version such as "go 1.27.1".'
    }
    $version = "go$($Matches[1])"
    $localGo = Join-Path $RepoRoot ".tools/$version/go/bin/go.exe"
    if (Test-Path -LiteralPath $localGo) { return $localGo }

    $onPath = Get-Command go -ErrorAction SilentlyContinue
    if ($onPath) {
        # GOTOOLCHAIN=local matches build.ps1: report the binary itself, never an auto-switched toolchain.
        $previousToolchain = $env:GOTOOLCHAIN
        $env:GOTOOLCHAIN = 'local'
        try { $reported = ((& $onPath.Source env GOVERSION) -join '').Trim() }
        finally { $env:GOTOOLCHAIN = $previousToolchain }
        if ($reported -ceq $version) { return $onPath.Source }
        Write-Host "Go on PATH is '$reported' but this project needs $version."
    }

    Write-Host "Installing $version for this user only (first launch; one-time download of about 80 MB)..."
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 -bor [Net.SecurityProtocolType]::Tls13
    $fileName = "$version.windows-amd64.zip"
    $releases = Invoke-RestMethod -Uri 'https://go.dev/dl/?mode=json&include=all'
    $entry = $releases | Where-Object { $_.version -ceq $version } | Select-Object -First 1
    $file = if ($entry) { $entry.files | Where-Object { $_.filename -ceq $fileName } | Select-Object -First 1 }
    if (-not $file) { throw "go.dev does not list $fileName. Install Go $version manually from https://go.dev/dl/" }

    $toolsDir = Join-Path $RepoRoot '.tools'
    $installDir = Join-Path $toolsDir $version
    New-Item -ItemType Directory -Path $toolsDir -Force | Out-Null
    $zip = Join-Path $toolsDir $fileName
    $staging = Join-Path $toolsDir "$version.partial"
    try {
        Invoke-WebRequest -Uri "https://go.dev/dl/$fileName" -OutFile $zip -UseBasicParsing
        $actual = (Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash
        if ($actual -ine $file.sha256) { throw "Checksum mismatch for $fileName; refusing to install." }
        if (Test-Path -LiteralPath $staging) { Remove-Item -LiteralPath $staging -Recurse -Force }
        Expand-Archive -LiteralPath $zip -DestinationPath $staging -Force
        if (Test-Path -LiteralPath $installDir) { Remove-Item -LiteralPath $installDir -Recurse -Force }
        Move-Item -LiteralPath $staging -Destination $installDir
    }
    finally {
        if (Test-Path -LiteralPath $zip) { Remove-Item -LiteralPath $zip -Force }
        if (Test-Path -LiteralPath $staging) { Remove-Item -LiteralPath $staging -Recurse -Force }
    }
    if (-not (Test-Path -LiteralPath $localGo)) { throw "Go install finished but $localGo is missing." }
    Write-Host "Installed $version to $installDir"
    return $localGo
}

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
    if (-not $GoExecutable) { $GoExecutable = Resolve-Go -RepoRoot $repoRoot }
    Write-Host 'dist/cse.exe is missing or stale; building first...'
    & (Join-Path $PSScriptRoot 'build.ps1') -GoExecutable $GoExecutable
    if (-not $?) { throw 'Build failed.' }
}

if ($Workspace) { & $exe --workspace $Workspace $Page } else { & $exe $Page }
exit $LASTEXITCODE

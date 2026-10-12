[CmdletBinding()]
param(
    [Parameter()]
    [string]$GoExecutable = 'go',
    [Parameter()][switch]$Release
)

$ErrorActionPreference = 'Stop'

$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$goModPath = Join-Path $repoRoot 'go.mod'
$commandRoot = Join-Path $repoRoot 'cmd/cse'
$outputDirectory = Join-Path $repoRoot 'dist'
$outputPath = Join-Path $outputDirectory 'cse.exe'
$productVersion = [IO.File]::ReadAllText((Join-Path $repoRoot 'VERSION')).Trim()
if ($productVersion -notmatch '^\d+\.\d+\.\d+(?:-beta\.[1-9]\d*)?$') { throw 'VERSION must be MAJOR.MINOR.PATCH or MAJOR.MINOR.PATCH-beta.N.' }
$buildKind = 'development'
if ($Release) { $buildKind = 'release' }

if (-not (Test-Path -LiteralPath $goModPath -PathType Leaf)) {
    throw "Missing module file: $goModPath"
}
$goModText = [IO.File]::ReadAllText($goModPath)
if ($goModText -notmatch '(?m)^\s*go\s+(\d+\.\d+\.\d+)\s*$') {
    throw 'go.mod must declare an exact Go version such as "go 1.27.1".'
}
$requiredGoVersion = $Matches[1]

if (-not (Test-Path -LiteralPath $commandRoot -PathType Container)) {
    throw "Missing CLI source directory: $commandRoot"
}

New-Item -ItemType Directory -Path $outputDirectory -Force | Out-Null

$temporaryEnvironment = @{
    GOTOOLCHAIN = 'local'
    GOOS = 'windows'
    GOARCH = 'amd64'
    CGO_ENABLED = '0'
    GOCACHE = (Join-Path $repoRoot '.cache/go-build')
    GOMODCACHE = (Join-Path $repoRoot '.cache/go-mod')
}
$previousEnvironment = @{}
foreach ($name in $temporaryEnvironment.Keys) {
    $previousEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
    [Environment]::SetEnvironmentVariable($name, $temporaryEnvironment[$name], 'Process')
}

function Invoke-Go {
    param([Parameter(Mandatory)][string[]]$GoArgs)

    Write-Host ("go " + ($GoArgs -join ' '))
    & $GoExecutable @GoArgs
    $goExitCode = $LASTEXITCODE
    if ($goExitCode -ne 0) {
        throw "Go command failed with exit code ${goExitCode}: go $($GoArgs -join ' ')"
    }
}

try {
    $reportedGoVersion = & $GoExecutable env GOVERSION
    $versionExitCode = $LASTEXITCODE
    if ($versionExitCode -ne 0) {
        throw "Unable to query the Go toolchain using '$GoExecutable' (exit $versionExitCode)."
    }
    $reportedGoVersion = (@($reportedGoVersion) -join '').Trim()
    $expectedGoVersion = "go$requiredGoVersion"
    if ($reportedGoVersion -cne $expectedGoVersion) {
        throw "This repository requires $expectedGoVersion, but '$GoExecutable' reports '$reportedGoVersion'. No toolchain was installed."
    }

    $revisionLines = & git -C $repoRoot rev-parse --short=12 HEAD
    $revisionExitCode = $LASTEXITCODE
    if ($revisionExitCode -ne 0) {
        throw 'Unable to read the Git revision for build metadata.'
    }
    $revision = (@($revisionLines) -join '').Trim()
    if ([string]::IsNullOrWhiteSpace($revision)) {
        throw 'Git returned an empty revision for build metadata.'
    }
    $statusLines = & git -C $repoRoot status --porcelain
    $statusExitCode = $LASTEXITCODE
    if ($statusExitCode -ne 0) {
        throw 'Unable to determine whether the working tree is dirty.'
    }
    if (@($statusLines).Count -gt 0) {
        if ($Release) { throw 'Release builds require a clean checkout. Commit reviewed changes and use its immutable version tag before building a release.' }
        $revision += '+dirty'
    }
    if ($Release) {
        $tagCommit = & git -C $repoRoot rev-parse "refs/tags/v$productVersion^{commit}"
        if ($LASTEXITCODE -ne 0) { throw "Release requires the existing tag v$productVersion. This script never creates tags." }
        $headCommit = & git -C $repoRoot rev-parse HEAD
        if ($LASTEXITCODE -ne 0 -or (@($tagCommit) -join '').Trim() -cne (@($headCommit) -join '').Trim()) { throw "Release tag v$productVersion must point at HEAD." }
    }
    $builtAt = [DateTimeOffset]::UtcNow.ToString("yyyy-MM-dd'T'HH:mm:ss'Z'", [Globalization.CultureInfo]::InvariantCulture)

    Invoke-Go -GoArgs @('test', './...')
    Invoke-Go -GoArgs @('vet', './...')
    Invoke-Go -GoArgs @('build', '-trimpath', '-ldflags', "-s -w -X main.revision=$revision -X main.builtAt=$builtAt -X main.buildKind=$buildKind -X canvasexpertse/internal/mcpserver.Revision=$revision -X canvasexpertse/internal/mcpserver.BuiltAt=$builtAt -X canvasexpertse/internal/mcpserver.BuildKind=$buildKind", '-o', $outputPath, './cmd/cse')

    Invoke-Go -GoArgs @('build', '-trimpath', '-ldflags', '-s -w', '-o', (Join-Path $outputDirectory 'cse-mcp-launcher.exe'), './cmd/cse-mcp-launcher')
    if (-not (Test-Path -LiteralPath $outputPath -PathType Leaf)) {
        throw "Go reported success but did not create $outputPath"
    }
    Write-Host "Built $outputPath ($expectedGoVersion, revision $revision, $builtAt)."
}
finally {
    foreach ($name in $temporaryEnvironment.Keys) {
        [Environment]::SetEnvironmentVariable($name, $previousEnvironment[$name], 'Process')
    }
}

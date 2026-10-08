[CmdletBinding()]
param(
    [Parameter()]
    [string]$GoExecutable = 'go'
)

$ErrorActionPreference = 'Stop'

$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$templateRoot = [IO.Path]::GetFullPath((Join-Path $repoRoot 'workspace-template'))
$binaryPath = [IO.Path]::GetFullPath((Join-Path $repoRoot 'dist/cse.exe'))
$distRoot = [IO.Path]::GetFullPath((Join-Path $repoRoot 'dist'))
$packagePath = [IO.Path]::GetFullPath((Join-Path $distRoot 'CanvasSidekick-0.1b-windows-amd64.zip'))
$pathSeparator = [IO.Path]::DirectorySeparatorChar

if (-not (Test-Path -LiteralPath $templateRoot -PathType Container)) {
    throw "Missing starter template: $templateRoot"
}

# Always rebuild so the archive contains the executable produced from this source tree.
& (Join-Path $PSScriptRoot 'build.ps1') -GoExecutable $GoExecutable
if (-not (Test-Path -LiteralPath $binaryPath -PathType Leaf)) {
    throw "Missing built executable: $binaryPath"
}

function Test-TemplatePathAllowed {
    param([Parameter(Mandatory)][string]$RelativePath)

    $normalizedPath = $RelativePath.Replace('\', '/')
    return $normalizedPath -match '^(AGENTS|CLAUDE|README)\.md$' -or
        $normalizedPath -match '^\.claude/settings\.json$' -or
        $normalizedPath -match '^guides/[A-Za-z0-9._-]+\.md$' -or
        $normalizedPath -match '^formats/[A-Za-z0-9._-]+\.md$' -or
        $normalizedPath -match '^settings/(courses|grading|students)\.md$' -or
        $normalizedPath -match '^examples/[A-Za-z0-9._/-]+\.(md|txt|yaml|yml|json|csv|html)$'
}

$studentNotesPath = Join-Path $templateRoot 'settings/students.md'
if (-not (Test-Path -LiteralPath $studentNotesPath -PathType Leaf)) {
    throw 'The clean template must include settings/students.md.'
}
$studentNotes = [IO.File]::ReadAllText($studentNotesPath)
if ($studentNotes -notmatch '(?i)starter file: no students or real student information are configured') {
    throw 'Refusing to package settings/students.md: it is not marked as the empty starter file.'
}

$gradingSettingsPath = Join-Path $templateRoot 'settings/grading.md'
if (-not (Test-Path -LiteralPath $gradingSettingsPath -PathType Leaf)) {
    throw 'The clean template must include settings/grading.md.'
}
$gradingSettings = [IO.File]::ReadAllText($gradingSettingsPath)
if ($gradingSettings -notmatch '(?i)starter file: no teacher-specific preferences are configured') {
    throw 'Refusing to package settings/grading.md: it is not marked as the empty starter file.'
}

$courseSettingsPath = Join-Path $templateRoot 'settings/courses.md'
if (-not (Test-Path -LiteralPath $courseSettingsPath -PathType Leaf)) {
    throw 'The clean template must include settings/courses.md.'
}
$courseSettings = [IO.File]::ReadAllText($courseSettingsPath)
if ($courseSettings -notmatch '(?i)starter file: no courses configured') {
    throw 'Refusing to package settings/courses.md without the empty starter marker.'
}

$claudeSettingsPath = Join-Path $templateRoot '.claude/settings.json'
if (-not (Test-Path -LiteralPath $claudeSettingsPath -PathType Leaf)) {
    throw 'The starter template must include .claude/settings.json.'
}
$claudeSettings = Get-Content -LiteralPath $claudeSettingsPath -Raw | ConvertFrom-Json
if ($claudeSettings.permissions.deny -notcontains 'Read(./.cse/**)') {
    throw 'The starter Claude settings must deny Read access to .cse/**.'
}

$tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$stageName = "CanvasSidekick-package-$([Guid]::NewGuid().ToString('N'))"
$stageRoot = [IO.Path]::GetFullPath((Join-Path $tempRoot $stageName))
$stagePrefix = $tempRoot.TrimEnd('\', '/') + $pathSeparator
if (-not $stageRoot.StartsWith($stagePrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw "Staging path is outside the system temp directory: $stageRoot"
}

$zipTempPath = Join-Path $distRoot (".CanvasSidekick-package-$([Guid]::NewGuid().ToString('N')).zip")
$zipTempFullPath = [IO.Path]::GetFullPath($zipTempPath)
$distPrefix = $distRoot.TrimEnd('\', '/') + $pathSeparator
if (-not $zipTempFullPath.StartsWith($distPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw "Temporary archive path is outside dist: $zipTempFullPath"
}

try {
    New-Item -ItemType Directory -Path $stageRoot | Out-Null
    $expectedEntries = [Collections.Generic.List[string]]::new()
    $sourcePrefix = $templateRoot.TrimEnd('\', '/') + $pathSeparator

    foreach ($entry in Get-ChildItem -LiteralPath $templateRoot -Force -Recurse) {
        if (($entry.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw "Refusing to package a linked template path: $($entry.FullName)"
        }
        if ($entry.PSIsContainer) { continue }

        $relativePath = $entry.FullName.Substring($sourcePrefix.Length).Replace('\', '/')
        if (-not (Test-TemplatePathAllowed -RelativePath $relativePath)) {
            throw "Template file is outside the package allowlist: $relativePath"
        }
        if ($relativePath.StartsWith('examples/', [StringComparison]::OrdinalIgnoreCase)) {
            $exampleText = [IO.File]::ReadAllText($entry.FullName)
            if ($exampleText -notmatch '(?i)synthetic') {
                throw "Refusing to package an example without an explicit synthetic-data label: $relativePath"
            }
        }

        $destination = [IO.Path]::GetFullPath((Join-Path $stageRoot ($relativePath.Replace('/', $pathSeparator))))
        $stagePrefixWithSeparator = $stageRoot.TrimEnd('\', '/') + $pathSeparator
        if (-not $destination.StartsWith($stagePrefixWithSeparator, [StringComparison]::OrdinalIgnoreCase)) {
            throw "Template path escapes the staging directory: $relativePath"
        }
        $destinationDirectory = Split-Path -Parent $destination
        New-Item -ItemType Directory -Path $destinationDirectory -Force | Out-Null
        Copy-Item -LiteralPath $entry.FullName -Destination $destination
        $expectedEntries.Add($relativePath)
    }

    $binaryDirectory = Join-Path $stageRoot 'cse'
    New-Item -ItemType Directory -Path $binaryDirectory | Out-Null
    Copy-Item -LiteralPath $binaryPath -Destination (Join-Path $binaryDirectory 'cse.exe')
    $expectedEntries.Add('cse/cse.exe')

    $stagedFiles = @(Get-ChildItem -LiteralPath $stageRoot -Force -Recurse -File)
    $stagedEntries = @($stagedFiles | ForEach-Object { $_.FullName.Substring(($stageRoot.TrimEnd('\', '/') + $pathSeparator).Length).Replace('\', '/') })
    $expectedSorted = @($expectedEntries | Sort-Object)
    $stagedSorted = @($stagedEntries | Sort-Object)
    if (($stagedSorted -join "`n") -cne ($expectedSorted -join "`n")) {
        throw 'Staging directory contents do not match the verified package allowlist.'
    }
    if (@($stagedEntries | Where-Object { $_ -match '(^|/)(\.cse|courses|scoring|pulled|cache)(/|$)' }).Count -gt 0) {
        throw 'Staging directory contains private runtime state or pulled course data.'
    }

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [IO.Compression.ZipFile]::CreateFromDirectory($stageRoot, $zipTempFullPath, [IO.Compression.CompressionLevel]::Optimal, $false)

    $archive = [IO.Compression.ZipFile]::OpenRead($zipTempFullPath)
    try {
        $archiveEntries = @($archive.Entries | ForEach-Object { $_.FullName.TrimEnd('/') })
        $archiveFiles = @($archiveEntries | Where-Object { $_ -ne '' })
        if ($archiveFiles.Count -ne $expectedEntries.Count) {
            throw 'Archive entry count does not match the verified staging directory.'
        }
        $archiveSorted = @($archiveFiles | Sort-Object)
        if (($archiveSorted -join "`n") -cne ($expectedSorted -join "`n")) {
            throw 'Archive file list does not match the verified staging directory.'
        }
        if ($archiveFiles -notcontains '.claude/settings.json') {
            throw 'Archive is missing the hidden .claude/settings.json privacy rule.'
        }
        if ($archiveFiles -notcontains 'settings/courses.md') {
            throw 'Archive is missing settings/courses.md.'
        }
        if (@($archiveFiles | Where-Object { $_ -match '(^|/)(\.cse|courses|scoring|pulled|cache)(/|$)' }).Count -gt 0) {
            throw 'Archive contains private runtime state or pulled course data.'
        }
    }
    finally {
        $archive.Dispose()
    }

    # Replace only the requested archive file after the new archive has passed validation.
    $distFullPath = [IO.Path]::GetFullPath($distRoot)
    if (-not $packagePath.StartsWith(($distFullPath.TrimEnd('\', '/') + $pathSeparator), [StringComparison]::OrdinalIgnoreCase)) {
        throw "Package output path is outside dist: $packagePath"
    }
    if (Test-Path -LiteralPath $packagePath -PathType Leaf) {
        Remove-Item -LiteralPath $packagePath -Force
    }
    Move-Item -LiteralPath $zipTempFullPath -Destination $packagePath

    $hash = (Get-FileHash -LiteralPath $packagePath -Algorithm SHA256).Hash.ToLowerInvariant()
    Write-Host "Packaged $packagePath"
    Write-Host "SHA256 $hash"
}
finally {
    if (Test-Path -LiteralPath $zipTempFullPath -PathType Leaf) {
        Remove-Item -LiteralPath $zipTempFullPath -Force
    }
    if (Test-Path -LiteralPath $stageRoot -PathType Container) {
        $resolvedStage = [IO.Path]::GetFullPath($stageRoot)
        if ($resolvedStage.StartsWith($stagePrefix, [StringComparison]::OrdinalIgnoreCase) -and $resolvedStage -match 'CanvasSidekick-package-[0-9a-f]{32}$') {
            Remove-Item -LiteralPath $resolvedStage -Recurse -Force
        }
    }
}

[CmdletBinding()]
param(
    [Parameter()]
    [string]$GoExecutable = 'go',
    [Parameter()][switch]$Release,
    # Where the current output set is published. Defaults to Source's dist/; tests may point elsewhere.
    [Parameter()][string]$OutputDirectory
)

$ErrorActionPreference = 'Stop'

$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$templateRoot = [IO.Path]::GetFullPath((Join-Path $repoRoot 'workspace-template'))
if (-not $OutputDirectory) { $OutputDirectory = Join-Path $repoRoot 'dist' }
$distRoot = [IO.Path]::GetFullPath($OutputDirectory)
# Single-folder package: the starter workspace with the Engine in engine\. Extracting it
# with Windows' Extract All gives a CanvasExpert-SE folder the teacher points the agent at.
# The teacher download carries the product version so archived copies are self-describing.
$zipName = "CanvasExpert-SE-$([IO.File]::ReadAllText((Join-Path $repoRoot 'VERSION')).Trim()).zip"
$packagePath = [IO.Path]::GetFullPath((Join-Path $distRoot $zipName))
$pathSeparator = [IO.Path]::DirectorySeparatorChar

if (-not (Test-Path -LiteralPath $templateRoot -PathType Container)) {
    throw "Missing starter template: $templateRoot"
}

$tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$stageName = "CanvasExpert-SE-package-$([Guid]::NewGuid().ToString('N'))"
$stageRoot = [IO.Path]::GetFullPath((Join-Path $tempRoot $stageName))
$stagePrefix = $tempRoot.TrimEnd('\', '/') + $pathSeparator
if (-not $stageRoot.StartsWith($stagePrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw "Staging path is outside the system temp directory: $stageRoot"
}

$buildRoot = Join-Path $stageRoot 'build'
$contentRoot = Join-Path $stageRoot 'content'
$zipTempFullPath = Join-Path $stageRoot 'package.zip'
$mcpbPath = Join-Path $stageRoot 'CanvasExpert-SE-Cowork.mcpb'
$binaryPath = Join-Path $buildRoot 'cse.exe'

# Windows PowerShell 5.1 (.NET Framework) writes backslash entry names with
# ZipFile.CreateFromDirectory, which is not a conforming ZIP. Write entries with
# forward slashes ourselves so the archive is identical on every host.
function New-PortableZip {
    param([Parameter(Mandatory)][string]$SourceDirectory, [Parameter(Mandatory)][string]$DestinationPath)

    Add-Type -AssemblyName System.IO.Compression
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $sourcePrefixLength = ($SourceDirectory.TrimEnd('\', '/') + [IO.Path]::DirectorySeparatorChar).Length
    $zip = [IO.Compression.ZipFile]::Open($DestinationPath, [IO.Compression.ZipArchiveMode]::Create)
    try {
        foreach ($file in (Get-ChildItem -LiteralPath $SourceDirectory -Force -Recurse -File | Sort-Object FullName)) {
            $entryName = $file.FullName.Substring($sourcePrefixLength).Replace('\', '/')
            [void][IO.Compression.ZipFileExtensions]::CreateEntryFromFile($zip, $file.FullName, $entryName, [IO.Compression.CompressionLevel]::Optimal)
        }
    }
    finally {
        $zip.Dispose()
    }
}

function Test-TemplatePathAllowed {
    param([Parameter(Mandatory)][string]$RelativePath)

    $normalizedPath = $RelativePath.Replace('\', '/')
    return $normalizedPath -match '^(AGENTS|CLAUDE|README|TEACHER|TEACHING-ASSISTANT)\.md$' -or
        $normalizedPath -eq 'cse.cmd' -or
        $normalizedPath -match '^\.claude/settings\.json$' -or
        $normalizedPath -match '^playbooks/[A-Za-z0-9._-]+\.md$' -or
        $normalizedPath -match '^context/(courses|grading|teaching-profile|students|school-calendar|engine-notes)\.md$'
}

$studentNotesPath = Join-Path $templateRoot 'context/students.md'
if (-not (Test-Path -LiteralPath $studentNotesPath -PathType Leaf)) {
    throw 'The clean template must include context/students.md.'
}
$studentNotes = [IO.File]::ReadAllText($studentNotesPath)
if ($studentNotes -notmatch '(?i)starter file: no students or real student information are configured') {
    throw 'Refusing to package context/students.md: it is not marked as the empty starter file.'
}

$gradingSettingsPath = Join-Path $templateRoot 'context/grading.md'
if (-not (Test-Path -LiteralPath $gradingSettingsPath -PathType Leaf)) {
    throw 'The clean template must include context/grading.md.'
}
$gradingSettings = [IO.File]::ReadAllText($gradingSettingsPath)
if ($gradingSettings -notmatch '(?i)starter file: no teacher-specific preferences are configured') {
    throw 'Refusing to package context/grading.md: it is not marked as the empty starter file.'
}

$teachingProfilePath = Join-Path $templateRoot 'context/teaching-profile.md'
if (-not (Test-Path -LiteralPath $teachingProfilePath -PathType Leaf) -or
    [IO.File]::ReadAllText($teachingProfilePath) -notmatch '(?i)starter file: no teacher-specific preferences are configured') {
    throw 'Refusing to package context/teaching-profile.md: it must be the empty starter file.'
}

$courseSettingsPath = Join-Path $templateRoot 'context/courses.md'
if (-not (Test-Path -LiteralPath $courseSettingsPath -PathType Leaf)) {
    throw 'The clean template must include context/courses.md.'
}
$courseSettings = [IO.File]::ReadAllText($courseSettingsPath)
if ($courseSettings -notmatch '(?i)starter file: no courses configured') {
    throw 'Refusing to package context/courses.md without the empty starter marker.'
}

$claudeSettingsPath = Join-Path $templateRoot '.claude/settings.json'
if (-not (Test-Path -LiteralPath $claudeSettingsPath -PathType Leaf)) {
    throw 'The starter template must include .claude/settings.json.'
}
$claudeSettings = Get-Content -LiteralPath $claudeSettingsPath -Raw | ConvertFrom-Json
if ($claudeSettings.permissions.deny -notcontains 'Read(./.cse/**)') {
    throw 'The starter Claude settings must deny Read access to .cse/**.'
}

try {
    New-Item -ItemType Directory -Path $contentRoot | Out-Null
    New-Item -ItemType Directory -Path $buildRoot | Out-Null
    # Compile once. Everything below packages these exact bytes; nothing is published until all of it passes.
    & (Join-Path $PSScriptRoot 'compile.ps1') -GoExecutable $GoExecutable -Release:$Release -OutputDirectory $buildRoot
    if (-not $?) { throw 'Compile step failed.' }
    if (-not (Test-Path -LiteralPath $binaryPath -PathType Leaf)) { throw "Missing built executable: $binaryPath" }
    $productVersion = [IO.File]::ReadAllText((Join-Path $repoRoot 'VERSION')).Trim()
    $identity = & $binaryPath version --json | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or $identity.version -cne $productVersion) { throw 'Built Engine does not match VERSION.' }
    $artifactIdentity = $productVersion
    if ($identity.build_kind -ne 'release') {
        $safeRevision = $identity.revision.Replace('+', '.')
        $safeTime = ([DateTimeOffset]$identity.built_at).ToUniversalTime().ToString("yyyyMMdd'T'HHmmss'Z'", [Globalization.CultureInfo]::InvariantCulture)
        $artifactIdentity += "-dev.$safeRevision.$safeTime"
    }
    $releaseRoot = Join-Path $distRoot 'release'
    $releasePackagePath = Join-Path $releaseRoot "CanvasExpert-SE-$artifactIdentity-windows-amd64.zip"
    $releaseMcpbPath = Join-Path $releaseRoot "CanvasExpert-SE-Cowork-$artifactIdentity-windows-amd64.mcpb"
    if ($Release -and ((Test-Path -LiteralPath $releasePackagePath) -or (Test-Path -LiteralPath $releaseMcpbPath))) {
        throw "Refusing to overwrite immutable release artifact in $releaseRoot"
    }
    $contentPrefix = $contentRoot.TrimEnd('\', '/') + $pathSeparator
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

        $destination = [IO.Path]::GetFullPath((Join-Path $contentRoot ($relativePath.Replace('/', $pathSeparator))))
        $contentPrefix = $contentRoot.TrimEnd('\', '/') + $pathSeparator
        if (-not $destination.StartsWith($contentPrefix, [StringComparison]::OrdinalIgnoreCase)) {
            throw "Template path escapes the staging directory: $relativePath"
        }
        $destinationDirectory = Split-Path -Parent $destination
        New-Item -ItemType Directory -Path $destinationDirectory -Force | Out-Null
        Copy-Item -LiteralPath $entry.FullName -Destination $destination
        $expectedEntries.Add($relativePath)
    }

    # The template itself never contains a program; the bundle adds exactly one below.
    if (@($expectedEntries | Where-Object { $_ -match '\.exe$|^(cse|engine)/' }).Count -gt 0) {
        throw 'The workspace template must not contain the program.'
    }
    if ($expectedEntries -contains 'webui.cmd') {
        throw 'The workspace template must not contain webui.cmd; the bundle adds it.'
    }
    New-Item -ItemType Directory -Path (Join-Path $contentRoot 'engine') | Out-Null
    Copy-Item -LiteralPath $binaryPath -Destination (Join-Path $contentRoot 'engine/cse.exe')
    Copy-Item -LiteralPath (Join-Path $repoRoot 'VERSION') -Destination (Join-Path $contentRoot 'VERSION')
    $expectedEntries.Add('VERSION')
    Copy-Item -LiteralPath (Join-Path $repoRoot 'LICENSE') -Destination (Join-Path $contentRoot 'LICENSE')
    Copy-Item -LiteralPath (Join-Path $repoRoot 'THIRD_PARTY_NOTICES.md') -Destination (Join-Path $contentRoot 'engine/THIRD_PARTY_NOTICES.md')
    $expectedEntries.Add('LICENSE')
    $expectedEntries.Add('engine/THIRD_PARTY_NOTICES.md')
    Copy-Item -LiteralPath (Join-Path $repoRoot 'scripts/package-webui.cmd') -Destination (Join-Path $contentRoot 'webui.cmd')
    & (Join-Path $PSScriptRoot 'package-mcp.ps1') -BinaryPath $binaryPath -OutputPath $mcpbPath
    if (-not $?) { throw 'Extension packaging failed.' }
    Copy-Item -LiteralPath $mcpbPath -Destination (Join-Path $contentRoot 'engine/CanvasExpert-SE-Cowork.mcpb')
    $expectedEntries.Add('engine/CanvasExpert-SE-Cowork.mcpb')
    $expectedEntries.Add('engine/cse.exe')
    $expectedEntries.Add('webui.cmd')

    $stagedFiles = @(Get-ChildItem -LiteralPath $contentRoot -Force -Recurse -File)
    $stagedEntries = @($stagedFiles | ForEach-Object { $_.FullName.Substring(($contentRoot.TrimEnd('\', '/') + $pathSeparator).Length).Replace('\', '/') })
    $expectedSorted = @($expectedEntries | Sort-Object)
    $stagedSorted = @($stagedEntries | Sort-Object)
    if (($stagedSorted -join "`n") -cne ($expectedSorted -join "`n")) {
        throw 'Staging directory contents do not match the verified package allowlist.'
    }
    if (@($stagedEntries | Where-Object { $_ -match '(^|/)(\.cse|courses|scoring|pulled|cache)(/|$)' }).Count -gt 0) {
        throw 'Staging directory contains private runtime state or pulled course data.'
    }

    New-PortableZip -SourceDirectory $contentRoot -DestinationPath $zipTempFullPath

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
        if ($archiveFiles -notcontains 'cse.cmd') {
            throw 'Archive is missing the cse.cmd launcher.'
        }
        if ($archiveFiles -notcontains 'engine/cse.exe' -or $archiveFiles -notcontains 'webui.cmd') {
            throw 'Archive is missing the bundled engine or webui.cmd.'
        }
        if ($archiveFiles -notcontains 'context/courses.md') {
            throw 'Archive is missing context/courses.md.'
        }
        if (@($archiveFiles | Where-Object { $_ -match '(^|/)(\.cse|courses|scoring|pulled|cache)(/|$)' }).Count -gt 0) {
            throw 'Archive contains private runtime state or pulled course data.'
        }
    }
    finally {
        $archive.Dispose()
    }

    # Publish the current set only now that everything passed. The Engine goes first: if a running
    # copy of dist/cse.exe is locked, this fails before any other current file changes.
    New-Item -ItemType Directory -Path $distRoot -Force | Out-Null
    $zipSum = (Get-FileHash -LiteralPath $zipTempFullPath -Algorithm SHA256).Hash.ToLowerInvariant()
    $sumsStage = Join-Path $stageRoot 'SHA256SUMS.txt'
    [IO.File]::WriteAllText($sumsStage, "$zipSum  $zipName`n", [Text.UTF8Encoding]::new($false))
    $current = [ordered]@{
        'cse.exe'                = $binaryPath
        'cse-mcp-launcher.exe'   = (Join-Path $buildRoot 'cse-mcp-launcher.exe')
        $zipName                = $zipTempFullPath
        'SHA256SUMS.txt'         = $sumsStage
    }
    foreach ($name in $current.Keys) {
        $pending = Join-Path $distRoot ".$name.new"
        Copy-Item -LiteralPath $current[$name] -Destination $pending -Force
    }
    foreach ($name in $current.Keys) {
        Move-Item -LiteralPath (Join-Path $distRoot ".$name.new") -Destination (Join-Path $distRoot $name) -Force
    }
    if ($Release) {
        # Explicit, separate location for the immutable public release set; ordinary builds never write here.
        New-Item -ItemType Directory -Path $releaseRoot -Force | Out-Null
        Copy-Item -LiteralPath $zipTempFullPath -Destination $releasePackagePath
        Copy-Item -LiteralPath $mcpbPath -Destination $releaseMcpbPath
        $releaseSums = @($releasePackagePath, $releaseMcpbPath) | ForEach-Object { (Get-FileHash -LiteralPath $_ -Algorithm SHA256).Hash.ToLowerInvariant() + '  ' + [IO.Path]::GetFileName($_) }
        [IO.File]::WriteAllText((Join-Path $releaseRoot "SHA256SUMS-$artifactIdentity.txt"), ($releaseSums -join "`n") + "`n", [Text.UTF8Encoding]::new($false))
    }

    Write-Host "Packaged $packagePath ($($identity.version), revision $($identity.revision), built $($identity.built_at), $($identity.build_kind))"
    Write-Host "SHA256 $zipSum"
}
finally {
    foreach ($leftover in Get-ChildItem -LiteralPath $distRoot -Force -Filter '.*.new' -ErrorAction SilentlyContinue) {
        Remove-Item -LiteralPath $leftover.FullName -Force
    }
    if (Test-Path -LiteralPath $stageRoot -PathType Container) {
        $resolvedStage = [IO.Path]::GetFullPath($stageRoot)
        if ($resolvedStage.StartsWith($stagePrefix, [StringComparison]::OrdinalIgnoreCase) -and $resolvedStage -match 'CanvasExpert-SE-package-[0-9a-f]{32}$') {
            Remove-Item -LiteralPath $resolvedStage -Recurse -Force
        }
    }
}

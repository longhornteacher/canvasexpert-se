# Builds the git-install distribution from one flattened tree.
#
# The tree is workspace-template/ flattened to the root (playbooks, TEACHER.md, AGENTS.md,
# CLAUDE.md, ...) plus VERSION, LICENSE, webui.cmd, get-engine.cmd, a .gitignore for teacher
# and runtime paths, and engine/manifest.txt naming the Engine files and their SHA-256.
# From that one tree this script writes:
#   - the Engine release assets (cse.exe, CanvasExpert-SE-Cowork.mcpb) in <OutputDirectory>/assets
#   - the fallback ZIP: the tree plus engine/cse.exe and engine/CanvasExpert-SE-Cowork.mcpb
#   - with -UpdateBranch, a commit of the tree on an orphan branch (default: userinstall)
# Teacher data (context/, Teacher_Override.md, courses/) is never part of the tree.
#
# Like package.ps1 this never pushes, tags or publishes. -UpdateBranch makes one local commit
# on the branch using a temporary index, so the checkout, HEAD and index of the repository are
# left alone. Push the branch yourself when intentionally releasing.
[CmdletBinding()]
param(
    [Parameter()][string]$GoExecutable = 'go',
    [Parameter()][switch]$Release,
    # Where assets and the ZIP are written. Defaults to Dev's dist/userinstall.
    [Parameter()][string]$OutputDirectory,
    # Skip compiling and use cse.exe and cse-mcp-launcher.exe from this directory (tests only).
    [Parameter()][string]$BuildDirectory,
    [Parameter()][switch]$UpdateBranch,
    [Parameter()][string]$BranchName = 'userinstall',
    # The repository that receives the branch. Defaults to this checkout.
    [Parameter()][string]$TargetRepository,
    # Where get-engine.cmd downloads from. Defaults to the GitHub release for VERSION; a
    # non-release build must pass one (for example a file:/// URL of <OutputDirectory>/assets).
    [Parameter()][string]$BaseUrl
)

$ErrorActionPreference = 'Stop'
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$templateRoot = Join-Path $repoRoot 'workspace-template'
if (-not $OutputDirectory) { $OutputDirectory = Join-Path $repoRoot 'dist/userinstall' }
if (-not $TargetRepository) { $TargetRepository = $repoRoot }
$sep = [IO.Path]::DirectorySeparatorChar
$utf8 = [Text.UTF8Encoding]::new($false)

# Only these template files reach a teacher by git or ZIP. context/ and Teacher_Override.md are
# seeded by the Engine on first run and belong to the teacher; they must not be tracked.
function Test-TreePathAllowed {
    param([Parameter(Mandatory)][string]$RelativePath)
    $p = $RelativePath.Replace('\', '/')
    return $p -match '^(AGENTS|CLAUDE|README|TEACHER|TEACHING-ASSISTANT)\.md$' -or
        $p -eq 'cse.cmd' -or
        $p -eq '.claude/settings.json' -or
        $p -match '^playbooks/[A-Za-z0-9._-]+\.md$'
}
function Test-TemplatePathTeacherOwned {
    param([Parameter(Mandatory)][string]$RelativePath)
    $p = $RelativePath.Replace('\', '/')
    return $p -match '^context/' -or $p -eq 'Teacher_Override.md'
}

function New-PortableZip {
    param([Parameter(Mandatory)][string]$SourceDirectory, [Parameter(Mandatory)][string]$DestinationPath)
    Add-Type -AssemblyName System.IO.Compression
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $prefix = ($SourceDirectory.TrimEnd('\', '/') + $sep).Length
    $zip = [IO.Compression.ZipFile]::Open($DestinationPath, [IO.Compression.ZipArchiveMode]::Create)
    try {
        foreach ($file in (Get-ChildItem -LiteralPath $SourceDirectory -Force -Recurse -File | Sort-Object FullName)) {
            $name = $file.FullName.Substring($prefix).Replace('\', '/')
            [void][IO.Compression.ZipFileExtensions]::CreateEntryFromFile($zip, $file.FullName, $name, [IO.Compression.CompressionLevel]::Optimal)
        }
    }
    finally { $zip.Dispose() }
}

function Invoke-Git {
    param([Parameter(Mandatory)][string[]]$GitArgs)
    $out = & git @GitArgs 2>&1
    if ($LASTEXITCODE -ne 0) { throw "git $($GitArgs -join ' ') failed: $out" }
    return ($out | Out-String).Trim()
}

$productVersion = [IO.File]::ReadAllText((Join-Path $repoRoot 'VERSION')).Trim()
$tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$stageRoot = Join-Path $tempRoot "CanvasExpert-SE-userinstall-$([Guid]::NewGuid().ToString('N'))"
$buildRoot = Join-Path $stageRoot 'build'
$treeRoot = Join-Path $stageRoot 'tree'
$zipRoot = Join-Path $stageRoot 'zip'

try {
    New-Item -ItemType Directory -Path $buildRoot, $treeRoot | Out-Null

    # --- Engine binaries -------------------------------------------------------------------
    if ($BuildDirectory) {
        foreach ($name in 'cse.exe', 'cse-mcp-launcher.exe') {
            Copy-Item -LiteralPath (Join-Path $BuildDirectory $name) -Destination (Join-Path $buildRoot $name)
        }
    }
    else {
        & (Join-Path $PSScriptRoot 'compile.ps1') -GoExecutable $GoExecutable -Release:$Release -OutputDirectory $buildRoot
        if (-not $?) { throw 'Compile step failed.' }
    }
    $binaryPath = Join-Path $buildRoot 'cse.exe'
    $identity = & $binaryPath version --json | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or $identity.version -cne $productVersion) { throw 'Built Engine does not match VERSION.' }
    if ($Release -and $identity.build_kind -ne 'release') { throw 'A -Release package needs a release build.' }
    $artifactIdentity = $productVersion
    if ($identity.build_kind -ne 'release') {
        $safeRevision = $identity.revision.Replace('+', '.')
        $safeTime = ([DateTimeOffset]$identity.built_at).ToUniversalTime().ToString("yyyyMMdd'T'HHmmss'Z'", [Globalization.CultureInfo]::InvariantCulture)
        $artifactIdentity += "-dev.$safeRevision.$safeTime"
    }
    $mcpbPath = Join-Path $buildRoot 'CanvasExpert-SE-Cowork.mcpb'
    & (Join-Path $PSScriptRoot 'package-mcp.ps1') -BinaryPath $binaryPath -OutputPath $mcpbPath
    if (-not $?) { throw 'Extension packaging failed.' }

    # --- The tree --------------------------------------------------------------------------
    $sourcePrefix = $templateRoot.TrimEnd('\', '/') + $sep
    foreach ($entry in Get-ChildItem -LiteralPath $templateRoot -Force -Recurse) {
        if (($entry.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) { throw "Refusing a linked template path: $($entry.FullName)" }
        if ($entry.PSIsContainer) { continue }
        $relative = $entry.FullName.Substring($sourcePrefix.Length).Replace('\', '/')
        if (Test-TemplatePathTeacherOwned -RelativePath $relative) { continue }
        if (-not (Test-TreePathAllowed -RelativePath $relative)) { throw "Template file is outside the tree allowlist: $relative" }
        $dest = Join-Path $treeRoot $relative.Replace('/', $sep)
        New-Item -ItemType Directory -Path (Split-Path -Parent $dest) -Force | Out-Null
        Copy-Item -LiteralPath $entry.FullName -Destination $dest
    }
    $settings = Get-Content -LiteralPath (Join-Path $treeRoot '.claude/settings.json') -Raw | ConvertFrom-Json
    if ($settings.permissions.deny -notcontains 'Read(./.cse/**)') { throw 'The starter Claude settings must deny Read access to .cse/**.' }

    New-Item -ItemType Directory -Path (Join-Path $treeRoot 'engine') | Out-Null
    Copy-Item -LiteralPath (Join-Path $repoRoot 'VERSION') -Destination (Join-Path $treeRoot 'VERSION')
    Copy-Item -LiteralPath (Join-Path $repoRoot 'LICENSE') -Destination (Join-Path $treeRoot 'LICENSE')
    Copy-Item -LiteralPath (Join-Path $repoRoot 'THIRD_PARTY_NOTICES.md') -Destination (Join-Path $treeRoot 'engine/THIRD_PARTY_NOTICES.md')
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'package-webui.cmd') -Destination (Join-Path $treeRoot 'webui.cmd')
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'get-engine.cmd') -Destination (Join-Path $treeRoot 'get-engine.cmd')
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'userinstall.gitignore') -Destination (Join-Path $treeRoot '.gitignore')
    # Batch files fail on bare-LF line endings; keep them CRLF on any checkout.
    [IO.File]::WriteAllText((Join-Path $treeRoot '.gitattributes'), "*.cmd text eol=crlf`n", $utf8)
    # Every other text file is LF, so the tree (and the branch) is identical whichever host built it.
    foreach ($file in Get-ChildItem -LiteralPath $treeRoot -Force -Recurse -File) {
        $ext = $file.Extension.ToLowerInvariant()
        if ($ext -eq '.cmd') {
            $text = [IO.File]::ReadAllText($file.FullName).Replace("`r`n", "`n").Replace("`n", "`r`n")
        }
        elseif ($ext -in '.md', '.json', '.txt', '.gitignore', '.gitattributes' -or $file.Name -in 'LICENSE', 'VERSION', '.gitignore', '.gitattributes') {
            $text = [IO.File]::ReadAllText($file.FullName).Replace("`r`n", "`n")
        }
        else { continue }
        [IO.File]::WriteAllText($file.FullName, $text, $utf8)
    }

    # --- Manifest --------------------------------------------------------------------------
    if (-not $BaseUrl) {
        if (-not $Release) { throw 'A non-release package needs -BaseUrl (for example a file:/// URL of the assets directory).' }
        $BaseUrl = "https://github.com/longhornteacher/canvasexpert-se/releases/download/v$productVersion"
    }
    $assets = [ordered]@{ 'cse.exe' = $binaryPath; 'CanvasExpert-SE-Cowork.mcpb' = $mcpbPath }
    $lines = @('# Written by scripts/package-userinstall.ps1. get-engine.cmd and the Engine read this file.',
        "version=$productVersion", "identity=$artifactIdentity", "base_url=$($BaseUrl.TrimEnd('/'))")
    foreach ($name in $assets.Keys) {
        $lines += "sha256.$name=$((Get-FileHash -LiteralPath $assets[$name] -Algorithm SHA256).Hash.ToLowerInvariant())"
    }
    [IO.File]::WriteAllText((Join-Path $treeRoot 'engine/manifest.txt'), (($lines -join "`n") + "`n"), $utf8)

    # --- Verify the tree before anything is written ---------------------------------------
    $treeFiles = @(Get-ChildItem -LiteralPath $treeRoot -Force -Recurse -File | ForEach-Object { $_.FullName.Substring($treeRoot.Length + 1).Replace('\', '/') })
    foreach ($required in 'AGENTS.md', 'CLAUDE.md', 'cse.cmd', 'get-engine.cmd', 'webui.cmd', '.gitignore', 'engine/manifest.txt', '.claude/settings.json') {
        if ($treeFiles -notcontains $required) { throw "Tree is missing $required" }
    }
    if (@($treeFiles | Where-Object { $_ -match '\.(exe|mcpb)$' }).Count -gt 0) { throw 'The tree must not contain the Engine; it is a release asset.' }
    if (@($treeFiles | Where-Object { $_ -match '^(context|courses)/' -or $_ -eq 'Teacher_Override.md' -or $_ -match '(^|/)(\.cse|scoring|pulled|cache)(/|$)' }).Count -gt 0) {
        throw 'The tree contains teacher or runtime data.'
    }

    # --- Assets and ZIP --------------------------------------------------------------------
    $assetRoot = Join-Path $OutputDirectory 'assets'
    New-Item -ItemType Directory -Path $assetRoot -Force | Out-Null
    $assetSums = @()
    foreach ($name in $assets.Keys) {
        Copy-Item -LiteralPath $assets[$name] -Destination (Join-Path $assetRoot $name) -Force
        $assetSums += (Get-FileHash -LiteralPath $assets[$name] -Algorithm SHA256).Hash.ToLowerInvariant() + '  ' + $name
    }
    [IO.File]::WriteAllText((Join-Path $assetRoot 'SHA256SUMS.txt'), (($assetSums -join "`n") + "`n"), $utf8)

    New-Item -ItemType Directory -Path $zipRoot | Out-Null
    Copy-Item -Path (Join-Path $treeRoot '*') -Destination $zipRoot -Recurse -Force
    Copy-Item -LiteralPath (Join-Path $treeRoot '.claude') -Destination $zipRoot -Recurse -Force
    Copy-Item -LiteralPath (Join-Path $treeRoot '.gitignore'), (Join-Path $treeRoot '.gitattributes') -Destination $zipRoot
    foreach ($name in $assets.Keys) { Copy-Item -LiteralPath $assets[$name] -Destination (Join-Path $zipRoot "engine/$name") }
    $zipPath = Join-Path $OutputDirectory "CanvasExpert-SE-$artifactIdentity-userinstall.zip"
    if (Test-Path -LiteralPath $zipPath) { Remove-Item -LiteralPath $zipPath -Force }
    New-PortableZip -SourceDirectory $zipRoot -DestinationPath $zipPath

    # --- Optional local commit on the branch ----------------------------------------------
    $commit = $null
    if ($UpdateBranch) {
        $gitDir = Invoke-Git @('-C', $TargetRepository, 'rev-parse', '--absolute-git-dir')
        $ref = "refs/heads/$BranchName"
        $parent = $null
        & git --git-dir=$gitDir rev-parse --verify --quiet "$ref^{commit}" 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) { $parent = Invoke-Git @("--git-dir=$gitDir", 'rev-parse', "$ref^{commit}") }
        $sourceRevision = $identity.revision
        $savedEnv = @{ GIT_INDEX_FILE = $env:GIT_INDEX_FILE; GIT_WORK_TREE = $env:GIT_WORK_TREE }
        try {
            $env:GIT_INDEX_FILE = Join-Path $stageRoot 'index'
            $env:GIT_WORK_TREE = $treeRoot
            Invoke-Git @("--git-dir=$gitDir", '-c', 'core.autocrlf=false', 'add', '-A', '-f', '.') | Out-Null
            $tree = Invoke-Git @("--git-dir=$gitDir", 'write-tree')
        }
        finally {
            foreach ($k in $savedEnv.Keys) {
                # A $null passed as a string would set the variable to empty, not remove it.
                if ($null -eq $savedEnv[$k]) { Remove-Item -LiteralPath "Env:$k" -ErrorAction SilentlyContinue } else { Set-Item -LiteralPath "Env:$k" -Value $savedEnv[$k] }
            }
        }
        $parentTree = $null
        if ($parent) { $parentTree = Invoke-Git @("--git-dir=$gitDir", 'rev-parse', "$parent^{tree}") }
        if ($parentTree -eq $tree) {
            $commit = $parent
            Write-Host "Branch $BranchName already holds this tree ($parent); no commit made."
        }
        else {
            $message = "Release $artifactIdentity (source $sourceRevision)"
            $commitArgs = @("--git-dir=$gitDir", 'commit-tree', $tree, '-m', $message)
            if ($parent) { $commitArgs += @('-p', $parent) }
            $commit = Invoke-Git $commitArgs
            Invoke-Git @("--git-dir=$gitDir", 'update-ref', $ref, $commit) | Out-Null
            Write-Host "Branch $BranchName is now $commit (local only; push it yourself when releasing)."
        }
    }

    Write-Host "Tree files: $($treeFiles.Count). Assets: $assetRoot"
    Write-Host "ZIP: $zipPath"
    Write-Host "Manifest identity $artifactIdentity, base_url $($BaseUrl.TrimEnd('/'))"
}
finally {
    if (Test-Path -LiteralPath $stageRoot -PathType Container) {
        $resolved = [IO.Path]::GetFullPath($stageRoot)
        if ($resolved.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase) -and $resolved -match 'CanvasExpert-SE-userinstall-[0-9a-f]{32}$') {
            Remove-Item -LiteralPath $resolved -Recurse -Force
        }
    }
}

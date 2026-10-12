[CmdletBinding()]
param([Parameter()][string]$BinaryPath = (Join-Path $PSScriptRoot '../dist/cse.exe'))

$ErrorActionPreference = 'Stop'
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$enginePath = (Resolve-Path -LiteralPath $BinaryPath).Path
$launcherPath = (Resolve-Path -LiteralPath (Join-Path (Split-Path -Parent $enginePath) 'cse-mcp-launcher.exe')).Path
$outputDirectory = Join-Path $repoRoot 'dist'
New-Item -ItemType Directory -Path $outputDirectory -Force | Out-Null
$outputPath = Join-Path $outputDirectory 'CanvasExpert-SE-Cowork.mcpb'
$productVersion = [IO.File]::ReadAllText((Join-Path $repoRoot 'VERSION')).Trim()
$identity = & $enginePath version --json | ConvertFrom-Json
if ($LASTEXITCODE -ne 0 -or $identity.version -cne $productVersion) { throw 'Engine version does not match VERSION; rebuild before packaging.' }
$artifactIdentity = $productVersion
if ($identity.build_kind -ne 'release') {
    $safeRevision = $identity.revision.Replace('+', '.')
    $safeTime = ([DateTimeOffset]$identity.built_at).ToUniversalTime().ToString("yyyyMMdd'T'HHmmss'Z'", [Globalization.CultureInfo]::InvariantCulture)
    $artifactIdentity += "-dev.$safeRevision.$safeTime"
}
$versionedPath = Join-Path $outputDirectory "CanvasExpert-SE-Cowork-$artifactIdentity-windows-amd64.mcpb"
if ($identity.build_kind -eq 'release' -and (Test-Path -LiteralPath $versionedPath)) { throw "Refusing to overwrite immutable release artifact $versionedPath" }
$manifest = Get-Content -LiteralPath (Join-Path $repoRoot 'integrations/cowork/manifest.json') -Raw | ConvertFrom-Json
$manifest.version = $productVersion
if ($identity.build_kind -ne 'release') { $manifest.version += "+dev.$safeRevision.$safeTime" }
$temporaryPath = Join-Path $outputDirectory ('.mcpb-' + [guid]::NewGuid().ToString('N') + '.zip')
Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem
try {
    $archive = [IO.Compression.ZipFile]::Open($temporaryPath, [IO.Compression.ZipArchiveMode]::Create)
    try {
        $manifestEntry = $archive.CreateEntry('manifest.json')
        $manifestWriter = [IO.StreamWriter]::new($manifestEntry.Open(), [Text.UTF8Encoding]::new($false))
        try { $manifestWriter.Write(($manifest | ConvertTo-Json -Depth 20)) } finally { $manifestWriter.Dispose() }
        [void][IO.Compression.ZipFileExtensions]::CreateEntryFromFile($archive, $launcherPath, 'launcher/cse-mcp-launcher.exe')
        [void][IO.Compression.ZipFileExtensions]::CreateEntryFromFile($archive, (Join-Path $repoRoot 'docs/mcp-adapter.md'), 'README.md')
        [void][IO.Compression.ZipFileExtensions]::CreateEntryFromFile($archive, (Join-Path $repoRoot 'LICENSE'), 'LICENSE')
        [void][IO.Compression.ZipFileExtensions]::CreateEntryFromFile($archive, (Join-Path $repoRoot 'THIRD_PARTY_NOTICES.md'), 'THIRD_PARTY_NOTICES.md')
    }
    finally { $archive.Dispose() }
    Move-Item -LiteralPath $temporaryPath -Destination $outputPath -Force
    Copy-Item -LiteralPath $outputPath -Destination $versionedPath
    Write-Host "Packaged $outputPath"
    Write-Host ('SHA256 ' + (Get-FileHash -LiteralPath $outputPath -Algorithm SHA256).Hash.ToLowerInvariant())
}
finally {
    if (Test-Path -LiteralPath $temporaryPath -PathType Leaf) { Remove-Item -LiteralPath $temporaryPath }
}

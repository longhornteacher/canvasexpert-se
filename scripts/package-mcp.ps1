[CmdletBinding()]
param([Parameter()][string]$BinaryPath = (Join-Path $PSScriptRoot '../dist/cse.exe'))

$ErrorActionPreference = 'Stop'
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$enginePath = (Resolve-Path -LiteralPath $BinaryPath).Path
$outputDirectory = Join-Path $repoRoot 'dist'
New-Item -ItemType Directory -Path $outputDirectory -Force | Out-Null
$outputPath = Join-Path $outputDirectory 'CanvasExpert-SE-Cowork.mcpb'
$temporaryPath = Join-Path $outputDirectory ('.mcpb-' + [guid]::NewGuid().ToString('N') + '.zip')
Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem
try {
    $archive = [IO.Compression.ZipFile]::Open($temporaryPath, [IO.Compression.ZipArchiveMode]::Create)
    try {
        [void][IO.Compression.ZipFileExtensions]::CreateEntryFromFile($archive, (Join-Path $repoRoot 'integrations/cowork/manifest.json'), 'manifest.json')
        [void][IO.Compression.ZipFileExtensions]::CreateEntryFromFile($archive, $enginePath, 'engine/cse.exe')
        [void][IO.Compression.ZipFileExtensions]::CreateEntryFromFile($archive, (Join-Path $repoRoot 'docs/mcp-adapter.md'), 'README.md')
    }
    finally { $archive.Dispose() }
    Move-Item -LiteralPath $temporaryPath -Destination $outputPath -Force
    Write-Host "Packaged $outputPath"
    Write-Host ('SHA256 ' + (Get-FileHash -LiteralPath $outputPath -Algorithm SHA256).Hash.ToLowerInvariant())
}
finally {
    if (Test-Path -LiteralPath $temporaryPath -PathType Leaf) { Remove-Item -LiteralPath $temporaryPath }
}

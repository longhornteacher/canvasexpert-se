[CmdletBinding()]
param(
    [Parameter(Mandatory)][Alias("Workspace")][string]$Installation,
    [Parameter()][string]$Engine,
    [Parameter()][ValidateSet('Both', 'Cowork', 'Work')][string]$Client = 'Both',
    [Parameter()][string]$CodexExecutable = 'codex'
)

$ErrorActionPreference = 'Stop'
$installationPath = (Resolve-Path -LiteralPath $Installation).Path
if ([string]::IsNullOrWhiteSpace($Engine)) { $Engine = Join-Path $installationPath 'engine/cse.exe' }
$enginePath = (Resolve-Path -LiteralPath $Engine).Path
if (-not (Test-Path -LiteralPath (Join-Path $installationPath 'context/courses.md') -PathType Leaf)) {
    throw 'The selected folder is not a CE-SE installation: context/courses.md is missing.'
}
if ([IO.Path]::GetExtension($enginePath) -ne '.exe') { throw 'Engine must be cse.exe.' }

if ($Client -in @('Both', 'Work')) {
    # The CLI uses the desktop app's MCP configuration and preserves other servers.
    & $CodexExecutable mcp add canvasexpert-se -- $enginePath --workspace $installationPath mcp
    if ($LASTEXITCODE -ne 0) { throw "Work MCP registration failed ($LASTEXITCODE)." }
    Write-Host 'Registered canvasexpert-se for local Work/Codex. Restart the client or refresh MCP connections.'
}

if ($Client -in @('Both', 'Cowork')) {
    $configDirectory = Join-Path $env:APPDATA 'Claude'
    $configPath = Join-Path $configDirectory 'claude_desktop_config.json'
    New-Item -ItemType Directory -Path $configDirectory -Force | Out-Null
    $configuration = [pscustomobject]@{}
    if (Test-Path -LiteralPath $configPath -PathType Leaf) {
        $configuration = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
        $backupPath = $configPath + '.cse-backup-' + [DateTime]::UtcNow.ToString('yyyyMMddHHmmssfff')
        Copy-Item -LiteralPath $configPath -Destination $backupPath
    }
    if (-not $configuration.PSObject.Properties['mcpServers']) {
        $configuration | Add-Member -NotePropertyName mcpServers -NotePropertyValue ([pscustomobject]@{})
    }
    $server = [pscustomobject]@{command=$enginePath; args=@('--workspace', $installationPath, 'mcp')}
    $configuration.mcpServers | Add-Member -NotePropertyName 'canvasexpert-se' -NotePropertyValue $server -Force
    $temporaryPath = $configPath + '.cse-new-' + [guid]::NewGuid().ToString('N')
    try {
        [IO.File]::WriteAllText($temporaryPath, ($configuration | ConvertTo-Json -Depth 100), [Text.UTF8Encoding]::new($false))
        Move-Item -LiteralPath $temporaryPath -Destination $configPath -Force
    }
    finally {
        if (Test-Path -LiteralPath $temporaryPath -PathType Leaf) { Remove-Item -LiteralPath $temporaryPath }
    }
    Write-Host 'Registered canvasexpert-se in Claude Desktop. Fully quit and reopen Claude, then check Cowork connectors.'
    Write-Host 'If Cowork requires a desktop extension, install dist/CanvasExpert-SE-Cowork.mcpb instead. Use only one CE-SE registration.'
}

Write-Host "Engine: $enginePath"
Write-Host "CE-SE installation: $installationPath"

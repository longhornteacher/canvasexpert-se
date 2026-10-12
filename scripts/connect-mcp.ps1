[CmdletBinding()]
param(
    [Parameter(Mandatory)][Alias('Workspace')][string]$Installation,
    [Parameter(Mandatory)][ValidateSet('Cowork', 'Work')][string]$Client,
    [Parameter()][switch]$ReplaceExisting,
    [Parameter()][switch]$NoUI
)
$ErrorActionPreference = 'Stop'
$installationPath = (Resolve-Path -LiteralPath $Installation).Path
$enginePath = Join-Path $installationPath 'engine/cse.exe'
$installArgs = @('install', '--client', $Client.ToLowerInvariant())
if ($ReplaceExisting) { $installArgs += '--replace-existing' }
if ($NoUI) { $installArgs += '--no-ui' }
& $enginePath @installArgs
if ($LASTEXITCODE -ne 0) { throw "CE-SE installation needs attention (exit $LASTEXITCODE); see the Engine's explanation above." }

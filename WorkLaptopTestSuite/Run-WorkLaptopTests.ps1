<#
.SYNOPSIS
  CE-SE WorkLaptop environment checks. Read-only apart from temporary test files
  and one dummy credential, all removed at the end.

.DESCRIPTION
  Answers the machine-specific questions in ARCHITECTURE.md §3 / D15 / D27:
  can an unsigned .exe run from user and OneDrive folders, do "downloaded from
  the internet" marks block it, does Credential Manager work without admin, can
  a loopback web page be served, and is Canvas reachable without TLS trouble.

  Must run in Windows PowerShell 5.1 (`powershell`, not `pwsh`) because the
  test .exe is compiled with Add-Type -OutputType ConsoleApplication.

.EXAMPLE
  powershell -NoProfile -ExecutionPolicy Bypass -File .\Run-WorkLaptopTests.ps1 -CanvasHost yourdistrict.instructure.com
#>
param(
    [Parameter(Mandatory = $true)][string]$CanvasHost,
    [string]$WorkspacePath,
    [switch]$KeepFiles
)

$ErrorActionPreference = 'Stop'
$results = @()
$started = Get-Date

function Hide-User([string]$text) {
    if (-not $text) { return $text }
    $t = $text.Replace($env:USERPROFILE, '%USERPROFILE%')
    return $t.Replace($env:USERNAME, '<user>')
}

function Add-Result([string]$id, [string]$name, [string]$status, [string]$detail) {
    $script:results += [pscustomobject]@{ Id = $id; Test = $name; Status = $status; Detail = (Hide-User $detail) }
    $color = @{ PASS = 'Green'; FAIL = 'Red'; WARN = 'Yellow'; INFO = 'Gray'; SKIP = 'DarkGray' }[$status]
    Write-Host ("[{0}] {1} {2}: {3}" -f $status, $id, $name, (Hide-User $detail)) -ForegroundColor $color
}

function Invoke-Exe([string]$path) {
    try {
        $out = & $path 2>&1 | Out-String
        if ($out -match 'CESE-HELLO') { return @('PASS', 'ran and printed expected output') }
        return @('FAIL', "ran but unexpected output: $($out.Trim())")
    } catch {
        return @('FAIL', $_.Exception.Message.Trim())
    }
}

if (-not $WorkspacePath) {
    $root = $env:OneDriveCommercial
    if (-not $root) { $root = $env:OneDrive }
    if (-not $root) { $root = Join-Path $env:USERPROFILE 'Documents' }
    $WorkspacePath = Join-Path $root 'CESE-WorkLaptopTest'
}
$tempDir  = Join-Path $env:TEMP 'CESE-WorkLaptopTest'
$localDir = Join-Path $env:LOCALAPPDATA 'CESE-WorkLaptopTest'
foreach ($d in @($WorkspacePath, $tempDir, $localDir)) { New-Item -ItemType Directory -Force -Path $d | Out-Null }

# ---------------------------------------------------------------- T1 environment
try {
    $os = Get-CimInstance Win32_OperatingSystem
    Add-Result 'T1.1' 'Windows version' 'INFO' "$($os.Caption) build $($os.BuildNumber)"
} catch { Add-Result 'T1.1' 'Windows version' 'WARN' $_.Exception.Message }

$mode = $ExecutionContext.SessionState.LanguageMode
Add-Result 'T1.2' 'PowerShell version / language mode' $(if ($mode -eq 'FullLanguage') { 'PASS' } else { 'WARN' }) "PS $($PSVersionTable.PSVersion); LanguageMode=$mode"

try {
    $pol = (Get-ExecutionPolicy -List | ForEach-Object { "$($_.Scope)=$($_.ExecutionPolicy)" }) -join '; '
    Add-Result 'T1.3' 'Execution policy' 'INFO' $pol
} catch { Add-Result 'T1.3' 'Execution policy' 'WARN' $_.Exception.Message }

try {
    $adminLine = ((whoami /groups | Select-String 'S-1-5-32-544' | Out-String).Trim()) -replace '\s+', ' '
    if ($adminLine) { Add-Result 'T1.4' 'Local admin membership' 'INFO' "member of Administrators: $adminLine" }
    else { Add-Result 'T1.4' 'Local admin membership' 'INFO' 'not a member of local Administrators (expected)' }
} catch { Add-Result 'T1.4' 'Local admin membership' 'WARN' $_.Exception.Message }

try {
    $xml = [xml](Get-AppLockerPolicy -Effective -Xml)
    $col = $xml.AppLockerPolicy.RuleCollection | Where-Object { $_.Type } | ForEach-Object { "$($_.Type)=$($_.EnforcementMode) ($(@($_.ChildNodes).Count) rules)" }
    if ($col) { Add-Result 'T1.5' 'AppLocker' 'INFO' ($col -join '; ') } else { Add-Result 'T1.5' 'AppLocker' 'INFO' 'no effective AppLocker rule collections' }
} catch { Add-Result 'T1.5' 'AppLocker' 'INFO' "could not read policy: $($_.Exception.Message)" }

try {
    $dg = Get-CimInstance -Namespace root\Microsoft\Windows\DeviceGuard -ClassName Win32_DeviceGuard
    Add-Result 'T1.6' 'WDAC (code integrity)' 'INFO' "kernel=$($dg.CodeIntegrityPolicyEnforcementStatus) usermode=$($dg.UsermodeCodeIntegrityPolicyEnforcementStatus) (0=off 1=audit 2=enforced)"
} catch { Add-Result 'T1.6' 'WDAC (code integrity)' 'INFO' "could not read: $($_.Exception.Message)" }

try {
    $sac = (Get-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\CI\Policy' -Name VerifiedAndReputablePolicyState -ErrorAction Stop).VerifiedAndReputablePolicyState
    Add-Result 'T1.7' 'Smart App Control' 'INFO' "state=$sac (0=off 1=on 2=evaluation)"
} catch { Add-Result 'T1.7' 'Smart App Control' 'INFO' 'not present / not readable' }

Add-Result 'T1.8' 'OneDrive locations' 'INFO' "OneDriveCommercial=$env:OneDriveCommercial; OneDrive=$env:OneDrive; test workspace=$WorkspacePath"

$tools = foreach ($c in 'git', 'python', 'py', 'node', 'go', 'winget', 'claude', 'codex', 'curl.exe', 'tar.exe') {
    $cmd = Get-Command $c -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($cmd) { "$c=$($cmd.Source)" } else { "$c=-" }
}
Add-Result 'T1.9' 'Available tools' 'INFO' ($tools -join '; ')

# ---------------------------------------------------------------- T2 unsigned exe
$exe = Join-Path $tempDir 'cese-hello.exe'
$compiled = $false
try {
    Add-Type -TypeDefinition 'public static class CeseHello { public static void Main() { System.Console.WriteLine("CESE-HELLO"); } }' `
        -OutputAssembly $exe -OutputType ConsoleApplication
    $compiled = Test-Path $exe
    Add-Result 'T2.0' 'Compile unsigned test exe' 'PASS' $exe
} catch {
    Add-Result 'T2.0' 'Compile unsigned test exe' 'FAIL' "Add-Type blocked or failed: $($_.Exception.Message). T2/T3 need a prebuilt exe instead."
}

if ($compiled) {
    $r = Invoke-Exe $exe; Add-Result 'T2.1' 'Run exe from %TEMP%' $r[0] $r[1]
    $lexe = Join-Path $localDir 'cese-hello.exe'; Copy-Item $exe $lexe -Force
    $r = Invoke-Exe $lexe; Add-Result 'T2.2' 'Run exe from %LOCALAPPDATA%' $r[0] $r[1]
    $wsBin = Join-Path $WorkspacePath 'cse'; New-Item -ItemType Directory -Force -Path $wsBin | Out-Null
    $wexe = Join-Path $wsBin 'cese-hello.exe'; Copy-Item $exe $wexe -Force
    $r = Invoke-Exe $wexe; Add-Result 'T2.3' 'Run exe from workspace (OneDrive) folder' $r[0] $r[1]
    try {
        $out = cmd /c "`"$wexe`"" 2>&1 | Out-String
        Add-Result 'T2.4' 'Run workspace exe via cmd.exe' $(if ($out -match 'CESE-HELLO') { 'PASS' } else { 'FAIL' }) $out.Trim()
    } catch { Add-Result 'T2.4' 'Run workspace exe via cmd.exe' 'FAIL' $_.Exception.Message }
} else {
    foreach ($id in 'T2.1', 'T2.2', 'T2.3', 'T2.4') { Add-Result $id 'Run unsigned exe' 'SKIP' 'no test exe' }
}

# ---------------------------------------------------------------- T3 internet marks (MOTW)
if ($compiled) {
    $wsBin = Join-Path $WorkspacePath 'cse'
    $motw = Join-Path $wsBin 'cese-hello-motw.exe'
    Copy-Item $exe $motw -Force
    try {
        Set-Content -Path $motw -Stream Zone.Identifier -Value "[ZoneTransfer]`r`nZoneId=3"
        $r = Invoke-Exe $motw; Add-Result 'T3.1' 'Run exe marked as downloaded' $r[0] $r[1]
        Unblock-File $motw
        $r = Invoke-Exe $motw; Add-Result 'T3.2' 'Run same exe after Unblock-File' $r[0] $r[1]
    } catch { Add-Result 'T3.1' 'Run exe marked as downloaded' 'WARN' $_.Exception.Message }

    try {
        $zipSrc = Join-Path $tempDir 'zipsrc'; New-Item -ItemType Directory -Force -Path $zipSrc | Out-Null
        Copy-Item $exe (Join-Path $zipSrc 'cese-hello.exe') -Force
        $zip = Join-Path $tempDir 'cese-test.zip'
        Compress-Archive -Path (Join-Path $zipSrc '*') -DestinationPath $zip -Force
        Set-Content -Path $zip -Stream Zone.Identifier -Value "[ZoneTransfer]`r`nZoneId=3"
        $unz = Join-Path $WorkspacePath 'unzipped'
        Expand-Archive -Path $zip -DestinationPath $unz -Force
        $uexe = Join-Path $unz 'cese-hello.exe'
        $marked = [bool](Get-Item $uexe -Stream Zone.Identifier -ErrorAction SilentlyContinue)
        $r = Invoke-Exe $uexe
        Add-Result 'T3.3' 'Marked ZIP -> Expand-Archive -> run' $r[0] "$($r[1]); extracted exe carries mark: $marked"
    } catch { Add-Result 'T3.3' 'Marked ZIP -> Expand-Archive -> run' 'WARN' $_.Exception.Message }
    Add-Result 'T3.4' 'Real OneDrive web download + Explorer unzip' 'SKIP' 'manual step; see README'
} else {
    Add-Result 'T3' 'Internet marks' 'SKIP' 'no test exe'
}

# ---------------------------------------------------------------- T4 Credential Manager
$credTarget = 'CESE-WorkLaptopTest'
try {
    cmdkey /generic:$credTarget /user:cese-test /pass:dummy-not-a-secret | Out-Null
    $listed = (cmdkey /list:$credTarget | Out-String) -match [regex]::Escape($credTarget)
    Add-Result 'T4.1' 'Write credential (cmdkey)' $(if ($listed) { 'PASS' } else { 'FAIL' }) "listed after write: $listed"
} catch { Add-Result 'T4.1' 'Write credential (cmdkey)' 'FAIL' $_.Exception.Message }

try {
    Add-Type -Namespace CeseCred -Name Native -MemberDefinition @'
[DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
public static extern bool CredRead(string target, int type, int flags, out System.IntPtr cred);
[DllImport("advapi32.dll")] public static extern void CredFree(System.IntPtr cred);
'@
    $ptr = [IntPtr]::Zero
    $ok = [CeseCred.Native]::CredRead($credTarget, 1, 0, [ref]$ptr)
    if ($ok) { [CeseCred.Native]::CredFree($ptr) }
    Add-Result 'T4.2' 'Read credential back (CredRead API)' $(if ($ok) { 'PASS' } else { 'FAIL' }) "CredRead returned $ok"
} catch { Add-Result 'T4.2' 'Read credential back (CredRead API)' 'WARN' "could not test from PowerShell: $($_.Exception.Message)" }

try { cmdkey /delete:$credTarget | Out-Null } catch { }

# ---------------------------------------------------------------- T5 loopback web page
$port = 47831
try {
    $tcp = New-Object System.Net.Sockets.TcpListener ([System.Net.IPAddress]::Loopback), $port
    $tcp.Start(); $tcp.Stop()
    Add-Result 'T5.1' 'Bind raw TCP on 127.0.0.1' 'PASS' "port $port"
} catch { Add-Result 'T5.1' 'Bind raw TCP on 127.0.0.1' 'FAIL' $_.Exception.Message }

foreach ($prefixHost in 'localhost', '127.0.0.1') {
    $id = if ($prefixHost -eq 'localhost') { 'T5.2' } else { 'T5.3' }
    $listener = $null
    try {
        $listener = New-Object System.Net.HttpListener
        $listener.Prefixes.Add("http://${prefixHost}:$port/")
        $listener.Start()
        $outFile = Join-Path $tempDir "http-$prefixHost.txt"
        $proc = Start-Process curl.exe -ArgumentList '-s', '-o', "`"$outFile`"", "http://${prefixHost}:$port/ping" -PassThru -WindowStyle Hidden
        $async = $listener.BeginGetContext($null, $null)
        if ($async.AsyncWaitHandle.WaitOne(8000)) {
            $ctx = $listener.EndGetContext($async)
            $bytes = [System.Text.Encoding]::UTF8.GetBytes('CESE-PONG')
            $ctx.Response.OutputStream.Write($bytes, 0, $bytes.Length); $ctx.Response.Close()
            $proc.WaitForExit(5000) | Out-Null
            $got = if (Test-Path $outFile) { Get-Content $outFile -Raw } else { '' }
            Add-Result $id "Serve HTTP page on $prefixHost" $(if ($got -match 'CESE-PONG') { 'PASS' } else { 'FAIL' }) "client received: $got"
        } else {
            Add-Result $id "Serve HTTP page on $prefixHost" 'FAIL' 'no request arrived within 8s'
        }
    } catch {
        Add-Result $id "Serve HTTP page on $prefixHost" 'FAIL' $_.Exception.Message
    } finally {
        if ($listener) { try { $listener.Close() } catch { } }
    }
}

# ---------------------------------------------------------------- T6 Canvas reachability / TLS
try {
    $code = (curl.exe -sS -o NUL -w '%{http_code}' "https://$CanvasHost/api/v1/users/self" 2>&1 | Out-String).Trim()
    $st = if ($code -eq '401') { 'PASS' } else { 'WARN' }
    Add-Result 'T6.1' 'Reach Canvas API (unauthenticated)' $st "HTTP $code (401 expected without a token)"
} catch { Add-Result 'T6.1' 'Reach Canvas API (unauthenticated)' 'FAIL' $_.Exception.Message }

try {
    $client = New-Object System.Net.Sockets.TcpClient($CanvasHost, 443)
    $script:issuer = $null
    $cb = [System.Net.Security.RemoteCertificateValidationCallback] { param($s, $cert, $chain, $err) $script:issuer = $cert.Issuer; $true }
    $ssl = New-Object System.Net.Security.SslStream($client.GetStream(), $false, $cb)
    $ssl.AuthenticateAsClient($CanvasHost)
    $ssl.Dispose(); $client.Close()
    $public = $script:issuer -match 'Amazon|DigiCert|Let''s Encrypt|Sectigo|GlobalSign|GoDaddy|Google Trust|Starfield|Entrust|ISRG'
    $st = if ($public) { 'PASS' } else { 'WARN' }
    Add-Result 'T6.2' 'Canvas TLS certificate issuer' $st "$($script:issuer)$(if (-not $public) { ' -- possible district TLS inspection' })"
} catch { Add-Result 'T6.2' 'Canvas TLS certificate issuer' 'WARN' "could not inspect: $($_.Exception.Message)" }

try {
    $proxy = (netsh winhttp show proxy | Out-String).Trim() -replace '\s+', ' '
    Add-Result 'T6.3' 'System proxy (WinHTTP)' 'INFO' $proxy
} catch { Add-Result 'T6.3' 'System proxy (WinHTTP)' 'INFO' $_.Exception.Message }

# ---------------------------------------------------------------- T7 agent hosts
foreach ($a in 'claude', 'codex') {
    $cmd = Get-Command $a -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($cmd) {
        try { $v = (& $cmd.Source --version 2>&1 | Out-String).Trim(); Add-Result 'T7' "$a CLI" 'PASS' "$v at $($cmd.Source)" }
        catch { Add-Result 'T7' "$a CLI" 'WARN' $_.Exception.Message }
    } else { Add-Result 'T7' "$a CLI" 'INFO' 'not on PATH (may still be available as a desktop app; see README)' }
}

# ---------------------------------------------------------------- T8 workspace file speed
try {
    $pdir = Join-Path $WorkspacePath 'pulled-speed'
    New-Item -ItemType Directory -Force -Path $pdir | Out-Null
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    for ($i = 1; $i -le 200; $i++) { Set-Content -Path (Join-Path $pdir "s$i.md") -Value "student $i`nsome submission text" }
    $write = $sw.ElapsedMilliseconds; $sw.Restart()
    $n = (Get-ChildItem $pdir -File | ForEach-Object { Get-Content $_.FullName -Raw }).Count
    $read = $sw.ElapsedMilliseconds
    Add-Result 'T8' 'Write/read 200 small files in workspace' 'INFO' "write ${write}ms, read $n files ${read}ms"
} catch { Add-Result 'T8' 'Write/read 200 small files in workspace' 'WARN' $_.Exception.Message }

# ---------------------------------------------------------------- report
$resultsDir = Join-Path $PSScriptRoot 'results'
New-Item -ItemType Directory -Force -Path $resultsDir | Out-Null
$report = Join-Path $resultsDir ("{0}-{1}.md" -f $env:COMPUTERNAME, $started.ToString('yyyyMMdd-HHmm'))
$lines = @(
    "# WorkLaptop test results",
    "",
    "- Machine: $env:COMPUTERNAME",
    "- Run: $($started.ToString('yyyy-MM-dd HH:mm'))",
    "- Canvas host: $CanvasHost",
    "",
    "| Id | Test | Status | Detail |",
    "|---|---|---|---|"
)
foreach ($r in $results) { $lines += "| $($r.Id) | $($r.Test) | $($r.Status) | $(($r.Detail -replace '\|', '/') -replace "`r?`n", ' ') |" }
$lines += ""
$lines += "## Manual steps (fill in)"
$lines += ""
$lines += "- T3.4 OneDrive web download + Explorer unzip + run: "
$lines += "- T5.4 Browser opens http://localhost page from Start-Process: "
$lines += "- T7.2 Claude Code / Codex usable on this machine (how installed): "
$lines += "- Notes: "
Set-Content -Path $report -Value $lines -Encoding UTF8

if (-not $KeepFiles) {
    foreach ($d in @($WorkspacePath, $tempDir, $localDir)) { Remove-Item -Recurse -Force $d -ErrorAction SilentlyContinue }
}

Write-Host ""
Write-Host "Report written to: $report" -ForegroundColor Cyan

# WorkLaptop test suite

These are checks that must run **on the district WorkLaptop**, because colleagues' machines have the same model and permissions (ARCHITECTURE D15). Their results decide the implementation language (Q9) and where the `cse` program can live.

Canvas API behaviors marked **[verify]** in ARCHITECTURE.md are *not* here. They can be checked from any machine against a sandbox course.

## Run it

From this folder, in **Windows PowerShell** (`powershell`, not `pwsh`):

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\Run-WorkLaptopTests.ps1 -CanvasHost <your-district>.instructure.com
```

- It takes under a minute. It creates temporary files in `%TEMP%`, `%LOCALAPPDATA%` and a `CESE-WorkLaptopTest` folder in OneDrive, plus one dummy credential, and removes them all at the end (`-KeepFiles` to keep them).
- It never uses your real Canvas token.
- The report lands in `results/<COMPUTERNAME>-<date>.md`. Your username is replaced with `<user>`. Fill in the manual steps at the bottom of the report, then commit it.
- **If the command itself is refused** (e.g. "running scripts is disabled on this system" with a MachinePolicy/UserPolicy execution policy), that is a result in itself. Note it in a results file by hand: PowerShell scripts are then off the table as a fallback language. Don't try to work around district policy.

## What each test answers

| Test | Question | If it fails… |
|---|---|---|
| T1 | What's enforced here: PowerShell language mode, execution policy, AppLocker, WDAC, Smart App Control? Which tools already exist? | Explains any T2–T5 failures. |
| T2.0 | Can PowerShell compile a tiny unsigned test `.exe`? | Usually means Constrained Language Mode. T2/T3 then need a prebuilt `.exe`, built elsewhere and brought over. |
| T2.1–T2.3 | Can an unsigned `.exe` run from `%TEMP%`, `%LOCALAPPDATA%`, the OneDrive workspace? | **T2.3 fails, T2.2 passes:** the program lives in `%LOCALAPPDATA%` and the workspace points at it. **All fail:** single-`.exe` languages (Go, C#, Rust) are out, and PowerShell becomes the fallback. |
| T2.4 | Does it run when launched through `cmd.exe` (as some agents do)? | Note the difference from T2.3. |
| T3.1–T3.3 | Do "downloaded from the internet" marks block the program, and does unzipping carry the mark onto it? | Sharing via ZIP needs an extra "Unblock" step in the friend setup instructions. |
| T4 | Can SE store and read back a credential in Windows Credential Manager without admin? | The PAT needs another per-user home. Decide before any code is written. |
| T5 | Can SE serve its setup page on `localhost` / `127.0.0.1`? | The WebUI needs a different approach (e.g. a local file form). |
| T6.1–T6.2 | Is Canvas reachable, and is the district inspecting TLS (a certificate issuer that isn't a public CA)? | **Inspection present:** the language must trust the Windows certificate store (Go does by default on Windows; Python and Node need care). |
| T6.3 | Is there a proxy? | The HTTP client must honor it. |
| T7 | Are Claude Code / Codex on PATH? | See manual step T7.2. |
| T8 | How fast is writing/reading many small files in the OneDrive workspace? | Very slow means `pulled/` should live outside OneDrive. |

## Manual steps (record in the report)

- **T3.4 — Real ZIP share:**
  1. Put any small `.exe` into a ZIP. You can rerun the script with `-KeepFiles` and zip `%TEMP%\CESE-WorkLaptopTest\cese-hello.exe`.
  2. Upload the ZIP to OneDrive from another machine and share it to yourself.
  3. On the WorkLaptop, download it **through the browser**, unzip it with **File Explorer** and run the `.exe` from a terminal.
  4. Record: did it run, and did Windows show a SmartScreen or "blocked" message?
- **T5.4 — Browser:** while a page is served (or later, with the real `cse setup`), does `Start-Process http://localhost:<port>/` open the default browser?
- **T7.2 — Agent host:** how is Claude Code (or Codex) running on this machine (desktop app, CLI in the user profile, other)? Did it need anything an admin had to approve?

## After running

The next session (on WorkLaptop) reads the committed report and records in ARCHITECTURE.md:

- the language decision (Q9);
- where the `cse` program lives (workspace vs `%LOCALAPPDATA%`);
- any friend-setup steps (e.g. Unblock).

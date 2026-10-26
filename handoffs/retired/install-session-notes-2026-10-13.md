> **Retired 2026-10-13.** Historical instructions and evidence; do not execute this plan. Retirement does not establish completion or new validation; preserve the dated evidence and teacher work described below.

# Fresh-unzip install session: notes for development (2026-10-13)

Setting: Windows 11 Enterprise, Claude Desktop (Code tab), fresh unzip at `C:\githubprojects\CanvasExpert-SE`, teacher said "Fresh unzip, install please," then chose **both** Cowork and Work. Engine build: `1.0.0-beta.1+ae41e92e9916+dirty.2026-10-12T23:09:05Z.development` (a "dirty" development build shipped in the zip).

No student data or secrets here. The PAT was entered only in the private WebUI and never seen by the agent.

## Timeline

1. Agent read `AGENTS.md` -> `playbooks/install.md`. Destination was unstated, so the playbook's "ask one plain question" applied. Teacher answered "both."
2. `.\cse.cmd install --client work` -> exit code 4.
   - `state: settings-saved`; wrote `C:\Users\<user>\.codex\config.toml` and a timestamped backup.
   - Local MCP check passed (26 tools, instructions read, configured courses read, `destination_verified: false`).
   - Error line: `private WebUI did not report startup; open webui.cmd in this Install and retry connection from the hub`.
3. `.\cse.cmd install --client cowork` -> exit code 4.
   - `state: extension-ready`, `extension_path` = `engine\CanvasExpert-SE-Cowork-this-install.mcpb`, `cowork_launch_outcome: launch_requested`.
   - Same local check pass, same WebUI-did-not-start error.
4. Agent ran `Start-Process .\webui.cmd`. The teacher then entered Canvas address + PAT in the form.
5. Teacher reported the `.mcpb` did not open in Claude (Windows "Select an app to open this .mcpb file" dialog listing Acrobat, Firefox, Notepad, VLC, VS Code, WMP, Word; no Claude). Teacher's verdict: this strands a new user.
6. Agent launched `%LOCALAPPDATA%\AnthropicClaude\claude.exe "<mcpb path>"`; **Claude Desktop showed its extension-install dialog (teacher confirmed).** This is the working approach.
7. `.\cse.cmd courses` failed: `context/courses.md instance differs from this machine's saved Canvas origin; update the configuration`.
   - `.\cse.cmd doctor --json`: engine, canvas_connection (authenticated read), hub, install, active_rules, credential_round_trip all pass; `course_config: not_configured`; **`saved_origin: fail`** ("Saved Canvas origin differs from context/courses.md").
8. Teacher supplied `pearland.instructure.com`. Agent set `instance: https://pearland.instructure.com` in `context/courses.md`; `.\cse.cmd courses` then listed 17 courses (10 available, 7 unpublished). Nicknames/current flags are NOT yet chosen; `courses:` is still `[]`.

## Problems observed -> changes needed

### A. `cse install --client cowork`: OS file association fails (HIGH)
- The helper reports `cowork_launch_outcome: launch_requested` but relies on the OS to open the `.mcpb`. On this machine `.mcpb` has no handler, so the user gets the "Select an app" chooser. Success is reported even though nothing opened.
- Verified fix: launch Claude Desktop directly with the file as argument, e.g. `%LOCALAPPDATA%\AnthropicClaude\claude.exe "<abs path to .mcpb>"` -> install dialog appears.
- Needed in Engine/installer code:
  - Locate Claude Desktop (stable launcher `%LOCALAPPDATA%\AnthropicClaude\claude.exe`; versioned `app-*` dirs also exist; also consider Store/MSIX and per-machine installs, and do not hardcode only one path).
  - Launch with the file as argument; fall back to Explorer `/select,` plus the printed instruction "drag it into Settings > Extensions".
  - Report an honest outcome (`launched-with-claude-desktop` / `fallback-explorer`), not just `launch_requested`.
  - Still cannot confirm trust/installation; keep "destination still needs a trust action."
  - Consider macOS equivalent (`open -a Claude <file>`) for non-Windows.
- Already done in this session: `playbooks/install.md` now documents this (agent-driven launch). The Go installer change has NOT been made; `engine-notes.md` has two lines about it.

### B. "private WebUI did not report startup" on both installs (HIGH)
- Playbook says to open the WebUI independently as part of `install` (and not to pass `--no-ui`). It failed in both runs; agent had to launch `webui.cmd` manually. Unknown whether it is a race (timeout too short on first start / antivirus first-run scan), a port problem, or the helper's startup handshake broken in this build. Investigate with logs; no logs were inspected here (`.cse/` is off limits to the agent).
- The helper also exits **4** for the whole command even though the primary action succeeded (settings saved / extension ready, local check passed). The agent has to parse JSON to know what happened. Suggest: distinct exit codes or a structured `warnings` array so "installed, UI needs manual start" is distinguishable from real failure.
- The helper's WebUI URL was not surfaced (no fallback URL in output). The agent had no way to hand the teacher a link, so it launched `webui.cmd` and hoped a browser opened. Print the local URL in the JSON when startup succeeds, and a clear "run webui.cmd" when it does not. (Note: the teacher did get to the form, so `webui.cmd` works.)

### C. Setup form saved credential, but `context/courses.md` kept the placeholder origin (HIGH)
- Starter `context/courses.md` ships with `instance: https://canvas.example.edu`. After the teacher entered their Canvas address + PAT in the form, `courses` and `doctor` reported the mismatch (`saved_origin`), meaning the form saved an origin but did not update `courses.md` (playbook claims "Setup records the Canvas address in `context/courses.md`"). Possible causes: form flow not finished at the time of my first check, teacher entered something different, or the form does not write `courses.md` when the courses list is empty/starter. Not determined; reproduce with a clean unzip.
- Expected behavior: finishing the form should write the origin to `courses.md` (or `courses`/`doctor` should treat the starter placeholder `canvas.example.edu` as "unset" and adopt the saved origin instead of failing).
- Better UX: error message `context/courses.md instance differs from this machine's saved Canvas origin; update the configuration` does not say what either value is, so the agent cannot tell which is wrong and the saved origin is unreadable to it. Include both origins (they are not secret) in `doctor`'s explanation and in the `courses` error.

### D. Canvas address collection UX (MEDIUM; playbook updated, engine could help)
- Teacher goal: user pastes any logged-in Canvas link; agent derives the base URL. The agent now has that guidance in `install.md` (origin only; drop path/query; confirm with teacher).
- Possible Engine support: a `setup --origin <url>` / typed tool that normalizes a pasted link to an HTTPS origin and writes `courses.md` + saved config, so the agent never hand-edits `courses.md` and normalization is tested code. Validate it rejects non-HTTPS/non-Instructure-lookalike hosts per policy, and strips `access_token`/session params from a pasted URL without echoing them.
- The teacher gave a bare host (`pearland.instructure.com`), so normalization must accept host-only input.

### E. `.\cse.cmd` stderr noise under PowerShell (LOW)
- `courses` prints progress (`courses…`, `courses: 608ms, 1 HTTP requests`) on stderr; PowerShell 5.1 wraps that as `NativeCommandError` and sets `$?` false even on success (courses succeeded; earlier `help setup` also showed exit 255). Consider keeping progress on stdout or only when a TTY, and `help <topic>` returning exit 0.
- `.\cse.cmd help setup` ignored the topic and printed the general usage with exit 255.

### F. Playbook gaps for "do as much as possible, hand-hold the rest" (MEDIUM; partly done)
Done in `playbooks/install.md` this session: "Walkthrough principles" and "Cowork extension (.mcpb) without the teacher doing it" sections. Still to consider:
- **"Both" destinations:** playbook says "never silently register both" and to ask Cowork vs Work. Teacher said "both" explicitly, which worked, but the playbook should say that an explicit "both" is fine and the order (Work first, then Cowork) is not important.
- **Work destination hand-off:** `settings-saved` writes `~/.codex/config.toml` (the Codex/Work config). The teacher still needs to reconnect/restart Work. Add a concrete, observable verification step (what they should see) and have the agent re-check `list_courses` `install_root` after reconnect. Not yet verified: neither destination has confirmed CE-SE tools in a real conversation.
- **Cowork post-install:** after the teacher clicks Install/trust, verify tools appear in a Cowork conversation; not yet done.
- **Order of operations:** best flow is (1) pick destination(s), (2) run helper(s), (3) open WebUI and get Canvas origin from a pasted link, (4) PAT in form, (5) `doctor`, (6) `courses`, nicknames, (7) launch mcpb/reconnect, (8) pilot. Playbook currently has the extension step before Canvas steps; reorder if desired.
- **Courses step:** with 17 courses (duplicates: FUND OF COMPUTER SCIENCE: one available, one unpublished; POWER READING 7: both unpublished), the agent should propose the likely current set (e.g. available + the teacher's name in the course name) as a suggestion, but nicknames must still come from the teacher per `setup.md`.
- `context/teaching-profile.md`, `grading.md`, `school-calendar.md` were not touched this session.

### G. Minor / observations
- The install helper's JSON includes a very long `child_engine_version` blob with a dependency list; the agent only needs adapter vs engine version equality. Trim for readability.
- "dirty ... development" build_kind in a distributed zip: confirm that is intended for the beta.
- `.codex/config.toml` backup file name pattern: `config.toml.cse-backup-<UTC timestamp>`; fine, but the user config dir ended up with a new backup on every run, so repeated installs will accumulate backups.
- Starter `courses.md` body text says "Starter file: no courses configured" and `schema: 1`; fine.

## State at end of this session (so the next one can pick up)
- Work: connection settings saved; teacher still needs to reconnect/restart Work and confirm `list_courses` returns `install_root` = `C:\githubprojects\CanvasExpert-SE`.
- Cowork: install dialog showed up in Claude Desktop; the teacher was about to click Install/trust. Not yet confirmed installed.
- Canvas: PAT saved in the local credential store; origin `https://pearland.instructure.com` in `context/courses.md`; `courses` works.
- Courses: not yet selected (no nicknames). Waiting on the teacher.
- Files changed in Install this session: `playbooks/install.md` (new walkthrough sections), `context/engine-notes.md` (3 lines), `context/courses.md` (instance), this file.

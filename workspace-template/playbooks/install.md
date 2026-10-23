# Set this all up for the teacher

Use this when the teacher has CanvasExpert-SE in a folder (cloned from git, or unzipped as the fallback), opens it in a coding agent (Claude Code, Codex, Antigravity) and says "set this all up," "set this up," "get me ready to teach," or "install this", or asks to connect Claude, ChatGPT or Antigravity. For full first-run setup, read both this playbook and `setup.md`. That folder is already **UserInstall**. Run directly here; setup connects apps and Canvas without copying the product into a second folder; no Dev checkout, developer runtime, separate Codex CLI or teacher terminal command is needed.

## Own the whole setup

A broad setup request means getting the teacher ready to work, not merely running the connection helper. Keep the existing install.md filename and cse install command as internal compatibility names; say "set up" to the teacher. A request only to connect one app stays limited to that app.

Do not add CE-SE, Go, Node, PowerShell or an agent CLI to PATH for setup. Run this folder's `cse.cmd` by its explicit path, or its `engine/cse.exe` directly. The helper registers absolute Engine paths for Work/Codex and Antigravity; Cowork's packaged launcher targets this same folder. No compiler, package manager or global command installation is required. If an agent cannot invoke a bare command, use the full path instead of changing PATH. Moving UserInstall afterward requires reconnecting the apps to its new location.

Before changing anything, inspect this folder's saved rules location and configured courses through its bundled Engine. Preserve completed setup, saved course nicknames and preferences. Offer shared setup once: "If you already keep your CanvasExpert setup in OneDrive or Google Drive, I can use that folder here too. Otherwise we can keep everything on this computer." If the teacher supplies a directory, follow setup.md to select it and reread saved courses/preferences before configuring new ones. Do not require Sync or a preferences questionnaire.

Finish the applicable steps: app connections prepared and remaining prompts explained; Canvas connection checked; requested courses saved with the teacher's nicknames; optional shared setup handled if requested; intended app actually reading this folder's saved courses. A local helper check alone does not mean an app is ready. If one step is blocked, continue independent setup and name the specific unfinished step. Then help with the teacher's original task or offer a first teaching task.

## How to talk to the teacher

The teacher is a classroom teacher, not a developer. Be brisk and friendly, in the first person, one step at a time. Continue authorized setup yourself; wait only when you need a teacher choice, private form entry or app approval. Do not ask permission to continue after each routine step. Say what is coming before each step and say plainly when it is done. Never say MCP, extension, trust prompt, PAT, origin, UserInstall, Engine, registration or adapter. The one term they must meet is Canvas's own button label, **New Access Token**: explain it the first time. Do not give time estimates. Never ask for the token in chat; it goes only in the private form. Do not read out the technical checklist further down; say only what the teacher needs to do next. Ask about preferences and the school calendar only when the requested teaching task needs them; late-timing preparation and current-term parent reports use the saved school calendar. Do not make calendar setup a condition of unrelated work.

The lines below are the script. Adapt the wording to the situation; keep the plain language.

### 1. Opening

Do not ask which app they use: they are already talking to one, and the Engine finds the rest.

> "I'll get CanvasExpert ready to use here: connect your apps and Canvas, help you choose your classes, and check that your teaching assistant can see them. I'll handle the setup; you'll enter your Canvas access token in a private form and approve any prompts your apps show."

Skip this opening if they have already told you what they want. If they only use a website version of an app, say so honestly: CanvasExpert connects to apps installed on this computer, not websites. Never install an app or subscription for them.

### 2. Connecting the apps

> "Connecting now."

Run the helper (see "Run the helper"); with no `--client` it connects every supported app it finds.

The helper also creates the teacher's **CanvasExpert WebUI** desktop shortcut for this folder. When it reports success, tell the teacher they can use that shortcut to reopen CanvasExpert's settings and course/student pages. A shortcut warning does not block the rest of setup: explain it briefly and give `webui.cmd` in this folder as the fallback. Do not claim the shortcut exists when creation failed, and do not replace a shortcut pointing to another folder without the teacher choosing which folder to keep.

Report app connection results in the teacher's terms:

> "I've set up CanvasExpert for {Claude, ChatGPT, Antigravity} on this computer. It can also work in {the others}, so if you ever start using one, just come back here and I'll connect it too."

The helper picks apps by their settings folders, which is a hint, not proof that an app is installed or in use. Don't say you "found" an app; if the teacher says they don't use one, drop it.

Then give what each app still needs:

**Claude, app opened with the file:**

> "Claude should be asking whether to add CanvasExpert. Please click to approve it. If you don't see anything, tell me and I'll walk you through adding it by hand."

**Claude, file shown in a folder window (or the app could not be opened):**

> "I've put a file called CanvasExpert-SE-Cowork in a folder window. In Claude, open Settings, find Extensions, and add that file. Tell me when it's in."

**ChatGPT or Antigravity:**

> "{App} is set up. When we're done here, quit it completely and open it again so it notices. I'll tell you when."

When you ask for a restart, say how to quit fully: "Closing the window can leave it running near the clock. Right-click its icon there (click the ^ arrow if you don't see it) and choose Quit, then open it again." If you are running inside that app, ask for the restart only as the very last step, after everything else is done, because quitting ends this conversation. Do not ask anyone to close an app during an active CE-SE review. If nothing supported was found, say so and ask which app they plan to use.

### 3. The Canvas form

> "Now I need CanvasExpert to be able to see your courses. Canvas lets you make a private key for that. It's called an **access token**: a key that lets CanvasExpert access your courses and carry out the Canvas work you request. You can delete it in Canvas any time. It is saved in Windows Credential Manager on this computer and sent only to your Canvas site when making requests, never to the assistant in chat.
>
> **My safety promise:** Setting this up does not publish coursework or change grades. When we work, I follow the scope you request. Scores, new assignments and quizzes, and assignment edits normally go through a private review page; you can skip review for one task, or tell me to always skip it.
>
> A form is opening in your browser."

If the page did not open, give the address the helper returned (never invent one) or say: "Double-click **webui.cmd** in your CanvasExpert folder."

> "In the form you'll type your **Canvas address**. Open Canvas the way you normally do (a school portal like ClassLink or Clever is fine), then look at the address bar: it's the part like `yourschool.instructure.com`. Then paste your key. Here's how to make one:
> 1. Sign in to Canvas.
> 2. Click **Account**, then **Settings**.
> 3. Scroll to **Approved Integrations** and click **+ New Access Token**. (Once you tell me your Canvas address I can give you a direct link to that page: `https://<your-address>/profile/settings`.)
> 4. For the purpose, type CanvasExpert. For the expiration, choose the end of the school year, or the longest your school allows, so you don't have to redo this mid-semester.
> 5. Click Generate and **copy the long code Canvas shows you.** It only shows it once.
> 6. Paste it into the form and click **Connect**.
>
> Please don't paste the code here in our chat. Tell me when you've clicked Connect."

**No + New Access Token button:** some districts turn off token creation for teachers. Say: "Your district has turned off access tokens for teacher accounts, so CanvasExpert can't connect to Canvas yet. Your Canvas administrator can turn it back on for you. Here's a note you can send them:" and give a short note in the teacher's voice: they'd like a Canvas personal access token for a local teaching assistant that runs on their own computer; the token stays in Windows Credential Manager and reaches only the district's Canvas; they choose the expiry and can delete it any time. Meanwhile you can still help them draft, and nothing else needs redoing later.

**Code pasted into the chat:** if the teacher pastes the token here anyway, don't use it, repeat it or save it anywhere. Say: "Thanks, but please don't use that one: anything typed in our chat may be kept. In Canvas, go back to Approved Integrations, delete the CanvasExpert token you just made, make a new one, and paste the new one into the form instead." Then carry on with the form.

If the teacher says their school limits token life, add: "Canvas will stop accepting this key when it expires. When that happens, I'll tell you and we'll make a new one the same way. Nothing else needs redoing."

### 4. After the form

- **It worked:** "Connected. I can see your Canvas account."
- **Canvas says the code is wrong, missing or expired:** "Canvas didn't accept that code. A missing character when copying is the usual cause, and codes also expire. Making a new one is quick: Account, Settings, + New Access Token, then paste it into the same form." Keep their saved courses and settings.
- **Could not reach Canvas** (timeout, network, school filter, Canvas outage): "I couldn't reach Canvas just now. It may be your school's network or Canvas itself. Your code is saved, so there's nothing to redo. We can try again in a few minutes, and I can still help you draft in the meantime." Do not tell them to replace the token.
- **Two different Canvas addresses:** "I see two different Canvas addresses: {A} and {B}. Which one do you sign in with?" Use their answer.
- **Not sure why it failed** (for example a permissions error on one course): say so plainly; do not assume the token is bad.

### 5. Choosing courses

Read saved courses first and preserve existing selections and nicknames. If courses are missing, read the Canvas list and use course IDs and states to distinguish similarly named sections. Ask which classes the teacher wants and what short names they use; do not invent nicknames from Canvas titles.

> "Here are your Canvas classes. Which ones are you teaching now, and what short name do you use for each?"

Skip this question when the requested courses are already configured.

Save what they confirm as `setup.md` describes: "Saved. You can change these any time."

### 6. Finishing up

Only ask for a restart, approval or new chat when that app still needs it. If the current assistant already has working CE-SE tools for this folder, verify the saved courses here and continue teaching.

> "Canvas and your classes are set up. To finish connecting {the app(s)}, {approve the pop-up / quit it completely and open it again}. Then start a new chat there and paste this:"

Give the starter message from `TEACHING-ASSISTANT.md`. After they say they have done it:

> "Great. Ask it to show you your courses. If it lists {their course nicknames}, you're all set. If not, tell me what it says."

Each connected app counts only after its assistant lists the right courses. If something is still pending, name only the one real next action ("Claude is waiting for you to approve its prompt; Antigravity needs to be closed and reopened").

Then give three doors instead of a questionnaire:

> "All set! To get our feet wet, what would help you most today?
> - **Make a page look great:** tell me a lesson topic.
> - **Build a quick quiz:** tell me the topic and how many questions.
> - **Clear a grading stack:** name an assignment with ungraded work and we'll go through it together.
>
> Pick whichever sounds easiest, or just tell me what's on your desk."

Do not ask about visual style, grading scale or teaching philosophy now. Ask on the first page build or first scoring session, and save the answers to `context/teaching-profile.md` and `context/grading.md`.

## What you do behind the scenes

Do not narrate this to the teacher.

### Get CanvasExpert onto the computer

Use this only when there is no CanvasExpert folder yet. CanvasExpert runs on Windows 10 or 11 only; on a Mac or Chromebook, say so and stop. Ask before downloading anything, and never overwrite an existing folder.

Check what your own session allows before you start. Downloading needs network access from your shell, and connecting apps writes their settings outside this folder (for example `%APPDATA%\Claude` and `%USERPROFILE%\.codex`). Some agents need approval for each; ask for it when needed and explain it in a sentence. Web search access does not mean your shell can download.

1. Choose a local folder that OneDrive and Google Drive do not sync (for example `%USERPROFILE%\CanvasExpert-SE`); district sync folders can block programs.
2. Check for git with `where git`. Use git when it is present; when it is not, use the release ZIP in step 2b. Never install git, or anything else, just to install CanvasExpert.
   - **2a, git present:** `git clone --branch userinstall --single-branch https://github.com/longhornteacher/canvasexpert-se.git "<folder>"`. In the folder, run `.\get-engine.cmd`. It downloads the Engine, checks it against the checksum recorded in `engine\manifest.txt`, and installs it in `engine\`. It needs only `curl.exe` and `certutil`, which Windows includes, and no admin rights. Exit codes: 3 the download did not match its checksum (it was discarded), 4 a needed Windows tool or the manifest is missing, 5 the download failed, 6 the running Engine could not be replaced.
   - **2b, no git:** download and unpack the release ZIP with Windows' own tools: `curl.exe -fL -o "%TEMP%\CanvasExpert-SE-windows.zip" https://github.com/longhornteacher/canvasexpert-se/releases/latest/download/CanvasExpert-SE-windows.zip`, then `mkdir "<folder>"` and `tar -xf "%TEMP%\CanvasExpert-SE-windows.zip" -C "<folder>"`, then delete the downloaded ZIP. This is a ZIP install; follow "ZIP fallback" below for its playbooks and updates.
   - If curl fails with `CRYPT_E_NO_REVOCATION_CHECK` or `CRYPT_E_REVOCATION_OFFLINE`, the network cannot reach the certificate revocation servers (seen on school networks). Retry that download once with `--ssl-revoke-best-effort`; `get-engine.cmd` does this itself. Treat any other curl error as an ordinary download failure.
3. Run `.\cse.cmd version`; the first Engine run adds the teacher's starter files under `context/` and `Teacher_Override.md`. If it fails with "Access is denied", follow "If Windows blocks the program" below. Then continue with the script at the top.

If github.com is blocked on this computer, say so plainly: the git install and the ZIP come from the same place, so neither will work on this network. The teacher can try from another network, or ask IT to allow github.com. Do not work around a district block.

### If Windows blocks the program

District security can stop `engine\cse.exe` from running. The usual signs are "Access is denied" when `cse.cmd` starts it (exit code 5; `get-engine.cmd` downloads the file but never runs it) and a Windows Security pop-up at the bottom right of the screen. Moving the folder or downloading again will not help, and you must not try to get around it.

Confirm the cause from the Windows log, which the teacher's own account can read:

`Get-WinEvent -FilterHashtable @{LogName='Microsoft-Windows-Windows Defender/Operational'; Id=1121; StartTime=(Get-Date).AddHours(-1)} | Where-Object Message -match 'cse\.exe' | Select-Object -First 1 -ExpandProperty Message`

Event 1121 with rule ID `01443614-cd74-433a-b99e-2ecdc07bfc25` means the district's Microsoft Defender blocks programs that are not yet widely used ("prevalence, age or trusted list"). Every CanvasExpert release is a new file, so time alone will not fix it, and allowing one file by its checksum stops working at the next update. If the log shows a different rule, or no event, report what you see and do not guess.

Tell the teacher: "Your district's security software blocks programs it doesn't recognise yet, and it is blocking CanvasExpert. That's a district setting, so your IT team needs to allow it. Here's a note you can send them:" Then give a short note in the teacher's voice with:

- what it is: CanvasExpert-SE, an open-source teaching assistant for Canvas (https://github.com/longhornteacher/canvasexpert-se), running on their own computer without admin rights
- what blocked it: Microsoft Defender attack surface reduction rule 01443614-cd74-433a-b99e-2ecdc07bfc25, Defender event 1121, and the blocked path from the event
- the request: an exclusion for the folder `<folder>\engine\` (it keeps working across updates), rather than a checksum allowance that each update would undo
- that it talks only to the district's Canvas and to GitHub for updates, and stores the Canvas token in Windows Credential Manager

Setup can't continue on this computer until IT acts. Say so plainly, and offer to help with drafting in the meantime.

### Updates: stage quietly, apply with the teacher's yes

Run `.\cse.cmd version --json` first. When `install_kind` is `git`, playbooks, `AGENTS.md` and `CLAUDE.md` come from git, and `.\cse.cmd playbooks update` changes nothing (it reports `managed_by: git`). The Engine and the playbooks are released together, so an update moves both. Teachers will not ask for updates, so this runs without being asked and in two parts. The Engine does the mechanics; you decide when to ask.

**Stage** (silent, at the start of a session): `engine_info` operation `update-stage` (CLI: `.\cse.cmd update stage --if-due`). It checks at most once a day, and when a newer release exists it downloads and verifies the new Engine into `engine\staged\` without changing any live file. `state` is `current`, `skipped` (already checked today), `pending` or `staged`. If it fails or GitHub is blocked, say nothing and let the next day's check retry; never interrupt the teacher's work. `update-status` (CLI: `.\cse.cmd update status`) reports a waiting update without contacting GitHub.

**Apply** (changes live files) when `update-status` reports `pending` or `staged`:
- **At the end of the teacher's work**: "Can I install the latest CanvasExpert? It takes a moment." On yes, apply now.
- **Otherwise at the start of the next session**, apply it before any work and say so in one line: "I installed the latest CanvasExpert update."

Run `engine_info` operation `update-apply` (CLI: `.\cse.cmd update apply`). Read its `state`:
- `applied`: report in plain words, with no merge questions: "You're on the newest CanvasExpert; your settings and choices are unchanged." Mention a new teaching option only when it is worth their attention, in a sentence. Apps that were running keep the old Engine until they restart: when it is a good time, say "Close and reopen Claude (or ChatGPT) when you're between tasks so it picks up the update." Never ask during an active review.
- `held` because a review, setup or students page is open: leave it; try again when the teacher is done or at the next session.
- `held` with `edited` files: these are leftover edits to shipped files. For each, understand what the teacher wanted and add it as a clearly worded line in `Teacher_Override.md`, then run `update-apply` with `archive_edits: true` (CLI: `update apply --archive-edits`). The Engine copies the old files to `playbook-old/` and applies. Do this quietly; tell the teacher only what changed for them.
- `held` because it could not fast-forward: change nothing else and tell the teacher you need help with this update. Never force it.
- `repaired`: the Engine had drifted from this release and was reinstalled; nothing to tell the teacher.
- An error naming `get-engine.cmd`: the playbooks may already be updated; ask the teacher to close the apps using CanvasExpert, then run `.\get-engine.cmd` and check `engine_matches_manifest: true` in `.\cse.cmd version --json`.

Unsent drafts and results under `courses/` are never touched by an update.

The teacher's own choices live in `Teacher_Override.md`, `context/` and the other root guides they wrote, which git never touches. Do not edit `playbooks/`, `AGENTS.md`, `CLAUDE.md` or `TEACHER.md` in a git install; save a standing preference in `Teacher_Override.md` instead.

### ZIP fallback

Use this when the folder came from the release ZIP: no git on the computer, or the teacher downloaded it themselves. The folder is an extracted ZIP and `install_kind` is `zip` or empty (any value other than `git`). On every "set up", "install" or "update", run `.\cse.cmd playbooks update` (MCP: `engine_info` operation `update-playbooks`) before anything else. The ZIP carries the Engine and the bootstrap files (`AGENTS.md`, `CLAUDE.md`, `README.md`, `TEACHING-ASSISTANT.md`, `cse.cmd`, `webui.cmd`); this command lays down the playbooks, `TEACHER.md`, `Teacher_Override.md` and the `context/` starter files from the Engine, and on an update adopts the new versions without overriding the teacher. It keeps each shipped version under `shipped/<version>/` for comparison; those copies are inactive, never current rules.

The report lists each file's state. `added`, `current`, `updated` and `teacher-owned` need nothing from you. For each `kept` file, reconcile it in the background: compare `shipped/<previous_shipped>/<path>` (when there is a previous version), the active file and `shipped/<current_shipped>/<path>`. Every difference between the active file and what shipped before is the teacher's choice; with no previous version, every difference is. Keep those choices, bring in new shipped improvements that don't conflict with them, and write the result to the active file. Where a new shipped default conflicts with a teacher choice, keep the teacher's. Never change `context/` or `Teacher_Override.md` during an update. Don't ask the teacher to merge anything.

To get a newer version, download the Windows ZIP from the latest [GitHub release](https://github.com/longhornteacher/canvasexpert-se/releases) (not GitHub's source-code ZIP) with the same `curl.exe` and `tar` commands as in "Get CanvasExpert onto the computer", extracting over this folder, then run the update above. It contains no playbooks, `context/` or other teacher files, but extracting does replace `AGENTS.md` and `CLAUDE.md`. Before extracting, copy those two files into `engine/pre-update-<date>/`; afterwards compare them with the backup and carry the teacher's own additions into the new files. If `engine/cse.exe` is in use, ask the teacher to close the apps using CanvasExpert first, and never during an active review.

If the teacher downloaded the ZIP in a browser, Windows marks its files as coming from the internet, and double-clicking `webui.cmd` or the desktop shortcut then shows a security warning every time. With the teacher's OK, clear that mark once before first use, as the file's Properties → Unblock does: `Get-ChildItem -LiteralPath "<folder>" -Recurse -File | Unblock-File`. Files fetched with curl and tar never carry it.

### Select the correct UserInstall

By default `.\cse.cmd install` connects every supported app whose configuration folder exists: Claude (`cowork`), ChatGPT/Codex (`work`) and Antigravity (`antigravity`). `.\cse.cmd install --inspect` shows the read-only hints it uses; they do not prove an app is installed, running or in a supported mode. Do not read whole client configuration files or private settings. All connected apps point at this same UserInstall, so they share one Canvas credential and course list.

Resolve this folder and its `engine/cse.exe`, not an existing CE-SE MCP connection, PATH executable or another teacher folder. An already-connected server may belong to a different UserInstall: it must not receive this teacher's setup or preferences.

### Run the helper

Run `.\cse.cmd install` yourself (add `--client work`, `--client cowork` or `--client antigravity` only to connect one app, for example when the teacher later installs another). This saves narrowly scoped connection settings, or prepares the Claude extension and tries to open it, checks the local Engine/MCP without Canvas writes, and starts the private WebUI independently of any destination restart. A repeat install reuses the same UserInstall's running WebUI. It needs no `codex` command. Do not add `--no-ui` for ordinary first installation. `--check-only` is a local diagnostic, not registration or proof of destination access.

Inspect the returned outcomes rather than assuming the whole operation succeeded. The helper's `install_root` identifies the folder this operation selected. If an existing CE-SE registration targets another UserInstall, explain the exact old and new folders to the teacher in plain words before using `--replace-existing`; that flag replaces the connection entry, not the old folder's files. Keep other servers/settings and all unsent work. Malformed client configuration stays intact: explain the helper's reported repair action without exposing unrelated configuration or secrets. An unavailable app does not prevent preparing the other selected route.

For Claude, read `cowork_launch_outcome`: `launch_requested` means Claude was asked to open the file (use the first Claude line above); `file_revealed` and `launch_failed` (and `prepared`, which happens only with `--no-ui`) mean the file is ready at `extension_path` (use the second Claude line, adding any error the helper reported). None of these means the extension is installed, trusted or connected. Report organizational restrictions or trust prompts as the app reports them; do not promise to bypass them. The extension launches this UserInstall's Engine; the teacher does not maintain a second Engine.

The WebUI: always give the helper's actual local form address as a fallback, since a browser launch was only requested, not seen. If the helper returns a `ui_warning`, the page may not have started: tell the teacher to double-click `webui.cmd`, and do not invent an address. A failed WebUI start does not undo saved connection settings or a prepared extension; report them separately. Never fetch the private form or receive the token in chat, a command, a file or client configuration. Inspect only non-secret setup status. Keep a working saved credential on repeat installation.

A rejected or missing token needs a new one in the form. A timeout, DNS, proxy, district restriction or outage is a connectivity problem: keep the token. Teachers whose tokens expire (a 90-day district limit is common) will meet the rejected-token case periodically; handle it as above without redoing courses or settings. `setup.md` covers courses and the technical detail.

UserSync is optional. Make the brief offer above once, preserve an existing binding, and configure a dedicated directory only when the teacher supplies one.

### Check each state honestly

Distinguish: the helper ran; connection settings saved or extension file prepared; local check passed; WebUI started; destination still needs an action; destination tools actually used. Saved settings and local checks are not connected tools. Loaded adapter identity and on-disk Engine identity are separate; if they differ, the destination must reconnect after active review work finishes.

After reconnect, the destination assistant reads current `AGENTS.md`, `context/teaching-profile.md`, `context/grading.md`, the relevant playbook and configured `list_courses`, and its returned `install_root` must match the helper's `install_root` and this folder before teacher work. If they differ, stop and connect the intended UserInstall. Do not claim a Teaching Assistant Project was created when only connection configuration was saved. Use a supported app action if available; otherwise give the starter message in `TEACHING-ASSISTANT.md`.

## Use the Engine through MCP or its launcher

Playbooks show the CLI form `.\cse.cmd <command>`; run it from UserInstall. When CE-SE MCP is connected, use its typed tools instead: `engine_reference` for command help and exact formats/examples/field behavior; `list_courses`, `pull_work`, `pull_assignment` and `pull_page` for discovery and pulls; `read_workspace_file` / `write_workspace_file` and `write_draft` for files; `validate_document` / `validate_quiz` for local validation; `start_review` / `review_status` for the review page; `push_document`, `push_quiz`, `post_results`, `upload_file`, and quiz-point tools for the corresponding playbook operations; `engine_info` for version, diagnostics and teacher-rules status. Use `list_workspace_files` to find files. The playbook's requested scope and review rules still apply.

Read-only `canvas_get` exposes raw Canvas JSON and can contain real names; prefer the task's pseudonymized pull when it provides the needed evidence. Workspace text reads/writes are limited to 1 MiB; base64 asset reads/writes to 20 MiB decoded. Larger assets already staged under an allowed workspace folder can use `upload_file`. File tools block hidden/private/Engine paths and links beneath the workspace; existing files need their current SHA-256 before replacement. Never use another tool to bypass those boundaries. Setup and student editors are private: give the teacher the returned browser URL and check session status only; never fetch their contents.

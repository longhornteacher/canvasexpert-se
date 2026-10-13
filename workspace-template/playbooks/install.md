# Install this for the teacher

Use this when the teacher unzips CanvasExpert-SE, opens the folder in Code/Codex and says "install this", or asks to connect Claude or ChatGPT. Routine Canvas/course work after setup is in `setup.md`. The extracted folder is **Install**; no Source checkout, developer runtime, separate Codex CLI or teacher terminal command is needed.

## How to talk to the teacher

The teacher is a classroom teacher, not a developer. Be brisk and friendly, in the first person, one step at a time, and wait for them before moving on. Say what is coming before each step and say plainly when it is done. Never say MCP, extension, trust prompt, PAT, origin, Install, Engine, Credential Manager, registration or adapter. The one term they must meet is Canvas's own button label, **New Access Token**: explain it the first time. Do not give time estimates. Never ask for the token in chat; it goes only in the private form. Do not read out the technical checklist further down; say only what the teacher needs to do next. Ask for preferences, the school calendar and similar details later, when a task needs them, not during first setup.

The lines below are the script. Adapt the wording to the situation; keep the plain language.

### 1. Opening

If the teacher has not said which app they use:

> "Happy to set this up. It takes three short things from you: tell me which app you use, approve one pop-up in that app, and paste a Canvas code into a private form. I'll do the rest. Are you using Claude, ChatGPT, or both?"

If they already said (including "for Cowork" or "for Work"), do not ask again: "Great, I'll set it up for Claude" (or ChatGPT, or both).

If they only use the website:

> "Thanks, that matters. CanvasExpert works with the Claude or ChatGPT app installed on this computer, not their websites. If you can install the app, I'll wait while you do. If not, we can't connect it from here."

Never install an app or subscription for them.

### 2. Connecting the app

> "I'm connecting {Claude / ChatGPT} now."

Then run the helper (see "Run the helper" below) and tell the teacher what to do based on the result.

**Claude, app opened with the file:**

> "Claude should be asking whether to add CanvasExpert. Please click to approve it. If you don't see anything, tell me and I'll walk you through adding it by hand."

**Claude, file shown in a folder window (or the app could not be opened):**

> "I've put a file called CanvasExpert-SE-Cowork in a folder window. In Claude, open Settings, find Extensions, and add that file. Tell me when it's in."

**ChatGPT:**

> "ChatGPT is set up. When we're done here, close it and open it again so it notices. I'll tell you when."

If the teacher chose both, do both and report each separately. Never connect an app they did not choose. Do not ask anyone to close an app during an active CE-SE review.

### 3. The Canvas form

> "Now I need CanvasExpert to be able to see your courses. Canvas lets you make a private key for that. It's called an **access token**: a key that lets this program read your courses. You can delete it in Canvas any time, and it never leaves this computer.
>
> A form is opening in your browser."

If the page did not open, give the address the helper returned (never invent one) or say: "Double-click **webui.cmd** in your CanvasExpert folder."

> "In the form you'll type your **Canvas address**. That's the web address you see when you're signed into Canvas, like `yourschool.instructure.com`. Then paste your key. Here's how to make one:
> 1. Sign in to Canvas.
> 2. Click **Account**, then **Settings**.
> 3. Scroll to **Approved Integrations** and click **+ New Access Token**.
> 4. For the purpose, type CanvasExpert. If your school limits how long a token can last (many do, often 90 days), choose the longest allowed.
> 5. Click Generate and **copy the long code Canvas shows you.** It only shows it once.
> 6. Paste it into the form and click **Connect**.
>
> Please don't paste the code here in our chat. Tell me when you've clicked Connect."

If the teacher says their school limits token life, add: "Canvas will stop accepting this key when it expires. When that happens, I'll tell you and we'll make a new one the same way. Nothing else needs redoing."

### 4. After the form

- **It worked:** "Connected. I can see your Canvas account."
- **Canvas says the code is wrong, missing or expired:** "Canvas didn't accept that code. A missing character when copying is the usual cause, and codes also expire. Making a new one is quick: Account, Settings, + New Access Token, then paste it into the same form." Keep their saved courses and settings.
- **Could not reach Canvas** (timeout, network, school filter, Canvas outage): "I couldn't reach Canvas just now. It may be your school's network or Canvas itself. Your code is saved, so there's nothing to redo. We can try again in a few minutes, and I can still help you draft in the meantime." Do not tell them to replace the token.
- **Two different Canvas addresses:** "I see two different Canvas addresses: {A} and {B}. Which one do you sign in with?" Use their answer.
- **Not sure why it failed** (for example a permissions error on one course): say so plainly; do not assume the token is bad.

### 5. Choosing courses

> "Which of these are you teaching right now?" (list the courses Canvas returns, likely current ones first, using the course ID and state to tell similar names apart) "For each one, what short name do you want me to use? For example 'Period 3 English' or 'ELA 7'."

Never invent nicknames. Then: "Saved. You can change these any time." Write the choices as `setup.md` describes.

### 6. Finishing up

> "CanvasExpert is ready on this computer. One last step so {Claude / ChatGPT} can use it: {approve the pop-up / close it and open it again}. Then start a new chat there and paste this:"

Give the starter message from `TEACHING-ASSISTANT.md`. After they say they have done it:

> "Great. Ask it to show you your courses. If it lists {their course nicknames}, you're all set. If not, tell me what it says."

If you set up both apps, check each one separately; each counts as connected only after its assistant lists the right courses.

If anything is still pending, name only the one real next action: "Claude is set up and waiting for you to approve its prompt; ChatGPT needs to be closed and reopened after your review finishes."

## What you do behind the scenes

Do not narrate this to the teacher.

### Select the destination and the correct Install

Internally Claude on this PC uses `--client cowork` and ChatGPT's local desktop app uses `--client work`. Decide from the app inventory and context you can actually see; `.\cse.cmd install --inspect` gives read-only configuration-directory hints, which do not prove an app is installed, running or in a supported mode. Do not read whole client configuration files or private settings. If the evidence is not enough, ask one concrete follow-up: "Do you use the app installed on this PC, or only the website?"

Resolve this extracted folder and its bundled `engine/cse.exe`, not an existing CE-SE MCP connection, PATH executable or another teacher folder. Run installation locally from here. An already-connected server may belong to a different Install: it must not receive this teacher's setup or preferences.

### Run the helper

Run `.\cse.cmd install --client work` (ChatGPT) and/or `.\cse.cmd install --client cowork` (Claude) yourself. This saves narrowly scoped connection settings, or prepares the Claude extension and tries to open it, checks the local Engine/MCP without Canvas writes, and starts the private WebUI independently of any destination restart. A repeat install reuses the same Install's running WebUI. It needs no `codex` command. Do not add `--no-ui` for ordinary first installation. `--check-only` is a local diagnostic, not registration or proof of destination access.

Inspect the returned outcomes rather than assuming the whole operation succeeded. The helper's `install_root` identifies the extracted folder this operation selected. If an existing CE-SE registration targets another Install, explain the exact old and new folders to the teacher in plain words before using `--replace-existing`; that flag replaces the connection entry, not the old folder's files. Keep other servers/settings and all unsent work. Malformed client configuration stays intact: explain the helper's reported repair action without exposing unrelated configuration or secrets. An unavailable app does not prevent preparing the other selected route.

For Claude, read `cowork_launch_outcome`: `launch_requested` means Claude was asked to open the file (use the first Claude line above); `file_revealed`, `launch_failed` and `prepared` mean the file is ready at `extension_path` (use the second Claude line, adding any error the helper reported). None of these means the extension is installed, trusted or connected. Report organizational restrictions or trust prompts as the app reports them; do not promise to bypass them. The extension launches this Install's Engine; the teacher does not maintain a second Engine.

The WebUI: always give the helper's actual local form address as a fallback, since a browser launch was only requested, not seen. If the helper returns a `ui_warning`, the page may not have started: tell the teacher to double-click `webui.cmd`, and do not invent an address. A failed WebUI start does not undo saved connection settings or a prepared extension; report them separately. Never fetch the private form or receive the token in chat, a command, a file or client configuration. Inspect only non-secret setup status. Keep a working saved credential on repeat installation.

A rejected or missing token needs a new one in the form. A timeout, DNS, proxy, district restriction or outage is a connectivity problem: keep the token. Teachers whose tokens expire (a 90-day district limit is common) will meet the rejected-token case periodically; handle it as above without redoing courses or settings. `setup.md` covers courses and the technical detail.

Sync is optional and needs no first-run questionnaire. Configure a dedicated Sync directory only when the teacher supplies one.

### Check each state honestly

Distinguish: the helper ran; connection settings saved or extension file prepared; local check passed; WebUI started; destination still needs an action; destination tools actually used. Saved settings and local checks are not connected tools. Loaded adapter identity and on-disk Engine identity are separate; if they differ, the destination must reconnect after active review work finishes.

After reconnect, the destination assistant reads current `AGENTS.md`, `context/teaching-profile.md`, `context/grading.md`, the relevant playbook and configured `list_courses`, and its returned `install_root` must match the helper's `install_root` and the extracted folder before teacher work. If they differ, stop and connect the intended Install. Do not claim a Teaching Assistant Project was created when only connection configuration was saved. Use a supported app action if available; otherwise give the starter message in `TEACHING-ASSISTANT.md`.

## Use the Engine through MCP or its launcher

Playbooks show the CLI form `.\cse.cmd <command>`; run it from Install. When CE-SE MCP is connected, use its typed tools instead: `engine_reference` for command help and exact formats/examples/field behavior; `list_courses`, `pull_work`, `pull_assignment` and `pull_page` for discovery and pulls; `read_workspace_file` / `write_workspace_file` and `write_draft` for files; `validate_document` / `validate_quiz` for local validation; `start_review` / `review_status` for the review page; `push_document`, `push_quiz`, `post_results`, `upload_file`, and quiz-point tools for the corresponding playbook operations; `engine_info` for version, diagnostics and teacher-rules status. Use `list_workspace_files` to find files. The playbook's requested scope and review rules still apply.

Read-only `canvas_get` exposes raw Canvas JSON and can contain real names; prefer the task's pseudonymized pull when it provides the needed evidence. Workspace text reads/writes are limited to 1 MiB; base64 asset reads/writes to 20 MiB decoded. Larger assets already staged under an allowed workspace folder can use `upload_file`. File tools block hidden/private/Engine paths and links beneath the workspace; existing files need their current SHA-256 before replacement. Never use another tool to bypass those boundaries. Setup and student editors are private: give the teacher the returned browser URL and check session status only; never fetch their contents.

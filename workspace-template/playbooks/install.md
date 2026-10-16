# Set this all up for the teacher

Use this when the teacher unzips CanvasExpert-SE, opens the folder in a coding agent (Claude Code, Codex, Antigravity) and says "set this all up," "set this up," "get me ready to teach," or "install this", or asks to connect Claude, ChatGPT or Antigravity. For full first-run setup, read both this playbook and `setup.md`. The extracted folder is already **UserInstall**. Run directly here; setup connects apps and Canvas without copying the product into a second folder; no Dev checkout, developer runtime, separate Codex CLI or teacher terminal command is needed.

## Own the whole setup

A broad setup request means getting the teacher ready to work, not merely running the connection helper. Keep the existing install.md filename and cse install command as internal compatibility names; say "set up" to the teacher. A request only to connect one app stays limited to that app.

Do not add CE-SE, Go, Node, PowerShell or an agent CLI to PATH for setup. Run this folder's `cse.cmd` by its explicit path, or its `engine/cse.exe` directly. The helper registers absolute Engine paths for Work/Codex and Antigravity; Cowork's packaged launcher targets this same folder. No compiler, package manager or global command installation is required. If an agent cannot invoke a bare command, use the full path instead of changing PATH. Moving UserInstall afterward requires reconnecting the apps to its new location.

Before changing anything, inspect this folder's saved rules location and configured courses through its bundled Engine. Preserve completed setup, saved course nicknames and preferences. Offer shared setup once: "If you already keep your CanvasExpert setup in OneDrive or Google Drive, I can use that folder here too. Otherwise we can keep everything on this computer." If the teacher supplies a directory, follow setup.md to select it and reread saved courses/preferences before configuring new ones. Do not require Sync or a preferences questionnaire.

Finish the applicable steps: app connections prepared and remaining prompts explained; Canvas connection checked; requested courses saved with the teacher's nicknames; optional shared setup handled if requested; intended app actually reading this folder's saved courses. A local helper check alone does not mean an app is ready. If one step is blocked, continue independent setup and name the specific unfinished step. Then help with the teacher's original task or offer a first teaching task.

## How to talk to the teacher

The teacher is a classroom teacher, not a developer. Be brisk and friendly, in the first person, one step at a time. Continue authorized setup yourself; wait only when you need a teacher choice, private form entry or app approval. Do not ask permission to continue after each routine step. Say what is coming before each step and say plainly when it is done. Never say MCP, extension, trust prompt, PAT, origin, UserInstall, Engine, registration or adapter. The one term they must meet is Canvas's own button label, **New Access Token**: explain it the first time. Do not give time estimates. Never ask for the token in chat; it goes only in the private form. Do not read out the technical checklist further down; say only what the teacher needs to do next. Ask for preferences, the school calendar and similar details later, when a task needs them, not during first setup.

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

> "I found {Claude, ChatGPT, Antigravity} on your computer and prepared {it / them} to use CanvasExpert! CanvasExpert can also work in {the ones not found}, so if you ever install one, just come back here and I'll connect it too."

Then give what each app still needs:

**Claude, app opened with the file:**

> "Claude should be asking whether to add CanvasExpert. Please click to approve it. If you don't see anything, tell me and I'll walk you through adding it by hand."

**Claude, file shown in a folder window (or the app could not be opened):**

> "I've put a file called CanvasExpert-SE-Cowork in a folder window. In Claude, open Settings, find Extensions, and add that file. Tell me when it's in."

**ChatGPT or Antigravity:**

> "{App} is set up. When we're done here, close it and open it again so it notices. I'll tell you when."

Do not ask anyone to close an app during an active CE-SE review. If nothing supported was found, say so and ask which app they plan to use.

### 3. The Canvas form

> "Now I need CanvasExpert to be able to see your courses. Canvas lets you make a private key for that. It's called an **access token**: a key that lets CanvasExpert access your courses and carry out the Canvas work you request. You can delete it in Canvas any time. It is saved in Windows Credential Manager on this computer and sent only to your Canvas site when making requests, never to the assistant in chat.
>
> **My safety promise:** Setting this up does not publish coursework or change grades. When we work, I follow the scope you request. Scores, new assignments and quizzes, and assignment edits normally go through a private review page; you can explicitly skip review for a task.
>
> A form is opening in your browser."

If the page did not open, give the address the helper returned (never invent one) or say: "Double-click **webui.cmd** in your CanvasExpert folder."

> "In the form you'll type your **Canvas address**. That's the web address you see when you're signed into Canvas, like `yourschool.instructure.com`. Then paste your key. Here's how to make one:
> 1. Sign in to Canvas.
> 2. Click **Account**, then **Settings**.
> 3. Scroll to **Approved Integrations** and click **+ New Access Token**. (Once you tell me your Canvas address I can give you a direct link: `https://<your-address>/profile/settings#access_tokens`.)
> 4. For the purpose, type CanvasExpert. For the expiration, choose the end of the school year, or the longest your school allows, so you don't have to redo this mid-semester.
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

Read saved courses first and preserve existing selections and nicknames. If courses are missing, read the Canvas list and use course IDs and states to distinguish similarly named sections. Ask which classes the teacher wants and what short names they use; do not invent nicknames from Canvas titles.

> "Here are your Canvas classes. Which ones are you teaching now, and what short name do you use for each?"

Skip this question when the requested courses are already configured.

Save what they confirm as `setup.md` describes: "Saved. You can change these any time."

### 6. Finishing up

Only ask for a restart, approval or new chat when that app still needs it. If the current assistant already has working CE-SE tools for this folder, verify the saved courses here and continue teaching.

> "Canvas and your classes are set up. To finish connecting {the app(s)}, {approve the pop-up / close it and open it again}. Then start a new chat there and paste this:"

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

### Select the correct UserInstall

By default `.\cse.cmd install` connects every supported app whose configuration folder exists: Claude (`cowork`), ChatGPT/Codex (`work`) and Antigravity (`antigravity`). `.\cse.cmd install --inspect` shows the read-only hints it uses; they do not prove an app is installed, running or in a supported mode. Do not read whole client configuration files or private settings. All connected apps point at this same UserInstall, so they share one Canvas credential and course list.

Resolve this extracted folder and its bundled `engine/cse.exe`, not an existing CE-SE MCP connection, PATH executable or another teacher folder. An already-connected server may belong to a different UserInstall: it must not receive this teacher's setup or preferences.

### Run the helper

Run `.\cse.cmd install` yourself (add `--client work`, `--client cowork` or `--client antigravity` only to connect one app, for example when the teacher later installs another). This saves narrowly scoped connection settings, or prepares the Claude extension and tries to open it, checks the local Engine/MCP without Canvas writes, and starts the private WebUI independently of any destination restart. A repeat install reuses the same UserInstall's running WebUI. It needs no `codex` command. Do not add `--no-ui` for ordinary first installation. `--check-only` is a local diagnostic, not registration or proof of destination access.

Inspect the returned outcomes rather than assuming the whole operation succeeded. The helper's `install_root` identifies the extracted folder this operation selected. If an existing CE-SE registration targets another UserInstall, explain the exact old and new folders to the teacher in plain words before using `--replace-existing`; that flag replaces the connection entry, not the old folder's files. Keep other servers/settings and all unsent work. Malformed client configuration stays intact: explain the helper's reported repair action without exposing unrelated configuration or secrets. An unavailable app does not prevent preparing the other selected route.

For Claude, read `cowork_launch_outcome`: `launch_requested` means Claude was asked to open the file (use the first Claude line above); `file_revealed`, `launch_failed` and `prepared` mean the file is ready at `extension_path` (use the second Claude line, adding any error the helper reported). None of these means the extension is installed, trusted or connected. Report organizational restrictions or trust prompts as the app reports them; do not promise to bypass them. The extension launches this UserInstall's Engine; the teacher does not maintain a second Engine.

The WebUI: always give the helper's actual local form address as a fallback, since a browser launch was only requested, not seen. If the helper returns a `ui_warning`, the page may not have started: tell the teacher to double-click `webui.cmd`, and do not invent an address. A failed WebUI start does not undo saved connection settings or a prepared extension; report them separately. Never fetch the private form or receive the token in chat, a command, a file or client configuration. Inspect only non-secret setup status. Keep a working saved credential on repeat installation.

A rejected or missing token needs a new one in the form. A timeout, DNS, proxy, district restriction or outage is a connectivity problem: keep the token. Teachers whose tokens expire (a 90-day district limit is common) will meet the rejected-token case periodically; handle it as above without redoing courses or settings. `setup.md` covers courses and the technical detail.

UserSync is optional. Make the brief offer above once, preserve an existing binding, and configure a dedicated directory only when the teacher supplies one.

### Check each state honestly

Distinguish: the helper ran; connection settings saved or extension file prepared; local check passed; WebUI started; destination still needs an action; destination tools actually used. Saved settings and local checks are not connected tools. Loaded adapter identity and on-disk Engine identity are separate; if they differ, the destination must reconnect after active review work finishes.

After reconnect, the destination assistant reads current `AGENTS.md`, `context/teaching-profile.md`, `context/grading.md`, the relevant playbook and configured `list_courses`, and its returned `install_root` must match the helper's `install_root` and the extracted folder before teacher work. If they differ, stop and connect the intended UserInstall. Do not claim a Teaching Assistant Project was created when only connection configuration was saved. Use a supported app action if available; otherwise give the starter message in `TEACHING-ASSISTANT.md`.

## Use the Engine through MCP or its launcher

Playbooks show the CLI form `.\cse.cmd <command>`; run it from UserInstall. When CE-SE MCP is connected, use its typed tools instead: `engine_reference` for command help and exact formats/examples/field behavior; `list_courses`, `pull_work`, `pull_assignment` and `pull_page` for discovery and pulls; `read_workspace_file` / `write_workspace_file` and `write_draft` for files; `validate_document` / `validate_quiz` for local validation; `start_review` / `review_status` for the review page; `push_document`, `push_quiz`, `post_results`, `upload_file`, and quiz-point tools for the corresponding playbook operations; `engine_info` for version, diagnostics and teacher-rules status. Use `list_workspace_files` to find files. The playbook's requested scope and review rules still apply.

Read-only `canvas_get` exposes raw Canvas JSON and can contain real names; prefer the task's pseudonymized pull when it provides the needed evidence. Workspace text reads/writes are limited to 1 MiB; base64 asset reads/writes to 20 MiB decoded. Larger assets already staged under an allowed workspace folder can use `upload_file`. File tools block hidden/private/Engine paths and links beneath the workspace; existing files need their current SHA-256 before replacement. Never use another tool to bypass those boundaries. Setup and student editors are private: give the teacher the returned browser URL and check session status only; never fetch their contents.

# CanvasExpert-SE

**1.0 beta · `1.0.0-beta.1` · Windows 11**

This folder is your local CanvasExpert-SE **UserInstall**. An AI agent (Claude or ChatGPT, for example) does the work here: scoring student work and creating or editing Canvas assignments and quizzes. Review is the default for scoring, new assignments, new quizzes and assignment edits; you can explicitly skip it for a task.

## Start here

1. Unzip CanvasExpert-SE into a folder on your computer's local drive.
2. Open that folder in Code/Codex and say **"Set this all up."** The agent connects every supported app it finds on your computer (Claude, ChatGPT, Antigravity), opens the private Canvas form, helps configure your classes and checks that your teaching assistant can read them.
3. Enter your Canvas address and access token in the private form. Approve the install prompt in your app (restart it only if it asks and you are not mid-review), then use the short message in `TEACHING-ASSISTANT.md` to start teaching there.

The folder you extracted is already this machine's **UserInstall**. Run directly here; setup connects apps and Canvas without copying the program into a second folder. These role names do not require you to rename any folders. Its actual path appears in the hub's Active locations summary and in the install helper's `install_root` result. After the destination reconnects, confirm its CE-SE course metadata reports that same UserInstall before asking it to work.

You do not need terminal commands, client settings edits, Dev or a separate Codex CLI. Your token is saved in Windows Credential Manager, never in a file or chat. The agent distinguishes a local check from a successful connection in your destination app. Claude may ask you to approve the CE-SE extension, or the agent may show you a file to install from Claude's extension settings. The ChatGPT connection is for the desktop app on this PC; a website alone cannot reach your UserInstall. School or organization restrictions can add steps.

If the Canvas form did not open, the agent will show you its local address, or you can double-click `webui.cmd`. Use it any time to open the hub for courses, students, feedback voice and capabilities. Tell the assistant about your teaching preferences when useful, or ask it to carry over existing instructions you choose; this is optional and grows as you work.

To share teacher rules across PCs, choose a dedicated UserSync directory in Settings → Teacher rules, or tell the agent its path. An empty directory starts with your current customized rules. An existing shared directory becomes authoritative and your differing local originals are preserved. After binding existing UserSync, review Settings → Your playbooks and context (or `cse playbooks status` / `engine_info` `playbooks-status`) for seed-managed differences. Choose any files to adopt explicitly; previous versions are archived. Teacher preferences are never replaced automatically. Configure the corresponding provider directory separately on each PC. Course choices, student notes, aliases/settings and private roster/contact records also use UserSync; private records remain outside agent context. Grading/planning session files and student work stay local. Returning to UserInstall preserves readable shared rules locally and archives differing local versions; UserSync stays intact. If UserSync is unavailable, CE-SE explains the problem instead of silently using old local rules.

## Three clear names

- **Dev**: the development project, including code, starter templates and build/release artifacts such as `dist/`. Teachers do not need it.
- **UserInstall**: the extracted local CE-SE folder, with the Engine, drafts, pulled work and local runtime state. All reusable teacher setup lives here by default.
- **UserSync**: an optional dedicated locally accessible OneDrive/Google Drive directory for reusable teacher setup: rules, playbooks, guides, preferences, course selections, student aliases/settings/notes and private roster/contact records. Each UserInstall reads and edits these files directly; the provider synchronizes them.

Code/Codex use Dev to develop CE-SE and UserInstall for teacher work. Your Claude or ChatGPT app normally uses UserInstall. MCP connects agents to UserInstall's Engine; it does not require an extra UserInstall. Agent tools call UserInstall the "workspace". Each connected Engine reports the actual UserInstall path; there is no required universal folder path.

## What's in this folder

- `AGENTS.md`, `CLAUDE.md` and `playbooks/`: how the agent does each job. They're yours to change.
- `context/`: your teaching profile, preferences and facts (courses, feedback voice, student notes, school calendar).
- `TEACHING-ASSISTANT.md`: a short starter for your destination assistant; current teacher files remain authoritative.
- `engine/`: the program that talks to Canvas. `cse.cmd` runs it for the agent.
- `courses/`: your pulled work and drafts, created as you go.
- `.cse/`: private runtime state. Student mappings, roster and contacts use the active setup authority (UserInstall or UserSync); the agent must not read either private directory.

New program versions add missing starter files at their active location but never replace your existing rules. To get the newest version of a playbook, use "Your playbooks and context" in the hub's Settings; the replaced version is archived. Context preferences are protected from automatic replacement.

New Quizzes support regular authoring, use and written-response scoring, including essays and file uploads. CE-SE provides criterion scores and feedback in a submission comment; you enter written-item points in SpeedGrader. Ask your agent to follow the current quiz and scoring playbooks for the limits.

## Keep this folder on your computer

Keep this folder on your computer's own drive, not in OneDrive or another synced folder: many districts block syncing programs, so the `engine` folder would not arrive on your other computers. To use another computer, unzip CanvasExpert-SE there too and ask the agent to set it up.

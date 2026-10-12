# CanvasExpert-SE

This folder is your local CanvasExpert-SE **Install**. An AI agent (Claude or ChatGPT, for example) does the work here: scoring student work and creating or editing Canvas assignments and quizzes. Review is the default for scoring, new assignments, new quizzes and assignment edits; you can explicitly skip it for a task.

## Three clear names

- **Source**: the development project, including code, starter templates and build/release artifacts such as `dist/`. Teachers do not need it.
- **Install**: the extracted local CE-SE folder, with the Engine, local course/student configuration, drafts, pulled work and private state. By default your teacher rules live here too.
- **Sync**: an optional dedicated locally accessible OneDrive/Google Drive directory for shared teacher rules, playbooks, guides and preferences. Each Install reads and edits these files directly; the provider synchronizes them.

Claude Code/Codex use Source to develop CE-SE and Install for teacher work. Claude Cowork/GPT Work normally use Install. MCP connects agents to Install's Engine; it does not require an extra Install. Agent tools call Install the "workspace".

## Start here

1. Connect CE-SE to your agent through MCP, or open Install as the project in a local agent app.
2. Tell the agent what you'd like to do, for example "set me up" or "start a scoring session". The first time, it connects to Canvas with you. You'll paste a Canvas access token into a small local form; it's saved in Windows Credential Manager, never in a file.

That's it. You don't need to edit anything here yourself; ask the agent instead. Double-click `webui.cmd` any time to open the hub, where you can see your courses, students, feedback voice and what CanvasExpert-SE can do.

To share teacher rules across PCs, choose a dedicated Sync directory in Settings → Teacher rules, or tell the agent its path. An empty directory starts with your current customized rules. An existing shared directory becomes authoritative and your differing local originals are preserved. Configure the corresponding provider directory separately on each PC. Course choices, student notes, private identities and student work stay local. Returning to Install preserves readable shared rules locally and archives differing local versions; Sync stays intact. If Sync is unavailable, CE-SE explains the problem instead of silently using old local rules.

## What's in this folder

- `AGENTS.md`, `CLAUDE.md` and `playbooks/`: how the agent does each job. They're yours to change.
- `context/`: your preferences and facts (courses, feedback voice, student notes, school calendar).
- `engine/`: the program that talks to Canvas. `cse.cmd` runs it for the agent.
- `courses/`: your pulled work and drafts, created as you go.
- `.cse/`: private student information. The agent is told not to read it.

New program versions add missing starter files at their active location but never replace your existing rules. To get the newest version of a playbook, use "Your playbooks and context" in the hub's Settings; the replaced version is archived. Context preferences are protected from automatic replacement.

New Quizzes support regular authoring, use and written-response scoring, including essays and file uploads. CE-SE provides criterion scores and feedback in a submission comment; you enter written-item points in SpeedGrader. Ask your agent to follow the current quiz and scoring playbooks for the limits.

## Keep this folder on your computer

Keep this folder on your computer's own drive, not in OneDrive or another synced folder: many districts block syncing programs, so the `engine` folder would not arrive on your other computers. To use another computer, unzip CanvasExpert-SE there too and ask the agent to set it up.

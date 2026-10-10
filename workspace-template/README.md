# CanvasExpert-SE

This folder is your CanvasExpert-SE workspace. An AI agent (Claude or ChatGPT, for example) does the work here: scoring student work and creating or editing Canvas assignments and quizzes. You review before anything is posted.

## Start here

1. Open this folder in your agent app (in Claude Code or Codex, open it as the project folder).
2. Tell the agent what you'd like to do, for example "set me up" or "start a scoring session". The first time, it connects to Canvas with you. You'll paste a Canvas access token into a small local form; it's saved in Windows Credential Manager, never in a file.

That's it. You don't need to edit anything here yourself; ask the agent instead. Double-click `webui.cmd` any time to open the hub, where you can see your courses, students, feedback voice and what CanvasExpert-SE can do.

## What's in this folder

- `AGENTS.md`, `CLAUDE.md` and `playbooks/`: how the agent does each job. They're yours to change.
- `context/`: your preferences and facts (courses, feedback voice, student notes, school calendar).
- `engine/`: the program that talks to Canvas. `cse.cmd` runs it for the agent.
- `courses/`: your pulled work and drafts, created as you go.
- `.cse/`: private student information. The agent is told not to read it.

New program versions add new files but never change yours. To get the newest version of a playbook, use "Your playbooks and context" in the hub's Settings.

## Keeping this folder in OneDrive or another synced folder

That's fine. If you'd rather keep the program out of the synced folder (for example, so each computer keeps its own copy), delete `engine/`, unzip the separate program download somewhere else on each computer, and double-click its `webui.cmd` once; `cse.cmd` then finds that program instead.

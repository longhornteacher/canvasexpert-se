# CanvasExpert-SE starter workspace

This folder is a clean starting point for a teacher's CanvasExpert-SE workspace. Use the shipped playbooks for setup, authoring, editing, and scoring; use `cse.cmd --help` when you need the exact command syntax for the installed build.

## Start here

1. Open this folder as the workspace in Codex or Claude Code.
2. Ask the agent to read `AGENTS.md` and `playbooks/setup.md`.
3. In a terminal opened at this folder, inspect the Engine with `.\cse.cmd --help`, for example `.\cse.cmd setup`. `cse.cmd` runs this computer's CanvasExpert-SE program against this workspace. Before first use, double-click `webui.cmd` in the separate program folder once so this computer knows where the program is.
4. Use setup to save the Canvas credential on this machine and configure the workspace. Then review the task's playbook before authoring or scoring.

Do not put a Canvas personal access token in a file, command line, prompt, or workspace note. Setup opens a local form and saves the credential through Windows Credential Manager.

The Engine is the program that talks to Canvas. Your playbooks (`playbooks/`) are how the agent does each task, and your context (`context/`) is your preferences and facts. Both started as a copy of the program's starter files and are yours to change.

## Folders

Two folders: the program folder (the Engine plus the starter playbooks; nothing private) and this workspace (your playbooks, context and student information).

New program versions add new files but never change yours. To get the newest version of a playbook, use Your playbooks and context in the hub's Settings, or delete it and reopen the hub.

- `AGENTS.md`, `CLAUDE.md` and `playbooks/` are the agent's instructions: short instructions for setup, authoring, editing, and scoring.
- `context/` is your context. `context/courses.md` contains the Canvas instance and configured course nicknames.
- `context/grading.md` holds teacher feedback preferences and grading context.
- `context/students.md` is for teacher-authored accommodations and student notes. Keep this starter copy empty of real student information.
- `context/school-calendar.md` is reference data the agent uses to work out school-day lateness; update it for the school's calendar.
- `.cse/` is reserved for private runtime state such as the student mapping. Do not inspect it as an agent.
- `cse.cmd` finds this computer's linked program. The program itself is deliberately not in this synced folder.
- `courses/` is where the Engine stores course work. It is populated as supported operations are run.

The course configuration and task document formats come from the Engine: run `.\cse.cmd help formats` for the reference and `.\cse.cmd help examples` for samples. Do not invent fields outside that reference and the CLI contract. Canvas Live is authoritative; where a specific Canvas behavior remains unresolved, the affected action's guide explains the limitation.

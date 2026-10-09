# CanvasExpert-SE starter workspace

This folder is a clean starting point for a teacher's CanvasExpert-SE workspace. Use the shipped guides for setup, authoring, editing, and scoring; use `cse.cmd --help` when you need the exact command syntax for the installed build.

## Start here

1. Open this folder as the workspace in Codex or Claude Code.
2. Ask the agent to read `AGENTS.md` and `guides/setup.md`.
3. In a terminal opened at this folder, inspect the CLI with `.\cse.cmd --help`, for example `.\cse.cmd setup`. `cse.cmd` runs this computer's CanvasExpert-SE program against this workspace. Before first use, double-click `webui.cmd` in the separate program folder once so this computer knows where the program is.
4. Use setup to save the Canvas credential on this machine and configure the workspace. Then review the task-specific guide before authoring or scoring.

Do not put a Canvas personal access token in a file, command line, prompt, or workspace note. Setup opens a local form and saves the credential through Windows Credential Manager.

## Folders

- `guides/` contains short instructions for setup, authoring, editing, and scoring.
- `settings/courses.md` contains the Canvas instance and configured course nicknames.
- `settings/grading.md` holds teacher feedback preferences and grading context.
- `settings/students.md` is for teacher-authored accommodations and student notes. Keep this starter copy empty of real student information.
- `settings/school-calendar.md` defines student dates used to describe elapsed lateness; update it for the school's calendar.
- `.cse/` is reserved for private runtime state such as the student mapping. Do not inspect it as an agent.
- `cse.cmd` finds this computer's linked program. The program itself is deliberately not in this synced folder.
- `courses/` is where the CLI stores course work. It is populated as supported operations are run.

The course configuration and task document formats are described in `guides/formats.md`. Do not invent fields outside that guide and the corresponding CLI contract. Canvas Live is authoritative; where a specific Canvas behavior remains unresolved, the affected action's guide explains the limitation.

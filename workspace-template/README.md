# CanvasExpert-SE starter workspace

This folder is a clean starting point for a teacher's CanvasExpert-SE workspace. The 0.1b workflow is a beta contract under implementation. The presence of these instructions does not mean the executable or every Canvas operation has been verified in a live course. Check the build's help and release notes for the commands and behaviors currently available.

## Start here

1. Open this folder as the workspace in Codex or Claude Code.
2. Ask the agent to read `AGENTS.md` and `guides/setup.md`.
3. In a terminal opened at this folder, inspect the bundled CLI with `.\cse\cse.exe --help`. Commands in this beta use an explicit workspace path, for example `.\cse\cse.exe --workspace . setup`.
4. Use setup to save the Canvas credential on this machine and configure the workspace. Then review the task-specific guide before authoring or scoring.

Do not put a Canvas personal access token in a file, command line, prompt, or workspace note. The intended setup flow opens a local form and saves the credential through Windows Credential Manager; verify that this flow is present in the build before relying on it.

## Folders

- `guides/` contains short instructions for setup, authoring, editing, and scoring.
- `settings/courses.md` contains the Canvas instance and configured course nicknames.
- `settings/grading.md` holds teacher feedback preferences and grading context.
- `settings/students.md` is for teacher-authored accommodations and student notes. Keep this starter copy empty of real student information.
- `.cse/` is reserved for private runtime state such as the student mapping. Do not inspect it as an agent.
- `cse/` is where the portable executable belongs when the workspace is packaged.
- `courses/` is where the CLI stores course work. It is populated as supported operations are run.

The course configuration and task document formats are described in `guides/formats.md`. Do not invent fields outside that guide and the corresponding CLI contract.

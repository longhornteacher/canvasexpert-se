# Set up a workspace

Use this guide once for each workspace and on each computer that will use CanvasExpert-SE. The files sync with the workspace; the Canvas credential is per machine.

## 0. Link the program (once per computer)

The program lives in its own folder, separate from this synced workspace. On each computer, the teacher double-clicks `webui.cmd` in the program folder once. That records where the program is. If `.\cse.cmd` reports that the program is not linked, ask the teacher to do this.

## 1. Check the CLI

Open a terminal in the workspace folder and inspect the available commands:

```powershell
.\cse.cmd --help
```

`cse.cmd` supplies this workspace automatically. For example:

```powershell
.\cse.cmd setup
```

Run `version` to see the program's version and build information.

## 2. Connect this computer

Run `setup` and follow the local form to enter the Canvas origin and personal access token. Do not paste the token into the terminal, an agent prompt, a file, or a note. Setup validates the credential and stores it in Windows Credential Manager without displaying it. If the browser does not open automatically, open the local URL printed by the command.

## 3. Configure courses

Use `courses` to discover courses you can access and their IDs. Then set teacher-selected nicknames and current-course flags in `settings/courses.md`; the CLI does not infer nicknames from Canvas titles. Configure only current courses you want a “scoring session” to scan. Use `modules <course>` when an assignment needs a module target. The starter `settings/courses.md` shows the version-one configuration shape; see `formats.md` for the field reference.

## 4. Maintain student notes

Use `students` for alias and monitored-status changes. Put teacher-authored accommodations or useful context in `settings/students.md`, associated with an unambiguous student and course. These notes may contain real names and remain in the teacher's synced folder. Never put a PAT there. Agents must not read the private `.cse/` mapping.

## 5. Review the school calendar

Review `settings/school-calendar.md` before scoring late work. It lists student dates for elapsed lateness calculations; update it when the school calendar changes.

## When setup is done

Confirm the workspace is open in the agent and that the CLI can report its version/help. Then read `authoring.md`, `edit.md`, or `score.md` for the task. Setup stores the credential in Windows Credential Manager. Routine supported operations do not depend on workspace certification metadata; follow the task guide for any specific unresolved limitation.

# Set up a workspace

Use this guide once for each workspace and on each computer that will use CanvasExpert-SE. The files sync with the workspace; the Canvas credential is per machine.

## 1. Check the CLI

Open a terminal in the workspace folder and inspect the available commands:

```powershell
.\cse\cse.exe --help
.\cse\cse.exe --workspace . --help
```

The beta command contract uses an explicit workspace path. For example:

```powershell
.\cse\cse.exe --workspace . setup
```

Run `version` to see the bundled program's version and build information.

## 2. Connect this computer

Run `setup` and follow the local form to enter the Canvas origin and personal access token. Do not paste the token into the terminal, an agent prompt, a file, or a note. Setup validates the credential and stores it in Windows Credential Manager without displaying it. If the browser does not open automatically, open the local URL printed by the command.

## 3. Configure courses

Use `courses` to discover courses you can access and their IDs. Then set teacher-selected nicknames and current-course flags in `settings/courses.md`; the CLI does not infer nicknames from Canvas titles. Configure only current courses you want a “scoring session” to scan. Use `modules <course>` when an assignment needs a module target. The starter `settings/courses.md` shows the version-one configuration shape; see `formats.md` for the field reference.

## 4. Maintain student notes

Use `students` for alias and monitored-status changes. Put teacher-authored accommodations or useful context in `settings/students.md`, associated with an unambiguous student and course. These notes may contain real names and remain in the teacher's synced folder. Never put a PAT there. Agents must not read the private `.cse/` mapping.

## When setup is done

Confirm the workspace is open in the agent and that the CLI can report its version/help. Then read `authoring.md`, `edit.md`, or `score.md` for the task. The bundled 0.1b CLI provides the local setup form and stores the credential in Windows Credential Manager; Canvas write capabilities remain subject to the limits documented by this build.

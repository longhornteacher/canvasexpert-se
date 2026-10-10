# Set up a workspace

Do this once per workspace and once on each computer. The files sync with the workspace; the Canvas credential stays on each machine.

1. **Link the program.** On each computer, the teacher double-clicks `webui.cmd` in the program folder once. If `.\cse.cmd` later says the program is not linked, ask the teacher to do this.
2. **Check the Engine.** In a terminal at the workspace folder, run `.\cse.cmd --help` and `.\cse.cmd version`.
3. **Connect this computer.** Run `.\cse.cmd setup` and have the teacher enter the Canvas address and access token in the local form it opens (if the browser doesn't open, use the printed local address). The token goes to Windows Credential Manager. Never let it be pasted into the terminal, a prompt, a file or a note.
4. **Configure courses.** `.\cse.cmd courses` lists the courses and IDs. Put the teacher's nicknames and `current` flags in `context/courses.md` (field reference in `help formats`); don't infer nicknames from Canvas titles. Mark as current only the courses a "scoring session" should scan.
5. **Student notes.** `.\cse.cmd students` changes aliases and monitored status. Teacher-written accommodations and notes go in `context/students.md`, tied unambiguously to a student and course; real names are fine there.
6. **School calendar.** Have the teacher check `context/school-calendar.md`; lateness depends on it.

Then read the playbook for the teacher's task.

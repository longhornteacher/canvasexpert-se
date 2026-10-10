# Set up a workspace

Do this the first time, and on each new computer (the Canvas credential stays on each machine). The teacher only talks to you and enters their Canvas token in the local form; you do everything else, including editing `context/` files.

1. **Check the Engine.** Run `.\cse.cmd version`. If it says the program was not found, the folder is incomplete: ask the teacher to unzip the full CanvasExpert-SE download again (or, if they keep the program in a separate folder, to double-click `webui.cmd` there once).
2. **Connect this computer.** Tell the teacher they'll need a Canvas access token and how to get one: in Canvas, Account, then Settings, then **+ New access token** under Approved Integrations; give it a purpose like "CanvasExpert-SE", leave the expiry blank or set it for the end of the school year, then copy the token. Run `.\cse.cmd setup`; it opens a local form for the Canvas address and token (if the browser doesn't open, give them the printed local address). The token goes to Windows Credential Manager. Never let it be pasted into the conversation, the terminal, a file or a note. Setup records the Canvas address in `context/courses.md`.
3. **Configure courses.** `.\cse.cmd courses` lists the courses and IDs. Show the teacher the list and ask what they call each class they want to use, and which are current (the ones a "scoring session" should scan). Write their answers to `context/courses.md` (field reference in `help formats`); don't infer nicknames from Canvas titles.
4. **Student notes.** `.\cse.cmd students` changes aliases and monitored status. Teacher-written accommodations and notes go in `context/students.md`, tied unambiguously to a student and course; real names are fine there. Offer this; don't require it.
5. **School calendar.** Lateness depends on `context/school-calendar.md`. Ask the teacher for the school's first and last days and holidays (or a calendar link or file) and fill it in. Offer this; don't block other work on it.

Then do what the teacher originally asked.

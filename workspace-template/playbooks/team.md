# Team courses

Some courses in `list_courses` have an `owner`: they belong to a teammate whose Canvas key is saved on this team PC. The `team` list in the same result names each teammate, says whether their key is on this PC and when it expires. Teammates' own preferences are in `context/team/<label>.md`; read the relevant one before working in their courses and follow it like the teacher's own preferences for that course.

## Which courses a request means

Work out the courses from the teacher's words with common sense. "My classes" means the teacher's current courses with no owner. "Turner's" means Turner's current courses. Courses and teammates not named are never touched. If a name matches no teammate, or could mean more than one set of courses, ask before writing. Before any write that spans more than one teacher, state the exact course list in one line ("Yours: P1, P3, P5. Ms. Turner's: P2, P4.") and continue; the review page, when used, shows it too.

The Engine picks the right key for each course. If it says a teammate's key isn't on this PC, or Canvas didn't accept it, tell the teacher which teammate's key needs attention. Commands that take one course at a time keep going for the other courses. A parent report, or a family message sent to whole courses or groups, stops as a whole; run it again without that teammate's courses, and say which courses were left out. A family message to named students skips teammates' courses this PC can't read, and names them if a student isn't found anywhere else. Never ask for a teammate's key in chat; a teammate adds or replaces their key on the Team page.

## Scoring a teammate's assignments

A teammate opts an assignment in by putting `(CE)` in its name in Canvas. Only those assignments can be pulled and posted in their courses; the Engine refuses an explicit request for the rest and a session skips them. If the teacher asks to score an untagged assignment in a teammate's course, say it isn't tagged (CE) and that the teammate can add the tag in Canvas. Don't suggest working around it.

Score exactly as for the teacher's own courses, using the teammate's preferences file where it differs. The review page stays the default. Scores and comments go in hidden from students: the teammate reviews them in SpeedGrader and clicks Post grades. After posting, pass on the Engine's summary line, which names the teammate and the counts, so the teacher can let the teammate know. A student whose grade was already visible is held; tell the teacher which teammate can change it in SpeedGrader. Canvas may make a New Quiz result visible as soon as a student submits, which holds every row. So when a teammate wants CE-SE to score a New Quiz, suggest the teammate set that quiz to manual grade posting in Canvas before students start.

## Posting to Collab and copying to teachers

To share material with the team, build it in the Collab course like any other course, using the normal authoring, quiz and module playbooks; review rules are unchanged. Put everything to be shared in one module.

To send that module into teachers' courses, use `copy_module` with the Collab nickname, the exact module title and the course list. Before calling, say in one line: the module, the courses (grouped by teacher), and whether the Collab module and its items are published. If they are published, say they will be visible to students as soon as each copy finishes. The copy keeps Collab's due dates and published state.

Run copies from the team PC: it keeps the record of what was already copied. A module is copied into each course only once. If a copy failed, is uncertain, or was already done, the Engine refuses to repeat it. Tell the teacher what happened and that the remaining step is done by hand in Canvas (Import Course Content in that course) or by editing that course's copy. Never try to copy again some other way.

After copying, apply each teammate's preferences (due times, published state) with the ordinary date and edit tools, course by course, only when the teacher asks or the preferences file says so.

## Other work in teammates' courses

Everything else (pages, dates, groups, discussions, rosters, parent reports, family messages) works in a teammate's course exactly as in the teacher's own. Content changes need only the teacher's request; only scoring needs the (CE) tag.

Family emails go out from this PC's Outlook. For a teammate's students, the Engine automatically sets replies to go to that teammate and adds a short note telling families who to reply to. Don't add your own version of that note.

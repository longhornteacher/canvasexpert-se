# 06: Playbooks, docs and capability page

Read [00-BRIEF.md](00-BRIEF.md) first. Suggested model: Haiku. Opus wrote all the playbook wording below. **Place it as written.** Change it only to fit surrounding grammar, or to fix a fact that differs from what was actually built. Report any such change in your final message.

Run this slice **after** 01–05 land, or in parts after each one: each part below names the slice it depends on. Don't describe a capability that isn't built yet.

## 1. New seed playbook: `workspace-template/playbooks/team.md` (after 02; sections marked for 03/04/05)

Check how seed playbooks are discovered (`internal/playbooks/shipped.go`; the embedded `workspace-template/`). If new files under `playbooks/` ship automatically, nothing else is needed. If there is an explicit list, add `team.md` to it.

Create the file with this content. Leave out a section whose slice isn't built yet.

````markdown
# Team courses

Some courses in `list_courses` have an `owner`: they belong to a teammate whose Canvas key is saved on this team PC. The `team` list in the same result names each teammate, says whether their key is on this PC and when it expires. Teammates' own preferences are in `context/team/<label>.md`; read the relevant one before working in their courses and follow it like the teacher's own preferences for that course.

## Which courses a request means

Work out the courses from the teacher's words with common sense. "My classes" means the teacher's current courses with no owner. "Turner's" means Turner's current courses. Courses and teammates not named are never touched. If a name matches no teammate, or could mean more than one set of courses, ask before writing. Before any write that spans more than one teacher, state the exact course list in one line ("Yours: P1, P3, P5. Ms. Turner's: P2, P4.") and continue; the review page, when used, shows it too.

The Engine picks the right key for each course. If it says a teammate's key isn't on this PC, or Canvas didn't accept it, tell the teacher which teammate's key needs attention. Commands that take one course at a time keep going for the other courses. A parent report or family-contact preparation that covers several courses stops as a whole; run it again without that teammate's courses, and say which courses were left out. Never ask for a teammate's key in chat; a teammate adds or replaces their key on the Team page.

## Scoring a teammate's assignments

A teammate opts an assignment in by putting `(CE)` in its name in Canvas. Only those assignments can be pulled and posted in their courses; the Engine refuses the rest. If the teacher asks to score an untagged assignment in a teammate's course, say it isn't tagged (CE) and that the teammate can add the tag in Canvas. Don't suggest working around it.

Score exactly as for the teacher's own courses, using the teammate's preferences file where it differs. The review page stays the default. Scores and comments go in hidden from students: the teammate reviews them in SpeedGrader and clicks Post grades. After posting, pass on the Engine's summary line, which names the teammate and the counts, so the teacher can let the teammate know. A student whose grade was already visible is held; tell the teacher which teammate can change it in SpeedGrader. Canvas may make a New Quiz result visible as soon as a student submits, which holds every row. So when a teammate wants CE-SE to score a New Quiz, suggest the teammate set that quiz to manual grade posting in Canvas before students start.

## Posting to Collab and copying to teachers

To share material with the team, build it in the Collab course like any other course, using the normal authoring, quiz and module playbooks; review rules are unchanged. Put everything to be shared in one module.

To send that module into teachers' courses, use `copy_module` with the Collab nickname, the exact module title and the course list. Before calling, say in one line: the module, the courses (grouped by teacher), and whether the Collab module and its items are published. If they are published, say they will be visible to students as soon as each copy finishes. The copy keeps Collab's due dates and published state.

Run copies from the team PC: it keeps the record of what was already copied. A module is copied into each course only once. If a copy failed, is uncertain, or was already done, the Engine refuses to repeat it. Tell the teacher what happened and that the remaining step is done by hand in Canvas (Import Course Content in that course) or by editing that course's copy. Never try to copy again some other way.

After copying, apply each teammate's preferences (due times, published state) with the ordinary date and edit tools, course by course, only when the teacher asks or the preferences file says so.

## Other work in teammates' courses

Everything else (pages, dates, groups, discussions, rosters, parent reports, family messages) works in a teammate's course exactly as in the teacher's own. Content changes need only the teacher's request; only scoring needs the (CE) tag.

Family emails go out from this PC's Outlook. For a teammate's students, the Engine automatically sets replies to go to that teammate and adds a short note telling families who to reply to. Don't add your own version of that note.
````

## 2. Routing sentences in existing seed playbooks (after 02)

**`workspace-template/AGENTS.md`**, "Route the task": add this sentence after the sentence ending in `group-work.md` first, then any authoring, native-content or scoring playbook needed for its supported individual/course materials.:

> When a request involves a course with an `owner`, a teammate, the Team page or the Collab course, also read `team.md`.

**`workspace-template/playbooks/score.md`**, at the end of the first paragraph under `## Pull` (after 03):

> In a teammate's course (a course with an `owner`), read `team.md` first: only assignments tagged `(CE)` can be scored, and scores go in hidden for the teammate to post. A session with no course covers only the teacher's own courses.

**`workspace-template/playbooks/family-contact.md`**, at the end of `## Prepare the message` (after 05):

> For a teammate's students, the Engine sets replies to go to that teammate and adds a short note saying so; don't write your own.

**`workspace-template/playbooks/setup.md`**, as a new step after step 3 (after 01):

> 3a. **Teammates (team PC only).** If the teacher runs CE-SE for a grade-level team, a teammate adds their own Canvas key on the Team page of the hub (`start_local_ui`, then Team → Add a teammate). The teammate sits at this PC for that step; never take a key in chat. The teammate's courses are then chosen on the same page.

**`workspace-template/playbooks/authoring.md`**, under `## Review and create`, as its last sentence (after 04):

> To share a finished module with the team through Collab, see `team.md`.

Do **not** edit `parent-updates.md`.

**Commit order** (00-BRIEF): commit the playbook edits, run `pastseeds_gen.go`, then commit `past_seeds.txt` separately.

## 3. Developer docs

**`AGENTS.md`** (root): add one paragraph after the Group work paragraph:

> **Team teaching (1.2b1)** is specified in [docs/team/](docs/team/README.md), with execution handoffs in [handoffs/active/team-1.2b1/](handoffs/active/team-1.2b1/00-BRIEF.md). A course may have an `owner`; the Engine routes each course to the right Canvas key, requires the teammate's `(CE)` tag for scoring, writes grades hidden for the teammate to post, copies a Collab module at most once per course, and routes family replies to the actual teacher. Read the README before touching routing, consent, post-policy, copy or family-delivery code.

**`docs/ARCHITECTURE.md`**: add a short "Team courses" section near the credential paragraph (the one beginning "PAT and Canvas origin live in per-machine Windows Credential Manager"). Summarize, without restating the brief:
- R1 storage locations (named Credential Manager entries; `context/team.md`; `.cse/team-log.jsonl` in UserInstall);
- the R2 routing rule;
- that consent (R3) and hiding (R4) attach to **ownership**, not to the routed key.

**`docs/PRIVACY.md`**:
- Add rows to the "What stays on the teacher's computer" table:
  - teammates' keys (Credential Manager on the team PC);
  - the team list and preferences (UserInstall or UserSync);
  - the action log (UserInstall).
- Add a numbered paragraph under "Where information can leave the computer":
  - On a team PC, CE-SE reads and writes teammates' courses with each teammate's own key.
  - That teammate's students' work and contacts are stored on the team PC like the teacher's own.
  - A teammate can end access at any time by deleting the token in Canvas.
  - Family emails for those students are sent from the team PC's Outlook, with replies routed to the teammate.
- Update "Effective" to the commit date and "applies to" to `1.2.0-beta.1 and later`.

**`.github/SECURITY.md`**: where it describes credential storage, add the teammate target name and that keys are only ever looked up by the IDs in `context/team.md`.

**`docs/team/README.md`**: update the Status table row for each finished slice (Status, Local evidence). Do not fill the Canvas Live column; 07 does that.

**`handoffs/README.md`**: the team paragraph already exists. Update its status sentence ("Nothing is built yet") to say what is built and how it was checked.

## 4. Capability page

Update `internal/localui/capabilities.go` and `docs/engine-capabilities.md` together, and set `capabilitiesReviewed` to the date.
- Under **"Partial or awaiting meaningful validation"**, add a group titled `Team teaching (1.2 beta)`, status yellow, with these items:
  - **Teammate keys on a team PC:** teammates add their own Canvas key on the Team page; CE-SE uses it only for their courses. Local tests.
  - **Right key for each course:** your key for your courses and wherever you co-teach; the owner's key otherwise. Local tests.
  - **Scoring only what teammates tag (CE):** checked when work is pulled and again right before posting. Local tests.
  - **Hidden grades for the teammate to post:** grades go in with manual posting; already-visible grades are held. Canvas hiding behavior pending live check.
  - **Copy a Collab module once:** sends one module to each named course with Canvas's course copy, never twice. Pending live check.
  - **Family replies go to the actual teacher:** Reply-To and a short note for teammates' students. Outlook draft check pending.
- Keep the distinction the file already makes between local tests, Canvas Live evidence and pending human validation. Slice 07 moves items as evidence is recorded.

## 5. CHANGELOG

Make sure each slice's `###` section exists under `## 1.2.0-beta.1 (unreleased)` and ends with an honest evidence note. Add `### Team playbook and docs` for this slice.

## Acceptance

- Full tests (embedded seed and capability tests included), vet and the diff check pass.
- `past_seeds.txt` is refreshed in its own commit.
- No document claims Canvas Live evidence that 07 hasn't recorded.

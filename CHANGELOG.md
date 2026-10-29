# Changelog

Notable changes are recorded here. Version names use semantic versioning; **1.0** is `1.0.0` and **1.0 beta** was `1.0.0-beta.1`.

## Unreleased

- Student names stay hidden from the agent's raw Canvas reads: with "Hide student names" on, `canvas_get` / `cse api` replace names in Canvas user objects with the student's saved Pokémon alias (or "(name hidden)") and drop logins, SIS IDs and emails. The teacher instructions (`AGENTS.md`, `TEACHER.md`, MCP server instructions) now tell the agent that students appear by alias and to ask the teacher for an alias instead of searching Canvas for a real name. Names written inside messages, student work and file names are unchanged. Local tests only.
- New Quiz feedback for a missed question: Canvas shows a choice's feedback only for the choice picked, so `push_quiz` now also puts each correct MC/MA choice's rationale in the question's incorrect feedback, green and naming the choice. A student who misses a question sees why their answer isn't right and why the correct one is. Local tests only; the quiz player's display is not yet viewed. Existing quizzes can be fixed through the New Quiz content edit flow.
- Scoring sessions flag possible AI use or copying for the teacher's review: `score.md` sets out the signals, the threshold, and the private in-conversation report with quoted suspect phrases. Writing far above grade level is flagged, strongly when the student is known to work at or below it (on-level section, Silver/Green tier, weaker earlier work). A flag never changes the score or reaches students or families.
- Per-item scoring feedback: a component may carry its own `feedback` and a hand-copy `exemplar`. The comment then reads one block per item (`Q11 Remote execution 10/12`, its feedback, and the exemplar only when the item earned under 90% of its points) instead of a score summary followed by repeated criterion lines.
- New Quiz hot-spot items: a QuizForge `HOTSPOT` item names a local image and the correct region (oval, square or polygon); `push_quiz` uploads the image once through the New Quizzes media flow, never sending the Canvas token to storage, and a rerun reuses it. Hot-spot items carry a correct-answer rationale like other auto-scored items. Local fake-Canvas tests only; no hot-spot item has been created in Canvas Live.
- Small text corrections: `replace_text` / `cse replace-text` makes exact find/replace edits in a course Page body or assignment description, holding without writing when a find matches nothing or several places or the baseline changed. Page fixes push directly; assignment fixes wait for the teacher's go-ahead in the conversation. Canvas Live CS8, 2026-10-20 (branch build): holds wrote nothing, and Page and assignment edits changed only the body.
- Agentic review of student-facing content (`playbooks/review.md`, draft one): the author's finished work goes to a fact-checker (every quotation, page reference, character, term, date and keyed answer proven against a source in hand, or cut), then independent logic and pedagogy reviewers, then an arbiter that rules on every reviewer finding, every possible-AI-use flag and any still-disputed fact. Upheld rulings are applied and the changes fact-checked again, ending in one round. Assignments, quizzes, Pages, edits and scoring reference it; in scoring the reviewers read every student's feedback against the rubric in one pass. The teacher sees only the arbiter's decisions that need them: upheld flags, questions about wording that may be the teacher's own, and findings in the teacher's own material. Helper agents are used where the app supports them, one per role, with a role-by-role fallback. Family messages, Inbox, discussion replies and parent updates are outside it for now. Playbook guidance only; not yet exercised in a teacher session.

## 1.7.0 (2026-10-28)

New Quiz accommodations, differentiation tags, shared rubrics, content edits and a conversation-only scoring check-in.

- New Quiz accommodations: `set_quiz_accommodations` / `cse quiz-accommodations` sets named students' extra time, extra attempts and fewer choices. A Canvas Live probe on CS8 (disposable unpublished quiz, read back on Moderate) showed that a request merges into the student's existing setting, `0` and `false` clear, and extra attempts are absolute, so only supplied fields are sent and a repeat is safe. Saved Quiz time and Fewer choices settings (Classroom > Students) are applied by `push_quiz` and by `from_saved` before publication; an unconfirmed request holds publication and a rerun resends it. Whether extra time and attempts take effect in a student's own session is not yet observed, and course-level sync stays off.
- Bulk differentiation-tag management (`kind: tags`, `set_tags`), with previews, drift holds and submitted-work protection.
- Shared rubrics: `list_rubrics` / `pull_rubric`, rubric references by ID or personal library key, and reuse across assignments and New Quizzes, with CS8 Canvas Live evidence.
- Existing-content edits: inherited Page audiences, exact module-item removal and replacement, narrow New Quiz text/link edits with fresh pulls and three-way conflict checks, and New Quiz assignment-group and date edits.
- The scoring and authoring review page (`start_review`) is gone; the check-in happens in the conversation with an Engine-rendered sample. The private family review and send page is unchanged.
- Direction images now download through the authenticated Canvas client.
- Known open item: a 2026-10-28 read found Canvas's `sessionless_launch` returning JSON with a `url` where the New Quiz file-download path expects the launch page, and a 404 from the participants route. Neither is confirmed against the Engine; check an uploaded-file pull before relying on it. Posting New Quiz item scores through the Engine remains blocked on a captured SpeedGrader request.

## 1.6.0 (2026-10-26)

Weekly ELA parent updates, a printable student reference and wider WebUI pages. This release also carries the 1.5.1 Canvas Inbox work below, which was tagged but not published on its own.

- Weekly ELA parent updates: an optional full-period pull (`cse parent-report --full-period`, MCP `full_period`) and a weekly update layout. The Engine lists Missing and Failing work from the saved report, adds each course's teacher announcement, can send an "all caught up" update and goes to enabled parents only. The default grade-update email and printable report are unchanged.
- Parent reports confirm submission visibility against each assignment, which fixes empty work-due lists seen in two ELA courses. A read-only check on 2026-10-26 returned complete inventories for all 51 students; Outlook draft inspection and delivery remain pending.
- Classroom > Students can print a compact, alphabetized table of the selected course's student names and privacy aliases. Notes, contacts and settings are left out, and printing does not save or discard form edits.
- The desktop shortcut icon is redrawn as a retro robot brain.
- Wide WebUI hub pages use the browser width (up to 120rem); an open contact editor fits without sideways scrolling, and the review page is wider with comments kept at reading width.
- Docs and developer probes no longer carry real course IDs or the teacher's name.

## 1.5.1 (2026-10-25, not published separately)

Canvas Inbox conversations.

- Added teacher-directed Canvas Inbox list, thread read, existing-thread reply, and per-conversation read/archive state operations through CLI and MCP. Thread refs hide Canvas IDs, reads do not mark messages read by default, replies contain only the message body and refuse stale threads, and the Inbox playbook keeps message judgment in the teacher conversation.
- Corrected state writes to send nested `conversation.workflow_state`, as Canvas requires. The public `inbox` action returns an archived thread to the inbox as read. Per-thread failures remain independent, and accepted writes need no confirmation reads. Focused regression fixtures reject the old flat payload and cover read, unread, archived and inbox-to-read translation.
- Canvas Live on 2026-10-25: list/read succeeded and the teacher confirmed three replies appeared in Canvas. After the repair, Canvas accepted read and archive for four selected conversations (eight outcomes); no replies were replayed or confirmation reads made. Unread, return-to-inbox and notifications remain unverified live. See [Engine capabilities](docs/engine-capabilities.md) for build-specific evidence and limits.

## 1.5.0 (2026-10-24)

Honest-effort floor grades.

- A course can set an honest-effort floor on the WebUI Courses page, as a percent of the assignment's points (`effort_floor` in `context/courses.md`). When the agent judges a submission a genuine attempt (`honest_effort: true` on the grade row) and the earned score is lower, the Engine enters the floor as the Canvas grade. The rubric and earned score stay as assessed, and the comment adds "Entered as N/P (class minimum for an honest attempt)" under the earned overall line. Review previews show the entered grade. Canvas applies any late deduction to the entered grade; shared group grades do not use the floor.

## 1.4.0 (2026-10-24)

Teacher check-ins move into the conversation, and drafts can be printed.

- Scores, new assignments, new quizzes and assignment edits are reported in the chat, and the agent writes to Canvas on the teacher's go-ahead there. Scoring reports give each assignment's score range, general impressions, held students and timing notes; the teacher can ask for changes, see a student's feedback or hold a student back before posting. "Score and post" still posts first and then reports. The review page remains available when the teacher asks for it or saves that as a preference.
- Draft PDF (`cse draft-pdf FILE [--handout]`, MCP `save_draft_pdf`) prints an assignment, Page, quiz or results file to Downloads, laid out for paper from the review page's rendering. The teacher copy includes settings, answer keys and private notes; the handout keeps only what students see, with one student's posted feedback per page for scores.
- The CanvasExpert WebUI desktop shortcut shows the original CanvasExpert easel icon. Installs carry `engine/canvasexpert.ico`, and setup sets it on a new shortcut or an existing one for the same folder.

## 1.3.0 (2026-10-24)

Teaching defaults a new install starts with, plus scoring and WebUI refinements.

### Teaching defaults (playbooks)

- Plain, grade-level wording rules for everything students read: word budgets, verb-first steps, sentence length by grade, a cut test and inline definitions ("hard thinking, easy reading").
- Every writing prompt gets a Canvas rubric by default, shaped like the STAAR rubric with a rating between each STAAR level. A writing assignment's rubric totals its points (100 by default); a New Quiz with written items gets one rubric whose criteria total each item's points, named by item number. Mixed quizzes give written items about 40% of the points.
- Quiz choices are written to survive shuffling, distractors match the answer's length, and spiral items are offered for unit quizzes.
- Accessibility basics (alt text, descriptive links, real headings, header rows, no color-only meaning) and a "Check yourself" section with collapsible answers at the end of substantial teaching Pages. Whether Canvas keeps the collapsible boxes is checked on the first real Page.
- Support tiers use two tag sets: **Green** (ELPS) is added automatically when the course's Green tag has students; **Silver**, **Red** and **Blue** readiness tiers (STAAR Did Not Meet, Approaches/Meets, Masters) are offered. Tier links show only the color name.

### Scoring and WebUI

- Teacher-visible WebUI text rewritten as plain, step-by-step help; Connect Canvas explains how to make an access token; Home buttons for family email, grade reports and book clubs.
- Scoring sessions score every packet in one conversation rather than a subagent per packet.
- Results validation and review resolve omitted assignment metadata in memory without rewriting drafts or invalidating their hashes. New results scaffolds include the packet's assignment snapshot, exact component/criterion examples and per-student work paths. Reuse of an earlier scoring interpretation also resolves omitted metadata, preserving handwritten results compatibility.
- Feedback-only comments use `comment-only` with `header: none` and no components, rubric or scoring basis. Review/post retain identity, attempt, evidence and duplicate checks; these comments do not suppress unscored New Quiz work. Supplied scores still require valid arithmetic.
- Results schema errors explain where criterion points belong and show the feedback-only form. Draft conflicts report the current hash while requiring a fresh read.
- Easier new-user installs. `get-engine.cmd` retries a download once with `--ssl-revoke-best-effort` when the network cannot reach certificate revocation servers (seen on district PCs); checksums still verify every file. Without git, the agent downloads and unpacks the release ZIP with Windows' `curl` and `tar` rather than installing git. When district security blocks the program ("Access is denied"), the install playbook confirms the cause from the Windows log and drafts a note for IT instead of offering the ZIP, which holds the same program. The teacher script now covers a missing token button, a token pasted into chat, finding the Canvas address and quitting an app fully. Claude Desktop's Store version is found by its launcher alias. The READMEs no longer name a version, and the developer `AGENTS.md` points installing agents to the README.

## 1.2.0 (2026-10-22)

Team teaching: teammates' Canvas keys on one team PC.

### Slice 07 live checks, Part A so far (2026-10-22)

- Checked live: operator-key routing on an owned course (A1), the untagged-assignment refusal (A2), and copy-once from Collab to CS8 (A7): all three items and the file arrived unpublished, the assignment and New Quiz landed in a new "Imported Assignments" group, a repeat copy and a (CE)-tagged item were refused. Hidden grades (A3) and the already-visible hold (A4) were also checked live with the Test Student: manual posting set through GraphQL with the legacy ID, grade and comment hidden until posted, then visible, and a later post held. The New Quiz comment (A5) waits for first real use because Canvas cannot open a New Quiz as the Test Student; family Reply-To (A6) needs a report with work to send.

### Assign To by differentiation tag (2026-10-22)

- `cse tags COURSE` lists a course's differentiation tag sets and tags with member counts (no students). A `kind: assign-to` document sets Assign To on modules, pages, New Quizzes, assignments, discussions and classic quizzes by tag, by name or ID, with replace or add, per-target dates for assignments and quizzes, and a `--preview` that shows each target's current -> new audience. `cse assign-modules` assigns each module to its same-named tag in one call, and `cse audience-report` shows who can see each module and item and flags items in more than one module. MCP: `list_tags`, `assign_to`, `assign_modules_by_tag`, `audience_report`.
- Names match across courses ignoring case and a trailing `s`; a missing or ambiguous name holds only that target. Writes are read back, reruns write nothing when Canvas already matches, removing access from submitted work is held unless authorized, native group assignments refuse tags, and tags are never created or edited.
- `cse set-file-access COURSE FILE_ID --access course|hidden|locked` (MCP `set_file_access`) changes an existing file, including files copied by `copy_module`. Fixed upload `--access hidden`: the access write now sends only what is needed, the readback accepts the `inherit` visibility Canvas reports, and a rejection shows Canvas's reason.
- Checked live in LANG ARTS 7 on disposable unpublished objects: tag listing, preview, module/page/assignment assignment with read-back and a no-write rerun, hidden upload, and set-file-access. New Quizzes, ungraded and graded discussions, add mode, reset and the apply_to_items cascade were also checked live. Classic quizzes, the group-assignment refusal, the submitted-work hold and clear_item_overrides are not. Live runs fixed tag listing (Canvas omits `non_collaborative` on categories) and the inherited-row report. No review page for assign-to yet; use `--preview`.

### Teammate keys and Team page (2026-10-22)

- The hub has a Team page under Classroom. On the team PC, a teammate pastes their own Canvas key and the date it expires; CE-SE checks it, saves it in Windows Credential Manager (never in a file) and lists the teammate in `context/team.md`. You then choose which of the teammate's Canvas courses to add to `context/courses.md`, each marked with an `owner`. The page can also test a key, rename a teammate, remove one, and shows expiry warnings (also on Home) and each teammate's recent actions from `.cse/team-log.jsonl`. Nothing uses a teammate's key yet. Local tests only.

- Each teammate card on the Team page has an "Email for family replies" box, and the Add page takes an optional school email that wins over the Canvas profile email. A card without an email says family emails about that teammate's students are held until one is added. Local tests only.

### Per-course key routing (2026-10-22)

- Courses marked with an `owner` now use the right Canvas key. Your own courses behave exactly as before, with no extra requests. For a teammate's course CE-SE asks Canvas once, with your key, whether you are an active teacher there: if so it keeps using your key; if Canvas says you are not (or refuses), it uses the teammate's saved key. Any other answer (Canvas busy, network trouble) stops that course instead of guessing. This covers the commands, the agent tools and the hub's roster and tag reads. A missing key says "Ms. Turner's Canvas key isn't on this PC. Add it on the Team page, or run this on the team PC."; a rejected key names the teammate and course. Every write made with a teammate's key is appended to `.cse/team-log.jsonl` without student names or comments, and the status line notes a key that expires soon. A scoring session with no course pulls only your own courses; a teammate's course is pulled only when named. `cse courses --as turner` lists that teammate's courses, and `list_courses` shows each course's `owner` and a `team` list (key on this PC, expiry) without any key. `cse api` and `canvas_get` still use only your key. Local tests only.

### Scoring consent and hidden grades (2026-10-22)

- In a teammate's course, CE-SE pulls and posts only assignments whose name contains `(CE)`: an explicit pull of an untagged assignment is refused, a session skips untagged assignments with a count line, and posting re-checks the live name. Before writing anything, posting sets the assignment to manual posting through Canvas GraphQL and posts nothing unless Canvas confirms it. A student whose grade is already visible (or whose visibility Canvas doesn't report) is held, for grades, comments, rubrics, late timing and group posts alike. The post result and review page say the grades are hidden and that the teammate posts them. Your own courses are unchanged. Local tests only; whether Canvas accepts the assignment ID, hides comments and handles re-grades of posted work as expected is pending live check (07).

### Copy a Collab module (2026-10-22)

- `cse copy-module SOURCE "MODULE TITLE" COURSE [COURSE...]` (MCP `copy_module`) copies one module, matched by exact title, with its items into each listed course using Canvas's selective course copy and each course's own teacher key. Dates and published state are copied as Collab has them. If any item in the module is tagged `(CE)`, the whole command stops before any copy, because each teacher adds `(CE)` in their own course.
- A module is copied into a course only once. CE-SE keeps a record by Canvas IDs under `courses/_team-copies/` and refuses any course that already has an entry (succeeded, failed, uncertain or interrupted). It saves the entry before asking Canvas to copy, never retries a copy, and holds a lock so two runs can't overlap. Copies run from the team PC; the records are not synced. A course whose teacher's key isn't on this PC is reported and skipped while the others proceed.
- The result is one line per course with its status, Canvas's copy warnings and any message; the command exits 3 when any course didn't succeed.
- After a copy succeeds and the new module is found, CE-SE reads each copied item once (read-only, routed with the course's own key) and prints a `Check after copy` block: items assigned to no one, items whose text still links to Collab (Canvas doesn't rewrite those, so students can't open them), and a count of items, published and unpublished. The facts are saved in the copy record; nothing is fixed. An item that can't be read adds the note "Some copied items couldn't be checked." and the copy stays succeeded. Local tests only.
- Local tests only; the selective-copy property shape, New Quiz and file behavior, and assignment-group placement are pending live check (07).

### Family replies go to the actual teacher (2026-10-22)

- Family Contact messages and parent grade updates for a student in a teammate's course now set the Outlook Reply-To to that teammate's Team page email and add one plain note just before the sign-off, for example "Please reply to Ms. Turner, your child's teacher, at turner@school.org. Replies to this email go to Ms. Turner." When one parent update covers courses with different teachers, Reply-To lists each teacher (plus you if one course is yours) and the note has one line per course.
- Emails about only your own courses are unchanged. The note appears in emails and on the review page, never on printed student report sheets, and the parent-update grade content and playbook are untouched. The test email to yourself shows the same note and Reply-To a family would get.
- If a teammate has no email on the Team page, that student's email is not sent and is recorded as skipped ("Ms. Turner has no email on the Team page, so replies couldn't be routed."); a later run sends it once the email is added. Review cards show "Replies go to" or "Held", and Family Contact previews are grouped as "Your students" / "Ms. Turner's students" when a message reaches both.
- Local tests only; Reply-To on a real Outlook draft is pending live check (07). No family email was sent.

### Team playbook and docs (2026-10-22)

- New seed playbook `playbooks/team.md` explains team courses to the agent: how a request maps to teammates' courses, the `(CE)` scoring rule and hidden grades, copying a Collab module once, and family replies. Short routing sentences in `AGENTS.md`, `score.md`, `family-contact.md`, `setup.md` (Add a teammate, on the team PC) and `authoring.md` point to it. `parent-updates.md` is unchanged.
- Developer docs (`AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/PRIVACY.md`, `.github/SECURITY.md`) describe the team storage locations, key routing, consent and family-reply behavior, and the 1.2b1 handoffs. The WebUI Capabilities page and `docs/engine-capabilities.md` add a Team teaching group, status yellow, with local tests and pending live checks kept separate. Local tests only.

### Quiz feedback colors set by the Engine (2026-10-22)

- New Quiz student feedback is now formatted by the Engine: every auto-scored item must have a rationale (MC/MA for every choice, correct and distractor; TF, NUMERICAL, FITB, MATCHING, ORDERING and CATEGORIZATION a `rationale`), so those items need an `id`, and validation refuses a quiz without them. The Engine renders correct MC choices green ("Why this is the best answer:") and distractors red ("Why this isn't the best answer:"), MA with "Why this choice is correct / isn't correct:", other items as green "Why this is correct:", and keeps an MC/MA general rationale uncolored; authored wrappers, styles, color tags and an earlier "Why this ...:" label are stripped so authors write plain text. Local tests only; rendering in the quiz player not yet viewed.

### Quizzes tell students their feedback is there (2026-10-22)

- The quizzes playbook has the agent end each quiz's instructions with one plain line saying every question explains why the correct answer is best and why the others aren't.
- Quiz corrections for points are a default: the instructions also say students earn points back by copying the correct answer's rationale for every question they missed. Quizzes default to one attempt with no retakes. The agent follows both defaults without asking; a teacher's later choice (retakes, points amount, turn-in method, no corrections) is saved in `Teacher_Override.md`. Listed on the Capabilities page. Playbook only.

## 1.1.0-beta.4 (2026-10-21)

Review pages stay open when the agent app restarts, and CI fixes.

### Review pages survive an app restart (2026-10-21)

- A review page started by `start_review` or `start_parent_review` used to close when the agent app restarted the tool connection, losing the teacher's Approve click (observed on the home PC; nothing posted). Review pages now keep running past the adapter, so the approval still posts; setup and students pages still close with it. Calling `start_review` again for the same file reconnects to the page that is still open, and once it closes `review_status` reports revision notes and points to the saved outcomes. Local tests and a compiled-Engine restart check; not yet observed with a real app restart.

### CI fixes (2026-10-21)

- The compiled MCP check expects the 41 tools that include `init_results`, and the audience privacy test matches roster IDs as whole numbers so random refs cannot trip it.

## 1.1.0-beta.3 (2026-10-21)

Lighter scoring reads, Engine-run updates for git installs, and results files that start from the packet.

### Results files start from the packet (2026-10-21)

- `cse results-init PACKET` (MCP `init_results`) writes `scoring/results-<YYYYMMDD>-<HHMM>.md` beside the assignment with the packet path, each student's exact current attempt, the operation the assignment allows (grade, or comment-only for a New Quiz) and any discussion-evidence digest, with short flags for excused, late, unreadable-file and partial-discussion students. It reuses an earlier results file's interpretation and scoring basis for the same assignment, so a scoring basis stays consistent across sessions and sections. Interpretation, components and feedback stay blank and fail validation until the agent fills them; it never overwrites an existing results file. Local tests only.

### Engine-run updates for git installs (2026-10-21)

- `cse update status|stage [--if-due]|apply [--archive-edits]` (MCP `engine_info` `update-status`, `update-stage`, `update-apply`) replaces the git and batch steps the install playbook asked the agent to run. Staging checks at most once a day and verifies the new Engine without live change. Applying holds while a review, setup or students page is open (pages now record themselves under `.cse/open-pages/` while they answer) or while shipped files carry local edits, archives those edits after the agent moves the teacher's choices into `Teacher_Override.md`, fast-forwards only, installs the Engine and checks its checksum. Local tests with real git repositories, a throwaway two-release trial with the real `get-engine.cmd`, and a GitHub clone check; no agent has run it for a teacher yet.

### Lighter packets for the agent (2026-10-21)

- Pulls no longer write assignment directions HTML twice into each packet or repeat each submission's HTML beside its readable text. Packets carry readable directions in the body (`directions_html` names the original HTML file) and student files carry `readable_text` with `body_html_path`; group pulls do the same for shared artifacts. Readable text now keeps code indentation inside ``` fences, link addresses, table cells and `[image: alt]` markers. On this PC's 441 existing pulled files the new layout is 67% smaller for packets and 22% for student files. Grading-basis checks never used the directions HTML; older packets and student files still read as before. Local tests only.

## 1.1.0-beta.2 (2026-10-21)

The Groups update, published as the latest release for all users. It carries every development note below since 1.0: book-club group work, Family Contact emails, current-period parent reports, school-calendar late timing, targeted scoring sessions, and installation and quiet updates by git (the `userinstall` branch with `get-engine.cmd`), with the Windows ZIP kept as the fallback.

## Development notes

### 1.1 beta 2 version and live session evidence (2026-10-21)

- `VERSION` is `1.1.0-beta.2`. The `userinstall` branch ignores `install-log.jsonl`. The Capabilities page records the 2026-10-21 Haiku scoring session: New Quiz comment-only students were not pulled again, and one graded-discussion entry was scored and posted. Before/after counts, resubmission and group-assignment session checks remain pending.

### Targeted scoring sessions (2026-10-21)

- Scoring sessions target unscored submissions using Canvas needs-grading counts, include not-yet-due work, report closed periods, and leave already-scored timing alone. Concurrent detailed lists replace individual submission fetches; one local results index suppresses completed attempts and reminds teachers when New Quiz comments are posted but item points still await entry. Explicit pulls without `--session` retain reassessment behavior.
- The compact report includes assignment group/weight, due state, workload and count explanations. Local tests pass. Read-only CS8 observations confirm assignment-list counts and reconcile the earlier Hotline Miami mismatch as a later submission; a read-only Dev session pull on CS8 produced the expected two-row report. Attachment downloads now run concurrently at the client's 4-request cap (CS8 fetch 19.8s to 5.6s). Broader live session checks remain pending. No release or UserInstall/UserSync adoption.

### Family Contact (2026-10-20)

- The existing private family review and Outlook delivery accept general class messages alongside unchanged grade updates. `prepare_family_contact` resolves course, group and stable-ref audiences for `contact.json`; the teacher reviews each version and recipient, then sends or fills Outlook Drafts. Markdown/HTML messages support local inline images and attachments, optional parents-only routing and the saved closing. Duplicate/stale audiences hold sending; accepted and uncertain outcomes retain their no-replay protections.
- General and book-club family-contact playbooks keep message choices in teacher guidance. Local fixtures establish Engine behavior. A 2026-10-20 classic Outlook check saved a synthetic draft with verified CID HTML/MIME, a visibly rendered inline image and a visible ordinary file attachment; no Send was called or real-family data used. Local classic Outlook with a configured profile is the machine requirement. The Outlook attachment Position argument was removed so ordinary files remain visible. Cloud synchronization and installed-client adoption remain unobserved.

### Group, timing and parent-report regression fixes (2026-10-19)

- Individual group grade/rubric writes renew live mode, basis and student evidence at each numeric action, preserving accepted stages when later actions are held. Discussion fallback retains `recent_replies`; missing-response scoring requires complete relevant evidence at pull and post while independent club conversations continue.
- Comment-only recovery retains the exact original attempted text and never blindly repeats uncertain writes. Shared late-only recovery checks every reviewed member; fresh full-audience timing still uses one representative write. Timing and academic discovery recognize accepted local result contents regardless of extension or ordinary local folder, with exact identity/attempt checks and private/link exclusions.
- Parent packets withhold comments for explicit null posting timestamps even without numeric grades, retaining awaiting-grading inventory and legitimate correction availability. Placeholder-only submission records permit structural group edits; genuine later-page work still holds structure. Group-set updates send only changed fields and preserve explicit clears.
- All nine review failures now have tracked normal-suite regressions, including prepared school-time/member provenance. Group-work guidance and capability records match delivered graded discussions, individual review cards and the recorded CS8 live scope. These are local fixes; no new Canvas Live requests, UserInstall/UserSync adoption, family delivery or distribution is claimed.

### Club units in a shared module (2026-10-19)

- One club audience can produce a course Page, an individual assignment and a New Quiz, each prepared, reviewed and pushed on its own so a held resource never blocks the others. Each resource is created unpublished, restricted to the selected clubs, placed in the requested existing module and published last. The module is a navigation list and carries no audience; each child keeps its own restriction, and an existing module keeps its unrelated entries while gaining one native entry per resource in the requested order. Membership refresh is per resource: assignment and quiz removals are work-protected, Page removals are not. Local synthetic evidence only; Canvas Live module-list, direct-access and student-rendering observations remain separate.

### New Quiz audiences from clubs (2026-10-19)

- A QuizForge file can carry the shared individual club audience in `metadata.canvas.audience`. The restriction targets the quiz's exact associated assignment, so the quiz stays one gradebook column and each student answers independently; a quiz whose assignment is native group work is refused. Push creates the quiz, adds items, applies settings and module placement, applies the audience and publishes last, so a refused or uncertain audience never publishes. The sidecar records accepted stages and a resume repeats only unfinished work; a sidecar bound to another Canvas or course is refused, and older sidecars keep working. Audience-only refresh returns an `assignment-dates` action and sends no quiz, item or settings write, while New Quiz attempts and grades protect removals. Local synthetic evidence only; Canvas Live club-audience quiz and student-view observations remain separate.

### Course Page audiences from clubs (2026-10-19)

- A course Page can carry the shared individual club audience. Push creates it unpublished, applies the complete date-details restriction, finishes requested module placement and publishes last; a refused or uncertain audience never publishes. Unrelated override rows and availability dates survive the collection replacement, `204` is accepted, and an audience-only update sends no stale body. Explicit refresh recomputes the managed student row while preserving manual rows and newer teacher edits. Local synthetic evidence only; Page/quiz student access observations remain separate.

### Explicit assignment audience refresh (2026-10-19)

- Agents can prepare and review a current-club audience refresh through the saved assignment receipt or latest action. The returned assignment-dates path preserves exact owned rows, manual exceptions, date semantics and original create receipts. Current/history work and grades protect access; safe independent changes still proceed with accurate partial results. Pending/uncertain writes are retained without replay. Local synthetic evidence only; Page/quiz authoring integration and student access observations remain separate.


### Individual book-club assignment audiences (2026-10-19)

- Dev can prepare a dated private-ref audience from selected collaborative clubs and create or initially target an individual assignment. Review shows recipients and preserved manual exceptions; empty, incomplete or changed snapshots hold publication. Student-only audiences need no artificial date, accepted/uncertain override writes are retained, and ordinary assignment-date updates preserve all overridden date values. Explicit refresh, course Page audiences and New Quiz audiences are delivered in the entries above. Local synthetic evidence only; no Canvas Live access check is claimed.

### Parent dates and private extra-time evidence (2026-10-18)

- Parent classification now preserves an explicit null student due date instead of falling back to the general assignment deadline. Completion/classification regressions cover null and empty dates, malformed dates, omitted-date fallback and individualized dates; the current-term/14-day reporting gate is unchanged.
- New individual and group scoring pulls include the saved private Extra time flag as true/false, using stable student identity and the active teacher-setup authority. Older absent fields remain unknown. The scoring/group playbooks resolve only established allowances to an absolute deadline, avoid double extensions already in Canvas and hold only timing when a flagged student has no documented rule. Local preparation checks cover calendar seconds, first-genuine attempts and preservation of Canvas's due-date guard. No duration is implied by the checkbox, and this does not edit Canvas deadlines. Packaging, installed playbook adoption and Canvas Live observation remain separate.

### School calendar and parent-report scope (2026-10-18)

- The school calendar now supports named inclusive grading periods and timezone-based current-period resolution. Late timing can be prepared from a first-genuine attempt decision, with school-time, skipped-date and current-Canvas timing provenance; `late-only` updates timing fields without changing grades or feedback. Local focused tests pass. Canvas Live timing writes and destination-client adoption remain pending.
- Parent reports now require a saved current-period scope and complete due-based inventory for the 14-day retrospective window, with Due Soon limited to the next 7 calendar days and clipped at the period boundary. Student-specific effective dates, explicit undated overrides, visibility evidence and unreleased-grade privacy are handled; all inventory rows appear in email and print. Empty windows are skipped, and legacy reports remain preserved but require a fresh pull before send or print. Focused local parent and calendar tests pass; Canvas Live, delivery and destination-client adoption remain pending.

### Parent-report classifications (1.1.0-beta.1)

- Parent reports resolve stale Canvas missing/no-submission indicators using positive teacher-entered grades before drafting. Current recorded zeros lead under Missing/0, retain grade history and never repeat under Corrections. The Engine computes skip eligibility after classification: every included course must meet the configured cutoff (90 by default), with no Missing/0 items. Playbook and context seeds carry these standing rules. Verification is local; no new Canvas Live or delivery claim.

### Collaborative group sets

- Dev can list collaborative group sets and clubs, pull an exact-ID baseline, and create or edit group-set settings and club names/descriptions. Canvas differentiation tags remain excluded. Local implementation checks do not establish live or student access behavior.
- Dev can read stable-ref membership rosters, pending self-sign-up requests, and unassigned students, save explicit add/move/remove plans, and request synchronous even allocation. Membership changes are per-student and conflict-checked; pending requests hold explicit placements, submitted-work moves/removals require specific authorization, accepted writes are never replayed, and uncertain writes reconcile from a fresh roster without blind retry. Successful allocation reports map refs from Canvas's response without a membership read-back. No CE-SE Canvas membership writes or allocations have been run.
- Dev can create and edit collaborative group Pages, pull exact group Page baselines, and read revision history. Group Pages use exact group routes and identity checks, default to teacher-only editing, and preserve student body edits on conflict. Local operation-path tests cover routing, scoped reads, publication ordering, and unsent body preservation; student access and editor behavior remain unobserved in Canvas Live.
- Dev can create ungraded course, distributed club, and direct group discussion topics, and explicitly requested announcements; `pull-discussion` saves editable topic baselines and returned distributed child IDs. Collaborative set/group identity is checked; announcements require recorded intent and audience. Settings-only changes preserve newer live topic content, while requested title/body replacement is held on conflict. Slice 05a clearly rejects linked graded discussions. Local checks only; no CE-SE Canvas discussion writes or student views were tested.
- Group-set, club and discussion authoring now send Canvas REST fields at the documented top level. Discussion edits hold without writing when a fresh pull shows the audience group set, linked assignment, or announcement identity changed, preserving the unsent draft and recording the live baseline. Strict local fixtures reject wrapped payloads; no CE-SE Canvas Live writes were run.
- Dev can pull a complete/partial discussion packet beside the existing editable `topic.md`, including verified distributed club children, full replies merged by source identity, stable-ref/alias attribution, deleted markers, and attachment status. `discussion-reply` creates an explicitly requested teacher entry/reply after exact topic and parent checks; accepted IDs/outcomes persist without read-back, while uncertain replies are never replayed. Local operation-path and compiled MCP checks only; no Canvas pulls or replies were run.
- Dev can add native course Discussion and titled external URL items to ordered modules. Group links are course-checked, equal-titled destinations remain distinct on resume, and `external-resources` reads paginated collaboration/conference metadata without exposing raw participant IDs or provisioning provider resources. Local checks only; no CE-SE module writes or metadata reads were run against Canvas Live.
- The UserInstall seed now includes a Phase 1 book-club workflow playbook covering only the shipped group sets/memberships, shared Pages, ungraded discussions, written evidence, teacher replies, module links and existing resource metadata. Graded group work and later optional group tools remain clearly out of scope.
- Phase 2 slice 08 adds group assignment authoring with explicit grading mode, group-aware snapshots and resume identity, guarded structural edits, plus exact per-row assignment override creation/edit/removal and effective-date pulls. Student IDs are mapped only through existing private refs; group scoring and shared submission pulls remain later slices. Focused local checks and MCP smoke fixtures cover the paths; no CE-SE Canvas Live writes or student access were tested.
- Assignment-date resume records accepted override-create IDs and visibility outcomes. Uncertain date and visibility writes remain unresolved across resumes and are not replayed; no Canvas Live writes were run.
- Phase 2 slice 09 adds typed shared-artifact submission pulls for group assignments, with per-member attempts and grades, effective dates, comments, rubric data, and deduplicated attachments that retain their source associations. Broad pulls preserve completion filtering; explicit group/assignment pulls remain available for reassessment. Local operation-path checks cover scope, member identity, attachment handling, and unsent-result preservation; Canvas Live pulls and student access remain untested.

## 1.0.0 (2026-10-17)

First stable release.

### Install and update from one link

- An agent can install or update CanvasExpert from the repository link: the README tells it to download the newest Windows ZIP from a permanent release link, extract it to a local folder, and run `cse.cmd playbooks update`.
- The ZIP no longer carries playbooks, `TEACHER.md` or teacher files. The Engine lays them down on first run, so extracting a newer ZIP over an existing folder never overwrites a teacher's settings or choices. With UserSync, update restores the local pointer files.
- `cse playbooks update` (MCP `update-playbooks`) keeps each shipped version under `shipped/`, replaces playbooks a teacher never edited (including older versions recognized by fingerprint), and keeps edited ones for the agent to reconcile.

### Teacher choices

- `Teacher_Override.md` holds a teacher's overrides of shipped defaults, read after the playbooks and never replaced. Standing preferences such as "always skip review when I ask to post" are saved there.
- Shipped defaults no longer include one district's grade-portal and tutoring links; teachers supply their own. The school calendar starter includes a bell schedule.

### Feedback and scoring

- Feedback leads with `Overall: X/Y` (marked `(late)` when late), then one line per item. New Quiz feedback never carries an overall line.
- Class exemplars: two anonymous excerpts per prompt, sent as a comment-only follow-up once the last matching student is scored. `comment-only` now works on ordinary assignments and never changes a grade.
- Parent updates list corrections as soon as work is finished, not only after feedback is posted.
- Rationales are required only for auto-scored quiz items.

### Verification

Comment layout, the lateness write and the comment-only follow-up were verified in Canvas Live on Test-Student-only CS8 assignments; install and update were verified locally and on the developer's own install. A colleague's install from the link has not yet been observed. See the [capability record](docs/engine-capabilities.md).

## 1.0.0-beta.1

This beta brings together the current Engine, teacher playbooks and portable Windows Install. This entry describes the release scope; it does not claim a public release date or complete client validation.

### Teacher-first installation

- Bundled selected-client installation for local Work/Codex or Cowork, without a separate Codex CLI.
- Private WebUI launch independent of destination restart, plus local MCP diagnostics that distinguish saved configuration from destination tool access.
- Configuration preservation, repeat-install behavior and an explicit choice before replacing another Install's connection.
- Thin Cowork extension launcher targeting Install's Engine rather than carrying a second Engine.
- Three-step onboarding, a short teaching-assistant starter and an optional teaching profile consulted by scoring and authoring.

### Teaching workflows

- Assignment creation/editing, rubric attachment, native Pages, course assets and Module assembly.
- QuizForge New Quiz authoring, including essay and file-upload items and differentiated support pages.
- Scoped work pulls, rubric/local-basis assessment, exact score arithmetic, review and ordinary assessment/feedback posting.
- New Quiz written-response review and comment-only feedback; item points remain teacher-entered in SpeedGrader.
- Authoritative editable teacher rules in Install or optional Sync, with preservation of existing preferences and unsent work.

### Beta limits

Fresh-user installation under a standard Windows account, destination-client adoption of the new routes and a teacher-judged lesson/feedback example remain pending. Selected Canvas operations have separate recorded live evidence; local tests do not prove student rendering, client compatibility or every district configuration. See the [capability record](docs/engine-capabilities.md) for current evidence and limitations.

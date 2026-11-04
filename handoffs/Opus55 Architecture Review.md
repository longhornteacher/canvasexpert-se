# Opus55 Architecture Review — 2026-11-03

Read-only review of Dev at `f58ecaf` (dev = origin/dev). No code changed, nothing committed, no Canvas calls.
"Verified" = read in code; "inferred" = reasoned, not run.

## What is in good shape

- Layering is clean: no import cycles; `workflow` imports neither `localui` nor `mcpserver`. MCP runs the same `cse.exe` commands as the CLI for 51 of its tools, so safety checks are shared, not copied.
- The WebUI reuses `internal/workspace` for courses, students, contacts, rosters and team, rather than having its own versions.
- File writes are atomic (temp file + rename, `internal/workspace/document.go:295`).
- No Tier C group code exists, so the freeze is real.
- Updates protect `courses/`, `.cse/`, `context/`, `Teacher_Override.md` (gitignored), so unsent drafts survive a git update.

## Findings, ranked by impact on daily teacher use

### 1. Family-contact and parent-update work can't follow the teacher to another PC (verified)
- `internal/workflow/family_contact.go:76-93` forces runs into `courses/<course>/family-contact/<run>` in UserInstall and rejects `context/`.
- Send records (`outcomes.json`, `parent_send.go:45`) live beside them, keyed by student_ref and holding no addresses.
- On PC B, a recreated run has no idea what PC A already sent, so the same email can go to a family twice.
- A family-contact run holds only teacher-written messages, student_refs and send states: no student work. It is a good fit for UserSync.
- Ideas, smallest first:
  - (a) Keep one shared "already sent" ledger in UserSync (run key + student_ref + state + date), and have the send page refuse to resend to a student that ledger marks sent.
  - (b) Move whole family-contact run folders to UserSync.
  - Parent-update runs hold grade inventory, so keep those local and share only the ledger.
  - `ARCHITECTURE.md:33` ("sessions deliberately not durable across PCs") would need a one-line exception.
  - `.delivery.lock` is a per-PC guard. Over a synced folder it only helps if both PCs see it, which is acceptable for a one-teacher, one-PC-at-a-time setup.

### 2. OneDrive conflict copies can silently lose contact and student edits (inferred)
- `.cse/student-contacts.json`, `.cse/students.csv`, `context/students.md` and `courses.md` are rewritten whole.
- Suppose one PC saves while the other is offline or still syncing. OneDrive and Google Drive keep the losing version as a renamed copy (for example `student-contacts-<PC name>.json`). CE-SE reads only the main file, and the teacher never notices.
- Cheap fix: have `doctor` and Home look for conflict-copy names next to the shared files and warn in plain words. No merge system needed.

### 3. The weekly parent email is written for one class and one school (verified)
- `internal/workflow/parent_drafts.go`:
  - `:750` hardcodes the subject "ELA 7 Update – Week of …".
  - `:804-822` hardcodes the greeting, "All missing work…", the "Major grades" heading and "Bobcat Hour"/"flex.securly.com".
  - `:785`/`:813` hardcode a 70% failing cutoff.
  - `:172` refuses to build without `bobcat_hour_url`, while `playbooks/parent-updates.md:7` says a blank tutoring link is fine.
- The default layout's headings and subject (`:540-695`) are also fixed in Go.
- That makes it wrong for any colleague, and it puts the school's name in a public repo.
- Fix: put the wording, cutoff and tutoring line in `context/parent-updates.md`, keeping today's text as the seed default. Rename the `bobcat_*` keys to generic ones and keep reading the old names.

### 4. A cancelled or timed-out MCP call can kill a write halfway (verified mechanism, inferred effect)
- `internal/mcpserver/server.go:477-501` runs `post_results`, `push_document`, `push_quiz` and `inbox_reply` under the request's context with a 15-minute timeout.
- If the agent app cancels the call or the timeout hits, Windows kills `cse.exe` mid-write.
- Existing uncertain-write recovery should prevent a double post, but the teacher gets a confusing "uncertain" state.
- `longCommand` (`pulls.go:26-44`) already runs detached from the request. Use that path for write tools, or at least don't cancel a child that has started writing.

### 5. Updates can leave new playbooks running on an old Engine (inferred)
- `internal/gitupdate` `Apply` updates the playbooks first (`:172`) and the Engine second (`:175`).
- The MCP server already running keeps the old Engine until the app restarts.
- `get-engine.cmd:91-95` can fail its swap if `cse.exe.previous` is still running, and the result is new playbooks with old tools.
- Fix: download and swap the Engine first, update playbooks only after the swap succeeds, then tell the teacher in one line to restart the app.

### 6. The WebUI tells teachers unzipping over the folder is safe; it isn't (verified)
- `internal/localui/capabilities.go:103` and `docs/engine-capabilities.md:96` say the ZIP no longer carries playbooks or TEACHER.md.
- `scripts/package-userinstall.ps1:114-123` puts them in, and `install.md:192` warns about it.
- Unzipping a new release over the folder overwrites edited playbooks. Fix the text.
- `docs/playbook-ownership-and-updates.md` (300 lines) is mostly ZIP-era design, with the git model tacked on at `:296`. Cut it to the current rules.

### 7. Accommodated deadlines are left to the agent to count (verified)
- `playbooks/score.md:89` and `group-work.md:65` have the agent turn "+N school days" into a date using the calendar.
- The Engine already counts school days for lateness (`late_prepare.go`).
- This is exact date math, which the project's own rule says belongs in the Engine: accept the allowance and compute the deadline.

### 8. Leftover machinery from the retired review pages (verified)
- Scoring and authoring review pages were retired on 2026-10-28.
- `workflow/review.go` (688 lines), `review_author.go` (418) and about 300 lines of review cards in `group_post.go:1212-1372` remain.
- Only parent sends, Draft PDF and `validate --show` still use parts of it.
- Trim to what those three need. It is the clearest case of unused complexity.

### 9. Docs have drifted from the code (verified)
- **MCP tool count:** 58 registered, but `capabilities.go:101` says 37 and `engine-capabilities.md` says 38 and 26. `mcp-adapter.md` is missing `copy_assignment`, `list_rubrics`, `pull_quiz`, `pull_rubric` and `set_tags`.
- **Capabilities page (`capabilities.go`):**
  - `:140` says module item remove/replace is "not implemented", while `:40` says it is implemented.
  - Book clubs sit under "Future" though Tier A and B are live-checked.
  - The git install is shown as recorded evidence on local tests only.
- **`ARCHITECTURE.md`:**
  - `:7` says product line 1.0 while `VERSION` is 1.8.1.
  - `:116` repeats the same sentence.
  - `:94` says to "leave quiz/item grades to the teacher", which item_scores at `:122` contradicts.
  - `:118` calls group work "not implemented".
  - `:33`/`:39`/`:47` say UserSync and the teacher own the playbooks, which conflicts with git ownership at `:19`.
  - `:108` has a mojibake character.
  - `:122` still says results are saved "for review/post".
  - Several paragraphs run 300-500 words; it needs a plain rewrite more than more patches.
- `parent-update-flow.md:190-195` still reads as a proposal and lists Band.
- `SCORING-RULES.md` header date is stale.
- **Retire to `handoffs/retired` or git history:**
  - `docs/REVIEW-2026-10-19.md`
  - `docs/book-club-group-work-discovery.md`
  - `handoffs/CSSE_Book_Group_Audiences_Codex_Brief.md` (duplicates `retired/BOOK-GROUP-AUDIENCES.md`)
  - the one `handoffs/active` file, whose item scores were live-checked on 10-29
- **Private IDs:** real course IDs appear in the public repo and on the WebUI page (`capabilities.go:49` two real course IDs, `docs/cs8-test-objects.md`). That breaks the private-IDs-in-UserSync rule.

### 10. Smaller safety and maintainability items (verified)
- **Silent fallback to the teacher's own key:** `localui/hub_shared.go:80-83`. If `courses.md` can't be read, `clientForCourse` uses the teacher's own key instead of failing. It is only used for roster and tag reads, but it should fail the way the CLI does.
- **Git-owned files are writable through MCP:** the MCP file-write tool doesn't block writes to git-owned files (`workspace_files.go:324,372`). One stray agent edit to a playbook holds every later update until it's sorted out. Steer edits to `Teacher_Override.md`.
- **Hand-kept command lists:** `cmd/cse/remote.go` `runRemote` is one function of about 1,050 lines. The command list is maintained by hand in four places (arity map `remote.go:27`, `known` in `main.go:528`, help text, MCP `externalTool`). The help text has already drifted (`prepare-late-timing` is listed three times in `main.go:94-96`). Several tools that call Canvas aren't marked as external, and `copy_*` is marked non-destructive.
- **Repeated identity lookup:** the `/users/self` call is written out separately in five places.
- **Personal cloud folders:** UserSync discovery (`sync_discovery.go:41`) also scans personal OneDrive and Google Drive and suggests connecting them. Student names and parent contacts belong in the district account (PRIVACY.md leaves this to the teacher). A one-line nudge toward the school account would be cheap.
- **Large files:** `group_post.go` (1,678 lines) has three similar timing paths. Splitting isn't required (ARCHITECTURE says size alone isn't a reason), but merging the paths would reduce the bug surface that the 10-17 and 10-19 reviews hit.

## Recommended changes, in priority order

1. Add a shared "already sent" family ledger in UserSync, or move family-contact runs there, so a second PC can't email a family twice. Add the one-line exception to ARCHITECTURE.
2. Warn about OneDrive/Drive conflict copies of shared files in doctor and Home.
3. Move the weekly email's wording, failing cutoff and tutoring line out of Go into `context/parent-updates.md`, renaming the `bobcat_*` keys to generic ones.
4. Run MCP write tools detached from request cancel and timeout.
5. Reorder update apply: Engine first, then playbooks, then a restart line.
6. Fix the false ZIP claim and the MCP tool counts on the capabilities page, and remove the real course IDs from the public repo.
7. Let the Engine compute accommodated deadlines from "+N school days".
8. Delete the leftover review-page code.
9. Rewrite ARCHITECTURE.md short and plain, cut playbook-ownership-and-updates.md to current rules, and retire the point-in-time docs and handoffs.

# HANDOFF H: Durable teacher data is stored and retrievable

Date: 2026-11-04. Slice H of the [unified architecture review](../Unified%20Architecture%20Review.md), reframed by developer decisions 1 and 4. Layer: **Engine storage** (`internal/workspace`, `internal/workflow`) and **docs**. It is mostly verification; fix only a concrete gap.

## Decision (developer, 2026-11-04)

- Local functionality comes first.
- One PC at a time is fine.
- **Out of scope:** continuing a specific session on another PC, a sent ledger for duplicate prevention, moving runs to UserSync.
- **The goal:** everything meant to outlive a session (for example parent contact details) is reliably stored in its durable home and can be found again.

Before starting: `git fetch`, bring `dev` up to date, read root `AGENTS.md`, `docs/ARCHITECTURE.md` (shared/local boundary at ~19, ~33, ~55, ~65), [docs/family-contact-flow.md](../../docs/family-contact-flow.md) and the [shared execution notes](../README.md#execution-notes-for-the-active-handoffs). Facts mapped at `2818e36`.

## What the mapping found

**Resolvers** (`internal/workspace`):
- `ResolveTeacherPath(root, rel)` (`rules_location.go:136`) routes to the UserSync rules root when `IsSharedRulePathIn(root, rel)` is true:
  - `context/…` and teacher root `.md`/`.txt` files;
  - in a ZIP install also `playbooks/…`;
  - in a git install, `playbooks/` and the product root guides stay local and git-owned (`install_kind.go:70–90`).
- `ResolveRulesPath` (`:165`) always uses the rules root; it is used for the private `.cse` tables.
- `GetRulesLocation` (`:69`) errors when a configured UserSync folder is missing and never falls back to stale local data.
- `initializeReusableData` (`reusable_data.go:10`) copies five items into UserSync once when it is connected: `context/courses.md`, `context/students.md`, `.cse/students.csv`, `.cse/rosters.json`, `.cse/student-contacts.json`.

**Durable (should reach UserSync when configured):**

| Item | Path resolution | file:line |
|---|---|---|
| Student table `.cse/students.csv` | `ResolveRulesPath` | `workspace/students.go` `OpenStudents`/`Save` |
| Parent and student contacts `.cse/student-contacts.json` | `ResolveRulesPath` | `workspace/contacts.go:12,39,60–65` |
| Course rosters `.cse/rosters.json` | `ResolveRulesPath` | `workspace/rosters.go:20,37` |
| Student notes `context/students.md` | `Read/WriteTeacherDocument` | `localui/hub_context.go:252,306`; `workflow/pull_work.go:196` |
| Courses `context/courses.md` | `Read/WriteTeacherDocument` | `workspace/instance.go:16,59`; `localui/courses.go:152`; `settings.go:130`; `team.go:466,525` |
| Course notes, assignment expectations `context/courses/<id>/…` | MCP `write_workspace_file` → `safePath` → `ResolveTeacherPath` | `mcpserver/workspace_files.go:42–56,130–132` |
| Reteach `context/reteach.md` | same | same |
| `Teacher_Override.md` | root `.md`, shared in both kinds | `install_kind.go:84–90` |
| School calendar, parent-update settings, grading rules | `ReadTeacherDocument` / `ResolveTeacherPath` | `school_calendar.go:49`; `parent_drafts.go:43`; `hub_context.go:234,239,325` |
| Team `context/team.md`, `context/team/<label>.md` | `ResolveTeacherPath` | `workspace/team.go:14,104–136` |

**Local by design (ARCHITECTURE ~33, ~55):**
- team log `.cse/team-log.jsonl`;
- Collab copy records `courses/_team-copies`;
- family-contact runs: `contact.json`, `audience.json`, `assets/`, `outcomes.json` under `courses/<c>/family-contact/<run>`;
- parent-update runs: `report.json`, `drafts.json`, `outcomes.json`;
- review state `<file>.review.json`;
- grading results files, which also hold the posting receipts.

Some code checks the local `context/courses.md` only as a "this folder is UserInstall" marker (`paths.go:18,79`, `scaffold.go:91`, `installation.go:88`, `cse-mcp-launcher/main.go:21`); that is not data.

**Past family messages.** What a teacher can look up later, on the PC that ran them:
- `contact.json`: title, audience, per-message subject/body/attachments. No addresses.
- `drafts.json`: per-student decision and intro text.
- `outcomes.json`: per `student_ref`, a `state` (`sent`/`drafted`/`failed`/`uncertain`/`skipped`), `reason` and `at`. No addresses, and not the final rendered email.
- Nothing lists past runs or sends. The WebUI has no history page, `review_status` only knows the current session, and runs are found only by browsing `courses/` with workspace file tools.

## Open question for the developer (ask first, in plain words)

Decision 4 names "parent email" as a durable need. Parent and student **contact details** already live in UserSync (`.cse/student-contacts.json`), so that need may already be met. Ask which of these the teacher actually needs later:
- **(a)** contact details only: already durable; nothing to build;
- **(b)** a record of what was sent to which family, findable later on this PC: the run folders already hold it, but nothing lists it. A small read-only listing (MCP or WebUI) of past runs with date, title, counts by state would meet it;
- **(c)** the same record on another PC: a durable, privacy-reviewed "sent messages" summary in UserSync (no addresses, no grades, no student work). It would serve lookup only, explicitly **not** duplicate prevention.

Build only the option the developer picks.

## What to do

1. **Verify routing with one test.** Add a table test in `internal/workspace`. With UserSync configured, it asserts that each "durable" item above resolves under the UserSync root, and each "local by design" item under UserInstall. Also assert it errors (not falls back) when UserSync is configured but missing. Reuse the helpers from `rules_location_test.go` / `reusable_data_test.go`.
2. **Check first-connect completeness.** When a teacher first connects UserSync on a PC that already has local `context/` files, which of `context/school-calendar.md`, `context/parent-updates.md`, `context/grading.md`, `context/reteach.md`, `context/team*.md`, `context/courses/…` and `Teacher_Override.md` reach UserSync? `initializeReusableData` copies only five items. Read `SetRulesLocation` and its tests.
   - If other durable files are silently left behind locally (and then shadowed by empty UserSync paths), that is a concrete gap: fix it by copying missing files once, never overwriting existing UserSync files, and add a test.
   - If they are copied elsewhere, record where.
3. **Document the boundary** as a short table in `docs/ARCHITECTURE.md`, replacing prose where it is clearer: durable → UserSync when configured; local by design. Link it from the unified review.
4. **Build the option the developer chose** in the question above, if any.

## Done when

- The routing table test passes. Any first-connect gap is fixed with a test, or recorded as "none found" with the code reference.
- ARCHITECTURE has the inventory table.
- The developer's answer on past family messages is recorded here with the date, and that option is built or explicitly declined.
- Full checks pass.

## Not in this slice

Cross-PC session continuity, a duplicate-send ledger, provider conflict-copy warnings (decision 5: wait), moving run folders.

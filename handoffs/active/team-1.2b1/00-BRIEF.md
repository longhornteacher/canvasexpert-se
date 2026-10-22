# Team teaching 1.2.0-beta.1: execution brief

Prepared 2026-10-22 by Opus against Dev branch `dev`, revision `ff6f48a`. Decisions are final; they were confirmed with the developer on 2026-10-22. The product plan and status table are in [docs/team/README.md](../../../docs/team/README.md).

This brief does **not** authorize any of the following:
- Canvas Live writes, except the developer-run checks in [07](07-LIVE-CHECKS.md).
- Collecting real colleague keys.
- Release, packaging, or UserInstall/UserSync adoption.

## How to use these handoffs

- Execute the slices in order: **01 → 02 → 03 / 04 / 05 → 06 → 07**. Slices 03, 04 and 05 each depend only on 02. Run them one at a time, because they touch some of the same files (`remote.go`, `capabilities.go`, `CHANGELOG.md`).
- Each slice handoff is self-contained once you have read this brief. Read root [AGENTS.md](../../../AGENTS.md), [ARCHITECTURE.md](../../../docs/ARCHITECTURE.md) and [SCORING-RULES.md](../../../docs/SCORING-RULES.md) once before starting.
- **Model fit.** Slices 02, 03 and 04 need careful judgment within the contracts below; run them on Sonnet. Slices 01 and 05 are Sonnet or a careful Haiku. Slice 06 is Haiku. Slice 07 is driven by the developer, with an agent assisting.
- **Do not make product decisions.** Every consequential decision is recorded here. If the code contradicts a contract, or a slice needs a choice this brief doesn't make, **stop and report** the conflict with file:line evidence. In particular:
  - which key is used for a course;
  - when a grade may become visible to students;
  - whether something may be copied twice;
  - what families are told.
  
  Small local choices (helper names, file splits, test layout) are yours: match the surrounding code.
- **Line references** are from revision `ff6f48a`. Find the code by the quoted symbol if the lines have moved.

## Validation (every slice)

Use the repo toolchain, then run the diff check:

```bash
./.tools/go1.27.1/go/bin/go.exe test ./... -count=1
```

```bash
./.tools/go1.27.1/go/bin/go.exe vet ./...
```

```bash
git diff --check
```

If MCP tools were added or changed, run the compiled MCP smoke the same way earlier releases did. CI expects the exact tool count, 41 at `1.1.0-beta.4`, in `scripts/check-mcp.mjs:59`. Update it there.

Tests stay proportionate:
- one focused test per new rule;
- a token-aware fixture (slice 02) covering one representative command per area;
- no coverage quotas and no test matrices.

Local tests are never presented as Canvas Live proof.

**Commits.** Commit each slice on `dev` with the attribution line from the session. If a slice edits a seed playbook (`workspace-template/playbooks/*.md`, `workspace-template/AGENTS.md`), commit that edit first. Then run the following and commit `internal/playbooks/past_seeds.txt` as a separate `chore: refresh past seed fingerprints …` commit, as the history shows:

```bash
./.tools/go1.27.1/go/bin/go.exe run internal/playbooks/pastseeds_gen.go
```

## Vocabulary

| Term | Meaning |
|---|---|
| **you / operator** | The teacher whose CE-SE this is. Their key is the existing Credential Manager entry. Teacher-facing text says "you", never "operator". The **only** exception is the Add-a-teammate form, which is read by the teammate at the keyboard: there, "you" is the teammate. |
| **teammate** | A colleague whose key is saved on the team PC. Teacher-facing text uses their display `name` ("Ms. Turner"), never pronouns. |
| **owned course** | A configured course with a non-empty `owner`. |
| **routed key** | The key CE-SE chose for a course under decision R2. |
| **label** | A teammate's stable short ID (`turner`). It is used in `courses.md` `owner`, file names and logs. |

## Final decisions and contracts

### R1. Storage

**Teammate key.**
- Stored in Windows Credential Manager under the target `CanvasExpert-SE/Canvas/v1/teammate/<canvas_user_id>`.
- Same `SE01` blob and `encodeCredential`/`decodeCredential` as the operator key.
- The origin **must equal** the configured `instance`, as `remote.go:121-124` already checks.
- No enumeration API: CE-SE only ever looks keys up by the IDs listed in `team.md`.

**Team list: `context/team.md`.**
- Read and written with `workspace.ReadTeacherDocument` / `WriteTeacherDocument`, so it follows the UserSync authority when one is configured.
- A missing file means no teammates; that is not an error.
- The format:

```yaml
---
schema: 1
teammates:
  - label: turner            # ^[a-z][a-z0-9-]{0,23}$, unique
    name: Ms. Turner         # what families and teachers see; editable
    canvas_user_id: 123456   # from /users/self of their key; unique; never the operator's
    email: turner@school.org # Canvas primary email at add time; editable; used for family Reply-To
    key_expires: 2027-06-04  # YYYY-MM-DD the teammate chose in Canvas; required
---
Teammates whose Canvas keys are saved on the team PC. Keys themselves are never in this file.
```

Go types go in `internal/workspace/team.go`:
- `TeamConfig{Schema int; Teammates []Teammate}`
- `Teammate{Label, Name string; CanvasUserID int64 \`yaml:"canvas_user_id"\`; Email string; KeyExpires string \`yaml:"key_expires"\`}`
- `Validate()` enforces the comments above.
- Agents may read this file (no secrets).

**Teammate notes: `context/team/<label>.md`.**
- Prose preferences. Created with a two-line starter when a teammate is added; never deleted by CE-SE.
- The starter says: "Preferences for scoring, feedback, due times and publishing in <name>'s courses. Written by you after talking with <name>."

**Course owner.** Add the following to `domain.Course` (`internal/domain/types.go:13`):

```go
Owner string `yaml:"owner,omitempty" json:"owner,omitempty"`
```

When `courses.md` is read for a command, an owner that is not in `team.md` is an input error naming the course.

**Action log: `<UserInstall>/.cse/team-log.jsonl`.**
- Always the UserInstall root, **not** the teacher-setup authority, because the log is per machine.
- Append-only, one JSON object per line:

```json
{"time":"2026-10-22T14:03:11Z","teammate":"turner","course":"Turner P2","course_id":120700,"method":"PUT","path":"/api/v1/courses/120700/assignments/3700001/submissions/:student","status":200,"uncertain":false}
```

- Path redaction: any path segment following `submissions`, `users` or `students` that is numeric becomes `:student`. Strip query strings.
- No student names, refs, comment text or grades go in the log.

### R2. Key routing

The rule:
1. A course with **no owner** uses the operator key, exactly as today. No extra requests.
2. A course **with an owner**:
   1. Once per course per process, `GET /api/v1/courses/:id` with the **operator** key. If the response's `enrollments` array has an entry with `type == "teacher"` and `enrollment_state == "active"`, use the operator key (co-teaching).
   2. A 200 without an active teacher enrollment, or a `canvas.Error` with status 401, 403 or 404, means: use the owner's key.
   3. Any other error fails that course for this command and is **not cached**. This covers throttling, 5xx, network errors and cancellation. A transient error must never switch a co-taught course to the teammate's key.
   4. If the owner's key is not on this PC, the command fails for that course with the one missing-key message used everywhere (CLI, WebUI, Test button):
      > `Ms. Turner's Canvas key isn't on this PC. Add it on the Team page, or run this on the team PC.`

Where routing lives:
- **Workflow.** Through a new `Service.ForCourse(ctx, course) (*Service, error)`. It returns a shallow copy whose `Client` is the routed client and whose unexported `team` field records the identity (`label`, `name`, or empty for the operator). Courses without an owner return the receiver unchanged.
- **Key access.** Through an interface on `Service`:

```go
type KeySource interface {
	Operator() (platform.Credential, error)
	Teammate(canvasUserID int64) (platform.Credential, error)
}
```

  Production implements it with `platform`, through the exported constructor `workflow.PlatformKeys() KeySource`. Tests use a map. `Service` gains `Team workspace.TeamConfig` and `Keys KeySource`.

  **Ownership and routing are separate.** Ownership comes only from a course's `owner` in `courses.md` plus the team list. It drives consent (R3), hiding (R4) and family replies (R8), whatever `Keys` is.
  - When `Keys` is nil, every course routes to `Client` (the operator key), but ownership rules still apply.
  - Any `Service` that writes must have `Team` loaded. An owned course whose owner isn't in `s.Team` is an input error, never "unowned".
  - Every place that builds a `Service`, including `cmd/cse-probe/main.go:249`, reads `context/team.md` and calls `workspace.CheckOwners`.
- **Client caching.** The routed client is cached per course ID for the process (mutex-protected map on a shared pointer, so copies share it).
- **Teammate clients** are built with `canvas.New(credential.Origin, credential.Token)`. Zero the token string afterward, as `remote.go:117` does. Attach the action-log write hook (R6).

There is **no extra enrollment check before writes** (decision 6 in the plan). Canvas refuses a key whose owner isn't a teacher in that course. CE-SE turns that refusal into a plain message: a `*canvas.Error` of kind `authentication` from a teammate client is wrapped as:
> `Canvas didn't accept Ms. Turner's key for Turner P2. The key may have expired or been deleted, or Ms. Turner may not teach that course. Nothing more was done in that course.`

Wrap it at the routed client boundary (a small error-mapping wrapper, or wrapping in `ForCourse` callers). Do not change `canvas.Error.Error()`.

**Scoring sessions.** `pull-work` with no course (all current courses, `pull_work.go:166-172`) includes only courses with **no owner**. A teammate's courses are pulled only when named.

### R3. Scoring consent

- `domain.HasConsentTag(name string) bool` reports whether `strings.ToLower(name)` contains `(ce)`. That is the whole rule.
- It applies to every **owned course**, whichever key is routed.
- Required at pull (explicit assignment, session pull of a named owned course, group pull, Test Student pull).
- Re-checked on the live assignment read that `Post` and group post already do before the first write.
- Without the tag:
  - pull refuses that assignment;
  - a session skips it, with a count line;
  - post holds every row and preserves the results file.
- Messages:
  - Pull: `Essay 3 in Turner P2 isn't tagged (CE), so Ms. Turner hasn't opted it in to CE-SE scoring. Ask Ms. Turner to add (CE) to its name in Canvas.`
  - Post hold reason: `not tagged (CE) in Canvas right now; nothing posted`

### R4. Hidden grades

For an owned course, before the first grade, rubric, comment or late-timing write on an assignment, set that assignment to manual posting through Canvas GraphQL:

```graphql
mutation($id: ID!) {
  setAssignmentPostPolicy(input: {assignmentId: $id, postManually: true}) {
    postPolicy { postManually }
    errors { attribute message }
  }
}
```

- `$id` is the REST assignment ID as a string.
- The canvas client gains one typed method for this. The path allowlist (`client.go:130`) gains the exact path `/api/graphql`, and nothing else under it.
- Success means `postPolicy.postManually == true` with no `errors`.
- The mutation is idempotent, so on an uncertain result retry it **once**.
- Any other outcome holds every row of that assignment: `couldn't set Canvas to hide these grades; nothing posted`. **Never post to an owned course without a confirmed manual policy.**

Per-student hold:
- Each student's existing pre-write submission read (`post.go:179`, and the group equivalent) must show `PostedAtPresent && PostedAt == nil`.
- If `posted_at` is set, or the field is absent, hold that student: `grade already visible to the student; Ms. Turner can change it in SpeedGrader`.
- In group posting, one visible member holds that group's shared action.

**New Quizzes.** Canvas may post a New Quiz submission at submit time under automatic posting. If it does, every comment-only row on that quiz is held by this rule. That is deliberate: the comment would otherwise be visible at once. The playbook tells the operator to ask a teammate who wants CE-SE scoring on a New Quiz to set it to manual posting **before students start**. Slice 07 records what Canvas actually does.

The result summary for an owned course says, per assignment:
> `Turner P2 · Essay 3: 24 scored, hidden from students, ready for Ms. Turner to review and post. 2 held (already visible).`

### R5. Copying from Collab

- **Command:** `cse copy-module SOURCE "MODULE TITLE" COURSE [COURSE...]`
- **MCP:** `copy_module {source_course, module, courses[]}`, run through `longCommand`.
- The unit copied is one module in the source course, matched by **exact** title. Zero or several matches stop the command.
- **Consent can't arrive by copy.** If any item title in the source module contains `(CE)`, the whole command stops before any copy:
  > `Remove (CE) from "Essay 3" in Collab first. Each teacher adds (CE) in their own course.`
- **Copy once only.** A record at `courses/_team-copies/course-<source course id>-module-<module_id>.json` in UserInstall lists each destination course ID with a status. It is keyed by IDs, so renaming a nickname can't lose it. A whole run holds an exclusive lock, `courses/_team-copies/.copy.lock`, using the same pattern as the existing delivery lock. A second concurrent run is refused: `Another copy is running; try again when it finishes.` Records are per machine, so copies are run from the team PC (playbook). A destination that already has **any** entry (pending, succeeded, failed or uncertain) is refused:
  > `"Unit 3" was already sent to Turner P2 on 2026-10-22 (failed). CE-SE copies a module only once. Finish it in Canvas with Import Course Content, or edit that course's copy.`
- **Write the pending entry before the migration POST.**
- Each destination uses its routed key and runs independently, at most 4 at a time. One destination's failure never stops another.
- The flow and polling details are in [04](04-COLLAB-COPY.md).
- Dates and published state are copied as Collab has them. CE-SE does not shift dates or change published state during a copy.
- There is no review page for a copy. The playbook has the agent state the module and the resolved course list before calling.

### R6. Action log hook

- `canvas.Client` gains an optional exported field `OnWrite func(method, path string, status int, uncertain bool)`. It is called after every `Write`/`WriteAccepted`/GraphQL attempt that reached the network or was uncertain.
- Only teammate clients set it. Nothing else changes for the operator client.
- The hook appends the R1 log line, best-effort: a log failure is printed to the status writer once and never fails the Canvas write.

### R7. Expiry

- "Expires soon" means today ≥ `key_expires` − 14 days. Dates are compared in the local date.
- The status writer prints one line per command when a routed teammate key expires soon:
  > `Note: Ms. Turner's Canvas key expires 2027-06-04.`
- The Team page and Hub home show the same warning, with "Expired" past the date.
- An expired date does not block anything: CE-SE tries, and Canvas decides.

### R8. Family reply-to

When a family email concerns a student in an owned course:
- The Outlook message's Reply-To (`MailItem.ReplyRecipients`) is the owner's `email`.
- The body gets this note, just before the sign-off:
  > `Please reply to Ms. Turner, your child's teacher, at turner@school.org. Replies to this email go to Ms. Turner.`

If one parent-update email covers courses with different teachers:
- Reply-To lists every distinct owner email. If any of the courses is your own, it also includes Outlook's `SelfAddress`.
- The note becomes one line per teacher: `For <course nickname>, please reply to <name> at <email>.`

Emails that concern only your own courses are **byte-for-byte unchanged**.

The note goes in emails only: never on printed student report sheets (the `forStudent` branch of `ParentRun.render`).

If an owner has no `email`, that student's email is not sent. It is recorded as `skipped`, the state already used when a student has no contact on file (`parent_send.go:255`), so a later run sends it once the email is added. If a later run doesn't retry `skipped`, stop and report. The message is:
> `Ms. Turner has no email on the Team page, so replies couldn't be routed.`

Details are in [05](05-FAMILY-REPLY-TO.md).

### R9. Surfaces

- **MCP `list_courses`** (configured source) includes `owner` automatically through `domain.Course`. It adds a `team` array:

```json
[{"label":"turner","name":"Ms. Turner","email":"…","key_on_this_pc":true,"key_expires":"2027-06-04","expires_soon":false}]
```

  The response never includes a key.
- **CLI `cse courses --as <label>`** lists that teammate's teacher courses (for setup). `cse api` / MCP `canvas_get` stay operator-key only; say so in the tool description.
- **WebUI:** a Team page (slice 01), plus an owner column and owner-aware routing on the existing courses, roster and tags paths (slice 02).

### R10. Version

- `VERSION` becomes `1.2.0-beta.1` in slice 01.
- `CHANGELOG.md` gets `## 1.2.0-beta.1 (unreleased)` with one `###` section per slice. Each ends with an honest evidence note ("Local tests only." until 07 records more).

## What not to build

- Key syncing between PCs, or Credential Manager enumeration.
- Re-copying, or keeping copies in step with Collab.
- Blueprint sync.
- An extra pre-write enrollment check.
- A "never score" list.
- CE-SE messaging teammates.
- Teammate logins or roles inside CE-SE.
- Generic multi-account abstractions beyond `KeySource`.
- Changes to grading arithmetic or the parent-update grade guarantees.

## Stop and report if

- A write path to an owned course can be reached without going through `ForCourse` and the slice's tests can't cover it. Name the path.
- Canvas GraphQL rejects the legacy assignment ID in local evidence or documentation. Do not invent a fallback that posts visibly.
- Selective copy needs properties beyond those described in 04 to include a module's items.
- Any requirement would put a key, student name or private note into a file, log, page response or agent output.

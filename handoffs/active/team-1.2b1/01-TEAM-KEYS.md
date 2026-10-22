# 01: Teammate keys, team list and Team page

Read [00-BRIEF.md](00-BRIEF.md) first. Contracts R1, R7, R9 (WebUI part) and R10 apply here. Suggested model: Sonnet.

## Result

On the team PC, the operator opens **Team** in the Hub and adds a teammate:
- The teammate, sitting at the keyboard, pastes their Canvas key and the expiry date they chose.
- CE-SE checks the key, saves it in Credential Manager, and adds the teammate to `context/team.md`.
- The operator then picks which of the teammate's Canvas courses to add. They go into `context/courses.md` with `owner: <label>`.

The Team page can also test and remove teammates, and shows expiry warnings and recent actions. Nothing routes through teammate keys yet; slice 02 does that.

## Work

### 1. Version start (R10)

- `VERSION` becomes `1.2.0-beta.1`.
- Add `## 1.2.0-beta.1 (unreleased)` at the top of `CHANGELOG.md`, with a one-line summary: "Team teaching: teammates' Canvas keys on one team PC." Add a `### Teammate keys and Team page (date)` section.
- Run the version test (`cmd/cse/version_test.go`) and fix any expectation it hard-codes.

### 2. Platform: named teammate credentials

In `internal/platform/credential.go`:
- Add `func TeammateTarget(canvasUserID int64) string`, returning `"CanvasExpert-SE/Canvas/v1/teammate/" + strconv.FormatInt(id, 10)`.
- It rejects `id <= 0` by returning `""`. Callers then fail.

In `internal/platform/windows.go`, next to `LoadCredential` (:105):
- `LoadTeammateCredential(id int64) (Credential, error)`
- `SaveTeammateCredential(id int64, c Credential) error`
- `DeleteTeammateCredential(id int64) error`

Use the existing `loadAt`/`saveAt`/`deleteAt`. A missing teammate key returns an error wrapping `ErrNoCredential`. Do not use the "run cse setup" wording for teammates: add `ErrNoTeammateCredential = errors.New("this teammate's Canvas key isn't on this PC")` and return it wrapped.

In `internal/platform/other.go`, add matching stubs that return "requires Windows" errors.

**Test:** extend the credential self-test pattern only if cheap. `CredentialSelfTest` already proves named targets round-trip, so a pure test of `TeammateTarget` formatting is enough.

### 3. Workspace: team list

Create `internal/workspace/team.go` with the R1 types:
- `ReadTeam(root string) (TeamConfig, error)`: missing file → empty config with `Schema: 1`.
- `WriteTeam(root string, cfg TeamConfig) error`: validates, then calls `WriteTeacherDocument` with the R1 body text.
- `(TeamConfig) Validate() error` checks:
  - label pattern and uniqueness;
  - `canvas_user_id > 0` and unique;
  - `name` non-empty;
  - `key_expires` parses as `2006-01-02`;
  - `email`, if present, contains `@` and no spaces;
  - schema 1.
- `(TeamConfig) Find(label string) (Teammate, bool)`: case-insensitive on the label.
- `EnsureTeammateNotes(root, t Teammate) error`: creates `context/team/<label>.md` with the R1 starter, through `ResolveTeacherPath`, only if absent.

Courses validation goes where `courses.md` is read for commands (`remote.go:97`, and the WebUI/MCP readers that need routing in 02): every non-empty `Owner` must be found in the team list. Add `workspace.CheckOwners(config domain.CourseConfig, team TeamConfig) error` here. Slice 02 calls it.

Add `Owner` to `domain.Course` now (R1), so the courses page can write it.

**Tests:** round-trip, each validation rule (table-driven, one row each), a missing file, and notes not overwritten.

### 4. Action log writer (used from slice 02)

Create `internal/workspace/teamlog.go`:
- `AppendTeamLog(installRoot string, e TeamLogEntry) error` writes the R1 JSON line under `<installRoot>/.cse/team-log.jsonl`. Use the UserInstall root even when a UserSync authority is configured. Look at how `.cse/open-pages/` is located (`cse update` work, CHANGELOG 1.1.0-beta.3) and use the same install-root resolution.
- `RedactTeamLogPath(path string) string` implements the redaction in R1.
- `ReadTeamLog(installRoot string, label string, limit int) ([]TeamLogEntry, error)` returns the newest entries first, for the Team page.

**Tests:** redaction of `/submissions/123`, `/users/9`, `/students/4`, query stripping, and that a non-numeric segment is untouched.

### 5. WebUI: Team page

Follow the existing page pattern: `register(page{...})` in an `init` (see `hub.go:172`, `courses.go`), a template in `internal/localui/templates/team.html`, and the Hub navigation where Courses and Students are linked. Copy tone is warm-plain: a label plus one plain sentence, naming specifics (memory: UI copy tone). The page is for the operator, except the Add form, which the teammate reads.

**`GET /team`** shows:
- A heading: **Team**. "Teammates whose Canvas keys are saved on this PC. CE-SE works in each teammate's courses with that teammate's key."
- Per teammate, a card:
  - name and label;
  - email;
  - key status: one of
    - "Key saved on this PC"
    - "Key isn't on this PC"
    - "Expires 2027-06-04"
    - "Expires soon: 2027-06-04"
    - "Expired 2027-06-04"
  - their configured courses (from `courses.md`, by owner);
  - the last 10 action-log entries, as time, course, method and path;
  - buttons: **Test key**, **Choose courses**, **Remove**.
- An **Add a teammate** button.

**`GET /team/add`** has two parts.

First, an explanation for the teammate, in short steps:
1. "Sign in to Canvas in a private window on this computer."
2. "Go to Account → Settings → **New Access Token**. Purpose: `CE-SE on the team PC`. Expires: the last day of school."
3. "Copy the token, paste it below, then sign out of Canvas and close the private window."

Then a two-sentence "What CE-SE does with it":
> "CE-SE scores only assignments you tag (CE), and scores arrive hidden until you post them. You can stop it any time by deleting this token in Canvas."

Fields:
- **Token**: a password input, `autocomplete=off`.
- **Key expires**: a date input. It defaults to `school-calendar.md` `end_date` when that is set and in the future, and is otherwise empty and required.
There is no label field. The label is computed on save from the last word of the Canvas `short_name`, lowercased and stripped to `[a-z0-9-]`; add `-2`, `-3` on collision. It never changes afterward and only appears in files. The display name is editable on the card.

**`POST /team/add`.** Use the existing CSRF/`checkPost` and secret handling exactly as `/setup` does (`setup.go:205` area). Steps:
1. Build `Credential{Origin: <configured instance>, Token: <form>}`. The origin is not typed by the teammate; it is the configured `instance`. If `instance` is unset or still the placeholder, refuse: "Set up your own Canvas connection first."
2. Call `GET /api/v1/users/self` and `GET /api/v1/users/self/profile` with that key. Take `id`, `short_name` (fallback `name`), and `primary_email` (may be empty).
3. Refuse if `id` equals the operator's `/users/self` id:
   > "That's your own key. Each teammate adds their own key."
   
   Load the operator credential to check. If the operator has no saved key, refuse with the step-1 message.
4. Refuse if that `canvas_user_id` is already in the team list:
   > "Ms. Turner is already on the team. Remove that entry first to replace the key."
   
   Teacher-facing text never uses pronouns for a teammate; repeat the name instead.
5. Save the key with `SaveTeammateCredential`, then `WriteTeam` with the new entry (`name` defaults to `short_name`), then `EnsureTeammateNotes`.
6. If `WriteTeam` fails, delete the just-saved key so no orphan is left.
7. Zero the token string and redirect to `/team/courses?label=…`.

Validation failures re-render the form. Never echo the token back into the page.

**`GET /team/courses?label=…`** lists the teammate's active teacher courses:
- Use the existing `workflow.Courses(ctx, client)` (`canvas_reads.go:315`) with a client built from their key.
- Hide course IDs already configured, whoever owns them.
- Each row has a checkbox, the Canvas name and code, and an **empty, required** nickname input for each checked row. Never infer a nickname from the Canvas title (setup rule). The placeholder text can suggest a form, such as "e.g. Turner P2".

**`POST /team/courses`** appends the selected courses to `courses.md`:
- `owner: <label>` and `current: true`, with no `timezone`. `saveCourses` (`courses.go:133`) sets none for new rows either.
- Nickname uniqueness follows the same rule the existing courses page enforces.
- Write through the same function the courses page uses, so `instance` handling (`SyncInstance`) is unchanged.

**`POST /team/test`** tests a key:
- Loads the key, then calls `/users/self`.
- Success: "Ms. Turner's key works."
- 401: "Canvas no longer accepts Ms. Turner's key. Ask Ms. Turner to make a new one, then remove and re-add."
- Missing key: the single R2 missing-key message.

**`POST /team/remove`** (decided 2026-10-22 during execution: always removes the courses, because an owner missing from the team list stops every command after slice 02). It:
1. removes that teammate's courses from `courses.md` first;
2. deletes the key;
3. removes the team entry;
4. keeps notes, pulled work and the log;
5. reports what it did in one line.

**`POST /team/name`** edits the display name and email. Validate with `Validate()`.

**Courses page.** On the existing `/courses` page, show an **Owner** column (the teammate name, or "You"). Editing the owner there is out of scope; ownership is set only through the Team page.

**Home and health.** Where the Hub home shows connection status (`home.go`, `hub_context.go:174`), add one line per teammate whose key expires soon or has expired (R7). Don't call Canvas for this; use `team.md` dates and `Load…` existence only.

**Injection seams.** The localui tests use function-variable seams (`loadCredentialFn`, `whoamiFn`, …). Add the same for the teammate save, load and delete, `/users/self`, profile and course listing, so tests never touch Credential Manager or Canvas.

### 6. Docs touched in this slice

None beyond CHANGELOG; slice 06 carries the docs. Add a one-line entry to the capability data only if 06 hasn't started. Otherwise leave it to 06.

## Acceptance

- Adding a teammate with a valid fake key writes `team.md`, the notes starter and a saved key (seam). The token appears in no file, log, response body or redirect URL.
- The operator's own key is refused, and so is a duplicate `canvas_user_id`. An unset or placeholder instance refuses.
- A failed `WriteTeam` leaves no saved key.
- Chosen courses land in `courses.md` with `owner`, existing courses are untouched, and nickname collisions are refused.
- Remove removes the courses, deletes the key and entry, and keeps notes and log.
- Expiry states render for: none, soon (exactly 14 days before), and past.
- `team.md` validation rejects each bad field.
- Full tests, vet and the diff check pass. CHANGELOG: "Local tests only."

## Out of scope

Routing (02), consent and hidden grades (03), Collab copy (04), family reply-to (05), and docs and playbooks (06).

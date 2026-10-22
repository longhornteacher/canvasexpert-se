# 02: Per-course key routing

Read [00-BRIEF.md](00-BRIEF.md) first. Contracts R2, R6, R7 and R9 apply here. Requires slice 01. Suggested model: Sonnet.

## Result

Every course-scoped command uses the right key for its course:
- **The operator's key** for the operator's courses, and for owned courses where the operator is an active teacher.
- **Otherwise the owner's key.**

This covers the CLI, MCP (which re-runs the CLI) and the WebUI roster and tags paths.

Courses without an owner behave **exactly** as before, with zero extra requests. Teammate writes are recorded in the action log. A rejected teammate key produces a plain message naming the teammate and course. Scoring sessions with no course only include the operator's own courses.

## Background (from the 2026-10-22 survey)

- One process builds one `*canvas.Client` (`cmd/cse/remote.go:112-125`) and stores it in `workflow.Service.Client`. All workflow code reaches Canvas through `s.Client`, or through package functions given `s.Client`; there are about 300 references in 37 files. **Do not rewrite those references.** Re-scope the `Service` instead, with `ForCourse`.
- Workflow entry points resolve their course in one of three ways:
  - `s.Course(nickname)`;
  - from the document's `course:` field;
  - by matching `doc.Assignment.CourseID` against configured courses (`post.go:63`, `group_post.go:229`, `discussion_work.go:190`).
- Multi-course runs:
  - `pull-work` with no course (`pull_work.go:161-192`);
  - `parent-report` (`parent_report.go:164-165`);
  - `family-contact-prepare` (`family_contact.go:233`, `loadCourse` at :293/:302/:354).
- The WebUI builds clients in `hub_shared.go:27-57` (`canvasClient`, `fetchCourses`, `fetchRoster`) and `students.go:358-368` (`fetchTags`).
- `canvas.Error{Kind:"authentication"}` (status 401) prints "Canvas authentication failed; run cse setup" (`client.go:33`). Canvas also returns 401 when a valid key's owner lacks permission in a course.

## Work

### 1. Canvas client write hook (R6)

In `internal/canvas/client.go`, add the exported field `OnWrite func(method, path string, status int, uncertain bool)` to `Client`.
- Call it in `Write` and `WriteAccepted` (and in slice 03's GraphQL method, when that exists) once the request was sent: with the status for any response, or `uncertain=true` for an uncertain transport result.
- Do not call it for requests refused before sending (validation, a canceled context).
- `path` is the API path without origin.
- No behavior change when the field is nil.

**Test:** the hook sees method, path and status for one success and one 422. The existing client tests still pass.

### 2. Routing core: `internal/workflow/team_route.go`

Add to `Service` (`workspace_reads.go:17`):

```go
Team workspace.TeamConfig
Keys KeySource            // nil: no team routing; every course uses Client
InstallRoot string        // for the action log; empty disables logging
route *routeState         // shared across ForCourse copies; lazily created, mutex-guarded
team  *workspace.Teammate // identity of the routed key for this copy; nil = operator
```

`KeySource` is as in R2. Add an unexported `platformKeys struct{}` implementing it with `platform.LoadCredential` and `platform.LoadTeammateCredential`, plus the exported constructor `func PlatformKeys() KeySource { return platformKeys{} }`.

`func (s *Service) ForCourse(ctx context.Context, course domain.Course) (*Service, error)`:

1. If `course.Owner == ""`, return `s`, unchanged.
2. Find the owner in `s.Team`. If it's missing, return an input error: `course "Turner P2" has owner "turner", who isn't on the team list`. **This applies even when `s.Keys` is nil.**
3. If `s.Keys == nil`, return `s` unchanged: the operator key is routed, and ownership rules still apply through `Owned`.
4. Use the cached decision for `course.ID` if there is one.
5. Otherwise run the **operator teacher check**: `GET /api/v1/courses/<id>` with `s.Client`, decoding only `enrollments []struct{Type, EnrollmentState string}` (JSON `type`, `enrollment_state`).
   - A 200 with an entry where `type == "teacher"` and `enrollment_state == "active"` → decision "operator".
   - A 200 without one, or a `*canvas.Error` with status 401, 403 or 404 → decision "owner".
   - Anything else (throttled, 5xx, uncertain, transport, cancellation) → return the error for this course and **do not cache**.
6. If the decision is "owner":
   1. `s.Keys.Teammate(owner.CanvasUserID)`.
   2. If the key isn't on this PC, return the R2 missing-key message, `<Name>'s Canvas key isn't on this PC. Add it on the Team page, or run this on the team PC.`, as an input error. Define it once as a helper (`missingKeyMessage(name)`) and use that helper in 01's WebUI paths too.
   3. Check that `credential.Origin` matches `s.Config.Instance`, with the same comparison as `remote.go:121-124`.
   4. Build `canvas.New(credential.Origin, credential.Token)` and zero the token.
   5. Set `OnWrite` to append to the action log, with `teammate` = label, `course` = nickname and `course_id`. Print any log failure to stderr **once per process**.
   6. Cache the client by course ID.
7. Return a shallow copy, `cp := *s`, with `cp.Client` set to the routed client and `cp.team` set to the owner when the owner key is used (nil for operator).

Also add:
- `func (s *Service) teammate() *workspace.Teammate` (nil-safe), used by slices 03 and 05.
- `func (s *Service) Owned(course domain.Course) (workspace.Teammate, bool, error)`.
  - Owned means `course.Owner != ""`, with the teammate looked up in `s.Team`.
  - An owner missing from `s.Team` returns an error. Callers treat that as an input error and **never** as unowned.
  - It does not look at `Keys` or the routed key.
  - Slices 03 and 05 key their consent, hiding and reply rules on this.

**Every `Service` constructor site loads the team.** `cmd/cse/remote.go` (step 4) and `cmd/cse-probe/main.go:249` both read `context/team.md` and call `workspace.CheckOwners` before building the Service. cse-probe keeps `Keys` nil (operator key only), but sets `Team`, so the probe's `Post` and Test Student pull apply consent and hiding in an owned course. Test: a probe-style Service with `Keys == nil` and an owned course still requires the tag and sends the post-policy mutation (once 03 lands; add the test in 03).

**Authentication message.** Wrap errors from routed teammate clients so a `*canvas.Error` of kind `authentication` becomes the R2 message, while `errors.As` still finds the `*canvas.Error`:

```go
type teammateAuthError struct{ err *canvas.Error; name, course string }
```

`Error()` returns the R2 text, and it implements `Unwrap()`. The simplest correct place is a helper, `s.wrapErr(err) error`, which entry points apply to returned errors when `s.team != nil`. Choose one approach and apply it consistently at the entry points you scope in step 3.

**Expiry note (R7).** When `ForCourse` routes to an owner key that expires soon, print the R7 line once per teammate per process. Add a `Status io.Writer` field on `Service` (default `io.Discard`); `remote.go` sets it to its status writer.

### 3. Scope every course entry point

Immediately after each exported method resolves its course, add `s, err = s.ForCourse(ctx, course)`. In every case, return early on error, wrapped as in step 2. The list below comes from a 2026-10-22 survey; grep `s.Course(` and the `CourseID ==` matches again to catch new ones.

| Area | Where the course resolves |
|---|---|
| Assignment dates | `assignment_dates.go:25`, `:138` |
| Audience | `audience.go:151`, plus the page/quiz audience and refresh files that resolve via the same helpers |
| Authoring | `author.go:106`, `rubric_attach.go:19`, `quiz_author.go:226`, `quiz_points.go:55`, `page.go:83` and the push path, `course_upload.go:101` |
| Discussions | `discussion.go:56`, `discussion_pull.go:414`, `discussion_work.go:190` |
| Other reads | `external_resources.go:60`, `workspace_reads.go:120` (`PullItem`), `review_edit.go:35` |
| Groups | `groups.go:48`, `:79`, `:163`, `:216`, `group_memberships.go:40`, `:223`, `group_post.go:229` |
| Modules | `module_author.go:80` |
| Scoring | `pull_work.go:104`, `:161` (per course inside the loop), `post.go:63`, `late_prepare.go:245` |
| Family / parent | `parent_report.go:165` (per course inside the loop), `family_contact.go:233` and `loadCourse` (per course) |
| Tier pages | No change. `DeliverTierPages` (`workflow/hub.go:147`) is called only from `author.go:54` and `quiz_hub.go:92` after they have resolved their course. Make sure those callers pass the scoped service or client; don't add a course lookup inside `hub.go`. |

- **Multi-course loops.** Call `ForCourse` per course and use the scoped copy for that course's calls only. Do not reassign the outer `s`.
- **Results that report an error.** `parent_report` and `family_contact` currently abort on a Canvas error. Keep that behavior; only the message improves through the wrapper.
- **`pull-work` with no course** (`pull_work.go:166-172`): filter to `Current && Owner == ""`.
- **Package-level helpers taking `c *canvas.Client`**, such as `Courses`, `Modules` and `StudentEnrollments`, need no change. Callers pass the scoped `s.Client`.
- **Calls that are not course-scoped** (`/api/v1/files/:id`, attachment downloads, `/groups/:id`, progress URLs). These happen inside a course's flow. They must use the **scoped** `s.Client` of that flow. Check `direction_images.go:59/84`, `pull_work.go:772`, `group_pull.go:792` and `new_quiz_work.go:215/301` receive the scoped client. If a helper reaches for a client stored elsewhere, pass the scoped one down.

### 4. CLI (`cmd/cse/remote.go`)

After loading `courses.md` (`:97`):
1. `workspace.ReadTeam(root)`.
2. `workspace.CheckOwners(config, team)`. On error, `fail(inputError(...))`.

Build the Service (`:125`) with:
- `Team: team`
- `Keys: workflow.PlatformKeys()`
- `InstallRoot`: the install root (the same resolution slice 01 used)
- `Status`: the status writer

Leave `Keys` nil only in tests.

Direct-client commands:
- **`modules COURSE`** (`:308`): resolve the course, `ForCourse`, then use the scoped client.
- **`courses`** (`:294`): add `--as <label>`. Change its arity entry in the argument map (`remote.go:27`, currently `"courses": {0,0}`) to allow the two extra args. It loads that teammate's key, builds a client and lists their teacher courses. Unknown label: input error. Missing key: the R2 message.
- **`api`** (`:292`): unchanged, operator only. Update the `canvas_get` MCP tool description to say it uses your own key and doesn't reach a teammate's course.

**Request count.** The status line reads `c.Requests()` for the operator client only. Add the routed clients' counts by keeping a small registry in `routeState`, so the reported total stays honest.

### 5. WebUI routing (`internal/localui`)

- `fetchRoster(courseID)` (`hub_shared.go:47`) and `fetchTags(courseID)` (`students.go:358`) are given an ID. Look up the configured course by ID. If it is owned, route with the same rule. Easiest is to build a `workflow.Service{Root, Client: operatorClient, Config, Team, Keys: workflow.PlatformKeys(), InstallRoot}` and call `ForCourse`. Put this in one helper, `h.clientForCourse(ctx, courseID)`, and use it in both places.
- Missing or rejected key: the page shows the R2 message for that course and leaves the other courses working.
- The catalog (`fetchCourses`) stays operator-only. Teammate catalogs come only from the Team page (slice 01).

### 6. MCP surfaces (R9)

In `internal/mcpserver/server.go`, `configuredCourses` (`:423`) gains `Team []teamStatus \`json:"team"\``, built from `team.md` read through the same readable-path rule:
- `key_on_this_pc` comes from `platform.LoadTeammateCredential`; zero the token immediately.
- `expires_soon` follows R7.
- An empty team is `[]`.

`owner` already flows through `domain.Course`. Add one sentence to the `list_courses` tool description: courses with an `owner` belong to that teammate, and the `team` list says whether their key is on this PC.

### 7. Token-aware fixture and tests

Add a fixture helper, `teamFixture(t, handler)`, next to `workflowFixture` (`workflow_test.go:20`):
- **Courses.** Two configured courses on the same TLS server: `MINE` (ID 1, no owner) and `THEIRS` (ID 2, `owner: turner`).
- **Keys.** A fake `KeySource`: the operator token is `OP-TOKEN`; Turner's (`canvas_user_id` 77) is `TURNER-TOKEN`.
- **Server rules.**
  - Any request whose path contains `/courses/2` must carry `Bearer TURNER-TOKEN`; otherwise 401.
  - Any request containing `/courses/1` must carry `OP-TOKEN`.
  - `GET /api/v1/courses/2` with `OP-TOKEN` returns 401, so the operator isn't a teacher there.
  - A variant returns `enrollments:[{type:"teacher",enrollment_state:"active"}]` for the co-teaching case.

Representative tests, each a single command against `THEIRS` that must succeed using Turner's token:
- push assignment
- push page
- push module
- push quiz
- assignment-dates pull and push
- pull-work for one assignment
- post (comment-only is enough)
- group-set pull
- family-contact-prepare
- parent-report
- upload-file

Rules:
- `MINE` makes no `GET /api/v1/courses/1` check request (zero added requests). Assert on the request count.
- In the co-teaching variant, `THEIRS` uses `OP-TOKEN` and writes no action-log line.
- A teammate write appends a redacted log line. An operator write appends none.
- A missing teammate key gives the R2 message, and Canvas receives no request for that course.
- A 401 from Turner's client gives the R2 wording, and `errors.As(err, &*canvas.Error)` still works.
- `pull-work` with no course pulls only `MINE`.
- `CheckOwners` rejects an unknown owner.
- An operator teacher check that returns 503 fails that course with no teammate request, and a second call retries the check (not cached).
- `Keys == nil` with an owned course routes to the operator client, and `Owned` still reports the course as owned.

**WebUI test:** `fetchRoster` for an owned course uses the teammate seam.

## Acceptance

- All existing tests pass unchanged. The no-owner path makes no extra requests.
- The token-aware fixture tests above pass.
- `cse courses --as turner` lists with the teammate key.
- `list_courses` shows `owner` and `team`, with no key material.
- CHANGELOG `### Per-course key routing` section, ending "Local tests only."

## Out of scope

`(CE)` and hidden grades (03), Collab copy (04), reply-to (05), docs and playbooks (06).

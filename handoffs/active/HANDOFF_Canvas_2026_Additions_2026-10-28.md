# HANDOFF: Canvas 2026 additions: direction-image download, New Quiz accommodations, differentiation tags, shared rubrics

Date: 2026-10-28. To: whoever picks this up in Dev (CE-SE repo, `dev` branch). The teacher works on several PCs, so run `git fetch` and bring `dev` up to date before starting (root `AGENTS.md`).

This document stands alone. It comes from a review of Canvas changes between October 2025 and October 2026 (Canvas API change logs, release notes and developer docs), checked against the current Engine. Sources are listed in §9.

Execution review: 2026-10-28 against the current Dev working tree and official API references. **Ready for 6.1 Terra or 6.1 Sol one part at a time, with the evidence dependencies below.** Read the [shared execution notes](../README.md#execution-notes-for-the-three-active-handoffs). This review changes the plan only; it supplies no new Canvas Live evidence.

---

## 0. TL;DR

Five pieces of work, in five parts that can be built one at a time. **B before C**; the others are independent.

| Part | What | Engine / playbook | Size |
|---|---|---|---|
| A | Download direction images with the Canvas token, before Canvas ends tokenless file links | Engine, one call site | Tiny |
| B | Engine tool to set New Quiz accommodations (extra minutes, extra attempts, reduced choices) for named students on one quiz | Engine and a few `quizzes.md` lines | Small |
| C | Accommodations applied automatically from each student's saved settings: on every quiz the Engine builds, and on request to quizzes built by hand | Engine, WebUI Students page, playbook lines | Medium |
| D | Create, rename and delete differentiation tags, and set who is in them, in bulk | Engine and playbook | Medium |
| E | Shared rubrics: attach an existing Canvas rubric, a teacher rubric library across courses and PCs, and an Enhanced Rubrics check before 2026-12-19 | Engine and playbook | Medium |

Suggested order: **A** (do now), **E §6.1 Enhanced Rubrics check** (deadline 2026-12-19), **B → C**, **D**, then the rest of **E**.

| Startable work | Actual dependency / stopping point |
|---|---|
| A | Ready now; no live probe required. |
| E enhanced-rubric observation | Needs a concrete authorized disposable check; its absence does not block A or public-API development. |
| B | Implement the typed tool and local fixtures from documented routes. Settle omission, retry and effective-value semantics in §3.1 before enabling those behaviors; report the specific unknown rather than guessing defaults. |
| C | Depends on B's safe setting semantics and on knowing how course/quiz accommodations combine. Course-level sync (§4.5) is frozen until the teacher asks. |
| D | Implement structure/read/preview from the public contract. Resolve membership/response behavior in §5.1 before relying on it; protect in-use deletions conservatively until observed. |
| E reuse/library | Implement public list/detail/association paths now. Account visibility and association are optional extensions of the course-rubric path, not blockers for it. |

Do not turn this into one all-or-nothing release. Finish and record each independent slice; leave only unfinished parts active. No feature should claim live evidence from a fake server.

---

## 1. Rules for every part

- **Engine vs playbook.** The Engine handles Canvas access, identity, exact computation and safe writes. Choosing which students, which rubric or which tag stays judgment in the playbooks. Do not put judgment in the Engine.
- **Public repo.** Use course nicknames (ELA7, ELA7 PAP, CS8) and "the teacher" only. No real course IDs, names or Windows usernames in code, tests, docs or this file's successors. Tests use synthetic IDs (e.g. 900001). Real IDs live in UserSync `dev-reference`.
- **Canvas Live.** Every live write in a probe needs the teacher's fresh, concrete go-ahead in that session. Use CS8 and disposable objects. Ordinary assignment/tag checks may use the built-in **Test Student**. A New Quiz attempt/timer check needs a confirmed **teacher-controlled student account**, because the built-in Test Student cannot launch New Quizzes (see [team evidence](../../docs/team/README.md)). Create quizzes unpublished, restrict to that account, then publish only when an attempt is needed and authorized. Never test on a real student's attempt. List disposable objects in [cs8-test-objects.md](../../docs/cs8-test-objects.md); exact private IDs belong in the private dev reference. Live reads use the temporary `cmd/tmpdiag` pattern in [development-notes.md](../../docs/development-notes.md), which never prints tokens or student names.
- **Accepted write = done.** No read-backs after accepted writes, no "partially complete" messages for writes that landed, and a one-line report. Keep the cheap pre-write guards: exact course and instance, exact student identity, active enrollment, wrong-target refusal, and no replay of accepted writes. (A probe in §3.1 or §5.1 is dev testing, so reading back there is fine.)
- **Privacy.** Accommodations and tag memberships are sensitive. Explicit teacher-requested operations may identify students by stable ref/alias and the requested values, never real names or a reason. Automatic/from-saved accommodation success and preview output contains counts only; a failed student may be identified by alias with a sanitized actionable error, without disclosing saved settings. Never forward raw Canvas error bodies that can contain identities or private values. Saved settings stay out of ordinary MCP reads and packets; the Engine reads them privately.
- **Docs that change together.** For each part, update `internal/localui/capabilities.go` (status data and review date) and [engine-capabilities.md](../../docs/engine-capabilities.md). Add new document kinds and tool contracts to `engine-docs/formats.md`. Keep Canvas Live evidence, local tests and pending human checks separate. Local tests alone are not live proof.
- **Checks.** Use the fresh-build commands in the shared execution notes. Add discovery/schema checks for each new tool, not only a tool-count change (53 at this review; other handoffs change it). Keep tests in proportion to changed behavior.
- **Routing.** All course operations use `Service.ForCourse`; read [team/README.md](../../docs/team/README.md) before touching routing. Accommodations, tags and rubric authoring are content/settings operations, not a reason to impose scoring-only `(CE)` consent. Any rubric-assessment probe uses the existing scoring guards.

---

## 2. Part A: direction images through the authenticated download

### Why

Canvas announced that unauthenticated file downloads end and that the `verifier` URL parameter is removed from file links. The date was 2026-03-25, then 2026-06-17, and is now "TBD". When Canvas enforces it, a file link with a `verifier` and no Canvas token stops downloading.

### What is affected (corrected)

The earlier review said both direction images and New Quiz file uploads would break. **Only direction images are affected:**

| Call site | URL source | Affected? |
|---|---|---|
| `internal/workflow/direction_images.go:84` `DownloadUnauthenticated(ctx, file.URL, f)` | Canvas Files API `GET /api/v1/files/:id` → `url` (a `/files/:id/download?download_frd=1&verifier=…` link on the Canvas host) | **Yes**: this is exactly the link Canvas is ending |
| `internal/workflow/new_quiz_work.go:109` (Student Analysis results) | Pre-signed `inst-fs` storage URL from the report's Progress object | No: signed storage, not a Canvas file link |
| `internal/workflow/new_quiz_work.go:301` (New Quiz upload bytes) | Pre-signed `amazonaws.com` URL from the quiz host (see [new-quiz-written-work-spec.md](../../docs/new-quiz-written-work-spec.md), steps 7–9) | No: signed storage, must stay tokenless |
| `pull_work.go:820`, `group_pull.go:797` (submission attachments) | Canvas attachment URL | Already use authenticated `Client.Download` |

### Fix

In `direction_images.go`, replace `s.Client.DownloadUnauthenticated(ctx, file.URL, f)` with `s.Client.Download(ctx, file.URL, f)`.

`Client.Download` (`internal/canvas/client.go:601`) already does the right thing. It sends the token only to the same origin (`send` → `sameOrigin`). Its `CheckRedirect` (client.go:94) removes `Authorization` on every hop, permits the cross-host redirect to file storage only for downloads (`X-CSE-Download`), and restores the token only on same-origin hops. So the token never reaches the storage host.

Leave both New Quiz call sites on `DownloadUnauthenticated`. Its comment, and the written-work spec, say the signed storage GET must carry no authorization.

### Test

Update `direction_images_test.go` so the fake Canvas file endpoint **requires** the bearer token (401 without it) and then redirects to a fake storage host that **rejects** any `Authorization` header. One test covers both the new requirement and the no-leak rule.

### Evidence

Record it as local tests. The next real scoring pull of an assignment with an image in its directions becomes the Canvas Live observation; no probe is needed.

---

## 3. Part B: New Quiz accommodations tool

### What Canvas offers (official developer docs, GA, no beta label)

- **Per quiz:** `POST /api/quiz/v1/courses/:course_id/quizzes/:assignment_id/accommodations`
  - Body: a JSON **array** of `{ "user_id": int, "extra_time": int (minutes, 0–10080), "extra_attempts": int, "reduce_choices_enabled": bool }`.
- **Per course:** `POST /api/quiz/v1/courses/:course_id/accommodations`
  - Body: an array of `{ "user_id", "extra_time", "apply_to_in_progress_quiz_sessions": bool, "reduce_choices_enabled" }`. It has **no `extra_attempts`**.
- **Response (both):** `200` with `{ "message", "successful": [{user_id}], "failed": [{user_id, error}] }`. A partial result is still 200.
  - `400`: validation error.
  - `401`: no permission.
  - `404`: course or quiz not found.
- `reduce_choices_enabled` removes one wrong answer from multiple-choice items with 4 or more choices.
- **Not documented:** any GET, list or DELETE route; whether a POST replaces or merges a student's existing accommodation; how per-quiz and course-level values combine; whether course-level applies to quizzes created later. Also, the docs' curl examples use `/api/v1/...` while the route table says `/api/quiz/v1/...`.

The Engine already uses `/api/quiz/v1/courses/%d/quizzes` as the quiz base (`quiz_author.go:236`).

### 3.1 Resolve semantics (CS8, teacher-controlled student account, disposable timed New Quiz)

It needs the teacher's go-ahead for writes. Record answers with dates here before enabling behavior that depends on them. Typed adapters/local fixtures can proceed independently:

1. Which base path works: `/api/quiz/v1` or `/api/v1`?
2. Try a GET on both accommodation routes (read-only). If one returns current values, note its shape. It may support preview, pre-write preservation and uncertain-write recovery, never routine confirmation after an accepted write.
3. POST `extra_time: 10`, then POST the same student with only `reduce_choices_enabled: true`. Is `extra_time` kept (merge) or reset (replace)?
4. Does `extra_time: 0` / `reduce_choices_enabled: false` clear it?
5. Does an unpublished quiz accept accommodations?
6. On a quiz with a single allowed attempt, does `extra_attempts: 1` give a second attempt?
7. With a course-level `extra_time` set, check an **existing** quiz and a quiz **created afterwards**. Does each show it on the Moderate page? When per-quiz and course-level are both set, which wins, or do they add? Record the false/omitted behavior of `apply_to_in_progress_quiz_sessions`; automatic sync must not extend an in-progress attempt without a teacher request.
8. Repeat exactly the same `extra_attempts: 1` POST. Does it set the allowance to one extra attempt or increment it again? Do not infer retry safety from the field name. Record partial success response identities and whether the response actually reports effective values.

Check the results in the Canvas Moderate page and in the teacher-controlled student's session. Course-level writes can affect that account's other quizzes, so include that scope and restoration of its previous settings in the probe authorization. If the account is unavailable, record the attempt/timer checks as pending and continue independent work. Delete or list the disposable quiz.

### 3.2 Engine

- **Tool:** a new MCP action tool `set_quiz_accommodations` (CLI `cse quiz-accommodations`) that takes `{course, quiz_id, students: [{student, extra_time_minutes?, extra_attempts?, reduce_choices?}], preview?}`.
  - CLI: `cse quiz-accommodations COURSE QUIZ_ID REQUEST.json [--preview]`, where the file contains `{ "students": [...] }`. `quiz_id` is the associated assignment ID. Validate the whole input before writing; reject duplicate students, empty changes and unknown keys. Use presence-aware fields so explicit `0`/`false` differ from omission.
  - `student` is a stable student ref or a unique privacy alias from the saved roster (the same resolution other tools use; ambiguous or missing → error, never a guess).
  - Omitted fields are not sent when merge semantics are established. If the API replaces values, preserve omitted fields from a reliable current-value read. If that read is unavailable, hold incomplete requests and ask for the full intended replacement; **never substitute defaults for unknown existing accommodations**. Document the observed choice in `formats.md`.
- **Pre-write guards:**
  - Configured course and instance.
  - `quiz_id` is a New Quiz in that course: one GET of `/api/quiz/v1/courses/:c/quizzes/:id`.
  - Each student is an active student enrollment in that course.
  - Extra time is a whole number 0–10080; extra attempts is a nonnegative whole number (do not invent an undocumented maximum); reduced choices is boolean.
  - Refuse the whole call on any guard failure, before writing.
- **Write:** one bulk POST. Interpret the documented `successful`/`failed` arrays even on HTTP 200. Map identities back to requested students; an omitted, duplicate or unknown response identity is unresolved, never counted as success. Report successful/failed counts and sanitized per-student errors. A partial failure must not hide accepted students. Make no automatic retry and no post-success read-back.
  - A 5xx or timeout is uncertain. Recommend an explicit rerun only for fields proven to be absolute setters by §3.1; otherwise direct inspection in Moderate. Never say an unobserved increment is safe to replay.
- **Preview:** resolve students and show the intended values (and current values when §3.1 found a GET). Make no write.
- **No sidecar** for this explicit tool. The teacher's request is the record, and Canvas's Moderate page shows the state.

### 3.3 Playbook

Add a short section to `workspace-template/playbooks/quizzes.md`: when the teacher asks for extra time, another attempt or fewer choices for named students on a quiz, use `set_quiz_accommodations`. The request already includes the write, so no separate go-ahead is needed. Report in one line with counts and aliases, never reasons. Also add the tool to `engine_reference` help.

### 3.4 Implementation and local acceptance

Use a small `internal/workflow/quiz_accommodations.go` operation and the existing Canvas JSON transport; the body is a top-level array, not an `accommodations` object. Add CLI help/arity/dispatch and the typed adapter in `internal/mcpserver/operations.go`. Focused fake-server tests cover exact routed course/quiz/student identity, omitted versus zero/false fields, preview making no write, partial-200 outcomes, sanitized output and uncertain POST without retry. Do not add a course-sync tool; §4.5 is frozen.

---

## 4. Part C: accommodations applied automatically

### The idea

A teacher records a student's quiz accommodation once, on the Students page. From then on:

- every New Quiz the Engine builds applies it before the quiz is published;
- a quiz the teacher built by hand gets it with one request ("apply accommodations to the Unit 3 quiz");
- (course-level sync for quizzes built in Canvas is frozen; see §4.5.)

The agent never needs to know who has an accommodation.

### 4.1 Do not reuse the existing "Extra time" checkbox

The current **Extra time** column (`students.csv` column `extra_time`, a `bool`) means **more school days for a deadline**. Late-timing preparation reads it (`late_prepare.go`, scoring packets `extra_time`), and its tooltip says Canvas due dates don't change. Quiz timer minutes are a different accommodation. Keep that column and its meaning unchanged.

### 4.2 Saved settings

Add two private per-student fields to the students table (`internal/workspace/students.go`), in the active teacher-setup authority (UserSync when configured):

| Field | Values | Meaning |
|---|---|---|
| `quiz_time` | empty (standard), `50%`, `100%`, or whole minutes such as `15` | Extra time on timed New Quizzes |
| `reduced_choices` | `true`/`false` | Canvas "reduce choices" on every New Quiz |

- **CSV header.** Append the two columns after `alias_source`, preserving an explicit variant for today's 11-column header as well as the older variants. Absent columns mean standard. `OpenStudents` currently triggers alias migration from `len(rows[0]) < len(studentHeader)`; replace that condition with the actual pre-`alias_source` versions so adding columns does not rename established aliases. Parse the new cells like the existing flags: an invalid value fails loading with a row-specific error naming the field (the WebUI select cannot produce one; only a hand edit can). Never silently treat an invalid value as standard. Do not build raw-invalid-cell retention or a new row-error UI for this.
- **WebUI Students page** (`internal/localui/students.go`, `templates/students.html`). Add a **Quiz time** select (Standard / Time and a half / Double time / Extra minutes…, where the last shows a small number input) and a **Fewer choices** checkbox.
  - Add both to `data-draft-fields`.
  - Tooltips follow the UI copy tone (concrete, plain, warm). Suggested:
    - Quiz time: "More time on timed New Quizzes. CanvasExpert adds it to each quiz it builds, before students can see the quiz."
    - Fewer choices: "Canvas removes one wrong answer from multiple-choice questions with 4 or more choices."
  - The phone-width (≤40rem) table rules and print output must stay as they are; print excludes these columns like other settings.
- **MCP privacy.** These fields stay out of agent packets and MCP reads, like contacts. `students.go` save and load fixtures must cover that exclusion.

Use the existing `ResolveRulesPath` authority for the table and Students save handler; a roster refresh or alias/monitoring save must preserve the two new columns. Do not serialize the private `Student` struct wholesale into MCP responses. Whole-minute entries are positive and at most 10080; standard is empty, not the deadline `ExtraTime` flag. For percentage computations over the API maximum, hold rather than cap away an accommodation.

### 4.3 Applying on every quiz the Engine builds (`push_quiz`)

- Add an `accommodations` stage to `quizStages` (`quiz_author.go`). It runs **after items and settings and before publication**, and before the module stage if that is simpler; it must come before publication. Today `Stages` exist only for restricted quizzes ("Absent for ordinary quizzes"). Create `Stages` for any quiz where at least one enrolled student has a non-standard saved accommodation, so a resume skips an accepted stage. Old sidecars stay valid.
- **Computation (Engine, exact):**
  - **Percent:** `extra_time = ceil(time_limit_minutes × pct)` when the quiz has a time limit.
  - **Untimed quiz:** no `extra_time` is sent.
  - **Minutes:** sent as entered, but only when the quiz is timed.
  - **`reduced_choices`:** send a saved `true` regardless of timing. A saved standard/false value does not clear a manual Canvas setting; explicit clearing uses Part B. If replacement semantics require other fields, preserve known current values or hold rather than inventing them.
  - The time limit comes from the QuizForge settings the Engine just applied (`QuizSettings`); no extra read is needed.
- **Students:** active student enrollments in the course whose saved settings are non-standard. Resolve stable refs to user IDs from the saved roster. For a quiz restricted to a club audience (`quiz_audience.go`), include only the audience's students.
- **Record:** the public quiz sidecar's `stages.accommodations` holds status, count and a digest, without student refs or settings. This stage sends only `extra_time` and `reduce_choices_enabled` (never `extra_attempts`). If §3.1 Q3/Q4 show those are absolute setters, a rerun after a partial 200 or an uncertain POST resends the whole computed array: replaying accepted students sets the same values again and is harmless, so no per-student store is needed. Only if §3.1 shows otherwise, add the smallest private per-student outcome record in UserInstall `.cse/` (never through MCP reads), and note why here. Do not create a general job store.
- **Failure holds publication.** If Canvas rejects any student, or the write is uncertain, the quiz stays unpublished. The report names the affected students by alias, and a rerun of the same file resumes from this stage.
  - Publishing a timed quiz without a student's required accommodation is a consequential, easy-to-miss failure, so this is one of the cheap checks `AGENTS.md` keeps.
  - The playbook explains how to continue: fix it, or explicitly request publication anyway. Expose `--skip-accommodations` on `push-quiz` / optional `skip_accommodations` on MCP `push_quiz` for that invocation only; do not save it as a permanent QuizForge preference. Record an explicit skipped stage, not a successful application. Other audience/publication guards still apply.
- **Report:** one line, for example "Accommodations applied to 4 students" (count only). Make no read-back.

### 4.4 Applying to a quiz built by hand

Add `from_saved: true` to Part B's tool (`set_quiz_accommodations {course, quiz_id, from_saved: true}`), mutually exclusive with `students`; CLI uses `--from-saved` in place of `REQUEST.json`. It reads the quiz's time limit and effective individual audience, computes per student as in §4.3, and posts. Never give a restricted quiz's accommodations to students outside its audience. Preview/success output shows counts only. A saved Standard/false value is not permission to clear settings entered by hand in Canvas; expose explicit zero/false clearing through B and report that limitation. Reruns follow the same rule as §4.3.

### 4.5 Course-level sync: frozen

**Frozen 2026-10-28 by the developer: "maybe nice for later", built only if the teacher asks.** §4.3 covers every quiz the Engine builds and §4.4 covers a hand-built quiz on one request, so course-level sync would add only automatic coverage of quizzes built in Canvas, at the cost of a private applied-values record, clearing rules and a double-extra-time guard. Do not build it as part of Part C. Keep §3.1 Q7: Part C still needs to know whether a course-level value the teacher set by hand in Canvas adds to a quiz-level value the Engine sends, and if it does, §4.3/§4.4 report that conflict rather than doubling the time. The notes below are kept for a future request.

- If course-level accommodations apply to quizzes created later, add `sync_quiz_accommodations {course, preview?}`. It posts course-level values for students with **minutes** or `reduced_choices`; percent cannot be expressed course-wide, so percent students rely on §4.3 and §4.4, and the preview says so.
- **Clearing.** Record **what CE-SE applied** per course in a private file beside the students table in the same authority, never in model context. This record proves prior CE-SE intent, not current Canvas ownership. Clear automatically only when a reliable current-value read still matches the recorded value and the saved setting changed to standard. Without that read, report that the teacher must explicitly request the zero/false replacement or clear it in Canvas; never promise that a later manual edit is protected by a local record alone.
- Prevent double extra time: when course and quiz values add, do not apply both for the same student/quiz. If current effective course values cannot be read reliably, hold that affected automatic application and explain the conflict. Do not invent subtraction logic from stale local state. Course sync sends `apply_to_in_progress_quiz_sessions: false` by default; a change to an ongoing attempt needs an explicit request.
- If §3.1 shows course-level does **not** reach new quizzes, skip §4.5 entirely and note that in this file.

### 4.6 Playbook

- `quizzes.md`: accommodations from Students apply automatically; report counts only; a held publication explains its reason; use `from_saved` for a quiz built in Canvas.
- `setup.md` (Students step): one sentence on where to record quiz accommodations.
- No agent judgment about who needs accommodations: the teacher's saved settings are the authority.

### Evidence

- **Local:** CSV compatibility, the WebUI save, percent and minutes math, the untimed case, audience-only students, the stage resume, held publication, and privacy exclusion.
- **Canvas Live:** one Engine-built timed quiz on CS8 with the teacher-controlled account set to Time and a half, checked on the Moderate page and in that account's session. Pending account access does not block local implementation; it must remain labeled unobserved.

---

## 5. Part D: bulk differentiation tags

### Current boundary (this part deliberately changes it)

Today tags are read-only. Several places say so, and all must be updated:

- `list_tags` says "Tags are teacher-owned; this Engine never creates or edits them" (`internal/mcpserver/operations.go:180`);
- the `tags.go` header comment;
- the `assign_to` description ("never creates or edits tags");
- [ARCHITECTURE.md](../../docs/ARCHITECTURE.md) §Current scope ("teacher-managed Silver/Red/Blue tags in ELA7 only").

The new rule: **the Engine creates, renames and deletes tags and sets memberships only when the teacher asks.** It never invents a level for a student. Which students go in which tag is the teacher's decision; the agent may draft a placement from evidence, but the teacher approves it before the write.

### What Canvas offers

- **Structure, atomic:** `POST /api/v1/courses/:course_id/group_categories/bulk_manage_differentiation_tag`.
  - `group_category`: `{id?}` for an existing tag set or `{name}` for a new one; id plus name renames the set.
  - `operations`: `{create:[{name}], update:[{id,name}], delete:[{id}]}`.
  - If anything fails, the whole operation rolls back. Only for `non_collaborative` categories.
- **Who has which tags:** `GET /api/v1/courses/:course_id/bulk_user_tags?user_ids[]=…` maps each user ID to a list of tag group IDs. One call replaces the per-group membership listing in `differentiation.go`. The response shape is described but not shown; confirm it in the probe.
- **Add members:** `POST /api/v1/groups/:group_id/memberships` with `members[]` (a bulk add, for differentiation tags only).
- **Remove members:** `DELETE /api/v1/groups/:group_id/users/:user_id`. A bulk `DELETE /api/v1/groups/:group_id/users` also exists, but its parameters are undocumented, so use the per-user route unless the probe shows the bulk one plainly.
- **Not used:** `PUT /api/v1/groups/:id` with `members[]` replaces the whole membership and sends invitations, so don't use it. The CSV import (`import_tags`) is also out, because its column format is undocumented.
- Tags are always on now: the feature option was removed on 2025-10-22, and the account setting governs.

### 5.1 Probe first (CS8, Test Student, one disposable tag set)

1. Create a set with two tags in one bulk call; rename one; delete one. Confirm the response shape.
2. Recreate the deleted tag so two exist. Add the Test Student to tag 1 with `members[]`; read `bulk_user_tags`; move the student to tag 2 (add, then remove). Can a student be in **two tags of the same set** at once, and does adding to tag 2 implicitly remove tag 1? Record this before defining move order.
3. Assign a disposable page to tag 1 with `assign_to`, then delete tag 1. What happens to the page's Assign To row, and does the student lose access? This decides the delete guard below.
4. Clean up and record the evidence.

### 5.2 Engine

Use `kind: tags`, validated by the existing document validator, and a dedicated `set_tags {path, preview?}` action tool / `cse set-tags FILE [--preview]`. This gives preview a narrow typed surface without extending every `push_document` operation. Reuse the assign-to report style and document I/O, not its post-success readback policy. Preview performs no Canvas writes; it may save exact resolved target IDs and the membership/usage baseline into Engine-owned document fields while preserving the teacher's authored content.

```yaml
schema: 1
kind: tags
course: ELA7
set: {name: Reading level}    # optional id pins an existing set; new name creates it
tags:
  - name: Support
  - name: Core
  - name: Old tier
    delete: true
  - name: Blue
    rename_to: Accelerate     # final names must be unique; do not also create Accelerate
members:                      # optional
  mode: replace               # replace = exactly these students in each listed tag; add = only add
  Support: [s-ref-1, s-ref-2]
  Core: [s-ref-3]
```

Add Engine-owned instance/course/set/tag IDs, a relevant live baseline and outcomes on preparation. Permit an explicit tag `id` to disambiguate an existing name; never pick the first duplicate. Resolve membership keys against final names after renames. In `replace` mode, only listed tags get exactly the named students; unlisted tags stay unchanged. `add` never removes. An explicit empty list clears a listed tag only through the ordinary removal guards. Reject duplicate final names, delete-plus-rename/create for the same tag, membership requests for deleted tags and duplicate/conflicting placements before writing. `set.rename_to` renames an ID-pinned set; a name typo must not rename another set.

- **Order:**
  1. One bulk structure call for all creates, renames and deletes. It is atomic: accepted means all of it.
  2. Membership adds per tag (one `members[]` call each).
  3. Removals per student.
  - Save outcomes after each accepted stage, and resume only unfinished rows.
  - This order assumes adding a member does not implicitly remove a protected membership. If §5.1 observes implicit moves, evaluate the old membership's access/submission guard **before the add**. Never remove the old membership when the destination add failed or is uncertain. Do not invent a two-tag intermediate state if Canvas forbids it.
- **Guards before any write:**
  - Exact course and instance.
  - The set and every tag are `non_collaborative`, using the existing `nonCollaborative` check; refuse any collaborative group.
  - Students come from the saved roster as active enrollments.
  - Names resolve exactly as `TagCatalog.Resolve` does: ambiguous means an error.
- **Removals that take away access.** Removing a student from a tag, or deleting a tag, that Assign To uses for submitted work holds unless the document explicitly sets `allow_remove_with_submissions`. Reuse the relevant reads from `assign_to.go` and `audience_report`, but note that `allowRemove` is an audience-write option, not an existing tag-membership protection function. Account for direct and module-inherited usage. Unknown usage/submission coverage cannot prove a removal safe; hold that removal while independent safe work proceeds. Until §5.1 Q3 settles what deletion does, treat deleting an in-use tag as removing its members from every item that uses it, under the same `allow_remove_with_submissions` guard; the preview already lists those items for the teacher. Do not add a second flag. A blocked delete holds the single atomic structure batch, not just a fictitious sub-operation in an already-sent batch. Independent membership work on unaffected existing tags can continue.
- **Preview.** For each tag: current count → new count, aliases added/removed and items using it. Re-read the relevant names/memberships/usage before applying; if those inputs changed, preserve the draft and return a changed preview for the affected action. A teacher request specifying the exact changes already authorizes them; an agent-proposed placement needs the teacher's approval. No Engine approval token is required.
- **Copying a set to another course.** Same set and tag names, no members. Have the playbook write a `kind: tags` document for the destination course, so the Engine needs no new mechanism. A teacher with ELA7 and ELA7 PAP will want this.
- **Reads.** Switch `DifferentiationTags` (Students page column) to `bulk_user_tags` if the probe confirms the shape; that is one request instead of one per tag.
- **Names in saved documents.** Saved `assign-to` documents and Hub records hold tag IDs alongside names, so a rename doesn't break them. Confirm that `assign_modules_by_tag` (name pattern) reports a clear "no tag matches" after a rename rather than guessing.

**Recovery and code map.** Add domain validation and one `tag_manage.go` workflow using `tags.go`, `differentiation.go` and `audience_report.go` (locate current symbols if files move). Read [group-work contracts](../../docs/group-work/contracts.md) before borrowing membership helpers; collaborative group writes stay out of scope. Resume by re-reading, not a recovery store: before writing, resolve the live set with `TagCatalog`; a create or rename whose final name already resolves exactly once is done, a delete whose tag is gone is done, and a duplicate name is the existing ambiguity error for the teacher to inspect. Memberships compare against `bulk_user_tags` the same way, so a rerun adds and removes only what is still different. Save returned IDs into the document for reporting. Test the advertised partial-failure behavior, including destination-add failure protecting the old membership, a rerun after an uncertain structure call creating no duplicate tag, and a held delete making no structure POST.

### 5.3 Playbook

- Add a short "Differentiation tags" section to `native-content.md`, or wherever `assign_to` guidance lives.
- Use cases:
  - set up a tag set;
  - re-level students after an assessment (the teacher decides; the agent can propose a placement from scores, clearly labeled as a suggestion);
  - move one student;
  - copy a set to another course.
- Always preview first. Report counts and aliases, never a reason for a student's placement.
- WebUI: change the Students page "Canvas tag" tooltip from "managed in Canvas" to "Set in Canvas, or ask Claude or ChatGPT to change it." Do not build tag editing into the WebUI unless the teacher asks.

### Evidence

- **Local:** fake Canvas for structure, adds and removals, the hold on submitted work, collaborative refusal, and resume.
- **Canvas Live:** the §5.1 probe plus one real teacher-approved re-level.

---

## 6. Part E: shared rubrics

### Current behavior

Every rubric write creates a **new** course rubric from a full inline definition and associates it with `use_for_grading: false`:

- `rubricCreateFields` in `author.go:612`;
- `discussion.go:558`;
- `rubric_attach.go:98`.

The Engine never edits a rubric definition and attaches only when the assignment has no rubric. That rule stays. The result is a course Rubrics list full of near-duplicates, and no way to reuse a district or department rubric.

### 6.1 Enhanced Rubrics check (do first; deadline 2026-12-19)

Canvas makes Enhanced Rubrics mandatory on **2026-12-19** and drops Classic Rubrics. It has been on by default since 2026-03-21, so the October CS8 rubric runs most likely already used it, but that isn't recorded. The REST rubric docs list no API change. "Attach multiple rubrics" was dropped from the feature, which doesn't affect the Engine because it attaches one rubric.

On CS8, with the teacher's go-ahead:

1. Confirm the course shows Enhanced Rubrics.
2. On one disposable assignment visible only to the Test Student, run: create and attach, then post a rubric assessment with comment through the ordinary results path, then attach a rubric to a disposable New Quiz.

Record the results as Canvas Live evidence. If all pass, this is evidence only; no code changes. Anything that fails becomes the first fix in this part.

### 6.2 Read existing rubrics

- **Tool:** a new read tool `list_rubrics {course, include_account?}`. It lists course rubrics (`GET /api/v1/courses/:c/rubrics`) and, when asked and permitted, account rubrics (`GET /api/v1/accounts/:a/rubrics`; the account ID comes from the course).
  - Output per rubric: id, title, points, criteria count, context (course/account).
  - An account 401/403 is reported as "your Canvas account can't see school rubrics", not an error.
- **Detail:** `pull_rubric {course, rubric_id}` writes the full definition, in the existing `domain.Rubric` YAML shape, to the course drafts folder.
  - It includes where the rubric is used (`GET .../rubrics/:id/used_locations`). That response's fields are undocumented; confirm them live, and count only.

### 6.3 Attach an existing rubric

- **Document syntax.** Assignment, discussion and New Quiz attach documents accept `rubric: {id: N}` with no criteria.
- **Write.** The Engine attaches with `POST /api/v1/courses/:course_id/rubric_associations`, sending:
  - `rubric_association[rubric_id]`;
  - `rubric_association[association_id]`;
  - `rubric_association[association_type]: Assignment`;
  - `rubric_association[use_for_grading]: false`;
  - `rubric_association[purpose]: grading`.

  Equivalently send one nested `rubric_association` JSON object. These parameter names and the direct RubricAssociation response were verified in the [official Rubrics API](https://developerdocs.instructure.com/services/canvas/resources/rubrics) on 2026-10-28. Do not parse this response as the `{rubric, rubric_association}` envelope used by rubric creation. This is documented API evidence, not a live association check.
- **Guards:**
  - The assignment has no rubric yet; reuse `pushRubricAttachment`'s hold.
  - The rubric exists in this course or its account; one GET.
  - Probe whether a teacher token can associate an **account** rubric with a course assignment. If not, the playbook copies its definition into the course instead (§6.4 path).
- **Scoring.** The scoring and post paths already read the assignment's rubric settings, so a shared association needs no scoring change. Add a local test that posts an assessment against an assignment whose rubric is also used by another assignment.

### 6.4 Teacher rubric library (across courses and PCs)

Most teachers can't create account rubrics, so "shared" usually means "my SCR rubric in all three of my courses".

- **Library.** `context/rubrics/<name>.md` in the active rules authority (UserSync when configured). Its `rubric-library` front matter wraps the existing rubric definition (title, criteria, free-form comments) as specified in §6.4a; a short teacher description follows it.
  - The agent writes entries with `write_workspace_file` when the teacher says "save this rubric".
  - The seed ships the folder empty, with a README line.
- **Use.** A document may say `rubric: {library: "SCR 4-point"}`. At push the Engine:
  1. loads that file, refusing a missing or ambiguous name;
  2. lists course rubrics and looks for one with the same title **and** an identical definition (reuse `sameRubricDefinition`);
  3. attaches it if found, as in §6.3, or else creates it once in that course and attaches it.
  - Record the rubric ID and a digest of the library entry in the document's outcomes, so a resume doesn't create a second copy.
- **Edits.** When the library entry changes and the course copy no longer matches, the Engine **creates a new course rubric** for new attachments. It never edits the existing Canvas rubric, because past assessments use it. The report says "SCR 4-point changed; new attachments use the new version."
- **Playbooks.** `authoring.md` and `score.md` cover offering a library rubric when one fits, saving a new one when the teacher likes it, and not saving silently.

### 6.4a Contracts and implementation sequence

1. **Read surfaces.** CLI `cse rubrics COURSE [--include-account]` / MCP `list_rubrics`; CLI `cse pull-rubric COURSE RUBRIC_ID` / MCP `pull_rubric`. Use `Service.ForCourse` and pagination. Fetch detail when a list response lacks criteria; do not report an unknown count as zero. Missing account permission or unavailable `used_locations` yields a specific limitation while preserving the usable course definition. A detail pull saves a new draft rather than overwriting an existing unsent one. Do not request or export assessments/student data.
2. **Reference versus definition.** Authored rubric inputs accept exactly one of full inline definition, `{id: N}`, or `{library: "file stem"}`. Resolve a reference to a full definition before existing validation/scoring consumers need it. Do not loosen the existing `domain.Rubric` requirements for pulled assignment snapshots or scoring bases. New Quiz attachments continue through the existing assignment `operation: attach-rubric` path; do not invent a second rubric inside QuizForge JSON. Graded discussions use their associated assignment ID.
3. **Portable entry.** A library file has `schema: 1`, `kind: rubric-library`, and `rubric: {title, free_form_criterion_comments, criteria}` in front matter, with the teacher description below. Preserve criterion/rating content and portable local keys but strip course-specific rubric/association IDs and grading-coupling flags on export. The library key is the exact filename stem under `context/rubrics/`, not a search of rubric titles. Reject separators/traversal and resolve through `ResolveRulesPath`; never fall back to stale local copies when UserSync is unavailable. Reuse guarded `write_workspace_file` for saving the entry. Provide one synthetic example in `engine-docs/examples/`.
4. **Reuse.** Call `sameRubricDefinition(live, authored)` in that order; it normalizes authored ratings on its second argument. Ignore remote identity when comparing definitions but preserve criteria order, maxima, rating descriptions and free-form mode. If several identical course rubrics exist, use a deterministic existing ID (lowest is sufficient); do not create another. Same title with a different definition creates a new rubric for new attachments. Never replace an already attached rubric or rewrite past assessments.
5. **Outcomes.** Reuse `CreatedResources`/attachment recovery in `rubric_attach.go`, and the author/discussion creation stages. Persist the resolved library digest and selected rubric ID before the association stage; save accepted association IDs separately. A rerun never recreates a known accepted rubric. An uncertain create/attach uses the existing bounded recovery/hold pattern. A library edit during an unfinished operation holds that operation for a new draft instead of silently switching definitions. New association writes do not change assignment points and do not trigger a post-success verification GET; preserve existing attachment protection without copying its old confirmation-read behavior into the new path.
6. **Acceptance.** Tests must exercise assignment creation, graded-discussion creation and attach-to-existing-assignment/New-Quiz dispatch with a rubric reference, not just a standalone resolver. Cover local/UserSync library reads, same-definition reuse, new-version creation and no replay after an accepted create plus failed attach. Finish the format/playbook/MCP examples and paired capability entries for the surfaces that actually landed.

### 6.5 Out of scope for this part

Rubric CSV upload (`POST .../rubrics/upload`; the column format is undocumented, and the library covers the need), editing existing Canvas rubrics, outcome alignment, and curves.

### Evidence

- **Local:** list and pull, attach by ID, library reuse, dedupe and versioning, and the hold when a rubric is already attached.
- **Canvas Live:** §6.1, plus one library rubric attached to two assignments in CS8.

---

## 7. Docs to update when each part lands

- `internal/localui/capabilities.go` and `docs/engine-capabilities.md` (each part; new rows for accommodations, tag management and shared rubrics; update the Files row for Part A and the Rubric row for §6.1).
- `engine-docs/formats.md`: `set_quiz_accommodations`, the quiz `accommodations` stage, `set_tags` / `kind: tags`, `rubric: {id}` / `rubric: {library}`, `kind: rubric-library` and the new read tools. Keep student settings out of examples.
- `docs/ARCHITECTURE.md` §Current scope: tags no longer read-only; shared rubrics no longer "future".
- `workspace-template/playbooks/`: `quizzes.md`, `setup.md`, `native-content.md` (or the assign-to playbook), `authoring.md`, `score.md`.
- `scripts/check-mcp.mjs`: the tool count.
- `docs/cs8-test-objects.md`: every disposable probe object.
- This file: probe answers (§3.1, §5.1, §6.1) with dates. Move it to `handoffs/retired/` with a banner when done.

---

## 8. Done when

- **A:** direction images download with the token and never leak it to storage; local test green.
- **B:** a teacher can say "give [student] 15 more minutes and another attempt on the Unit 3 quiz" and it happens in one call, with a one-line report.
- **C:** a student marked Time and a half gets the right extra minutes on every timed quiz the Engine builds before publication, and on a hand-built quiz by request. Routine automatic output does not disclose the student or setting; failures expose only the minimum alias/reason needed to fix them. Saved-data compatibility, partial success, privacy and the existing deadline Extra time meaning are preserved.
- **D:** a teacher can set up, re-level and copy differentiation tags by asking, with a preview, and without stranding a student from work they submitted.
- **E:** Enhanced Rubrics verified live before 2026-12-19. A rubric saved once can be attached in any course without duplicates, and an existing Canvas rubric can be attached by ID.

Each part ends with its capabilities row and evidence record updated, distinguishing Canvas Live from local tests.

---

## 9. Sources

- [2026 API and CLI Change Log](https://community.instructure.com/en/discussion/664378/2026-api-and-cli-change-log) and [Enforcements, Deprecations, and Breaking Changes](https://community.instructure.com/en/kb/articles/664261-instructure-enforcements-deprecations-and-breaking-changes) (file verifier removal; dates)
- [New Quizzes Accommodations API](https://developerdocs.instructure.com/services/canvas/resources/new_quizzes_accommodations)
- [Group Categories API](https://developerdocs.instructure.com/services/canvas/file.all_resources/group_categories) (`bulk_manage_differentiation_tag`, `import_tags`)
- [Groups API](https://developerdocs.instructure.com/services/canvas/resources/groups) (`members[]`, `bulk_user_tags`, membership delete)
- [Rubrics API](https://developerdocs.instructure.com/services/canvas/resources/rubrics) (account rubrics, `used_locations`, upload)
- [Enhanced Rubrics overview](https://community.instructure.com/en/discussion/664713/enhanced-rubrics-feature-overview) and [release notes 2026-03-21](https://community.instructure.com/en/kb/articles/664347-canvas-release-notes-2026-03-21) (mandatory 2026-12-19)

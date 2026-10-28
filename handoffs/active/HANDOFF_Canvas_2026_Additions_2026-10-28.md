# HANDOFF: Canvas 2026 additions: direction-image download, New Quiz accommodations, differentiation tags, shared rubrics

Date: 2026-10-28. To: whoever picks this up in Dev (CE-SE repo, `dev` branch). The teacher works on several PCs, so run `git fetch` and bring `dev` up to date before starting (root `AGENTS.md`).

This document stands alone. It comes from a review of Canvas changes between October 2025 and October 2026 (Canvas API change logs, release notes and developer docs), checked against the current Engine. Sources are listed in §9.

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

---

## 1. Rules for every part

- **Engine vs playbook.** The Engine handles Canvas access, identity, exact computation and safe writes. Choosing which students, which rubric or which tag stays judgment in the playbooks. Do not put judgment in the Engine.
- **Public repo.** Use course nicknames (ELA7, ELA7 PAP, CS8) and "the teacher" only. No real course IDs, names or Windows usernames in code, tests, docs or this file's successors. Tests use synthetic IDs (e.g. 900001). Real IDs live in UserSync `dev-reference`.
- **Canvas Live.** Every live write in a probe needs the teacher's fresh, concrete go-ahead in that session. Use CS8 with the built-in **Test Student** and disposable objects; list each disposable object in [cs8-test-objects.md](../../docs/cs8-test-objects.md). Live reads use the temporary `cmd/tmpdiag` pattern in [development-notes.md](../../docs/development-notes.md), which never prints the token or student names.
- **Accepted write = done.** No read-backs after accepted writes, no "partially complete" messages for writes that landed, and a one-line report. Keep the cheap pre-write guards: exact course and instance, exact student identity, active enrollment, wrong-target refusal, and no replay of accepted writes. (A probe in §3.1 or §5.1 is dev testing, so reading back there is fine.)
- **Privacy.** Accommodations and tag memberships are sensitive (IEP, 504, English learner, intervention). Tool output names students by stable ref and privacy alias, never by real name, and **never states a reason**. The Engine reads the saved settings itself; the agent does not need them in context.
- **Docs that change together.** For each part, update `internal/localui/capabilities.go` (status data and review date) and [engine-capabilities.md](../../docs/engine-capabilities.md). Add new document kinds and tool contracts to `engine-docs/formats.md`. Keep Canvas Live evidence, local tests and pending human checks separate. Local tests alone are not live proof.
- **Checks.** `go test ./...`, `go vet ./...`, and the compiled MCP smoke check (`scripts/check-mcp.mjs`). Each new tool changes its asserted count on line 59 (currently 53). Keep tests in proportion to the changed behavior; no test matrices for edge cases the teacher would notice and could cheaply fix.

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

### 3.1 Probe first (CS8, Test Student, one disposable timed New Quiz with 2 attempts)

It needs the teacher's go-ahead for writes. Answer these and write the answers into §3 of this file before building:

1. Which base path works: `/api/quiz/v1` or `/api/v1`?
2. Try a GET on both accommodation routes (read-only). If one returns current values, note its shape. It may be used for a **preview** only, never as a read-back after a write.
3. POST `extra_time: 10`, then POST the same student with only `reduce_choices_enabled: true`. Is `extra_time` kept (merge) or reset (replace)?
4. Does `extra_time: 0` / `reduce_choices_enabled: false` clear it?
5. Does an unpublished quiz accept accommodations?
6. On a quiz with a single allowed attempt, does `extra_attempts: 1` give a second attempt?
7. With a course-level `extra_time` set, check an **existing** quiz and a quiz **created afterwards**. Does each show it on the Moderate page? When per-quiz and course-level are both set, which wins, or do they add?

Check the results in the Canvas Moderate page and in a Test Student session. Record the answers with the date as Canvas Live evidence. Delete or list the disposable quiz.

### 3.2 Engine

- **Tool:** a new MCP action tool `set_quiz_accommodations` (CLI `cse quiz-accommodations`) that takes `{course, quiz_id, students: [{student, extra_time_minutes?, extra_attempts?, reduce_choices?}], preview?}`.
  - `student` is a stable student ref or a unique privacy alias from the saved roster (the same resolution other tools use; ambiguous or missing → error, never a guess).
  - Omitted fields are not sent (or, if §3.1 shows replace semantics, are sent as their current value from the preview GET when one exists, else as the documented default). Choose this from the probe result and state it in `formats.md`.
- **Pre-write guards:**
  - Configured course and instance.
  - `quiz_id` is a New Quiz in that course: one GET of `/api/quiz/v1/courses/:c/quizzes/:id`.
  - Each student is an active student enrollment in that course.
  - Values within the documented ranges.
  - Refuse the whole call on any guard failure, before writing.
- **Write:** one bulk POST. Report successful students by alias, and failed students by alias with Canvas's `error` text. Make no retry and no read-back.
  - A 5xx or timeout is reported as uncertain. Because the call sets values rather than adding to them, the teacher can simply rerun it; say so in the message.
- **Preview:** resolve students and show the intended values (and current values when §3.1 found a GET). Make no write.
- **No sidecar** for this explicit tool. The teacher's request is the record, and Canvas's Moderate page shows the state.

### 3.3 Playbook

Add a short section to `workspace-template/playbooks/quizzes.md`: when the teacher asks for extra time, another attempt or fewer choices for named students on a quiz, use `set_quiz_accommodations`. The request already includes the write, so no separate go-ahead is needed. Report in one line with counts and aliases, never reasons. Also add the tool to `engine_reference` help.

---

## 4. Part C: accommodations applied automatically

### The idea

A teacher records a student's quiz accommodation once, on the Students page. From then on:

- every New Quiz the Engine builds applies it before the quiz is published;
- a quiz the teacher built by hand gets it with one request ("apply accommodations to the Unit 3 quiz");
- if §3.1 shows course-level accommodations reach future quizzes, one sync covers quizzes built in Canvas too.

The agent never needs to know who has an accommodation.

### 4.1 Do not reuse the existing "Extra time" checkbox

The current **Extra time** column (`students.csv` column `extra_time`, a `bool`) means **more school days for a deadline**. Late-timing preparation reads it (`late_prepare.go`, scoring packets `extra_time`), and its tooltip says Canvas due dates don't change. Quiz timer minutes are a different accommodation. Keep that column and its meaning unchanged.

### 4.2 Saved settings

Add two private per-student fields to the students table (`internal/workspace/students.go`), in the active teacher-setup authority (UserSync when configured):

| Field | Values | Meaning |
|---|---|---|
| `quiz_time` | empty (standard), `50%`, `100%`, or whole minutes such as `15` | Extra time on timed New Quizzes |
| `reduced_choices` | `true`/`false` | Canvas "reduce choices" on every New Quiz |

- **CSV header.** Append the two columns after `alias_source`. Add a new header variant to the existing header list (`studentHeader`, `extraTimeStudentHeader`, `legacyStudentHeader`) so older tables still read; absent columns mean standard. Unparseable values keep the raw text and report it on the Students page, like the existing invalid-entry handling. Never drop a teacher's entry.
- **WebUI Students page** (`internal/localui/students.go`, `templates/students.html`). Add a **Quiz time** select (Standard / Time and a half / Double time / Extra minutes…, where the last shows a small number input) and a **Fewer choices** checkbox.
  - Add both to `data-draft-fields`.
  - Tooltips follow the UI copy tone (concrete, plain, warm). Suggested:
    - Quiz time: "More time on timed New Quizzes. CanvasExpert adds it to each quiz it builds, before students can see the quiz."
    - Fewer choices: "Canvas removes one wrong answer from multiple-choice questions with 4 or more choices."
  - The phone-width (≤40rem) table rules and print output must stay as they are; print excludes these columns like other settings.
- **MCP privacy.** These fields stay out of agent packets and MCP reads, like contacts. `students.go` save and load fixtures must cover that exclusion.

### 4.3 Applying on every quiz the Engine builds (`push_quiz`)

- Add an `accommodations` stage to `quizStages` (`quiz_author.go`). It runs **after items and settings and before publication**, and before the module stage if that is simpler; it must come before publication. Today `Stages` exist only for restricted quizzes ("Absent for ordinary quizzes"). Create `Stages` for any quiz where at least one enrolled student has a non-standard saved accommodation, so a resume skips an accepted stage. Old sidecars stay valid.
- **Computation (Engine, exact):**
  - **Percent:** `extra_time = ceil(time_limit_minutes × pct)` when the quiz has a time limit.
  - **Untimed quiz:** no `extra_time` is sent.
  - **Minutes:** sent as entered, but only when the quiz is timed.
  - **`reduced_choices`:** sent regardless of timing.
  - The time limit comes from the QuizForge settings the Engine just applied (`QuizSettings`); no extra read is needed.
- **Students:** active student enrollments in the course whose saved settings are non-standard. Resolve stable refs to user IDs from the saved roster. For a quiz restricted to a club audience (`quiz_audience.go`), include only the audience's students.
- **Record:** `stages.accommodations` with status and a digest of the sent `(ref, extra_time, reduce_choices)` set, not the values themselves. That lets a later apply detect a change without storing sensitive detail in the sidecar.
- **Failure holds publication.** If Canvas rejects any student, or the write is uncertain, the quiz stays unpublished. The report names the affected students by alias, and a rerun of the same file resumes from this stage.
  - Publishing a timed quiz without a student's required accommodation is a consequential, easy-to-miss failure, so this is one of the cheap checks `AGENTS.md` keeps.
  - The playbook explains how to continue: fix it, or tell the Engine to publish anyway with a `skip_accommodations` flag passed only on the teacher's say-so.
- **Report:** one line, for example "Accommodations applied to 4 students" (count only). Make no read-back.

### 4.4 Applying to a quiz built by hand

Add `from_saved: true` to Part B's tool (`set_quiz_accommodations {course, quiz_id, from_saved: true}`). It reads the quiz's time limit (one GET), computes per student exactly as in §4.3, and posts. Preview shows counts and aliases. This also covers a quiz the Engine built before the teacher saved a new accommodation.

### 4.5 Course-level sync (only if §3.1 shows it reaches new quizzes)

- If course-level accommodations apply to quizzes created later, add `sync_quiz_accommodations {course, preview?}`. It posts course-level values for students with **minutes** or `reduced_choices`; percent cannot be expressed course-wide, so percent students rely on §4.3 and §4.4, and the preview says so.
- **Clearing.** No GET is documented, so record **what CE-SE applied** per course: a private file beside the students table in the same authority, never in model context. A later sync sends zero/false only for students CE-SE previously set and the teacher has since changed to standard. Accommodations a teacher set by hand in Canvas are never cleared.
- If §3.1 shows course-level does **not** reach new quizzes, skip §4.5 entirely and note that in this file.

### 4.6 Playbook

- `quizzes.md`: accommodations from Students apply automatically; report counts only; a held publication explains its reason; use `from_saved` for a quiz built in Canvas.
- `setup.md` (Students step): one sentence on where to record quiz accommodations.
- No agent judgment about who needs accommodations: the teacher's saved settings are the authority.

### Evidence

- **Local:** CSV compatibility, the WebUI save, percent and minutes math, the untimed case, audience-only students, the stage resume, held publication, and privacy exclusion.
- **Canvas Live:** one Engine-built timed quiz on CS8 with the Test Student set to Time and a half, checked on the Moderate page and in a Test Student session.

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
2. Add the Test Student to tag 1 with `members[]`; read `bulk_user_tags`; move the student to tag 2 (add, then remove). Can a student be in **two tags of the same set** at once?
3. Assign a disposable page to tag 1 with `assign_to`, then delete tag 1. What happens to the page's Assign To row, and does the student lose access? This decides the delete guard below.
4. Clean up and record the evidence.

### 5.2 Engine

Add a new document kind, `kind: tags`, pushed with `push_document`, or a dedicated `set_tags` tool if that fits the existing assign-to pattern better. Follow `assign_to`'s shape: preview first, per-row outcomes, no replay.

```yaml
kind: tags
course: ELA7
set: Reading level            # existing set name or ID; a new name creates the set
tags:
  - name: Support
  - name: Core
  - name: Accelerate
  - name: Old tier
    delete: true
  - name: Blue
    rename_to: Accelerate     # either a rename or a create, not both for one name
members:                      # optional
  mode: replace               # replace = exactly these students in each listed tag; add = only add
  Support: [s-ref-1, s-ref-2]
  Core: [s-ref-3]
```

- **Order:**
  1. One bulk structure call for all creates, renames and deletes. It is atomic: accepted means all of it.
  2. Membership adds per tag (one `members[]` call each).
  3. Removals per student.
  - Save outcomes after each accepted stage, and resume only unfinished rows.
- **Guards before any write:**
  - Exact course and instance.
  - The set and every tag are `non_collaborative`, using the existing `nonCollaborative` check; refuse any collaborative group.
  - Students come from the saved roster as active enrollments.
  - Names resolve exactly as `TagCatalog.Resolve` does: ambiguous means an error.
- **Removals that take away access.** Removing a student from a tag, or deleting a tag, that Assign To uses for an item where the student has submitted work: **hold** that row unless `allow_remove_with_submissions` is set. Reuse the `assign_to` check (`assign_to.go`, `allowRemove`) and `audience_report` usage data. If the probe shows deleting a tag silently strips Assign To rows, a delete of an in-use tag holds unless `allow_delete_in_use` is set.
- **Preview.** For each tag: current count → new count, the aliases added and removed, and which items use the tag (from the audience report). Show it to the teacher, then write on their go-ahead.
- **Copying a set to another course.** Same set and tag names, no members. Have the playbook write a `kind: tags` document for the destination course, so the Engine needs no new mechanism. A teacher with ELA7 and ELA7 PAP will want this.
- **Reads.** Switch `DifferentiationTags` (Students page column) to `bulk_user_tags` if the probe confirms the shape; that is one request instead of one per tag.
- **Names in saved documents.** Saved `assign-to` documents and Hub records hold tag IDs alongside names, so a rename doesn't break them. Confirm that `assign_modules_by_tag` (name pattern) reports a clear "no tag matches" after a rename rather than guessing.

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
  - `association_id`;
  - `association_type: Assignment`;
  - `use_for_grading: false`;
  - `purpose: grading`.

  That is the same "independent rubric, automatic grading off" rule as today. This route is in the Rubrics API docs but was not in the review's fetch, so confirm the exact parameter names live.
- **Guards:**
  - The assignment has no rubric yet; reuse `pushRubricAttachment`'s hold.
  - The rubric exists in this course or its account; one GET.
  - Probe whether a teacher token can associate an **account** rubric with a course assignment. If not, the playbook copies its definition into the course instead (§6.4 path).
- **Scoring.** The scoring and post paths already read the assignment's rubric settings, so a shared association needs no scoring change. Add a local test that posts an assessment against an assignment whose rubric is also used by another assignment.

### 6.4 Teacher rubric library (across courses and PCs)

Most teachers can't create account rubrics, so "shared" usually means "my SCR rubric in all three of my courses".

- **Library.** `context/rubrics/<name>.md` in the active rules authority (UserSync when configured). It holds YAML front matter in the existing `domain.Rubric` shape (title, criteria, free-form comments) plus a short teacher description.
  - The agent writes entries with `write_workspace_file` when the teacher says "save this rubric".
  - The seed ships the folder empty, with a README line.
- **Use.** A document may say `rubric: {library: "SCR 4-point"}`. At push the Engine:
  1. loads that file, refusing a missing or ambiguous name;
  2. lists course rubrics and looks for one with the same title **and** an identical definition (reuse `sameRubricDefinition`);
  3. attaches it if found, as in §6.3, or else creates it once in that course and attaches it.
  - Record the rubric ID and a digest of the library entry in the document's outcomes, so a resume doesn't create a second copy.
- **Edits.** When the library entry changes and the course copy no longer matches, the Engine **creates a new course rubric** for new attachments. It never edits the existing Canvas rubric, because past assessments use it. The report says "SCR 4-point changed; new attachments use the new version."
- **Playbooks.** `authoring.md` and `score.md` cover offering a library rubric when one fits, saving a new one when the teacher likes it, and not saving silently.

### 6.5 Out of scope for this part

Rubric CSV upload (`POST .../rubrics/upload`; the column format is undocumented, and the library covers the need), editing existing Canvas rubrics, outcome alignment, and curves.

### Evidence

- **Local:** list and pull, attach by ID, library reuse, dedupe and versioning, and the hold when a rubric is already attached.
- **Canvas Live:** §6.1, plus one library rubric attached to two assignments in CS8.

---

## 7. Docs to update when each part lands

- `internal/localui/capabilities.go` and `docs/engine-capabilities.md` (each part; new rows for accommodations, tag management and shared rubrics; update the Files row for Part A and the Rubric row for §6.1).
- `engine-docs/formats.md`: `set_quiz_accommodations`, the quiz `accommodations` stage, `kind: tags`, `rubric: {id}` / `rubric: {library}`.
- `docs/ARCHITECTURE.md` §Current scope: tags no longer read-only; shared rubrics no longer "future".
- `workspace-template/playbooks/`: `quizzes.md`, `setup.md`, `native-content.md` (or the assign-to playbook), `authoring.md`, `score.md`.
- `scripts/check-mcp.mjs`: the tool count.
- `docs/cs8-test-objects.md`: every disposable probe object.
- This file: probe answers (§3.1, §5.1, §6.1) with dates. Move it to `handoffs/retired/` with a banner when done.

---

## 8. Done when

- **A:** direction images download with the token and never leak it to storage; local test green.
- **B:** a teacher can say "give [student] 15 more minutes and another attempt on the Unit 3 quiz" and it happens in one call, with a one-line report.
- **C:** a student marked Time and a half on the Students page gets the right extra minutes on every timed quiz the Engine builds, before publication, and on a hand-built quiz by request. The agent never sees who or why.
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

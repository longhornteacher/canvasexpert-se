# 04: Copy a Collab module into teachers' courses

Read [00-BRIEF.md](00-BRIEF.md) first. Contract R5 applies here. Requires slice 02. Suggested model: Sonnet.

## Result

`cse copy-module Collab "Unit 3: Argument" "My P1" "My P3" "Turner P2" "Turner P4"` (MCP `copy_module`) does the following:
- Copies the Collab module titled exactly "Unit 3: Argument", with its items, into each listed course, using Canvas's selective course copy and each course's routed key.
- Records the outcome per course and **never copies the same module into the same course twice**.
- Reports one line per course.

Teachers who prefer to import from Collab themselves keep doing that in Canvas; this command doesn't change it.

## Canvas flow per destination

All requests use the destination's routed client (`ForCourse`). Destinations run concurrently, at most 4 at a time, each fully independent.

1. **Record pending** (see Records) before any write.
2. **Create:**
   ```
   POST /api/v1/courses/:dest/content_migrations
   {"migration_type":"course_copy_importer","settings":{"source_course_id":"<src id>"},"selective_import":true}
   ```
   - Save the returned `id` into the record at once.
   - A known failure (`canvas.Error`, not uncertain) means status `failed`, with a plain message.
   - Uncertain means status `uncertain`. Stop that destination.
3. **Wait for the selection step.** Poll `GET /api/v1/courses/:dest/content_migrations/:id`.
   - Continue when `workflow_state` is `waiting_for_select`.
   - `failed` → `failed`.
   - Deadline 2 minutes → `uncertain`, with the message `Canvas hadn't prepared the copy after 2 minutes; check Import Course Content in <course>.`
4. **Find the module.**
   ```
   GET /api/v1/courses/:dest/content_migrations/:id/selective_data?type=context_modules
   ```
   - Find the single entry whose `title` equals the module title exactly.
   - Zero or several matches → `failed`: `Canvas's copy list didn't show exactly one "Unit 3: Argument"; finish it by hand in <course>.` The unfinished migration stays in Canvas; that's acceptable, and the message says so.
5. **Select.** Build the selection from the entry's `property` **and every `property` in its `sub_items`, recursively**.
   - Each property is a bracket path such as `copy[context_modules][id_abc123]`. Convert each to nested JSON with value `"1"`.
   - `PUT /api/v1/courses/:dest/content_migrations/:id` with that `{"copy":{…}}` body.
   - Known failure → `failed`. Uncertain → `uncertain`.
6. **Wait for completion.** Poll the same migration GET.
   - `completed` → go to step 7.
   - `failed` → `failed`.
   - Deadline 15 minutes → `uncertain`: `The copy was still running when CE-SE stopped waiting; check Import Course Content in <course> before doing anything else.`
7. **Issues.** `GET /api/v1/courses/:dest/content_migrations/:id/migration_issues`. Keep each `description`, truncated to 200 characters, in the record and result. These are Canvas warnings about the content, never student data.
8. **Locate the copy.** List the destination's modules (`workflow.Modules`) and find exact-title matches.
   - Exactly one: record `dest_module_id`.
   - Otherwise: record the count in the message. The status stays `succeeded`, because the copy completed.
   - This is identity discovery for later edits, not a confirmation read.
9. **Record `succeeded`** with `finished_at`.

**Polling.**
- Interval: 2 seconds for the first 30 seconds, then 5 seconds.
- Respect `ctx` cancellation. Cancellation during a wait → `uncertain`.
- `new_quiz_work.go:68-104` has an existing one-off poll loop. Write a small shared helper, `pollUntil(ctx, every func(elapsed) time.Duration, deadline time.Duration, check func() (done bool, err error)) error`, in the workflow package and use it here. Refactoring the New Quiz loop to use it is optional: do it only if trivial and covered by its existing tests.

**If Stop and report applies:** the brief's last item covers selective copy needing more than module plus `sub_items` properties. Local tests can't prove the property shape. If the documentation or any recorded evidence contradicts it, stop and report. Otherwise implement as written; slice 07 confirms it live.

## Pre-flight (before any destination starts)

All of the following use the **source** course's routed client. Normally the source is the operator's own course (Collab), which has no owner.

1. **Courses.**
   - Source and destination nicknames resolve with `s.Course`.
   - Remove duplicate destinations.
   - The source must not appear among the destinations.
   - At least one destination.
2. **Module.** List the source modules; an exact title match must be unique.
   - Zero matches: `No module titled "Unit 3: Argument" in Collab.`
   - Several: `Collab has 2 modules titled "Unit 3: Argument"; rename one first.`
3. **Items.** List the module's items (`GET /api/v1/courses/:src/modules/:id/items?per_page=100`, paginated).
   - Empty: `"Unit 3: Argument" in Collab has no items yet.`
   - If any item `title` satisfies `domain.HasConsentTag`, stop: `Remove (CE) from "Essay 3" in Collab first. Each teacher adds (CE) in their own course.`
4. **Copy once only.** Read the record. Every destination with any existing entry is refused, with the R5 message (date and status from the entry). The other destinations proceed.
5. **Keys.** Route every remaining destination (`ForCourse`) before starting any copies. A destination whose routing fails, such as a missing key, is reported with its message, gets **no** record entry, and the others proceed.

## Records

Path: `courses/_team-copies/course-<source course id>-module-<source module id>.json` in UserInstall. It is keyed by Canvas IDs, so renaming a Collab nickname can't lose it.

**Lock.** The whole run (pre-flight through the last destination) holds an exclusive lock at `courses/_team-copies/.copy.lock`. Reuse the existing delivery-lock pattern: find it with `grep -rn "delivery.lock" internal`.
- If the lock is held, refuse: `Another copy is running; try again when it finishes.`
- This stops two overlapping runs with different course orders from both passing the copy-once check.

**Per machine.** Records are local to UserInstall. The playbook says to run copies from the team PC. Don't sync records.

```json
{
  "schema": 1,
  "source": {"nickname": "Collab", "course_id": 119000, "module_id": 456, "module_title": "Unit 3: Argument"},
  "destinations": [
    {"course_id": 120700, "nickname": "Turner P2", "teammate": "turner",
     "status": "succeeded", "migration_id": 9001,
     "started_at": "2026-10-22T14:00:00Z", "finished_at": "2026-10-22T14:03:10Z",
     "dest_module_id": 7788, "issues": [], "message": ""}
  ]
}
```

- Keyed by destination `course_id`.
- `teammate` is the owner label, or empty for the operator's own courses.
- Statuses: `pending`, `succeeded`, `failed`, `uncertain`. A leftover `pending` from a crashed run is treated like `uncertain` for the copy-once rule.
- **Concurrency.** Destinations update the record concurrently. Guard it with a mutex and rewrite the whole file with the repo's existing atomic-write helper on each change.

## Surfaces

**CLI.**
- `copy-module SOURCE MODULE COURSE [COURSE...]` in `cmd/cse/remote.go`, plus the command list and help text.
- Output is a table: Course, Teacher (name or "You"), Status, Issues, Message. It's followed by: `Copied as in Collab: due dates and published state are unchanged.`
- Exit code 3 if any destination isn't `succeeded`, matching how partial posts exit.

**MCP.**
- `copy_module {source_course: string, module: string, courses: []string}`, through `longCommand` like `pull_parent_report`.
- Description: "Copies one Collab module, by exact title, once into each listed course, using each course's teacher key. Never re-copies. State the module and the course list to the teacher before calling."
- Update the compiled-tool count in `scripts/check-mcp.mjs:59` (41 → 42).

**Not added:** no review page and no `--force`/re-copy flag.

## Tests

Use the fake Canvas from 02's token-aware fixture, with the source course as operator-owned and destinations both owned and unowned.

**Happy path**, two destinations:
- correct keys per destination;
- the POST body;
- a selection PUT containing the module and both nested sub-item properties as nested JSON;
- completion;
- issues recorded;
- `dest_module_id` recorded.

**Pre-flight refusals:**
- missing module;
- duplicate titles;
- an empty module;
- an item with `(CE)`: no destination request is made at all;
- the source among the destinations.

**Copy once:**
- a second run to the same destination is refused for every prior status: `succeeded`, `failed`, `uncertain` and leftover `pending`;
- a new destination still proceeds.

**Failures:**
- POST 422 → `failed`;
- POST 500 → `uncertain`, with no PUT;
- `waiting_for_select` never arrives → `uncertain`. Use an injectable clock or short deadlines in tests;
- selective data without a match → `failed`, with no PUT;
- migration `failed` → `failed`;
- one destination failing doesn't stop the other.

**Missing key:** one destination with a missing teammate key → reported, no record entry, and the others proceed.

**Lock and rename:**
- A run while the lock is held is refused, with no Canvas requests.
- After renaming the source nickname in the config, a second run to the same destination is still refused (the record is keyed by ID).

**Logging:** teammate destinations log their writes in the action log; operator destinations don't.

## Acceptance

The tests pass, plus full tests, vet, the diff check and the MCP smoke with the new count. CHANGELOG `### Copy a Collab module`, ending: "Local tests only; the selective-copy property shape, New Quiz and file behavior, and assignment-group placement are pending live check (07)."

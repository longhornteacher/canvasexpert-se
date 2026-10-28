> **Retired 2026-10-27 — implemented in Dev.** Historical instructions follow; do not execute them again. Local focused fixtures, the full uncached Go test suite and vet passed. A read-only Dev Engine pull succeeded on an affected ELA7 Page. On an unpublished disposable CS8 New Quiz, an assignment PUT changed group and base dates, and assignment and New Quizzes API reads agreed; the quiz was deleted. The new quiz Engine edit and module removal/replacement have local test evidence, not live Engine write evidence. No release or UserInstall update was performed; the teacher's engine-notes entries can be removed after a release containing these fixes reaches UserInstall.

# HANDOFF: Close three gaps in editing existing Canvas content

Date: 2026-10-27. From: Claude Code session in Dev. To: an agent working in the CE-SE repo.

All three gaps are **Engine** changes, with small playbook updates noted per gap. They come from the teacher's UserInstall `context/engine-notes.md` (entries dated 2026-10-27), written during real ELA 7 / PAP ELA 7 book-club work. Course IDs for the nicknames below are in UserSync `dev-reference/course-ids.md`; keep real course IDs and the teacher's name out of the repo.

Read root `AGENTS.md`, [ARCHITECTURE.md](../../docs/ARCHITECTURE.md) and the [group-work README](../../docs/group-work/README.md) before starting. Gap 1 touches audience/date-details code.

---

## 0. TL;DR

1. **`pull_page` refuses Pages that inherit a module audience.** The Engine says "observed date-details visibility is contradictory". The cause is confirmed (§1): it's a wrong validation assumption, not odd Canvas data. 30 study-guide Pages in both ELA courses are affected.
2. **The module document can't remove or replace an existing item.** `push` on a module only adds items. Swapping a PDF File item for an ExternalUrl needs a manual delete in Canvas.
3. **An existing New Quiz can't change its assignment group or base dates.** `PlanEdit` refuses every edit on an `external_tool` assignment except `attach-rubric`. Copied quizzes landed in the 0%-weight Imported Assignments group, and the teacher had to move them and set unlock/due/lock by hand.

Suggested order: 1, 3, 2. Gap 1 is a bug fix; gap 3 blocks grading weight; gap 2 has a cheap manual workaround.

---

## 1. `pull_page` fails on Pages with a module-inherited audience

### Evidence (read-only Canvas Live GET, 2026-10-27, ELA7)

Failing study-guide Page, `GET /api/v1/courses/<ELA7>/pages/page_id:<id>/date_details`:

```json
{
  "graded": false,
  "only_visible_to_overrides": false,
  "visible_to_everyone": false,
  "unlock_at": null, "lock_at": null,
  "overrides": [{
    "context_module_id": <module>, "context_module_name": "Book Club: Ghost",
    "group_category_id": <tag set>, "group_id": <tag>, "non_collaborative": true,
    "id": <override>, "title": "Ghost", "unassign_item": false,
    "sub_assignment_due_dates": []
  }]
}
```

A Page in the same course that pulls fine reports `only_visible_to_overrides: false, visible_to_everyone: true, overrides: []`.

So the Page has no override of its own. It lives in a module whose audience is a **differentiation tag** (`non_collaborative: true`), so Canvas reports both visibility flags `false` and lists the module's row with `context_module_id`. The teacher's Page and module IDs are in the engine-notes entry.

### Where it breaks

- [internal/domain/page.go:175](../../internal/domain/page.go) rejects `OnlyVisibleToOverrides == VisibleToEveryone` as contradictory. Both `false` is a real Canvas state (module-restricted item).
- [internal/workflow/page_dates.go](../../internal/workflow/page_dates.go) `pageDatesSnapshot` ignores `ContextModuleID`. Once the check above passes, it would record the module's row as one of the Page's **own** `group_id` rows. A later collection write would then send it back as a Page override: a wrong-target write that turns a module audience into a Page audience. Fix both together.
- [internal/workflow/assign_to.go](../../internal/workflow/assign_to.go) already handles this for assignments: rows with `ContextModuleID > 0` become `Inherited` and are shown but never written. Reuse that idea, not a new framework.

### What to build

- Accept `only_visible_to_overrides: false` with `visible_to_everyone: false` when every override row that explains it is inherited (`context_module_id > 0`). Keep refusing both `true`, and refuse both `false` with no inherited row, with a plain message naming the Page.
- Keep inherited rows out of the Page's writable `observed_dates.overrides`. Record them separately (read-only, e.g. `inherited_overrides` with module name and tag/section/student target) so the pulled file explains who sees the Page, and include them in the fingerprint so a module-audience change made between pull and push holds the write.
- Ordinary body/title edits on such a Page must not touch date-details at all. An audience or availability-date change on a Page with inherited rows: send only the Page's own rows; never echo inherited ones.
- `non_collaborative: true` marks a differentiation tag, not a book-club group. Don't map it into collaborative-group logic (the 10-19 engine-notes entry warns about exactly this confusion).

### Tests and checks

- Unit: domain validation for the three flag combinations above; snapshot splitting own vs inherited rows; fingerprint change when an inherited row changes.
- Workflow fixture: pull → body edit → push sends one Page PUT and no date-details write; an audience push on a Page with an inherited row omits it from the payload.
- Live, read-only is enough to confirm the fix: `pull_page` succeeds on one affected ELA7 study-guide Page. Don't write to ELA7/PAP pages for testing (real students); if a live write check is wanted, build a module with a differentiation-tag audience in CS8 using disposable objects and ask the developer first.

### Docs

Update the Pages row in [engine-capabilities.md](../../docs/engine-capabilities.md) and `internal/localui/capabilities.go` (with review date) to say module-inherited Page audiences are read and preserved. Mark the live evidence as read-only.

---

## 2. Module document can't remove or replace an existing item

### Current behavior

[internal/workflow/module_author.go](../../internal/workflow/module_author.go) `PushModule` creates or adopts the module, sets position and publication, then walks `items` and **adds** each missing one. [internal/domain/module.go](../../internal/domain/module.go) `ModuleItem` has no module-item ID and no remove intent. The teacher wanted a module's PDF File item replaced by an ExternalUrl in the same spot; the agent could add the link but not remove the File.

### What to build (keep it small)

- Let an item name an existing live item to remove, by its `module_item_id` from `list_modules` (e.g. `remove: [<module_item_id>, ...]` on the module document, or `replaces: <module_item_id>` on the new item; pick whichever reads more naturally in the playbook).
- Before deleting, re-read the module and require the exact item ID with the same type and content ID/URL/title the agent observed; otherwise hold with a plain message. Deleting a module item only removes it from the module: the File, Page or assignment stays in the course. Say that in the report.
- A replacement is add first, then remove, then position the new item where the old one was. Save each outcome as it lands (the file already records per-item outcomes) so a rerun doesn't add twice or delete something else.
- Refuse removing an Assignment, Quiz or Discussion item unless the teacher's document says so explicitly. Students navigate to graded work through modules, so a stray removal is worth a guard; File, Page, ExternalUrl and SubHeader items are cheap to restore.

### Tests

Fake-Canvas tests for: replace File→ExternalUrl keeps position; mismatch on re-read holds with no DELETE; rerun after a completed replace makes no requests; course content is never deleted (only `DELETE /modules/:m/items/:id`).

### Playbook

Add a short "Replace or remove a module item" note to `workspace-template/playbooks/native-content.md`, which covers module documents.

---

## 3. Existing New Quiz: assignment group and base dates

### Current behavior

[internal/workflow/edit_plan.go:26-29](../../internal/workflow/edit_plan.go) refuses any `update` on an assignment with `external_tool` submission type ("a New Quiz can only gain a rubric here…"). `pull_assignment_dates` edits override rows only. The teacher needed copied Book Club Quiz 1 quizzes (ELA7 and PAP ELA7) moved from the 0%-weight Imported Assignments group to Major Grade, with unlock 10/30 8:00 AM and due/lock 10/30 4:00 PM. That had to be done by hand in Canvas.

### What to build

- Allow an `update` on a pulled New Quiz that changes **only** `assignment_group_id`, `unlock_at`, `due_at` and `lock_at`. Everything else (title, HTML, points, submission types, publication, rubric) must equal the observed baseline; refuse otherwise with the existing message. Question content and points stay with `quiz-points` / Canvas, as [ARCHITECTURE.md](../../docs/ARCHITECTURE.md) says.
- Choose the write path by evidence, on CS8 with a disposable New Quiz: either the assignment PUT `PlanEdit` already builds, or the New Quizzes API (`PATCH /api/quiz/v1/courses/:course/quizzes/:assignment`, which carries `assignment_group_id`, `due_at`, `unlock_at`, `lock_at`). Confirm after the write that both the assignment and `GET /api/quiz/v1/...` agree, since a write through one side that the quiz engine doesn't pick up would leave students seeing the old dates.
- Keep the existing read-before-write recheck: if those fields changed in Canvas since the pull, hold and keep the unsent edit.
- Date-only moves on a quiz that already has student attempts are fine; there's no need for a submission guard on dates or group.

### Tests and checks

- Unit: `PlanEdit` allows the four fields on `external_tool`, refuses each other field, still refuses a rubric change.
- Fake-Canvas workflow test for the chosen path and the conflict hold.
- Live on CS8 only (disposable New Quiz): move group, set the three dates, confirm via both APIs, delete afterward. Record the result in engine-capabilities.

### Playbook and docs

- `workspace-template/playbooks/edit.md`: replace "A pulled New Quiz accepts only `attach-rubric`" with the new rule (rubric attach, plus group and base dates).
- Mention in `workspace-template/playbooks/team.md` (where `copy_assignment` / `copy_module` guidance lives) that a copied quiz may land in Imported Assignments and can now be moved with a pull/edit/push.
- Update the New Quiz rows in engine-capabilities.md and capabilities.go together.

---

## 4. Done means

- The three behaviors above work, with fake-Canvas tests, `go test ./...` and `go vet ./...` passing.
- Capability page and doc updated with honest evidence labels (local tests vs Canvas Live).
- The teacher's engine-notes lines for these gaps can be deleted once a release with the fixes reaches their UserInstall. Tell the developer; don't edit UserInstall from Dev.
- Retire this file to `handoffs/retired/` with a banner when done.

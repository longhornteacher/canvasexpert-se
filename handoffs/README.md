# Handoffs

A handoff is bounded execution work for an agent session. It should say whether a change belongs to the Engine or the playbooks. Open work lives in `active/`; completed or superseded handoffs move to `retired/` with a retirement banner. Retired files are history and evidence, not instructions. Retiring one does not mean every observation in it was verified.

## Active work

Execution-readiness review: **2026-10-28**, against the current Dev working tree. The remaining handoffs are written for **6.1 Terra or 6.1 Sol** to execute one bounded slice at a time. Readiness here means the scope, integration points, contracts, checks and unresolved evidence are explicit; it does not mean the features are built or the undocumented Canvas behavior is known.

| Handoff | Execution status | What the review clarified |
|---|---|---|
| [Quiz accommodation writes and New Quiz item scores](active/HANDOFF_Quiz_Accommodation_Writes_and_New_Quiz_Item_Scores_2026-10-28.md) | Consolidated 2026-10-28 from the open remainder of the two retired handoffs below. Part A writes are implemented and the student-side timer/attempt effect was teacher-confirmed on 2026-10-29; only the course/quiz combination is open. Part B (Engine-written New Quiz item scores) is **complete** (2026-10-29): captured, built, L1–L8 passed live, and the playbooks use it. | Which test identity is available, what each observation enables, and the item-score Engine design to build only after the save is observed. |

<a id="execution-notes-for-the-three-active-handoffs"></a>

## Execution notes for the active handoffs

1. Follow root `AGENTS.md`: fetch, compare `dev...origin/dev`, and pull if behind before editing. Inspect `git status --short` and preserve any existing changes; do not reset, stash, reimplement or commit someone else's work blindly. The existing-content Engine work and both review passes were committed and pushed to `dev` on 2026-10-28. Line numbers in handoffs are navigation hints; locate the current symbols with `rg`.
2. Read `docs/ARCHITECTURE.md` and `docs/SCORING-RULES.md`, then the selected handoff. Read the team/group contracts when that slice touches their behavior. Start the next independently executable slice rather than stopping the whole handoff for an unrelated live check. No subagents, release, install, UserInstall/UserSync mutation or real-family send is authorized by these handoffs themselves.
3. Implement through the existing domain → workflow → CLI/MCP adapters. Register a new CLI name in help, known commands, arity and dispatch; register its typed MCP schema, annotations and command allowlist where needed. Update document validation/dispatch, `engine_reference` help/formats/examples and the relevant seed playbook together. Do not leave a helper that cannot be reached through the advertised teacher flow.
4. Handoff work overlaps in `cmd/cse/main.go`, `remote.go`, MCP registrations, `scripts/check-mcp.mjs`, capabilities/formats and quiz/edit/score playbooks. Execute slices sequentially in one checkout, or explicitly coordinate ownership if the user later requests parallel work. Reconcile these files against the then-current tree; never replace a whole file from an old handoff. Content-edit documents, QuizForge creation, accommodations and student item-score results are separate operations with separate outcomes.
5. A concrete teacher request already authorizes its ordinary product write. Development probes require the explicitly scoped live go-ahead described in each handoff; missing live approval/account access blocks only that probe or dependent unknown protocol. Explain the exact missing evidence and continue independent work. Never use the classroom MCP connection to execute a Dev test accidentally.
6. For implementation, run focused tests while editing, then these checks once for the completed slice. The build is a temporary Dev test binary, not a package or teacher install:

   ```powershell
   go test ./...
   go vet ./...
   $handoffCheckEngine = Join-Path ([System.IO.Path]::GetTempPath()) ("cse-handoff-check-" + [guid]::NewGuid().ToString("N") + ".exe")
   go build -o $handoffCheckEngine ./cmd/cse
   node scripts/check-mcp.mjs $handoffCheckEngine
   git diff --check
   ```

   Run each command after the previous succeeds; use the available Node runtime if it is not on PATH. Recount the discovered MCP tools and test the changed schemas; do not assume the starting count of 53 survives other slices. After review-page retirement the scoring `--review` smoke branch is removed, while family review has its own surviving tests. Do not run Canvas writes to make this smoke check pass.
7. Record what changed, checks actually run, and evidence still pending at the end of the slice. Update both capability records and their review date when implementation changes a capability. A fake-server pass is local evidence; an API reference is documented behavior; neither is Canvas Live proof. Retire a whole handoff only when its requested independent parts are complete, or clearly superseded with remaining work linked. A completed part may be recorded while the rest stays active.

This 2026-10-28 readiness review changed handoff documentation only, checked official public API references, and made no Canvas calls or teacher-install changes. It does not update product capability status by itself.

A second pass the same day checked the named files, symbols and call sites against the Dev tree (they hold; `hideGrades`/`gradeVisibleHold` live in `team_consent.go`) and removed recovery machinery that idempotent writes do not need, per root `AGENTS.md`: content-edit resume is a three-way live/observed/desired compare, not a pending-intent store. Quiz accommodations rerun the whole absolute-value array unless §3.1 shows otherwise, and course-level sync (Canvas 2026 §4.5) is frozen until the teacher asks, so Part C does not wait on it. Tag management resumes by re-resolving names and memberships, with one removal flag rather than two. Invalid saved quiz settings fail loading like the existing flags. Non-idempotent writes (New Quiz result versions) keep their pending/accepted records.

## Completed work and evidence

The [Canvas 2026 additions handoff](retired/HANDOFF_Canvas_2026_Additions_2026-10-28.md) is retired (2026-10-28): authenticated direction-image downloads, bulk differentiation tags and shared rubrics (with CS8 Canvas Live library-reuse, New Quiz attachment and Test Student assessment evidence) are complete; the explicit accommodation tool, saved quiz settings, `from_saved` planning and the publication hold are in Dev with local evidence. The [review-page retirement handoff](retired/HANDOFF_Retire_Review_Page_and_New_Quiz_Item_Scores_2026-10-28.md) is retired with Part 1 complete: the scoring/authoring review page and `start_review` are gone, the check-in is in the conversation with an Engine-rendered sample, and family review is unchanged. Accommodation writes and New Quiz item scores moved to the active handoff above. Full uncached tests, vet, the 57-tool compiled MCP smoke and `git diff --check` were rerun and passed at retirement.

The [existing New Quiz content-edit handoff](retired/HANDOFF_Existing_New_Quiz_Content_Edits_2026-10-27.md) is complete in Dev (2026-10-28): fresh exact quiz/item pulls, narrow text/link changes, independent conflict outcomes and three-way reruns. Full uncached Go tests, vet and the 54-tool compiled MCP smoke passed. An unpublished disposable CS8 quiz exercised the Engine instructions/prompt/general-feedback edits and a no-PATCH rerun; requested fields and sibling feedback matched a dev read-back, then the quiz was deleted. Linked-file access, student rendering and attempt effects retain explicit evidence limits.

The [existing-content editing handoff](retired/HANDOFF_Existing_Content_Edit_Gaps_2026-10-27.md) is complete in Dev: inherited Page audiences, exact module-item removal/replacement and narrow New Quiz group/base-date edits. Its banner records the read-only Page pull, disposable quiz API probe and remaining live Engine evidence limits.

The retired [WebUI width handoff](retired/HANDOFF_WebUI_Width_2026-10-26.md) records the implemented layout changes and the limit on recorded visual verification. The retired [parent-report and weekly ELA layout handoff](retired/HANDOFF_Parent_Report_Fix_and_Weekly_Layout_2026-10-26.md) records the Dev fix and read-only live inventory evidence; the independent Missing count, Outlook draft inspection, delivery and UserInstall adoption remain pending.

Current development authority lives in the root `AGENTS.md`, `docs/ARCHITECTURE.md` and `docs/SCORING-RULES.md`; build instructions and dated observations live under `docs/`.

The retired [team teaching 1.2b1 handoffs](retired/team-1.2b1/00-BRIEF.md) (2026-10-22) planned teammates' Canvas keys on one team PC, per-course key routing, `(CE)` scoring consent with hidden grades, a copy-once Collab module copy and family replies routed to the actual teacher. Slices 01-06 are built in Dev; the developer-run live checks (07) passed A1, A2, A3, A4 and A7 on 2026-10-22, with A5, A6 and the teammate and co-teacher parts left to happen on real use. The [product plan](../docs/team/README.md) carries the status table.

The retired [targeted scoring session brief](retired/TARGETED-SCORING-SESSION.md) (2026-10-21) replaced whole-course session discovery with Canvas's `needs_grading_count`, added assignment-group/due-state reporting and one local results index per pull. Implementation and local tests are complete; remaining live observations wait for real work.

The [Family Contact handoff](retired/FAMILY-ANNOUNCEMENTS.md) is complete in Dev. General family messages reuse the parent-update review, private recipient routing and Outlook delivery, with course/group/student audiences, per-message previews, inline images and file attachments. Full Go tests, vet and the compiled 40-tool MCP smoke passed; actual classic Outlook inspection confirmed a visible inline cover and an ordinary file attachment. No family email was sent. The [current contract](../docs/family-contact-flow.md) carries the supported flow and remaining evidence limits.

The retired [book-group audience brief](retired/BOOK-GROUP-AUDIENCES.md) records all five slices implemented, validated and committed in Dev: individual assignment audiences and refresh, course Page audiences, New Quiz audiences, and the integrated shared-module unit. Its original pause checkpoint is retained as history and superseded by the dated execution records. The preserved [source handoff](CSSE_Book_Group_Audiences_Codex_Brief.md) remains unchanged. Canvas Live access, student views and UserInstall/UserSync adoption remain separate observations; no release or adoption is claimed by this Dev work.

The [group, timing and parent-report fixes](retired/REVIEW-REGRESSION-FIXES.md) are complete and reviewed in Dev: all nine Engine findings have fixes and normal-suite regressions, and group-work guidance/evidence records are reconciled. Full tests, vet, diff checks and the compiled 38-tool MCP smoke passed before the audience integration. Distribution and UserInstall/UserSync adoption were not part of this work.


The [calendar/scoring delivery follow-up](retired/CALENDAR-SCORING-DELIVERY.md) is complete: the user requested the Git merge and local update, the beta ZIP was built and adopted in UserInstall, effective UserSync playbooks updated, and teacher context/overrides were preserved. The existing 1.0 adapter still needs a reconnect; pending Canvas/client/family observations remain in the capability record and are not prerequisites for ordinary supported use.

The [school-calendar/scoring plan](retired/SCHOOL-CALENDAR-SCORING-AND-PARENT-UPDATES.md), [current-term parent-report contract](retired/PARENT-REPORT-CURRENT-TERM.md), and [parent-date/Extra time follow-up](retired/PARENT-DATES-AND-EXTRA-TIME-FOLLOWUP.md) are retired with completion banners and dated evidence. The last closes the explicit-null parent date defect and exposes the saved private flag for documented accommodation decisions; it supplies no default allowance. Full Go tests, vet and a fresh 38-tool compiled MCP smoke passed on 2026-10-18, without Canvas writes.

Earlier group-work and parent-update handoffs are in `retired/` with banners, alongside the completed [Studio light/dark handoff](retired/STUDIO-LIGHT-DARK.md), approved reference, implemented screenshots and dated local checks. `retired/` keeps the records that carry unique dated evidence (the fresh-install notes and triage, the New Quiz scoring probe results, the MCP verification record) and the retired Antigravity/streamlined-install spec; everything else is in Git history.

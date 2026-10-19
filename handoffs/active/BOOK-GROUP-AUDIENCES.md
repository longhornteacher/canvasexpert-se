# Book-group audiences for individual content: execution brief

Prepared 2026-10-19 against Dev branch `dev`, revision `56109c6`. **Execution authorized by the user on 2026-10-19. Slice 1 is implemented; slices 2-5 remain planned.** This brief does not authorize Canvas Live operations, release, or UserInstall/UserSync adoption.

Source requirements: [original book-group audience handoff](../CSSE_Book_Group_Audiences_Codex_Brief.md). That supplied file is preserved unchanged. This brief supplies the execution sequence and code audit; it does not turn the source's request to implement into authorization to implement during this planning session.

Read root [AGENTS.md](../../AGENTS.md), [ARCHITECTURE.md](../../ARCHITECTURE.md), [SCORING-RULES.md](../../SCORING-RULES.md), the [group-work contracts](../../docs/group-work/contracts.md), and the assigned slice below. This is an extension of the delivered group-work features, separate from the frozen Tier C features.

## Result and scope

The teacher can ask: “Create the Ghost calendar Page, individual reading response, and New Quiz for Book Club → Ghost, in the existing Book Club module.” The Engine resolves current club members into a recorded individual student audience. Each student submits independently and receives an individual grade. The Page is a native course Page with its own access restriction.

Selecting a source club never sets native assignment `group_category_id`, uses a collaborative `group_id` override, changes group membership, or converts a quiz into shared work. No duplicate differentiation tags are required. A snapshot is explicit and refreshable; it is not a permanent live binding.

Direct module audience editing, Tier C group features, automatic roster synchronization, background services, bulk classroom migration, and changes to scoring policy are outside this brief. Reuse native module assembly. The current group calendar Pages, discussions, memberships and student work remain intact unless a later teacher task explicitly requests changes.

## Capability audit at preparation time

These are code observations, not fresh test results or Canvas Live evidence. Recheck the relevant code when executing.

| Area | Existing behavior and code anchors | Work required here |
|---|---|---|
| Collaborative group identity and private roster mapping | `internal/workflow/groups.go`: `groupCategory`, `ListGroups`; `group_memberships.go`: `groupRosterLive`, `ReadGroupRoster`; existing `workspace.StudentStore` and paginated reads | Add selected-group audience expansion and dated snapshots. Do not invent a second roster store. |
| Enrollment and membership states | `groupRosterLive` maps course enrollments and accepted/invited memberships; requested memberships are separate. Its `eligible` map filters the unassigned list, while assembled group members are not equivalently filtered. | Reusing its member list alone is insufficient to prove audience eligibility. Apply explicit supported enrollment/membership eligibility and selected-group completeness. |
| Individual assignment targeting | `internal/domain/types.go`, `validation.go`; `internal/workflow/author.go`, `assignment_dates.go`: create-time student-ref overrides, visibility stage before publication, exact per-row writes, saved outcomes | Reuse this path; integrate source groups, audience preparation, undated audiences and ownership. |
| Audience/date updates | `internal/domain/assignment_dates.go`, `internal/workflow/assignment_dates.go`: exact assignment ID, stable refs, override IDs/baselines, nullable dates, visibility outcomes | Current create rows require a date; update rows require unchanged targets, and a row with no date edits is preserved without a write. Membership refresh is therefore missing, despite existing override support. |
| New Quiz assignment targeting | `PullAssignmentDates`/`PushAssignmentDates` use `GetAssignment` with an exact assignment ID and have no blanket New Quiz refusal; `quiz_author.go` saves `quizSidecar.AssignmentID` and finishes through `/assignments/:id` | An assignment-side route exists in code. Prove it with a quiz-shaped fixture; do not claim deployed quiz targeting is already verified. `QuizSettings` has no audience field and `finishQuiz` does not apply audience restrictions. |
| Ordinary course Pages | `internal/domain/page.go`, `internal/workflow/page.go`: course/group routes, exact `page_id:` identity, body conflicts, native module placement, create/publication receipts | Add course-Page student audiences, date baselines and independent audience actions. The ordinary Page contract currently carries none of these. |
| Restricted Hub Pages | `internal/workflow/hub.go`: `readDates`, `setOverrides`, tag-only `pageDates` model and Hub delivery | Page date-details calls already exist. The helper replaces the collection with a single tag target; it cannot safely serve ordinary mixed student/section audiences unchanged. Preserve Hub semantics. |
| Reviews and adapters | `internal/workflow/review.go`, `review_author.go`, `review_edit.go`; `cmd/cse/main.go`, `remote.go`; `internal/mcpserver/server.go`, `operations.go` | Assignment/quiz review exists, but `assignment-dates` is not a supported review kind. Add audience preparation and the necessary review/dispatch integration with each owning slice. |
| Shared module | `internal/workflow/module_author.go`; existing Page/assignment/quiz placement paths | Reuse native items and preserve existing entries/order. Item access and student module visibility are separate evidence questions. |
| Seed guidance | `workspace-template/playbooks/group-work.md` still says linked graded discussions are unsupported, despite the implemented Engine contract | Correct the relevant source guidance as slice 1 lands. Do not overwrite teacher-modified installed playbooks. |

The group-work README contains older introductory status text alongside later implementation/live records. Use its dated evidence and current code, not the introductory paragraph alone, to establish the baseline. Do not repeat original group-work slices or require their unrelated follow-up fixes as prerequisites here.

## API facts that shape the implementation

Primary documentation checked 2026-10-19:

- [Assignments and overrides](https://developerdocs.instructure.com/services/canvas/resources/assignments): ad hoc student targets require active student enrollment and cannot overlap another ad hoc override. Updates must retain all currently overridden values explicitly. Source club IDs are not individual-content targets.
- [Learning Object Dates](https://developerdocs.instructure.com/services/canvas/resources/learning_object_dates): course Page date-details supports student overrides and visibility; supplying a collection replaces it, and success returns 204. Preserve all unrelated rows and finish paginated reads before replacement. Pages have availability dates, no graded due date.
- [New Quizzes](https://developerdocs.instructure.com/services/canvas/resources/new_quizzes): the quiz resource routes use the associated assignment ID. Reuse the saved assignment identity, validating its origin/course association.

These are upstream contracts, not observations of the district deployment. A permission/endpoint refusal holds the affected item with its saved work and precise diagnostic; it does not justify falling back to Everyone. Accept successful writes without confirmation GETs or response-shape success conditions. Reads for a subsequent distinct action's relevant preconditions remain allowed.

## Common contract and workflow decisions

Slice 1 owns the small shared contract, implemented as part of usable individual-assignment support rather than as a standalone framework slice. Later slices consume it.

1. **Intent:** one optional `audience` object identifies individual semantics, the exact source collaborative set and selected groups, and snapshot policy. Keep source IDs nested under the audience source, separate from native group-assignment fields. Use one source set per content item and support a deduplicated union of selected groups. Names assist discovery/display only.
2. **Preparation:** provide one typed Engine operation to prepare an initial audience or explicitly prepare its refresh. Preferred interface: `cse prepare-audience COURSE FILE [--refresh]` and MCP `prepare_audience {course, path, refresh?}`. These names are proposed, not existing commands. Finalize them in slice 1, using an existing surface instead only if it supplies the same explicit prepare/apply separation. Preparation reads Canvas and writes the course-local draft/state; it makes no Canvas writes.
3. **State:** persist origin/course/target identity, source IDs, sorted stable refs, observation time, relevant membership/enrollment fingerprint, completeness/reasons, exact owned override IDs and baselines, and distinct audience/publication outcomes. Use existing documents, quiz sidecars and `ActionOutcome`; no agent-maintained parallel manifest. Saved IDs may be absent after an accepted bodyless response: retain acceptance and hold any later identity-dependent edit rather than replaying the create or guessing ownership.
4. **Teacher interaction:** the agent prepares before an assignment/quiz review. Review displays the recorded audience, count/time, individual semantics, dates, publication and any holds. Push rechecks relevant source membership/enrollment and audience baselines before unfinished audience/publication actions. A changed snapshot requires explicit preparation again; push never silently refreshes it. Preserve task-scoped skip-review and existing direct Page/module behavior.
5. **Empty/incomplete:** an empty resolved audience is a normal draft condition. Preserve local drafts or existing unpublished resource IDs and explain that members can be added later; do not publish the requested new restricted content. Pending, invited, excluded or unresolved records are explicitly accounted for. Never label an incomplete read complete or publish only its known subset by accident. Apply completeness to selected source groups; an unrelated club must not hold a valid item.
6. **Ownership:** own only Engine-created rows with recorded exact IDs and state. A matching title is insufficient. Preserve manual exceptions and unrelated dates/overrides; show when those exceptions leave additional students with access. If a student already has a manual ad hoc override, preserve that row and report its effect rather than creating a conflicting duplicate or claiming the row as owned. If requested exclusivity conflicts with existing access, report the conflict instead of claiming it was achieved.
7. **Refresh:** prepare add/remove/retain, preserved exceptions, unknown facts and protected removals; apply only the prepared scope. Preserve access for students with submissions, quiz attempts, grades or other relevant protected work. An unreadable protection check holds the removal. Use existing explicit authorization conventions for a teacher-requested exact exception; add no generic force flag. Refresh never edits club memberships, transfers work, manufactures missing scores or resets attempts.
8. **Dates:** omitted authoring fields preserve intent. For APIs that require a full overridden-value set on update, construct that payload from the checked live baseline plus requested changes. Preserve inherited dates versus explicitly overridden/null dates. Do not create a fake deadline just to satisfy the current date-required validator.
9. **Accepted work:** retain accepted resources and per-stage outcomes; ordinary reruns skip them, including after later teacher edits. Pending/uncertain creates are not blindly replayed. Reuse existing bounded reconciliation only where needed for these paths. Source changes hold unfinished dependent actions, not unrelated content. For an already-published item's failure, report its actual remaining exposure; do not promise it became unpublished or automatically unpublish unrelated accepted content.

The source handoff's YAML is illustrative. Slice 1 must publish the final validated assignment example and concrete state contract; slices 3 and 4 add Page/QuizForge examples using that contract. Do not advertise proposed syntax as supported before the owning slice lands.

## Sequence

Execute serially, one writer at a time. Every slice touches shared schemas, adapters or guidance. Each slice delivers its own CLI/MCP, review where applicable, examples and evidence updates; do not defer these to the final integration slice.

| Slice | Usable result | Depends on | Status |
|---|---|---|---|
| 1 | Resolve book-club members and create/initially target an individual assignment | Existing Engine | Implemented in Dev; local evidence only |
| 2 | Explicitly refresh an individual assignment's audience without losing access to work | 1 | Planned |
| 3 | Create/update/refresh a restricted native course Page | 1, 2 | Planned |
| 4 | Create a restricted New Quiz and refresh its audience through its assignment | 1, 2; run after 3 | Planned |
| 5 | Demonstrate the complete shared-module workflow and finish cross-feature guidance | 1–4 | Planned |

### Slice 1 — Member resolution and individual assignments

**Outcome:** “Give Ghost readers this individual response” works through the Engine, with an inspectable prepared snapshot and unchanged independent submission/grading semantics.

Work:

- Implement the common contract and preparation operation; support one or multiple exact selected clubs, privacy-safe stable refs, deterministic unions and meaningful state/completeness diagnostics. Reuse group/enrollment/private identity helpers, correcting only what this resolver needs.
- Account separately for course enrollment and group membership states. Active enrollment is required by the documented student-override route; an invited membership/enrollment is not proof the API can assign that student. Report unsupported/pending records without silently granting access or discarding them.
- Extend ordinary assignment creation and `assignment-dates` initial application to consume prepared refs. Support audience-only rows with no artificial date. Reject native shared-assignment settings on this individual-audience path; preserve existing native group assignment and explicit-ref contracts.
- Record override ownership immediately. Apply restrictions before final publication, retaining existing rubric/module stages. Integrate audience facts into create review, and support review of prepared audience-only assignment updates through existing review callbacks.
- Add CLI/remote/offline validation and typed MCP preparation/dispatch. Correct seed instructions relevant to individual audiences and the stale graded-discussion statement. Document that refresh lands in slice 2.

Likely files: `internal/domain/types.go`, `assignment_dates.go`, `validation.go`; `internal/workflow/groups.go`, `group_memberships.go`, `author.go`, `assignment_dates.go`, `review*.go`; existing command/MCP files. Add a small audience-specific domain/workflow file if that keeps the implementation direct.

Acceptance: exercise the real prepare→review→push path with synthetic Canvas: two clubs, overlap, an unassigned student, exact course/set checks, a tag refusal, eligible versus pending/unresolved membership, selected-group incompleteness and an empty club. Assert correct student-only targeting and unchanged independent semantics; no broad publication on audience failure; unchanged unrelated overrides/dates; a post-preparation membership change holds that item while an independent item continues. Demonstrate an undated audience and preservation of an explicit student exception. Check privacy at returned-document/diagnostic boundaries. These scenarios may share fixtures; do not build a combinatorial matrix.

### Slice 2 — Explicit refresh and work-preserving removals

**Outcome:** “Update this response for the current Ghost readers” prepares a clear difference and applies it without silently removing students' existing work or teacher exceptions.

Work:

- Extend preparation with explicit refresh against current selected membership, the existing content audience and recorded owned rows. Save add/remove/retain and protected/manual/unknown cases in the existing course-local state.
- Add baseline-controlled membership changes to owned ad hoc student override rows. The current dates-only validator/no-op path is insufficient; do not work around it with date changes or delete/recreate cycles.
- Preserve complete overridden-date semantics in update payloads, unrelated rows and manual exceptions. Compare the fields being replaced and actual row ownership; do not let an unchanged rerun undo a subsequent teacher edit.
- Recheck work protection before removals and source/target baselines before applying the prepared plan. Keep protected recipients in the applied audience and label the outstanding removals accurately. Apply independent safe additions/removals where the API operation permits; retain distinct outcomes so completed actions are not repeated.
- Finish refresh review/direct-write routing, examples and guidance. Export only the small common behavior Pages/quizzes will actually consume, not a generalized operation framework.

Likely files: slice 1 audience code; `internal/workflow/assignment_dates.go`, `canvas_reads.go` and existing submission reads; domain override/state validation and audience review/adapters.

Acceptance: add a new eligible member; remove a student with no protected work; hold removal for a genuine submission or grade and for unreadable evidence; distinguish an unsubmitted placeholder from actual work. Preserve manual rows, a newer teacher edit, inherited/overridden/null dates and accepted outcomes through a restart. A changed preparation holds only the affected action. Cover ownership identity and an uncertain override write without duplicate creation.

### Slice 3 — Restricted native course Pages

**Outcome:** “Make the Ghost calendar a course Page for Ghost readers” creates a native Page that can be placed in the shared module, and supports later audience-only changes and refresh.

Work:

- Extend the ordinary course Page contract and pull/create/update flow with the shared audience intent/state, availability dates and exact date-details baseline. Keep group wiki Pages and Hub tier Pages on their existing semantics.
- Implement a complete Page date-details read and safe student-audience replacement. The current Hub DTO reads only group targets; retain all supported unrelated rows, dates and pagination. If an unknown row cannot be preserved safely, hold that replacement and explain it.
- Keep audience-only patches separate from Page body/title updates and body-conflict checks where possible. Changing who sees a Page must not send stale HTML. Preserve teacher editing roles and default teacher-only editing; add no Page due date.
- Create unpublished, persist Page identity, apply the complete intended restriction, finish requested native module placement and publish last. Use exact Page identity conventions already present in `page.go`, not a title/slug guess. Restriction failure leaves the new Page unpublished and resumable.
- Apply the slice 2 refresh/ownership rules to the Page's managed rows. A collection write is indivisible: preserve protected/manual rows in the submitted collection. Accept documented 204 success without requiring returned override IDs; if a later operation needs IDs, resolve them as that operation's preflight or hold ambiguity.
- Complete Page pull/push/optional review and MCP integration, valid examples and guidance within this slice. No new mandatory Page approval gate.

Likely files: `internal/domain/page.go`; `internal/workflow/page.go`, relevant `hub.go` read/payload helpers only if safe to share, audience code; command/MCP/review dispatch and Page tests.

Acceptance: create restricted course Page→native module item→publication; a failed/uncertain audience stage cannot publish it; 204 is accepted; all unrelated override rows and availability dates survive collection replacement. Change the live body after pull, then apply an audience-only update without a body PUT. Exercise explicit refresh, source changes and preservation of a manual exception. Keep existing group Page and Hub restriction regression checks green. Unsupported endpoints produce a precise held outcome with the draft/ID retained.

### Slice 4 — New Quiz authoring and audience-only updates

**Outcome:** “Give Ghost readers this quiz” uses the same audience preparation/review and creates independent attempts; later audience changes operate solely through the associated assignment.

Work:

- First exercise `pull-assignment-dates`/`push` against a synthetic New Quiz assignment through the real workflow. Record what already works and reuse it. Do not route through ordinary assignment content-edit code that intentionally refuses quiz edits.
- Extend strict `metadata.canvas` parsing, local validation and quiz review with audience intent. Extend the existing `.cse.json` sidecar with origin/course/audience state and distinct outcomes, preserving `assignment_id`, tier Page state and existing resource identities. Old sidecars remain usable; do not discard one to retry.
- Integrate audience application into `PushQuiz`/`finishQuiz` before publication, alongside the existing question, settings and module stages. For these restricted quizzes, persist accepted stage outcomes so an audience/publication failure resumes only unfinished work.
- Provide and document the audience-only/refresh route using the exact associated assignment, including New Quiz attempt/grade protection before removals. It must not call quiz/item create/update or reapply quiz settings merely to alter the audience.
- Complete `push_quiz`, `start_review`, preparation/refresh adapters and examples in this slice. Preserve Hub, question content/points, result visibility, attempts, dates and New Quiz written-work comment-only scoring limits.

Likely files: `internal/workflow/quiz_author.go`, `quizforge.go`, `quiz_format.go`, `quiz_hub.go`, `review_author.go`; assignment audience code, sidecar state and command/MCP adapters.

Acceptance: quiz-shaped assignment fixture proves the exact targeting route; source set IDs never become native group settings; wrong-origin/course/assignment state prevents writes. Audience failure keeps the new quiz unpublished, publication failure retains all accepted question/audience/module stages, and resume does not duplicate those resources. An audience-only refresh sends no question/settings writes or attempt reset; genuine quiz attempts/grades protect removals. Preserve existing quiz/Hub examples and old sidecar behavior.

### Slice 5 — Shared Book Club module and completion evidence

**Outcome:** one conversational request produces the Page, individual response and New Quiz, with native entries in an existing shared module and accurate instructions for future membership changes.

Work:

- Reuse existing module create/update/placement machinery; preserve unrelated entries/order. Use native course Page and Assignment references (including the quiz's associated assignment) according to existing contracts. Do not broaden module feature scope.
- Add one synthetic end-to-end example: Ghost members A/B, Alone members C/D, an unassigned student, a shared existing module and the three Ghost resources. Include an actual staged failure/resume and a membership refresh with a protected removal. Continue independent valid content when one item is held.
- Finish cross-feature playbook routing and worked examples in `group-work.md`, `authoring.md`, `native-content.md`, `edit.md` and `quizzes.md` as applicable. The agent handles files and Engine preparation; the teacher does not maintain lists or hand-edit manifests.
- Reconcile final documented syntax/commands with implemented validators and adapters; do not leave examples using the original conceptual schema. Keep review selection and direct-write scope intact.
- Record local evidence separately from pending direct-access and module-list/student-rendering observations. A teacher-token/API result is not proof of a targeted or untargeted student's session. If a later user supplies a concrete live scope, observe both direct URLs and the shared module; otherwise state these observations remain pending without blocking delivery.

Acceptance: the integrated example demonstrates correct individual audiences and native references, no broad publication on failure, meaningful resume and explicit protected refresh. Check normal individual work, native group work, group Pages, Hub Pages, explicit student overrides, graded discussions, quiz scoring and review handlers at the boundaries actually touched; reuse existing focused checks. Finish with `go test ./...`, `go vet ./...`, a fresh temporary CLI build and `scripts/check-mcp.mjs` against that build. Do not build/adopt a release as part of this slice unless separately requested.

## Documentation and completion for each slice

Update `engine-docs/formats.md`, relevant field limits, `docs/mcp-adapter.md`, tool help and the playbook seed with delivered behavior. Extend `scripts/check-mcp.mjs` when a tool/adapter changes. Keep `internal/localui/capabilities.go` and `docs/engine-capabilities.md` aligned and dated when implementation or evidence changes; add notable behavior to `CHANGELOG.md`. This planning session changes none of those capability claims.

Run proportionate focused behavioral checks for the owning slice, then broader checks only when its integration warrants them. Tests should exercise real prepare/push/review/payload behavior, including meaningful negative writes, rather than asserting source strings. The source handoff's acceptance list is covered across slices 1–5; it is not a quota or a requirement for 26 separate tests.

For each completed slice, append actual files changed, final contract decisions, commands/results, held or partial behavior and remaining evidence limits here. Mark a slice implemented only after its work exists and checks pass. Leave it planned if only designs were refined. After slice 5, report the final capability map, valid Page/assignment/quiz examples and exact CLI/MCP create/refresh paths. Retire this handoff only when delivered or explicitly superseded.

Keep real teacher data, credentials, UserInstall and UserSync untouched during Dev implementation. Historical classroom IDs in the source handoff are reference only; synthetic tests use synthetic IDs. Any later migration from existing group-calendar links to native restricted course Pages is a separate teacher-authorized task preserving the originals and student work, never an automatic completion step.

## Future execution prompt

> Implement slice N of `handoffs/active/BOOK-GROUP-AUDIENCES.md` in Dev. Read the current repository instructions, shared group-work contracts and original source handoff, then verify the slice's dependencies in current code. Complete only the assigned slice, including its adapters, review where applicable, examples, guidance and focused validation. Reuse existing student overrides, Page/quiz lifecycles and private identity mapping. Preserve individual submissions, restricted visibility, manual exceptions, unsent work and accepted outcomes. Do not run Canvas Live operations, alter UserInstall/UserSync, migrate classroom materials or release unless separately authorized. Report actual results and evidence limits and update the slice record before stopping.

## Execution record - slice 1 (2026-10-19)

Implemented domain `audience` and assignment/date integration in `internal/domain/{audience.go,types.go,assignment_dates.go,validation.go}`; selected-only resolver and prepare/apply/ownership in `internal/workflow/audience.go`; create/dates/review integration in `author.go`, `assignment_dates.go`, `review.go`, `review_author.go`; presence-aware override read fields in `canvas_reads.go`; focused behavior in `audience_test.go` and corrected date payload assertion in `assignment_dates_test.go`. CLI `main.go`/`remote.go` and MCP `operations.go`/`server.go` expose preparation and assignment-dates review dispatch. Smoke inventory is now 39 tools. Documentation, field limits, MCP guide, capability pair/date, changelog and authoring/edit/group-work seeds were updated; `engine-docs/examples/synthetic-individual-club-assignment.md` is locally validated by compiled MCP smoke. The original source handoff and pre-existing handoff index edits remain intact.

Final contract: `audience: {mode: individual, policy: snapshot, source: {group_category_id: SET, group_ids: [CLUB...]}}`; source IDs never become native shared-work settings. Engine-managed `state` contains origin/course/target, sorted source/refs, observation time and fingerprint, completeness/reasons, known enrollment exclusions, manual-access exceptions, observed visibility/override baseline, exact owned override baseline/ID/outcome, and separate application/visibility outcomes. Publication remains in the existing create receipt. Active enrollment plus accepted membership is assignable; known inactive/completed enrollments are explicitly excluded, while invited/pending/unresolved facts or incomplete selected reads hold publication. Only selected clubs are read. Student override rows may be undated. Existing manual rows keep dates/recipients and are excluded from newly created ad hoc rows. Extra existing access is disclosed before review, never claimed as exclusive. Course-wide/negative/unknown override target shapes hold this narrow path rather than being misread as positive student access.

Prepare reads Canvas and writes only the course-local draft. Initial `assignment-dates` audience application uses a pulled exact assignment document with its explicit rows unchanged; the agent removes the unchanged pulled `only_visible_to_overrides` request but retains its observed baseline. Date edits are separate. Empty snapshots stay drafts. Membership/enrollment/visibility/manual-baseline changes hold unfinished actions; accepted restrictions are not restored after later teacher edits. Initial preparation cannot erase accepted/uncertain ownership. Known accepted bodyless writes succeed without read-back; identity-dependent continuation with missing owned IDs holds. A pending/uncertain publication is not blindly replayed. `--refresh` / MCP `refresh: true` currently reports unavailable; slice 2 owns that implementation.

Checks actually run: `.tools/go1.27.1/go/bin/go.exe test ./internal/workflow -run 'TestAudience|TestPushAssignmentDates' -count=1` passed after the final targeted fixes. Affected package suite `go test ./internal/workflow ./internal/domain ./internal/mcpserver ./cmd/cse ./internal/localui` passed during integration and was rerun at slice close. A fresh temporary CLI build at `C:/Users/adamb/AppData/Local/Temp/cse-audience-slice1-db0f4902-0695-47ef-a3d6-7b77203e582c/cse.exe` passed `node scripts/check-mcp.mjs ENGINE`: **39 tools**, compiled stdio, root binding, schema discovery, new example and existing document/QuizForge validation; no Canvas writes. Tests use synthetic IDs/private fixtures and cover overlap, unassigned students, exact identity/tag refusal, known versus pending/unresolved enrollment/membership, selected-read failure, empty drafts, source-change zero writes, manual exception disclosure/preservation, stale manual/visibility baselines, uncertain override no-replay and accepted restriction/publication-failure resume (including pending publication no-replay).

Remaining evidence: no Canvas Live operation, real UserInstall/UserSync change, release or adoption was performed. Direct targeted/untargeted student access, module listing and actual student rendering remain pending. Explicit refresh and Page/quiz integration remain the next slices; ordinary supported initial assignment use is not blocked by those pending observations.

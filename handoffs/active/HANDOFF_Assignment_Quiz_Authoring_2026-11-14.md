# Assignment and quiz authoring: finish construction with fewer agent steps

Status: **implementation started; incomplete**. Requested by the developer on 2026-11-14 after reviewing assignment/quiz building and pushing. Goal: efficient, clean authoring through existing Engine and MCP paths, preserving drafts, teacher intent and honest outcomes.

## Partial implementation � 2026-11-14

Ordinary quiz receipts now record origin/course and stages; legacy unbound IDs are verified against the course quiz list before binding, conflicts hold, and a completed publication is reported on rerun. Focused synthetic tests pass. Item identity capture and the remainder of slice 1 are not implemented; slices 2�4 remain open. Do not retire this handoff. The full workflow test run failed on the updated legacy fixture (subsequently corrected) and an unrelated concurrent-download test server panic; rerun the required checks after completion.

## Start and scope

Read root `AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/SCORING-RULES.md` and `handoffs/README.md`. Fetch/compare `dev...origin/dev`; pull if behind. Planning baseline: `5dc068c`, six commits ahead of fetched `origin/dev`. Existing uncommitted work touched `cmd/cse/remote.go`, installation/localui files and MCP server/operations/workspace-file tests, with new `internal/mcpserver/result_text*.go`. Inspect and preserve those changes; never replace whole files from this baseline.

Execute the four slices below sequentially using existing packages. No new public tool is required. Leave changes uncommitted for review unless the user requests commits. No release, push, installation, UserInstall/UserSync edits, Canvas Live probes or classroom writes are included. Use synthetic fixtures; no helpers unless separately requested by the user.

Intended flow: saved draft → local validation and playbook content review → authorized push → compact outcome. The push owns mechanical sequencing and publishes last; the agent authors content/rubrics and makes teaching decisions. Preserve review sizing and requests that already authorize creation.

## Source findings

- `internal/workflow/author.go`, `Push`: assignment/rubric/module/publication receipts and request/time metrics already exist. Fresh POST is followed by `GetAssignment` despite returning an `Assignment`. Rubric follow-up reads serve a separate points/coupling check.
- `internal/workflow/quiz_author.go`, `PushQuiz`, `quizSidecar`, `quizStages`, `pushQuizSteps`, `finishQuiz`: stages and origin/course binding are conditional on audiences, accommodations, prior stages, skip options or retakes. Ordinary reruns can reapply settings/publication. Ordinary item POST responses are discarded; stimulus entry IDs are used transiently.
- `internal/workflow/quizforge.go`, `ReadQuizForge`: already validates shapes, rationales, images and settings. Keep one shared validator.
- `internal/mcpserver/operations.go`, `validateQuiz`: returns title, raw item count and warnings. Push tools invoke commands; `cmd/cse/remote.go` prints outcomes. Reconcile with the ongoing result-text cleanup.
- `workspace-template/playbooks/quizzes.md`: ECR/SCR rubric attachment is a separate post-create action, potentially after publication. `rubric_hint` is private author guidance, not an Engine-generated rubric.
- Reuse `internal/workflow/rubric_author.go`, rubric-reference helpers, domain rubric types/validation, audience/accommodation/hub operations, existing author/quiz tests and focused `engine-docs/` references. Read team/group contracts before touching their routing or audience behavior; redesigning those is outside scope.

## 1. Consistent quiz receipts and item identities

Initialize the existing stages and Canvas origin/course identity for every new quiz. Reuse these fields rather than introducing a general ledger. Preserve accepted items/settings/module/audience/accommodation/publication stages; a completed rerun must not repeat accepted construction writes.

Validate saved identity before using a saved assignment ID. Legacy sidecars remain usable: establish the exact quiz in the requested course through an existing scoped read before binding missing identity, or hold specifically when it cannot be established. Never silently rebind a conflicting receipt. Keep uncertain-create recovery bounded and never blindly replay a create.

Capture item creation responses in ordinary and stimulus paths. Persist authored QuizForge ID → Canvas entry/item identifiers needed by written-work context, with position/type only as useful metadata. Inspect current pull/scoring types and fixtures to distinguish entry IDs from actual item IDs; preserve string identifiers. Previously accepted files without usable authored IDs remain supported, with unresolved mappings reported rather than identity invented from titles/positions.

Persist returned identities promptly. Legacy/uncertain partial pushes may use the existing scoped item list; successful creation must not add a read solely to rediscover returned IDs. Missing identity in an accepted response is not permission to replay the item POST. Preserve partial-resume behavior and report mapping limitations honestly. Expose receipt path/mapping availability without dumping quiz bodies, keys, access codes or private expectations. Update the playbook's expectations-save procedure to use the mapping.

Do not build reconciliation for arbitrary editing/reordering/deletion of unfinished quizzes. Clear holds and teacher-directed repair suffice where no student work is at risk. Preserve protection of existing student work.

Acceptance: ordinary quiz with no accommodations gets bound stages; completed rerun sends no accepted construction writes; interruption resumes unfinished stages; conflicting identity writes nothing; representative legacy receipt resumes safely; ordinary/stimulus-linked written items save correct identities; missing response IDs do not duplicate accepted items. Extend existing fixtures, not a new matrix.

## 2. Optional authored rubric inside quiz creation

Add optional `metadata.canvas.rubric` using existing rubric shapes: inline definition, `{id: N}`, or `{library: "name"}`. This additive field spelling is the implementation direction unless current code already has an equivalent supported field. Do not generate a rubric from `rubric_hint` in the Engine.

- Validate using shared rubric rules. Reuse reference resolution, identical-definition reuse, association, independent grading (`use_for_grading: false`) and persisted outcomes. Extract a small shared helper only where actually reused; do not duplicate assignment rubric machinery.
- Attach to the associated assignment after items and before publication. Save creation/association outcomes; resume unfinished work. Failed/uncertain required attachment holds publication and preserves accepted items. Retain points/coupling checks; a written-only rubric need not total the entire mixed quiz's points.
- No supplied rubric retains current support. MC+Why reasoning items still use a local basis and do not trigger an automatic Canvas rubric. The playbook authors ECR/SCR rubrics in the initial draft. Existing-quiz attachment keeps its existing operation.
- Preserve retake privacy, explicit publication, audience restrictions and accommodation holds. Never unpublish a previously published quiz on legacy resumption; report a blocked attachment and actual known state.
- Update formats/example, playbook and tool description. Remove the routine post-create pull/attachment detour for rubrics already supplied in the draft.

Acceptance: mixed quiz attaches an independent written rubric before publication; failure leaves a new quiz unpublished and resume does not recreate accepted items/rubric; ID/library references reuse current behavior; points remain honest; no-rubric and MC+Why stay supported. Reuse existing rubric fixtures.

## 3. Useful local validation and completion summaries

Extend shared workflow validation output used by `validate_quiz`, keeping local/offline operation without credentials or Canvas calls:

- Scored-question counts by type, passages separately, and total points from the same prepared content used by push.
- Effective time limit, attempts, shuffle and result visibility, including defaults; supplied dates/module and requested publication.
- Compact descriptions of requested rubric/audience/support stages and existing actionable warnings. Do not imply live audience, accommodation or permission resolution. Never print access codes or private student settings.

Push output must retain partial/held/uncertain outcomes, identity, URL, receipt path, mapping availability and the unfinished step. Distinguish requested publication from state known through a receipt/accepted response. A rerun that skips accepted publication must not falsely report unpublished. Keep ordinary output compact and compatible with the current MCP envelope, without duplicate verbose payloads or polling redesign. Optional quiz request/time metrics should reuse the assignment report pattern, not a telemetry system.

Acceptance: mixed quiz with a passage yields accurate counts/points/defaults without Canvas calls or access-code leakage; warnings remain actionable; CLI/MCP preserve accurate partial and completed-rerun output. Update reporting instructions and reference examples together.

## 4. Remove demonstrated redundant reads

For a fresh ordinary assignment, reuse the returned `Assignment` when it supplies fields needed by subsequent stages. Retain a narrow fallback read for missing fields or a consequential live check. Resumption retains conflict/identity reads. Do not remove rubric points/coupling checks merely to reduce counts.

For quiz module placement, saved accepted outcomes skip repeat work. Eliminate a fresh quiz's initial module-list read only if the existing operation can directly save the returned module-item ID and retain uncertain-write recovery. A fresh quiz differs from a resumed one.

Use fake-server request traces/counts to establish saved calls for representative fresh/resumed cases. No timing framework or speculative speed claim. Keep sequential item creation and native stimulus linking. Parallel item writes, QTI import, bulk APIs, batch tools and a universal authoring framework are outside scope.

## Checks and delivery

Run focused existing tests while editing. For completed changes run `go test ./internal/domain ./internal/canvas ./internal/workflow ./internal/mcpserver ./cmd/cse ./internal/localui`, `go vet ./...`, and `git diff --check`. Run `scripts/check-mcp.mjs` against a temporary compiled Dev Engine when public contracts/schema or tested output shape changes. Broaden only for affected packages or concrete failures; no coverage quota or source-text architecture tests.

Update relevant formats/examples, seed authoring/quiz/teacher-context guidance, and architecture text only where contracts change. Update `internal/localui/capabilities.go` and `docs/engine-capabilities.md` together with review date and local-only evidence for new behavior. Add an unreleased changelog entry. Fake-server tests establish neither live rendering nor student access.

Report delivered behavior, actual checks, demonstrated request reductions, compatibility limits and any incomplete slice. Record results here and retire when done. Existing [pending observations](PENDING_OBSERVATIONS.md) are independent and do not block this work.

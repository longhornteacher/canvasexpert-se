> **Retired 2026-10-19 — implemented, verified and reviewed in Dev.** All nine Engine fixes and the group-work guidance corrections are complete. The implementation record below contains the local checks; earlier failure evidence is preserved in the linked reviews. Distribution and teacher-folder adoption were not part of this work.

# Fix the group, timing and parent-report review findings

Date: 2026-10-19. Status: completed and retired after final review. Work belongs in **Dev**. This handoff covers the nine reproduced Engine defects and the contradictory group-work guidance in the [October 19 review](../../docs/REVIEW-2026-10-19.md) and [October 17 follow-up](../../docs/group-work/REVIEW-2026-10-17-followup.md).

## Outcome and scope

Make supported grading, discussion evidence, interrupted-write recovery and parent reporting honor their existing contracts. Put the demonstrated regressions in the normal test suite so a green build checks these protections. Preserve saved sessions and unsent work.

Read root `AGENTS.md`, `ARCHITECTURE.md` and `SCORING-RULES.md`, then `docs/group-work/README.md`, `contracts.md`, and the relevant slices 05/06/07/08/09/10/11. Engine fixes own mechanics; the seed playbook owns teacher/agent judgment. The reviews describe observed failures, not additional feature requests.

Use one implementer at a time, in the order below. These changes share posting helpers, fixtures and capability records. No Tier C work, general operation framework, new approval token, database, persistent result index or migration is needed. No Canvas Live requests, delivery, packaging, UserInstall/UserSync edits, release, commit or push are included in this handoff.

## 1. Protect each individual group grade/rubric write — P1

Files: `internal/workflow/group_post.go`, `post.go`, and existing posting tests.

`postGroupIndividual` currently checks mode once, then `postGrade` can perform several writes. Renew the relevant live assignment mode/basis and the target student's group, attempt and artifact checks immediately before each distinct numeric/rubric write on the individual group path. This includes the combined grade/rubric write and both stages of a coupled rubric. A small group-specific pre-write helper/callback is sufficient; ordinary individual callers should retain their current behavior.

If the mode changes to shared, send no remaining individual numeric/rubric writes. Keep earlier accepted outcomes and hold affected remaining actions; do not undo or replay them. Keep the check scoped to the current student except where a live mode change affects everyone. Pre-write checks reduce the race; do not claim atomicity or add a lock/service to eliminate it.

Acceptance: change live mode to shared immediately after the first coupled rubric PUT. There must be no following `posted_grade` PUT or subsequent individual numeric/rubric write. Retain the accepted rubric outcome. The unchanged-mode control still posts the computed grade in the required order, and an ordinary non-group coupled assessment still works.

## 2. Decode discussion fallback replies — P1

Files: `internal/workflow/discussion_pull.go`, discussion pull/evidence tests.

Preserve Canvas's `recent_replies` preview on fallback entry reads as well as the `replies` shape used by full-view data. Normalize and merge by existing source-qualified identity. Fetch additional replies through existing pagination when indicated; an unreadable required page must retain partial/unavailable evidence rather than become complete.

Acceptance: make the full view unavailable and return one root with one `recent_replies` entry, `has_more_replies: false`, and no `replies_count`. Both entries must reach the packet with correct attribution and source/parent identity. Reuse the current pagination/deduplication tests; do not create a second discussion reader.

## 3. Retain fresh discussion coverage during posting — P1

Files: `internal/workflow/discussion_work.go`, `discussion_work_test.go`, and existing evidence types if necessary.

`newDiscussionGuard` must retain the fresh packet's coverage and relevant source limitations, not only its entries. An assessed `missing` component requires complete relevant evidence both at pull and at post. An unchanged own-entry digest does not establish completeness. Preserve existing topic/assignment, checkpoint, own-post and cited-context checks.

Use the existing source identity to scope a hold: unread replies in one club must not block another independently complete club. Do not invent student absence from unknown authors, missing pages or unreadable evidence.

Acceptance: start with complete evidence, fail a required reply read at post, and leave visible own posts unchanged. No grade/rubric/comment containing the unsupported missing-response assessment may post for that student. An independent student whose relevant conversation remains complete can proceed. Existing all-complete and changed-post cases continue to behave correctly.

## 4. Stop replaying uncertain comment-only writes — P2

Files: `internal/workflow/post.go`, group posting callers and existing comment/recovery tests.

Fix `postCommentOnly` so saved `pending`/`uncertain` outcomes are reconciled before another PUT. Use the originally saved `PostedText` and exact intended attempt when available. If that text is present, preserve the action. Otherwise keep uncertainty and explain it; absence from the latest read is not proof the write failed. Editing the local feedback must not replace the original attempted text during uncertain recovery. A legacy uncertain record lacking saved text must not be blindly replayed either.

Known failures remain retryable; accepted actions remain done. Reuse the existing shared/private group comment recovery semantics without introducing a new ledger. The shared helper must retain ordinary and New Quiz comment-only boundaries and explicit private group-comment flags.

Acceptance: a comment PUT with an ambiguous result followed by a resume sends no second PUT when the text is absent. An exact original-text match reconciles without writing, even if the local feedback changed. A known rejected write can retry, and a succeeded write is never repeated. Cover the individual group caller and the existing ordinary helper path proportionately.

## 5. Reconcile shared timing against every member — P2

Files: `internal/workflow/group_post.go`, `late_timing.go` only if needed, `group_post_test.go`.

Keep whole-group completion in the group workflow. For an uncertain shared late-only action, mark `preserved` only when every renewed member of the complete reviewed audience matches the requested timing. The representative's state alone cannot settle the action. A mix of target and original-baseline states remains uncertain/held with no blind retry. Retain existing audience, attempt, due date, custom-status and calendar guards.

For a fresh shared action, likewise do not let the individual helper's early “already matches” return establish completion for members who still need the requested change. Preserve the single representative-route write contract and the distinction between a new authorized action and an ambiguous previous write.

Acceptance: reuse the three-member fixture with an uncertain outcome; make only the selected representative match, leaving the others at their original baselines. The group must not become complete and no PUT may be sent. Keep the existing unanimous-match/no-PUT and fresh-action/one-PUT cases. Add one group preparation-to-post case using actual `prepare_late_timing` school-time/member provenance rather than only handcrafted legacy timing.

## 6. Withhold parent feedback independently of grade values — P2

Files: `internal/workflow/parent_report.go`, `parent_report_test.go`; rendering tests only where affected.

Follow the existing rule that explicit `posted_at: null` withholds feedback. Do not make that protection conditional on `score`, `entered_score` or `graded_at` being populated. Suppress the text before it enters the agent-readable parent packet; withholding only in final HTML is too late.

Keep the complete Work due inventory and honest status: a submitted response with no grade is awaiting grading, not evidence of a numeric grade being withheld. Preserve legitimate correction availability without copying withheld teacher comments. Keep omitted posting timestamps distinct from explicit null for existing saved-data/fixture compatibility; do not casually reinterpret every omitted field as null. Existing unreleased numeric-grade redaction remains intact.

Acceptance: decode literal JSON with a submitted visible in-window response, a teacher comment, and all four fields (`posted_at`, `score`, `entered_score`, `graded_at`) null. The inventory row remains and no teacher-comment text reaches the report. Retain the scored/unreleased privacy case and a released-comment control. Keep the explicit-null due-date fix unchanged.

## 7. Align result discovery with accepted file contracts — P2

Files: `internal/workflow/session_discovery.go`, `late_prepare.go`, `pull_work.go` as needed, and discovery/preparation tests.

Replace the three timing-history walkers' independent `.md`/`courses/` assumptions with one small common result-file enumeration path. It must find valid results accepted by the posting/file contract, including `.md`, `.yaml`, `.yml` and legitimate local result locations outside `courses/`. Do not add a new posting restriction to make the scans agree. Keep academic completion discovery aligned where it shares this behavior.

Use document kind and exact instance/course/assignment/student/attempt identity to recognize records. Preserve the distinction between accepted CE-SE provenance and a teacher-owned status that merely matches a target. Results remain local to UserInstall: do not scan UserSync or expose private/Engine files through agent tools. Respect existing path/link protections. No persistent index or broad file-access redesign is needed.

Acceptance: an accepted result is still found after changing its extension to `.yaml`/`.yml`, and from an allowed local folder outside `courses/`. A CE-SE `none` result remains eligible for calendar-driven reconsideration; an accepted `late` result retains provenance for preparation. An unrelated course/student/attempt must not suppress or authorize anything. Reuse existing second-session/no-replay tests. Include the supported filename cases in ordinary tests, not only the scratch overlay.

## 8. Distinguish submissions from placeholder records — P2

Files: `internal/workflow/author.go`, existing group-assignment edit tests.

The structural-edit guard must detect genuine submitted work rather than `len(submissions) > 0`. Reuse existing submission identity/evidence conventions. An unsubmitted row with attempt 0 and null `submitted_at` does not by itself block a requested group-set/grading-mode edit. Genuine work still protects that structure; unrelated requested assignment edits continue under existing rules.

`Client.List` follows pagination, so `per_page=1` is not a bounded one-row check. Correct that misleading comment and use a proportionate existing read/filter or traversal; do not build a new pagination framework.

Acceptance: placeholder-only responses permit the structural edit. Genuine work on a later page still holds it; unrelated permitted edits can proceed without clearing the structural hold.

## 9. Send only requested group-set changes — P2

Files: `internal/workflow/groups.go`, existing group-set update tests.

Build the outgoing patch from the same normalized baseline differences used for conflict detection. A populated optional field in a pulled draft is not itself a request to change it. Preserve explicit clear/change semantics for self sign-up, group limit and automatic leader settings; retain conflicts on a field the teacher actually changed.

Acceptance: rename a set from a pulled baseline while Canvas independently changes self sign-up. Send only the name and preserve the live setting. Check the same changed-field logic for the existing optional settings with focused cases, including an explicit clear; an unchanged draft needs no PUT.

## 10. Reconcile guidance and evidence records

Update `workspace-template/playbooks/group-work.md`: include implemented graded-discussion authoring/assessment in its scope and planning guidance. Apply `shared_assessment_status: ready` only to shared grading; explain supported individual-mode evidence and separately selectable student cards. Keep the checkpoint limit, New Quiz boundary and frozen Tier C scope explicit. Review instructions directly; do not add tests that search for prose phrases.

As fixes land, update `CHANGELOG.md`, `internal/localui/capabilities.go` and `docs/engine-capabilities.md` together with the actual review date. Reconcile affected stale statements, including per-slice claims that contradict the recorded CS8 live check. State exactly which findings are fixed and locally tested; retain pending student/client/Canvas observations. Add dated resolution notes or links to the two reviews rather than erasing the original failure evidence. Update format/architecture contracts only if the final implementation changes their stated behavior.

## Tests and completion

The nine reproduction cases are currently in ignored local scratch:

- `tmp/_group-review-20261017/review_probe_test.go` and its helper overlays: six earlier failures.
- `tmp/_calendar-review-20261019/review_probe_test.go`: three additional failures.

Extract the relevant cases into current tracked package tests; do not copy old overlaid helper files over today's tests. The brief and reviews describe the required fixtures even if scratch is unavailable. Use literal JSON for omitted/null/present API cases; encoding `Submission` can silently invent presence. No test should depend on these scratch paths, saved credentials, a real teacher folder or Canvas Live.

Demonstrate each relevant failure before its fix, then run focused regressions as that change lands. Once the combined changes are ready, run `go test ./...`, `go vet ./...` and `git diff --check` with the Go toolchain required by `go.mod`. Run the existing compiled MCP smoke if adapter/tool/document-contract changes actually affect that surface; do not invent a new acceptance campaign or repeatedly rebuild unchanged packages.

Completion means all nine behaviors have normal-suite regressions and fixes, the playbook contradictions are resolved, affected evidence records are accurate, and verification results are recorded in this brief. Report any remaining held item specifically and preserve independent progress. Leave changes reviewable in Dev; distribution and teacher-context adoption are separate work.


## Completion and verification - 2026-10-19

All nine Engine fixes and the instruction/evidence corrections are complete in Dev. The handoff remains active only for final review and retirement; there are no held implementation requirements.

| Brief item | Implemented behavior and tracked acceptance |
|---|---|
| 1 - individual group write boundary | `postGrade` accepts a group-specific numeric pre-write callback. Every combined write and both coupled stages renew the live mode/basis and target group/attempt/artifact. A mode switch after the first rubric retains its accepted outcome and sends no later numeric/rubric write; existing unchanged-mode and ordinary coupled controls pass. |
| 2 - fallback replies | The discussion entry decoder merges `recent_replies` with `replies` through the existing identity/pagination path. The literal fallback fixture retains both root/reply, source/parent identity and student attribution. |
| 3 - fresh discussion completeness | The guard retains the fresh packet and checks relevant stored/fresh sources before assessing missing work. Unknown relevant authors and absent sources remain incomplete. Real distributed pull/post fixtures show that an incomplete club is held while an independent complete club continues, whether partial coverage starts at pull or post. Cited context cannot narrow coverage for a student with no attributable posts. |
| 4 - uncertain comment-only recovery | Pending/uncertain writes reconcile the original saved text on the intended attempt and otherwise remain uncertain without another PUT, including legacy records without saved text. Group resume reconciles original text despite edited local feedback; ordinary controls cover wrong-attempt matches, accepted actions and known failures. Existing New Quiz comment-only boundaries pass. |
| 5 - shared timing completion | The group workflow reconciles every reviewed member before preserving an uncertain shared action; mixed target/baseline states remain uncertain with no write. A fresh action still sends one full-audience representative write when only that representative matched at baseline. An actual `PrepareLateTiming` fixture posts prepared school-time/member provenance once. |
| 6 - parent privacy | Explicit null posting state withholds teacher comments independently of grade fields before packet construction. Literal all-null input retains an awaiting-grading inventory row and correction availability without comments; released and omitted-posting controls retain legitimate feedback. Existing numeric-grade redaction and explicit-null due-date tests pass. |
| 7 - result discovery | Four completion/provenance callers use one content-based local traversal, including Markdown/YAML/YML/text/JSON-named and extensionless results outside `courses/`. Exact instance/course/assignment/student/attempt and accepted provenance remain required. Hidden/Engine/link paths are excluded without redirecting reads into UserSync; provider placeholders retain the existing link distinction. Accepted `none` remains reconsiderable and suppresses an unchanged second packet; accepted `late` still authorizes preparation. Shared academic completion now also checks the packet member's exact attempt. |
| 8 - structural edit placeholders | A full paginated read tests genuine positive-attempt/nonempty-submitted-at evidence, so placeholder-only results permit structure edits. A literal later-page submitted row holds structure while a requested title update still succeeds. |
| 9 - group-set patches | The outgoing patch is built from the same baseline differences as conflict detection. Rename-only and unchanged controls preserve independently changed optional settings. Change/clear controls cover signup, limit and leader fields. A narrow strict group-set decoder preserves explicit nulls that yaml.v3 otherwise omits from optional-field callbacks. |
| 10 - guidance and evidence | The group-work seed includes graded discussion planning/authoring/assessment, shared-only readiness, individual evidence/cards and explicit checkpoint/New Quiz/Tier C limits. Capability data/date, evidence table, group README, changelog and dated review resolutions agree; earlier CS8 evidence retains its recorded scope. |

The nine original probes were first moved into `internal/workflow/review_regression_test.go` against the current fixtures (only HTTP server hooks added), and all nine failed for their intended reviewed behavior before production fixes. Focused regressions then passed as fixes landed. Additional accepted-result/provenance controls are in `internal/workflow/session_discovery_test.go`; the ordinary comment-only completion assertion now retains its already accepted outcome. No scratch overlay is required by any tracked test.

Final checks on the combined source:

- `go version`: `go1.27.1 windows/amd64`, matching `go.mod`.
- `go test ./...`: passed, including domain, workflow, workspace, local UI, MCP adapter and CLI packages. Repeated after the final source-scoping correction and final embedded guidance changes; passed.
- `go vet ./...`: passed.
- `git diff --check`: passed after the final record update.
- `go build -o tmp/_review-regression-fixed/cse.exe ./cmd/cse`: passed.
- `node scripts/check-mcp.mjs tmp/_review-regression-fixed/cse.exe`: passed all 38 tools, bundled-root/mismatch checks, seed preservation, configured courses, stdio discovery, path exclusions, guarded context/assets and QuizForge/document validation. The smoke used synthetic temporary fixtures without `--review`; no Canvas requests or writes were made.

New implementation/test files: `internal/workflow/result_files.go` and `internal/workflow/review_regression_test.go`. Existing Engine changes are limited to `internal/domain/groups.go` and the workflow files named in the brief, plus current fixture hooks and focused test controls. Documentation changes are the group-work seed, changelog, capability data/record, group-work README and dated resolutions in both reviews. Existing review/brief/index work from this session is preserved.

No distribution, packaging, release, commit, push, family delivery or UserInstall/UserSync edits were performed. The compiled executable is an ignored local verification artifact. These fixes have local evidence only: pre-write checks reduce races without atomicity, and the previously pending student/client/Canvas observations remain pending. Final review checked the production changes, regression controls, outcome preservation and evidence/guidance alignment; no unfinished requirement remained. The handoff and its index were retired after review.

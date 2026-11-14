# Batch 1: scoring pull isolation and truthful partial results

Approved by the developer on 2026-11-14 after the scoring-incident evaluation. Dev was clean at `1304b71`, equal to freshly fetched `origin/dev`. This brief owns this batch; the incident is evidence, not authorization to replay grades.

## Outcome

A failed assignment must not hide independent scoring work. A course pull must say whether it finished, name failed assignments and phases, preserve available packets and unsent results, and offer exact-assignment recovery. Report lifecycle failures must carry useful sanitized facts without implying their unobserved cause.

## Ownership and implementation boundaries

One implementation agent owns `internal/workflow/pull_work.go`, `targeted_session.go`, `new_quiz_work.go`, focused workflow tests, `cmd/cse/session_report.go` and its tests, necessary CLI/MCP glue/tests, and the matching capability/contract documentation. Read root AGENTS.md, architecture and scoring rules first; read the relevant team/group contracts if changing their behavior. Keep routing, consent and group scoring semantics intact.

Use the existing orchestration and report types. Prefer a small per-assignment boundary and explicit outcomes over a generalized jobs/recovery framework. Inspect both failure seams: concurrent session-submission discovery currently cancels peers on any error, and assignment processing returns immediately on a New Quiz report error. Isolate assignment-local retrieval/identity failures; never turn unsafe evidence into packets. Genuine parent cancellation, shared setup failures and unusable local storage may still stop the run, with unfinished known assignments reported honestly. Do not swallow errors as empty success.

Known assignments need explicit processing outcomes so failure, filtered/no eligible work, intentional skips and not-reached work cannot collapse into zero pulled. Preserve Canvas needs-grading counts separately from candidates and packets. Existing inactive, missing, visibility and completed filters must retain their reasons. Aggregate output should account for all known assignments and clearly label partial results. Give failed assignment course/title/ID, phase and a bounded reason plus an exact scoped retry instruction; existing exact-assignment pulls suffice for recovery. Preserve successful packet paths, including preceding work when an assignment later fails, without overwriting any prepared results. CLI/MCP must retain useful partial output while signaling failure/partial completion.

Add bounded report diagnostics at the existing report request/poll/parse boundary: phase, reason code, recognized status, poll count/duration, URL-field presence, and HTTP status when actually available. Never log credentials, progress/download URLs (signed or otherwise), raw payloads, response content, names or answers. Distinguish pending/timeout, terminal failure, completed-without-URL, empty results and malformed responses. Reset per-poll decoding state so absent fields cannot inherit prior values. Do not guess another response shape or add an alternative retrieval/write route without evidence. Errors must preserve cancellation semantics. Empty/unmatched results must not become blank responses or grades. Keep diagnostics local to existing reports/errors rather than creating a telemetry system.

## Acceptance and focused gate

- Synthetic middle-assignment report failure: preceding and following independent assignments produce packets; the failed assignment is named with its phase; the overall result is partial.
- Synthetic middle-assignment submission-discovery failure: peers continue; the bad assignment produces no unsafe evidence; summaries distinguish failure from no eligible work.
- Early stop/cancellation: known unfinished assignments are not called checked or empty; parent cancellation still stops work.
- Counts and filtered reasons remain truthful, including inactive and absent submissions; successful packets and pre-existing unsent results survive.
- Report fixtures distinguish completed/missing URL, failed, malformed and empty responses; a pending-to-complete fixture verifies lifecycle handling without a slow test. No raw report or URL leaks into diagnostic output.
- Existing MCP identical-input polling still waits on one run, including when the eventual result is partial/failure.
- Existing attempt/version and posting guards remain unchanged. No grades/comments are written during development.

Start with focused workflow/CLI/MCP tests and run `go test ./internal/workflow ./cmd/cse ./internal/mcpserver ./internal/localui` after implementation. Run the repository's applicable MCP contract check only if its public schema/inventory changes. No coverage quota or unrelated test expansion. Update `internal/localui/capabilities.go` and `docs/engine-capabilities.md` together, including review date and local-only evidence; update scoring/MCP docs or seed wording only as behavior requires. Do not claim a Canvas Live fix from local tests.

## Non-goals and delivery

No effort-floor writes, PDF/OCR pipeline, Canva integration, updater changes, policy revisions, release/install update, or replay of the 56 accepted posts. No private teacher data in Dev. Do not access live Canvas or UserInstall without a concrete read-only diagnostic scope from the developer. The ELA7 completed-without-URL cause remains unverified until a sanitized real lifecycle observation is available; that does not block this isolation fix.

Return changed files, actual test results, remaining limits, and any scope decisions. Leave changes uncommitted for review. The coordinating agent reviews the implementation and evidence before reporting completion.

## Implementation and review result (2026-11-14)

Implemented and reviewed in Dev. Assignment-local submission/report failures now preserve independent packets and return explicit partial outcomes. Known assignments have processing outcomes; unknown eligibility stays unknown. Failed/not-reached rows carry exact retries, including session/group scope. Existing explicit invalid-group requests retain their precise refusal. Packet/student counters advance only after packet save. Parent cancellation and fatal local/shared stops retain a sanitized explanation; cancellation remains detectable with `errors.Is`.

Student Analysis diagnostics distinguish terminal failure, pending timeout, completed-without-URL, empty and malformed reports. They reset poll fields and omit server contents, raw status strings and addresses. Unknown nonterminal states still receive bounded polling. All-unmatched attempts produce a failed assignment without a blank student packet. No alternate retrieval route was introduced.

Validation reported by the implementation agent and reviewed by the coordinator:

- `go test ./internal/workflow ./cmd/cse ./internal/mcpserver ./internal/localui` passed (workflow 47.464s, CLI 1.032s, MCP 8.249s, WebUI 14.046s).
- Focused regressions passed for report/submission/identity/attempt isolation, existing wrong-group refusal, cancellation, lifecycle/privacy/stale fields, scoped retry output and SDK-visible partial results. Existing identical-input polling remains covered.
- `git diff --check` passed. The public MCP tool schema and inventory did not change.

The coordinator reviewed orchestration, error handling, diagnostics, retries, synthetic coverage and paired capability/agent documentation. Git history records delivery of this batch. No release, UserInstall update or Canvas access/write occurred. The real ELA7 retest's completed-without-URL cause and Canvas Live behavior of this fix remain unverified. Whole-quiz effort floors and attachment/LTI work remain outside this batch. Remaining execution work is in [the Claude Code handoff](HANDOFF_Scoring_Remaining_Work_for_Claude_Code_2026-11-14.md).

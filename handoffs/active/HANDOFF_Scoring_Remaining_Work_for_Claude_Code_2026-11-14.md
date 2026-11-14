# Remaining scoring work for Claude Code

Development handoff, 2026-11-14 (America/Chicago). The developer requested this handoff after batch 1 was implemented and reviewed. Start with batch 2. Batch 3 is a separate bounded verification task. The live report diagnosis can proceed independently when its specific access scope is authorized. Smaller observations below are not prerequisites for either batch.

## Start here

Work in **Dev**, `D:\Development Projects\CanvasExpert-SE`. Read the current root `AGENTS.md`, `docs/ARCHITECTURE.md` and `docs/SCORING-RULES.md`. Fetch and compare local `dev` with `origin/dev` before editing; preserve unrelated edits and pull if behind. Read the team/group contracts before changing their routing, ownership, consent or shared-assessment behavior.

The developer's working arrangement is senior-agent planning with implementation delegated to capable execution agents. This document supplies execution direction; inspect the actual current source and use focused implementations and tests. Keep educational judgment in playbooks/context, arithmetic and guarded Canvas access in the Engine.

Find delivery commits through `git log --oneline -- handoffs/active/HANDOFF_Scoring_Pull_Isolation_2026-11-14.md`. Dev began this work at `1304b71`, equal to fetched `origin/dev`; the commit carrying this handoff includes batch 1. This work has not been released or installed in UserInstall. The incident runtime was release 2.2.2, source `1e6b735b502d`, with userinstall commit `e3c5dd8`. A teacher's unchanged installed binary does not contain the Dev fix.

This handoff authorizes no real grade/comment writes, family sends, release, binary replacement or changes to teacher setup. Saved credentials and historical IDs are not live authorization. Prepare concrete local changes and a reviewable probe plan before requesting the live scope required by the current scoring rule. Private evidence, receipts, student aliases/names, responses, signed URLs and configuration stay outside Dev. Use synthetic fixtures.

## Completed batch 1: preserve its behavior

Read [the implementation brief and results](HANDOFF_Scoring_Pull_Isolation_2026-11-14.md). Assignment-local submission-discovery and New Quiz report failures now allow independent assignments to continue. Known assignments carry complete/filtered/skipped/failed/not-reached outcomes. Eligibility can be unknown. Packet/student counters advance after packet save. Failures name course/title/ID and phase; retries retain session and explicit group scope. CLI partial results exit 3; MCP returns `state: incomplete`, `outcome: partial` with useful text and packet paths. Identical-input pending polls wait on the same run. Explicit invalid-group requests retain their precise refusal.

`internal/workflow/quiz_report.go` now reports sanitized lifecycle facts: phase/reason, allowlisted status, poll count/duration, URL-field presence, and HTTP status when available. It resets decoding state each poll. Unknown nonterminal states retain bounded polling. Completed-without-URL, empty, malformed, terminal-failed and timed-out reports are separate outcomes. Empty/all-unmatched reports cannot create blank student packets.

Required validation passed: `go test ./internal/workflow ./cmd/cse ./internal/mcpserver ./internal/localui`, focused new regressions and `git diff --check`. This is local evidence only. Preserve successful packets, unsent results and accepted posting receipts in every later batch.

## Batch 2: explicit whole-quiz honest-effort minimum

**Built in Dev 2026-11-14, local tests only.** `comment_only.honest_effort` (needs `item_scores` and a course `effort_floor`) reports earned total, existing adjustment, minimum and total to enter; the raise-only fudge write is enabled for quizzes whose points equal the assignment's after a live check on one real LANG ARTS 7 student (35.5 raised to 50, gradebook followed, rerun wrote nothing); unequal-scale quizzes, late deductions, the student view and re-score unwinding are unobserved. Item scores write as before meanwhile.

### Problem and intended behavior

The teacher has a 50% honest-effort floor in the affected courses. Ordinary grades already support an agent's explicit `honest_effort` judgment. New Quiz item-score posting keeps earned item points and existing fudge points, but cannot raise the quiz total to the course floor. The incident encountered two qualifying genuine attempts below the floor; their item scores/comments succeeded and the teacher was given a manual SpeedGrader adjustment. A separate blank written response was correctly excluded.

Build a deliberate, reviewable, attempt-guarded whole-quiz floor adjustment. The agent must explicitly document that the written/creative work is a genuine try; a low total alone supplies no such judgment. Pure multiple choice, blank/placeholders and unknown evidence receive no invented minimum. Reuse the configured course floor and exact decimal arithmetic.

Keep earned item points, rubric criteria and feedback score lines as earned. The floor adjustment may only raise the proposed entered grade. Show the earned total, any existing teacher adjustment, the course minimum and the proposed entered total before a write. Preserve existing unrelated teacher adjustments and all unselected items/comments. State which values are observed and which are computed; Canvas calculates late deductions. Never substitute the written subtotal for the whole quiz total.

The incident's proposed blanket never-lower-across-attempts rule conflicts with the current ordinary-scoring policy. Preserve that ordinary policy and its retake-credit protection. Make this new floor action raise-only; do not introduce a global grade-preservation policy. Verify any teacher-specific override in its authorized private context if relevant, without copying it into Dev.

### Source ownership and design seam

Inspect these sources before choosing the smallest additive result/preview contract:

- `internal/domain/types.go`: grade `HonestEffort`, `CommentOnlyOperation`, `ActionOutcome` and the quiz-score receipt fields.
- `internal/workflow/new_quiz_item_scores.go`: `postItemScores`, result-version/score/graded-at guards, persisted pending/uncertain outcomes and `quizTotal`.
- `internal/canvas/newquiz_grading.go`: `NewQuizGrading`, `FudgePoints`, `PostScores` and `newQuizResultBody`; current item-score writes preserve the observed fudge value.
- `internal/workflow/post.go`, `review.go`, `internal/workflow/effort_floor_test.go` and existing New Quiz grading/item-score tests: reuse the relevant arithmetic, validation and outcome patterns.
- `engine-docs/formats.md`, `docs/new-quiz-written-work-spec.md`, `docs/mcp-adapter.md`, seed `playbooks/score.md` and both capability records for the agent-facing contract.

Existing native result writes already send the whole item array with `fudge_points`; recorded evidence establishes preservation, not the new floor calculation. The current scoring rule explicitly requires an approved live probe before applying a New Quiz floor. Prepare a precise protocol/probe plan and safe local implementation with synthetic transport tests; keep the new write unavailable for ordinary use until its needed live semantics are established. Keep existing item-score use available.

Use an explicitly authorized disposable quiz/test account to establish quiz-point versus assignment-point scaling (including unequal maxima), whether the returned total is pre-penalty, additive adjustment behavior, version identity, and retry behavior. Obtain exactly the developer-approved scope before any live change. Do not use real incident students as a probe. Retain only sanitized wire shapes and observed facts in Dev. A live probe is distinct from shipping the feature.

Before applying a prepared action, recheck course/student/attempt identity and the observed result version. Hold that student when the attempt, item scores, adjustment or relevant grade state changed. Reuse persisted pending/uncertain/succeeded outcomes and bounded reconciliation; an uncertain write cannot be blindly retried. Compute from an authoritative baseline so repeated apply cannot accumulate the floor twice. Preserve successful item-score outcomes if a later floor action is held; recovery must not replay them. Scope consent, ownership and posting policy remain in force.

Synthetic examples: on a confirmed common 100-point scale, earned items 30 plus an existing teacher adjustment of +5 gives 35; reaching a 50 minimum requires only the additional 15, while preserving the existing +5. A retry should remain at 50. A current entered result of 70 receives no floor reduction. Treat these as product arithmetic examples; the live scaling probe determines the actual native units.

### Acceptance and delivery

Cover qualifying below-floor work, already-at/above-floor work, blank/unjudged exclusion, an existing adjustment, a changed attempt/version, repeated apply/ambiguous transport, and preserving a higher grade. Add the scaling case established by the probe. Tests must assert truthful item scores/comments and unrelated adjustments, not just the final number.

Deliver code, a concise contract/probe record, focused tests, matching help/examples and capability records. Label local tests, live observations and pending student/client checks separately. While the live step is unavailable, explicitly report which local work is done and keep the documented SpeedGrader fallback. Never report the two real manual adjustments completed without their authorized execution and receipts.

## Batch 3: verify original upload delivery and visual inspection

**Run 2026-11-14 (synthetic only):** ranged MCP transfer and SHA-256 matched for a PNG, a text PDF and an image-only PDF; the client rendered the PNG and the text PDF. Scanned-PDF page rendering is unverified on this machine (no poppler); no transfer or count-labeling gap was found, so no code changed. Still not tried: a real student original, a file near the cap, slide decks, other clients.

Much of the code is already built: commit `1304b71` saves originals of every type under 150,000,000 bytes per file, distinguishes `downloaded`/text-preview `readable` from unavailable files, and adds ranged MCP reads for larger originals. It removed the New Quiz 50 MiB/file and 500 MiB/pull limits. Inspect the current implementation and the November 13 handoff as historical context before proposing more code.

Start with synthetic text/scanned PDFs and PNG posters. Demonstrate the intended client can receive an original through the supported runtime and actually open/render the pages or image. Use existing file/PDF/image tools. A base64 round trip or a local path alone does not prove that a remote client can inspect it. If a destination client is unavailable, report the client-specific evidence limit and continue any independent local verification.

Source seams: `internal/workflow/student_upload.go`, `pull_work.go`, `new_quiz_work.go`, `internal/domain/types.go`, `internal/mcpserver/workspace_files.go`, `scripts/check-mcp.mjs`, and seed scoring guidance. Current file metadata/asset reads already support transfer and hashes; extend only for a demonstrated gap. Preserve current-attempt identity and historical provenance. Report current submissions, unique students and attachment records separately when presenting counts; four historical/current PNG attempts by one uploader are not four students.

The old incident label "unsupported attachment format; original preserved" established no corruption or illegibility. Current guidance permits opening a safely saved legacy original. The teacher excluded the affected poetry work during that session; this handoff does not reopen or score it. Use synthetic files first. Actual student originals require an authorized private route and the applicable privacy boundary; never send raw named work to an arbitrary external OCR service. Continue independent students when evidence cannot be inspected, and hold unknown evidence rather than assigning zero.

Deliver actual client evidence and narrowly fix a reproducible transfer/inspection or count-labeling gap if one exists. Reuse existing download, corrupt/unavailable-file and attempt tests as relevant. Do not add an Engine OCR/rendering pipeline, generalized format taxonomy or recovery framework merely to satisfy a hypothetical case. Update capability evidence honestly even when the outcome is verification only.

## Independent follow-up: diagnose the missing Student Analysis URL

**Run 2026-11-14 (teacher-authorized, release 2.3.0 Engine, read-only except the report POST):** a scoped pull of assignment 3721619 completed on the first try: 2 students, both written responses and the auto-scored subtotal (45/60 each) present, no failure phase. The earlier missing-URL failure did not reproduce, so no request shape was observed and nothing was changed; it was most likely transient report generation. Batch 1 would now name the phase and facts if it recurs. The fresh packet is the current evidence for scoring the two students; nothing was posted.

Known incident target: course 120638, assignment 3721619, "Informational Final Retest: Nature and Practical Knowledge". Two Canvas needs-grading flags remained; no response packet was retrieved. The counterpart in course 120669, assignment 3721673, was successfully scored and posted and must not be replayed.

The exact old error proves the Engine parsed `workflow_state: completed` and then found no `results.url`. It does not establish Canvas's underlying failure or the two attempts' states. No raw lifecycle evidence was retained. Batch 1 prevents this assignment from hiding unrelated work but does not claim to fix that root cause.

After receiving concrete read-only diagnostic scope and establishing the correct runtime/UserInstall, use the new bounded facts to observe the request/poll lifecycle and the actual states of the two attempts. Student Analysis generation uses a POST as part of retrieval; include that exact operation in the diagnostic scope. Compare only sanitized facts from the failing and working request shapes. Keep signed URLs, private responses and authentication out of logs and fixtures. Do not add a guessed parser shape or alternate retrieval route from speculation.

Once a real shape is observed, add a sanitized fixture and the smallest justified correction. Recheck exact enrollment/attempt identity; an alternate native read, if justified, must preserve those guards. Recover only the still-unresolved assignment with a fresh scoped session pull under teacher authorization. Use its fresh evidence and existing posting outcomes; never infer blank answers, reuse a stale packet as current evidence, or replay the 56 accepted posts from the incident.

## Smaller observations: do not make them prerequisites

- **Canva LTI:** `basic_lti_launch` work is an external-tool access gap. Existing SpeedGrader/export-and-upload guidance is appropriate. The Quill-named assignment's Canva launch did not establish diagnostic completion. Build an integration only when requested; never dereference opaque launch tokens with generic network code.
- **Updater:** `update-status` reads cached refs. In the incident source, a same-day cache skip of `update-stage` reports `skipped`, while successful `current` follows a fetch. Preserve that distinction. Check-freshness metadata is an optional improvement, not an established stale-install defect. Do not pull/reset/update a teacher install just to answer a status question.
- **Agent reporting:** parse large inventory output locally, keep file/student/attempt counts separate, and treat empty results scaffolds as unfinished work. Batch 1 supplies bounded scoring discovery; do not build another inventory system without an observed need.

## Gates and completion report

For each actual code batch, run focused tests for changed behavior and then the applicable package checks. The existing scoring gate is `go test ./internal/canvas ./internal/workflow ./cmd/cse ./internal/mcpserver ./internal/localui`; add other affected packages only when relevant. Use `scripts/check-mcp.mjs` against a temporary compiled Engine if public MCP schema/inventory or asset transfer changes. Run `git diff --check`. Stop expanding checks after relevant tests pass unless a new change/failure justifies more.

Update `internal/localui/capabilities.go` and `docs/engine-capabilities.md` together with the review date; keep format/help/examples and playbook wording consistent with enabled behavior. Preserve the separation of local, Canvas Live and human/client evidence. No version bump, release, push or UserInstall update is part of this handoff.

Report the concrete behavior delivered, changed files, exact tests/results, live evidence if authorized, remaining manual tasks and any disabled/unverified write path. Each batch should be separately reviewable. Do not claim all incident issues resolved because batch 1 or the 56 prior posts succeeded.

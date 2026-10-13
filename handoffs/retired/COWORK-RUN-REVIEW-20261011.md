> **Retired 2026-10-13.** Historical instructions and evidence; do not execute this plan. Today's scope and dispositions are in [01 — Handoff cleanup](01-HANDOFF-CLEANUP.md). Retirement does not establish completion or new validation; preserve the dated evidence and teacher work described below.

# Cowork run review and proposed improvements

Reviewed 2026-10-11. This is a diagnosis and proposed implementation scope for discussion, not authorization to implement every suggestion. Follow source AGENTS.md, ARCHITECTURE.md and SCORING-RULES.md.

Second review (Claude, 2026-10-11): source claims in findings 1, 2, 4 and 5 and the `Posted` behavior in finding 6 were spot-checked and hold (`cmd/cse/remote.go:50`, `mcpserver/operations.go:136`, `quiz_author.go:66` and `:277-306`, `author.go:522`, `localui/review.go:278`). No shipped example or doc uses the `mapping:` placement found in the old SCR files. This revision narrows review-status scope, broadens unknown-key detection, drops the feedback count, and moves two small confirmed fixes into batch 1.

## Evidence and conclusion

Source: teacher-pasted `CSE_test_run_summary_20261011.md` and timestamped log, build `c32070c53278+dirty`, built `2026-10-11T19:31:10Z`. The original output files were not found in the Windows folders searched. The installed synthetic quiz draft and pushed assignment record were inspected, along with current source, Git history and focused local checks. No Canvas requests or writes were made during this review; no student work/results were modified.

The run establishes a successful cloud Claude -> remote-devices -> local CE-SE MCP -> teacher review -> Canvas authoring workflow for one assignment and one mixed MC/TF/ESSAY New Quiz, both unpublished. It also establishes ordinary student text/history pulls through that route and local validation of canonical scoring documents. It does not establish grade/comment posting through MCP, student rendering, written-item limit enforcement, publication, or direct desktop Work/Cowork compatibility.

The main problem is incomplete and inconsistently enforced authoring contracts. The agent guessed fields; some failed clearly, others silently disappeared. Review and completion output then concealed the mismatch. Fix that alongside the confirmed review dispatch bug before adding queues, caching, or folder migrations.

## Findings tied to source

### 1. Rationales: ignored input shape, not demonstrated transform regression

`internal/workflow/quizforge.go` accepts a top-level `rationales` array keyed by `item_id`. Neither `qfItem` nor `qfChoice` declares the inline `rationale` fields used by this run. `json.Unmarshal` ignores them deliberately, and the saved draft has no top-level rationales. A local probe parsed that actual draft successfully and found zero recognized rationale entries. Existing `TestQuizForgeTransformMatchesVerifiedShapes` passes and checks both MC answer feedback and TF neutral feedback from the supported structure.

Canonical example, outside `items`:

```json
"rationales": [
  {"item_id": "q1", "choices": [
    {"id": "a", "rationale": "Explain the correct choice."},
    {"id": "b", "rationale": "Explain the distractor."}
  ]},
  {"item_id": "q2", "rationale": "Explain the true/false answer."}
]
```

This is still a product bug: validation claimed success while student feedback was silently lost. `engine-docs/formats.md` never shows this structure, and embedded examples have no quiz. The playbook refers to optional external QuizForge material the agent did not have.

Proposed: embed a small supported QuizForge contract and runnable synthetic MC/TF/ESSAY example, reachable through `engine_reference`. This alone would have prevented both the TF key guess and the lost feedback.

The bug class is silently ignored item/choice keys, not only this run's misplaced `rationale`; the next guessed key will differ. Declare the intentionally tolerated fields (`rubric_hint`, `metadata`, the top-level `quizforge_version`, teacher notes) as known, then warn on any other unknown item or choice key, naming the item and pointing to the reference. Give inline `rationale` on an item or choice a specific error with the canonical `rationales` location. A blanket `DisallowUnknownFields` rejection remains too blunt.

Show canonical rationales in the teacher review, clearly distinguished from the question and answer key. `buildQuizReview` currently omits all rationales, even valid ones. That review display is the feedback check; no separate authored/sent/stored feedback count is needed. `PushQuiz` does not read back item feedback, so do not claim stored feedback in any output.

### 2. Review dispatch: confirmed argument-count bug

`cmd/cse/remote.go` sets `quizReview := cmd == "review" && len(args) == 2`. `internal/mcpserver/sessions.go` appends any supplied course argument. Thus an assignment or results document plus course becomes a QuizForge parse, exactly as reported. This is not specific to MCP. `workflow.BuildReview` already distinguishes YAML workspace documents from QuizForge input by content.

Proposed: identify file format before choosing the command path; do not infer type from argument count or extension (QuizForge can be an enveloped `.txt`). For workspace documents, allow a supplied course only if it agrees with the embedded course/assignment course; reject mismatches before opening review or contacting Canvas. For QuizForge, require its explicit course. Share the classification already available where practical. Reject invalid inputs before allocating a review process when cheap.

### 3. Old scoring files: invalid authoring, no evidence of schema tightening

Git commit `3244fbb`, the initial validated workflows, already has `KnownFields(true)`, `AttemptRef` with only attempt/submitted_at, and `ResultsDocument.Interpretation`. `mapping` was not accepted at the document root or inside an attempt in that version. The evidence supports authoring-side schema mistakes, not a newly orphaned supported schema. The exact earlier dirty binary is unavailable, so do not claim to have replayed it.

Quoted decimals are supported: `Decimal.UnmarshalYAML` parses scalar values, and `MarshalYAML` itself emits strings. They are not the reason those files failed.

Proposed: retain strict parsing, but add an actionable document-field error/hint pointing to `interpretation.mapping`, `grade.components` and `engine_reference` formats/examples. The current uncommitted improvement to `ValidateInterpretation` does not fix unknown-field errors: strict decoding fails before that validator runs. Do not build an automatic migration that invents criterion/component evidence. The teacher/agent can re-express saved judgments in a new canonical file, verify them against the pulled work, and leave originals intact. Validate new results immediately as the scoring playbook already directs.

### 4. Rubric grading flag: intentional policy, contradictory accepted input

`rubricCreatePayload` hardcodes association `use_for_grading: false`; independent grading is required by SCORING-RULES.md. Create validation accepts true, review shows criteria without the effective coupling, and the saved requested rubric still says true. Canvas did not spontaneously downgrade this value; the Engine sent false.

Proposed: reject `use_for_grading: true` on newly authored rubrics with a clear message explaining independent rubric assessments and separately computed grades. Omitted/false continues normally. Do not reject or change true on snapshots of existing coupled rubrics; their honest posting support must remain. Review should state the effective independent grading behavior. No silent rewrite of teacher drafts.

### 5. Quiz validation/settings: two additional source-confirmed gaps

`mcpserver.validateQuiz` calls only `ReadQuizForge`. The latter checks item shapes, but not `q.settings()`; review and push call that separately. A synthetic local file with `metadata.canvas.allowed_atempts` passes the parser used by validation, then fails review with `unknown field "allowed_atempts"`. Make local validation use the same settings validation as review/push, before a review session or Canvas create.

`QuizSettings.quizSettings()` applies `allowed_attempts` only when `allow_multiple_attempts` is true. Thus this run's `allowed_attempts: 1` was ignored, leaving default single-attempt behavior. This is not proof that Canvas violated the requested one attempt, but the same shape with 2 would silently fail to request two attempts. Review currently prints the raw allowed_attempts value even when the payload ignores it. Explain the enabling flag in the contract, reject contradictory multi-attempt configurations, and derive review/report facts from the effective payload. Do not turn requested settings into claims of Canvas readback.

`finishQuiz` also writes `published: true` before reading/adding the requested module item. If module placement fails, the quiz can be visible while reporting partial. This was not exercised in the run (publication was false), but directly violates the supported creation order and can expose unfinished content. Separate publication from the settings PUT and perform it after requested module placement. Reuse the existing sidecar; no new operation framework.

### 6. Review status and file paths: worthwhile small interface improvements

`reviewTask.status` returns prose only. Completed output concatenates historical stderr, including the obsolete open-page instruction. `remote.go` discards `PushReport.Path` in the assignment-review completion summary, although direct push already prints it. This is why the agent could not discover its moved draft from the outcome. Preserve the existing assignment archive and quiz sidecar behavior; return the resulting file path rather than changing storage layouts.

Proposed, limited to what the run demonstrated: a simple `state` (pending / finished / failed) alongside the readable text; the URL only while the page is usable; the resulting file path; revision notes when present. Skip a multi-state lifecycle. `ReviewOutcome.Posted` currently becomes true when the posting callback is invoked, even if it returns an error (the error is appended to the summary). Carry explicit completed/partial/held outcomes instead of deriving success from that boolean, so "finished" never implies all writes succeeded.

Include the active review_id in the single-slot refusal so the agent can recover. Pending-only cancellation is optional: add it only if cheap, preserving draft and revision notes, never killing a process already writing to Canvas or implying rollback. The run's stuck slot cleared when the teacher clicked. Keep one active review: no demonstrated need for a queue or simultaneous pages.

The loopback link requiring a teacher on the Engine machine is expected. An unattended test can explicitly request skipping review for its exact scope; broad push/pull permission did not change the product's default review decision. Do not add automatic review bypass or expiry.

### 7. Bridge drop: evidence is insufficient to assign an Engine cause

Multiple connectors disappeared together. The same reported build after reconnection proves binary identity, not process identity or uninterrupted lifetime. `Server.reviews` is in memory and `stopSessions` kills child pages on server shutdown. A page and known review_id surviving this incident suggest the local server stayed alive; this is not a guarantee across an Engine/MCP restart.

Reported completion was recovered at 19:58:23Z; assignment creation timestamp was 19:57:52Z; disconnect was first observed at 19:58:12Z. The event may have happened sooner, but the log does not establish that the click caused it. Preserve this as an external bridge observation. Inspect existing bridge logs only when needed; do not add reconnect orchestration to CE-SE from this one incident.

### 8. Lower-priority observations

| Run item | Recommendation |
|---|---|
| Duplicate timestamped pulls | Keep preservation behavior. File-size equality is not byte equality. Identical pulls do not establish no student changes, because already-handled work may be excluded. Prefer the demonstrated targeted pull when the teacher names an assignment. Do not add cache/deduplication from this run alone. |
| Different pull_assignment/pull_work layouts | Different artifacts: an editable authoring snapshot versus a scoring packet with history. Return paths consistently; avoid migration. |
| Late counts with Pulled=0 | Expected: Late/Missing describe discovered submissions and can overlap Handled. Add a short table legend and consistent course prefixes. |
| Unpublished assignment has Missing=28 | Visible output confusion, not automatic-zero behavior. Default course-wide discovery can skip unpublished assignments; explicit assignment scope can retain intentional inspection. Decide this small behavior separately. |
| Timing footers | Useful on network operations. Do not add ceremony to instantaneous local file/validation calls. Review duration includes teacher wait; it is not Engine write latency. |
| First-error validation | Clear indexed errors plus accessible examples are sufficient initially; no aggregate error framework required. |
| Essay word-limit representations | Existing limitation, not a new regression. Build -> Preview still needs a human check. Do not alter payloads based only on disagreeing readback properties. |
| secure_params JWT | Instructure's source constructs a signed assignment ID/description payload and consumes it in LTI launches. This is not evidence of PAT leakage. A small opt-in response projection or narrow documented omission can reduce noise; avoid broad redaction of raw escape-hatch data or signed asset URLs needed for supported reading. |
| noexec harness mount | Harness issue; use the interpreter explicitly. No Engine fix. |

Primary source for secure_params: [Instructure AbstractAssignment](https://github.com/instructure/canvas-lms/blob/master/app/models/abstract_assignment.rb) and [assignment API serializer](https://github.com/instructure/canvas-lms/blob/master/lib/api/v1/assignment.rb). Upstream source does not establish the precise district deployment version or permission scope of this particular token.

## Proposed implementation batches

1. **Authoring contract and confirmed small fixes:**
   - Embedded QuizForge accepted-shape reference/example through `engine_reference`.
   - Unknown item/choice key warnings, plus the specific inline-`rationale` error.
   - Local quiz validation runs the same settings check as review/push.
   - `allowed_attempts` without `allow_multiple_attempts` is rejected or explained, not silently ignored; review shows effective settings from the payload.
   - Recognized rationales shown in review.
   - Format-based review dispatch with the cheap course-mismatch check (wrong-target guard).
   - Quiz publication after requested module placement.
   - `use_for_grading: true` refused on newly created rubrics; existing coupled snapshots stay supported.

   Focused regressions: canonical feedback survives, inline rationale fails helpfully, an unknown key warns while tolerated metadata/notes stay accepted, setting typo fails local validation, assignment/results with course routes correctly, failed module step leaves a new quiz unpublished, new-rubric true fails.
2. **Review status:** simple state field, URL only while usable, returned pushed path, honest completed/partial/held outcome, active review_id in the refusal. Optional pending-only cancel. No queue or durable session service.
3. **Scoring errors:** concise strict-schema diagnostics at the parse boundary pointing to `interpretation.mapping`, `grade.components` and `engine_reference`, without relaxing scoring requirements.

**Separate teacher task (do not drop):** the SCR 4/5/6 `results.md` files hold unsent teacher judgments in an invalid shape. Re-expressing them canonically against the pulled work is its own scoped task, leaving originals intact.

When implementation lands, update `internal/localui/capabilities.go` and `docs/engine-capabilities.md` again so entries no longer describe the fixed items as unresolved, keeping local-test versus Canvas Live evidence distinct.

These are proposals for agreement; no implementation agents were dispatched. Capabilities evidence is updated now because the user-provided run materially changes what has been observed.

## Verification and remaining scope

Passed existing focused tests: `go test ./internal/workflow -run 'TestQuizForgeTransformMatchesVerifiedShapes|TestQuizForgeRejectsWhatCanvasCannotDo' -count=1`. Local diagnostic probe under ignored `.test-workspaces/cowork-log-review-20261011/` confirmed the actual draft recognizes zero rationale entries and a setting typo is accepted by the parser but rejected by review. Those are local observations, not new Canvas Live trials.

Some log annotations overstate elapsed time: 19:51:24Z is about 2m38s after the observed 19:48:46Z review start, not "10+ min". Assignment review's recorded 9m13.898s total is plausible. Use exact timestamps and Engine phase counters, not narrative estimates.

Reported leftovers for teacher cleanup: CS8 assignment 3698400, New Quiz assignment 3698401 and rubric 71839; module 636143 contains returned items 9026780 and 9026782. Both assignments were reported unpublished. No cleanup was performed, and no fresh live state check was made in this review.

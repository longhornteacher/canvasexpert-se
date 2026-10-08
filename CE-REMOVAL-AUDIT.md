# What has no place in CanvasExpert-SE

**2026-10-07 — architecture audit, not code changes.** Inspected CE `dev` at `9dda1a12b4128b2be34d7e649ebccbd0c9aaee21`. Paths below refer to `D:\Development Projects\CanvasExpert`. This audit revises the initial SE seed: speed is the goal, and CE's safeguards/workflows are not presumed necessary.

## 1. The decision rule

Keep a blocking check only when it prevents a named, material failure that still exists on one PC with approved agents, and a cheaper approach does not suffice. Identify its actual target and fields. Otherwise delete it, make it informational, or perform it in the background.

Optimize teacher waiting and agent round trips first, then network, repeated serialization/scanning, disk activity and background contention. Do not trade a five-millisecond validator for a minute of avoidable conversation. No benchmark was run for this audit: costs below are structural findings, not measured timings.

The first seed was too conservative in inheriting compulsory scoring preview/approval, generic operation machinery and blanket live checks. Those defaults are withdrawn. Preview remains a capability; it is not an entry fee for every authorized action.

## 2. Delete outright

| CE mechanism and source anchor | Why it costs time or blocks work | SE disposition |
|---|---|---|
| Identity vault, pseudonym allocation/resolution, transfer secrets: `feedback_vault.py`, `identity_vault_service.py`, `identity_ledger.py`, `pseudonym_secret.py` | Extra lookups, private state dependencies, setup and failure modes for an identity boundary SE does not need | Delete services, identity-key setup, vault recovery and dependent checks; use Canvas user IDs and names |
| Scrubbing and outbound privacy scans: `feedback_scrub.py`, `feedback_safety.py`, `mcp_server/pseudonym.py::gate` | Scans/transforms student text; can withhold usable results | Delete the PII pipeline and pseudonym-shaped schemas, including name-in-feedback warnings |
| Second structural output gate: `mcp_server/tools.py::final_response_gate` | Walks results to detect student structures, reopens the vault and gates again after tool-level handling | Delete; keep ordinary output serialization |
| Distributed stores: `shared_storage.py`, `shared_kv.py`, `shared_work.py` | Journals, conflict files, lease state and recovery exist for concurrent computers | Replace with local DB rows/transactions; no compatibility subsystem |
| Acquisition ownership/presence/mailboxes: `mirror/acquisition_owner.py`, `mirror/acquisition_requests.py`, `runtime.py` owner worker | Election and heartbeat work no longer has a second machine to coordinate | Delete; one local coordinator owns acquisition |
| Cross-device work transfer: `transfer_work_item`, shared-work lease refusals | Makes usable local sessions conditional on ownership/takeover state | Delete tool and lease protocol; use local in-flight target ownership |
| SAFE evidence publication and synchronized commit reduction: `mirror/evidence_publish.py`, `evidence_store.py`, `evidence_index.py` | Private -> safe -> files -> index copies exist partly to preserve privacy and deterministic merge across devices | Replace topology with direct local transactions and queries; retain historical facts where useful, not merge machinery |
| Privacy-only readiness: `readiness.py::_probe_privacy`, Names/vault console | Imports privacy services and probes private storage readiness; unnecessary status concepts for SE | Delete privacy component and screens; actual storage errors are handled where storage is used |
| Agent-facing censorship of names, IDs, local paths and diagnostic details | Forces indirect lookups and obscures failures in an approved environment | Return useful identities, paths and errors directly; continue excluding credentials |
| Student-free Course Catalog schemas and blanket no-student-to-tier knowledge | Extra privacy boundaries prevent useful joins and named-student differentiation | Use normal indexed views over one store. Permit teacher-directed student/tier associations; do not automatically choose tiers |
| Approved-host attestations or policy certification proposed in the first seed | Would add a new gate after removing the old one | No runtime implementation; approved environment is a deployment assumption |

Remove their instructions, tests and refusal codes with the implementation. Do not preserve privacy/replication tests under a renamed "safety" heading. Keep any independent parser, transport or data-integrity logic discovered inside a retired module by moving only that logic to its actual owner.

## 3. Remove read and preparation friction

| Concrete CE behavior | Source anchor | SE replacement |
|---|---|---|
| Saved/Current course membership is checked before student reads | `mcp_server/tools.py::_saved_course_gate_check`, `_course_gate_check` | Course lists are discovery defaults. An explicit accessible course ID works immediately; Canvas determines remote access |
| Evidence-reader resolution first opens a vault and may demand index maintenance or a separate mirror refresh | `mcp_server/tools.py::_evidence_reader` | Query local records directly; a miss fetches the exact scope in the same request. Optional search-index failure falls back to ordinary queries |
| Only mirror/index data may reach the agent, even when a focused live request would answer faster | MCP mirror-only section; `docs/mirror.md` | Cache-first, focused-live-on-miss, explicit offline/current modes; no relay prohibition and no full-course refresh prerequisite |
| Scoring preparation refuses non-current/old projections or needs `use_existing_mirror` confirmation | `powergrader/scoring_preparation.py::_freshness_refusal` | Return available evidence with its age; fetch missing requested data. Check relevant current attempt/grade facts when posting, not via a teacher assertion that nothing changed |
| A frozen scoring packet can become unreadable after the course mirror revision changes | `mcp_server/tools.py::_session_mirror_check`, `_session_mirror_changed` | A snapshot stays readable. Mark newer relevant evidence if known; no course-wide invalidation. Check the actual selected rows at send time |
| Preparing an already-open healthy session returns `ok: false`, `scoring_session_already_open` | `mcp_server/tools.py::_open_scoring_session_refusal` | Idempotent open-or-get success, returning useful context/results immediately |
| Prepare returns an ID, then the agent must call packet tools to obtain the content | `scoring_preparation.py::_ready_payload`, `get_scoring_packet` | Return assignment context and the first requested submission page with the handle; page only when the data size requires it |
| SAFE packets are exported/validated as part of preparation | `scoring_preparation.py`, `powergrader/scoring_packet.py` | Read directly by evidence IDs/versions; save snapshots only when resumption or requested review needs them |
| Timeline/overlap work runs during applicable preparations | `scoring_preparation.py::_attach_overlap_evidence`, `attach_writing_timelines` calls | Lazy or cached enrichment. Include it when the task/rubric needs it; otherwise don't delay ordinary scoring |
| Catalog freshness can stop an operation | `operation_ledger/executor.py::_execute_target`, `catalog_not_current` | Resolve only necessary assignment-group/module/target facts; exact-ID operations need no catalog-wide freshness |

CE's current read contracts already allow much old/partial evidence. Do not pretend all old data is universally blocked today: the specific remaining problems above occur in reader initialization, scoring preparation, session access and selected action paths. Similarly, readiness is a status probe in the inspected module; do not label it a universal execution gate without evidence.

Remove the agent-managed refresh dance, not the cache. Cached data can be the fastest path; forced cache repair is not. Background refresh is useful only where it measurably avoids foreground waiting, and yields to requested work.

## 4. Remove action ceremony

### Authorization once, at the level the teacher gave it

CE scoring guidance requires a staged preview and a direct subsequent request to post (`docs/guides/scoring-sessions.md`, CanvasAgent instructions). SE drops that universal requirement.

| Teacher request | SE behavior |
|---|---|
| "Score and post Assignment X using its rubric" | Get necessary evidence, score, submit exact results and report; no compulsory second approval turn |
| "Draft feedback for Assignment X" | Save/return a draft; do not post |
| "Show me the scores before posting" | Save a review; resume it when the teacher approves |
| "Give this student the specified extra time" | Make the exact authorized change; no forced preview/apply pair for a reversible local setting |
| A genuinely ambiguous student, missing grading basis, or instruction outside the requested scope | Ask the one question needed; continue independent authorized work |

CE already has direct authoring via `push_content_live` and some workflows honor prior instruction; keep that convenience. The audit is not a claim that every CE action currently demands a second approval.

### Cut or narrow these mechanisms

| CE mechanism | Evidence | SE disposition |
|---|---|---|
| Separate stage, preview and apply round trips even for an already authorized scoring task | `stage_scoring_results`, `get_scoring_preview`, `apply_staged_scoring_results` | One result-submission call executes authorized work; retain save/preview for requested review |
| Packet, plan, question-review, stage and batch digests; repeated plan construction | `mcp_server/tools.py::_stage_scoring_results_locked`, `_stage_packet_refusal`, `get_scoring_preview`, `_apply_staged_scoring_results_locked` | One saved-record revision where work crosses calls; validate immutable stored payload once at ownership boundary. File hashes remain where they identify bytes |
| Mandatory confirmation that unselected/held rows receive nothing | `powergrader/scoring_apply.py::QUESTION_OPTIONS`, `build_plan`, `held_not_scored` | Report held rows; proceed with authorized independent selected rows |
| Comment-only, extra-credit and pseudonym-in-feedback questions | Same question table and builder | Explicit feedback-only mode and teacher-authorized extra credit need no reconfirmation; pseudonym question disappears. Unexpected values outside the instruction still need resolution |
| Repeated late-day/insincere-attempt questions | Same question table; scoring stage review | Apply already specified teacher policy. Ask only about materially missing decisions for affected rows; no invented penalty or automatic misconduct judgment |
| Entire scoring batch rejected if any selected grade drifted | `scoring_apply.py::check_live_grade_state` | Partition changed/unchanged rows; proceed with independent authorized rows, hold conflicts. Preserve an explicit all-or-nothing request |
| Generic baseline capture for every operation, including catalog freshness checks | `operation_ledger/executor.py::_execute_target` | Per-action relevant-field check only; no old-value preflight for creates |
| An extra live posting-policy read during staging solely for warnings | `mcp_server/tools.py::_posting_policy`, `scoring_apply.py::read_posting_policy` | Reuse acquired policy metadata. Fetch current policy only when requested visibility depends on it; otherwise report unknown without a foreground warning-only call |
| Routine separate agent `verify_live` after push | `mcp_server/tools.py::verify_live`, authoring guidance | The action returns observed result/IDs. Read back only missing facts or uncertain outcomes; no second check of already confirmed facts |
| Grade send depends on validating historical score-ledger scope and appending through another store | `powergrader/session_actions.py::push_grades`, `score_ledger.validate_scope` call | Persist current send intent and score data in one local transaction. Export history asynchronously; unrelated old archive trouble does not block the new write |
| Generic claim/lease framework for simple commands | `operation_ledger/executor.py`, `claims.py`, JSON storage documents | Local target ownership plus minimal durable action rows; no lease heartbeat. Keep ordered checkpoints for genuinely multi-step actions |
| Separate local roster preview/apply tools | `preview_roster_student_change`, `apply_roster_student_change` | Direct validated edit with prior value for undo; conflict check only if a saved edit would overwrite a newer edit |

Checks against immutable internally owned records should not be repeated at each wrapper. Validate external input once, use database constraints where they suffice, and check record versions only at real edit/send boundaries. A new optional preview must not reinstate the entire retired digest stack.

## 5. What still earns its cost

These are narrowly justified by concrete SE failures. They are not permission to retain CE's broader implementations.

| Minimal mechanism | Concrete failure prevented | Lowest-cost location |
|---|---|---|
| Exact target identity and basic payload validity | Posting to the wrong student/course; sending an invalid score/date or unintended field | Resolve ambiguity once; type/shape validation at input. No roster-wide name scanning |
| Selected-row attempt/grade comparison for evidence-based grading; relevant-field comparison for overwrites | Replacing a newer teacher grade or applying feedback to a resubmitted attempt | One scoped batched fetch, coalesced with necessary live acquisition; no mirror/catalog-wide revision check |
| Minimal durable intent, result IDs and duplicate-request lookup | Crash or retry creates duplicate assignments, uploads or comments | One DB transaction before send and a result update afterward; reuse existing response for completed retry |
| One local execution owner for conflicting writes | Two agent tabs post the same work simultaneously | In-process target lock plus durable state; no distributed lease |
| Reconciliation before resending an uncertain non-idempotent request | Timeout concealed a successful remote write | Automatic exact-ID/read-only reconciliation if possible; hold only unresolved target |
| Readback when a response does not establish requested effect | Canvas late-policy transformation or partial action differs from intended result | Reuse response first; otherwise one relevant batched read. No duplicate agent verification |
| Student-facing payload allowlist | Private teacher commentary accidentally posted as student feedback | A serializer that selects explicit public fields; no content/PII classifier |
| Credential isolation, loopback request protection, safe URL/file handling | A webpage triggers local mutations, a redirect leaks the token, or an attachment escapes its directory | Existing small transport/server/parser boundaries; no host certification framework |
| Finite resource limits, cancellation, rate-limit handling | Huge files or unbounded pagination freeze the PC or exhaust Canvas requests | Transport/extraction bounds, useful partial output and continuation. No arbitrary small limits justified only by privacy |
| Snapshot consistency and no replay after restore | Corrupt backup or duplicate sends from an older checkpoint | Backup/restore time only; zero prerequisite on normal reads/actions |

Retained checks stop only the work whose result depends on them. Failure to durably record an outgoing mutation stops that mutation; it does not block local reading. A broken optional index can be rebuilt later. A missing original needed for grading holds that student, not the whole course.

## 6. Fast paths to implement

**Read:** local query -> useful result. On miss: exact Canvas fetch -> useful result plus cache update. If acquisition is long, return progress/available evidence once with a continuation handle. No privacy scan, ownership election, published-fact conversion, required index regeneration, or teacher confirmation.

**Ordinary grading:** one context call supplies assignment/rubric and selected student evidence -> agent scores -> one submit-results call validates targets, checks relevant live rows in a batch, records/sends, checks any unconfirmed score effects in a batch, and returns per-row results. Missing evidence is explicit. Saved review is optional. There can still be necessary pagination and Canvas calls; the goal is to eliminate artificial agent/tool transitions.

**Create content:** accept/render/validate the requested content -> record intent -> create/upload with necessary dependent checkpoints -> return Canvas IDs and observed status. No roster, mirror readiness, entire-catalog freshness, or generic before-value check. Ambiguous titles do not substitute for exact IDs when recovering an uncertain create.

**Local setting:** validate requested edit -> commit with previous value -> return. No review object, Canvas call, receipt export or approval question.

**Startup:** open local store and serve. Reconcile only interrupted targets, in the background or when that target is used. Do not wait for cloud status, full evidence index rebuild, authoring-library regeneration, OCR initialization, or historical backfill. A required database migration is a real storage prerequisite; optional services are lazy.

## 7. Evidence that SE is actually faster

When a slice changes a workflow's cost, record teacher interruptions, MCP calls, Canvas calls, bytes scanned/returned, and time to first useful result/total completion on the same representative task. Use cold and warm runs for the affected path; reuse unchanged baseline evidence. Do not claim speed from a smaller line count.

Structural acceptance targets:

- Warm read: zero Canvas calls, zero privacy scans, no mandatory side effects.
- Cache-miss read: zero teacher questions and no agent-managed refresh/retry loop.
- Authorized score-and-post: normally one context call plus one results call; zero extra approval turns. Size-driven paging is explicit.
- Grade drift in one independent row: that row held; unchanged authorized rows finish without restaging the whole assignment.
- Unrelated mirror changes: zero discarded scoring work and no blocked snapshot reads.
- Direct local setting: one call and one transaction.
- Backup, optional OCR, comparisons and diagnostics: no foreground prerequisite unless specifically requested/needed.
- Retry after ambiguous send: no duplicate external action. Faster must not mean the teacher spends the saved time cleaning up Canvas.

These targets belong in the first executor briefs, alongside correctness examples. Do not implement an instrumentation platform or a runtime checklist to enforce them: small timings/counters in the benchmark are enough.

## 8. Second pass: keep the build itself lean

The second pass rechecked the same clean CE baseline and both SE planning documents. SE still contains no implementation or Git checkout. The following source findings and planning corrections supplement the earlier audit; no runtime timings or live Canvas behavior were measured.

| Finding | Evidence | Change to the SE handoff |
|---|---|---|
| Agent instructions can recreate deleted backend gates | `api/mcp_server/server.py::_SERVER_INSTRUCTIONS` still directs refresh/retry, SAFE packet staging, preview and a later teacher go; `api/tests/mcp_server/test_server_instructions.py` asserts those instructions | Rewrite the affected instructions, descriptions, guides and expectations with each workflow. Direct execution must be the taught path, not just an undocumented backend capability |
| Inherited executor rules can turn routine discrepancies into teacher interruptions | CE `AGENTS.md` says to ask when defaults do not fit or a preflight assumption is false | The SE brief gives decision authority for routine implementation choices. Escalate changes to outcome, scope or live-state preservation; record ordinary deviations and continue |
| Startup performs work unrelated to the first request | `api/runtime.py::start` builds the authoring library and recovers pending operations synchronously before launching evidence/index and ownership workers; `api/runtime_host.py::create_host_app` attempts console import on host creation | Serve after required local-store/runtime setup. Initialize optional services when used; inspect interrupted targets without a global startup hold. Make MCP-only startup independent of console import. Keep a simple local worker mechanism |
| The HTTP fast path has an opportunity for connection reuse | `api/platform_services/canvas_client.py::_physical_get` uses module-level `requests.get`; some attachment paths already use a session | Reuse bounded transport connections with a clear worker/session owner; preserve redirect credential handling. This is a focused transport change, not grounds for replacing the HTTP stack. Do not share mutable sessions unsafely or enable automatic retries for uncertain writes |
| A single database could accidentally serialize unrelated work | Architectural risk introduced by the proposed shared SQLite store, not a measured CE defect | No network, rendering, OCR or backup work inside database write transactions. Use target ownership across sends and short transactions for intent/results; no global action lock |
| Fewer calls can still produce slower agent work | CE already budgets server instructions/tool listings in `test_server_instructions.py`; response size was missing from SE's explicit targets | Preserve compact schemas; measure result bytes too. Shared assignment/rubric context appears once, requested evidence stays complete or explicitly paged, and guides are fetched only when relevant |
| Cache-first reads need a cheap post-write rule | A consequence of the new SE read/write design | Patch known current fields from successful responses or invalidate only affected fields. Cache trouble must not cause a confirmed write to be repeated; immutable evidence stays intact |
| The original sequence could encourage a foundation rewrite and delayed deletion | Seed section 9 previously put a complete 38-tool mapping before implementation, bundled authoring with first scoring, and placed removal in a final phase | Inventory as workflows are ported; build the first read with only needed schema; score an existing assignment before authoring is ready; delete obsolete dependencies and guidance in each slice |

No replacement gate is needed for the removed approval system. In particular, do not introduce an approval service, intent-registration step, or `teacher_confirmed` flag that the agent must supply on every direct action. Follow the teacher's authorization, validate the concrete scope/payload, and retain only the relevant conflict and recovery behavior.

The executor's brief should fit the work: one outcome, exact source entry points, owned files, a few meaningful acceptance cases and applicable checks. No framework-first brief, full compatibility matrix, mandatory benchmark suite for unrelated edits, or separate ceremony to accept an already verified slice. Do not create automated tests for this documentation pass.

Additional acceptance examples for the affected implementation slices:

- A delayed Canvas send does not hold a database write transaction or prevent an unrelated cached read/local edit.
- A successful edit followed by a read cannot return the known previous value as current; optional cache failure cannot change success into an invitation to resend.
- MCP startup does not import the optional console or initialize OCR; either remains available on demand where supported.
- A multi-student evidence response supplies shared context once, makes paging/omissions explicit, and preserves the evidence needed to score.
- The instructions delivered to an agent describe the direct authorized path; obsolete tests cannot require the retired preview/approval sequence.

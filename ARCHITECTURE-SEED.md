# CanvasExpert-SE: architectural seed

**Date:** 2026-10-07

**Status:** Second speed/leanness pass complete; planning only, no implementation or migration performed.

**Purpose:** Full useful CanvasExpert functionality with the shortest practical path from teacher intent to result. Remove requirements, checks and intermediate workflows that do not prevent a specific material failure in SE.

## 1. Product direction

CanvasExpert-SE is a single-machine, teacher-controlled Canvas runtime for use with agents in data-policy-approved environments. The teacher works conversationally with an agent; SE supplies course knowledge, student evidence, authoring services, grading workflows, and dependable Canvas actions. A small local console supports setup, status, recovery, and receipts.

The two defining decisions come from the teacher:

1. **One active machine.** Cloud storage protects teacher data through backup. It does not coordinate active machines, merge application state, or carry work between concurrent runtimes.
2. **Real identities are allowed at the agent boundary.** Approved agents can work with names, Canvas identifiers, submissions, and other relevant student information directly. SE does not maintain an identity vault, substitute pseudonyms, or create scrubbed copies to make routine work possible.

These decisions remove product requirements, not just configuration options. SE should not become CE with two feature flags and all of the old machinery underneath.

**Speed is the governing product requirement:** fewer teacher interruptions, fewer agent tool round trips, less duplicated processing and storage, and less waiting for unrelated work. A CE check does not survive merely because it exists or once caught a bug under CE's different assumptions. The burden of proof is on keeping it: name the concrete SE failure, the smallest check that prevents it, and why a warning, narrower check, cached result or recovery would not suffice. Speculative risks do not qualify.

The companion [CE removal audit](CE-REMOVAL-AUDIT.md) records specific source findings, deletions, replacements and the small set of retained checks. It is part of this direction. In particular, SE does **not** inherit CE's compulsory post-preview scoring approval, mirror-only reads, course-wide session invalidation, or blanket write-preflight rules.

Everything else below is a proposed architectural default unless identified as inherited behavior. This document establishes direction for discussion and later executor briefs; it does not authorize a wholesale rewrite, live Canvas changes, or deletion of existing teacher state.

## 2. Baseline and meaning of parity

This seed was grounded in the local `D:\Development Projects\CanvasExpert` checkout on branch `dev`, commit `9dda1a12b4128b2be34d7e649ebccbd0c9aaee21`. Its MCP schema is version 82 with 38 listed tools. The SE directory was empty when inspected. No remote comparison or runtime acceptance test was performed; this is a local source-and-document baseline.

CE's domain knowledge, Canvas integrations, rendering and extraction are valuable. Reuse them without treating its runtime ceremony as a requirement. The largest structural cuts are distributed stores and privacy projections; the largest interaction cuts are unnecessary prepare/read/preview/apply round trips and teacher confirmations.

**Parity means preserving teacher outcomes and important semantics, not preserving every tool name, storage format, or privacy-specific screen.** Existing documentation includes implemented contracts, target designs, and unfinished field acceptance. The current CanvasMirror handoff is explicitly YELLOW. Carry forward working behavior and identified gaps; do not describe CE's planned behavior as proven functionality.

| Capability to preserve | SE expectation | CE starting points |
|---|---|---|
| Setup and agent connections | Canvas account/token, workspace, current/previous courses, desktop connection help, diagnostics | `api/runtime.py`, `api/runtime_host.py`, `api/ai_clients.py`, `api/platform_services/` |
| Course knowledge | Assignments, pages, modules, groups, sections, gradebook, roster; bounded reads with age and coverage | `api/mirror/`, `api/course_catalog.py`, `api/gradebook_queries.py` |
| Authoring | AssignmentForge, PageForge, Classic/New QuizForge; validation, presentation, attachments, rubric, dates, publish state, module placement | `api/default_docs/AI Authoring/`, `engine/`, `api/content_push.py`, ledger adapters |
| Differentiation | Existing Support/Core/Accelerate content, Hub/Bridge workflows, public tags, SIS grade bridges | Forge contracts, `docs/contracts/sis-grade-bridge-contract.md` |
| Scoring and feedback | Discover/read/score/post within the teacher's scope; optional saved drafts and previews; feedback-only and feedback-revision modes | `api/powergrader/`, `docs/guides/scoring-sessions.md` |
| Grade management | Existing-grade adjustments, curves, extra attempts/reopening, late policy and entered-score semantics | `api/grade_adjustment.py`, `api/score_curves.py`, grading and adjustment contracts |
| Student context | Extra time, monitored students, classroom profiles and teacher notes using exact student identity | `api/roster_service.py`, roster configuration |
| Evidence and history | Observed submission attempts, originals, extraction/OCR, writing timelines, comparisons, score history | `api/mirror/`, `api/powergrader/writing_timeline.py`, `api/score_ledger.py` |
| Reliable actions | Exact targets, minimal durable send records, target-specific conflict protection, useful results and recovery | `api/operation_ledger/`, scoring apply services |
| Local work | Drafts, reusable materials, feedback guidance, resumable sessions, teacher artifacts | Workspace services, session stores, authoring library |

Source-relative paths above refer to CE, not files already present in SE. Record each current MCP tool's disposition as its workflow is ported: retain, adapt, replace with an equivalent outcome, or retire because it exclusively serves a removed requirement. Keep a small inventory so nothing useful is lost; completing all 38 entries is not a prerequisite to the first workflow. In particular, retire cross-device `transfer_work_item`; adapt `list_work_items` to local resumable work. Do not use an unchanged tool count as a parity gate.

Canvas limitations do not disappear with pseudonymization. CE currently does not support posting New Quizzes item scores or per-item feedback through its agent scoring path; preserve honest capability reporting and its ordinary-assignment writing workflow. Broader New Quizzes support requires separate investigation and verification.

## 3. Target system shape

```text
Teacher + approved agent host
             |
      host-neutral MCP
             |
  One local SE runtime <---- loopback control console
             |
    shared application services
      /          |           \
 Canvas reads   local work   authorized Canvas actions
      |          |           |
      +---- local state -----+
             |
  consistent backup snapshots ----> approved cloud storage

Canvas remains the authority for current Canvas state.
SE owns teacher drafts, captured history, work state, and receipts.
```

Keep Python, the existing MCP approach, FastAPI for the optional console, and the offline rendering/extraction investment. There is no demonstrated reason to replace the language, add microservices, introduce a message broker, or build an agent harness.

Keep MCP and browser handlers thin. Application services own behavior and remain usable without starting the console. The runtime returns useful data immediately, with stable IDs and concise status. Agent-host rendering presents previews when requested or needed for a missing decision. Do not make every request construct a review object or digest chain.

One OS-level lock owns a local SE installation/workspace. Additional local agent clients attach to that runtime. Background refresh and extraction can use bounded local concurrency. “Single machine” does not mean one CPU core, one agent conversation, or the absence of races.

Use database transactions and one local execution owner per conflicting target to prevent overlapping sends. Durable pending-send records survive crashes; in-process locks need no lease heartbeat or expiration protocol. Use record versions where an edit crosses calls. Do not retain device election, distributed heartbeats, shared leases, or machine-to-machine takeover protocols.

Keep database transactions short: commit intent, do network work outside the transaction, then commit results. Hold the relevant target ownership across that sequence, never a database write transaction or global application lock across Canvas calls, OCR, rendering, or backup. Unrelated reads and actions remain available. Reuse the existing transport with bounded connection reuse and concurrency; do not make an async rewrite, generalized job scheduler, or plugin framework a foundation milestone.

## 4. Data ownership and storage

### 4.1 A local transactional store plus files

Proposed default: one SQLite application database on a local, non-synced disk, plus immutable original/artifact files and ordinary teacher-authored documents. Use the existing source layout initially; package renaming is not the first product milestone.

Illustrative layout, outside the source checkout:

```text
<local-SE-data>/
  state.sqlite3           current projections, work state, settings, durable events
  originals/              immutable captured source files, addressed by digest
  artifacts/              rendered drafts and explicit exports; snapshots when needed
  workspace/              teacher-editable library, drafts, and exported documents
  cache/                  regenerable extraction/search/rendering intermediates
  runtime/                local locks and transient runtime metadata

<approved-cloud-backup>/
  snapshots/<snapshot-id>/
    manifest.json
    state.sqlite3
    originals/ ...
    artifacts/ ...
    workspace/ ...
```

The layout is illustrative, not an instruction to create or relocate the user's data now. The cloud destination and retention policy remain to be selected.

| Data | Authority | Recovery rule |
|---|---|---|
| Current Canvas roster/content/submission/grade projections | Canvas | Refresh or rebuild; never treat cached values as write authorization |
| Captured attempts, original uploads, score events and receipts | SE's retained observations | Back up and preserve; Canvas may no longer supply the historical evidence |
| Drafts, feedback guidance, local student context and teacher settings | SE/teacher | Back up; cannot be reconstructed from a mirror refresh |
| Saved drafts, optional sessions and outbound action records | SE | Transactional persistence; reconcile uncertain sends before retry |
| Search indexes, thumbnails and other derived caches | Underlying records/files | Rebuild without requiring a new Canvas acquisition where possible |
| Canvas credentials | OS credential store | Re-enter/reconnect after recovery; exclude from ordinary backups and agent output |

One physical database does not make all tables disposable. Explicitly separate current projections from durable teacher evidence and operation records in the schema and in reset/pruning code. A “refresh mirror” must never erase score history, originals, drafts, or receipts.

Store the source Canvas instance/account context with Canvas IDs. A student key is an origin/account-scoped Canvas user ID; names are display/search fields, not identifiers. Store course enrollment separately so a student can appear in multiple courses without duplicate identities. Disambiguate duplicate names before preparing a mutation.

Persist original bytes once and use references for consumers. Extraction records include source digest, extractor/version, completion status, and provenance; retain useful partial results and explicit failures. No parallel SAFE/original representation is needed merely to change identity. Return assignment context and requested submissions together; evidence IDs/versions identify what the agent scored. Persist a frozen packet only for a saved review or resumable task, not as an obligatory file-export step before every read.

Retain useful historical observations and the minimal outbound action/result record in the database. Score details can share that transaction. Replace cross-device journals and merge reducers with local transactions. Receipt files, reports and richer archive projections are on-demand or background exports, never a second mandatory write path. Do not validate an entire history archive to authorize one new score. Avoid a general event-sourced system for settings and current projections.

### 4.2 Cloud backup is a separate workflow

The working database, WAL files, locks, and queues must not live in a consumer sync folder. A cloud copy is recovery material, never a live peer. A teacher document folder may later be cloud-synced for convenience, but SE must not depend on that sync to commit or resume application work.

Build consistent, versioned snapshots locally using SQLite's supported backup mechanism or a quiesced database, plus the exact referenced immutable files. Coordinate capture of editable documents so each included file has a defined captured version. Pin referenced blobs against pruning while the snapshot is built. Publish a manifest with schema/application version, capture time, file inventory and checksums only after the snapshot is complete.

Upload/copy completed snapshots to approved storage. An incomplete or still-uploading snapshot is not recoverable. Distinguish **local snapshot complete**, **cloud transfer pending**, and **cloud copy verified**; handing a file to a sync client does not prove remote durability. Cloud outages must not block ordinary local work. Expose the last verified cloud backup and unprotected work interval plainly.

Use the teacher's approved cloud environment and its normal protection. Do not build a new identity/privacy gate or custom encryption/key-transfer subsystem. Any separately requested backup encryption must have an independently recoverable key. Backup creation and verification run outside the teaching workflow; neither backup lag nor a missing cloud-status integration blocks local work. A restore drill, rather than an elaborate readiness subsystem, establishes whether the chosen backup can be recovered.

Restore into a separate local directory with the runtime stopped. Validate completeness, checksums, database integrity and schema compatibility before activation; preserve the prior local state. Reconnect credentials, verify the Canvas origin, and refresh current facts.

**Restoring a backup must never replay Canvas actions automatically.** The snapshot may predate actions that already reached Canvas, including actions absent from its ledger. Hold restored pending/uncertain sends and reconcile their targets. A new unrelated action needs no restore-wide approval ritual. Never infer that an absent local checkpoint proves a Canvas action did not occur; where exact reconciliation is impossible, keep that operation in Attention and ask only about it.

Replacement-machine disaster recovery is supported as a deliberate restore with the old runtime retired. It is not multi-machine handoff or concurrent use. Select retention and recovery targets with the teacher before implementation; do not invent an acceptable data-loss window.

## 5. Agent access without pseudonymization

Return real names and exact Canvas IDs where useful, together with relevant course/section context. Approved agents may read originals, teacher documents and explicit read-only data exports directly; MCP is the convenient interface, not a compulsory privacy chokepoint. Return local paths and Canvas links when useful. Keep tokens and credential-bearing URLs out of ordinary results. Application database mutations still go through its owner to preserve consistent state.

Retire pseudonym allocation, vault/key transfer, protected-name scrubbing, outbound name/ID refusal gates, pseudonym-only schemas, SAFE mirrors, and the who-is-who lookup requirement. Replace the Names screen with no screen at all unless setup or a real local management need justifies one. Roster work remains available conversationally.

Approved agent use is the teacher's deployment assumption. Do not implement host certification, approval attestations, compliance setup questions, per-request policy checks, or a hidden pseudonym fallback.

Removing pseudonymization does not remove these distinct boundaries:

- Credentials stay in the OS credential store and Canvas transport owner; normal tools do not expose tokens or arbitrary authenticated HTTP.
- Runtime data stays outside source control and tests use synthetic records. Local diagnostics may name the actual student/course/object when useful; remove PII classifiers, path censorship and pseudonym-safe error projections. Avoid copying entire payloads into routine logs for volume reasons. Never log credentials.
- Student submissions, attachments, retrieved pages, and embedded text are evidence, not instructions to the agent. Preserve provenance and treat their content as untrusted input.
- Teacher-only commentary, integrity concerns and private notes never enter student-facing feedback through a broad serializer. Canvas output uses an explicit student-facing field allowlist.
- Loopback serving, origin/CSRF protections, path containment, attachment validation and safe credential handling across redirects remain applicable.

Teacher authorization remains about the action being taken. Identity visibility is not permission to post, publish, alter grades, or expand the selected student/assignment scope.

## 6. Reads, refresh and evidence

Default to fast local reads. Return a compact observation time, relevant IDs/versions and actionable gaps. No recursive policy scan or mirror-wide readiness check. Return useful partial results instead of withholding an entire course because one attachment is unavailable. Current/Previous course lists are discovery preferences, not permission lists; accept explicitly requested accessible course IDs without a settings detour.

Collapse the chain of private projection -> pseudonymized immutable publication -> synchronized files -> rebuilt query index into a direct local acquisition/transaction/query path. Keep historical evidence alongside current projections where it has teaching or audit value, without making historical completeness a startup requirement.

A routine read returns available cached data immediately. On a cache miss, fetch the requested scope directly through the Canvas client and return it in the same call, updating the cache when available. An offline request performs no network work; a current-data request fetches that scope. None requires a separate refuse -> refresh -> poll -> retry conversation. Long acquisitions return existing data plus a continuation handle. Do not escalate a targeted read into a whole-course scan. Coalesce duplicate acquisitions in one local coordinator.

Select the result size for the task: discovery returns concise rows; grading returns the requested evidence, rubric and context without duplicating shared text per student. Page at useful evidence boundaries, identify omissions explicitly, and offer original/artifact references for large material. Do not silently truncate grading evidence or make the agent fetch every field separately. Keep server instructions and tool schemas compact; supply detailed guides only when relevant. Measure response bytes as well as call count: one enormous response is not automatically faster.

After a confirmed write, update the affected current projection from the response or mark only its affected fields stale for a focused fetch. A following ordinary read must not present a known pre-write value as current. Preserve saved evidence and unrelated cached data. A failed optional cache update does not turn a confirmed remote success into a failed action or invite a resend; report the result and repair the cache later. Failure to preserve required evidence or send records is a distinct error, not an optional cache failure.

Mirror readiness is not a global product gate. Missing roster or OCR evidence must not prevent unrelated authoring. Age alone must not hide useful evidence. A missing attachment blocks only work that actually needs it. No mandatory semester backfill, grading-period configuration, or history-reconstruction ritual belongs in the first implementation.

A refresh cannot rewrite a saved review. Existing snapshots remain readable even if the current mirror changes or disappears. Before posting evidence-dependent results, compare only the selected students' relevant attempt/grade state; an unrelated roster, course or mirror revision does not invalidate work. OCR, comparisons, integrity analysis and writing timelines run only when needed or requested, and reuse cached results.

## 7. Actions, scoring and recovery

The default cooperation loop is:

```text
teacher intent -> get needed context -> execute within scope -> report result
```

An explicit instruction such as "score and post these submissions using this rubric" authorizes that work without a second "go" after a mandatory preview. "Draft feedback" authorizes drafting. "Show me before posting" requests a review checkpoint. Ask only for a genuinely missing decision or a material change outside the existing direction. Authorization is not invented, but SE adds no blanket requirement that it be repeated.

Express this in the shipped agent instructions and direct tools. Do not replace the removed preview with an approval-token service, mandatory intent-registration call, or per-request `teacher_confirmed` attestation. The runtime validates concrete action scope and payload; the agent follows the teacher's instruction. A saved review still identifies the exact revision being approved.

Direct actions accept the exact targets and payload, validate their shape once, record the send, execute and return the result. Saved drafts/reviews use one ID and revision to prevent accidental replacement across calls. Keep cryptographic hashes for file identity and deduplication where useful; remove packet + plan + review + batch + stage digest ladders and repeated plan reconstruction merely to prove the same local data still equals itself.

Do not preflight every write. Pure creates need no old-value drift check. Patches protect only fields whose current value matters to the request. Scoring/overwrites based on earlier evidence use one targeted batched comparison of the relevant grade/attempt state; a fetch already performed immediately before the send phase may satisfy it without another identical read. An earlier scoring-context fetch does not remain current merely because it belongs to the same task. No check makes the subsequent remote write atomic. If one independent target conflicts, hold it and continue authorized unaffected targets, reporting partial results. Honor an explicitly requested all-or-nothing selection before any sends, without promising transactional rollback across Canvas.

Persist minimal outbound intent before a consequential mutation, then checkpoint returned IDs/results. Completed repeated requests return their existing result. Uncertain non-idempotent sends are reconciled before retry; read-only reconciliation may run automatically. Hold only the ambiguous target when proof is unavailable. Keep multi-step checkpoints for uploads, tier families and other genuinely dependent operations. A one-field change does not need the full generic Operation Ledger review/claim/lease apparatus.

Use existing domain adapters where they save work, but replace their storage and ceremony rather than making SE permanently inherit them. Local roster/settings edits are direct undoable mutations. Missing scores in an explicit feedback-only request, allowed extra credit, or held rows outside the selected posting set do not generate mandatory questions. Incomplete evidence or a missing policy decision affects only the rows/actions that need it; do not invent a late penalty or punitive judgment to avoid asking.

Scoring continues to distinguish authored/raw score, entered score, Canvas's observed post-policy score, and deductions. Preserve teacher-chosen scoring guidance, rubric basis, curve math, late policy, feedback-only behavior, existing-comment revision, attempt history, and private agent commentary. No silent reinterpretation of scores is justified by the SE fork.

Use the Canvas response as the result when it supplies the required postcondition. Retain one targeted batched score readback where Canvas applies late policy or transformations not established by that response. Remove ritual agent `verify_live` calls after the runtime already confirmed the same facts. Read back only missing facts or uncertain outcomes; queue nonessential reconciliation. Report accepted versus verified honestly and never automatically repeat an uncertain comment or create.

## 8. Reuse and deliberate removal

| Preserve or adapt | Remove or replace |
|---|---|
| Domain services, Canvas transport quirks, Forge rendering/validation, extraction, grading semantics and focused tests | Pseudonym/vault services and agent-boundary scrubbing |
| MCP as the convenient host-neutral interface; direct artifact access | Pseudonym-shaped parameters/results, compulsory MCP-only data access and privacy-only refusal codes |
| One local runtime, local client attachment, bounded background workers | Cross-device acquisition election, presence files and request mailboxes |
| Optional sessions/reviews, local revisions, minimal send state and results | `_Shared` journals, device leases, takeover UI, conflict-copy reconciliation and obligatory multi-call reviews |
| Captured evidence, originals and useful local query views | Synced fact/commit merge topology, mandatory peer-driven indexing and mirror-only read restrictions |
| Setup, diagnostics, operation recovery and receipt console | Names/vault-transfer UI and distributed-store readiness gates |
| Cloud disaster recovery | Running SQLite or active work stores from cloud-synced files |

Relevant seams include `api/feedback_vault.py`, `api/feedback_scrub.py`, `api/feedback_safety.py`, `api/identity_vault_service.py`, `api/identity_ledger.py`, `api/mcp_server/pseudonym.py`, `api/shared_storage.py`, `api/shared_kv.py`, `api/shared_work.py`, and the mirror acquisition/evidence publishers. These are investigation targets, not a blanket deletion list: separate any independent validation or durable-history behavior before retiring a module.

SE should have its own application name, credential namespace, local-data root, MCP server identifier, update source, and default console-port strategy. CE and SE must not accidentally share a mutable workspace, select each other's credentials, attach to each other's runtime, or update from the wrong repository. Distinguish the running product in every receipt and status surface.

## 9. Transition and implementation sequence

Prefer a focused derivative of CE over a blank rewrite. Pin a source baseline, retain its licensing and provenance, and port/refactor in complete workflows. Keep one SE target architecture; temporary adapters must belong to a named migration slice and have a removal condition. Do not maintain a perpetual dual CE/SE mode or promise automatic upstream merges.

The senior agent owns architecture, scope and acceptance. After direction is agreed, it writes one bounded executor brief at a time with precise source references, file ownership, acceptance evidence and focused checks. Lower-tier execution agents implement those briefs. Each brief states the interaction and runtime cost it removes; do not replace retired checks with renamed equivalents. Executors resolve routine implementation choices and stale source references themselves, recording material deviations. Ask only when a discrepancy changes the teacher-visible outcome, authorized scope, or preservation of live state; a mismatched helper name is not a stop condition. This seed is not itself a command to launch implementation.

The SE brief and these decisions supersede inherited CE workflow rules. Replace conflicting `AGENTS.md` guidance, `_SERVER_INSTRUCTIONS`, tool descriptions, shipped guides and tests in the same slice as the behavior they describe. CE explicitly teaches and tests preview/approval, privacy and refresh rituals; copying its instructions unchanged would restore those gates even after deleting their backend code. Do not copy CE's blanket ask-on-deviation rule into SE.

| Sequence | Deliverable | Acceptance evidence |
|---|---|---|
| 0. Baseline and boundaries, within the first brief | Pin CE source; isolate SE identity/configuration; record the first workflow's tool disposition and inherited gaps | CE data and runtime remain untouched; no full parity audit or redesign prerequisite |
| 1. First useful read | Only the runtime/store pieces needed to read one course's roster, assignment, submission and original attachment with real identities; extend the schema as later consumers arrive | Restart preserves retained evidence; duplicate names are distinguishable; partial acquisition remains usable; no vault or shared-sync prerequisite |
| 2a. First scoring loop | Existing ordinary assignment: read -> score -> authorized post -> result; optional saved review | No repeat approval; target-specific conflicts and uncertain sends handled correctly; authoring is not a prerequisite |
| 2b. First authoring loop | Create an ordinary assignment and return observed result/IDs, using only the shared send machinery it actually needs | No mirror prerequisite, approval ceremony, or unrelated catalog scan; uncertain create does not duplicate |
| 3. Remaining parity | Pages, both supported quiz authoring paths, differentiation/bridges, curves/adjustments, attempts, profiles, feedback revision and retained evidence | Capability matrix complete, documented limitations visible, affected workflow tests pass |
| 4. Backup, restore and pilot cutover | Consistent snapshot, verified cloud copy, isolated restore, explicit state import if needed | Restore drill recovers irreplaceable artifacts and refuses automatic replay; credential and product isolation verified |
| Every slice; final residue check | Remove the retired dependencies, screens, guides and test expectations touched by that workflow; check remaining residue at parity completion | No shipped workflow calls retired systems; cleanup does not become a separate product milestone |

Design the backup format alongside the local schema and make a usable local snapshot early; complete cloud recovery before SE becomes the sole owner of new live teacher work. Do not defer preservation until after real data depends on the new system.

Each row is an outcome, not permission for a broad platform project. Split it when one executor cannot implement and verify it from a concise brief. Add tables, abstractions, UI surfaces and background jobs only for that slice's consumer. Reuse CE's existing console where useful; console redesign, installer work and a finished all-feature schema do not precede the first read. Lazy-load optional document/OCR engines; missing optional dependencies affect only requests needing them. Keep the existing Python/HTTP stack unless a measured bottleneck justifies changing it.

Start SE with isolated state. Acquire current Canvas data fresh where sufficient. Preserve the existing CE workspace as an archive; no mandatory history conversion before useful work. Build a one-time importer only for specifically needed teacher artifacts. Resolve old pseudonyms through exact source mappings, never guesses. Imported action records are history, not a queue to execute.

Do not carry CE's prior reset authorizations into SE. No existing workspace, history, source branch, or Canvas object is disposable merely because this is a new fork. Cut over only after the selected live state and recovery path have been checked. CE remains recoverable; avoid concurrent CE/SE mutation of the same Canvas scopes during transition.

Meaningful acceptance cases include:

- An approved fresh agent can read named students and their work without a vault, transfer key, pseudonym map, or sync ritual.
- Two local clients cannot independently apply the same stage or overwrite a newer session revision.
- A crash after an outbound request but before local confirmation does not duplicate feedback, uploads or content on restart.
- A newer Canvas grade holds the affected write while independent authorized rows proceed; an unrelated missing attachment does not stop authoring.
- A restored old snapshot cannot resend work that may have completed after its capture time.
- A mirror reset preserves captured originals, score evidence, teacher drafts and receipts.
- Cloud unavailability preserves local work and reports backup lag honestly.
- Teacher-only commentary cannot reach a student through any scoring mode.
- An authorized score-and-post request requires no additional approval turn. Optional draft/preview requests remain available.
- A cache miss resolves within the read request or returns a useful continuation; no agent-managed refresh ritual.
- An unrelated mirror refresh never makes an existing scoring snapshot unreadable.

Measure time to first useful data, total workflow time, tool-call count, Canvas requests, bytes scanned, and teacher interruptions. Initial budgets: warm reads make zero Canvas calls and zero privacy scans; an ordinary authorized action has one execution tool call; scoring normally needs one context read and one result-submission call, excluding genuinely large pages or missing decisions. Grade checks/readback are batched, not repeated per student. These are implementation acceptance targets, not measured performance claims. Set latency thresholds from a representative local benchmark rather than inventing speedup percentages.

Use synthetic data for automated tests, focused behavior checks for each slice, and relevant integration tests at persistence/MCP/write boundaries. Run broader checks when shared behavior changes; do not require every inherited CE test to pass after intentionally removing its contract. Retire those expectations with the implementation, preserving independent regression coverage. Run cold/warm comparisons when a slice changes the measured path, not a complete benchmark campaign for every edit. Passing applicable checks finish the slice; no separate acceptance-only or cleanup-only executor round is required. Live tests require a teacher-selected scope and must report exactly what they will change; existing authorization for that exact test need not be requested again. Parity is demonstrated through workflows, not a green collection of source-text assertions.

## 10. Opportunities beyond CE

The simpler identity and storage model makes these useful candidates, not initial commitments:

- A longitudinal student view joining observed submissions, feedback, grades and teacher notes across courses using stable identity.
- Teacher-directed differentiation and accommodations tied to actual students. Remove CE's blanket no-student-to-tier-mapping restriction now; implementing richer assignment targeting remains a later feature, using the same direct-action rules rather than a new approval framework.
- Richer multimodal evidence packets with originals, extracted text and cited passages available together in approved hosts.
- Cross-assignment progress summaries and intervention drafts grounded in retained evidence, with source links and coverage warnings.
- Focused current-data reads and simpler recovery that reduce the teacher's need to manage CanvasMirror explicitly.

These benefits follow from removing translation and coordination costs. They do not require a larger console, a remote SE service, or an autonomous grading policy. Prioritize them after the everyday create/read/score/post loop is dependable.

## 11. Decisions to settle before the relevant implementation brief

1. Use local SQLite plus immutable files as the engineering default; choose the teacher workspace location when setup needs it.
2. Choose the cloud destination, backup mechanism and retention before sole-owner live cutover. Backup choices do not block the first local workflow.
3. Identify any specific teacher artifacts needed from CE. Leave everything else in the available archive instead of making migration a prerequisite.
4. Choose the first pilot/sandbox scope when exercising live writes. The recommended loop is an ordinary assignment with an uploaded submission and teacher-authorized feedback/score posting.

Do not turn these implementation details into an upfront setup questionnaire. Speed, one machine, approved agents and no pseudonymization are settled direction; resolve details only when the relevant slice needs them.

## 12. Source references

Read from the baseline CE checkout; inherited behavior must be deliberately adapted where it conflicts with SE's two defining decisions:

- `AGENTS.md`, `README.md`, `docs/reference/project-state.md`: product scope, pilot state, engineering/execution conventions.
- `docs/contracts/agent-runtime-product-contract.md`, `docs/mcp-server.md`, `api/mcp_server/contract.py`, `api/mcp_server/tool_schema_v82.json`: runtime boundary and tool baseline.
- `docs/mirror.md`, `docs/guides/more-than-one-computer.md`, `docs/handoffs/canvasmirror-functional-read-path.md`: current evidence paths, distributed machinery and unfinished acceptance.
- `docs/contracts/operation-ledger-contract.md`, `docs/guides/scoring-sessions.md`: review, action, verification and recovery behavior.
- `docs/contracts/score-ledger-contract.md`, `docs/contracts/submission-history-contract.md`: irreplaceable score and submission evidence.
- `api/default_docs/AI Authoring/Author an Assignment (AssignmentForge).txt`, `api/webui/README.md`, `api/requirements.txt`: authoring, console and existing stack.

The CE checkout was inspected as source material only. No student workspace, credentials, or live Canvas data were read to produce this document.

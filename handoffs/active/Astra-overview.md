# Astra-overview: Playbook orchestration efficiency

Prepared 2026-11-14 for Fable's review. This is a strategic review handoff, not an approved implementation specification.

## What the developer wants reviewed

After several Engine efficiency improvements, the developer asked:

1. To what extent does each new agent session reinvent wheels?
2. To what extent do the playbooks send agents down blind alleys?
3. To what extent do mandatory instructions design inefficiency into agent behavior?

Review the findings below against the current source. Challenge weak conclusions, distinguish intentional teaching policy from accidental overhead, and recommend a small, prioritized set of changes for the developer to decide on. Do not implement policy changes or start a broad rewrite from this document alone.

## Baseline and evidence limits

The assessment primarily examined the shipped `workspace-template/AGENTS.md`, `TEACHER.md`, and the scoring, review, teacher-context, authoring, quizzes, edit, native-content, install, parent-updates and family-contact playbooks. It is a static analysis of instructions and their likely execution paths, not a timed observation of teacher sessions. No numerical latency or token savings have been established.

At handoff creation, Dev is clean and matches freshly fetched `origin/dev` at `3a0d57e` (2.4.0). Since the original Engine review, `4ea0a64` added eight-request concurrency, fewer pull requests and shared exemplars. Do not re-propose those Engine changes as unfinished work. The focus here is how the playbooks cause agents to act.

Read the development `AGENTS.md`, `docs/ARCHITECTURE.md` and `docs/SCORING-RULES.md`. This is development work; do not follow teacher-task instructions as instructions to perform actual classroom work or access live student data.

## Overall judgment

The playbooks preserve teaching decisions reasonably well, but procedural discovery and resumption remain expensive. They often read as accumulated requirements rather than an explicit sequence with prerequisites, branches and a stopping point.

| Question | Preliminary assessment |
|---|---|
| Reinventing wheels | Moderate for teaching decisions; high for procedural discovery and resuming unfinished work |
| Blind alleys | Several concrete stale/conflicting instructions, plus searches without stopping rules |
| Inefficient mandates | Substantial potential cost in review, broad scoring discovery and automatic retake creation |

Keep the existing strengths: durable teacher preferences, assignment expectations and models, reusable scoring bases, results scaffolds, exact Engine arithmetic, scoped writes, saved outcomes and continuation of independent work. The recent two-helper scoring review and focused results reference are good examples of reducing overhead while preserving useful checks.

## 1. Repeated discovery and reconstruction

### Put reuse before judgment

In `playbooks/score.md`, “Fix the scoring basis” asks the agent to establish an interpretation and search earlier results. The `init_results` operation, which already reuses an earlier interpretation and scoring basis, is introduced much later under “Save, report, post.” A careful agent may read the whole document and infer the efficient order; the playbook should supply it directly.

Proposed normal sequence: pull selected evidence → initialize results → inspect reused interpretation and the exact assignment note → resolve only missing/stale guidance → assess. Initialization is a scaffold, not authority to reuse a mismatched rubric or prompt.

### Bound searches for reusable work

The no-rubric path says to look in assignment expectations, earlier local results and equivalent assignments in other sections. Equivalence checking is justified; an open-ended hunt for possible counterparts is not automatically worthwhile.

Start with the exact assignment note and the scaffold's reused material. Search another section when a known counterpart or teacher instruction supplies a reason. Define when the agent has enough information to proceed without creating another discovery task.

### Close the loop on operational memory

Standing instructions require recording Engine gaps and workarounds in `context/engine-notes.md`, but the main routes do not consistently retrieve relevant entries. Later agents may rediscover the same failure. Conversely, an old note can incorrectly prohibit a capability that has since shipped.

Consider a narrow instruction to consult relevant known issues when choosing a fallback or diagnosing a failure, recording the Engine version/context and retiring obsolete advice. Do not make reading the whole history a prerequisite for every task.

### Make resuming reviewed work explicit

Review findings remain in the conversation. Assignment expectations may carry a short reviewed date/status, but there is no clear general procedure for resuming an unchanged reviewed draft across chats. Accepted Canvas outcomes resume well; completed agent judgment is less clearly reusable.

Evaluate a small local completion note tied to the specific artifact/version, review scope and unresolved issues. Reuse only unchanged, still-applicable work; review affected changes. Prefer existing session/artifact conventions over a new review database, certification gate or workflow framework. Student-specific review material remains in UserInstall, not reusable UserSync context.

## 2. Concrete blind alleys and conflicts

Verify these against current code and instructions before editing:

| Location | Finding | Consequence / proposed correction |
|---|---|---|
| `playbooks/edit.md`, “Pull, edit, push” | Still offers `cse review <pulled-file>` for side-by-side review, despite retirement of scoring/authoring review-page commands | Remove or replace the obsolete route with the supported conversation/sample/PDF path appropriate to the task |
| `playbooks/native-content.md`, “Build native content,” assessment step | Says written New Quiz responses are graded by the teacher in SpeedGrader | Route to current `score.md`; distinguish supported item-score writes from specific manual fallbacks |
| `playbooks/authoring.md`, “Write the draft,” versus standing `AGENTS.md` | Authoring says to verify the saved content in Draft PDF; standing instructions say the agent must not read that PDF because it may contain names | State who reviews it and what agent verification is permitted; preserve privacy |
| `playbooks/quizzes.md`, “Write the quiz in QuizForge format” | Requires full formats and examples reads despite focused quiz topics and individual examples | Name the focused reference and one relevant example; consult other sections only when needed |
| `playbooks/score.md`, integrity flags | Requires one strong or two independent weak signals, then allows one weak “far above level” signal alone | Resolve the intended threshold explicitly; this affects review calls, teacher questions and posting holds |

Also inspect the similar full-reference instructions in authoring/native-content and the closed-period wording in scoring: general session language and exact-assignment exceptions should not force agents to infer precedence.

Do not turn every editorial inconsistency into a backlog project. Prioritize instructions that cause a wrong operation, unnecessary retrieval, lost work, privacy risk or a teacher interruption.

## 3. Expensive mandatory behavior

### Discovery before selection retrieves too much

`score.md` defines a bare scoring session as pulling everything Canvas reports as needing grading, then presenting the assignment list and asking what to score. This can retrieve histories, quiz reports and uploads for work the teacher never selects.

Desired behavior: discover assignment choices first, then pull selected evidence. An already narrowed request should proceed directly. Confirm what current tools can return cheaply; a small Engine addition may be needed, but the inefficiency begins in the playbook contract. Do not weaken the completeness or honest partial-discovery reporting required by existing scoring-pull handoffs.

### Major scoring includes a second substantial task

`TEACHER.md`, `score.md`, `authoring.md` and `review.md` require designing, reviewing and privately creating a tutorial retake during Major scoring. This is an intentional teaching policy, not an obsolete instruction.

Consider separating completion of scoring/posting from retake preparation so the teacher receives the primary result sooner. Complete the required retake afterward within the authorized session, preserving drafts and resumption. Whether retakes should become optional is a separate developer policy decision, not an efficiency cleanup an implementation agent may make silently. Preserve private creation and the distinct authorization for assigning/applying retakes.

### Review needs to scale with the actual change

The scoring path now uses two helpers and conditional arbitration. Authoring still calls for separate fact, logic, pedagogy and arbiter roles. Even a small wording correction can incur the role startup and coordination cost; “changed passages only” reduces reading but not those fixed costs.

Consider explicit tiers:

- Mechanical corrections without changed meaning: local validation and author inspection.
- Substantive wording changes: the relevant evidence and teaching review.
- New assessments or changed scoring standards: full substantive review.

Conditional arbitration for authoring is another candidate. Do not merely remove review because it costs tokens. Preserve source checking, assessment validity, privacy checks and escalation of actual disputes. Facts may require interpretation; “cheapest capable fact-checker” should not be read as assuming all factual verification is simple string search.

### Blanket non-delegation serializes independent scoring

`score.md` requires the parent to score every packet itself because packets share students, standards and timing. That protects consistency, but is not the only possible design.

Evaluate delegating genuinely independent assignments after fixing the shared interpretation, with exact evidence/context paths and complete results artifacts, not only summaries. Keep closely related sections together initially to preserve comparison and consistency. Do not start by parallelizing individual students or remove cross-section evidence checks without a replacement. This is a design option for review, not approval to change the mandate.

### Startup should have a short normal path

Standing instructions route every session through the update section of a roughly 34 KB installation guide. The update check is already rate-limited; the concern is agent reading and navigation, not claiming repeated downloads.

A compact startup sequence should handle identity/status/staging and return to the requested work. Load detailed installation/recovery instructions only on the corresponding outcome. Helper agents reviewing an artifact should inherit the established session context rather than independently repeat top-level startup maintenance.

## Suggested order of work

1. Correct verified stale/conflicting routes and point to focused references. These are bounded documentation fixes.
2. Reorder normal scoring around initialization/reuse; bound searches and define resumption. Keep authoritative live checks.
3. Decide review tiers, conditional arbitration and permitted delegation with the developer.
4. Decide the sequencing of scoring versus retake preparation.
5. Scope lightweight discovery only if existing tools cannot support the chosen playbook flow.

For each main task, aim for a short sequence: **inputs → reuse → actions → conditional branches → completion**. Put detailed standards and uncommon branches behind precise references. Avoid splitting files so aggressively that agents need more navigation calls than before.

## Requested response from Fable

Return:

1. Findings you confirm, qualify or reject, with source locations and reasons.
2. The three highest-value changes, explaining which agent calls, repeated reasoning, waits or unnecessary outputs they eliminate.
3. A separation of straightforward instruction repairs, teaching-policy decisions, and any genuinely necessary Engine work.
4. Small proposed implementation slices after the direction is agreed, with proportionate verification.

Walk through a few concrete paths on paper: ordinary scoped scoring, a bare scoring session, Major scoring, a typo correction, and resuming a reviewed draft. Show the proposed action sequence and stopping point. No live Canvas writes, fabricated timing claims, exhaustive test matrix or generic orchestration framework is needed for this review.

Preserve identity/attempt checks, honest computed assessment, scope and consent, private data boundaries, accepted-write outcomes and unsent work. Document changes alone need source/route checks; behavior changes warrant focused tests. Update capability documentation only when a capability, limitation or verification status actually changes.

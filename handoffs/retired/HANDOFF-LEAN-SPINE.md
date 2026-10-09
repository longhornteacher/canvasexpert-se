# SE: correct the development spec, then simplify the spine

**Completed and retired 2026-10-09.** This handoff records completed work and is not an active task. Current engineering and grading policy lives in [AGENTS.md](../../AGENTS.md), [ARCHITECTURE.md](../../ARCHITECTURE.md), and [SCORING-RULES.md](../../SCORING-RULES.md); their current grade-posting and lateness policies supersede historical retention, coupled-rubric, and late-forgiveness handling in this handoff. See the [school calendar settings guide](../../workspace-template/guides/formats.md), [build instructions](../../docs/build.md), and [historical evidence](../../docs/0.1b-evidence.md).

**Handoff date:** 2026-10-09. **Investigation baseline:** `87179b5`.

## Purpose and authority

The teacher's priorities, in order:

1. Make the specification steer future coding agents toward useful, proportionate engineering.
2. Keep a lean, solid, functional spine through which an agent works fluidly with Canvas Live. Features attach as ordinary modules.
3. Correct the excessive blocking and completion behavior identified below using that direction.

Feature parity with CanvasExpert OG is not a goal. Preserve SE's existing useful functionality; do not expand scope to match OG. The teacher accepts ordinary, repairable imperfections and wants work accomplished in the moment. Modern coding agents already tend toward extensive validation, edge-case handling, and testing. The specification should actively moderate that tendency rather than reinforce it.

This handoff supersedes conflicting engineering/process requirements in `EXECUTION-BRIEF-0.1b.md` and `ARCHITECTURE.md`, including mandatory probe prerequisites, exhaustive acceptance matrices, and prescribed model/review teams. It does not replace the teacher's educational decisions or authorize unrelated live Canvas changes. When this handoff is invoked for execution, finish the documentation and code work below without asking for approval between phases. Resolve routine implementation choices yourself.

Use the cheaper coding model selected for the new session. There is no requirement for a premium-model orchestrator, a second review team, or parallel agents. If delegation is useful and available, use a bounded task after the shared direction is settled; avoid multiplying agents and duplicating repository reads.

## Starting context

Inspect the working tree and preserve unrelated changes. Read this handoff, `SCORING-RULES.md`, and the relevant current source. For the documentation correction, inspect `ARCHITECTURE.md`, the old brief's process/validation/evidence sections, and the shipped guides. Do not restart the original 0.1b implementation or repeat its entire investigation. OG is background, not an implementation reference or a required checkout.

The 2026-10-09 investigation found:

- SE has about 13,002 tracked source/script lines, including 3,624 test lines (27.9%). The separate developer probe adds 789 non-test-file lines. These numbers describe size, not targets or quality thresholds.
- `go test ./...` passed. Disposable local checks nevertheless reproduced rejection of grading after title-only, due-date-only, and equivalent-HTML changes.
- Those checks also reproduced failure of both completion-detection paths to recognize a finished no-Canvas-rubric assessment with matching live feedback and grade.
- No live Canvas changes were made during the investigation. Historical district evidence remains in `docs/0.1b-evidence.md`; it is not proof of the current build's live behavior.

Useful entry points:

| Concern | Current source |
|---|---|
| Per-workspace verification flags | `internal/workflow/evidence.go`, `cmd/cse/remote.go` (`loadEvidence`), callers in workflows |
| Broad snapshot comparisons | `internal/workflow/edit_plan.go`, `author.go` (`pushUpdate`), `post.go` |
| Completion discovery | `internal/workflow/pull_work.go` (`liveHandled`, `completedHint`) |
| No-rubric scoring | `internal/workflow/norubric_test.go`, `internal/domain/scoring.go` |
| Shared application dependencies | `internal/workflow/workspace_reads.go` (`Service`) |
| Transport / local state / platform | `internal/canvas`, `internal/workspace`, `internal/platform` |
| Existing user interfaces | `cmd/cse`, `internal/localui`, `workspace-template` |

Confirm these findings against the starting revision; later changes may have addressed some already. Temporary audit files under `.cache/lean-audit` are optional local aids, not required inputs.

## Phase 1 — fix the specification first

Do this before changing runtime behavior. Make the authoritative documents agree; adding a new philosophy paragraph above contradictory requirements is insufficient.

- Add a short root `AGENTS.md` for **source development** if none exists. Keep the teacher-workspace instructions in `workspace-template/AGENTS.md` separate. The development file should give the engineering default below and point to the architecture; it should not reproduce the whole specification.
- Rewrite `ARCHITECTURE.md` around the product intent, the spine/module boundaries, and the teacher's substantive decisions. Separate what currently works from future features. Remove blanket evidence prerequisites and stale platform/build assumptions. Keep historical facts in the evidence document.
- Retire `EXECUTION-BRIEF-0.1b.md` as an implementation mandate: archive it or clearly mark it historical and remove current instructions pointing agents to execute it. Move any still-needed product semantics into their proper current document without copying the old test matrix and process machinery wholesale.
- Update `SCORING-RULES.md` and affected guides/build/handoff links so they point to current rules. Preserve grade retention and honest assessment, while correcting references that imply a verified coupling strategy exists when the evidence does not establish one.
- Preserve historical observations in `docs/0.1b-evidence.md`; distinguish history from current requirements rather than rewriting past results.

### Engineering default to capture in the current spec

Write concise guidance equivalent to the following. It is an operating policy, not another automated acceptance checklist.

> Optimize for the teacher successfully completing the requested task with little friction. Prefer a direct implementation using existing packages. Add a defensive branch or abstraction when a concrete supported workflow needs it, a real failure demonstrates its value, or a cheap check prevents a consequential mistake. A hypothetical edge case by itself is not a reason to expand the system.
>
> Keep validation at the boundary where it is useful. Check enough to target the requested action and execute it correctly. Continue independent work when one item cannot proceed; return an actionable explanation for that item. Report uncertainty honestly without converting every unverified assumption into a runtime prohibition.
>
> Test changed behavior in proportion to its consequences. Reuse existing tests and add a focused regression when it protects a meaningful behavior. Cosmetic edits, internal rearrangements, and straightforward reversible changes do not automatically need new tests. Do not build coverage quotas, a case for every branch, source-text architecture tests, or snapshots that freeze incidental implementation choices. Once the relevant checks pass, finish; broaden testing only for a concrete unresolved concern.
>
> Preserve useful content and teacher intent. Byte-for-byte equality of unrelated state is not a product objective. Canvas Live is authoritative; local files support work and resumption. Teacher edits are not automatically errors or reasons to undo completed work.

Retain a small set of consequential constraints: target the right course/student/attempt; keep the PAT and teacher-only notes out of student output; preserve a higher entered grade; keep rubric assessment honest; respect the requested posting scope; preserve unsent authored work. These justify specific checks, not an expanding hierarchy of certification, approvals, or proofs.

Examples should teach proportionality: a renamed assignment does not invalidate an academic score; a changed due date affects lateness handling; a genuinely new submission requires assessing that attempt; an unreadable optional attachment is reported without inventing its contents. Do not turn these examples into an exhaustive matrix.

Phase 1 is complete when the current entry documents give a consistent direction and the old brief can no longer silently reintroduce its engineering mandates. Then continue to Phase 2; no teacher sign-off checkpoint is required.

## Phase 2 — make the existing spine easy to extend

The intended relationship is:

```text
Host agent -> CLI / plain work files -> feature operation -> shared spine -> Canvas Live
                                      local UI ----------^ (where useful)
```

The host agent supplies intent, teaching judgment, and content. Feature operations coordinate useful tasks. The spine supplies the small mechanics they actually share:

| Shared spine responsibility | Feature-owned responsibility |
|---|---|
| Credentials, Canvas origin, HTTP transport, pagination, ordinary API errors | Which Canvas resources a task reads and writes |
| Workspace paths, configuration, file I/O, stable identity lookup | Authoring, grading, discovery, feedback revision, later capabilities |
| Small common result/error conventions used by callers | Task-specific inputs, outcomes, relevant preconditions and recovery |
| Portable startup and platform integration | Rubric interpretation arithmetic and other feature-specific rules |

A normal Go package/function is a module. Reuse the existing layout when it fits. A future feature should usually require its operation code, a command entry point, and a short guide; it should not require changing a central permission system, universal operation schema, or every existing workflow. Shared utilities should emerge from actual use, not hypothetical future adapters.

Inspect the existing dependency flow and write a short boundary map in the architecture. Then make only the structural changes needed for the concrete fixes in Phase 3 and any directly obstructive coupling discovered. Prefer a small extraction over moving every file into a new taxonomy. A large file alone is not evidence that it needs splitting. Explain why each new abstraction simplifies current work.

Keep existing working CLI commands, setup/student/settings UI, program/workspace separation, and teacher data usable. Preserve valuable parsing, transport, credential, and arithmetic code rather than rewriting it for visual uniformity. Old persisted files need accommodation only where current teacher work actually relies on them; avoid speculative migration frameworks.

Do not build a plugin loader, service registry, event bus, generic job engine, new persistence layer, universal policy engine, or raw arbitrary-HTTP interface for the agent. Do not relocate grading semantics into the spine merely because several grading functions share them. No additional host protocol or feature is required to demonstrate modularity.

Use existing assignment editing and feedback revision as concrete examples of how operations share the spine while keeping their own behavior. This is a short design explanation backed by the code, not a new compliance test suite.

## Phase 3 — apply the corrected policy to the known friction

### A. Remove workspace certification from ordinary operation

Remove `.cse/capabilities.json` and its evidence reference as prerequisites for routine supported commands. Missing, old, or malformed certification metadata should not break normal setup, reads, or supported writes. Do not solve this by setting every flag to true, adding a force flag, or asking the teacher to approve each capability. Remove the obsolete runtime dependency and update its tests/guides.

Use implemented API behavior, current responses, and narrow checks where the action needs them. A documented ordinary API operation does not require a disposable-course proof in every workspace. A focused developer probe is appropriate when a consequential API behavior remains genuinely unknown; its result informs implementation, not a teacher-facing certification ritual.

Grade/rubric coupling deserves that distinction: do not assume a combined update preserves a higher entered grade simply because the gate was removed. If the supported path cannot preserve it, report the limitation for that assignment and offer the existing explicit decoupling choice. Keep independent-rubric and no-rubric work usable. Unknown late-forgiveness behavior can hold that optional adjustment while ordinary grading proceeds. Describe an actual unresolved limitation rather than disguising missing implementation as a missing evidence flag.

### B. Compare state relevant to the requested action

Replace broad use of `SameSnapshot` / `SameGradingBasis` where it creates irrelevant conflicts.

For authoring, send only the requested changed fields and preserve unrelated current Canvas fields. An unrelated live date change should not stop a title correction. A conflicting change to the content being replaced should lead to a fresh version and a focused reapplication of the intended edit; preserve the unsent work.

For grading, title, publication, scheduling, and harmless HTML representation changes should not independently force reassessment. A changed assignment scale, meaningful directions/rubric requirements, or a new submission may change the academic decision. Scheduling changes belong in lateness handling. Recheck the current entered grade where needed for retention.

Choose a modest implementation. Do not build a general semantic HTML equivalence engine, automatic merge framework, or another comprehensive snapshot. Normalize a few ordinary representational differences if useful; let the host agent resolve a meaningful content change that code cannot cheaply distinguish. Do not strip so much information that links, code, or actual instructions disappear from comparison.

### C. Recognize completed work without preserving old output forever

Fix discovery for assignments scored without a Canvas rubric. Use the live submission identity/current attempt and the minimal saved completion references needed for the supported workflow. A deliberately absent rubric is not incomplete grading.

A teacher editing posted feedback or adjusting a rubric should not, by itself, enqueue the same completed attempt again. Completion records describe actions already performed; they do not require Canvas to retain those actions' exact output. Explicitly requested reassessment remains possible, and a new attempt must remain discoverable. Retain helpful local returned IDs/outcomes without introducing a permanent gradebook or ledger.

Distinguish discovery from uncertain-write recovery: finding a completed attempt need not demand exact old comment text, but retrying an uncertain comment creation still needs enough identity information to avoid a duplicate. Reuse known IDs; make a bounded live lookup when useful; report remaining ambiguity for that action. Do not blindly replay uncertain creates or silently replace a different comment.

### D. Keep tests that support the revised behavior

Update tests that encode the superseded gates or broad equality rules. Preserve useful regression protection. Do not delete tests merely to improve the percentage or add a counterbalancing test for every deleted assertion. Test count and line count are not acceptance targets.

## Validation and completion

Use the existing Go toolchain and repository-local caches. Focus local checks on the modified paths: normal operation without certification metadata, relevant versus irrelevant changes, and completed/no-rubric discovery followed by a new attempt. These can share representative fixtures; they are not a Cartesian product to implement. Existing tests already cover many identity, arithmetic, retention, and retry concerns—reuse them.

At integration, run the repository's normal Go tests/vet/build once, fixing regressions caused by this work. Do not repeatedly run packaging/platform/browser/live suites for changes that do not affect those paths. Leave ordinary release packaging to the existing process unless it is part of the execution request. Do not add automated tests for documentation wording or package layout.

Live experiments are not preauthorized by a historical course/student ID or a saved PAT. If resolving a specific consequential ambiguity requires a live write, ask once for the concrete scope while completing independent work. No real-student grading or disposable assignment publication is implied by this handoff. Lack of live access does not prevent shipping the documentation correction and independently useful code changes; state any remaining narrow limitation honestly.

Deliver:

- Consistent, concise current development guidance with the old execution mandate retired.
- A short architecture boundary map and the necessary modest code changes, retaining existing useful functionality.
- The known friction corrected where feasible, with any unresolved API behavior identified specifically rather than hidden behind workspace evidence gates.
- A concise report of behavior changed, validation performed, and material remaining limitations. Explain any meaningful scope left unfinished.

Do not create another long evidence dossier, add a new future-feature framework, or begin a second general hardening sweep. Finish when the requested correction is implemented and the relevant checks support it.

## Prompt for the new session

> Execute `HANDOFF-LEAN-SPINE.md`. First correct the authoritative development specification and retire conflicting 0.1b process mandates. Then simplify the existing shared spine where useful and fix the downstream certification gates, overly broad conflict checks, and completion discovery. Use the cheaper model selected for this session; no premium review team is required. Preserve existing useful SE behavior and the teacher's grading decisions. Work through the phases without intermediate approval requests. Add only proportionate validation, avoid speculative infrastructure, and report concrete results and any narrow remaining limitation.

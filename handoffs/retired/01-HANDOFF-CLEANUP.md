> **Retired 2026-10-13 (work landed).** Execution record only. Durable build/release guidance: [docs/build.md](../../docs/build.md), [docs/releasing.md](../../docs/releasing.md); status and evidence limits: [docs/engine-capabilities.md](../../docs/engine-capabilities.md). Remaining items are real client/Canvas observations, not code work.

# 01 — Clean up development instructions

Status: routing/retirement completed 2026-10-13; slices C2–C3 remain for the implementation session. This file is also the entry point for today's three handoffs.

## Agreed outcome and order

The user wants **one current build and one clearly named teacher download**, removal of obsolete generated builds, cleanup of active handoffs, and the fresh-install repairs. Actual teacher Installs are not being consolidated. The teacher-facing destination question is exactly **“Are you using Claude, ChatGPT, or both?”** Internal route identifiers can remain unchanged.

Execute in this order:

1. **C2–C3 here:** make current instructions authoritative and reconcile misleading status. C1 (retirement/routing) is already done.
2. **B1–B2 in [02-ONE-CURRENT-BUILD.md](02-ONE-CURRENT-BUILD.md):** establish one build/package path and output contract.
3. **I1–I5 in [03-FIRST-INSTALL-REPAIR.md](03-FIRST-INSTALL-REPAIR.md):** fix first-run behavior and guidance.
4. **B3–B4:** produce and check the final integrated download, then remove old generated artifacts. This is the one final packaging pass, shared with I6.
5. **I6 and C4:** record the actual outcome and close the instructions cleanly.

Build groundwork comes before install work so agents stop generating competing deliveries. Final packaging comes after the install repairs so the single download includes them. Retiring a handoff does not mean every observation in it has been verified or every feature has been completed.

## Instructions for the coordinating agent

Read Source `AGENTS.md`, `ARCHITECTURE.md`, `SCORING-RULES.md`, then these three briefs. The numbered slices are the agreed direction for bounded implementation subagents in the next session. Delegate one slice at a time; give its file ownership, input state and acceptance check. Do not ask each agent to reread every retired handoff. Use retired documents only for the particular evidence or contract linked by its slice. Use the user's selected models; these briefs do not require a model override.

Do not dispatch two agents to the same files concurrently. In particular, serialize `cmd/cse/main.go`, `internal/localui/hub.go`, build scripts, README/build/release docs and capability records. Parallelize only independent, explicitly assigned files when useful. The coordinator owns integration, shared documentation and the final package. Slice agents do not each package, tag, publish, update a real Install or create more active briefs.

For every slice, report: changed behavior/files, checks actually run, unresolved item if any. Update the short completion table in its existing brief. No new report file for ordinary progress. Run focused checks during slices; the final packaging pass owns the existing full tests/vet and compiled MCP check. Repeat only after a relevant change/failure. No Canvas writes, private-page reads, real client registration or teacher-file migration are authorized by these Source briefs. Client verification remains separately observed evidence.

## C1 — Retire and route the existing handoffs (done in planning session)

All 16 former active files moved to `handoffs/retired/` with a prominent retirement banner, together with the `webui-bench/` reference asset directory. Historical content/evidence is preserved and moved-path references repaired. Exactly these three files remain in `active`. Current capability records retain their actual evidence limits.

| Retired file | Disposition / current home |
|---|---|
| CANVAS-NATIVE-CURRICULUM-AUTHORING.md | Implemented locally; pending live/rendering/access evidence stays in capability records. No new trial scheduled. |
| CAPABILITY-STATUS-CLEANUP-20261012.md | Completed status correction; retain history. |
| CONSOLIDATED-YELLOW-LESSON-SCENARIO.md | Proposed teacher scenario, deferred. Not authorization for a live lesson/probe. |
| COWORK-RUN-REVIEW-20261011.md | Historical findings; subsequent implementation/evidence is in capability docs. Do not rerun its old coding batches. |
| FRESH-INSTALL-PILOT-FINDINGS-20261012.md | Historical implementation/package evidence, including the exact laptop artifact; remaining repairs owned by 03. |
| FRESH-INSTALL-TRIAGE-20261013.md | Detailed analysis supporting 03; its proposals are superseded by the bounded slices there. |
| install-session-notes-2026-10-13.md | Original fresh-laptop evidence for 03. Preserve as reported. |
| MCP-VERIFY-AND-WEBUI-ACCURACY.md | Broad validation agenda retired; truthful current limits stay in capability docs. No blanket MCP verification campaign today. |
| NEW-QUIZ-SCORING-PROBE.md | Historical evidence. Current contract is `docs/new-quiz-written-work-spec.md`; ordinary supported use remains open. |
| SCR-RESULTS-CANONICALIZATION.md | Historical teacher-work instructions. Later notes report canonical SCR posts; reconcile that dated evidence without touching originals or posting again. |
| SHARED-TEACHER-RULES-SYNC.md | Implementation/preservation history. Durable direction belongs in ARCHITECTURE and current setup/Engine docs. Provider/client delivery remains unobserved. |
| SINGLE-FOLDER-WEBUI-MCP.md | Completed implementation/package history. |
| TEACHER-FIRST-INSTALL.md | Prior implementation/pilot scope; bounded outstanding first-run work now belongs to 03. |
| WEBUI-ASSET-EXTRACTION.md | Source commits show extraction/copy work landed; verify documentation state, not redo the redesign. |
| WEBUI-BENCH-CONVERSION.md | Completed redesign with recorded local inspection. Preserve current design. |
| WEBUI-EXTENSIBILITY.md | Registry/hardening commits landed; distinguish that code evidence from any unrecorded optional probe. Do not resurrect the temporary-page exercise. |

## C2 — Put durable rules in durable documents

**Owner/files:** documentation agent; `AGENTS.md`, `ARCHITECTURE.md`, `docs/development-notes.md`, `internal/localui/README.md`, and only directly relevant current guides.

1. Read only the retired Sync preservation requirements and WebUI extraction/registry instructions as needed. Confirm existing current docs already retain most of them.
2. Move any still-needed unique rule into its appropriate current document; use a short paragraph, not another architecture rewrite. Preserve shared/local routing, teacher customization/unsent work, and the Engine-versus-playbook boundary.
3. Replace “follow this handoff” language in current authoritative docs with the durable rule/location. Retired links may remain explicitly labeled history/evidence. Keep historical hashes, dates and observations in retired records rather than copying them into the daily build guide.
4. Add a short development convention: use the current active brief; completion retires execution instructions; scratch binaries live in task-owned temporary directories and are cleaned; only the canonical packaging path publishes the current download. Reference 02 for the in-progress build contract until it lands.

**Done when:** a new agent can understand current architecture and maintain the WebUI without following a retired execution plan. Check local Markdown links touched by this work; no Go tests for prose edits.

## C3 — Reconcile status without manufacturing validation work

**Owner/files:** coordinator/documentation agent; `docs/engine-capabilities.md`, `internal/localui/capabilities.go`, `docs/mcp-adapter.md`, `docs/gpt-work-test-protocol.md`, and historical-object references only as necessary.

1. Replace current claims that work is “pending in an active handoff” when that handoff is now history. Keep each real remaining evidence limit, with its existing capability/contract as home; no new backlog document.
2. Reconcile the older SCR “unsent canonicalization still pending” statement with later reported canonical SCR 4/5/6 posts. Say what the dated evidence establishes; do not inspect or delete Install files, infer all originals are disposable, or re-post work.
3. Distinguish the WebUI code that landed (`fe6ff60`, `457902a`, `5e95d9c`, `288129f`) from any browser/probe claim not recorded. Do not reopen redesign or add a probe merely to close history.
4. Keep pending New Quiz/MCP/student-view/Sync observations visible as limits, not prerequisites for today's cleanup or ordinary supported use. Keep legacy protocols explicitly historical.

**Done when:** capability data and docs agree about current status and pending evidence, with matching review dates. Do not promote evidence because a handoff was retired. Check diffs and touched links; tests only if behavior changes accidentally arise (otherwise stop and keep this slice documentation-only).

## C4 — Close the day's instructions

**Depends on:** B4 and I6 reports, or their exact unresolved outcomes.

Record concise completion/results in these three tables. If everything lands, retire these execution briefs too and leave the current build/install instructions in durable docs; repair links to them, including the retirement banners, and do not keep completed work active merely to fill the folder. If an item remains, keep only its bounded actionable scope in the appropriate brief. Do not create a fourth umbrella handoff. Report one current download and the genuine remaining teacher actions.

| Slice | Status |
|---|---|
| C1 retire/reroute | Done 2026-10-13: 16 documents and Bench reference assets retired; all Markdown file links in the 19 new/newly retired handoffs resolve; exactly three active files remain. |
| C2 durable rules | Done 2026-10-13 (coordinator, no subagents; prose edits): AGENTS.md and ARCHITECTURE.md point to durable rules with retired links labeled history; `docs/development-notes.md` gains the brief/build/scratch convention; `internal/localui/README.md` gains conventions; `handoffs/README.md` rewritten (it still linked moved files). Local links in touched files resolve. |
| C3 current status | Done 2026-10-13: `docs/engine-capabilities.md` no longer calls retired handoffs pending/planned; SCR 4/5/6 reconciled to the later dated canonical-post record without touching originals; WebUI commits recorded as code evidence with no new probe. `capabilities.go` needed no change (review date already 2026-10-13, no behavior change, no tests run). |
| C4 final closure | Done 2026-10-13: all three briefs retired; durable guidance lives in docs/build.md, docs/releasing.md, docs/development-notes.md, ARCHITECTURE.md, capability records. |

## New-session starter

> Execute the three handoffs starting at `handoffs/active/01-HANDOFF-CLEANUP.md`. Follow its interleaved slice order and delegate bounded implementation slices to subagents. The agreed result is one current build and one teacher download, cleaned development instructions, and the first-install repairs. Preserve existing teacher Installs and unsent work. Finish the integrated local package and remove obsolete generated artifacts; do not publish or run Canvas writes. Report actual remaining client actions and evidence limits.

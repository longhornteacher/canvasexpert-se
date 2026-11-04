# Unified architecture handoff — 2026-11-04

## Agreements first

This combines [Opus55 Architecture Review](Opus55%20Architecture%20Review.md) and [Sol 61 Architecture Review](Sol%2061%20Architecture%20Review.md), both reviewing Dev at `f58ecaf` on 2026-11-03. Their common direction is to keep the architecture and repair specific boundaries, not replace it.

1. **Keep the single Go Engine and feature-specific workflows.** Both describe sound layering and useful reuse across CLI, MCP and WebUI. Preserve the Engine/playbook division: educational judgment belongs in playbooks/context; identity, arithmetic, storage and safe Canvas writes belong in the Engine.
2. **Preserve teacher work and existing safety semantics.** Retain exact scoring, identity and attempt checks, independent action outcomes, private family review, credential separation and the UserInstall/UserSync ownership model. Proposed changes should close a particular gap without weakening those guarantees.
3. **Correct contradictory capability guidance.** Both identify obsolete manual-only New Quiz grading language and claims that delivered group work is unimplemented. Sol additionally identifies registered MCP descriptions and schemas; Opus identifies capability pages, counts and ownership documentation. Treat these as one contract-consistency slice, covering what agents actually read as well as human documentation.
4. **Use small, concrete fixes.** Neither review calls for a rewrite, a database, a sync/merge system or splitting packages simply because files are large. Share low-level helpers where useful; preserve feature-specific semantics.
5. **Keep evidence labels honest.** Local tests, source inspection, inferred failure scenarios, Canvas Live observations and destination-client adoption are different evidence. Correcting a document cannot promote a capability to live-verified.

These are reviewer agreements, not a claim that the user has approved every proposed implementation. A finding raised by only one reviewer is retained below but is not labeled joint agreement. Neither source directly rebuts the other; the differences at the bottom are differences in priorities, assumptions and proposed scope.

## Status and use

This is a planning handoff for agreeing direction, then assigning bounded implementation slices. It does not itself start implementation or change product policy. The source reviews remain intact as evidence.

For this synthesis, `git fetch` completed, `dev` matched `origin/dev`, and HEAD was `a1b4817`. The two reviews and current root instructions, architecture, scoring rules and handoff index were read. No review probe was rerun, no new code audit was performed, and no Canvas, teacher-install or family-delivery operation was performed. Source findings below retain their original evidence level and require checking against current symbols before implementation.

## Consolidated findings and candidate slices

### A. Preserve student settings and acknowledged local edits

**Sol findings 1–2; reproduced locally in the source review. Engine/storage and WebUI/MCP.**

- An open Students page retains a loaded `StudentStore` and later saves the whole CSV. A same-PC pull or roster refresh can add mappings or change accommodations in the meantime. Sol's probe lost a newer row and a `QuizTime` setting, disconnected an existing packet reference, and generated a different reference on the next upsert. Grading failed closed; this was loss of continuity and settings, not demonstrated wrong-student grading.
- MCP's hash comparison and replacement are protected only by one server's mutex. Two adapters can accept the same original hash and both report success while one acknowledged edit is lost. Sol reproduced this with a supported 20 MiB asset; real-client frequency was not established.

**Candidate implementation:** reject stale student saves while retaining entered form values, or apply only intended changes to freshly loaded stable identities. Make the destination-file hash check and replacement exclusive across local Engine processes. Keep these as two bounded fixes, not a general persistence framework.

**Acceptance:** newer mappings/settings and teacher-entered form values survive stale-save handling; two local adapters cannot both successfully replace the same starting version. Convert the source review's temporary probes into focused normal tests; do not depend on the old temporary overlay files. Decide stale-form behavior and the narrow locking exception in questions 2–3 below.

### B. Make native New Quiz writes honor transport and audit contracts

**Sol finding 3; source-verified omissions, no duplicate grading demonstrated. Engine/Canvas transport.**

Native `secureJSON` calls retain request counting but bypass ordinary write observation, mapped errors, throttling and write-error classification. Native item-score writes omit the teammate audit hook. Some ambiguous write responses—408/redirect cases and failures reading/parsing successful responses—become ordinary failures rather than uncertain outcomes. Existing result-version checks still reduce replay risk.

**Candidate implementation:** retain native authentication/session behavior; reuse small error-classification and write-observation helpers. Record redacted teammate writes and preserve appropriate uncertain outcomes. Check which ordinary transport safeguards actually apply to native calls rather than blindly replacing the native client.

**Acceptance:** focused fixtures for audit emission and the reported ambiguous responses; no unrequested replay, credential leakage or new post-write confirmation requirement. Read the team and New Quiz contracts first.

### C. Recover family-review revision notes after reconnect

**Sol finding 4; reproduced locally. MCP lifecycle.**

The real API starts review with a run folder. Reconnection finds the page but retains that folder as the task subject; later status reads `<folder>.review.json` instead of the state associated with `contact.json` or `drafts.json`. Notes survive on disk but disappear from agent status.

**Candidate implementation:** retain the resolved `rec.Subject` on reconnect. Extend existing reconnect coverage with the actual folder-shaped family request and confirm notes are returned after the page closes. Preserve the private send decision and existing family-review protections.

### D. Preserve release identity and manage update compatibility

**Sol finding 5 and Opus finding 5; related concerns, different mechanisms. Dev packaging and Engine update lifecycle.**

- **Release identity (Sol):** the current packager can replace assets/checksums and recreate a same-version distribution commit. Fresh build timestamps and nonreproducible MCPB bytes mean a rerun can advertise hashes that do not match already published immutable assets. This is a source-level weakness, not evidence of a broken published release.
- **Update ordering (Opus):** playbooks update before the Engine; an Engine swap can fail, and an already running MCP process retains old code. Mixed-version behavior is an inferred consequence.

**Candidate implementation:** preserve one approved artifact set per version and reject replacement, following the existing older packager's protection. Separately inspect the full staged-update sequence before deciding whether Engine-first ordering is sufficient. A binary swap alone cannot refresh an already running MCP process.

**Acceptance:** a conflicting same-version package is refused without damaging preserved artifacts; update failure and restart guidance describe the actual executable and playbook state. Local fixtures only; no release or UserInstall update is part of this handoff. Resolve question 8 before fixing update order.

### E. Reconcile the product's advertised contracts

**Both reviews; source-verified contradictions. Engine tool metadata, seed playbooks and documentation.**

Audit these together, preserving the distinction between git-managed and fallback ZIP installs:

- New Quiz `item_scores`, remaining manual entry cases, saved accommodations and `from_saved`/`push_quiz` guidance, including registered MCP descriptions and schemas.
- Implemented group Tier A/B versus frozen Tier C; module item removal/replacement; current tool registration inventory versus hand-written counts and missing MCP guide entries.
- The false claim that fallback ZIPs omit playbooks/`TEACHER.md`, versus the packaging script and install warning. Do not recommend extracting over teacher-edited files as safe.
- Git-owned product playbooks versus teacher-owned overrides/context and legacy/fallback ownership rules. Shorten `playbook-ownership-and-updates.md` around current rules without erasing supported distinctions.
- Stale product-line/date wording, repeated accommodation text, encoding damage, retired review terminology and proposal-era parent-update text.
- Historical discovery/review documents and duplicated handoffs: retire only material actually superseded. The active quiz handoff still records an unresolved course/quiz accommodation combination; completed item scoring alone is not grounds to retire the whole file.
- Public course IDs and test-object evidence flagged by Opus: inspect what is actually private and remove inappropriate identifiers from public material while retaining useful redacted evidence.

**Acceptance:** each advertised operation agrees with its maintained feature contract and implementation; update `internal/localui/capabilities.go` and `docs/engine-capabilities.md` together, including review date where their status changes. Link changing capability evidence from a shorter architecture document. No local result becomes Canvas Live proof. This slice can correct established contradictions without waiting for new feature decisions.

### F. Keep consequential MCP writes recoverable across cancellation

**Opus finding 4; subprocess cancellation mechanism source-verified, teacher impact inferred. MCP execution.**

Request-bound commands can be killed by cancellation or the 15-minute timeout during a write. Existing uncertain-write recovery may protect against duplicates, but the teacher can be left with an unclear outcome. Opus proposes the existing detached `longCommand` path or protecting an already started write from cancellation.

**Related Sol suggestion:** preserve structured held/partial completion status. The CLI uses exit code 3, while `Server.command` presents all nonzero exits as generic tool errors with outcomes in prose. Saved receipts still protect completed writes; no duplicate-write defect was demonstrated.

These address different problems. Design cancellation, reconnect/status and partial completion together only where the existing lifecycle supports them; do not introduce a job framework. Decide what cancellation means before detaching writes (question 7). Verify accepted outcomes remain discoverable and completed actions are not replayed.

### G. Move school-specific message choices and exact deadline math to their proper owners

**Opus findings 3 and 7; source-verified. Context/playbooks plus bounded Engine mechanics.**

- Parent email wording includes ELA 7, school-specific tutoring text, a hardcoded 70% failing threshold and a mandatory `bobcat_hour_url` that conflicts with playbook guidance allowing a blank link. Opus proposes configurable wording/cutoff/tutoring text in `context/parent-updates.md`, generic keys and backward-compatible reads of old keys.
- Scoring/group playbooks have the agent turn an approved “+N school days” allowance into a date even though the Engine already does school-calendar arithmetic for lateness. Opus proposes accepting the allowance and computing the deadline mechanically.

**Acceptance after policy decisions:** colleagues can use appropriate wording without changing report inventory, privacy or grade-report guarantees; blank tutoring configuration behaves consistently. The Engine computes an explicitly supplied allowance, while the teacher/agent still decides eligibility, genuine submission and educational policy. No accommodation default is invented. Resolve questions 9–10 before expanding schemas.

### H. Multi-PC delivery and provider conflict warnings

**Opus findings 1–2; local storage locations source-verified, cross-PC consequences/provider behavior inferred. Proposed scope changes, not established defects in the supported local-session contract.**

Family-contact and parent-update outcomes remain beside local runs. A separately recreated run on another PC has no shared sent history. Opus proposes a shared sent ledger, or moving whole family-contact runs to UserSync; parent grade-report runs would remain local. Opus also proposes doctor/Home warnings for provider-created conflict copies beside shared contacts/student/course files.

These require decisions before an execution handoff. Current architecture deliberately keeps sessions local and assumes one teacher uses one PC at a time. A synced filesystem lock is not reliable cross-PC exclusion; an absent or delayed ledger cannot prove a message was never sent. A newly recreated run also needs a meaningful common identity for deduplication. Provider conflict naming and delivery behavior were not observed in either review.

Decide supported cross-PC continuation and the evidence needed for a useful warning in questions 1 and 4–5. Do not build a ledger, migrate runs or add heuristic warnings merely to complete the other fixes.

### I. Bounded cleanup and smaller adapter issues

**Opus findings 8 and 10; source observations, with value/scope still to establish.**

- Remove genuinely unused scoring/authoring review machinery only after tracing surviving family-send, Draft PDF and `validate --show` consumers. Delete dead paths; preserve shared renderers and private review.
- Make WebUI course-client selection report unreadable course configuration instead of silently falling back to the operator's key. Opus reports the affected callers are roster/tag reads. Read the team routing contract and test the exact fallback case.
- Consider preventing MCP edits to git-owned product files and directing teacher changes to `Teacher_Override.md`; distinguish fallback installations before imposing a global rule.
- Correct demonstrated CLI/help/annotation drift, including repeated `prepare-late-timing`, missing external-operation annotations and copy-operation destructive hints. Consolidating all command lists is a separate scope choice, not automatically required.
- Repeated `/users/self` lookup and three group timing paths are possible small reuse opportunities, not mandatory refactors. Large files alone do not justify work.
- Consider a brief school-account nudge when selecting UserSync. Personal account discovery is reported behavior; neither review establishes that it should be prohibited.

## Proposed implementation order after direction is agreed

This ordering is the synthesis recommendation, not agreement on the sources' different rankings:

1. **A:** reproduced student-save loss and cross-adapter overwrite, in separate bounded slices.
2. **B and C:** native write auditing/classification and the reproduced family-review reconnect defect.
3. **D release identity and E:** package immutability and established contract contradictions. Documentation corrections can proceed independently of the Engine fixes.
4. **I routing fallback**, then the agreed portions of **F, G and D update lifecycle**, based on the answers below.
5. **H and discretionary I cleanup:** only where the chosen workflow or demonstrated maintenance benefit warrants them. Do not turn every observation into backlog work.

Each execution handoff should identify owner layer, current symbols, exact behavior, preserved contracts and focused checks. Reproduce meaningful reported defects in normal tests, then run applicable repository checks for the completed slice. These source reviews' successful `go test ./...` and `go vet ./...` runs are historical baseline evidence, not validation of future changes. Development does not authorize real-family sends or live classroom mutations.

## Conflicts, disagreements and questions to resolve them

The sources contain no explicit point-by-point rebuttals. The following includes light tensions and omissions that could otherwise be mistaken for consensus. Suggested resolutions are this synthesis's recommendations.

### 1. What should be first: demonstrated local loss or cross-PC family continuity?

**Difference:** Opus ranks a shared family sent ledger first; Sol ranks reproduced same-PC student-state loss first and does not propose changing local session ownership.

**Question:** Have you actually needed to resume or recreate family sends on a second PC, or experienced duplicate sends? If so, what exact workflow failed? Should reproduced local data loss remain first, or does that observed delivery problem take precedence?

**Suggested resolution:** fix local data loss first unless concrete family-delivery experience changes the priority. This also follows the project's rule against expanding scope from conceivable failures alone.

### 2. Does “no locks” prohibit a small local commit guard?

**Tension with the current contract:** Sol recommends cross-process exclusion; Opus praises atomic writes. These are compatible technically—atomic replacement does not protect the preceding hash comparison—but architecture currently says “no locks.” One-PC use does not exclude multiple local apps.

**Question:** Should that wording explicitly permit short per-file local commit guards while continuing to exclude distributed locks and a synchronization framework?

**Suggested resolution:** yes, for the reproduced local overwrite. State the narrow exception rather than implying a new cross-PC guarantee.

### 3. How should the Students page handle stale edits?

**Open choice within Sol's proposal:** reject-and-preserve versus applying intended field changes to fresh records. Opus does not choose either approach.

**Question:** If an open page is stale, is a clear conflict notice with all entered values retained acceptable, or must nonconflicting edits save automatically? If the same accommodation changed elsewhere, should the teacher choose which value survives?

**Suggested resolution:** use the smallest solution that preserves both newer stored data and unsaved form values; do not silently choose between competing edits.

### 4. If cross-PC family work is supported, what exactly travels?

**Scope tension:** Opus offers a sent ledger or whole family-contact runs; Sol emphasizes the existing explicit ownership model. Neither establishes a shared run identity or provider-level exclusion. Opus's “no student work” rationale does not make message text non-sensitive.

**Question:** Do you want cross-PC access to unsent drafts and revision notes, only a warning about prior sends, or neither? Must a recreated message match the original send, and by what stable identity? Is one-PC-at-a-time use with completed provider synchronization an acceptable limitation?

**Suggested resolution:** define that workflow before choosing storage. Do not promise duplicate prevention from an eventually synchronized ledger or `.delivery.lock`.

### 5. Are provider conflict-copy warnings justified now?

**Evidence difference:** Opus infers silent provider conflicts; Sol reproduces same-PC overwrite races and explicitly avoids claiming provider behavior. Those are distinct problems; one fix does not establish the other.

**Question:** Are there actual OneDrive/Google Drive conflict files or a documented supported naming pattern to detect? If not, should this wait for evidence rather than add speculative scans and false alarms?

**Suggested resolution:** fix demonstrated local races now; add a narrowly supported warning when evidence identifies what it should detect. No merge system.

### 6. How far should documentation and review-code cleanup go?

**Light difference:** Opus favors a broad plain rewrite, retirement of historical files and trimming old review machinery; Sol favors durable boundary documentation and bounded consistency corrections. Neither establishes that all remaining review code is dead, and the active quiz handoff has an independent unresolved part.

**Question:** Should the first pass correct contradictions and clearly dead paths only, followed by a separate short rewrite, or include restructuring now? Which historical evidence must remain easy to find?

**Suggested resolution:** correct behavior-affecting contracts first. Preserve live consumers and unresolved evidence; retire whole handoffs only when all parts are complete or explicitly carried forward.

### 7. What should cancelling a write mean to the teacher?

**Different adapter priorities:** Opus wants writes detached from request cancellation; Sol wants clearer structured partial/held completion and preserves the current CLI adapter. These complement each other but do not by themselves specify cancellation behavior.

**Question:** Should cancellation stop before the next independent item, detach the whole operation to completion, or only shield an in-flight write? How should the reconnecting agent discover completed, held and uncertain actions?

**Suggested resolution:** agree the lifecycle first, then reuse existing task/status and receipt mechanisms. Do not silently make a cancelled batch continue all writes merely by changing its context.

### 8. Is Engine-first updating sufficient, and how should restart work?

**Different release concerns:** Opus proposes Engine-first ordering and a restart line; Sol identifies immutable artifact identity. Neither proposal resolves the other, and a running MCP process still executes old code after a successful swap.

**Question:** Is an explicit restart requirement enough to prevent mixed-version use, or should application of staged playbooks wait until a coordinated restart/session boundary? What state should remain if the second update step fails?

**Suggested resolution:** fix artifact immutability independently; inspect and specify the smallest coherent update transition before adopting Engine-first as the solution.

### 9. Should the public seed preserve the current school-specific email?

**Internal tension in Opus's recommendation:** it criticizes school-specific text in the public repository while also proposing today's wording as the seed default. Sol does not address email customization.

**Question:** Should the public seed be generic, with existing ELA/school wording preserved in the teacher's context, or should the current weekly ELA layout remain an explicitly selected preset? Should the failing cutoff be course-specific or report-specific, and should a blank tutoring link omit that section?

**Suggested resolution:** generic reusable defaults plus preserved existing teacher choices; decide the configuration level before renaming keys. Keep old-key compatibility and the report guarantees.

### 10. What accommodation input should the Engine compute?

**Shared principle, one-review proposal:** both preserve the judgment/mechanics boundary; only Opus requests an allowance-to-deadline operation. The current contract does not authorize inventing allowance defaults.

**Question:** Is the required input strictly an explicitly approved “+N school days,” and how should it preserve due-time/timezone and handle non-school days? Is an existing calendar helper sufficient for the actual teacher workflow?

**Suggested resolution:** move only deterministic calendar arithmetic; leave eligibility and allowance choice with the teacher/agent. Implement for the requested workflow rather than designing a general accommodation language.

### 11. Should git ownership be enforced by MCP, or only explained?

**Light tension:** Sol treats explicit ownership/routing as a strength; Opus identifies that the file-write adapter still permits agent edits to git-owned files that then block updates. This is an enforcement gap, not disagreement about ownership itself.

**Question:** In git installs, should routine MCP writes to product-owned playbooks/guides be rejected with an actionable override path? What intentional customization remains supported in fallback ZIP installs?

**Suggested resolution:** enforce the already chosen git ownership boundary if current code confirms the gap, while preserving the fallback install's documented customization behavior.

### 12. Which smaller findings merit work beyond direct corrections?

**Difference in breadth:** Opus includes command-list consolidation, duplicate identity lookups, group timing reuse, public identifiers and a school-account nudge; Sol prioritizes demonstrated defects and discourages abstraction by size. Silence from Sol is not endorsement or rejection.

**Question:** Beyond incorrect routing, metadata/help and inappropriate public identifiers, which items cause real maintenance or teacher friction? Do you want a brief school-account recommendation, with personal accounts still selectable?

**Suggested resolution:** make concrete low-cost corrections; defer discretionary consolidation without creating a mandatory backlog. Preserve useful redacted test evidence, and do not treat every numeric Canvas ID as a credential.

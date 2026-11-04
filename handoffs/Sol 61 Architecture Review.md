# Sol 61 Architecture Review

Date: 2026-11-03  
Reviewed Dev revision: `f58ecaf` (`dev` matched `origin/dev`)  
Status: review findings and recommended direction; implementation has not started.

## Architectural assessment

The core architecture is sound. Keep the single Go Engine, feature-specific workflows, editable playbooks, and file-based UserInstall/UserSync model. The main weakness is inconsistent enforcement at a few storage, transport, and adapter boundaries.

The review covered the major packages, teaching contracts, distribution scripts, integrations, and existing tests. Findings below distinguish reproduced local defects from source-level weaknesses and architectural debt. No Canvas Live or destination-client behavior is inferred from local tests.

This document records the review. It does not authorize implementation, release, teacher-install changes, Canvas writes, or family sends. Before implementing an agreed slice, read the current root `AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/SCORING-RULES.md`, and the relevant feature contracts. File and line references describe the reviewed revision; locate current symbols before editing.

## Findings, in priority order

### 1. High: an open Students page can erase newer mappings and accommodations

**Area:** Engine storage and private student editor.

The standalone student editor loads its table once in `Students` ([internal/localui/students.go](../internal/localui/students.go), lines 346–361), retains it for the page lifetime, and later calls `h.store.Save()` at line 317. `StudentStore.Save` ([internal/workspace/students.go](../internal/workspace/students.go), lines 216–230) rewrites the entire CSV without checking whether its loaded version changed.

An ordinary same-PC agent pull or roster refresh while that page remains open can add students or change settings. The teacher's later Save can remove those new mappings or restore old accommodation settings. This does not require simultaneous multi-PC use.

A synthetic probe reproduced all of these consequences:

- The table dropped from two rows to one.
- A newer `QuizTime: "50%"` setting became empty.
- The removed student's existing packet reference became unmapped.
- The next `Upsert` generated a different random reference.

Grading fails closed rather than targeting another student, but saved work and notes can become disconnected, and accommodations disappear silently.

**Recommended fix:** reject stale saves while preserving form entries, or apply only the teacher's intended changes to freshly loaded records using stable identity. Keep any local commit guard small; this does not justify a general persistence framework.

### 2. Medium: file conflict protection stops at one MCP adapter

**Area:** Engine workspace writes through connected apps.

`writeWorkspaceBytes` ([internal/mcpserver/workspace_files.go](../internal/mcpserver/workspace_files.go), lines 238–280) protects hash comparison and replacement with `s.mu`, which belongs to one `Server`. Separate connected apps have separate servers.

Two adapter instances can accept the same `expected_sha256`, report success, and replace the same file, losing one acknowledged edit. A focused synthetic probe reproduced this with a 20 MiB asset, within the supported asset limit, on its first attempt. The same comparison-and-replacement helper serves drafts and teacher context.

This demonstrates a same-PC boundary defect. It does not establish cloud-provider conflict behavior or the frequency of overlapping writes in real clients.

**Recommended fix:** make hash comparison and replacement exclusive across local Engine processes for the destination file, then compare the current hash inside that protected commit. Keep the scope at the file boundary rather than adding a database or synchronization framework.

### 3. Medium: native New Quizzes bypass shared transport safeguards

**Area:** Engine Canvas transport and teammate audit behavior.

The specialized native New Quizzes HTTP path creates another client and calls `secureJSON` directly ([internal/canvas/newquiz.go](../internal/canvas/newquiz.go), lines 198 and 307–355). Request counting survives because the transport is shared, but ordinary `Client.OnWrite`, `MapError`, throttling, and write-error classification do not.

Two consequences are verified in the implementation:

- Native item-score writes in `NewQuizGrading.PostScores` ([internal/canvas/newquiz_grading.go](../internal/canvas/newquiz_grading.go), line 176) never invoke the teammate audit hook registered in [internal/workflow/team_route.go](../internal/workflow/team_route.go), line 197. This contradicts the team contract's promise to record every Canvas write made with a teammate's key.
- Native 408/redirect responses and some failures reading or parsing a successful write response become ordinary validation failures. `PostScores` marks transport errors and statuses of 500 or higher uncertain, leaving those other ambiguous cases recorded as failed.

Existing result-version checks reduce replay risk. The review did not demonstrate duplicate grading.

**Recommended fix:** retain native authentication and session handling, but reuse small helpers for error classification and write observation. Grade-changing native calls should emit redacted audit records. Add focused fixtures for these specific behaviors rather than a universal operation framework.

### 4. Medium: family-review reconnection loses access to revision notes

**Area:** MCP recovery of the private family review lifecycle.

`start_parent_review` passes the run folder to `startReviewSession` ([internal/mcpserver/server.go](../internal/mcpserver/server.go), line 307). `FindPage` correctly locates the reviewed `contact.json` or `drafts.json` within that folder, but the reconnected task retains the folder as its subject ([internal/mcpserver/sessions.go](../internal/mcpserver/sessions.go), line 63).

After the page closes, `externalReviewStatus` calls `LoadReviewState(task.subject)` at line 480. It therefore looks for `<folder>.review.json`, while the notes are saved in `<folder>/contact.json.review.json` or `<folder>/drafts.json.review.json`.

A synthetic folder-shaped reconnect probe reproduced empty revision notes. The notes remain on disk; the agent does not receive them. Existing reconnect coverage uses a file subject rather than the actual family API's folder subject.

**Recommended fix:** preserve `rec.Subject` when reconnecting and extend the existing fixture with the real family-folder request shape.

### 5. Medium: the current release packager lacks immutable-artifact protection

**Area:** Dev distribution to the userinstall branch and release assets.

[scripts/package-userinstall.ps1](../scripts/package-userinstall.ps1) overwrites assets and checksums at lines 174–181, deletes an existing same-identity ZIP at lines 188–190, and can create another distribution commit with changed manifest hashes at lines 215–225.

[scripts/compile.ps1](../scripts/compile.ps1), line 99, embeds a fresh build timestamp, and the MCPB is documented as nonreproducible. Rebuilding therefore changes artifact bytes while retaining the release version and URL.

A rerun followed by pushing the regenerated userinstall branch can advertise hashes incompatible with already published immutable assets. This is a source-level release-integrity weakness; the review found no evidence of a currently broken published release.

**Recommended fix:** preserve one approved artifact set per release version and refuse replacement during release packaging, matching the existing protection in `scripts/package.ps1`. A reproducible-build overhaul is unnecessary for this fix.

### 6. Medium: model-facing contracts contradict implemented capabilities

**Area:** Playbook and Engine contract consistency.

The registered `post_results` description ([internal/mcpserver/operations.go](../internal/mcpserver/operations.go), line 360) says New Quizzes can receive only an attempt-scoped comment, while the implementation and scoring playbook support item scores. The `from_saved` schema and `push_quiz` description also retain pending-setter language after saved accommodation writes were implemented.

[docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md), line 94, retains older manual-item-grading language despite describing Engine-written item scores later. Line 118 describes delivered group functionality as unimplemented, conflicting with the group-work contract and capability records.

These declarations influence what agents attempt. Their inconsistencies affect product behavior rather than merely documentation quality.

**Recommended fix:** correct the contradictions, keep architecture documentation focused on durable boundaries, and link changing capability status to its maintained evidence record. Include registered tool descriptions and schema guidance when reviewing a feature's advertised behavior. Preserve the distinction between local tests, Canvas Live evidence, and pending human validation.

## Architectural strengths to preserve

- **Exact scoring:** `internal/domain` uses rational arithmetic and deliberate boundary rounding. Interpretation and evidence remain agent-supplied, while the Engine validates their mechanical consistency.
- **Consequential posting checks:** feature workflows renew identity, attempt, audience, grading-basis, consent, and visibility checks and persist independent action outcomes. Completed writes are not casually replayed.
- **Group audience separation:** shared assessment and whole-group comments are distinct from private per-student feedback. Keep these feature-specific semantics rather than flattening them into ordinary individual results.
- **Ownership and routing:** UserInstall/UserSync routing is centralized. Git-managed product files and reusable teacher setup have explicit ownership. Unavailable UserSync does not silently select stale local settings.
- **Credential boundaries:** Windows Credential Manager keeps Canvas credentials outside reusable teacher files.
- **Private review:** reviewed family HTML uses sandboxed previews, origin checks, session secrets, and approval digests that include attachment bytes. No demonstrated HTML escape into credentials or private page state was found.
- **Teacher judgment:** the Engine/playbook division substantially keeps educational judgment outside mechanical code.
- **Portable distribution:** the single local Engine and explicit update staging fit the supported Windows, standard-user environment. A synthetic `get-engine.cmd --stage --manifest NEXT` check downloaded and verified next-version bytes without replacing live files.

## Recommended direction

Project growth does not justify a rewrite or package splits based only on file size. Retain the modular monolith and per-feature operations.

Address local save integrity first, then native transport consistency, followed by family-review recovery, release integrity, and contract cleanup. These can be bounded implementation handoffs once the direction is agreed. Shared helpers should enforce the demonstrated low-level boundaries without absorbing feature judgment into a universal operation framework.

An additional, lower-priority adapter improvement is to preserve structured completion status: the CLI deliberately returns exit code 3 for held or partial work, while `Server.command` turns every nonzero exit into a generic tool error with outcomes embedded in prose. Saved Engine receipts still protect completed writes; no duplicate-write defect was demonstrated. A small status/exit-code envelope could improve client interpretation while retaining the existing CLI adapter.

## Validation and evidence limits

Baseline checks passed:

```text
go test ./...
go vet ./...
```

Three additional synthetic probes used temporary Go overlays and failed their preservation assertions, reproducing the findings rather than modifying the repository's tests:

| Probe | Reproduced result |
|---|---|
| `TestReviewStaleStudentSavePreservesNewerRowsAndSettings` | A stale store save removed a newer student mapping, reset a quiz accommodation, and disconnected the old reference. |
| `TestArchitecturalReviewConcurrentAdapters` | Both adapter instances accepted the same original hash; one acknowledged asset write was lost. |
| `TestArchitecturalReviewFamilyFolderReconnect` | Saved family revision notes were absent from reconnected MCP status. |

The overlays were temporary review tools, not committed regression tests. Their setup and assertions should be incorporated into focused normal-suite regressions when the corresponding fixes are implemented; implementation must not depend on their temporary filesystem paths.

The original suite remained green. The review made no repository changes and performed no live Canvas or Outlook actions. This saved report is a subsequent documentation-only change. No fresh destination-client adoption, cloud-provider propagation, or student-facing validation is claimed.

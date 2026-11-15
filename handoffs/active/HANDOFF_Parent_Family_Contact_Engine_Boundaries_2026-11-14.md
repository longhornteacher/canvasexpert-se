# Parent updates and external communication: remove Engine content restrictions

Status: **implementation started; slice 1 incomplete**. Requested by the developer on 2026-11-14. Audited baseline: `a41cfb0`; `dev` matched fetched `origin/dev`, with a clean working tree before this document.

Executor progress 2026-11-14: parent tables accept inclusive date, course and course-qualified assignment filters, grade and submission statuses, and `predicate_mode` (`any` or `all`); unknown table fields are rejected. Report pulling accepts an explicit range or named period in CLI and MCP, preserves the default, and records range provenance; historical pulls use current Canvas facts. Assignment-scoped evidence issues allow an explicitly excluded course or assignment to avoid a hold, while legacy flags and potentially relevant issues still hold. Exact assignment selection currently requires the assignment to appear in saved due rows or issues. Focused tests cover scope, union/intersection, threshold boundary and relevant holds. Exact undated/upcoming selection, further relevance refinement, remaining slices and completion checks are not yet implemented. No live operations were performed.

Verification so far: focused workflow tests passed after the latest changes; the requested six-package Go test command passed before the final unknown-field and date-coverage validation edits, and `go vet ./...`, `git diff --check`, and the compiled synthetic MCP smoke (68 tools) passed afterward. Repeat the full package checks after completing implementation.

## Governing decision

The developer's direction:

> Engine's job is enabling functionality and preventing falsehood, not determining what content is appropriate or proper.

The triggering request was to notify families about selected missing/not-turned-in/low-scoring assignments within teacher-chosen dates. The exact cutoff, categories, eligibility and wording belong to the teacher and agent. Do not settle those choices here or replace the full-term rule with a hard-coded two-week rule.

Accept content and scope choices; execute them against accurate facts. Preserve arithmetic, identity, grade privacy, private contacts, actual recipient scope, reviewed-content binding, accepted outcomes and uncertain-send protection. Useful defaults need not become compulsory content rules. Validate what the requested communication uses; keep independent deliveries moving when one cannot proceed.

Factual guarantees apply to the facts and routing the Engine supplies. Free prose is already agent-authored; do not add a semantic content classifier to decide whether that prose is appropriate or whether an educational offer should be made.

This supersedes restrictive Engine requirements in the retired 2026-11-07 parent-email handoff: whole-current-term-only retrieval, threshold-only selection, exactly one mandatory table and blanket address rejection. It does not authorize sending family emails outside the teacher's private review page.

## Start and scope

Read root `AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/SCORING-RULES.md`, `docs/PRIVACY.md`, `handoffs/README.md`, `docs/parent-update-flow.md` and `docs/family-contact-flow.md`. Read `docs/team/README.md` before changing reply routing, and `docs/group-work/README.md` plus `contracts.md` before changing family group resolution. Inspect current symbols rather than trusting historical line numbers.

Fetch and compare `dev...origin/dev`; pull if behind. Preserve current work, including the separate assignment/quiz authoring handoff. Execute the slices sequentially with existing packages. No subagents are authorized by this handoff. Leave changes uncommitted unless requested.

This document is the fixing handoff for another session; its creation changed no runtime behavior. Implementation covers existing parent-report/email/print and Family Contact paths, plus the announcement prose sentinel below. Inbox was inspected as an adjacent boundary. New conversation creation, Inbox attachments/forwarding, new email providers, arbitrary external address books, SMS, scheduling and frozen Tier C club messaging are outside scope. Do not build a universal communications framework.

No release, push, install, UserInstall/UserSync mutation, real-family sends, real grading or Canvas Live probes are authorized. Use synthetic fixtures and fake senders. Pending live/client observations are evidence limits, not prerequisites for supported local functionality.

## Confirmed restrictions

These findings come from source and existing test assertions, not the teacher's original chat or an inspection of their installed Engine.

| ID | Source and current behavior | Mechanical problem / disposition |
|---|---|---|
| P1 | `parent_report.go`: `ParentReportWithScope`, `makeParentReportScope`; CLI `parent-report`; MCP `ParentReportInput`. Requires a valid calendar and current active term; `full_period` is ignored. | No explicit range or selected-period retrieval. A complete supplied range should not require fixing unrelated calendar setup. Add caller scope, keeping current-term behavior as default. |
| P2 | `parent_report.go`: `buildStudentReport`. Undated work is dropped; upcoming work is email-inaccessible in the separate print list. | Cannot explicitly select an undated assignment or use upcoming facts in an email. Permit requested fact selection without inventing dates. |
| P3 | `parent_drafts.go`: `ParentTable`, `tableRows`. Only `below_percent` selects rows; when present it requires a released graded score. | Cannot combine missing-without-grade OR low-scoring work, date bounds, course or exact assignment choices. Add small typed selectors. |
| P4 | `parent_drafts.go`: `validate`, `Render`, `renderTable`, `ParentStudentDraft`. Exactly one standalone table is mandatory; subject/body/table are global. Student overrides only decide send/skip/flag; their intro is print-only. | Cannot make separate selected fact sections, a prose-only update, individual wording or an update with attachments. Enable composition over verified facts and reuse Family Contact media. |
| P5 | `parent_drafts.go`: `validatePrivacy`. Rejects every authored email address, including labels and URLs. | Teacher public contact text and `mailto:` links fail. Remove the blanket regex ban; preserve private-contact and internal-identifier boundaries. Family Contact already permits authored addresses. |
| P6 | `parent_drafts.go`: `parentHoldReason`, `resolveDecisions`; report completion flags. Any course flag or malformed inventory row holds the student regardless of selected rows. Completion also examines upcoming work. | An unrelated unused row can prevent a narrow update. Require completeness only for the requested result; unknown evidence that could affect it still matters. |
| P7 | `parent_gradesheet.go`: `RenderStudent`; `parent_send.go`: `BuildStudentSheets`. Always prints every saved student's due/upcoming work with fixed columns/wording. | No explicitly narrower printed report. Keep the existing grade check as default; allow explicit selection/labels without requiring email drafts. |
| P8 | `parent_drafts.go`: `LoadParentRun`; `family_contact.go`: `LoadFamilyContact`. Plain `json.Unmarshal` ignores unknown properties. | A caller-invented date/status filter can be ignored, producing a broader message than intended. Reject unsupported selector fields at the authoring boundary with a useful error; preserve documented legacy fields. |
| F1 | `parent_send.go`: `familyRecipients`, `BuildParentReview`, `deliverFamilyMail`; `family_review.go`. Only student-plus-parents or parents-only; enabled parent email always mandatory. | Student-only mail cannot use an available student address. Add explicit students-only routing; never silently change requested modes or enabled-parent choices. |
| F2 | `family_contact.go`: `resolveFamilyAudience`; `family_review.go`; `ParentOutcomes`. One message per student per run; review/outcomes keyed only by student. | Distinct course messages or a book update plus reminder block the run. Enable explicit per-message deliveries with stable message/student action keys, preserving old outcomes/defaults. |
| F3 | `family_contact.go`: `LoadFamilyAudience`, `CheckFamilyAudience`. Mtime invalidates an unchanged file; contact digest includes wording and selectors. | Identical rewrites and wording-only edits require live re-preparation. Remove mtime rejection; separate audience identity from reviewed content identity. Changed content still needs fresh review. |
| F4 | `family_contact.go`: audience identity/checking; `family_review.go`: build/send. Digests include all read-course enrollments and all groups/members/names in selected sets. Every blocking problem holds every card, before teacher Skip is applied. | Unrelated drift or an excluded problem stops independent recipients. Revalidate relevant selected deliveries; hold affected scopes and never add unreviewed recipients. |
| F5 | `family_contact.go`: explicit `student_refs` resolution. Reads every configured course; an unrelated operator-owned course failure aborts even when a ref is found elsewhere. | One exact student depends on unrelated course access. Add course-qualified student selectors and avoid unrelated prerequisites; retain bare-ref compatibility without guessing enrollment. |
| F6 | `family_contact.go`: `add`, `FamilyAudienceRow`. Same-message dedup retains one course/group, preferring a group and otherwise the first match. | Other contexts are lost although Reply-To and group wording depend on them. Preserve relevant contexts; do not choose an arbitrary group/teacher. Hold only genuinely ambiguous deliveries. |
| F7 | `family_render.go`: `renderFamily`; `family_reply.go`: `replyFor`; `loadParentConfig` callers. Always appends saved closing and fixed teammate explanation; reads parent settings for free-form closings. | Cannot explicitly replace/omit footer prose; unused parent-update configuration can block free-form mail. Make footer choices overridable while keeping actual reply routing. |
| D1 | `parent_send.go`: `deliverFamilyMail`; family content preflight. An individual render error returns from the loop; one invalid/oversized message can block all preflight. Missing destinations sometimes continue without reasons; grade holds can become generic skips. | Isolate affected messages/actions and report honest reasons. Shared storage, outcomes/lock and Outlook availability failures remain legitimate shared stops. |
| D2 | `parent_send.go`: `SendParentTest`; family review model. No Family Contact self-test; parent test requires an eligible send student. | Family eligibility can block testing rendering to the teacher alone. Reuse the teacher's private-page self-test for a renderable selected sample, forced to self only. |
| A1 | `internal/domain/discussion.go`: `ValidateBody`; `workflow/discussion.go`: `updateDiscussion`. Announcement create/update requires nonempty prose `announcement_intent`; any text satisfies it. | This cannot establish authorization and duplicates explicit document type/destination. Remove the prose prerequisite, retaining announcement identity, scope, baselines and receipts. |

Existing tests explicitly assert several restrictions: `TestDraftValidation` (rejects `me@school.example`), `TestFamilyAudienceDuplicateMessagesAreReviewableAndBlockDelivery`, `TestFamilyAudienceRejectsContentDriftMembershipDriftAndSnapshotEdits` (includes touching unchanged content), and `TestFamilyReviewUncertainSendHeldAndMembershipDriftStopsAllMail`. Replace obsolete assertions deliberately; preserve their substantive identity, edited-snapshot, privacy and no-replay cases.

Media integration detail for P4: parent `Render` already calls `renderFamily`, but returns only subject/HTML through `Rendered`; it drops the returned attachments and asset digest. Inline local images can therefore leave CID references without passing their files to Outlook. Reusing that renderer must carry its full media contract through parent review and delivery, not merely allow an `attachments` field in JSON.

## Retain these checks and actual capability boundaries

- Verified Canvas origin/course/student/assignment/group identity, published/student-visible scope and excusal facts. Exact group ambiguity is a wrong-target check. Preserve individualized effective dates, explicit null versus omission and assignment-scoped visibility completion.
- Exact released-score arithmetic. An absent/unreleased grade is not zero. Submission evidence stays independent: a submitted zero is not missing; paper work is not unsubmitted merely because it has no upload. Unknown percentages cannot satisfy score predicates.
- Complete relevant evidence for a complete list or an empty qualifying result. Unknown effective dates that could fall in range cannot be waved away. Explicit excluded assignments and known out-of-range work need not hold unrelated results.
- Private contacts, active UserSync authority, enabled-contact choices, Engine-side name/address resolution and no private recipient data in agent outputs. Permitting authored public addresses does not permit contact-file reads or private-roster interpolation into shared prose. Retain internal student-ref/alias leakage guards.
- Teacher-controlled private family review/send, exact rendered-content/assets/recipient binding and loopback protections. No new conversational approval token and no raw-address sending tool.
- Constrained run/assets paths, symlink/junction checks, safe HTML, executable/remote-load neutralization and the current conservative 20 MiB email estimate. Report missing/unsupported media; do not add remote fetching or a media subsystem.
- Delivery lock, unreadable-outcomes hold, pending record before Outlook, no replay of accepted/uncertain actions and fresh Outlook items. These prevent duplicate real mail. Actual COM/platform/profile failures are technical limits, not policy gates.
- Known factual-column validation. Rejecting invented history when no history exists is justified. Fixed supported fact types are not themselves inappropriate policy; column order/labels/selection and message composition are authored choices.

Not every absent feature is an unnecessary block: no generic mail client, new contacts store, BCC/broadcast API, arbitrary addresses, new Inbox threads or forwarding/attachments are requested here. The current run folder convention is not preventing the triggering request; preserve UserInstall-local session ownership and existing conventions.

## Slice 1: caller-chosen fact scope and selection

Extend existing report workflow options, CLI and typed MCP input instead of adding a parallel command. Add explicit inclusive local-date bounds (`from`, `through`, YYYY-MM-DD) or an explicit named grading period. They are alternative scope choices. Omitted scope retains current-term-through-as-of; preserve `full_period` compatibility. An explicit complete range needs a valid IANA timezone, not an active period or unrelated school-closure configuration. Use an explicit timezone or configured course timezone and record the actual effective scope/as-of. Do not mislabel a range as a term. Reject reversed dates; distinguish known future assignment facts from claims about future grades.

Allow requested exact course-qualified assignments and known undated/upcoming facts. Do not invent dates. Email-only selection must not destructively rewrite report facts or silently narrow later default printing; narrowing the pull itself remains an explicit caller choice.

Add typed table selectors for due bounds, course-qualified assignments, grade status, submission status and score threshold. Date/course/assignment choices narrow the universe. Support explicit union or intersection of status/score predicates; document the rule so missing OR low does not become missing AND low. Existing `below_percent` drafts keep their meaning. Do not build an expression language or infer correction/late-work eligibility from elapsed days, low grades or lock dates.

Preserve structured evidence issues with enough assignment/course scope to determine relevance, keeping readable flags for compatibility/display. Unknown evidence potentially affecting requested selection remains a hold. A known excluded row does not. A course-wide inability to identify relevant work remains a hold on that requested course result. Do not send a reassuring empty list from incomplete potentially qualifying evidence.

Acceptance: bounded missing-without-grade OR below-threshold selection; exact threshold excluded; individual due-date override; explicit historical range without an active period; irrelevant versus relevant issue; one requested exact undated assignment. Reuse report/visibility fixtures, not a broad edge-case matrix.

## Slice 2: authorable messages over verified facts

Keep schema 3 drafts loadable. Add optional named table definitions/placeholders through the existing renderer, retaining `{{assignment_table}}` as the legacy default. Allow zero tables, one table or distinct named tables. Validate only definitions actually used. A standalone Markdown block requirement is acceptable for rendering; a mandatory single table is not.

Add per-student subject/body/table-choice overrides without changing their identity or supplying new grade facts. Reuse `FamilyMessage`/`renderFamily` Markdown/HTML and local media/attachment support, including review filenames and content/asset digest. Different layouts must not require the agent to transcribe student scores, links or dates into hand-made tables. Factual cells still come from report data. Support authored grade-status labels where needed to distinguish no grade, awaiting grading and unreleased grade without inventing zero.

Remove blanket authored-email-pattern rejection from parent drafts. Keep private-file boundaries and internal-identifier leakage protection; do not promise semantic detection of all privacy errors in prose. Public teacher addresses and `mailto:` links should behave consistently with Family Contact.

Let messages use, replace or suppress the saved closing and explanatory reply note, preserving old defaults. Actual Reply-To remains computed from the relevant course owners; suppressing explanatory prose does not reroute replies. Load only settings needed by the selected options, so an explicitly supplied/suppressed closing does not depend on unused parent-update configuration.

Allow an explicitly configured printed report to reuse fact selectors, supported columns and labels. No options retains the existing grade check, and `drafts.json` remains optional. Preserve private name output and existing PDF/local ownership. Extend the grade-sheet renderer; do not introduce a template engine.

Acceptance: prose-only update; two differently selected fact sections; per-student wording; public contact link; local attachment; omitted closing; narrowly selected print. Preserve correct scores/names and unreleased-grade privacy. A changed attachment invalidates prior review; existing default draft/print remains usable.

## Slice 3: recipient modes, message identity and scoped holds

Add `students_only` beside omitted/student-plus-parents and `parents_only` in both paths. Resolve requested destinations only; a parent address is required only when parents are targeted. Never silently fall back to another recipient mode or override disabled parent preferences. Missing requested destinations hold specifically.

Add an explicit per-message Family Contact delivery mode. Keep legacy/default one-version-per-student behavior for variant mail, while allowing the author to choose distinct messages. In per-message mode, review, Skip/Revise, delivery, pending/accepted outcomes and retries use stable `(message key, student ref)` action identity. Repeated selectors for the same message still deduplicate. Do not pass composite review keys to a student-only skip map. Keep student refs separately in agent-facing outcomes.

Preserve schema 2 outcomes. A legacy sent/uncertain student record cannot be discarded or treated as permission to resend when switching modes. If attribution to a message is unavailable, hold that student's ambiguous legacy actions specifically, while independent unsent actions can continue. Do not migrate every run folder or invent a ledger service. Preserve draft-only no-repeat and subsequent fresh Send at the same action granularity.

Support course-qualified explicit student selectors and preserve relevant course/group contexts during same-message deduplication. Do not require unrelated course access for a verifiably selected student. Bare-ref compatibility must not guess among conflicting reply/group contexts.

Replace whole-inventory membership gates with relevant selected-delivery checks. Renew actual target identity/membership before send. Changed membership or unresolved selectors hold affected deliveries/messages where independent scope can be established. Do not silently substitute recipients, switch messages or add newly eligible people absent from review. New recipients need fresh review; unchanged approved people can proceed. A failed read that makes an entire selected message audience unknowable still holds that message.

Separate preparation audience identity from content identity; remove mtime rejection. Identical rewrites and wording-only edits do not require live roster preparation if selectors/identities are unchanged; changed wording/assets still receive fresh review. Keep verification that an edited audience snapshot cannot redirect a message. Do not delete drift protection wholesale.

Acceptance: students-only with no parent email; two messages to one student with independent Skip/retry/no-replay; preserved legacy sent/uncertain outcomes; identical rewrite; wording-only fresh review; selected versus unrelated membership drift; a selected student despite unrelated course failure; truthful multi-course reply/group context. Extend existing synthetic family fixtures and revise whole-run-stop assertions.

## Slice 4: delivery isolation, self-test and announcement sentinel

Carry message/action validation failures into review and outcomes. One render, size, evidence or recipient problem must not abort independent approved mail. Shared storage failures, unreadable outcomes, locks and unavailable Outlook can still stop the batch for their actual reason. Preserve accepted/uncertain records.

Report teacher Skip, empty selection, evidence hold, unavailable destination, render failure, drafted, sent and uncertain truthfully, without addresses. Do not persist Engine holds as reasonless teacher skips. Keep compact summaries plus per-action reasons, so a partial retry identifies what remains and why.

Reuse the existing private-page self-test for both paths. The teacher selects a renderable message/sample; rendering tests need no eligible family send. Force To to the profile's own address and omit family To/Cc. Keep no agent-triggered send/test endpoint. Do not call actual Outlook during this implementation; verify with a fake. An unrenderable sample reports its exact error.

Remove mandatory nonempty `announcement_intent` checks on create/update; accept the old field as optional descriptive context. Keep explicit announcement type and exact course/group/topic identity, observed edit baselines, publication handling, receipts and uncertain-create discrimination. Do not collapse announcements into discussions or change teacher/agent authorization instructions.

Adjacent Inbox audit: `InboxReply` checks conversation identity and the exact latest message ref, then sends the supplied body only to existing participants. No topic classification or sensitive-content ban exists in code. Retain these mechanics and uncertain-write treatment. Reply-only scope and absent new-thread/forwarding/attachment capabilities remain outside this cleanup.

Acceptance: one invalid message with independent fake success; distinct skip/hold/failure reasons and accepted outcome preservation; self-only test with no eligible family; no family destination leakage; explicit announcement create/update without prose sentinel, retaining wrong-target/conflict/no-replay checks.

## Contracts, verification and completion

Update CLI help/parsing/arity, MCP schemas/descriptions, formats/examples and review/outcome consumers together. Inspect `cmd/cse/{main,remote}.go`, `internal/mcpserver/{server,sessions,workspace_files}.go`, `engine-docs/formats.md`, `internal/localui/{review.go,templates/review.html}` and saved review-state readers when introducing message action keys. Keep actual student refs available without addresses. An unsupported requested filter must error clearly, never be silently ignored and broaden delivery.

Update `docs/parent-update-flow.md`, `docs/family-contact-flow.md`, relevant architecture/scoring paragraphs and both capability records (`docs/engine-capabilities.md`, `internal/localui/capabilities.go`) with the implementation date and local-only evidence. Retire contradictory historical assertions that read as active limits. Add an unreleased changelog entry. Preserve dated historical live/Outlook evidence without claiming this change establishes new live delivery, printing or installed-client behavior.

Seed instructions should explain enabled inputs and teacher/agent responsibility. Existing full-term/below-70 rules can remain defaults with explicit teacher overrides; they must not claim the Engine requires them. Do not settle this developer's classroom policy or overwrite teacher-owned context.

Run focused existing tests per slice. At completion run `go test ./internal/domain ./internal/workflow ./internal/mcpserver ./internal/localui ./internal/outlook ./cmd/cse`, `go vet ./...` and `git diff --check`. Run `scripts/check-mcp.mjs` against a temporary compiled Dev Engine because public contracts change. Verify review action selection and rendering with synthetic tests. No real-family page or Outlook operation is needed. Broaden only for affected packages or concrete failures.

Record delivered slices, actual checks, compatibility limits and incomplete work. Retire this handoff only after implementation and local verification. Keep pending live/human observations separate, without making them gates on ordinary supported use.

Audit-session validation: source and existing test assertions inspected; no Go tests or live service calls run. Only this handoff and its index entry were written.

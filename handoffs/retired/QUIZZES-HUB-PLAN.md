> **Retired 2026-10-10.** Rationale for Classic quiz writing, now implemented and superseded by handoffs/retired/CLASSIC-QUIZ-RETIREMENT.md.

# Quiz and Hub capability plan

2026-10-09. Proposed direction for discussion, not an implementation authorization or execution brief. No Canvas calls or implementation changes were made to prepare this plan.

The detailed implementation decisions are now fixed in [CLASSIC-WRITING-EXECUTION.md](CLASSIC-WRITING-EXECUTION.md). Use that brief for agent execution; it supersedes the unresolved implementation choices below. This document remains the strategic rationale. Quiz resume hardening is excluded.

## Recommendation

Keep the existing quiz authoring and Hub implementation. Add one complete teacher workflow: pull, assess, and post Classic quiz writing with a score and comment per question. Continue to use New Quizzes for auto-scored work and ordinary Assignments for standalone writing.

OG CE is a source of Canvas observations and teaching workflows, not a target architecture or parity checklist. Its Classic design records authoring and Hub as shipped but scoring as a next batch backed by a probe. Port that observed API behavior, not an assumed finished scoring subsystem.

## Engine versus Playbook

Determinism is necessary for many Engine operations but is not the dividing line. A fixed rule that gives every blank answer zero, assigns equal weights, or assigns students to a support tier is deterministic and still makes a teacher decision.

The useful boundary is **deciding what should happen versus reliably executing an explicit decision against Canvas**. The Engine also owns narrow integrity constraints that remain necessary regardless of teaching preferences: correct identity, score bounds, restricted-page visibility, credential privacy, and preserving unsent work.

| Work | Owner | Boundary |
|---|---|---|
| Choose quiz engine, question types, prompts, distractors, standards, supports | Agent using Playbook/context | Engine translates supported authored structures and explains unsupported ones. |
| Choose points, scoring criteria, response units, weights, and rubric mapping | Teacher/agent | Engine requires explicit numeric inputs and performs exact arithmetic; it does not infer pedagogical weights. |
| Decide which response deserves which score and why | Agent | Engine checks question identity, current attempt, numeric bounds, and writes the supplied assessment. |
| Interpret blank, accidental, incomplete, or unreadable work | Agent | Engine preserves the evidence and marks failed extraction; missing evidence is not an automatic zero. |
| Compare earlier academic attempts | Agent | Engine supplies correctly associated histories; it does not select the strongest work from entered grades. |
| Resolve course, student, quiz, assignment, question, and attempt IDs | Engine | Names and positions are not write identities. No private student mapping in ordinary agent files. |
| Count TEKS already declared on items | Small agent script initially | Standards alignment and coverage adequacy are judgment. Promote a reusable report only when repeated use warrants it. |
| Decide accommodations and lateness | Agent using context | Existing Engine timing writer applies supplied timing; Canvas applies penalties. No calendar-policy subsystem. |
| Choose tier supports and Silver/Red/Blue educational meaning | Playbook/context | ELA7 policy stays here. Engine resolves an explicitly supplied tag and enforces restricted page delivery. It does not read membership or place students. |
| Publication and posting intent | Teacher, interpreted by agent | Engine executes the requested scope and verifies relevant results. Draft stays local; score-and-post authorizes posting. |
| Recover an interrupted write | Engine | Save returned IDs/outcomes, read the exact target, and explain ambiguity. No operation ledger or universal recovery framework. |
| Feedback quality and privacy of its meaning | Agent | Keep teacher context separate from output fields. The Engine cannot reliably infer that a sentence reveals an accommodation or is pedagogically appropriate. |

Repeated, exact computation can justify Engine support when it has a stable input/output contract and meaningfully saves context or prevents consequential mistakes. Being expressible as code does not by itself justify a product feature. One-off counts and calendar calculations can remain agent-run scripts.

## Findings that change the plan

1. `UnsupportedAssignment` in `internal/workflow/canvas_reads.go` currently excludes quizzes, and the submission DTO does not retain per-question `submission_data`. Classic scoring requires an actual read contract as well as a write endpoint; removing the exclusion alone would be incorrect.
2. `internal/workflow/post.go` posts ordinary assignment assessments using `posted_grade`. OG CE's recorded Classic probe used question scores and comments; Canvas updated the total itself. Classic scoring must not flow through the ordinary assignment-total writer.
3. Quiz resume uses existing item counts. Its intended use is retrying an interrupted push with unchanged source and no intervening item edits, not general repair. A hypothetical middle deletion does not justify new recovery machinery, tests, or a scoring prerequisite. No frequency or actual teacher failure was established for that scenario.
4. OG CE's historical Hub student-view observation and CE-SE's current Canvas read-back evidence are different evidence. Neither upgrades the other into a current CE-SE student-experience check.

## Sequence

### 1. Classic writing: read through the normal scoring workflow

Keep `pull-work` as the teacher-facing entry point. Add a direct Classic-specific branch using existing Canvas transport, workspace identity, attachment handling, and file conventions. Do not generalize assignment and quiz grading into a new adapter framework.

First supported use: CE-SE-authored, ordinary individually submitted Classic quizzes with essay or file-upload questions. Do not require an authoring sidecar for grading if live Canvas establishes all necessary identities. Do not promise question-bank/randomized/versioned quiz support until responses can be matched to their actual attempt questions without guessing.

The packet needs:

- The course, linked assignment and Classic quiz identity; stable student reference; quiz-submission identity and exact attempt.
- Question IDs, types, prompts, possible points, and the current response/attachment references. Use Canvas's attempt-scoped question endpoint, once per fetched attempt; no local versioning machinery.
- Attempt-associated earlier responses and existing per-question scores/comments as evidence. Select histories by identity, never by assuming the last array row is the desired attempt.
- Auto-scored results and Canvas totals as context, without asking the agent to rescore objective items.
- Separately identified teacher context and readable/unreadable attachment outcomes. Use existing extraction support; no new OCR subsystem.

The execution brief resolves quiz-submission mapping and attempt-specific question reads using official API documentation and source, with answer/comment read-back grounded in OG's recorded probe. These are implementation contracts, not new CE-SE Live evidence. A genuinely ambiguous student/attempt is held individually while independent work continues.

Discovery must include submitted writing awaiting review, recognize completed requested work, retain new attempts, and permit explicit reassessment. Do not use the whole quiz's numeric grade as proof that its writing has been handled.

### 2. Classic writing: explicit results and narrow posting

Keep `validate` and `post` as entry points, with a clearly discriminated Classic result shape. Add feature-specific types/functions rather than filling an ordinary assignment grade object with quiz exceptions.

Agent-facing output should contain a stable student reference, exact attempt, and explicitly selected question IDs with an honest numeric score and student-facing comment. Private assessment rationale remains separate. The Engine does not require a new per-question rubric grammar merely to accept a score; where a real rubric applies, the agent uses it and states the mapping. Reuse existing decimal arithmetic for an explicit rubric mapping if needed, without inventing that mapping or implicitly posting an assignment rubric.

Posting mechanics:

1. Resolve the complete identity chain and confirm the intended submitted attempt is still current. Recheck selected question existence/type/maxima and the relevant grading mode. Hold stale or ambiguous targets without erasing prepared results.
2. Validate each requested score against its question maximum, including nonnegative bounds. Do not silently clamp. An unreadable response stays unassessed; ready questions and students can proceed.
3. Send only the requested question scores/comments to Classic quiz submission grading. Leave objective-item scores untouched. No assignment `posted_grade`, whole-quiz scaling, fudge points, automatic rubric write, or posting-policy change.
4. Read back the selected question results on the intended attempt. Canvas owns the resulting quiz total and its attempt-selection policy; report that total separately from the assessment of this attempt. A difference alone is not a reason to overwrite it.
5. Store small per-question outcomes alongside prepared results. After a timeout, read the exact attempt before retrying. Do not blindly overwrite a teacher edit. Confirmed completed work remains complete; explicit reassessment can replace it.

Question comments satisfy the default feedback request. Do not also manufacture an overall submission comment. If requested, reuse existing comment mechanics with separate confirmation.

Carry existing lateness behavior only through its explicit timing operation with current-attempt/live-due checks; do not invoke the assignment-grade writer to obtain it. Confirm that path on Classic during the bounded live check before claiming verified support.

The scoring Playbook should explain how to apply the actual prompt/rubric, handle missing required writing, compare prior work, and write concise per-question feedback. It should also distinguish an honest question assessment from Canvas's selected overall quiz grade. Do not automatically assign zero to missing submissions or use an existing higher grade to suppress an honest assessment.

### 3. Close the student-experience evidence gap

This is a bounded human check, not a new Engine feature or prerequisite for all development. Inspect one representative Classic writing response/comment and one Hub's intended access versus denied access. Inspect New Quiz item rendering/feedback through the supported preview or an appropriately enrolled account. Distinguish editor preview from enrolled-student access proof.

Any live experiment needs fresh, concrete authorization for the course, objects, submission identity, writes, and cleanup. Historical credentials and IDs are not authorization. Prepare the specific experiment only after the implementation and local checks make it reviewable.

## Proportionate validation and stopping rule

Extend existing workflow fixtures. Test externally meaningful behavior, not internal helper structure or exact incidental request counts.

- Scoring: a mixed quiz retains objective-item scores, writes selected writing scores/comments, and confirms them through read-back.
- Consequential boundaries: wrong/stale identity, changed question maximum, invalid score, and one unreadable response alongside ready work.
- Recovery: an uncertain write that actually landed and a conflicting live change; neither causes blind replay or loss of unsent work.
- Discovery: pending manual work, new attempts, and explicit reassessment remain reachable.

These are behaviors to cover with compact existing-style fixtures, not a required test count or cross-product matrix. Run focused changed-package tests while developing, then the ordinary build/vet/relevant suite once for integration. Broaden only for a concrete failure or unresolved concern.

If authorized, use one small mixed Classic quiz for live score/comment/read-back and student presentation, adding a second attempt only to resolve the actual identity/total-policy question. Reuse historical evidence for unchanged authoring endpoints. No all-types rerun, all-bank import sweep, simulated district certification, or permanent fault-injection apparatus.

Stop when the supported workflow works, consequential boundaries are checked, and limitations are stated. Keep Canvas Live evidence, local checks, and pending human validation distinct in `internal/localui/capabilities.go` and `docs/engine-capabilities.md`; update the review date and matching formats/Playbooks with each delivered change. Seed Playbook updates must not overwrite existing teachers' customized files.

## Deferred until a teacher task needs them

Images in questions, attaching Hub tiers to existing content, a later-publish Hub helper, module/group resolution by name, TEKS coverage reporting, and broader real-bank compatibility. A later-publish helper may be valuable, but it should be a direct action on known pages, not a background reconciler.

No Bridge revival, New Quiz writing/scoring or Hub, QTI import revival without new evidence, generic family contract, operation ledger, mirror/checkpoint system, universal policy schema, new agent protocol, or broad OG parity effort.

The execution brief is ready for a Sol implementation owner, with an optional subsequent Luna documentation pass. Leave quiz resume unchanged; teacher-directed correction or recreation of a disposable authoring mistake is sufficient when no student work would be lost. Implementation has not been dispatched. Review against the teacher workflow and this boundary rather than rewarding extra infrastructure.

## Source basis

- CE-SE: `AGENTS.md`, `ARCHITECTURE.md`, `SCORING-RULES.md`, `handoffs/active/QUIZZES-HUB-CODEX.md`, relevant workflow/domain source, `docs/engine-capabilities.md`, and scoring/quiz Playbooks.
- OG CE: `docs/reference/classic-quiz-design.md`, `docs/reference/assignment-differentiation-design.md`, and `api/README.md` recorded Canvas findings. Historical observations were read locally; no new Canvas/API behavior was verified for this plan.

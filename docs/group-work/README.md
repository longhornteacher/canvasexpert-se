# Group work implementation plan

Date: 2026-10-17. Status: development specifications; no slice is implemented by this plan. The user requested enough detailed slices to implement the group-work capabilities identified in [discovery](../book-club-group-work-discovery.md). These documents are coding handoffs for Dev. They do not authorize Canvas Live experiments, classroom writes, or changes to UserInstall.

Read [shared contracts](contracts.md), then the assigned slice. Follow [AGENTS.md](../../AGENTS.md), [ARCHITECTURE.md](../../ARCHITECTURE.md), and [SCORING-RULES.md](../../SCORING-RULES.md). Each slice includes its CLI/MCP surface, document changes, integration work, consequential checks, and acceptance scenarios. Names and schemas below are proposed implementation contracts, not commands that already exist.

## Product result

A teacher can establish book clubs, let students choose or assign memberships, supply shared working spaces, hold and assess discussions, collect shared products, give group or individual grades, arrange peer critique and conferences, and communicate within the intended audience. The agent handles instructional decisions; the Engine handles Canvas, identity, arithmetic, and scoped writes. Ordinary individual assignments and New Quizzes continue to work.

## Slice map, tiers and dependencies

Tiers say how the work is scheduled. **A** and **B** are scheduled. **C** is built only when the teacher asks for that capability; nothing in A or B depends on it.

| Slice | Tier | Deliverable | Requires |
|---|---|---|---|
| [01](01-groups.md) | A | Read, create, and edit collaborative group sets and clubs | Existing Engine |
| [02](02-memberships.md) | A | Membership plans, self sign-up, and optional even allocation | 01 |
| [03](03-group-pages.md) | A | Member-editable group Pages and revision reads | 01 |
| [05](05-discussion-authoring.md) | A | Discussions and announcements: 05a ungraded/distributed, 05b linked graded | 01 |
| [06](06-discussion-evidence.md) | A | Complete attributed discussion evidence and teacher replies | 01; 05 contracts (existing topics may be used) |
| [16](16-navigation-and-external-resources.md) | A | Discussion/external module links, collaboration/conference reads | 01; 05 for authored discussion links |
| [17](17-book-club-playbook.md) | A | Playbook, grown per phase (v1 after Phase 1) | Slices delivered so far |
| [08](08-group-assignment-authoring.md) | B | Group assignment creation, scoped overrides; **owns the shared snapshot foundation** | 01 |
| [09](09-group-submissions.md) | B | Shared-artifact pulls with per-member submission identity | 01, 08 |
| [10](10-shared-assessment.md) | B | Shared grades/rubrics and explicit group/private feedback | 09, **Gate P** |
| [11](11-individual-group-assessment.md) | B | Individual grades on a shared product | 09, 10, Gate P |
| [07](07-discussion-assessment.md) | B | Individual discussion assessment through existing review/post | 06, 08 (snapshot foundation) |
| [12](12-peer-review.md) | C | Peer-review settings, allocation, and completion evidence | 01, 02, 08; 09 for shared products |
| [13](13-group-calendar.md) | C | Group meeting events and edits | 01 |
| [04](04-group-files.md) | C | Group folders and file upload receipts | 01 |
| [14](14-group-appointments.md) | C | Group conference slots and reservation reads (Scheduler availability unverified; published slots cannot be unpublished) | 01 |
| [15](15-club-messaging.md) | C | Explicitly scoped Canvas Inbox messages (high consequence, low need; the teacher can message in Canvas) | 01, 02 |

## Phases

Slices run **one at a time**; see Execution. Each phase ends usable.

1. **Phase 1, working clubs and conversations:** 01 → 02 → 03 → 05a → 06 → 16 → playbook v1 (17, covering only what exists). Writes are additive and easy for the teacher to inspect. 05b (graded linked discussions) follows once Phase 1 has been used, or alongside 07.
2. **Gate P, one authorized live check** (below), before any Phase 2 code that depends on propagation.
3. **Phase 2, shared products and assessment:** 08 → 09 → 10 → 11, then 05b and 07. Update the playbook as each lands.
4. **Phase 3, on request only:** 12, 13, 04, then 14 and 15. Their full specs stay here so a later request starts from a plan, not from scratch.

## Gate P: live propagation check

Slices 10 and 11 rest on Canvas source reading (representative-member route, `group_members` audience, group vs individual rubric propagation, `comment[group_comment]`), not on observed behavior of the deployed Canvas. Before coding 10/11, run one small check **only on a disposable object the teacher authorizes**, never on a historical course ID:

- one disposable group assignment, two or three test students in one group set, one rubric;
- shared mode: does a grade written through one member propagate to all; does a rubric assessment propagate; what does `group_members` return for the representative; does `group_comment=true` reach all members and `false` only one;
- individual mode: does a grade or rubric reach only the named student.

Record what was observed as Canvas Live evidence in [engine-capabilities.md](../engine-capabilities.md) and `internal/localui/capabilities.go`. If observed behavior differs from upstream, revise slices 10 and 11 before implementing them. Slices 08 and 09 do not need the gate. No student data enters Dev fixtures. Whether to run Gate P, and on what course, is the teacher's call; implementers do not start it unprompted.

## Boundaries

- Tier A and B surfaces are scheduled deliverables, including self sign-up and individual group grades. Tier C (peer critique, calendars, group files, conference scheduling, Inbox messaging) is fully specified but built only on the teacher's request; an implementer does not start it as a side effect of another slice.
- Do not provision Google/Microsoft documents or video meetings through invented Canvas REST writes. Slice 16 covers discovery and links; provider creation remains a separate integration. The public resources document reads: [Collaborations](https://developerdocs.instructure.com/services/canvas/resources/collaborations), [Conferences](https://developerdocs.instructure.com/services/canvas/resources/conferences).
- No group quizzes, collaborative real-time editor, permanent contribution score, role-management service, new general job engine, generic HTTP-write tool, or database. Preserve existing New Quiz support.
- Do not delete clubs containing student work, replace student Pages, shuffle active groups, or send announcements/messages merely because a unit is being assembled. Specific teacher scope controls these actions. Empty draft-object cleanup can remain teacher-directed in Canvas; automated deletion is not required for this unit.
- Discussion checkpoints are a separate Canvas grading variant. Slice 07 detects them and reports a precise limit without blocking ordinary discussions or adding checkpoint support to this plan.

## Execution and completion

Run slices **serially, one implementer at a time**. Every slice edits the same shared files: `cmd/cse/main.go`, `cmd/cse/remote.go`, MCP registration and schemas, the MCP smoke check, `engine-docs/formats.md`, `internal/localui/capabilities.go` and `docs/engine-capabilities.md`. Each slice lands (merged or committed, checks green) before the next starts. Parallel agents are allowed only for slices that share none of these edits, which in practice means none; do not use a worktree split to avoid the sequence. Each slice updates the capability status for its own scope at the end.

For an assigned slice, implement its domain/workflow functions, CLI and MCP adapters, required review rendering, examples and seed guidance together. Use the existing command switches and feature functions. Avoid a preliminary framework slice. Each implementer records files changed, focused checks run, actual outcomes, and remaining evidence limits in their handoff. Record a slice's implementation status only from completed work, not intentions.

Reusable coding-agent handoff:

> Implement the assigned slice in `docs/group-work/`, reading its shared contracts and the repository's current instructions first. Verify its dependencies in the actual code; a specification file is not proof that a dependency was implemented. Complete the slice's CLI/MCP/review/documentation integration and meaningful acceptance checks. Preserve existing individual, Hub and New Quiz behavior. Do not expand into unrelated slices or run Canvas Live writes without a concrete authorized scope. Report what works, the checks run, actual partial/held outcomes, and evidence still pending. Update the capability records with the real implementation status.

A slice's acceptance list names the **protections** to verify: wrong-target writes, student/unsent work, honest grades, feedback audience, and compatibility with existing behavior. It is not a test matrix. Cover each listed protection with a proportionate check that exercises the real operation path; do not add tests for resume, duplicate or cosmetic cases the teacher would see and cheaply repair unless a bullet says so.

Run focused existing tests and meaningful new regressions for the changed behavior. Run broader checks when integration warrants them; do not create quota-driven matrices. Update `engine-docs/formats.md`, `engine-docs/field-limits.md` where applicable, `docs/mcp-adapter.md`, and the packaged MCP smoke expectations for changed surfaces. Keep `internal/localui/capabilities.go` and `docs/engine-capabilities.md` aligned and dated.

An authorized live check is useful evidence, not an automatic prerequisite for ordinary use of an implemented supported path. Grade propagation, feedback audience, and student access deserve explicit observation when authorized because local fakes cannot establish Canvas behavior. Do not run probes against historical course IDs just because credentials are available.

The final integrated example uses at least two clubs with different books, multiple members, an unassigned student, a common graded discussion, separate club notebook Pages, a shared product and individual reflections, plus any Tier C features actually built. It exercises resumption from actual partial outcomes and scope preservation; it is a focused example, not a certification system.

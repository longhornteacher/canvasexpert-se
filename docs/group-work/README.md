# Group work implementation plan

Date: 2026-10-17. Status: development plan with Phase 1 slices 01, 02, 03, 05a, 06, 16 and playbook v1 implemented in Dev; later planned slices are not implemented by this plan. The teacher requested enough detailed slices to implement the group-work capabilities identified in [discovery](../book-club-group-work-discovery.md). These documents are coding handoffs for Dev. They do not authorize Canvas Live experiments, classroom writes, or changes to UserInstall.

Read [shared contracts](contracts.md), then the assigned slice. Follow [AGENTS.md](../../AGENTS.md), [ARCHITECTURE.md](../../ARCHITECTURE.md), and [SCORING-RULES.md](../../SCORING-RULES.md). Each slice includes its CLI/MCP surface, document changes, integration work, consequential checks, and acceptance scenarios. Names and schemas below are proposed implementation contracts, not commands that already exist.

## Implementation status

| Slice | Status | Local evidence | Canvas Live / student evidence |
|---|---|---|---|
| 01 — collaborative group sets and clubs | Implemented in Dev | Focused domain/workflow checks cover exact IDs, collaborative-only listing, pagination, baseline protection, and persisted IDs on resumed set/club creation. Compiled MCP smoke discovers all three group tools. | No CE-SE Canvas writes or student club access were tested. |
| 02 — memberships, self sign-up, and allocation | Implemented in Dev | Focused tests cover stable-ref/alias-only roster reads, unassigned students, submitted-work holds on moves/removals, per-row outcomes, synchronous allocation response mapping without a success read-back, privacy of the returned response, and reconciliation of uncertain allocation. Compiled MCP smoke discovers the membership baseline tool. | No CE-SE Canvas membership changes or allocations were run; student-facing sign-up and membership access remain unobserved. |
| 03 — shared group Pages | Implemented in Dev | Local operation-path checks cover exact group routing and identity, member role mapping, unpublished-first creation, no post-write confirmation read, revision metadata/body reads, unsent body conflict preservation, and title-only updates that preserve live body. Compiled MCP smoke discovers the revision-history tool. | No CE-SE Canvas Page writes were run; group editor behavior and student access remain unobserved. |
| 05a — ungraded discussions and announcements | Implemented in Dev | Local checks cover course/distributed/direct-group scope, collaborative group identity, returned child topic IDs, pulled edit baselines, settings-only patches, explicit announcement intent, and clear rejection of linked graded discussions. Compiled MCP smoke discovers `pull_discussion`. | No CE-SE Canvas discussion writes or student-view checks were run. |
| 06 — conversation evidence and teacher participation | Implemented in Dev | Local checks cover complete nested reply pulls beyond the embedded preview, newest-version merge by source identity, mapping only through existing private student references (unmapped authors remain unknown; no broad roster read), no names or Canvas user IDs in packets, exact distributed child scope, accepted reply ID persistence without read-back/replay, and cross-topic parent refusal. Compiled MCP smoke checks the expanded `pull_discussion` contract. | No CE-SE discussion pulls or teacher replies were run against Canvas Live; full-view availability, restricted topics, attachment access, teacher/student rendering and notifications remain unobserved. |
| 16 — unit navigation and external resources | Implemented in Dev | Local checks cover native course Discussion IDs, same-course group URL guards, ordered/destination-aware ExternalUrl resume, collaboration/conference pagination and envelope normalization, unavailable URLs, and stable-ref-only participants. Compiled MCP smoke discovers the external-resource tool. | No CE-SE module writes or external-resource reads were run against Canvas Live; module/student visibility, teacher-selected provider access and conference link usability remain unobserved. No provider provisioning or content/recording reads are supported. |
| 17 v1 — Phase 1 book-club playbook | Implemented in Dev seed | Teacher routing and authoring/scoring/native-content references point to the scoped workflow; this is instruction content, not a new Engine operation. | No additional Canvas evidence; the playbook states current evidence limits and does not claim student access. |

## Product result

The Phase 1 product result is establishing book clubs, managing memberships, supplying shared group Pages, authoring ungraded discussions and explicit announcements, reviewing written discussion evidence, posting an explicitly requested teacher reply, and linking the resources in course modules. Shared group assignment assessment, graded discussions and later optional group tools remain planned work. Ordinary individual assignments and New Quizzes continue to work.

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
2. **Gate P, one authorized live check** (below), before any Phase 2 code that depends on propagation. **Run 2026-10-17; results below.** Contracts for 10 and 11 stand; late timing, excused/hidden members and real submissions remain unobserved.
3. **Phase 2, shared products and assessment:** 08 → 09 → 10 → 11, then 05b and 07. Update the playbook as each lands.
4. **Phase 3, on request only:** 12, 13, 04, then 14 and 15. Their full specs stay here so a later request starts from a plan, not from scratch.

## Gate P: live propagation check

Slices 10 and 11 rest on Canvas source reading (representative-member route, `group_members` audience, group vs individual rubric propagation, `comment[group_comment]`), not on observed behavior of the deployed Canvas. Before coding 10/11, run one small check **only on a disposable object the teacher authorizes**, never on a historical course ID:

- one disposable group assignment, two or three test students in one group set, one rubric;
- shared mode: does a grade written through one member propagate to all; does a rubric assessment propagate; what does `group_members` return for the representative; does `group_comment=true` reach all members and `false` only one;
- individual mode: does a grade or rubric reach only the named student;
- group discussion: for a graded group discussion, does the linked assignment return a non-null `group_category_id`? It decides whether slice 07 must distinguish a discussion-linked assignment from a shared product.

Results are recorded below. Record what was observed as Canvas Live evidence in [engine-capabilities.md](../engine-capabilities.md) and `internal/localui/capabilities.go`. If observed behavior differs from upstream, revise slices 10 and 11 before implementing them. Slices 08 and 09 do not need the gate. No student data enters Dev fixtures. Whether to run Gate P, and on what course, is the teacher's call; implementers do not start it unprompted.

## Gate P results (2026-10-17)

Run on CS8 (course 121046) with raw API calls from a throwaway probe, not CE-SE. Disposable objects (a group set, one group, a group assignment with a one-criterion rubric, a graded group discussion) were created and then deleted. The group briefly held two real students; the assignment was published only to that group. Those two students may have received Canvas notifications (group membership, assignment, three submission comments). No real submission was made, so student view, lateness, excused/hidden members and real attempts were **not** observed.

| Question | Observed |
|---|---|
| Built-in Test Student in a group | Refused: `POST /groups/:id/memberships` returned 403; `PUT /groups/:id/users/:id` 404. Group tests need real or separately enrolled students. |
| Shared mode (`grade_group_students_individually: false`): grade via one member | Propagated: the other member showed the same `score`/`entered_score`. |
| Shared mode: rubric assessment via one member | Propagated to the other member with the same points and criterion comments. |
| `comment[group_comment]=false` | Only the named member received it. |
| `comment[group_comment]=true` | Both members received it, as separate comments with **different comment IDs**. One returned ID does not identify all copies. |
| Individual mode: grade and rubric | Each reached only the named student. |
| Switching mode after grades existed | Canvas accepted the `PUT` (no real submission existed). Any hold on structural changes after submitted work is CE-SE policy, not a Canvas rule. |
| `GET .../assignments/:id/users/:uid/group_members` | Returned both members in either mode as `[{id: "<string user id>", name}]`. The `id` is a **string** and `name` is present, so normalize and keep names out of packets. |
| Graded group discussion | Linked assignment returned `group_category_id: null`, `grade_group_students_individually: false`, `has_sub_assignments: false`, `submission_types: [discussion_topic]`, and `checkpoints: []` with `include[]=checkpoints`. The topic returned `group_category_id` and `group_topic_children` `[{group_id, id}]` immediately. |
| Unexplained | Grade `PUT`s returned 403 "user not authorized" on a member with no existing submission record, twice, including after publishing. After a comment write on that member succeeded, the same grade `PUT` succeeded. Cause not determined. The Engine should treat a 403 on a grade write as a specific hold and not retry blindly. |

Consequences: the representative-member route and the group-comment flag contracts in slices 10 and 11 stand. Slice 07's open risk is resolved: ordinary guards do not misclassify a graded group discussion. Not yet observed: late timing, excused/hidden members, actual shared-artifact submissions, and student-facing views.

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

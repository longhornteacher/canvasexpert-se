# Group work implementation plan

Date: 2026-10-25. Status: Tier A and B slices 01-03, 05a/05b, 06-11, 16 and the book-club playbook are implemented in Dev. The CS8 dev Engine check on 2026-10-17 exercised the specific paths recorded below; the October 19 regression fixes have local evidence only. Tier C remains frozen until requested. The existing teacher Inbox has a separate, explicitly requested read/reply/state implementation; new club conversation creation remains frozen. These documents are coding handoffs for Dev and do not authorize Canvas Live experiments, classroom writes or UserInstall changes.

Read [shared contracts](contracts.md), then the assigned slice. Follow [AGENTS.md](../../AGENTS.md), [ARCHITECTURE.md](../ARCHITECTURE.md), and [SCORING-RULES.md](../SCORING-RULES.md). Each slice includes its CLI/MCP surface, document changes, integration work, consequential checks, and acceptance scenarios. Proposed contracts are implemented only where the status table says so.

## Implementation status

| Slice | Status | Local evidence | Canvas Live / student evidence |
|---|---|---|---|
| 01 — collaborative group sets and clubs | Implemented in Dev | Focused domain/workflow checks cover exact IDs, collaborative-only listing, pagination, baseline protection, and persisted IDs on resumed set/club creation. Compiled MCP smoke discovers all three group tools. | CS8 dev Engine, 2026-10-17: set/club create/read/pull/update. October 19 local regressions preserve unrelated live settings and explicit null clears. Student club access remains unobserved. |
| 02 — memberships, self sign-up, and allocation | Implemented in Dev | Focused tests cover stable-ref/alias-only roster reads, pending requests with exact group identity and unresolved-author holds, unassigned students, per-row pending-request holds, submitted-work holds on moves/removals, accepted-write no-replay, uncertain move/allocation reconciliation, synchronous allocation response mapping without a success read-back, and privacy of the returned response. Compiled MCP smoke discovers the membership baseline tool. | CS8 dev Engine, 2026-10-17: membership add, move and remove for two students. Self sign-up/allocation and student access remain unobserved. |
| 03 — shared group Pages | Implemented in Dev | Local operation-path checks cover exact group routing and identity, member role mapping, unpublished-first creation, no post-write confirmation read, revision metadata/body reads, unsent body conflict preservation, and title-only updates that preserve live body. Compiled MCP smoke discovers the revision-history tool. | CS8 dev Engine, 2026-10-17: member-editable group Page create/pull/edit/unpublish and revision history. Student editor/access behavior remains unobserved. |
| 05a — ungraded discussions and announcements | Implemented in Dev | Local checks cover course/distributed/direct-group scope, collaborative group identity, returned child topic IDs, pulled edit baselines, settings-only patches, explicit announcement intent, and clear rejection of linked graded discussions. Compiled MCP smoke discovers `pull_discussion`. | CS8 dev Engine, 2026-10-17: distributed and direct-group discussions. Student views and notifications remain unobserved. |
| 06 — conversation evidence and teacher participation | Implemented in Dev | Local checks cover complete nested reply pulls beyond the embedded preview, newest-version merge by source identity, mapping only through existing private student references (unmapped authors remain unknown; no broad roster read), no names or Canvas user IDs in packets, exact distributed child scope, accepted reply ID persistence without read-back/replay, and cross-topic parent refusal. Compiled MCP smoke checks the expanded `pull_discussion` contract. | CS8 dev Engine, 2026-10-17: whole-thread pull with child sources and a teacher reply. October 19 local regressions retain fallback recent_replies and relevant source coverage. Restricted topics, attachment access, teacher/student rendering and notifications remain unobserved. |
| 16 — unit navigation and external resources | Implemented in Dev | Local checks cover native course Discussion IDs, same-course group URL guards, ordered/destination-aware ExternalUrl resume, collaboration/conference pagination and envelope normalization, unavailable URLs, and stable-ref-only participants. Compiled MCP smoke discovers the external-resource tool. | CS8 dev Engine, 2026-10-17: module SubHeader, Discussion and ExternalUrl. Student visibility, external provider metadata/access and conference-link usability remain unobserved; provisioning remains separate. |
| 17 — book-club playbook | Implemented in Dev seed | The seed playbook (`group-work.md`, with routing from `authoring.md`, `score.md` and `native-content.md`) covers every shipped slice: clubs and memberships, Pages, ungraded and graded discussions, discussion evidence and replies, module links, group assignments, shared and individual group grading, and graded discussion assessment. It is instruction content, not an Engine operation. Tier C features are not described because they are not built. | The CS8 live check exercised the Engine paths the playbook directs; teacher use of the playbook itself is unobserved. First-use checks are in [real-use-checks.md](real-use-checks.md). |
| 08 — group assignment authoring and dates | Implemented in Dev | Focused local checks cover group mode presence/compatibility, exact group-set assignment identity and resume, structural-change holds after submissions, override exact-target/baseline checks, explicit-null date clearing, privacy-safe effective-date refs and accepted-write no-replay. Compiled MCP smoke validates group assignment and assignment-dates examples. | CS8 dev Engine, 2026-10-17: group assignment, group overrides, club-only visibility and date edits. October 19 local regressions distinguish placeholders from genuine later-page submitted work. Effective-date student view and student access remain unobserved. |
| 09 — shared-artifact submission pulls | Implemented in Dev | Focused local checks cover selected-group-only submission reads, exact assignment/group/set scope, placeholder attempts remaining unknown, preserved unsent results, typed group packet/work validation, and individual completion hints. | CS8 dev Engine, 2026-10-17: real shared submission pull with one artifact, two members, complete audience and newly tracked attempts. Excused/hidden members, differing real artifacts and student views remain unobserved. |
| 10 — shared grades, rubric and feedback audience | Implemented in Dev | Focused local checks (fake Canvas through the real pull) cover one representative grade/rubric write per group, coupled rubric order, whole-group versus private comment flags, held subset/changed-audience/changed-attempt groups with no propagated write, real skip-key suppression, failed-comment resume without repeating the grade, shared-mode-only validation, and review cards that keep private text separate. | CS8 dev Engine, 2026-10-17: one shared grade/rubric reached both members, correctly scoped group/private comments without replay, and shared late timing. October 19 recovery and prepared school-time/member-provenance fixes have local checks only. Calendar-backed correction, excused/hidden members, student views and notifications remain pending. |
| 11 — individual grades on a shared product | Implemented in Dev | Focused local checks cover per-student grades/rubrics on separate routes with explicit `group_comment=false`, attempt holds limited to one student, a live switch to shared grading holding all remaining writes, per-student and shared-comment skip keys, completion hints following the student, and mode/row compatibility validation. | CS8 dev Engine, 2026-10-17: distinct student grades/rubrics/private comments. October 19 local regressions renew mode/basis/evidence before every numeric stage and reconcile uncertain comment-only writes without replay. Per-student timing and student views/notifications remain unobserved. |
| 05b — graded discussion authoring | Implemented in Dev | Focused local checks cover the group set on the topic with an individual linked assignment, saved topic/assignment/rubric identity, rubric failure holding publication with a resume that repeats only the rubric, and validation of unsupported scopes. | CS8 dev Engine, 2026-10-17: graded distributed discussion with rubric; group set on topic and individual linked assignment; linked rename. Graded-discussion scoring, student rendering and notifications remain unobserved. |
| 07 — individual discussion assessment | Implemented in Dev | Focused local checks cover discussion pulls through pull-work, own-entry references/digests, per-student distinct grades without group_comment, single-student holds on a changed post, relink and checkpoint holds, and the result evidence requirement. | No graded-discussion scoring was run in Canvas Live. October 19 local regressions require complete relevant evidence at pull and post before assessing missing work; independently complete clubs continue. Checkpoints, student rendering and scale remain unobserved. |

## Product result

The delivered Tier A/B product establishes clubs and memberships, shared Pages, ungraded and graded course/distributed discussions, explicit announcements/replies and module links; authors group assignments and scoped dates/audiences; pulls source-linked shared products; posts shared or individual group assessments with separate feedback audiences; and individually assesses written discussion evidence. Ordinary individual assignments and New Quizzes continue through their existing paths. The separately implemented Inbox surface handles existing teacher conversations only; it does not create club threads. Recorded CS8 evidence and pending observations are scoped above; the remaining Tier C work remains frozen.

Book-club emails to families use the separate [Family Contact flow](../family-contact-flow.md), including per-book versions, inline covers and attached reading schedules. They use the teacher's private Outlook review page and do not require Canvas Inbox messaging. The corresponding seed guidance is `playbooks/family-contact-book-clubs.md`.

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
| [15](15-club-messaging.md) | C | Create new club conversations with explicit private-each or shared-thread audiences; existing teacher Inbox read/reply/state is implemented separately | 01, 02 |

## Phases

Slices run **one at a time**; see Execution. Each phase ends usable.

1. **Phase 1, working clubs and conversations:** 01 → 02 → 03 → 05a → 06 → 16 → playbook v1 (17, covering only what exists). Writes are additive and easy for the teacher to inspect. 05b (graded linked discussions) follows once Phase 1 has been used, or alongside 07.
2. **Gate P, one authorized live check** (below), before Phase 2 code depending on propagation. **Completed 2026-10-17; results below.** The later dev Engine check also exercised real shared submissions and shared timing. Excused/hidden members and student views remain unobserved.
3. **Phase 2, shared products and assessment:** 08 → 09 → 10 → 11, then 05b and 07. Update the playbook as each lands.
4. **Phase 3, frozen (maybe nice for later):** 12, 13, 04, then 14 and 15. Nothing is planned or scheduled. Their full specs stay here so a teacher request can start from a plan, not from scratch.

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

## Live results (2026-10-17, CS8)

Live check, 2026-10-17 on CS8 (course 121046) with the dev Engine, disposable `[CSE 1.1b DISPOSABLE]` objects only, all deleted afterwards: group set and club create/read/pull/update; membership add, move and remove for two students; a member-editable group Page create, pull, edit, unpublish and revision history; ungraded distributed and direct-group discussions, a graded distributed discussion with rubric whose group set stayed on the topic while the linked assignment stayed individual, linked-topic rename syncing the assignment, whole-thread pull with child sources, and a teacher reply; a module with a SubHeader, Discussion and ExternalUrl; a group assignment with group overrides, visibility restricted to the club, and date edits; a real shared submission pull (status ready, one artifact, two members, complete audience, new attempts tracked); a shared grade and rubric that reached both members once, group and private comments with the right audience and no replay on rerun, plus a shared late timing write; an individual-mode assignment with different grades, rubrics and private comments per student; and the local review page rendering group and private cards.

Observed differences and limits:

- Canvas returns `discussion_topic_id: null` on an assignment read; the linked topic ID is `discussion_topic.id` with `include[]=discussion_topic`. The Engine reads both.
- A teacher's own discussion entry comes back with `author_kind: unknown`, never as a student.
- Canvas's course late policy computed the late deduction after the Engine wrote the entered grade and `seconds_late`; the Engine did not compute a penalty.
- Canvas returns whole-number points as `20.0`; feedback text now prints `20`.
- Submissions were made on behalf of two real students on disposable, club-only assignments through a dev-only helper (`tmp/cs8-group-live`); the Engine has no submit command. A first grade on a member with no submission remains blocked in the Engine (no attempt), consistent with the Gate P 403.
- Not exercised live: self sign-up and even allocation (they would touch the whole class), graded-discussion assessment (needs a student's own post; the Test Student discussion was created and deleted unused), checkpointed discussions, excused/hidden members, student-facing views and notifications.
- Review page: the CLI opened the loopback page in the teacher's browser, and the teacher approved it within seconds (cards are approved by default), which explains the posts that looked automatic. A second live run, with nobody clicking, confirmed the page waits: one private card was marked Revise through the page, and only the shared grade, rubric, shared comment and the other private comment reached Canvas. The Revise card was reported back and nothing was posted for it.

The checks to make the first time each feature is used with real students are in [real-use-checks.md](real-use-checks.md).

## Boundaries

- Tier A and B surfaces are scheduled deliverables, including self sign-up and individual group grades. Remaining Tier C (peer critique, calendars, group files, conference scheduling and creating club conversations) is frozen as "maybe nice for later": specified, not scheduled, built only if the teacher asks; an implementer does not start it as a side effect of another slice. The existing teacher Inbox is separate and does not enable creating new club threads.
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

## Standing teacher preferences (carried from the retired handoff) (these come from the teacher; they are not in any agent's memory)

- **No read-backs.** An accepted Canvas write (no HTTP error) is done. Do not confirm by shape of the reply, do confirmation GETs after writes, or print "partial/uncertain" noise for writes that landed. Keep cheap *pre-write* guards: course/student/attempt/group identity, wrong target, duplicates, audience. Do not add retry or verification machinery after writes. A 408/5xx/redirect on a write is uncertain: do not blindly replay.
- **Proportionality.** Do not add coverage quotas, certification rituals, universal operation frameworks or speculative migrations. A conceivable failure is not a reason to add code; ask whether the teacher would notice it and what recovery costs. Keep cheap checks for wrong-target writes, privacy exposure, dishonest grades and loss of student/unsent work. A slice's acceptance list names protections, not a test matrix.
- **Honest grades.** Post the computed assessment even when it is lower than an entered grade. Never calculate a late penalty in CE-SE. No contribution multipliers. No automatic zeros without complete relevant evidence.
- **Canvas Live is authoritative.** Preserve teacher intent and unsent work. Protect credentials and teacher-only notes. Respect requested posting scope.
- **Teacher-facing copy** (review cards, WebUI text, Playbook seed text, error messages the teacher sees): a short label plus one plain sentence that names specifics. Warm but plain: no jokes, no exclamation marks, no internal jargon in primary text. Consequential actions get a brief explanation by the action. Honesty limits unchanged.
- **Installation constraints.** The district locks work laptops (no admin, no installs). District OneDrive blocks `.exe`, `.js`, `.py` and similar, so never design anything that relies on syncing executables or scripts, and never put the CE-SE folder in OneDrive. UserSync (optional cloud folder) is for the agent app's data only.
- **Three-step bar.** Setup and any new teacher workflow must be agent-driven in conversation; never require a teacher to hand-edit files.
- **Docs honesty.** Keep Canvas Live evidence, local tests and pending human validation distinct. Local tests alone are never live proof. When a capability, limitation or status changes, update `internal/localui/capabilities.go` and `docs/engine-capabilities.md` together with a review date. Keep `engine-docs/` contracts and `workspace-template/` aligned with behavior, and add notable user-visible changes to `CHANGELOG.md`.

## Regression resolution (2026-10-19)



The [October 17 follow-up](REVIEW-2026-10-17-followup.md) and [October 19 review](../REVIEW-2026-10-19.md) now have dated resolution records. All nine Engine failures are fixed and have tracked normal-suite regressions; the seed guidance covers delivered graded discussions, shared-only readiness and separately selectable individual cards. The changes renew individual numeric-action mode checks, retain fallback replies and relevant fresh coverage, preserve uncertain comment/timing outcomes, protect parent feedback, align local result discovery and keep authoring edits scoped. No new Canvas Live or student/client evidence is added, and no UserInstall/UserSync adoption is claimed.

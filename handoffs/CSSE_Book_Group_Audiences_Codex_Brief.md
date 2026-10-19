# Implement book-group audiences for individual Canvas content in CS:SE

## 1. Task and intended outcome

Implement a complete Engine workflow for assigning individual Canvas content to the students who currently belong to selected collaborative book groups.

The teacher should be able to say:

“Create the Ghost calendar Page, individual reading response, and New Quiz for the members of Book Club → Ghost. Put them in the existing Book Club module.”

CS:SE should resolve that group’s members, apply the appropriate Canvas student audience restrictions, and preserve independent submissions and individual grades.

This is a development task. Inspect the repository, implement the necessary changes, add meaningful tests, update the relevant documentation and agent guidance, and report the results. Do not stop with a proposal if the implementation can be completed in this repository.

Audit existing functionality first. Some required capabilities already exist. Reuse them rather than adding parallel implementations or describing an existing capability as missing.

## 2. Important conceptual distinction

Keep these concepts separate throughout schemas, commands, tools, documentation, and tests:

- Collaborative Canvas groups: the teacher’s Book Club groups, used for shared work and separate discussions.
- Individual-content audiences: students allowed to access an ordinary course Page, individual assignment, or quiz.
- Differentiation tags: non-collaborative groupings that Canvas can also use for content targeting when enabled.

For this feature, collaborative group membership is a source for an individual audience.

Selecting Book Club → Ghost must NOT automatically turn an individual assignment into a native group assignment. It must NOT make a quiz a shared group submission. “Grade group members individually” still describes a group assignment and is not a substitute for independent student submissions.

The primary implementation must work without requiring the teacher to maintain a duplicate set of differentiation tags.

## 3. Environment and working rules

The observed runnable UserInstall is:

C:\Users\adamb\CanvasExpert-SE

This is not necessarily the development checkout. Start in the repository opened for this Codex session and follow its applicable AGENTS.md. Locate the actual development source before editing. Do not assume that patching a runnable installation or generated binary is the right development workflow.

Observed Engine baseline on October 19, 2026:

- CanvasExpert-SE 1.1.0-beta.1
- Revision: 56109c689258
- Built: 2026-10-19T14:34:53Z
- Loaded adapter reported the same revision.

Treat this as an observed baseline, not a requirement to reset the development checkout to that revision.

Preserve these architectural boundaries:

- Canvas access and private student identity resolution stay inside Engine.
- Agent-facing documents use existing stable student references, not real student names, logins, SIS IDs, contact details, or raw Canvas user IDs.
- Respect UserInstall versus the active UserSync rules authority.
- Do not inspect private .cse contents or credentials through agent/file tools.
- Preserve existing drafts, saved IDs, receipts, uncertain outcomes, and teacher edits.
- Maintain the existing review/direct-write policy. Do not introduce a new universal approval gate for routine Pages or modules.
- This development task does not request migration, deletion, or republishing of real classroom materials. Use existing repository test conventions and approved test scopes. Any production migration should remain a separate, explicit teacher action.

## 4. What already exists: verify before changing

The current Engine reference describes:

1. Collaborative group-set and group discovery.
2. Group membership pulls using stable student references and observed baselines.
3. Group membership edits with per-row outcomes and submitted-work protections.
4. Individual assignment creation with student/date override support.
5. An assignment-dates document with exact assignment identity, student_refs, override baselines, visibility changes, and persisted outcomes.
6. Ordinary course Pages and group Pages.
7. Restricted Hub support Pages using differentiation tags.
8. New Quiz creation with saved assignment identity and resumable sidecars.
9. Native module assembly with Page, Assignment, Discussion, File, ExternalUrl, and SubHeader items.
10. Linked graded discussions for course topics and distributed group discussions.

Do not rebuild those features unnecessarily.

Specifically investigate whether the existing assignment-dates machinery can already target the assignment associated with a New Quiz. Its absence from the quiz-creation options does not prove that the underlying Engine cannot perform the operation afterward.

For each proposed gap, distinguish:

- Already implemented and reachable through an existing workflow.
- Implemented internally but missing from authoring, review, CLI, or MCP integration.
- Actually missing.
- Blocked by a Canvas constraint rather than an Engine omission.

Produce a concise capability map early, then proceed with implementation.

## 5. Product requirements

### 5.1 Resolve individual audiences from collaborative groups

Support one or more exact collaborative groups from an exact configured course and group set.

The resolver must:

- Validate Canvas origin, course, group-set identity, and each selected group’s identity.
- Distinguish collaborative groups from differentiation tags.
- Resolve membership through the existing Engine privacy layer.
- Use current eligible course-enrolled members according to established membership/enrollment semantics.
- Explicitly handle accepted, invited, pending, inaccessible, and unresolved memberships.
- Form a deterministic, deduplicated union when multiple groups are selected.
- Record whether the audience is complete and when it was observed.
- Never silently substitute a similarly named group from another set or course.

Names may assist discovery and display. Exact observed IDs establish identity.

An empty group is a valid classroom condition. The teacher may create content before assigning students. It must result in a clear empty-audience/draft state, not a fallback to Everyone.

An incomplete audience must not be silently treated as complete.

### 5.2 Ordinary individual assignments

Allow an ordinary assignment to use members of selected book groups as its individual audience.

Reuse the existing override and only-visible-to-overrides implementation where appropriate.

Preserve:

- Independent submissions and individual grades.
- Submission types, points, rubrics, dates, and publication intent.
- Existing unrelated teacher-authored audience/date overrides.
- Presence-aware updates: omission preserves; an explicitly supported clear operation clears.

Do not send the source book group as the assignment’s native group_category_id or group_id merely to select its members.

### 5.3 Ordinary course Pages

Add a supported audience workflow for ordinary course Pages, including create and update.

These should be actual course Pages placed as native Page module items. They must not require moving the content into a group wiki or converting it into a differentiation Hub support Page.

Requirements:

- Restrict the Page to the resolved student audience.
- Preserve teacher-only editing unless an existing supported request explicitly says otherwise.
- Preserve Page identity, body baselines, title, dates, module placement, and existing content-conflict behavior.
- Keep audience-only edits independent of unrelated content edits where the current architecture permits.
- Do not overwrite newer live Page text while changing its audience.
- Distinguish Page availability dates from assignment due dates. Do not invent graded due dates for Pages.
- Do not treat CSS hiding, a module label, or a group link as an audience restriction.

Existing group Pages and Hub Pages must continue to work under their own semantics.

### 5.4 New Quizzes

Integrate the same individual-audience concept into New Quiz authoring and updates.

Use the quiz’s actual associated assignment ID and reuse existing assignment audience machinery where it is valid.

Requirements:

- Each targeted student takes their own quiz attempt and receives an individual grade.
- Audience setup is included in the real authoring/review/push lifecycle, not left as an undocumented manual step.
- Preserve the existing quiz sidecar and all accepted item/resource identities.
- Publication waits until the intended audience restriction and all other required creation stages succeed.
- An audience failure must not leave a newly created quiz published to everyone.
- Audience-only changes must not recreate the quiz, recreate questions, reset attempts, or alter settings.
- Preserve due dates, availability dates, attempt settings, result visibility, question content, and points unless explicitly changed.
- Confirm the assignment-side behavior through code inspection and meaningful tests; do not infer it solely from quiz endpoint names.

If the existing Engine already provides a working multi-step path, make it explicit, reliable, documented, and consistently available to CLI/MCP callers rather than implementing a redundant path.

### 5.5 Shared Book Club module

The main use case is one shared Book Club module per course, containing native items targeted to different book audiences.

Module assembly already exists. Reuse it.

Expected behavior to validate:

- The module remains available to the intended course audience.
- Each student receives access to their assigned book’s native Pages, individual assignments, and quizzes.
- Unauthorized direct access is restricted by the item’s actual Canvas audience.
- Module-item visibility is evaluated separately from direct item access.
- Existing module entries and ordering are preserved except for requested additions or moves.

Do not promise that module restrictions alone secure every item outside the module.

Direct module audience support is a useful follow-on if it fits the same abstraction, but it is not a prerequisite for completing this one-shared-module use case. Clearly state whether it is included or deferred.

## 6. Schema and interface design

Inspect existing document contracts before choosing field names.

Prefer a small, consistent extension that separates teacher intent from Engine-owned resolution state.

Teacher intent should identify:

- Individual work semantics.
- The source collaborative group set and selected group IDs.
- Desired audience behavior.
- Existing publication and date intent.

Engine-owned state should retain, as appropriate:

- Origin and course identity.
- Resolved stable student references.
- Observation time and membership fingerprint.
- Completeness and unresolved-source status.
- Canvas override/resource identities.
- Observed audience/visibility baseline.
- Which rows or resources this document owns.
- Per-stage accepted, held, partial, or uncertain outcomes.

A conceptual shape might be:

audience:
  mode: individual
  source:
    kind: collaborative_group_members
    group_category_id: 50
    group_ids: [701]
  membership_policy: snapshot

This is an illustrative proposal, NOT an existing valid schema. Adapt the names and structure to the repository. The final examples must validate against the implementation.

Do not put this source group_category_id into the native group-assignment setting by accident.

Update all necessary layers:

- Document models and strict validation.
- Pull/create/update document generation.
- QuizForge metadata or the repository’s chosen quiz authoring contract.
- Local planning and validation.
- Review payloads and presentation.
- Push/resume logic.
- CLI and MCP descriptions and routing.
- Saved outcomes and supported examples.

The agent should not need to call raw Canvas writes or maintain a second audience manifest outside Engine.

Support existing explicit student audiences and established tag workflows without breaking them. A new generalized tag-management feature is not required for this task.

## 7. Membership changes and explicit audience refresh

Default to a recorded membership snapshot for a prepared/reviewed operation. Do not silently create a permanent live binding or background synchronization service.

Provide a clear Engine workflow to compare current group membership with an existing content audience and prepare a refresh.

The refresh must show:

- Students to add.
- Students to remove.
- Students retained.
- Unresolved membership or enrollment facts.
- Unrelated overrides that will be preserved.
- Removals held because existing work or other conditions need a decision.

Safeguards:

- Recheck relevant membership before applying a prepared audience action.
- If membership changed after preparation/review, hold that affected action and preserve the draft and resources. Follow existing conflict conventions.
- Additions and removals must not be confused with group-membership edits. Selecting an audience must not move students between groups.
- Do not unassign a student with existing submissions, quiz attempts, graded work, or other protected evidence merely because their book group changed.
- Preserve access for protected cases by default, report the held removal, and use existing explicitly scoped authorization conventions if the teacher requests that exact change.
- Never erase grades, manufacture missing scores, transfer work between assignments, or create replacement attempts as part of audience refresh.
- Do not silently remove a teacher-authored manual audience exception.
- Prevent an unchanged rerun from undoing a later teacher change that the document no longer owns.

Do not add a scheduler or require the teacher to keep two rosters synchronized manually.

## 8. Canvas API implementation notes

Verify the current primary documentation and the repository’s existing API client before implementing routes.

Relevant official references:

- [Learning Object Dates API](https://developerdocs.instructure.com/services/canvas/resources/learning_object_dates)
- [Assignments and assignment overrides](https://developerdocs.instructure.com/services/canvas/resources/assignments)
- [New Quizzes API](https://developerdocs.instructure.com/services/canvas/resources/new_quizzes)
- [Modules API](https://developerdocs.instructure.com/services/canvas/resources/modules)
- [Pages API](https://developerdocs.instructure.com/services/canvas/resources/pages)
- [Canvas module audience guide](https://community.instructure.com/en/kb/articles/660898-how-do-i-assign-a-module-to-individual-sections-differentiation-tags-or-students)

Important documented details:

- Course Pages have GET/PUT date_details routes under /api/v1/courses/:course_id/pages/:url_or_id/date_details.
- Learning-object date updates accept audience/visibility information, including only_visible_to_overrides and assignment_overrides.
- The documented date_details update replaces the supplied override collection: omitted existing rows can be deleted. An empty collection clears it.
- A successful update can return HTTP 204 without a JSON body.
- New Quiz APIs identify the associated assignment through assignment_id.
- Native assignment group overrides have group-assignment constraints; they are not a general way to assign individual quizzes to collaborative groups.

Therefore:

- Preserve unrelated rows and live baselines when using collection replacement.
- Prefer established per-row Engine operations where suitable.
- Do not infer ownership from a matching override title.
- Treat tags and collaborative groups according to their actual API semantics.
- Do not depend on a differentiation-tag account setting for the primary membership-expansion workflow.
- Handle missing permissions and unsupported deployed endpoints precisely, preserving unpublished work and reporting the actual gap.

## 9. Publication, conflicts, and resume behavior

Extend the existing state machines instead of adding a fragile second posting path.

For a new restricted item, the workflow should establish a complete audience plan before creation where possible, create initially unpublished, persist the returned identity, apply the intended restrictions/targets, complete other required setup and module placement, and publish only after required stages succeed.

Handle empty/incomplete audiences explicitly. Never recover by publishing to Everyone.

Required properties:

- Persist accepted IDs and outcomes immediately.
- Resume only genuinely unfinished stages.
- Do not duplicate resources after timeouts, transport failures, restarts, or uncertain responses.
- Do not discard a quiz sidecar, Page receipt, override ID, or create outcome to force a retry.
- Conflict checks should cover the fields and identities relevant to the requested action.
- A conflict or failure for one item should not prevent independent valid items from completing.
- Preserve source/date/body intent and unsent edits on conflict.
- A 204 response is a normal successful outcome when documented.
- Keep publication and audience outcomes distinguishable.
- An accepted write is not a reason for routine repeated confirmation reads or replay.
- Use the repository’s established visibility-verification conventions where needed before publishing restricted content. Keep checks bounded and purposeful.

Explain precisely what is proven by local tests, Canvas responses, or an actual student session. Do not equate stored settings with verified student rendering.

## 10. Review and teacher experience

Keep the teacher-facing experience simple:

“Ghost readers will each complete this independently.”

Where the existing workflow uses review, show:

- Selected group set and book group names/identities.
- Intended individual-work semantics.
- Resolved audience count and as-of time.
- Empty, incomplete, or changed-membership conditions.
- Due and availability dates.
- Publication intent.
- Add/remove/retain changes for a refresh.
- Any protected removals or manual exceptions.

Do not expose raw student IDs or private grouping rationale in student-facing content.

Preserve existing authorization rules for assignment/quiz creation, Pages, modules, and explicitly authorized direct writes. Fix missing capability rather than adding unnecessary approval steps to ordinary work.

## 11. Documentation and capability guidance

Reconcile stale guidance against actual code and observed behavior.

In the observed installation, group-work.md still says linked graded discussions are unsupported. The current formats reference supports linked graded course/distributed discussions, and two were successfully created during the classroom session.

Update the authoritative source documentation/playbook seeds and tool help to agree about:

- Existing group assignments.
- Existing graded distributed discussions.
- Group Pages versus ordinary course Pages.
- Native group work versus individual work selected from group membership.
- Explicit student overrides and book-group audience expansion.
- New Quiz audience support and remaining limits.
- Membership snapshots, refresh, publication, and recovery.

Respect the existing playbook update mechanism. Do not indiscriminately overwrite teacher-modified installed playbooks, context, or Teacher_Override.md.

Remove or resolve relevant engine-notes entries only when the capability is actually implemented and verified to the stated evidence level.

## 12. Real classroom context: reference only

Canvas instance:

https://pearland.instructure.com

| Course nickname | Course ID | Book Club set ID | Existing Book Club module |
|---|---:|---:|---:|
| LANG ARTS 7 - Beckham | 120638 | 5291 | 637696 |
| PAP LANG ARTS 7 - Beckham | 120669 | 5289 | 637697 |

| Book group | ELA 7 group ID | PAP group ID |
|---|---:|---:|
| Alone | 21121 | 21115 |
| Ghost | 21122 | 21118 |
| Last Cuentista | 21123 | 21117 |
| Mexikid / existing PAP name MexiKid | 21124 | 21119 |
| Probability of Everything | 21125 | 21116 |
| Tequila Worm | 21126 | 21120 |

Book group names intentionally omit a leading “The.”

Source materials:

C:\Users\adamb\OneDrive - Pearland ISD\Laoshi 26-27\Book Club Work 26-27

Each book has an existing Calendars/*_canvas.html student-facing fragment. Assessment master files contain teacher-only keys and rationale; do not expose those while exercising this feature.

Current work to preserve:

- Six group calendar Pages per course.
- One published shared Book Club module per course containing links to those group Pages.
- A published First Impressions distributed group discussion in each course, placed at the beginning of its Book Club module.
- ELA 7 discussion 304365, linked assignment 3705185.
- PAP discussion 304372, linked assignment 3705186.
- First Impressions participation due October 20, 2026, at 4 p.m. America/Chicago: 2026-10-20T16:00:00-05:00.
- Existing membership and student work must remain intact.

Operational notes and receipts are under:

courses/LANG ARTS 7 - Beckham/book-club-2026/
courses/PAP LANG ARTS 7 - Beckham/book-club-2026/

Each contains unit.md and the actual creation documents.

At the most recent check, ELA 7 groups were empty, PAP had members in five groups, and PAP Tequila Worm was empty. These are historical observations, not permission to populate groups or assumptions about current membership. The teacher manages membership.

No student-session access test was performed in that classroom setup. Do not upgrade that evidence claim.

The real IDs above are context for understanding and later optional migration, not hardcoded implementation constants or authorization to mutate those courses during development.

## 13. Tests and acceptance criteria

Use meaningful behavioral tests that fit the repository. Cover schemas, API payloads, state transitions, conflicts, and integration boundaries. Do not limit tests to mirroring helper implementation.

Minimum scenarios:

1. Selecting one collaborative group resolves exactly its eligible members.
2. Selecting multiple groups produces a deterministic deduplicated union.
3. A same-named group in another course/set is not substituted.
4. A differentiation tag is not mistaken for a collaborative source group.
5. Cross-origin, wrong-course, deleted, or inaccessible sources are rejected or held with useful diagnostics.
6. Empty groups do not broaden visibility or silently target Everyone.
7. Partial membership or unresolved stable references do not silently publish an incomplete audience.
8. An individual assignment remains individually submitted and graded.
9. A native course Page receives the intended student restriction without becoming a group Page or Hub support Page.
10. A New Quiz uses the correct associated assignment and retains independent attempts.
11. A newly created item with failed audience setup remains unpublished and resumable.
12. A publish failure after successful audience setup resumes without recreating content or overrides.
13. An HTTP 204 audience update is accepted without requiring a response object.
14. Collection-replacement routes preserve unrelated existing overrides and dates.
15. Omitted fields preserve settings; explicitly supported clears work intentionally.
16. Membership changes after preparation/review hold the affected action.
17. Explicit refresh adds a new eligible member without moving group membership.
18. Refresh protects students with existing work from silent removal.
19. Manual teacher exceptions are preserved or reported as conflicts.
20. A Page audience edit does not overwrite newer live body text.
21. A quiz audience edit does not recreate questions, reset attempts, or alter unrelated settings.
22. Retrying after an accepted or uncertain write does not duplicate resources.
23. A failure for one book does not block independent valid books.
24. Native module placement preserves existing entries and ordering.
25. Existing group assignments, group Pages, Hub support Pages, explicit student overrides, and graded group discussions do not regress.
26. Agent-visible artifacts, logs, and error output do not expose private roster identities.

Add regression coverage for any newly discovered edge case that materially affects this workflow.

For visibility, distinguish automated/API assertions from actual student-session evidence. If approved live testing is available, demonstrate both a targeted and an untargeted student’s access, including direct URLs and the shared module. If not, document that limitation without blocking otherwise valid implementation work.

## 14. Representative end-to-end demonstration

Use synthetic identities and an approved test scope.

Prepare one shared Book Club module and two collaborative groups:

- Ghost readers: Student A and Student B.
- Alone readers: Student C and Student D.

Create:

- A native Ghost course Page targeted to Ghost members.
- An individual Ghost writing assignment targeted to Ghost members.
- A Ghost New Quiz targeted to Ghost members.

Show through appropriate evidence that:

- The audiences resolve correctly.
- These resources are individual work, not native shared-submission assignments.
- The module uses native content references.
- Targeting failure does not publish content broadly.
- A restart resumes accepted resources.
- An explicit audience refresh detects a changed membership.
- Existing work prevents an unsafe silent unassignment.

If the repository can already do part of this through existing commands, demonstrate that path and add only the integration necessary to make the full workflow supported.

## 15. Expected completion report

At completion, provide:

- The capability audit: what existed, what was newly implemented, and what was deferred.
- The actual changed components and files.
- The final supported schema with valid examples for a Page, individual assignment, and New Quiz.
- The exact CLI/MCP workflow for creating content and refreshing audiences.
- Test commands and actual results.
- Evidence levels and any remaining Canvas/deployment limitations.
- Documentation/playbook updates.
- Confirmation that the real classroom setup was not migrated or changed by this development task.
- Any optional migration plan for replacing existing group-calendar links with native restricted course Pages, clearly separate from implementation and not executed automatically.

The success criterion is that a teacher can select a book group as the source of an individual content audience and Engine reliably completes the Canvas work without manual student lists, duplicate rosters, accidental shared submissions, or unintended course-wide visibility.

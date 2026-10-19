# 10 — Shared grades, rubric assessment, and feedback audience

Requires: [09](09-group-submissions.md) and **Gate P** ([README](README.md#gate-p-live-propagation-check)): the observed propagation behavior confirms or revises the representative-route, audience-endpoint and comment-flag contracts below before this slice is coded. Result: assess one club product, review all affected students, and post the honest shared grade/rubric with deliberately scoped feedback.

## Result contract

Add a distinct `kind: group-results`, `packet` pointing to a group packet, assignment snapshot, ordinary `interpretation`/optional `scoring_basis`, and keyed `groups` rows. A row selects `group_id`, `artifact_key`, exact observed member refs, and `operation: grade|comment-only`. Do not accept ordinary individual results for a shared-mode assignment.

For `grade`, reuse the existing component/assessment/feedback structure but require `feedback_audience: group|none`. `none` carries no shared feedback. The common grade has `components`, derived `assessment`, `feedback`, `header`, optional `teacher_note`/academic prior comparison/`late_timing`, and `outcomes`; it has no single member `attempt`, because all affected attempt assertions come from the referenced group-work packet. Optional `individual_feedback` rows carry `student_ref`, `text`, and Engine `outcome` for private student-facing text; teacher-only notes remain separate. For `comment-only`, use the existing criterion/components/feedback semantics without grade/rubric/late payloads and require group audience. Store per-action outcomes; grade, rubric, shared comment, and each individual comment have distinct keys. A shared result's criterion comments are student-facing for the whole affected group.

Specimen row within `groups` (the surrounding document carries the ordinary assignment snapshot, packet, interpretation and basis):

```yaml
group_id: 701
artifact_key: artifact-1
member_refs: [student-ref-a, student-ref-b]
operation: grade
grade:
  components:
    - key: interpretation
      evidence: present
      criteria:
        - criterion_id: evidence
          points: 80
  assessment: 80
  feedback_audience: group
  feedback: Your interpretation connects a clear claim to relevant evidence.
individual_feedback:
  - student_ref: student-ref-a
    text: Your reflection identifies a useful next question for the club.
```

Document all generated member IDs/attempt assertions through the packet; no raw Canvas user IDs are authorable. This slice supports only known `grade_group_students_individually: false`. Slice 11 adds the other mode. Existing `revise-feedback` on ordinary individual results remains unchanged; do not pretend one returned comment ID identifies all copies of a shared comment. Whole-group comment revision is outside this slice's acceptance scope.

## Review and posting

Use `review`/`start_review`, `post`/`post_results` with explicit kind dispatch. Review has one shared-grade card per group showing artifact, full affected aliases/refs, group-mode setting, common criterion scores, computed assessment, shared comment and separate private-comment cards. The group card is indivisible for a shared grade. Private text must never appear in the shared comment or criterion comments. Deselecting a private comment does not change the grade's audience.

Before each propagated action, check live assignment basis/mode, current group/set/course, full propagation membership and per-member current attempts/evidence. Require the live audience to equal the reviewed snapshot and requested scope. A subset student request cannot become a group grade. A changed member, excused member, hidden/unassigned target, unresolved enrollment, or divergent artifact holds the group grade with a specific explanation; other groups continue. Preserve actual completed outcomes if a later check fails.

Canvas shared-mode grades and rubrics propagate from one representative member route; whole-group comments use a separate flag. Use one deterministic representative from the validated artifact/member records and record that endpoint choice. Never loop the same shared grade over every student. [Grade/audience implementation](https://github.com/instructure/canvas-lms/blob/master/app/models/abstract_assignment.rb), [rubric propagation](https://github.com/instructure/canvas-lms/blob/master/app/models/rubric_association.rb).

Reuse exact arithmetic and existing independent/coupled rubric ordering. For a coupled rubric, post the honest rubric first and then the computed grade as a separate action. Recheck audience/attempts before that distinct next action; no success-confirmation readback. Send one whole-group comment with explicit `comment[group_comment]=true` and the intended attempt; private comments explicitly use false and the member route. Comment audience does not control grade propagation.

Do not overwrite a higher entered grade into the rubric calculation. For no-rubric work, use the explicit local basis and write no rubric. Render the same honest feedback in review and post. Common late timing may be sent only for a single explicit full-audience action with a matching effective date/timing basis for every affected member. Otherwise omit timing and explain that valid grade/feedback work continued; never calculate a late penalty in CE-SE.

## Recovery and implementation

Add `group_results.go`, `group_post.go`, and group review builders using existing assessment/render helpers. Keep ordinary `post.go` branches stable; share only actual arithmetic/render/action helpers, not an operation registry. Completed outcomes refer to the group assignment/artifact/member scope. Membership changes do not cause an accepted shared action to be replayed against a new audience. For a timeout, reconcile only from bounded relevant evidence on explicit resumption; an unrelated individual comment with equal text is not proof of a shared comment.

## Gate P observations (2026-10-17, see [README](README.md#gate-p-results-2026-10-17))

Shared-mode grade and rubric propagated from one member; `group_comment=true` created separate per-member comment copies with different IDs; `group_comment=false` reached only the named member; `group_members` returns string `id` plus `name`. A first grade write on a member with no submission record returned 403 for an undetermined reason: hold that member's group action specifically and do not retry blindly. Late timing was not observed.

## Code anchors (verified in Dev 2026-10-17)

- Today's `postGrade` PUTs to `/submissions/<one user>` per student row (`post.go:366`). In shared mode Canvas applies that to the whole group, so a group assignment must never reach `Post`; the guards at `scoring.go:286`/`408` and `canvas_reads.go:287` are the only barrier. Keep them. Group results get their own post entry point; do not add a branch inside `postGrade`.
- Skip is `s.Skip[r.StudentRef]` (`post.go:101`, `312`), fed from the review form in `localui/review.go:244-264` and set in the callback at `cmd/cse/remote.go:390`. A group/action card ref matches no student ref and would be silently ignored, posting the group anyway. The `group-results` callback needs its own skip semantics keyed by group and action, and the failed-count loop at `remote.go:393` is student-keyed too.
- The review callback's fallback sends any non-assignment, non-quiz kind to `service.Post`. `ValidateResults` rejects other kinds, so today that fails closed, but only after the teacher approved the card. Add an explicit `group-results` branch, and make sure other new kinds (discussion, group-set) are not routed there by accident. `push`/`post` are separately allowlisted at `remote.go:88` and `91`.
- Existing comment writes never set `comment[group_comment]`, so they remain individual. The shared comment is a new explicit action.

## Acceptance

- One reviewed shared assessment issues one grade/rubric sequence and one shared-comment action for the exact group; an independent club receives its own score.
- A changed member, changed attempt, mode change, excused/hidden member, subset request, or a generic roster missing someone in the assignment-specific audience holds the propagated write without affecting other groups.
- A coupled rubric whose scale differs from assignment points sends truthful criteria and the honest computed grade in the correct order.
- A private-feedback sentinel appears only for its selected member, never in shared payloads or other members' review cards.
- A failed comment resumes without repeating accepted grade/rubric actions.
- Deselecting a group grade or one private-comment card suppresses exactly that action in the real post callback; ordinary student, assignment/quiz and parent-email review selection is unchanged.

Update architecture/scoring documentation when implemented to replace the current blanket group-scoring limit with the actual supported mode and audience rules.

# 07 — Individual discussion assessment

Requires: [06](06-discussion-evidence.md) and [08](08-group-assignment-authoring.md) (assignment snapshot foundation: discussion identity, checkpoint metadata, `SameGradingBasis` normalization). Uses 05b-authored discussions or any existing graded discussion. Result: assess each student's discussion work with conversation context and post individual grades/rubrics/comments through the ordinary review workflow.

## Contract

Extend `pull-work COURSE ASSIGNMENT_ID` so a linked graded discussion is recognized and its verified topic/child evidence is pulled. Scope remains the selected assignment. Broad unfinished-work discovery may include supported graded discussions; it must not mark an entire conversation completed when one student has been assessed.

An ordinary `packet`/`student-work` retains submission/attempt identity and adds a `discussion_evidence` reference: packet path, source topic/group, authored entry references and a deterministic digest of relevant own entries plus context explicitly cited by the assessment. Return the student's own evidence separately from other people's context. Current membership is not authorship. Use full conversation evidence for the agent's judgment; do not treat a reply count as a grade.

Continue using `kind: results`, `operation: grade|comment-only|revise-feedback` and existing component/criterion contracts. Add a result-side reference to the observed evidence selection/digest. A linked group discussion is individually assessed, not a shared product with propagated grades. Do not set `comment[group_comment]` on this path. Course-level assignment feedback is distinct from posting publicly in the discussion.

## Implementation

Use slice 08's snapshot discussion fields (topic identity and the checkpoint indicator). Obtain the association from Canvas, not assignment title or `group_category_id` alone. Resolve the live submission for each student and use existing numeric/no-rubric scoring, truthful rubric writes, and accepted-write completion.

Before posting, check the current attempt, relevant evidence versions, assignment/rubric basis, and topic/assignment relationship. A changed assessed post or cited context holds that student for re-pull; an unrelated student's new post should not invalidate everyone. Incomplete reads produce unknown evidence, never automatic zero. Missing required responses can receive honest zero components only after complete relevant evidence and the teacher's actual task requirements establish that absence. Students without a submitted Canvas attempt remain specifically held under the existing no-automatic-zero rule.

Checkpointed discussions are a separate Canvas grading variant and are not supported here. Slice 08 captures the presence-aware `has_sub_assignments`/`checkpoints` metadata. When it indicates checkpointing, preserve and read the evidence but hold only that item's grade action, explaining that scoring happens in SpeedGrader. Missing metadata alone is not evidence of checkpointing and never makes an ordinary discussion unsupported. Do not substitute an aggregate grade, and do not make checkpoint support a dependency of ordinary discussion grading.

Review displays the student's attributed posts with needed context, honest criterion results, feedback, and individual audience. Existing late-timing rules apply using the student's effective due date; uncertain discussion timing omits only the timing action.

## Code anchors (verified in Dev 2026-10-17)

- `unpullableAssignment` (`canvas_reads.go:280`) already lets `discussion_topic` assignments through, so `pull-work` can reach a graded discussion today. Per-student checks (`unsupportedSubmission`: excused, unassigned, no submission/no attempt) are reusable unchanged.
- **Resolved by Gate P (2026-10-17):** a graded group discussion's linked assignment returned `group_category_id: null`, so the existing group guards do not reject it. The group set lives on the topic (`group_topic_children` lists child topics and group IDs). Keep the guards as they are for real shared products.
- `completedHint` (`pull_work.go:562`) already recognizes `kind: results` grade rows per student/attempt, so individually assessed discussions get completion handling without new code. It ignores comment-only and revise rows; keep that behavior.

## Acceptance

- Two students in the same child conversation receive different results and only individual feedback.
- Changing one assessed entry holds that student only; incomplete coverage or unknown authors never become invented zeros.
- Topic/assignment/group identity mismatch blocks writes; a checkpointed discussion holds only its grade action with a precise explanation.
- Existing arithmetic, no-rubric, coupled-rubric, stale-attempt, review and duplicate-comment behavior still passes; a successful post does no confirmation GET.

Update scoring and discovery guidance so ordinary discussion evidence is no longer presented as merely the optional `discussion_entries` field on a REST submission.

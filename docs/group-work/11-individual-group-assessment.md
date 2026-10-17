# 11 — Individual grades on a shared product

Requires: [09](09-group-submissions.md), [10](10-shared-assessment.md) and Gate P ([README](README.md#gate-p-live-propagation-check)); the individual-mode observations apply here. Result: assess individual students who share a product without changing the assignment's live grading mode or exposing private feedback to clubmates.

## Contract

Support only a known live `grade_group_students_individually: true`. Extend `group-results` rows with `students` containing ordinary per-student grade/comment-only operations and their component interpretations, local scoring basis where needed, and attempt/evidence assertions. A group row cannot carry both a shared grade and individual grades. Reuse existing `StudentResult`/`GradeOperation` structures where they accurately represent the operation; do not copy them merely to rename fields.

The shared artifact remains linked once from the group packet. Each student's record retains their current Canvas submission/attempt, current group identity and evidence selection. Individual academic judgments must come from observed work and the teacher's criteria; membership alone does not prove equal contribution, and the Engine implements no participation multiplier. Existing interpretation variants can represent established accommodations without publishing their private rationale.

An optional explicit shared-feedback action can accompany these individual results, but it is separate from the students' grades/comments and is reviewed against the full group audience. Deselecting/holding one student cannot silently convert their private comment into group feedback.

## Implementation

Extend group validators/review/post builders, not ordinary individual post authorization. Before each student grade/rubric action, check live mode is still individual, assignment basis and that student's current attempt/evidence. A mode change holds remaining grade/rubric actions; it is never corrected by writing the mode back. An unresolved student or resubmission holds that student while independent students can proceed in individual mode.

Grade and rubric writes target only the student route; comments explicitly use `group_comment=false`. Shared-mode rubric propagation must not be invoked. Reuse honest grade computation and coupled rubric sequencing, exact comment targeting, duplicate handling, teacher-only notes, and accepted-write completion. [Mode-dependent grade handling](https://github.com/instructure/canvas-lms/blob/master/app/models/abstract_assignment.rb), [mode-dependent rubric handling](https://github.com/instructure/canvas-lms/blob/master/app/models/rubric_association.rb).

Per-student late timing uses that student's actual effective due date and attempt. It does not alter the shared academic assessment or another member's lateness. Optional group feedback uses slice 10's full audience check independently of these individual timing actions.

Integrate unfinished-work/completion handling: shared artifacts deduplicate downloads, but completion keys remain student/assignment/attempt/evidence for this mode. Assessing one student must not hide the remaining club members. An explicit re-pull permits reassessment without automatically rewriting completed outcomes.

## Code anchors (verified in Dev 2026-10-17)

`completedHint` (`pull_work.go:562`) matches only `kind: results` grade rows by student ref and attempt. Individual-mode `group-results` rows must be added to that check so completing one member does not hide others and finished members do not reappear. The results-document guards (`scoring.go:286`, `408`) stay in force for `kind: results`; the new kind has its own validators. Per-student writes can reuse the existing `postGrade`/`postCommentOnly` building blocks only through a group-specific caller that has already verified live mode is individual.

## Acceptance

- Three members receive distinct truthful grades/rubrics and private comments; no shared-comment flag is sent unless that separate action was requested.
- A new attempt holds only that student; a live switch to shared grading holds all remaining numeric/rubric writes.
- One completed student does not hide the unfinished members in discovery.
- Late timing is written only to the matching student; ordinary non-group results and New Quiz comment-only paths are unchanged.

Update review descriptions and scoring guidance to explain the difference between one shared submission and individual grades on it. Record local/live evidence separately.

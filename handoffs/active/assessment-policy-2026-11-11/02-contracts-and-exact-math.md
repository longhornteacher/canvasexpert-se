# Slice 02 - Narrow contracts, provenance and exact math

Status: **not started**. Kind: Engine domain/contracts, formats and synthetic examples. Depends on the decisions in [README](README.md), not on release of slice 01. No Canvas writes are needed.

## Result and boundaries

Define the small amount of machine-readable intent needed to identify a Major companion, hold it unassigned, apply the teacher's floor and safely carry a retake result back to the exact original. The agent still judges category, parallel content and honest effort. Do not add a policy expression language, configurable rule evaluator or arbitrary grade override.

Current anchors: `internal/domain/types.go`, `validation.go`, `effort_floor.go`, `assignment_dates.go`, `group_results.go`; `internal/workflow/quiz_author.go` (`QuizSettings`, sidecar/stages), `results_assignment.go`, `results_init.go`, `pull_work.go` (`enteredGrade`); `cmd/cse` formats/help; document kind dispatch in workflow/CLI/MCP. Locate exact current symbols with `rg` rather than trusting line numbers.

## Contract decisions

### Category and original identity

Use optional `assessment_category: daily|major` on relevant authored assignment/QuizForge intent and the persisted original/companion authoring record. Do not require it on unrelated legacy drafts. Where the timetable or retake behavior depends on it, an unresolved category needs a clear teacher/agent decision instead of silently choosing Daily.

QuizForge `metadata.canvas` gains narrow intent for:

```json
{
  "assessment_category": "major",
  "retake_for": {
    "instance": "https://canvas.example.edu",
    "course_id": 100,
    "assignment_id": 200
  },
  "unassigned": true,
  "published": false,
  "access_code": "teacher-private-code",
  "post_to_sis": false
}
```

This is a contract sketch, not a full valid quiz. The implementer may place the category outside the Canvas subobject if current conventions require it; normalize the chosen contract in every example/validator. `retake_for` must identify the same configured Canvas instance/course as the destination and a different existing original assignment. A positive ID, title prefix or handwritten note alone does not validate the relationship. Sidecar stores the confirmed original identity and the created companion ID; durable context stores only reusable association/expectations.

For a tutorial companion, require a nonblank code, initial `unassigned: true`, no requested publication, no inherited/shared module audience and no ordinary multiple-attempt enablement. A normal teacher-authored quiz without `retake_for` keeps its existing behavior. Reject contradictory intent before any create. Publication/assignment for tutorials is a distinct explicit release operation defined in slice 03, not an edit of the initial create file followed by blind replay.

### Honest-effort input and floor

Keep `Course.EffortFloor` and `GradeOperation.HonestEffort`; do not replace the existing course setting or bulk-migrate other teachers. Add the equivalent explicit honest-effort input to New Quiz policy scoring and shared-group grade operations where required by slice 05. No implicit true default for old files. The configured percent remains teacher-changeable; this teacher's adoption sets 40, including courses that previously had no floor.

Add only the outcomes/baseline fields actually needed for a separate quiz grade adjustment. A numeric floor operation must be distinguishable from a pure `comment-only` row. Do not make every feedback-only comment unexpectedly write a grade. If extending `CommentOnlyOperation`, require an explicit policy-adjustment request in addition to `honest_effort`, document the exception and keep omitted fields backward-compatible.

### Retake application document

Prefer a focused `kind: retake-results` routed through existing `validate`/`post`/`post_results` entry points. It is not an ordinary `grade` row against fabricated original components. Proposed shape:

```yaml
schema: 1
kind: retake-results
course: Example
instance: https://canvas.example.edu
course_id: 100
original_assignment_id: 200
retake_assignment_id: 201
retake_packet: courses/Example/assignments/201-retake/pulled/example/packet.md
students:
  - student_ref: s-example
    retake_attempt: {attempt: 1} # use the actual full AttemptRef contract
    honest_effort: true
    original_observed: {}      # Engine-prepared identity/grade/attempt baseline
    retake_observed: {}        # complete result/attempt evidence, not supplied totals
    outcomes: {}               # Engine-owned grade/comment stage outcomes
```

The exact existing AttemptRef and outcome structures should be reused. An Engine preparation step supplies original/current baselines and trustworthy complete quiz totals. Authored `earned_percent`, `final_grade`, raw Canvas student IDs, arbitrary targets, cap overrides or arbitrary `posted_grade` fields are refused. A source total must come from a fully scored, identity-matched current result; ungraded written items are not zeros. The Engine recomputes the proposal during validation/preparation and renews live state before posting.

Record normalized original entered academic grade as known value, known empty or unknown, using `enteredGrade` semantics. Do not reconstruct a pre-penalty grade from displayed score plus guessed deductions. Keep the original raw score, points, due/timing state, attempt/version and posting state needed for conflict/no-lowering checks. A changed grade after review holds that student's application for fresh preparation; never overwrite the teacher's intervening edit.

### Grade math

Use the formula in the README. Extend/reuse `domain.EnteredGrade` and decimal/rational helpers with a small retake calculation function. It should accept confirmed quantities and produce earned percentage, floored retake grade, capped original candidate and original target, without deciding genuine effort or calculating Canvas late penalties.

Worked examples, percentages before late deduction:

| Original | Retake earned | Genuine effort | Applied original | Reason |
|---|---:|---|---:|---|
| 52/100 | 88/100 | yes | 70/100 | Cap contribution at 70 |
| 65/100 | 54/100 | yes | 65/100 | Preserve higher original |
| 84/100 | 96/100 | yes | 84/100 | Never cap the existing original |
| 25/100 | 10/100 | yes | 40/100 | Honest effort floor before retake cap |
| 25/100 | 10/100 | no | 25/100 | No floor; no lowering |
| 12/30 | 18/20 | yes | 21/30 | Normalize source, then scale to original |
| known empty | 30/50 | yes | 60% of original points | Empty is distinct from unknown |

Do not make the arithmetic reject an original below 40: it may be historical, late-adjusted, non-effort work or a manual entry. Applying the current retake rule is separate from retroactively sweeping old grades.

## Validation and privacy

- Bind exact instance, configured course, original ID, companion ID, stable student ref and attempts; all raw identity resolution stays inside the Engine.
- Original must be the confirmed Major target and source must be its confirmed companion. Do not infer from the title or copy a link across course sections without remapping IDs.
- Live group mode, excused/visibility state, owned-course consent/post policy and current rubric/points are renewed at the write boundary.
- Floored/retake grades must never be smuggled into earned criterion scores. Keep proof of earned versus entered values in Engine-rendered validation output.
- Access code is teacher-only authoring data; suppress it from student PDFs, ordinary reports, feedback, Pages and capability/status JSON. Private teacher draft presentation may intentionally show it; do not log it unnecessarily.
- Drafts, pulled totals and result receipts remain in UserInstall. General reusable mapping and reviewed expectations use existing shared context routing.

## Focused tests and acceptance

Add meaningful domain tests for the seven examples above, a non-100-point fractional case with one final rounding, unknown/zero denominator, no-floor false flag and known-empty versus unknown original grade. Reuse table tests; no Cartesian validation matrix.

Test one wrong-origin/course pair, self-link, blank code and contradictory published/unassigned intent. Test a legacy quiz and legacy feedback-only result retain their old behavior. Validate one synthetic ordinary-original retake application and one New Quiz-original example without credentials. Confirm invalid arbitrary earned/final totals are rejected and the new kind reaches the existing dispatch with useful diagnostics.

Update the smallest relevant Engine formats/examples and reference tests in this slice; adapters and actual writes follow the consuming slices. Completion record names the accepted schema, any refinements, tests run and dependencies still unimplemented.

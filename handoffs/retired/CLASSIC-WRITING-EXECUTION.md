> **Retired 2026-10-10.** Implemented (a2e2820). Classic Quizzes are being removed; see handoffs/retired/CLASSIC-QUIZ-RETIREMENT.md.

# Execution brief: score Classic quiz writing

Prepared 2026-10-09. Audience: a Sol implementation agent, followed by a Luna documentation agent if desired. This brief fixes the design decisions; implement them directly. Preparing this brief does not launch implementation or authorize Canvas Live activity.

## 1. Deliverable and authority

A teacher can discover or explicitly pull a Classic quiz, assess its essay/file-upload responses, validate a local result, and post a score plus comment to each selected question. Objective-item scores remain Canvas-owned. One unreadable question or blocked student does not prevent independent ready work from posting.

Read, in order: repository `AGENTS.md`, `ARCHITECTURE.md`, `SCORING-RULES.md`, this brief. `QUIZZES-HUB-PLAN.md` explains the reasoning. This brief supersedes that plan's unresolved implementation choices. The historical OG CE files provide API observations, not architecture instructions.

**Scope is this grading workflow. Leave quiz authoring/resume, Hub delivery, and their sidecars unchanged.** The hypothetical middle-deletion resume problem is not a prerequisite, regression project, or queued follow-up. Do not revive it while implementing this brief.

No new dependencies, commands, services, ledgers, operation frameworks, locks, reconciliation systems, or schema migration. Use ordinary Go functions and the existing strict Markdown/YAML documents. Preserve existing teacher workspace files; change seed Playbooks only.

## 2. Decisions already made

| Question | Decision |
|---|---|
| User entry points | Keep `pull-work [course] [assignment]`, `validate FILE`, and `post FILE`. |
| Target lookup | Existing assignment ID/link/exact-title lookup. The quiz's backing assignment is the target. Do not add quiz URL/name lookup in this change. |
| Supported writing | `essay_question` and `file_upload_question` on submitted, individually graded Classic attempts. |
| Quiz provenance | CE-SE-authored quizzes are the design center. Do not require a sidecar or reject an otherwise compatible teacher-created quiz merely for its provenance. |
| Attempt to write | Latest submitted attempt only, provided the quiz submission is currently finished. Earlier attempts are read-only evidence. An in-progress newer attempt holds that student. |
| Question definitions | Fetch the questions for that exact quiz submission and attempt, using Canvas's existing API parameters. No local version model or position-based mapping. |
| Authorship of judgment | The agent chooses an honest score and comment using prompt, rubric, actual response, prior work, and teacher context. |
| Scoring representation | Direct question points plus comment. No new criterion aggregation grammar, mandatory artificial rubric, equal weighting, or quiz-total scaling. |
| Rubrics | Include an existing assignment rubric as context. The agent explains its question mapping in private `scoring_notes`. This operation writes no rubric assessment and must say so. Ordinary assignment rubric posting stays unchanged. |
| Feedback | A question comment containing an Engine-rendered assessment line followed by the agent's supplied feedback. No automatic submission-level comment. |
| Total | Read/report Canvas's result. Do not set `posted_grade`, `fudge_points`, or a whole-quiz total. |
| Lateness | Optional existing `LateTiming` contract, applied separately through a small extraction of the existing timing writer. Canvas computes penalties. |
| Posting policy | Neither change it nor build posting-policy management. Successful API posting is not proof the student can currently see feedback. |
| Result persistence | Save action outcomes inside the result document using `workspace.WriteDocument`. No additional journal. |
| Historical retries | Confirm uncertain writes by reading the exact target. A differing result stays uncertain; no blind retry loop or auto-repair. |
| Live validation | Not authorized by this brief. Finish local implementation and report its evidence honestly. |

Support what these direct APIs can identify without guessing. Do not add a bank/randomization compatibility subsystem or special-case particular quiz titles/types beyond the two writable types. An unmappable response is a specific reported limitation, not a reason to infer identity or disable all other students.

## 3. Canvas contract and evidence

Use the existing authenticated `canvas.Client`; do not add another HTTP client or expose raw response objects to the agent.

The documented quiz-submission resource provides `id`, `quiz_id`, `user_id`, `submission_id`, `attempt`, `finished_at`, `workflow_state`, `score`, and `kept_score`. List/single/write responses use a `quiz_submissions` array envelope. `submission_id` links to the assignment submission. The write uses `quiz_submissions: [{attempt, questions: {questionID: {score, comment}}}]`. These facts are documented, not newly live-verified here. [Canvas Quiz Submissions API](https://developerdocs.instructure.com/services/canvas/resources/quiz_submissions)

For question definitions use `GET /api/v1/courses/:c/quizzes/:q/questions?quiz_submission_id=:qs&quiz_submission_attempt=:attempt&per_page=100`. Both identity parameters are required to select the questions presented for that attempt. Use the same path for current and earlier attempts. [Canvas Quiz Questions API](https://developerdocs.instructure.com/services/canvas/resources/quiz_questions)

OG CE's `D:\Development Projects\CanvasExpert\docs\reference\classic-quiz-design.md`, section “Verified Canvas facts: Classic Quiz grading,” records the response-content path and a successful per-question grading probe on 2026-09-30: assignment `include[]=submission_history`, essay `text`, upload `attachment_ids`, question `points`, and posted `more_comments`. It also records that Canvas accepts out-of-range scores; our bounds check is therefore required. This is historical OG evidence, not evidence that CE-SE's new code works live.

Additional source checks used to settle implementation details:

- [Quiz-submission controller](https://github.com/instructure/canvas-lms/blob/master/app/controllers/quizzes/quiz_submissions_api_controller.rb): teacher list behavior must not be treated as a complete attempt history. Obtain responses from assignment history instead.
- [Submission serializer](https://github.com/instructure/canvas-lms/blob/master/lib/api/v1/submission.rb): quiz history is composed from quiz attempts; do not assume a history row's `id` is the top-level assignment submission ID. Use top-level assignment identity and the explicit quiz-submission link.
- [Quiz-submission serializer](https://github.com/instructure/canvas-lms/blob/master/lib/api/v1/quiz_submission.rb): use a whitelisted DTO. In particular, do not project `validation_token`, user IDs, or the entire payload into documents.

No implementer needs to re-probe these facts before coding. If the real API later contradicts the contract, report the exact discrepancy and affected scope; do not invent fallback matching or broaden the project.

## 4. File and function placement

The names below are fixed public/document contracts. Private helper names can vary when clearer; do not turn that freedom into a redesign.

| File | Change |
|---|---|
| `internal/domain/classic_scoring.go` (new) | Classic document types, structural validation, exact point validation, question-comment rendering helper if not placed in render. |
| `internal/workflow/classic_reads.go` (new) | Whitelisted quiz/quiz-submission/question/answer DTOs and the small read helpers below. |
| `internal/workflow/classic_pull.go` (new) | Classic pull branch, history projection, completion lookup. |
| `internal/workflow/classic_post.go` (new) | Source loading, posting, question outcomes, read-back. |
| `internal/workflow/canvas_reads.go` | Add `SubmissionData []ClassicAnswer` to `Submission`; keep ordinary assignment rejection intact. |
| `internal/workflow/pull_work.go` | Dispatch Classic assignments before `UnsupportedAssignment`; extract attachment downloading for real reuse. Do not broadly loosen submission checks. |
| `internal/workflow/post.go` | Dispatch `classic-quiz-results` before parsing ordinary `ResultsDocument`; extend report with Classic question details. Ordinary grading path stays semantically unchanged. |
| `internal/workflow/late_timing.go` | Extract explicit-argument timing helper, retaining the current ordinary wrapper and behavior. |
| `internal/canvas/client.go` | Add envelope-list support through the existing pagination loop; details below. |
| `cmd/cse/main.go`, `remote.go` | Recognize new document kinds, validate/preview source-aware Classic results, and display question outcomes. |
| `engine-docs/formats.md`, `engine-docs/examples/` | Document the exact contracts and one synthetic example set. |
| `workspace-template/playbooks/score.md`, `quizzes.md` | Agent workflow and truthful supported scope. |
| `internal/localui/capabilities.go`, `docs/engine-capabilities.md` | Matched status/evidence updates when implementation is complete. |

Use existing `domain.Decimal`, `AttemptRef`, `AssignmentSnapshot`, `Attachment`, `LateTiming`, `ActionOutcome`, `PriorComparison`, `workspace.NotesFor`, `workspace.ConstrainPath`, `sameAttempt`, `SameGradingBasis`, `failedOutcome`, and `timingMatches` where their semantics match. Do not stuff Classic scoring into `GradeOperation` just to reuse its writer.

### Pagination adaptation

`Client.List` currently handles a top-level JSON array only. Add `ListField(ctx, path, field string, out any) error`; move the existing loop into one unexported helper receiving an optional envelope field. `List` delegates with an empty field, and `ListField` requires a nonempty field. Inside page decoding, extract the named array; absent/null/non-array fields are invalid responses. Preserve the existing Link handling, origin restrictions, bounded loop, and atomic replacement of `out`.

Use `ListField(..., "quiz_submissions", &rows)` for quiz submissions. This is a small transport necessity, not permission to add a generalized JSON query or arbitrary endpoint interface. One two-page envelope test is enough; reuse the existing transport tests for shared mechanics.

## 5. Document contracts

All documents remain `schema: 1`. Add these kinds; do not change ordinary `packet`, `student-work`, or `results` semantics. Documents use the existing front matter parser and preserve their Markdown body.

### `classic-quiz-packet`

Go type `ClassicQuizPacket`:

- `schema`, `kind`, `course`: same roles as an ordinary packet; kind is `classic-quiz-packet`.
- `assignment`: existing `AssignmentSnapshot`, including any live rubric.
- `quiz_id`: positive Classic quiz ID.
- `students`: workspace-relative paths to `classic-quiz-work` files.

Use the existing `courses/<course>/assignments/<assignment-id>-<slug>/pulled/<timestamp>/` folder layout. Body contains the live quiz description. Each pull gets a new folder and never replaces prepared work. No separate quiz filesystem hierarchy.

### `classic-quiz-work`

Go type `ClassicQuizWork`:

| Field | Type / meaning |
|---|---|
| `schema`, `kind` | `1`, `classic-quiz-work` |
| `student_ref`, `alias` | Existing pseudonymous identity/presentation rules |
| `assignment_id`, `quiz_id` | Positive resource IDs |
| `submission_id`, `quiz_submission_id` | Top-level assignment submission ID and linked Classic submission ID; neither is a Canvas user ID |
| `current` | `ClassicAttemptWork` |
| `prior` | Optional earlier `ClassicAttemptWork` array, ascending attempt number |
| `unhandled_question_ids` | Current manual-question IDs not completed in CE-SE's local results; on explicit pull, all current manual-question IDs |
| `notes`, `monitored` | Existing private teacher-note and monitored-student meanings |

`ClassicAttemptWork`:

- `ref`: `AttemptRef` derived from assignment history's attempt/submission timestamp.
- `questions`: `ClassicQuestionWork` array in displayed position order; identity is always question ID.
- `late`, `effective_due_at`: existing timing evidence. Use the current top-level submission's live cached due date for current timing; never manufacture a prior date.
- `canvas_score`: optional observed attempt score, not a grade to preserve or an agent target.
- `warnings`: optional short strings for unavailable earlier evidence. These are operational/private and never sent as comments.

`ClassicQuestionWork`:

| Field | Type / meaning |
|---|---|
| `question_id` | Positive ID from the attempt's definitions |
| `position`, `name`, `question_type` | Display/context; type stays Canvas's spelling |
| `prompt_html`, `prompt_text` | Original prompt and `render.HTMLText` projection |
| `points_possible` | Required nonnegative `Decimal` |
| `response_present` | Whether an answer row for this ID was returned; empty response text alone is not missing API evidence |
| `response_html`, `response_text` | Essay HTML and readable text; omit for objective items |
| `attachments` | Existing `Attachment` objects for upload responses |
| `observed_score` | Optional `*Decimal` from the answer's `points`; null is not zero |
| `observed_comment` | Existing question `more_comments`, empty if absent |
| `warning` | Optional specific missing/unreadable response information |

Include all attempt questions so writing can be understood alongside objective work and text-only passages. Do not include objective answer keys, raw answer payloads, or reimplement their scoring. Their observed points are sufficient for this first workflow. The agent has no need for transport URLs/tokens or enrollment/user payloads.

### `classic-quiz-results`

Go type `ClassicQuizResults`:

- `schema: 1`, `kind: classic-quiz-results`.
- `packet`: workspace-relative path to its source packet.
- `assignment`: copy of the packet's `AssignmentSnapshot`, using existing identity/grading-basis comparison.
- `quiz_id`: copied from the packet.
- `scoring_notes`: required nonempty private explanation of the common scoring approach/question rubric mapping. The Engine checks presence, not educational quality.
- `students`: nonempty list of `ClassicStudentResult`.

`ClassicStudentResult` has `student_ref`, `attempt` (`AttemptRef`), `questions` (nonempty array), optional `strongest_prior` (`PriorComparison`), optional private `teacher_note`, optional `late_timing` (`LateTiming`), and Engine-managed `late_outcome` (`ActionOutcome`). There is no operation selector: this document is question grading only.

`ClassicQuestionResult` has:

- `question_id`: selected current manual question.
- `evidence`: `present`, `missing`, or `unknown`, chosen by the agent.
- `score`: optional `*Decimal`; required for present/missing, forbidden for unknown.
- `comment`: student-facing feedback; required nonempty for present/missing, empty for unknown.
- `teacher_note`: optional private explanation; required nonempty for unknown so the hold is actionable.
- `outcome`: optional Engine-managed `ActionOutcome`.

Omitted question IDs mean **do not touch**. Omitted student rows mean do not touch those students. Do not require scoring every essay merely because it exists. Unknown evidence holds only that question. `evidence: missing` does not assign zero automatically; it requires the agent's explicit score like any other assessment.

Example result shape (the full `assignment` snapshot is copied from the real packet, not authored from this abbreviated illustration):

```yaml
schema: 1
kind: classic-quiz-results
packet: courses/ELA7/assignments/100-example/pulled/20261009T120000/packet.md
assignment: <copy the entire packet assignment mapping here>
quiz_id: 200
scoring_notes: >-
  Assess the two explanations against the prompt's claim and evidence requirements;
  each question uses its own Canvas maximum. Objective items remain unchanged.
students:
  - student_ref: student-reference-from-packet
    attempt:
      attempt: 1
      submitted_at: '2026-10-09T15:00:00Z'
    questions:
      - question_id: 301
        evidence: present
        score: '7.5'
        comment: Your claim is clear. Explain how the second quotation supports it.
      - question_id: 302
        evidence: unknown
        teacher_note: The uploaded document could not be read; preserve it for review.
```

The documentation agent must ship fully valid synthetic examples, with complete snapshots and matching packet/work files. The abbreviation above is explicitly an illustration, not a parser fixture.

## 6. Local validation and rendering

Keep two useful boundaries:

1. Domain validation checks schema, field shape, positive identities, valid attempts, unique student/question IDs, allowed evidence values, finite nonnegative explicit scores, required text, timing shape, and allowed existing outcome statuses. Structural errors fail before writes, as with ordinary results.
2. A workflow-level `loadClassicSources`/`ValidateClassicResults` step loads the packet/work files through `ConstrainPath`, resolves results to source students/attempts/questions, and checks that selected questions are essay/upload and scores fit source maxima. `validate` runs this step without credentials or network. `post` repeats it, then compares relevant live facts.

Offline validation resolves references within the supplied packet/work files only; it does not require a private student-store entry. Resolve `student_ref` to the private Canvas user ID at posting. This lets a synthetic example set validate without importing invented students into a teacher workspace.

Do not use `validateNumericTarget` unchanged: it intentionally rejects `online_quiz`. Add a small Classic-specific numeric/mode check. Reuse `ValidateSnapshot` for snapshot shape. Keep ordinary submission validators unchanged.

For scores, use exact decimal comparison. Normalize posting/display values with existing `RoundRat(score.Rat(), 2)` and validate both supplied and posted values within `0..points_possible`; never clamp. Store the normalized score in the validated in-memory result before saving/posting. Do not add float arithmetic or quiz-total computation.

Render exactly `Assessment: <score>/<points_possible>\n\n<comment>` for each assessed question, using `Decimal.String()` and the live-checked maximum. Preserve the supplied feedback text; no HTML template or whole-assignment feedback renderer. The renderer receives only score, maximum, and student-facing comment. It never receives `scoring_notes`, `teacher_note`, `strongest_prior.basis`, `notes`, or `late_timing.basis`.

Preview output is one line per requested question: student ref, attempt, question ID, proposed score/max or held reason; include supplied lateness once per student. Do not label a sum of selected questions as the quiz grade. `validate` retains its existing success/error exit convention: structurally valid unknown evidence can preview as held without making the entire file malformed.

Source consistency errors such as an invented question ID or a student absent from the packet fail local validation. A genuine unavailable answer in an otherwise valid packet becomes a per-question hold. Readable extraction is a convenience: the agent may use another tool to read a preserved file and then choose `evidence: present`; the Engine must not veto that merely because built-in extraction was unavailable.

## 7. Read and discovery algorithm

### Read helpers

Implement small helpers for:

- `GET /courses/:c/quizzes/:q` under `/api/v1`, decoding only quiz ID, assignment ID, title/description, quiz type, scoring policy, points as needed.
- Paginated quiz submissions, decoding the whitelisted resource fields in section 3.
- Exact single quiz submission read for pre-write checks.
- Attempt-scoped question list using the parameters in section 3.
- Assignment submissions/history through the existing helpers.
- `GET /api/v1/files/:id` for upload attachment metadata, then the existing safe download path.

Add a `ClassicAnswer` DTO for `question_id`, nullable `points`, `text`, `attachment_ids`, and `more_comments`. Do not parse Canvas's polymorphic `correct` field; the two manual types are identified from question definitions. Decode IDs with the existing numeric conventions that handle Canvas's quoted numeric IDs where applicable. Preserve nullable score distinction.

### Pull routing

Within the existing course/assignment loop, before `UnsupportedAssignment`, route an assignment with a positive `QuizID` to a `pullClassicWork` helper. That helper confirms the live Classic quiz links back to this assignment and has `quiz_type: assignment`; keeps the existing numeric, individual, nonanonymous, nonmoderated limits; and requires `online_quiz`. New Quizzes/external-tool assignments remain unsupported by this grading path. Do not make ordinary `pull-item`/authoring accept quizzes as a side effect.

Pass the already-loaded course, student store, active roster, course roster, notes, explicit-scope flag, optional built-in probe identity, and report/summary context into the helper. A small private arguments struct is acceptable; do not refactor all pull operations into plugins.

1. Fetch quiz submissions once for the quiz; index by their user/submission identities. On automatic discovery, discard finished `complete` submissions before fetching detailed work. For candidates, inspect their attempt questions normally; if no essay/upload questions exist, report that there is no writing to assess and skip. Do not add a separate quiz-definition prefetch or optimization cache.
2. Filter against the existing active student roster and explicit built-in probe scope. Missing, excused, inaccessible, and unfinished work is reported, never automatically zeroed. Ordinary pulls still exclude Test Student.
3. For each candidate, read its top-level assignment submission with history. Join `qs.user_id == sub.user_id`, `qs.submission_id == sub.id`, `qs.quiz_id == quiz.id`, and `sub.assignment_id == assignment.id`. Require quiz and assignment current attempt numbers to agree and the quiz state to be `pending_review` or `complete` with a finished timestamp. An ambiguous join or newer in-progress quiz holds that student only.
4. Find the response-bearing history entry with the exact current attempt number; verify its timestamp against the top-level current attempt. If the top-level submission itself has matching response data it can supply it. Never pick `history[-1]`, never use an array position as an attempt, and do not compare history-row `id` with top-level assignment submission ID.
5. Fetch current attempt definitions and join answer rows by question ID. A returned answer row with empty essay text is evidence of an empty response. A missing answer row is reported as unavailable, not silently treated as a blank answer. Keep independent mapped questions usable.
6. For earlier submitted history entries, fetch their attempt-specific definitions using the same quiz-submission ID and each earlier attempt number. Preserve responses in ascending attempt order. If earlier evidence cannot be read, warn for that attempt and continue current assessment; never fabricate the strongest prior work.
7. Resolve upload IDs to attachment metadata, then reuse downloads/extraction. Extract the existing attachment loop from `attemptWork` into a concrete helper taking attachments and attempt/path identity; preserve its current ordinary-assignment behavior. File lookup/download failures become per-attachment outcomes, not whole-class failures.
8. Save the student store before projecting new references, as ordinary pull does. Build Classic documents, notes via `NotesFor`, and report paths/counters with the existing summary types.

Current attempt definitions are fetched once per candidate and reused within that pull. Do not add a persistent cache or try to deduplicate supposedly identical attempts across students. This trades a straightforward API read for correct identity without new machinery.

### Discovery completion

Explicit assignment pulls include compatible submitted attempts even when already graded; they support reassessment.

Automatic discovery skips a current quiz submission that Canvas reports `complete`; a quiz total or an assignment `score` alone is insufficient. For `pending_review`, expose manual questions not already confirmed by CE-SE results for that exact instance/course/assignment/quiz/student/attempt. A partial result must not suppress the remaining questions.

Implement a small local scan of `classic-quiz-results` files using `DocumentKind`, existing result outcomes, and identity fields. Scan once per Classic pull and collect successful/preserved question IDs by student and exact attempt; no persistent index. Also report paths of matching results with unresolved requested timing so the agent can finish those existing results rather than rescore the questions. This reporting applies even when Canvas now marks the attempt complete. Do not re-open completed questions just because the teacher edited feedback/points afterward; explicit reassessment is the route for that.

Keep every current question in a generated work document as context; `unhandled_question_ids` communicates the remaining local grading workload. If no grading remains, count the student as handled and report any unresolved timing result path separately; do not generate an empty grading task just for timing. Preserve the ordinary results scan/behavior. No new task scheduler, requeue system, or permanent gradebook.

## 8. Post algorithm and outcome rules

### Dispatch and local preparation

At `Service.Post`, inspect the kind first and call `postClassicResults` for the new kind. Keep ordinary behavior otherwise. Classic posting loads/validates all local sources before any write, including the stable private student lookup. Do not trust agent-supplied Canvas user IDs; there is no such field.

Check result/packet instance, course, assignment, quiz identity and `SameGradingBasis`; each result attempt must match its student's pulled current attempt. Selected question IDs must exist in that attempt. Optional strongest-prior identity must identify a supplied earlier attempt, but the Engine does not judge whether it was actually strongest.

If every requested question and requested timing action has a definite outcome, return the saved outcome report. Do not contact Canvas solely to recertify already-completed work.

### Live checks and writes

1. Fetch the live assignment and quiz once for this post. Confirm their relationship and `SameGradingBasis` against the packet/result. A changed points/rubric/grading mode holds the affected quiz work. Title, directions, formatting, dates, and publication alone do not invalidate question scores; due changes affect timing only.
2. For each unfinished student, read the live top-level assignment submission and exact linked quiz submission. Check the same identity chain and current finished attempt as pull. A failed check holds that student's unfinished actions and continues other students.
3. Fetch current attempt questions and response history once. For each requested unfinished question, confirm its ID/type/maximum still matches the pulled question. Changes hold that question. Do not compare entire question objects or prompt HTML at post; educational changes belong to the agent's reassessment judgment.
4. `unknown` evidence or a missing current response row holds that question with its precise reason. Other ready questions still proceed.
5. For a fresh or definitely failed action, an already equal score and rendered comment can be recorded `preserved`. Otherwise include its explicit desired score/comment in this student's single question-map PUT. An existing higher score does not suppress an honest assessment. Ordinary fresh reassessment does not require preserving prior scores/comments.
6. Before PUT, mark included questions `pending` and save the result. Send a single request for that student/attempt, containing only the ready selected questions. No grade/rubric/fudge/comment creation fields. Stop before writing if this local save fails.
7. After a successful or uncertain PUT, perform one fresh assignment-history read and quiz-submission read. Confirm selected per-question points and rendered `more_comments` on the intended attempt. Use decimal equality and only CRLF/LF normalization for comment comparison; do not trim or rewrite meaningful feedback. Save `succeeded` for confirmed actions and `uncertain` for unconfirmed actions. A 200 without confirmation is not success. Independent questions can have different outcomes.
8. After a definite API refusal, store `failed` on included questions and continue independent students. Do not split a refused student batch into speculative single-question retries in this change.
9. Apply optional lateness separately, then save/report its outcome. A due-date timing hold leaves confirmed scores/comments intact. A newly started attempt holds timing.

Report the observed Canvas quiz score and `kept_score` separately, when available; they can differ because of Canvas attempt policy. Label them as observations, never as values CE-SE promises or needs to force. Report Canvas's pending-review state when some writing remains.

### Repeat and uncertain recovery

Use existing `ActionOutcome` values: `succeeded`/`preserved` are definite for question actions; `not-requested` is for absent timing only. Set `ID` to the question ID and `PostedText` to the exact rendered comment when marking a write pending or confirmed; keep them on uncertain outcomes. Do not introduce another persistence record.

On repeat, skip definite actions. For `pending`/`uncertain`, read before writing: matching score/comment on the intended attempt confirms completion; mismatch remains `uncertain` and is not automatically resent, even if it resembles the old baseline. Explain that the teacher can inspect the question and explicitly reassess through a fresh pull/result. No backoff loop, baseline-diff engine, hash ledger, or replay token.

For deliberate reassessment, create a fresh result from a fresh pull without inherited outcomes. Do not instruct the agent to erase saved successful outcomes as a retry technique. If a save fails after a remote action, preserve the earlier file and return known in-memory outcomes with the error, as ordinary posting does.

One pre-write check cannot make Canvas writes atomic against a simultaneous resubmission. Do not claim such a guarantee or build a lock protocol; exact attempt targeting and post-write confirmation are the applicable protections.

### Lateness helper extraction

Extract the body of `postLate` into `postLateTiming(ctx, assignmentSnapshot, userID, attemptRef, target *LateTiming, outcome *ActionOutcome, save func() error) error`. Keep `postLate` as a wrapper for existing callers. Build the same assignment submission endpoint directly from the explicit IDs; do not fabricate an ordinary results/grade object.

Behavior remains: fresh submission read; current attempt/live due checks; supplied timing only; no calendar parsing, score write, or locally calculated penalty. The Classic caller uses this helper after its question actions. Omitted timing is `not-requested`; unknown question evidence does not by itself forbid an independently justified timing adjustment. Existing timing tests must continue passing.

## 9. CLI and teacher-facing output

Extend `validateDocument` for the three kinds. Make Classic validation source-aware at the command boundary by passing workspace root to the relevant helper; ordinary validators need no redesign. Do this before loading credentials in `runRemote` for `post`, consistent with existing local validation.

Extend `PostReport` with `Classic bool` and `QuizStudents []ClassicStudentPostReport`. A Classic student report contains `StudentRef`, `Attempt`, `Status`, `Message`, `Questions []ClassicQuestionPostReport`, `Late domain.ActionOutcome`, nullable `CanvasScore` and `KeptScore`, and `CanvasState`. A question report contains `QuestionID`, normalized `Score *domain.Decimal` (nil for unknown evidence), `PointsPossible domain.Decimal`, and `Outcome domain.ActionOutcome`. Populate it from in-memory results even if the last save failed. Do not force question comments into the existing whole-assignment `Comment` column. For Classic output print:

`Student ref | Attempt | Question | Score/max | Status | Details`

Then one timing/Canvas-total observation line per student when applicable. Keep report errors privacy-safe. Return existing partial exit code `3` when any requested action remains held/failed/uncertain, `0` when requested scope is complete, and the existing error mapping for input/transport failures. Partial requested scope can complete while Canvas still reports other unrequested writing pending review; state that explicitly.

Do not show a private teacher note in a student comment. It may appear in the teacher's local preview/report as a hold reason.

## 10. Playbook and documentation copy requirements

Add a Classic section to `score.md`; update the statement in `quizzes.md` that scoring is unavailable. Keep ordinary scoring guidance intact. State:

- Use the backing assignment ID for `pull-work`; identify Classic packets by kind.
- Read grading voice and student context; judge actual responses against the prompt and official rubric where present. State a consistent mapping in private `scoring_notes`; question maxima already establish the scoring scale.
- Use `unhandled_question_ids` for an unscoped scoring session; honor the teacher's narrower requested question/student scope. Explicit pulls support reassessment.
- Return direct honest question scores and concise feedback. Do not infer equal weighting, average answered-only questions, preserve higher entered grades, or change objective scores.
- Blank required writing is an academic judgment; missing/unreadable API evidence is not zero. Another reading tool may inspect preserved attachments. Unknown evidence holds that question while other work continues.
- Earlier attempts are evidence, not write targets. Select the strongest earlier academic work based on its contents. Report missing prior evidence without inventing a comparison.
- Draft saves locally; show-me-first stops for review; score-and-post authorizes posting. Question comments are posted directly and follow Canvas visibility. This operation does not write a Canvas rubric assessment or a separate submission comment.
- Use existing lateness judgment guidance, keep private context out of feedback, and do not double an extension already reflected in the effective due date.
- Rerun the same results to confirm uncertain outcomes; if unresolved, report them specifically. A fresh explicitly requested reassessment uses a fresh pull/result.

Update `engine-docs/formats.md` with fields, scopes, normalized points, comment prefix, and outcome semantics. Add one complete synthetic packet/work/results example set under `engine-docs/examples/`. Verify it with the implemented parser/source validator; no source-text or Markdown snapshot tests.

After local tests pass, move Classic writing from the red future section to the yellow section in `capabilities.go`, and revise the Classic-authoring description so it no longer says writing scoring is absent. Suggested evidence text: “Classic essay/file-upload question scoring and comments are implemented and checked locally. Canvas Live validation of this CE-SE path and student-visible feedback is pending; OG CE previously verified the underlying endpoint.” Match `docs/engine-capabilities.md` and the actual review date. Do not change authoring/Hub verification claims or turn unrelated lights green.

Update `ARCHITECTURE.md` and `SCORING-RULES.md` only with the concise necessary clarification: Classic writing posts bounded question assessments/comments, Canvas owns quiz totals, and this operation does not write assignment rubric assessments. Preserve ordinary assignment scoring policy.

## 11. Proportionate checks and definition of done

Use `workflowFixture`/`httptest` patterns, synthetic identities, and one small reusable Classic fixture: one objective question, two essays, and one upload. The fixture need only implement endpoints actually used here; it is not a Canvas simulator.

Cover these behaviors with compact tests; cases may share a test/fixture. This is not a quota or Cartesian product:

1. Two-page enveloped quiz-submission read retains all rows and preserves existing pagination restrictions by reuse.
2. Pull joins the correct student/submission/attempt, uses attempt-scoped question definitions, reads deliberately out-of-order history, projects essay/upload evidence and private notes correctly, and preserves existing files on a second pull. Include an unreadable attachment without losing other questions.
3. Validation distinguishes zero from omitted score, rejects out-of-bounds/nonmanual/invented targets, and holds unknown evidence locally. One compact table is sufficient; do not enumerate every YAML syntax error already covered by the parser.
4. Posting a lower honest essay score and question comments leaves objective points unchanged and sends no assignment total/rubric writes. Assert the private sentinel is absent from student-facing payloads. Confirm read-back and per-question outcomes.
5. A stale student attempt holds that student while another posts; a changed selected question maximum holds that question while another question posts. These checks are about identity/real point bounds, not every quiz mutation.
6. An uncertain PUT that actually landed is confirmed without another PUT; differing read-back remains uncertain. An unchanged completed repeat performs no new write. Do not demand exact incidental GET counts.
7. Discovery retains unfinished questions after partial completion, skips completed current work, finds a newer attempt, and explicit pull permits reassessment.
8. Supplied timing uses the extracted helper; a changed due date holds timing but does not undo successful question results. Existing ordinary timing tests continue to pass.

During development run focused tests for changed packages. At integration run:

```powershell
go build ./...
go vet ./cmd/... ./internal/...
go test ./cmd/... ./internal/...
git diff --check
```

Format only changed Go files, preserving the repository's line-ending conventions and avoiding whole-tree churn. No coverage target, benchmarks, source-architecture tests, fuzzing campaign, all-item authoring sweep, or new student-view automation.

Done means the described local workflow is implemented, relevant checks pass, example documents validate, and docs accurately distinguish local results from pending Live evidence. Do not keep adding tests after that without a concrete failing behavior. No teacher workspace certification or live probe is needed to finish this implementation handoff.

An optional future authorized live check should exercise this feature only: one mixed Classic quiz, a synthetic submitted attempt, selected writing scores/comments, retained objective score, and read-back/student presentation. Another attempt is useful only for the actual attempt-selection behavior. Do not fold Hub/New Quiz authoring checks into this feature's completion requirements.

## 12. Agent execution order and delivery report

Use one implementation owner (Sol) for the coupled Engine changes. Work sequentially in four passes within the same task; each pass leaves the build usable where practical. Do not launch separate coding agents against overlapping files.

1. **Contracts and reads:** types/validators, envelope pagination, whitelisted DTOs/read helpers. Add focused local contract/read tests.
2. **Pull integration:** route Classic discovery, project current/prior work and attachments, completion lookup. Preserve ordinary pull behavior.
3. **Post and CLI:** source-aware validation, per-question writer/read-back/outcomes, extracted timing helper, preview/reporting. Run the relevant behavior checks.
4. **Documentation and integration:** fully valid examples, Playbooks, capability record, concise policy clarifications; run the normal integration commands.

Luna may own pass 4 only after the Engine contract is implemented and stable. Its authority is documenting this contract and checking example validity, not changing the API/schema or expanding scope. A single Sol agent can instead finish all four passes. This brief does not require multi-agent orchestration.

Implementers may choose local helper names, straightforward loop organization, and fixture details. They must not choose new teaching policy, add product workflows, rework quiz resume, or invent alternate identity/recovery mechanisms. If an actual contradiction prevents implementation, return the specific conflicting facts and smallest proposed decision to the senior agent; continue independent work. Ordinary coding choices need no approval.

Final delivery report: what the teacher can now do; changed files; checks actually run and their results; local-only versus Live evidence; any remaining concrete limitation. Do not include a speculative hardening backlog. Do not commit, publish, or make Canvas calls unless separately instructed.

### Ready-to-send implementation prompt

> Implement `handoffs/active/CLASSIC-WRITING-EXECUTION.md` in this repository. Read the listed authority files first, then follow its four sequential passes and fixed contracts. You own the Engine implementation and integration; complete documentation too unless a separate documentation agent is explicitly assigned. Keep quiz authoring/resume and Hub code untouched. Use local synthetic tests only; do not access Canvas Live, commit, or publish. Make routine coding choices yourself. Return an actual contract contradiction to the senior agent with the smallest proposed resolution, without expanding scope or stopping independent work. Finish after the brief's relevant checks and documentation are complete, and report actual results without a speculative backlog.

### Ready-to-send documentation prompt (optional, after pass 3)

> Complete pass 4 documentation for the implemented `handoffs/active/CLASSIC-WRITING-EXECUTION.md`. Read the brief and the actual Classic types/validators first. Update only its listed documentation, example, seed Playbook, and capability-status files; do not redesign or alter Engine behavior. Make complete matching synthetic packet/work/results examples and validate them using the implemented offline validators. Clearly label current CE-SE checks as local and Canvas Live/student experience as pending. Do not alter teacher workspace copies, launch Canvas probes, commit, or publish. Report any code/document contract mismatch to the implementation owner instead of inventing a different contract.

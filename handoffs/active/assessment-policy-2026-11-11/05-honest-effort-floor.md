# Slice 05 - Forty-percent floor across genuine efforts

Status: **not started**. Kind: Engine grade entry, quiz discovery and honest feedback rendering. Depends on [02](02-contracts-and-exact-math.md). Read [README](README.md), `docs/SCORING-RULES.md`, `docs/new-quiz-written-work-spec.md`, team routing/consent rules and the group contracts before changing their paths.

## Result

The teacher's configured 40% floor works for Daily and Major, ordinary individual work, individual group work, genuine shared artifacts and MC/written/mixed New Quizzes, including tutorial companions. The earned item/rubric assessment remains visible and accurate. Missing/blank/placeholder work receives no floor. A pure feedback comment remains a pure feedback comment.

The value 40 is this teacher's selected configuration, not a hardcoded Canvas scoring rule or a migration of every CE-SE installation. Keep the existing `effort_floor` field, its percentage validation and teacher control. Slice 08 changes this teacher's current 50/50/unset course values; this slice implements the supported behavior for any valid configured value.

## Existing code and preservation requirements

- `internal/domain/effort_floor.go`: `EnteredGrade` already computes a rounded minimum from assignment points using rational math.
- `internal/domain/types.go`: `Course.EffortFloor`, `GradeOperation.HonestEffort`, `CommentOnlyOperation`; no current quiz floor request.
- `internal/workflow/post.go`: `postGrade`, floor lookup and entered-grade calculation; `review.go` preview/render paths must agree.
- `group_post.go`: individual rows already use ordinary floor rendering/posting; `postGroupShared` currently posts `g.Assessment` directly. `GroupGradeOperation` currently lacks an effort judgment.
- `new_quiz_item_scores.go`: exact item writes, result-version baseline, observed score/graded-at checks, stage receipts and `QuizScore`.
- `internal/canvas/newquiz_grading.go`: complete native result, current fudge points, native host/token boundaries, `PostScores` and no automatic retry.
- `pull_work.go`: `needs_grading_count == 0` skip, response/report pulls and written-only result-version capture. `results_index.go`/completion hints and session summaries govern repeat discovery.
- `internal/render/render.go`: feedback entry note, `partText` and `itemComment` exemplar labels/thresholds; validation examples and Draft PDF must match posting.

Do not change truthful item scores to reach a class minimum. Do not write a whole-quiz total by adding written-part scores while auto-scored items are missing. A no-response quiz with a 0 in Canvas does not establish genuine effort.

## A. Ordinary and group work

1. Keep ordinary `EnteredGrade` as the single floor arithmetic helper. Confirm every grade-entry/preview path uses the same resolved course value and explicit effort flag. A floor note reports earned and entered points separately.
2. Preserve Daily revisions never lowering the prior entered grade. The playbook may use existing comment-only feedback for a lower-scoring revision. Do not silently implement this teacher's choice for all other teachers or inflate an assessment to match the retained grade.
3. Individually graded group rows retain individual flags and outcomes. Add a focused regression only if the new contract/render integration changes their current path.
4. Shared-grade assignments assess one shared artifact. Add an explicit `honest_effort` judgment for that shared artifact, use `EnteredGrade` once for the shared grade, and render earned rubric points plus the shared entered-grade note. Do not pretend it proves individual contribution.
5. Retain existing complete membership/artifact/attempt/ready-audience guards. A mixed or unknown per-member effort situation that cannot truthfully be represented by one shared grade holds that group numeric action; it does not justify silently switching Canvas to individual grading or granting a floor to a placeholder member. Continue independent groups/feedback.
6. Update pre-write match/reconciliation, ordinary/shared grade payloads and rendered previews to compare the intended **entered** value, not only the earned assessment. Rubric payloads remain earned. Preserve coupled-rubric ordering; a rubric write must not overwrite the policy-adjusted grade.
7. Keep Canvas late deductions separate. No new deduction is introduced on later attempts, and no claim of a guaranteed post-late 40 is made. Unresolved timing does not become an excuse to invent academic evidence.

## B. New Quiz floor mechanism

First settle the smallest actual Canvas write that changes the entered quiz grade while preserving earned items and teacher adjustments. This is a bounded protocol task, not a new prerequisite for ordinary authoring, feedback, Daily corrections or ordinary floors.

Preferred integration is a separately recorded **policy grade adjustment**, using the existing native result path only after its adjustment semantics are established. Existing Live evidence proves preservation of `fudge_points` during item grading, not CE-SE-authored floor adjustments. Inspect current public API/source and captured result shape. A documented associated-assignment grade override is another candidate only if its New Quiz synchronization behavior is established. Do not present a fake-server acceptance as proof that the real Canvas grade stays adjusted.

If a newly required undocumented write shape has not been established, finish the domain/adapter/fixture work and report the specific remaining protocol evidence. The teacher can enter that particular floor in SpeedGrader until the path is supported. Do not block the other slices or publish a false end-to-end capability claim. No Dev Live write is authorized by this plan itself.

Required semantics regardless of protocol:

1. An explicit policy-adjustment request and evidence-backed effort flag activate the floor. A normal `comment-only` feedback row without that request never changes a grade.
2. For mixed/written quizzes, finish truthful written-item points first. Require every relevant item to be actually graded; an incomplete result holds only the floor/grade-application stage. Keep accepted item scores and comments.
3. For MC-only quizzes, read the complete current attempt and exact result version too. Validate all scored-item contributions/possible points and current academic total. Do not manufacture `item_scores` or a scoring basis merely to reach the floor stage.
4. Compute the floor using the quiz/assignment points relationship Canvas actually uses. If native `points_possible`, assignment points and item totals differ, use the established scale mapping explicitly or hold the adjustment; never assume 100 or use an unverified denominator.
5. Keep current manual adjustments intact. If using native fudge points, add only the policy delta required to bring the current confirmed academic grade to the floor; retain the original adjustment and record the CE-SE delta separately in the existing local outcome. Do not replace the whole adjustment with `40 - item score` or repeatedly add the same delta.
6. Recheck exact student, course, quiz, attempt, source result version, observed score/graded-at and relevant current grading state just before the numeric write. A SpeedGrader change since the pull holds that student's adjustment, including a comment-only save that changed the result version.
7. Model the numeric adjustment as a distinct pending/accepted/held/uncertain outcome from item points and whole-assignment feedback. Preserve one accepted result-version write and its baseline. On uncertain completion, reconcile bounded current evidence rather than creating another version blindly.
8. A score already at/above the floor needs no numeric write. Record the policy review as handled for that attempt/version without treating mere feedback as completed scoring. A later new attempt or relevant teacher grade change must be discoverable on explicit review.
9. Render the honest earned assessment and the entered minimum clearly, for example `Earned: 23/100. Entered as 40/100 under the honest-effort minimum.` Retain current New Quiz item/auto-score formatting and do not mislabel a written subtotal as the complete quiz assessment.
10. Keep owned-course `(CE)` consent, routed credentials and hidden posting rules for all numeric/comment stages. Neither a floor nor a companion is permission to reveal an already hidden grade or overwrite an excused/held student.

## C. Auto-scored-quiz discovery must actually reach the floor

A fully auto-graded quiz can have zero Canvas needs-grading count, so merely extending item-score posting is insufficient.

Implement a bounded quiz-policy candidate path within the existing targeted session flow. The course assignment inventory is already available. For open/current eligible New Quizzes with a configured floor, read current submissions in batches using the existing Engine transport and select submitted current attempts below the floor that have not already received a completed policy review for that current evidence. Respect the existing closed-period exclusion and requested course/assignment scope. Do not perform an unrelated historical grade sweep or fetch every student's attachments just to discover candidates.

Candidates must be labelled `quiz policy review` (or equally clear wording), separate from Canvas `needs grading`. A teacher/agent still judges effort from pulled response evidence. On-level zero work, blank attempts and unknown evidence are not auto-promoted. Results/completion hints can remember a reviewed non-effort attempt so it does not repeat unchanged every session; they are local session convenience, not a permanent synchronized gradebook.

For a targeted auto-scored quiz, permit a fresh policy pull even when Canvas says no grading is needed. Save result-version baselines for those candidates regardless of whether written items exist. Keep ordinary session needs-grading behavior and New Quiz item-points reminders intact; do not equate `comment posted` with `floor applied` or `written items graded`.

An unknown total/report or an inaccessible quiz produces a specific held/report row; other assignments continue. Keep privacy: stable refs/aliases and selected response evidence only, no raw roster identities.

## D. Exemplars and rendering

Remove the unconditional `Hand-copy exemplar` wording from the shared renderer. A neutral `Exemplar`/`Model answer` label is valid for both categories; Daily credit directions belong to the authored feedback/playbook, not an Engine assumption.

Provide explicit display intent if retaining the historical below-90% default for old result files. For example, an optional `exemplar_visibility: always|below_threshold` can preserve backward compatibility while new Major/Daily instructional feedback chooses `always` where it promises the exemplar. Use the same choice in item comments, whole-assignment blocks, validation and PDFs. Alternatively use the existing class follow-up comment-only route for universal exemplar delivery, but the playbook must not promise an exemplar that the chosen route silently suppresses. Document the one coherent delivery contract instead of adding multiple overlapping switches.

## Meaningful tests

- Ordinary genuine 22/100 -> entered 40; unchanged earned rubric/comment scores. Blank/non-effort 22 stays 22. A 20-point task gets an 8-point minimum.
- Shared genuine artifact gets one floor-adjusted grade sequence for its exact ready audience; no duplicate per-member shared grade writes. Individually graded rows remain individual. A mixed/unknown shared audience does not receive a fabricated floor.
- MC-only candidate with `needs_grading_count: 0` is discoverable; an above-floor or already-reviewed unchanged attempt is not repeatedly pulled. A new attempt returns. A closed period remains report-only.
- MC-only, written-only and mixed fixtures apply one explicit floor adjustment without changing earned item points or existing teacher adjustment. Pure feedback-only results send no numeric write.
- Partial item grading holds only the floor stage. Teacher result-version change holds the numeric write. Accepted/uncertain outcomes are not blindly replayed.
- A non-100-point quiz uses the supported actual scale. Unavailable denominator/entered-score evidence holds rather than guessing.
- Neutral exemplar labels and explicit universal delivery work at 20% and 95% through the actual changed rendering paths.

Use a compact set of stateful fake-host tests, not every format times every failure. Reuse existing floor, item-score, result-version, group and session fixtures.

## Completion

Update `docs/SCORING-RULES.md`, relevant group/New Quiz contract paragraphs, formats/examples, changed CLI/MCP schemas, session report wording and both capability records. Retire the old blanket `no New Quiz floors`/`shared mode has no floor` statements only to the extent the new path is implemented. Record protocol evidence, supported fallback and pending Live/student-view facts separately.

# Slice 07 - Integration, validation and capability evidence

Status: **not started**. Kind: complete teacher flow and honest capability accounting. Depends on slices [01](01-playbooks-and-policy.md)-[06](06-retake-grade-application.md). Read [README](README.md) and `handoffs/README.md`. No Canvas Live writes, release or teacher-install changes are authorized by this file alone.

## Result

The implemented pieces work together through the actual typed tools and CLI: author a Major plus companion; expose passage-only reading; release named tutorial recipients within four school days; score honestly with the 40% effort floor; apply at most 70% to the original without lowering it. Daily corrections still work and do not acquire the Major restrictions. Every advertised operation is reachable and accurately documented.

## Adapter and reference integration

Review the current versions of `cmd/cse/main.go`, `remote.go`, command/help/arity tables, `internal/mcpserver/operations.go`, `pulls.go`, `server.go`, document validation/dispatch, reference examples and `scripts/check-mcp.mjs`.

1. Register any new document kind in load/validate/render/post dispatch. Do not allow `retake-results` to fall through ordinary scoring validation or arbitrary assignment writes.
2. Expose explicit tutorial release and retake preparation if new commands were actually necessary. Prefer typed extensions to existing tools; keep read-only preparation annotations separate from write annotations. Update allowlists, help, arity and MCP JSON schemas together.
3. Expose new QuizForge settings in the strict settings validator and effective teacher summary. Unknown keys should still fail rather than silently omitting no-audience/cap intent.
4. Preserve existing `push_quiz`, ordinary `pull_work`, feedback-only comments, item-score results, `post_results`, Draft PDFs and family review. Do not add a new confirmation token or a second review page.
5. Report each object/stage separately: original created, companion prepared, reading published/not requested, tutorial release, source scored, floor applied/not needed, original grade raised/preserved/held and comment posted. A single `completed` label must not conceal an unassigned/exclusion/publication failure.
6. Keep access codes and private grading/accommodation baselines out of student handouts, public status/capability responses, family outputs and student feedback. Teacher-only code handoff is explicit.
7. Validate original/companion IDs and exact points in rendered samples. The companion's non-counting state and original target are visible to the teacher before any grade application.

## Small integrated synthetic scenarios

Use existing stateful fake Canvas/native server helpers. Two main scenarios plus focused safety regressions are sufficient; do not build a universal classroom simulator.

### Scenario A - Major mixed assessment

Create an original Major in a configured course with floor 40. Provide a reviewed parallel mixed quiz and passage-only Page. Verify:

1. Original creation succeeds once; companion contains new passage content and linked original identity, exact title, code-required settings, unpublished/zero-recipient state, exclusion from final grade and SIS off.
2. Reading Page can be published/placed independently. It contains no quiz questions/key/code; it does not change quiz audience/publication.
3. Named tutorial release uses the original student-effective deadline plus four school days and appropriate saved accommodation; no Everyone audience or unrequested module audience appears.
4. Source written items/auto-scored items produce complete honest totals. A genuine low attempt receives an explicit 40 minimum without changing earned criterion/item scores. An above-floor 88 stays 88 in source assessment.
5. Source 88 plus original 52 -> original entered 70. Original 84 remains 84. A teacher edit since preparation holds that student while another succeeds.
6. Inject one target-comment failure after accepted source/target writes; resume produces no duplicate source version, original adjustment or gradebook column.
7. A New Quiz original uses the implemented target adjustment route without overwriting original items or teacher fudge points. This may be a focused extension of the same fixture rather than a second full authored unit.

### Scenario B - Daily formative work and auto-scored floor review

Use a Daily written assignment and a Daily MC-only quiz in the same course:

1. Written feedback offers permitted revision/exemplar work and the existing credit arrangement, with no tutorial-only requirement or 70 cap.
2. A genuine 22/100 ordinary effort enters 40 with honest rubric scores. A lower later revision receives honest feedback while the prior higher grade stays under this teacher's preference.
3. A submitted MC-only current attempt below 40 is discovered as quiz-policy review despite `needs_grading_count: 0`. A blank/non-effort attempt stays unraised; a genuine attempt gets the floor; unchanged handled attempts do not reappear.
4. A new MC attempt becomes eligible for fresh review. Closed periods and unrelated assignments keep current discovery boundaries.
5. Exemplars are delivered at both low and high scores and no renderer forces handwriting-for-points language.

### Consequential safety regressions

Cover one wrong-origin/course association, a source new-attempt/result-version conflict, an unknown original entered grade, an excused/hidden student, and a shared-group original that would otherwise propagate one member's retake award. Use existing group/team fixtures to show preserved audience/consent/hidden-post behavior; do not reproduce their entire historical test suites in new files.

## Required completion checks

Run focused tests while implementing. Once the integrated slice is ready, run these sequentially, stopping to fix failures:

```powershell
go test ./...
go vet ./...
$assessmentCheckEngine = Join-Path ([System.IO.Path]::GetTempPath()) ("cse-assessment-check-" + [guid]::NewGuid().ToString("N") + ".exe")
go build -o $assessmentCheckEngine ./cmd/cse
node scripts/check-mcp.mjs $assessmentCheckEngine
git diff --check
```

This is a temporary Dev check binary, not release packaging. Record the actual discovered tools/schema results; do not hardcode a tool count copied from another handoff. No classroom Canvas call is needed to make an MCP smoke test pass.

Review active policy references with targeted `rg`. Check every seed playbook and teacher-context sample for Major copying credit, blanket `No retakes`, response-length eligibility, universal below-90% exemplar assumptions and unconditional correction timetable wording. Preserve legitimate assessed MC+Why reasoning, Daily formative credit, STAD awards and parent-update restrictions. Historical handoffs remain historical.

## Capability documentation

Update `internal/localui/capabilities.go` and `docs/engine-capabilities.md` together, with the review date. Replace the outdated blanket quiz-copying default with the supported Major/Daily distinction. Document separately:

- Same-course companion authoring, exact relationship/title, access code and no-audience preparation.
- Non-counting/SIS-off companion settings and tutorial-only selected release.
- Passage-only advance reading and its independent audience/publication.
- Four-school-day original-effective-date eligibility and actual accommodation support.
- Effort floor on ordinary, group and quiz paths, with the pre-late meaning explicit.
- MC-only policy-candidate discovery and completion behavior.
- Original grade application with 70 cap, no lowering and original-type/group limitations.
- Honest scores, universal exemplar delivery and neutral labels.
- Manual fallback only for specifically unsupported numeric/access behavior, without a blanket prohibition on New Quizzes or ordinary teaching work.

A local fixture proves payload/order/holds/resumption, not stored Canvas values, student access, provider delivery or installed-client adoption. Keep old observations where still relevant, but do not use a 2026-10-29 item-score check as proof that a new floor/retake adjustment works.

## Bounded Canvas Live observations, when explicitly requested

Use a disposable course/object and an approved test identity. Existing connection access is not permission to change real grades. Scope and state the exact objects, writes and cleanup first as the development guidance requires. Needed observations:

| Observation | Evidence to capture without private data |
|---|---|
| Initial companion state | Same course, unpublished, empty effective audience, code requirement, non-counting flag and SIS off; title/IDs and accepted settings, no secret value in logs |
| Advance reading | A real student/test identity can read the Page/file before code release while the quiz remains inaccessible |
| Tutorial release | Named recipient can enter with code; unselected student cannot; supported saved accommodation takes effect |
| Complete New Quiz floor | A genuinely attempted synthetic low result gets the entered minimum without changing earned item points or a pre-existing teacher adjustment; gradebook/native result behavior is understood |
| Capped application | Synthetic retake 88/original 52 applies 70; an original 84 stays 84; ordinary and New Quiz original mechanisms have distinct evidence |
| Late/posting semantics | Prior late state and hidden posting survive where applicable; no second late charge or notification claim is invented |
| Feedback/student display | Glows/grows, exemplar, earned/entered distinction and the original comment actually display as intended |

An authorized Dev probe may perform bounded readbacks to establish unknown behavior; normal production accepted writes remain done without a verification ritual. Do not unpublish/delete an object once real students have work. Preserve synthetic evidence needed for a partial/uncertain result, and clean up only the explicitly disposable objects within scope. No family sends are involved.

Unknown student view or mobile presentation is recorded as pending, not a blocker for already supported operations. Unknown numeric write semantics hold only that unsupported write and its capability claim.

## Completion record

Record the implementation commits/files, actual suite/smoke results, synthetic scenarios covered, Live observations actually performed and remaining per-capability limits. Link slice 08 for adoption; do not mark the teacher's running installation updated from a passing Dev test.

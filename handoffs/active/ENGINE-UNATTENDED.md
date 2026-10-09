# Unattended Engine validation and repair

Prepared 2026-10-09. Planning result, not an executed live sweep. Read with `ENGINE-VALIDATION.md`, `ARCHITECTURE.md`, and `SCORING-RULES.md`.

## Feasibility

Most Engine development checks can run unattended. The existing developer tool, `cmd/cse-probe`, already targets CS8/course 121046 and Canvas's built-in Test Student, with synthetic text submissions/resubmissions, direct fixture grades/comments, comment editing, rubric coupling, due-date changes, publication, and shared workflow pull/post entry points. It is a stepwise tool, not an unattended runner.

Historical CS8 evidence records successful student-view submissions and two real Canvas attempts; teacher submission-on-behalf and grading while unpublished returned 403. These are leads to recheck, not guarantees about today's permissions. The automated route should try the existing student-view path, not request a student's credentials.

Current reconnaissance: the installed Engine is linked to this repository's `dist/cse.exe`; its version is `2d49715d2c0b+dirty`, built 2026-10-09T13:40:39Z. A live CS8 modules read succeeded on 2026-10-09; eight modules were returned, with no ZZ Sweep module. `go test ./...` passed (most package results were cached). No live writes were performed for this feasibility review.

| Area | Unattended coverage | Remaining constraint |
|---|---|---|
| Build, local validation, exact math, rendering, privacy boundaries | Available now through existing tests and CLI examples | A passing test suite is not live Canvas proof. |
| Unpublished assignment creation, rubrics, edits, repeat/resume | Available through production CLI; diagnose and repair the reported defects | Use clean named fixtures and fresh live reads. |
| Module placement | Production operation can run unattended | Create a dedicated module through a narrow fixture helper or have one ready; module creation is not a new product requirement. |
| Synthetic text, placeholders, resubmissions, attempt history | Existing Test Student probe can automate these if current permissions still allow | Current probe owns one persisted assignment; a few separate fixtures need small development-tool changes. |
| Grade/rubric/comment posting, revision, timing, repeated posts | Shared workflow available through probe; production post can be verified against Test Student | Historical unpublished grading denial means a published test fixture may be necessary. Scope publication before launching the run. |
| File submissions and downloads | Local attachment tests exist; live uploads are API-supported in principle | Probe currently submits text only. A narrow upload helper and current permission check are needed; otherwise save one attachment exercise for a student. |
| Conflicts and recovery | Mix of live fixture edits and existing controlled HTTP tests | Deliberate connection loss/uncertain responses belong in local tests; do not manufacture unreliable production networking. |
| Ordinary roster discovery and completion discovery | Existing controlled HTTP tests; live read-only ordinary discovery where useful | Production excludes Test Student. The special pull entry point does not prove ordinary enrollment/discovery behavior. Finish with one real student's actual submission/resubmission. |
| Student-facing experience and teaching quality | Synthetic content can test mechanics and instruction-following | A short teacher/student check is still useful for actual presentation, usability, and feedback voice. |

Canvas documents file-ID submissions and a submission file-upload route: <https://developerdocs.instructure.com/services/canvas/resources/submissions>. This supports investigating a small test helper; it does not establish district permission. Do not depend on backdated submission timestamps: use actual successive submissions and fixture due-date edits, read back the effective due date, and use fixed timestamps in local calendar tests.

## Long-session execution shape

Use an agent-led sequence of existing commands, focused tests, and bounded repairs. No new job engine, MCP layer, general-purpose API console, or large test framework is needed.

1. Establish a separate development workspace from the current seed, configured only for CS8. Keep its files and outcomes separate from the teacher's unsent work. Use an explicitly identified executable, recording its revision and hash for each verified capability. Preserve unrelated checkout changes.
2. Run the existing local checks once. Verify live origin/course identity and inspect existing test resources before creating anything. Old IDs are lookup leads, not authorization to alter an arbitrary existing object.
3. Diagnose the reported rubric maxima and create/resume mismatch from the smallest reproducible case. Delegate focused coding changes after the session's direction is accepted. Verify through actual production CLI operations and fresh Canvas reads.
4. Use a small fixture set: rubric assignment, rubric-free assignment, and file-upload assignment if needed. Add one dedicated module. Persist IDs immediately. Extend the developer helper only as required for selecting these exact fixtures, creating the module, or uploading a synthetic file.
5. Create synthetic text attempts with explicit expected academic scores: placeholder, genuine initial response, stronger/weaker revision, and harmless instruction-like text treated as student data. These require no student action or waiting for school dates.
6. Exercise pull -> prepare results -> validate -> post -> fresh read -> revise feedback -> fresh read. Use production `cse` operations wherever supported; use the Test Student pull exception only where necessary. Probe direct `grade` is for arranging an existing-score fixture, not evidence that production `post` works. Likewise, direct probe create/attach does not prove production authoring.
7. Check differing assignment/rubric scales, a computed score below the existing grade, truthful fractional criterion values, local scoring basis, late timing, preserved unsent work, and repeat handling. Check changed grading basis/current attempt and due-date conflicts using controlled changes to the test fixtures. Use local multi-student tests for independent-row continuation.
8. Run relevant regressions after each repair; rebuild the executable used for the live check. Finish with the required build/test/vet checks. Recheck affected live behavior after subsequent changes that touch it, without repeating unrelated successes.
9. Leave a compact capability record and a short final handoff. Include any published test fixtures and exact IDs requiring later cleanup. Never promise that an assignment with submissions can be unpublished again.

## Scope to settle once at launch

The teacher has already made CS8 available for testing and assignment creation. The launch instruction should explicitly cover the remaining student-visible test actions so a long session need not pause at its first grade write:

- Create clearly named disposable CS8 fixtures and a dedicated module.
- Publish those fixtures if needed; post grades, rubric assessments, feedback and lateness changes only to the built-in Test Student on those exact fixtures.
- Arrange synthetic submissions and fixture state through the existing student-view route; do not impersonate an ordinary enrolled student.
- Configure grading fixtures to omit them from final grades and SIS posting, and verify those settings. Publishing can still expose a test assignment to enrolled students; do not claim Test Student isolation means invisibility. Keep the fixture clearly labeled as a test and verify narrower visibility if available.
- Make focused code/playbook fixes, rebuild, and repeat the affected check without returning for routine implementation choices.
- Leave existing coursework, real student grades/comments, course-wide policies, and credentials untouched. Do not delete existing resources or broaden into new product features.

This is a proposed launch scope, not a claim that the feasibility request itself asked to execute every listed write.

## Failure handling while the teacher is away

An actual Engine defect gets a focused repair and rerun. A permissions denial or unavailable student route gets one clear blocker entry; continue all independent work. Do not repeat an unchanged failing write, bypass permissions, or wait for the teacher when another slice can proceed. Ask only when the remaining work requires an unresolved consequential choice or expanded scope. At the end, collect those decisions in one short handoff.

Maintain `docs/engine-capabilities.md` during execution: capability, verified result, build/date, fixture/evidence reference, and material limit. Distinguish **live verified**, **local verified**, and **pending**. Keep raw logs separate and bounded; promote successful behavior into the record. Retain only the current blocker and a brief fix reference, not a chronological failure history. Include no secrets, real names, real student writing, or private notes.

## Small human finish

After the unattended run, prepare exact assignment links and a short real-student exercise: text submission, file upload if not already covered live, and one resubmission after the Engine has recorded completion. This checks normal enrollment/discovery and student-facing presentation. Ask for the participating alias/student_ref only when preparing this exercise. Do not make the entire long session depend on it.

## Suggested launch message

> Run the unattended Engine validation plan in handoffs/active/ENGINE-UNATTENDED.md. CS8 is authorized for clearly named disposable test assignments and a test module, including publication where needed. You may create synthetic submissions and post grades, rubric assessments, feedback, and lateness changes for Canvas's built-in Test Student on those fixtures. Omit grading fixtures from final grades and SIS posting; I understand published test assignments may be visible. Make focused repairs, rebuild, and verify them. Delegate bounded coding tasks and continue independently when one check is blocked. Preserve existing coursework and real students' records. Keep a concise record of verified capabilities and finish with the remaining human checks. No MCP work.

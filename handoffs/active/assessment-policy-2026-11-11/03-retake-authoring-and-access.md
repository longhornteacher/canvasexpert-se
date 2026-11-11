# Slice 03 - Retake creation, access and tutorial release

Status: **not started**. Kind: Engine authoring and selective release, with playbook integration. Depends on [02](02-contracts-and-exact-math.md). Read [README](README.md), `docs/team/README.md` and the relevant audience contracts before implementation.

## Teacher-visible result

Creating a Major creates two independent objects: the normal original and its protected tutorial New Quiz. The companion is in the same course, has the exact required title, has a code, is assigned to nobody, remains unpublished and cannot count as a second Major. A later request opens it only for the named tutorial students and their supported dates/accommodations.

The Engine does not invent the new assessment. The authoring agent supplies the reviewed parallel quiz, models and mapping under slice 01. A failed companion step does not delete an accepted original or cause a second original on resume. Report the exact unfinished companion step and preserve both drafts/IDs.

## Current integration points

- `internal/workflow/quiz_author.go`: `QuizSettings`, `settings`, `quizSettings`, `PushQuiz`, `quizSidecar`, `quizStages`, `finishQuiz`.
- `assignment_dates.go`, `quiz_audience.go`, `audience*.go`, `assign_to.go`: existing exact assignment overrides/visibility and outcome semantics.
- `quiz_saved_accommodations.go`: `planSavedQuizAccommodations` and publication guard; an unpublished restricted quiz currently derives its audience from its own overrides.
- `internal/domain/assignment_dates.go`, `validation.go`: after-create empty restriction is possible, ordinary assignment create with no override is refused.
- CLI/MCP quiz push and document routes, help/formats/examples, draft-PDF/student handout rendering.

## Build steps

### A. Pair the original and companion without duplicate creates

1. Validate the requested original relationship using the routed course client. A written assignment is a valid original; the companion is always New Quizzes. Confirm course/instance, existing original ID, points and the teacher/agent's Major classification.
2. Use two local drafts and their existing create/sidecar records. Reuse the accepted original ID. Store the confirmed original and companion association in the companion sidecar immediately when each ID is known; do not identify a pair by a prefix search alone.
3. Derive the exact companion title from the original name at authoring. An original title rename later does not redirect an existing pair; substantive content/category changes trigger a reviewed alignment decision in `edit.md` rather than automatic replacement of work.
4. If an exact linked companion exists, resume its unfinished stages or report its accepted state. Do not overwrite teacher edits, duplicate an already-complete quiz or destroy a quiz containing submissions. An unrelated same-title quiz gets the existing useful ambiguity explanation.
5. For newly authored pairs, keep the original unpublished while preparing the companion, then complete the originally requested publication only after the companion's required protected state is ready. Coordinate this through existing create/edit requests and saved IDs; do not build a universal two-object transaction system. A companion failure produces a specific partial result and resumable drafts, not a falsely completed Major or deletion of the accepted original. Existing already-published originals are not unpublished merely because their companion is missing.

### B. Establish the initial protected state

1. Generate/accept an agent-chosen nontrivial code, save it only in the teacher-only local authoring record and send it through existing `quiz_settings.require_student_access_code`/`student_access_code`. Fail local validation on blank/whitespace-only code. Do not print it in ordinary tool logs or student artifacts.
2. Create unpublished using the existing quiz path. Add supported item/stimulus content and normal feedback; do not append Daily copying-for-points language. Keep the original default single-attempt behavior separate from the companion.
3. Extend the finish stage with explicit unassigned intent: set the associated assignment to `only_visible_to_overrides: true` with no direct recipient rows. On a fresh unpublished quiz, inspect whether Canvas unexpectedly supplied recipients and reconcile only the Engine-created no-work state. On resume, hold if the teacher has since published/assigned it or student work exists; do not remove real recipients/work blindly.
4. Track accepted unassigned state in the existing sidecar/stages. Do not manufacture an empty tag, test student recipient, empty section or native group assignment. Do not relax the general rule that a requested club audience must be nonempty just to implement a deliberately unassigned retake.
5. Leave the protected quiz out of shared/student modules during initial preparation. Module-derived audiences can defeat a naive `no direct overrides` claim. If a requested module would grant access, explain and hold that placement rather than broadening the audience. The separately published reading Page may be placed normally.
6. Require the companion to be excluded from the final course grade and SIS passback. Prefer explicit associated-assignment `omit_from_final_grade: true` plus `post_to_sis: false`, exposed through the narrow quiz settings/finish path. Check the then-current official assignment API before implementation; extend typed fields rather than using arbitrary HTTP writes. If Canvas cannot support exclusion in the relevant course, report the specific teacher-directed alternative (such as an existing verified 0%-weight group); do not silently create another counting Major or a new group.
7. Record initial preparation complete only when item content, code request, no-audience request, non-counting settings and origin/pair identity have definite outcomes. Normal accepted writes are complete without routine post-write readbacks. Report accepted settings versus actual student-access observations honestly.
8. Do not apply saved accommodations to all active students while this quiz has no audience. An intentional empty audience produces `not-requested`/no recipient changes at this stage. Preserve the later publication guard; empty preparation is not evidence that future tutorial students need no accommodation.

### C. Release for the named tutorial students

Use a small explicit tutorial-release document through `push_document`, or extend the existing exact assignment-dates path with the confirmed retake relationship. The actual name is chosen consistently with slice 02. It must be reachable through typed tools and must not be a hidden helper.

Required input: confirmed original/companion identity, stable student refs, the exact original-effective-date eligibility record from slice 04, requested tutorial availability and a baseline of the companion's current recipient/publication state. The teacher's ordinary task request supplies write scope; no separate approval token is needed.

Operation order:

1. Re-read exact original/companion identity and current recipient/publication/submission facts using the routed course client. Confirm code requirement and non-counting settings have not been removed. Keep the code teacher-only.
2. Confirm every requested student is an eligible active recipient in the original, not excused/hidden, and still inside the supported four-school-day window. Do not treat a section/group ID as permission to assign Everyone. A held student's record does not block independent ready students.
3. Create or update only the requested individual overrides and dates. Preserve unrelated accepted tutorial recipients and their submitted/saved work. This flow does not revoke students merely because a new tutorial batch is smaller.
4. Resolve/apply saved time/reduced-choice accommodations for the actual recipient set using existing mechanisms. Confirm timer values and dates; never expose private flags in the quiz or feedback. Unresolved accommodation/identity holds publication for the affected release under existing safe-publication semantics; do not bypass with a global skip by default.
5. Publish only when this explicit release requests it and necessary prior stages succeeded. Code protection remains enabled. Hand the code privately to the teacher with the tutorial preparation summary, not to a student-visible Page or automatic message.
6. Record per-stage outcomes so a publication failure resumes publication without adding duplicate questions, recipient rows or gradebook columns. If a formerly complete stage was changed externally, preserve the teacher's later edit and request a fresh narrow baseline rather than replaying old intent.

The code restricts starting access, not student location and not necessarily an already-started session. Do not claim it proves in-person attendance or stops a student who already opened an attempt. Teacher tutorial supervision and the specified availability window remain part of the procedure. Do not build code rotation or session-revocation machinery absent a real need.

## Focused checks

Use fake Canvas/native hosts with existing fixtures:

- One ordinary written original and one MC original produce same-course New Quiz companions with the exact prefix and confirmed linkage.
- Payloads request code protection, unpublished state, empty restricted audience, SIS off and final-grade exclusion; a normal quiz's payload is unchanged.
- A partial unassigned/exclusion step resumes without another quiz or another item set. A partial publish resumes without another student override.
- A copied sidecar with wrong instance/course or a source linked to a different original holds before a write.
- An already published/work-bearing quiz is not forcibly unpublished or stripped of recipients on create-file resume.
- Empty preparation sends no accommodation array; a later named release applies the actual students' saved settings before publication.
- A shared-module audience cannot be mistaken for no audience; independent advance reading placement remains possible.
- A student handout/status report does not contain the code. Test the actual render/output boundary, not a source-text search.

Do not add all possible Canvas fault combinations. Reuse current accepted/uncertain outcome behavior and add one meaningful failure/resume regression at each new consequential stage.

## Completion and evidence

Update formats/examples, CLI/MCP routing and relevant playbook steps with the actual contract. Update both capability records: distinguish local payload/resume coverage, the existing limited assignment-visibility Live evidence and the new end-to-end student-access check still pending. No Canvas Live write or teacher installation occurs under this slice unless separately included in the execution request.

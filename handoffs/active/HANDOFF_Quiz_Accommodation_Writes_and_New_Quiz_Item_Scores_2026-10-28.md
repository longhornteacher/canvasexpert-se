# HANDOFF: Finish New Quiz accommodation writes; post New Quiz item scores

Date: 2026-10-28. To: whoever picks this up in Dev (CE-SE repo, `dev` branch). Run `git fetch` and bring `dev` up to date before starting (root `AGENTS.md`).

This handoff carries the open remainder of two retired handoffs: [Canvas 2026 additions](../retired/HANDOFF_Canvas_2026_Additions_2026-10-28.md) (Parts B and C) and [Retire review page and New Quiz item scores](../retired/HANDOFF_Retire_Review_Page_and_New_Quiz_Item_Scores_2026-10-28.md) (Part 2). Their execution records hold the dated evidence for what landed; this file is the instruction for what is left. Read the [shared execution notes](../README.md#execution-notes-for-the-active-handoffs) for the check commands and live-write rules.

**Status 2026-10-29:** Part A's Q6 is answered (teacher-confirmed 2026-10-29); only Q7, the course/quiz combination, is open, and course sync stays frozen. Part B is un-parked and its Step A save is recorded (§3.1 capture record), so the §3.2 Engine work can start. Do not guess an undocumented protocol and write to Canvas to "see what happens"; do not test on a real student's record or attempt. When the evidence below is unavailable, stop and say exactly which observation is missing.

---

## 0. What already exists in Dev (do not rebuild)

- `set_quiz_accommodations` / `cse quiz-accommodations COURSE QUIZ_ID REQUEST.json [--preview]`: strict presence-aware input, routed course plus exact quiz/assignment/active-student guards, one bulk POST, per-student `successful`/`failed` mapping, sanitized errors, no retry, no read-back, no sidecar (`internal/workflow/quiz_accommodations.go`). **Until omission semantics are known, a write requires all three values** (`extra_time`, `extra_attempts`, `reduce_choices`); incomplete rows hold.
- Saved settings: `quiz_time` and `reduced_choices` in the private students table and the WebUI Students page, separate from deadline Extra time.
- `from_saved` planning (`quiz_saved_accommodations.go`): exact upward percent rounding, untimed quizzes get no time, Standard/false never clears, over-limit holds, counts-only output, audience from `assignment_visibility`.
- `push_quiz` accommodations stage: plans, records count/digest, then **holds publication** (`savedQuizAccommodationHold`); `--skip-accommodations` / `skip_accommodations` records an explicit skip for that run. No accommodation POST is sent by this stage or by `from_saved`.
- Unpublished quizzes: `assignment_visibility` can omit unpublished recipients, so planning holds rather than inventing an audience/count.
- New Quiz written work: comment-only posting; the teacher enters item points in SpeedGrader. `entry_path.resourceId` now decodes as number or string.

Read-only CS8 evidence so far: accommodation GET routes returned 404 at both `/api/quiz/v1` and `/api/v1` (no current-value read exists); New Quiz detail returns a string ID and omits `course_id`. No accommodation POST, attempt or timer check has been made. The teacher has confirmed there is **no teacher-controlled student account** — only the teacher account and real students.

## 1. Unblock: obtain a test identity

Both parts need a Canvas user that belongs to no real student. Ask the teacher which is available; record the answer here with the date:

1. **Built-in Test Student** (accommodations only). It cannot launch New Quizzes, but the accommodation POST may accept its user ID and the Moderate page may list it. That is enough for §2.1 Q1–Q5 and Q8 (setting semantics), not for Q6/timer behavior or any of Part 2. Try this first; it needs only a concrete teacher go-ahead for the disposable quiz and POSTs.
2. **A second account enrolled as a student in CS8** (district admin or a teacher-owned login). Needed for attempt/timer checks and all of Part 2.

**Answer (2026-10-28):** the teacher reports option 2 is available, a second account enrolled as a student in CS8. Its identity and the per-step live go-ahead (disposable quiz, accommodation POSTs, Step A dev-tools save) are still to be given before any Canvas write.

If neither is available, both parts stay here unchanged. The existing holds are the correct product behavior meanwhile.

## 2. Part A: accommodation semantics, then enable the writes

### 2.1 Probe (CS8, disposable timed New Quiz, unpublished unless an attempt is needed)

Record dated answers here before enabling anything that depends on them.

1. Which base path accepts the POST: `/api/quiz/v1/...` or `/api/v1/...`?
2. POST `extra_time: 10`, then POST the same student with only `reduce_choices_enabled: true`. Is `extra_time` kept (merge) or reset (replace)?
3. Do `extra_time: 0` / `reduce_choices_enabled: false` clear the setting?
4. Does an unpublished quiz accept accommodations?
5. Repeat the identical `extra_attempts: 1` POST. Absolute setter, or does it increment?
6. *(Needs account 2.)* On a one-attempt quiz, does `extra_attempts: 1` give a second attempt? Does `extra_time` lengthen the student's timer?
7. With a course-level `extra_time` the teacher set by hand, do course and quiz values add on an existing quiz and on one created afterwards? (Course-level sync stays frozen; this answer only decides whether §4.3/§4.4 must report a double-time conflict.) Include restoration of the account's previous course setting in the authorization.

**Observed 2026-10-28 (CS8, disposable unpublished quiz 3710787, teacher-owned account 105645, per-quiz POST; read back on the teacher's Moderate page):**
- Q1: `/api/quiz/v1/courses/:c/quizzes/:id/accommodations` returns 200 `{message, successful:[{user_id}], failed:[]}`; the same route under `/api/v1` is 404. The response carries no effective values and no GET exists.
- Q4: an unpublished quiz accepts the POST.
- Q2: **merge.** `extra_time:10`, then only `reduce_choices_enabled:true`, left Moderate at Time + 10 min and Answer Choices Reduced. Omitted fields are kept.
- Q3: **clears.** `extra_time:0` then showed Time + 0 min (row remains) with choices still Reduced; `reduce_choices_enabled:false` then removed the reduced-choices line.
- Q5: **absolute setter.** `extra_attempts:1` showed Additional 1 attempt (default 1, total 2); the identical repeat POST still showed Additional 1 (total 2).
- Still unobserved: Q6 (second attempt and timer for a student; needs the account's own login) and Q7 (course-level combination).
- **Q6, 2026-10-29 (teacher-confirmed, CS8 test account 105645):** the student received the extra time on the timer and the extra attempt in their own session. Q7 (course-level combination) remains unobserved; course sync stays frozen. Sent in order: `extra_time:10`, `reduce_choices_enabled:true`, `extra_time:0`, `reduce_choices_enabled:false`, `extra_attempts:1` twice. Quiz 3710787 stays until cleanup (see cs8-test-objects.md).

Check results on the Moderate page (dev read-backs are fine here). List or delete the disposable quiz in [cs8-test-objects.md](../../docs/cs8-test-objects.md).

### 2.2 Enable, according to the answers

- **Explicit tool.** If Q2 shows merge, send only supplied fields and drop the all-three requirement; if replace, keep it (there is no current-value read). Document the observed rule in `engine-docs/formats.md`.
- **Saved application.** If Q3/Q5 show absolute setters, the `push_quiz` stage and `from_saved` send the whole computed array (`extra_time`, `reduce_choices_enabled`, never `extra_attempts`); a partial 200 or uncertain POST holds publication and a rerun resends the whole array. If not absolute, add only the smallest private per-student outcome record in UserInstall `.cse/` and note why here. Replace `savedQuizAccommodationHold` with an accurate report (counts only; failed students by alias with a sanitized reason).
- **Unpublished audience.** If Q4 shows unpublished quizzes accept accommodations, define safe pre-publication audience planning: an unrestricted quiz uses active enrollments; a restricted quiz uses the Engine's own audience stage targets (`quiz_audience.go`), never local membership guesses. If unpublished quizzes do not accept them, sequence the stage as publish-restricted-first only if the teacher approves that exposure; otherwise keep the hold.
- **Q7 add.** If course and quiz values add, hold the affected student's automatic application and explain the conflict; do not subtract from stale state.

Local tests for each enabled behavior (fake server: partial 200, uncertain POST, rerun, privacy of counts-only output). Update both capability records and `quizzes.md`.

### 2.2 execution record (2026-10-28)

Implemented in Dev from the Q1-Q5 observations: the explicit tool now sends only supplied fields (the all-three hold is gone); `push_quiz` and `from_saved` POST the computed `extra_time` / `reduce_choices_enabled` array (never `extra_attempts`) before publication, and `savedQuizAccommodationHold` is replaced by `quizAccommodationHold` plus counts and failed students by alias; an unconfirmed or partial POST holds publication and a rerun resends the whole array, so no per-student outcome record was added (merge plus absolute setters make a resend idempotent). Unpublished audience: unrestricted means every active student, restricted means the quiz's own individual overrides read back from Canvas; a section, group or empty override holds. Q7 (course/quiz combination) is unobserved, so no double-time conflict report exists; course sync stays frozen. Docs, playbook and both capability records updated. Local evidence only: `go test ./...`, `go vet ./...`, the 57-tool compiled MCP smoke and `git diff --check` passed. Not done: a live Engine-built timed quiz push, and Q6/timer checks, which need the student account's own login. Quiz 3710787 still needs deleting (listed in cs8-test-objects.md).

### 2.3 Done when

A teacher can give named students time/attempts/fewer choices in one call with any omitted field handled per the observed rule; a student saved as Time and a half gets correct minutes before publication on every Engine-built timed quiz and on a hand-built quiz via `from_saved`; Canvas Live evidence for one Engine-built timed quiz is recorded (timer evidence needs account 2 and stays labeled unobserved without it).

## 3. Part B: New Quiz item scores through the Engine

No public API writes New Quiz item scores. SpeedGrader's **Update** sends an internal call (read from the public New Quizzes UI bundle, build `2026-10-28.21`):

```
POST {quiz_host}/api/quiz_sessions/{quiz_session_id}/results
Authorization: <grading token>   AuthType: Signature
{ "results": [ <every item result of the attempt, without "id"> ], "fudge_points": <number> }
```

The grading token comes from `GET {backend_url}/api/participant_sessions/{ps}/grade` (`Bearer <native token>`, `X-Domain: <Canvas host>`), which returns `host`, `token`, `quiz_api_quiz_session_id`; it differs from the file-download `/results` token. Each POST creates a new result version. A read-only CS8 check (2026-10-28) returned 200 for the whole read chain; the API returns snake_case while the UI model is camelCase. **No write has been made or observed.** A total-only `posted_grade` and browser automation are out of scope.

### 3.1 Step A: observe one teacher save (needs account 2)

Disposable New Quiz in CS8 (one essay, one file upload, one auto-scored item), restricted to account 2 before publishing. Account 2 submits; **the teacher** sets the essay points, adds an item comment and clicks Update with browser dev tools recording. Repeat once with Grade by question on and once changing only fudge points. Record a redacted note in [new-quiz-written-work-spec.md](../../docs/new-quiz-written-work-spec.md) and synthetic before/request/response fixtures under `internal/canvas/testdata/`: header names (no values), request casing/nulls/types, fields changed besides `score` (`gradeStatus`, `gradedAt`, `graderId`), where the grader comment goes, the result-version identifier, the participant/user/attempt/session mapping, and whether and how fast the gradebook total and hidden-grade policy follow. No HAR, cookies, signed URLs or tokens in Dev.

### 3.1 observation record (2026-10-28, teacher-directed, real class)

At the teacher's explicit direction, one SpeedGrader save was made on a real class attempt (LANGUAGE ARTS 7, ECR New Quiz, attempt 2, two essay items): full credit on both items with item feedback "full credit", then Update. Canvas then showed score 100 (was 80), `workflow_state: graded`, `graded_at` updated, `grade_matches_current_submission: true`, `posted_at` unchanged from the earlier post; no assignment-level comments were added (the text is item feedback inside the result). **Not captured:** the in-frame quiz-host request, headers, body shape, result-version id and timing: the Browser pane records only page-level traffic, and the teacher declined to capture it by hand. Step A therefore stays open; no fixtures or spec note were written and Part B Engine work stays blocked. Two read-only findings on the same course: `sessionless_launch` now returns JSON with a `url` (the page must be fetched from it) where `NewQuizFiles` reads the first response as the page, and the participants route returned 404 in a temporary live read. Neither is confirmed against the Engine path; check both before relying on a New Quiz pull.

### 3.1 capture record (2026-10-29, teacher-captured, test account)

Part B is un-parked (teacher decision, 2026-10-29). CS8 disposable New Quiz assignment 3710957, visible only to account 105645 and omitted from the final grade: q1 choice 2 pts (auto), q2 essay 5 pts, q3 file upload 3 pts. The account submitted attempt 1 (auto score 2). The teacher graded in SpeedGrader in Chrome with dev tools recording and exported a sanitized HAR (kept outside Dev, to be deleted). Two Updates were captured: q2 set to 5 (total 10), then to 4 (total 9).

- **Read chain inside the SpeedGrader frame:** `POST {lti_host}/api/native/launch`, then `GET {lti_host}/api/participant_sessions/{ps}/grade` (headers `Accept`/`Content-Type: application/json`, `X-Domain: <root account uuid>`; the sanitized export strips `Authorization`). It returns `host` (the quiz-api host), `token`, `quiz_api_quiz_session_id` and `participant_session` (`id`, `participant_id`, a quiz-internal `user_id` that is not the Canvas user ID, `status`, `submitted_at`, `multiple_attempts`). Then `GET {host}/api/quiz_sessions/{qs}`, `.../session_items` and `.../results/{result_id}/session_item_results`.
- **Write:** `POST {host}/api/quiz_sessions/{qs}/results` with `AuthType: Signature` (the `Authorization` value was not captured) and a JSON body `{"results": [...], "fudge_points": null}` → **201**. `results` holds **every item** in position order, auto-scored items unchanged. Each entry is the current `session_item_results` row converted to camelCase at every level (`item_id`→`itemId`, `points_possible`→`pointsPossible`, `graded_at`→`gradedAt`, `answer_feedback`→`answerFeedback`, `regrade_info`→`regradeInfo`, `feedback.item_feedback`→`feedback.itemFeedback`, `scored_data`→`scoredData` with `grade_status`→`gradeStatus`, `result_score`→`resultScore`, `user_responded`→`userResponded`), without `grading_method`, plus `"errors": {}` and `"graderId": null`. `gradedAt` carries the previous result's value, not the save time. Only the changed item's `score` differs; numbers go as plain numbers (`4`, where the read returns `4.0`).
- **Response and versions:** each POST returns a new result whose `id` is the version identifier (24991184, then 24991193), with `grading_method: manual_grading`, `score`, `points_possible`, `percentage`, `fudge_points`, `status: graded` and an embedded `quiz_session`. That `quiz_session.metadata` carries the student's full name and user uuid: keep it out of logs, outcomes and fixtures. Reading `session_item_results` under the new result ID returns the saved scores in snake_case with `graded_at` set to the save time.
- **Canvas follows at once:** the submission showed score 9, `graded_at` in the same second as the second POST, `grader_id` -794 (the New Quizzes tool, not the teacher), `workflow_state: graded`, and `posted_at` unchanged under the automatic post policy.
- **Third save (same day, one Update):** q2 back to 5, an item comment and fudge points 0.5 went in one POST. The comment is `feedback.graderFeedback: {"content": "<p>…</p>"}` on that item, beside the untouched `itemFeedback: {}`, and reads back as `feedback.grader_feedback.content`; it is not a Canvas submission comment. `fudge_points: 0.5` is top-level; the result scored 10.5 of 10 (`percentage` 1.05) and the Canvas submission took 10.5 in the same second. `gradedAt` again carried the previous result's time.
- **Recorded:** spec note in [new-quiz-written-work-spec.md](../../docs/new-quiz-written-work-spec.md#item-score-write-protocol-observed-2026-10-29); synthetic before/request/response fixtures in `internal/canvas/testdata/newquiz_item_scores/`. Step A is complete enough to build §3.2.
- **Still unobserved:** Grade by question, the `gradeStatus` of a never-graded item before its first save (attempt 2 is still open to the test account), and a manual post policy (L7, part of the live plan).

### 3.2 Engine (only after Step A)

- **Client** (`internal/canvas/newquiz.go`): split the existing chain into a per-assignment native session, then per-attempt `NewQuizGrading` (participant session → `/grade` → quiz session → authoritative result → item results; the attempt must match or hold) and `PostNewQuizResult`. Tokens go only to their own HTTPS `instructure.com` host; the PAT never leaves Canvas. No automatic retry on this POST.
- **Body:** start from the attempt's current item results, apply only Step A's observed transform, change only target manual-item scores, observed status fields and supplied grader feedback. Preserve untouched items, unknown fields and `fudge_points`. Never send a partial list.
- **Results format:** `operation: item-scores` on a `kind: results` row with `item_scores: {attempt, components, item_feedback?, feedback?}`. Reject auto-scored targets. Item score = `item maximum × earned / applicable maximum`, exact decimal, rounded once to two places, shared with `internal/render/render.go` `PartPoints` so sample, comment and posted points agree. No effort floors, late timing or `EnteredGrade`.
- **Guards:** exact course/assignment/user; participant session belongs to that user and quiz; latest submitted attempt equals the pulled one; hold anonymous/moderated grading or changed maxima; a teacher edit to targeted items since the pull holds that student. Team: `Service.ForCourse`, `(CE)` consent, `hideGrades`/`gradeVisibleHold` (`team_consent.go`).
- **No replay:** save a pending outcome before POST and the accepted result-version identity after. Completed outcomes never POST again. An uncertain write reconciles against the authoritative result or holds. Item-score POST first, then the optional whole-quiz comment through the existing comment path; a failed score posts no comment; a failed comment resumes alone. Never persist grading tokens.
- **Integration:** `domain/types.go`, `scoring.go`, `new_quiz_work.go`/`pull_work.go` (attempt baseline), `results_init.go`, `post.go`, `render.go`, `session_discovery.go`/`targeted_session.go`. Extend every switch that handles `comment-only`. A partial item selection must not hide remaining unscored items from discovery.

### 3.2 execution record (2026-10-29, local tests only)

Built in Dev. Differences from the plan above, kept deliberately small:

- **Results format:** a New Quiz `comment_only` body takes `item_scores: true` instead of a separate `item-scores` operation, so validation, rendering, review and session discovery are shared with the comment path. The Engine owns `item_scores_outcome`. Every interpretation component must name an `item_id`, one part per item. Item points come from `render.ItemPoints`, which the comment's part lines now use too.
- **Client:** `newQuizNative` (sessionless launch → ENV → workflow JWT → native launch) is shared by file reads and grading, and follows a JSON `sessionless_launch` reply that names a one-time same-origin URL. `NewQuizGrading` takes the participant and quiz session IDs from the Canvas submission's `external_tool_url`, so the participants route is not needed; then `/grade`, the quiz session (`authoritative_result` id and `fudge_points`) and `session_item_results`. `PostScores` builds the observed body and returns the result version. Tokens stay unexported and on their own hosts. A transport error or 5xx on the POST is uncertain.
- **Guards:** hold, writing nothing, when the submission has no quiz session link, the result's attempt is not the pulled attempt, `fudge_points` is absent from the read (rather than risk erasing it), or the Canvas score differs from the pulled `observed_score` (someone graded since the pull). The plan's per-item edit check was replaced by that score comparison, because a never-graded item's `gradeStatus` is unobserved. Written items are sent with `gradeStatus: "graded"`; that value is observed only on already-graded items.
- **No replay:** the outcome is saved as pending before the POST. A pending or uncertain write is reconciled on the next run: if the quiz result already shows the intended points it is `preserved`, otherwise `uncertain` and nothing is sent. The comment posts only after the scores land; a held or uncertain score posts no comment.
- **Item comments (teacher direction, same day):** each part's feedback, with its hand-copy exemplar below 90%, is written as that item's `feedback.graderFeedback.content` in the same POST, so each item's rationale sits on the item. The whole-assignment comment then carries only the closing remarks and is `not-requested` when there are none. Reconciliation compares both scores and item comments.
- **Not built:** the native session is read again for each student, not cached.
- **Tests:** canvas fixtures check the exact observed body against `internal/canvas/testdata/newquiz_item_scores/`, token hosts, the JSON launch URL and the holds; workflow tests cover score-then-comment with no rerun write, the since-pull hold, and an uncertain write reconciled without repeating. `score.md` and the teacher-facing capability are unchanged until L1–L8 pass.

### 3.3 Live plan (account 2, disposable quiz, go-ahead per step)

L1 essay score posts and gradebook total matches · L2 essay + file upload with item comments · L3 teacher's fudge points preserved · L4 re-score creates a new version · L5 rerun makes no POST · L6 two attempts: stale unfinished attempt-1 result holds, attempt 2 stays discoverable · L7 manual post policy keeps the grade hidden · L8 teacher's later SpeedGrader edit is not overwritten. L1–L8 are release evidence. Record L9 (question regrade survival), L10 (restore earlier version) and L11 (mid-run token expiry, only with two test accounts) as run or unobserved.

### 3.3 L1 record (2026-10-29, teacher-directed, real class)

At the teacher's direction L1 ran on a real student instead of the test account: PAP LANG ARTS 7 (course 120669), New Quiz assignment 3708517 "The Last Human Light: Plan Your Features ECR" (100 points: eight auto-scored items worth 80, two 10-point essays), one student by alias, attempt 1, Pending Review at 70 with both essays never graded. Dev build 4be18b2 run with `--workspace` against UserInstall, not the classroom MCP connection.

- **Pull:** `pull-work COURSE 3708517 --session` read the 23 needing-grading students read-only in about 11 s. The results file held only the one student. While setting up, the Dev `cse api` read returned the student only by saved alias, the first live evidence for raw-read name hiding.
- **Write:** `post` with `item_scores: true`, both essays scored 5/10 with item comments and no closing remarks. `item_scores_outcome: succeeded`, new result version 24994274; the whole comment was `not-requested`. The POST built from never-graded items (sent with `gradeStatus: graded`) was accepted, and the read included `authoritative_result.fudge_points` (the write requires it).
- **Canvas:** score 80 (70 + 5 + 5), `workflow_state: graded`, `graded_at` 16:46:03Z within the write, `grader_id` -794 (the New Quizzes tool), `posted_at` unchanged under the automatic post policy, no submission comment added.
- **Rerun:** a second `post` sent nothing (`graded_at` unchanged).
- **Pending:** a look at the two item comments in SpeedGrader and in the student's results view. One Dev `cse api` re-read right after returned empty output; the same read through MCP succeeded; not investigated further.

L1 passes for one student. L2–L8 remain.

### 3.4 Done when

Step A is recorded with fixtures; Engine path and local tests are in; L1–L8 pass on account 2. Then update `score.md`, `new-quiz-written-work-spec.md`, `SCORING-RULES.md`, root `AGENTS.md` (the "teacher enters item points in SpeedGrader" sentence) and both capability records, stating plainly that this uses an undocumented internal New Quizzes call; the SpeedGrader-by-hand path stays as the fallback. Until then the comment-only capability stays as it is.

## 4. Not in this handoff

Observations that arrive with ordinary use and need no work: the next scoring pull with a direction image (authenticated download), a real teacher-approved tag re-level and browser student access after tag changes, Enhanced Rubrics browser appearance and account-rubric association. Course-level accommodation sync stays frozen until the teacher asks. A results-file `curve` is not requested.

> **Retired 2026-10-28.** Historical instructions follow; do not execute them again. Part 1 is complete in Dev: the scoring/authoring review page and `start_review` are removed, the check-in happens in the conversation with an Engine-rendered sample (`validate --show` / `validate_document.show`), and the private family review page, Draft PDF and terminal previews remain. The `resourceId` number/string decode fix landed. **Part 2 (New Quiz item scores) was not started:** it needs a teacher-controlled student account, which does not exist yet; it moved to [HANDOFF_Quiz_Accommodation_Writes_and_New_Quiz_Item_Scores_2026-10-28.md](../active/HANDOFF_Quiz_Accommodation_Writes_and_New_Quiz_Item_Scores_2026-10-28.md). Re-verified at retirement: full uncached tests, vet, 57-tool MCP smoke and `git diff --check` passed.

# HANDOFF: Retire the review page; post New Quiz item scores through the Engine

Date: 2026-10-28. To: whoever picks this up in Dev (CE-SE repo, `dev` branch). The teacher works on several PCs, so run `git fetch` and bring `dev` up to date before starting (root `AGENTS.md`).

This document stands alone. Part 2's findings come from a 2026-10-28 research session: official Canvas docs, the public New Quizzes UI bundle, and one read-only Canvas Live check. Sources are in §5.

Execution review: 2026-10-28 against the current Dev working tree. **Part 1 is ready for 6.1 Terra or 6.1 Sol execution. Part 2 is ready for the bounded evidence step, not an invented write protocol.** Read the [shared execution notes](../README.md#execution-notes-for-the-three-active-handoffs). The earlier live observations below are retained as reported evidence; this review made no Canvas calls.

---

## Execution record — 2026-10-28

Part 1 is complete in Dev. Scoring/authoring CLI and MCP review entry points and the optional scoring-page smoke branch are removed. Offline `validate RESULTS --show STUDENT_REF` / `validate_document.show` returns only the selected ref/alias, hold status and exact comment from the shared rendering path; held rows omit the comment and unknown references fail. A focused CLI test compares the text against the posting renderer and checks alias selection, privacy, held/unknown selection and unchanged unsent files; compiled MCP checks exercise the same selections. Conversation reports now include impressions, distribution, conditional hypothetical curve options and one exact feedback sample. Private family review and shared reconnect/status, Draft PDF and terminal previews remain. Full uncached Go tests, vet and the fresh 56-tool compiled MCP smoke passed, including retained family/privacy/rendering tests. No Canvas write was needed. Installed-client adoption is unobserved.

Part 2 remains waiting for the exact teacher-controlled CS8 account and teacher-recorded redacted SpeedGrader save capture in §3.3. The user clarified that no teacher-controlled student account exists: there is the teacher account and the real student accounts. The requested disposable attempt tests cannot be performed in that setup; no real-student attempt was selected or written. No item-score request protocol has been guessed.

The independent native file-read compatibility fix is implemented: resourceId accepts the observed numeric form and the existing string form; the full synthetic token-isolation/attempt-matching chain passes both with access_token. This is local evidence against the earlier reported live payload shape, not a live download observation or item-score write protocol. No grading POST was added. Final combined checks passed: full uncached Go tests, vet and the 57-tool compiled MCP smoke (including the accommodation tool); the earlier Part 1-only count was 56.

## 0. TL;DR

| Part | What | Engine / playbook | Size |
|---|---|---|---|
| 1 | Retire the scoring/draft review page as a requirement and as code. The teacher check-in happens in the conversation: impressions, score ranges with curve options, and one example of the exact feedback, then the Engine posts. | Mostly playbooks and docs; remove `cse review` / `start_review` | Medium |
| 2 | Post New Quiz **per-item scores** (and per-item grader feedback) through the same internal grading call SpeedGrader uses, then test it thoroughly in Canvas Live. This replaces "the teacher types item points in SpeedGrader". | Engine, with playbook lines once live checks pass | Large |

The parts are independent; either can go first. Part 2 changes the New Quiz section of `score.md` that Part 1 also edits, so whoever lands second merges by hand.

**Teacher decision (2026-10-28):** after a few weeks of real use, agent↔teacher interaction stays in the agent app. No review page in the scoring or authoring flow. If the teacher wants a pretty page, the agent writes one on request (§2.4). A browser-automation fallback for item scores is **not** being pursued.

---

## 1. Rules for both parts

- **Engine vs playbook.** The Engine does Canvas access, identity, exact computation and safe writes. What to say to the teacher, which sample to show and which curve to suggest are playbook judgment.
- **Public repo.** Use course nicknames (CS8, ELA7) and "the teacher". No real course IDs, student names or Windows usernames in code, tests, docs or successors of this file. Tests use synthetic IDs (e.g. 900001).
- **Canvas Live.** Every live write needs the teacher's fresh, concrete go-ahead in that session. Live reads use the temporary `cmd/tmpdiag` pattern in [development-notes.md](../../docs/development-notes.md), which never prints tokens, signatures or student content. Delete it afterwards. List every disposable object in [cs8-test-objects.md](../../docs/cs8-test-objects.md).
- **Accepted write = done.** In teacher use, no read-backs after accepted writes; Engine action reports are concise. The agent still gives the classroom report in §2.2. Keep the cheap pre-write guards: exact course, assignment, student and attempt identity; wrong-target refusal; no replay. Part 2's live test plan is dev testing, so reading back there is expected.
- **Docs that change together.** Update `internal/localui/capabilities.go` (status data and review date) and [engine-capabilities.md](../../docs/engine-capabilities.md) together. Keep Canvas Live evidence, local tests and pending human checks separate.
- **Checks.** `go test ./...`, `go vet ./...`, and `scripts/check-mcp.mjs`. Removing `start_review` lowers the asserted tool count on line 59 by one. The Canvas 2026 handoff also changes that count, so recount rather than assume.

---

## 2. Part 1: retire the review page

### 2.1 Where it got lost

On 2026-10-24 the default check moved into the conversation, but the page stayed as an option ("when the teacher asks or has saved that preference"). Several places still present it as the default or as part of the flow:

- `README.md:70`: "use the teacher review page by default". This is stale and contradicts the playbooks.
- The MCP server instructions in `internal/mcpserver/server.go:204`: "use start_review only when the teacher asks…".
- `workspace-template/playbooks/score.md:99`: a whole paragraph on the page, `.review.json` and `review_status`.
- `workspace-template/Teacher_Override.md:5`: the example preference "always show me scores on the review page".
- `docs/new-quiz-written-work-spec.md` ("the review page should show the comment's scores…"), `docs/SCORING-RULES.md:29`, `docs/ARCHITECTURE.md:84`, `engine-docs/formats.md:178` (audience drafts reviewed with `cse review`).
- `internal/localui/capabilities.go:108` and `docs/engine-capabilities.md:89` (the "Review page" capability rows), and the WebUI "How it works" page, `internal/localui/templates/how.html`.
- `group-work.md:65`, `authoring.md`, `quizzes.md`, `edit.md`, `team.md` and `install.md`: check each mention.

Run `rg -n -i 'review page|start_review|cse review|review_status|\.review\.json' README.md docs engine-docs workspace-template cmd internal scripts` for the complete current list. Classify each match: remove scoring/authoring entry points, retain parent/family behavior and Draft PDF support, and leave dated historical evidence accurate. A global deletion of the word `review` is incorrect.

### 2.2 The check-in, in the conversation (playbook: `score.md` "Report in the conversation")

The current report already covers count, range and spread, impressions, held students, timing and exemplars. Change it to this, one block per assignment (and per class when sections differ):

1. **Overall impressions:** what most students did well, the most common gap, anything surprising. Two or three sentences.
2. **Score ranges for a possible curve:** the distribution in assignment points (for example "median 7/10; most 6–9; four below 5"). For a New Quiz, give the range per written part and for the quiz total. Then give one or two curve options with their effect stated plainly (for example "+1 to everyone puts the median at 8 and leaves no one below 5"). Offer a curve only when the distribution suggests one; it is never applied without the teacher asking (see §2.3).
3. **One example of the feedback, exactly as the student will see it.** Pick a typical student who is not held and show the Engine-rendered comment text (and, after Part 2, that student's per-item points) by alias. It must be the Engine's rendering, not the agent's paraphrase. Add `cse validate RESULTS --show STUDENT_REF` and optional `show` on the existing MCP **`validate_document`** tool (there is no MCP tool named `validate`). Validate normally, then return only the selected row's ref/alias, hold status, rendered student comment and computed item points when available. Reuse `BuildReview`/the shared rendering path; never expose `TeacherNote`, private contact data or the entire submission as part of this sample. Unknown refs fail clearly; held rows return the reason without presenting an unsendable comment as final. Keep it read-only and usable without a Canvas connection.
4. Held students by alias with the specific reason, missing work as a count, late timing corrected or held, exemplars ready.
5. Ask whether to post. On the go-ahead, `post_results`. When the request already said "score and post", post first and give the same report with the outcomes.

Keep the existing rules for teacher changes ("go easier on…", "redo Maya's", holding someone back) and the Draft PDF for a teacher who wants to read everything.

Use quiz totals only when the Engine has an exact, complete basis for that attempt. Before Part 2, or when auto-scored evidence is incomplete, report written-part ranges and say the quiz total is unavailable; do not add up a guessed total or present it as a posted grade. Curves remain hypothetical report text. Existing honest-effort floors must not be silently applied to New Quiz item points or `fudge_points`.

### 2.3 Curve: report only until the teacher confirms a shape

`SCORING-RULES.md` requires the honest computed assessment, and there is no curve field today. For now, curve options appear only in the report. Applying a curve requires a separate requested feature; an execution agent must not add it to finish this handoff. If a later request leaves the amount or shape unclear, clarify only that missing decision. A possible future shape (not an implementation requirement here):

- A document-level `curve` in the results file (points or percent, capped at points possible).
- The Engine computes `posted = assessment + curve` exactly.
- The student's comment shows the computed score and the curve on separate lines, so the honest score stays visible.
- For a New Quiz after Part 2, the same curve maps to New Quizzes' own `fudge_points`, which the student also sees separately.

Do not build this speculatively.

### 2.4 A page on request

If the teacher asks for a nicer view, the agent may write a one-off local HTML file in the course's scoring folder in UserInstall. It is not an Engine feature and has no approve or post buttons; posting still happens through the conversation. It uses student aliases unless the teacher asks for names. It must never be published to any cloud page or share service, because it holds student work. Add two sentences on this to `score.md`. Build nothing else for it.

### 2.5 Remove

- **CLI:** the `review` command in `cmd/cse/remote.go` (the `case "review"` branch, covering results, drafts, QuizForge quizzes, assignment edits and assignment dates), its usage line, `cmd/cse/review_dispatch_test.go`, and the `"review": true` entry in the known-commands map in `cmd/cse/main.go:517`.
- **MCP:** remove the `start_review` registration, `ReviewInput`, the scoring-specific `startReview` wrapper, its allowlist entry and instructional text. **Keep `startReviewSession`, the `"review"` session kind, its hub record/reconnect/status machinery and URL prefix:** `start_parent_review` calls the same helper today. Narrow human-facing messages to parent/family review where appropriate; do not break its existing wire/lifecycle contract.
- **Workflow:** keep `LoadReviewState`, `SaveReviewState` and `ReviewDigest`: the surviving parent/family handler in `internal/localui/review.go` uses all three. Keep `BuildEditReview` too while `BuildReview` dispatches to it for Draft PDF. Remove a scoring-page-only helper only after checking its actual callers. For an assignment edit, the agent describes the change in the conversation from the pulled Canvas Live copy, as `edit.md` already says for the check-in.
- **Playbooks and docs:** everything listed in §2.1. Update `README.md` to describe the conversation check-in. Change the Teacher_Override example to a preference that still applies (for example "always show me two example comments before posting").
- **Capabilities:** retire the "Review scores and drafts on a local page" row in both places. Note the date and say that the parent/family review page is unaffected.
- **Smoke and tests:** remove the scoring-page `--review` branch from `scripts/check-mcp.mjs`, its flag/help references and obsolete scoring CLI dispatch tests. Keep shared rendering, CSRF, family privacy, page reconnection/status and Draft PDF tests. Add one focused CLI/MCP sample-rendering test comparing the selected text with the normal post renderer, including a held/unknown selection. Assert `start_review` is absent and `start_parent_review`, `review_status`, `save_draft_pdf`, and `validate_document.show` remain discoverable. Do not delete whole shared test files because their names contain `review`.

### 2.6 Keep (out of scope here)

- **The private parent/family review page** (`start_parent_review`, `cse parent-review`, `review_status` for its outcomes, and the `internal/localui/review.go` handler and template it runs on). It exists for privacy, not workflow: real recipient names and addresses must stay out of the agent's context, so the teacher's Send happens on that page. Retiring it would need a different send-authorization design and is the teacher's call. Its tests must pass unchanged.
- **`ReviewModel` / `BuildReview`**: `save_draft_pdf` renders from it (`cmd/cse/main.go:566`, `internal/localui/draft_pdf.go`). A rename is optional. Doc comments that call it "the review page's rendering" should say "the draft rendering".
- **`assign-to --preview` / `assign-modules --preview`**: these are terminal previews, not the page.

### 2.7 Done when

- No teacher-facing text (playbooks, README, WebUI, MCP descriptions or instructions) offers or requires a scoring, draft, quiz or edit review page.
- `score.md` describes the §2.2 check-in, including the exact-text example, and the example comes from the Engine.
- Parent and family review, Draft PDF and `assign-to --preview` work as before; their tests pass unchanged.
- Tests, vet and the MCP smoke pass with the new tool count. Both capability records are updated.

---

## 3. Part 2: New Quiz item scores through the Engine

### 3.1 What is known (2026-10-28)

**No public API writes New Quiz item scores.** Checked: the New Quizzes API (quiz create, read, update and delete only) and the New Quiz Items API (authoring); the Classic quiz submission API (Classic only); LTI AGS (only the tool itself can post); GraphQL. A total-only grade through the public submissions API (`posted_grade`) is rejected because the quiz record and the gradebook would disagree and New Quizzes may overwrite it.

**What SpeedGrader's "Update" button does.** This was read from the public New Quizzes UI bundle: `remoteEntry.js` on Instructure's CloudFront host, which the native launch page names. In the module that builds the "update quiz session results" action, the request is:

```
POST {quiz_host}/api/quiz_sessions/{quiz_session_id}/results
Authorization: <grading token>
AuthType: Signature
Accept: application/json
Content-Type: application/json

{ "results": [ <every session item result of the attempt, without its "id"> ], "fudge_points": <number> }
```

- The UI sends **all** item results (the UI model minus `id` and `gradingMethod`), not only the changed ones. Each POST creates a new **result version**; `GET /api/quiz_sessions/{qs}/results` lists them.
- The UI's item model fields are `answerFeedback, attempt, feedback, gradedAt, graderId, itemId, regradeInfo, pointsPossible, position, score, scoredData`. The per-item teacher comment is `feedback.graderFeedback.content`. Manual items carry `scoredData.gradeStatus` (`waiting` / `graded` / `correct` / `incorrect`).
- The grading token comes from the New Quizzes backend: `GET {backend_url}/api/participant_sessions/{ps}/grade` with `Authorization: Bearer <native token>` and `X-Domain: <Canvas host>`. It returns `host`, `token`, `quiz_api_quiz_session_id` and `participant_session`. **This token differs from the `/results` token** that the file-download chain uses.
- After a successful POST the UI posts `refreshGrades` to SpeedGrader. That suggests New Quizzes itself sends the new total to the Canvas gradebook, but it is not proven.
- New Quizzes also has its own grading route, `/moderation/{assignment}/grading/{participant_session}`. It is only relevant to a browser approach, which is not being pursued.

**Read-only Canvas Live check, an existing CS8 quiz, teacher's PAT, 2026-10-28.** The exact object belongs in the private dev reference, not this public handoff. These steps returned 200: sessionless launch, launch page, `jwts`, native launch, participants, `/participant_sessions/{ps}/results`, `/participant_sessions/{ps}/grade`, quiz session, item results, and the results list (one version) using the grading token. Notes for the Engine:

- The native launch returns `access_token`, not `token`.
- `entry_path.resourceId` is a JSON **number**. The current `internal/canvas/newquiz.go` struct reads it as a string; check whether the file-download chain still works live. Fix it here if not.
- The API returns snake_case (`item_id`, `scored_data`, `points_possible`, `graded_at`, `answer_feedback`, `regrade_info`, `feedback.item_feedback`).
- The quiz session's `authoritative_result` carries `fudge_points`, `score`, `points_possible` and `requires_additional_review`.

**Unknown until observed:**

1. Wire casing of `results[]`. The UI model is camelCase, the API returns snake_case, and no key conversion was found in the request path.
2. What the UI changes on a graded item besides `score` (`scoredData.gradeStatus`, `gradedAt`, `graderId`).
3. Whether the Canvas gradebook total updates automatically, and how fast.
4. How the grader comment is saved: in the same POST or separately.
5. The behavior with multiple attempts and with a manual post policy.

**Evidence level:** bundle reading plus Canvas Live reads, one attempt. **No write has been made or observed.**

### 3.2 Teacher input needed before any live write

- **A teacher-controlled student account in CS8.** New Quizzes cannot be taken as the built-in Test Student (recorded in [team live evidence](../../docs/team/README.md)), so live tests need a student attempt that belongs to no real student, such as the teacher's own second account enrolled as a student. Before any write to an existing attempt, the teacher confirms whose attempt it is. Never write to a real student's attempt in testing.
- A fresh go-ahead in the session for each live write step below.

### 3.3 Step A: observe one real save (before writing Engine code)

Create a disposable New Quiz in CS8, initially unpublished, with one essay, one file upload and one auto-scored item. Restrict its associated assignment to the confirmed teacher-controlled account **before publishing it** for that account to submit; an unpublished quiz cannot supply the required student attempt. Include creation, restriction, publication, saves and cleanup in the concrete live authorization. Then **the teacher** opens SpeedGrader, sets the essay's points, adds an item comment and clicks **Update**, with browser developer tools recording network traffic.

Record in a redacted note:
- method, path and header **names** of every request to the quiz host and backend (no token values)
- the request and corresponding pre-save item-result JSON with synthetic identities and answer/feedback content; preserve field casing, nulls, types and list structure (no HAR, cookies, signed URLs or token values in Dev)
- the response keys and new `score`

Then check the Canvas gradebook total and how soon it changed. Repeat once with **Grade by question** on and once changing only fudge points. Record whether these observations settle unknowns 1–4; do not assume they will. If the teacher cannot capture a save, report the missing wire evidence and continue Part 1 or other independent work. Do not guess request casing/status fields and try a grade POST as a substitute for observing the protocol.

Store a short redacted protocol note with the dated evidence in `docs/new-quiz-written-work-spec.md` and synthetic before/request/response fixtures under `internal/canvas/testdata/`. The note must establish the exact participant/user/attempt/session mapping, grade-comment field and save behavior, result-version identifier, and gradebook/hidden-grade propagation. If a particular behavior remains unknown, mark that operation pending and keep the existing comment-only/SpeedGrader path available. Raw student payloads and credentials remain outside Dev.

### 3.4 Engine design

**Canvas client (`internal/canvas/newquiz.go`).** Split the existing chain into a reusable per-assignment native session (launch, `jwts`, native launch, participants), then add per-attempt calls:

- `NewQuizGrading(ctx, …, userID, attempt)`: participant session → `/grade` → host, token and quiz session id → `GET` quiz session → authoritative result → `GET` its `session_item_results`. Require the item results' `attempt` to equal the target attempt, or hold.
- `PostNewQuizResult(ctx, grading, items, fudgePoints)`: `POST {host}/api/quiz_sessions/{qs}/results` with the observed body.

Keep the existing host rules. `backend_url` comes from the Canvas launch page and `host` from the backend's own response; both must be HTTPS and under `instructure.com`. Tokens go only to their own host. The PAT never leaves Canvas.

**Building the body.** Start from the attempt's current item results and apply only the read-to-write transformation established by Step A (including casing and removal of server-only keys). Preserve every untouched item's values and unknown nested fields; JSON whitespace/key order need not be byte-identical. Change only target manual-item scores, the observed graded/status fields, and supplied grader feedback. Preserve existing feedback when none is supplied and preserve `fudge_points` unconditionally in this handoff. Never send a partial list, because a missing item could zero out work the student did. Disable automatic retries for this result-version-creating POST.

**Results format.** Add `operation: item-scores` with an `item_scores` body on a normal `kind: results` student row. Required: exact `attempt` and `components`; optional: `item_feedback` keyed by canonical packet item ID, and whole-quiz `feedback`/`header`. Reuse document `interpretation`, variants and rubric/local `scoring_basis`. Engine-owned state has separate item-result and whole-comment outcomes. `post_results` remains the write surface; no general internal-API tool is added. The agent never types item points. Reject auto-scored targets in this slice; overriding their grades is a separate capability, even when requested.

**Item mapping and arithmetic.** `partPoints` currently resolves component keys to item maximums; it does not compute item scores or prove a unique session-result match. Normalize packet `item_id`/`entry_id` aliases to one canonical item, then to exactly one result in the selected attempt using Step A's observed mapping. Require one scored component per targeted written item in this first slice; missing, duplicate or ambiguous mappings hold the student. For each component, compute `item maximum × earned criterion total / applicable criterion maximum total`, using the existing repeated/direct interpretation rules and exact decimal arithmetic. Round once to the existing two-decimal posting precision; reject a zero numeric denominator, unknown evidence and out-of-range values. Component weights across other items do not rescale this item's score. Share this computation with the feedback renderer (`internal/render/render.go`), whose existing `PartPoints` path already displays this ratio, so sample, comment and posted points agree. Do not route through aggregate `EnteredGrade`, effort floors, late timing or rubric-assessment posting.

For a selected-item request, the supplied component keys select the target items through `interpretation`; require the complete applicable criterion scores for each selected component. Unselected components are untouched, never converted to zero/missing. Keep the ordinary whole-assignment requirement in `ComputeAssessment` unchanged; use a focused per-item validator/calculator for this operation. `item_feedback` may name only selected canonical items; omission preserves live grader feedback, an explicit empty string clears only that selected item's grader comment. If no whole-quiz `feedback` is supplied, record its outcome as not requested and post no extra assignment comment. Example student row (the enclosing results file supplies its packet/basis and an interpretation mapping `response` to the synthetic item `900003`):

```yaml
- student_ref: s-example
  operation: item-scores
  item_scores:
    attempt: {attempt: 1, submitted_at: "2026-10-28T09:00:00-05:00"}
    components:
      - key: response
        evidence: present
        criteria:
          - {criterion_id: evidence, points: 3}
    item_feedback:
      "900003": "Your example supports the claim."
    feedback: "Use the same detail in your next response."
```

**Pre-write guards (cheap, required):** exact course, assignment and Canvas user; verify the participant/session belongs to that user and quiz, not merely that it has the same attempt number. The latest submitted attempt must still be the pulled attempt; hold anonymous/moderated grading, changed grading basis or changed target item maxima. Capture a baseline of the targeted result scores/feedback when pulling scoring evidence; a later teacher edit to those fields holds that student's stale result, preserving unsent work. Use the latest complete item list for untouched items. This is a pre-write conflict check, not an atomic guarantee.

Read [team/README.md](../../docs/team/README.md) before integration. Reuse `Service.ForCourse`, `(CE)` consent, and `hideGrades`/`gradeVisibleHold` (in `team_consent.go`, called from `post.go`); an owned course requires the tag even when the operator's own key is routed. Current behavior sets manual posting before the first scoring write and holds already-visible submissions. A manual policy by itself is insufficient. Preserve excused/hidden/evidence checks from the ordinary comment-only path. Verify through L7 that the internal result API also respects that policy; do not claim teammate support from ordinary assignment tests alone.

**No replay and recovery:** save a pending item-result outcome before POST and accepted result-version/session identity immediately afterwards. Completed outcomes skip POST even if someone later changes the grade in Canvas; an old results file must never undo that teacher edit. For an unfinished outcome, first enforce latest-attempt/identity guards, then compare the target values. If they already equal the computed score and supplied comment, record "already entered" without POST. For a timeout/interrupted pending write, reconcile against the current authoritative result; if it cannot be matched, hold that student for inspection instead of blindly creating another version. Save only the private minimum needed to resume in UserInstall; never persist grading tokens.

**Post order per student:** item-score POST first, then the optional whole-quiz comment through the existing attempt-scoped comment path. If the POST fails or is uncertain, post no whole-quiz comment. If scores succeed but the comment fails, report both outcomes and resume only the comment. Continue independent students after a per-student API failure; stop on a shared wrong-target or inability to persist recovery state. An accepted write remains accepted even if its total is absent from the response: report the total as unavailable without a confirmation GET.

**Output:** one line per student by alias with the new quiz total that New Quizzes returned (the Engine does not compute the quiz total itself).

**Integration map:** `internal/domain/types.go` / `scoring.go` (new operation, `ValidateResults`, arithmetic and outcome validation), `internal/canvas/newquiz.go` (session and observed wire calls), `new_quiz_work.go` / `pull_work.go` (attempt baseline), `results_init.go` (scaffold), `post.go` (guards, outcomes, resume), `review.go` / `internal/render/render.go` (sample and exact points), and `session_discovery.go` / `targeted_session.go` (completion tracking). Extend every operation switch/hold/report helper that handles `comment-only`. Old results without the new baseline stay valid for comment-only; request a fresh pull for item scores. Keep the existing rule for older scored comments ("comment posted, item points not entered"), recognize successful item-score outcomes, and leave feedback-only comments unable to mark written scoring complete. A partial item selection must not hide remaining unscored items from discovery.

### 3.5 Local tests (fake servers; proportionate)

- The full chain against fakes, asserting each host gets only its own token (the PAT never reaches the backend or quiz host; the grading token never reaches Canvas).
- The posted body follows Step A's exact wire transform, with untouched item values/unknown fields and `fudge_points` preserved. Use its synthetic fixture.
- An attempt mismatch, an unknown item id, points over `points_possible`, and anonymous grading each hold with a specific reason and make no POST.
- Computation/unique item mapping, stale targeted teacher edits, and the no-replay skip (including a completed result whose live grade later changed).
- A failed POST means no comment is posted; an accepted score plus failed comment resumes only the comment; an uncertain POST is reconciled or held; independent students continue.
- Team `(CE)` consent, manual policy and already-visible holds apply to this new operation; partial versus complete scoring outcomes remain truthful in discovery.
- `entry_path.resourceId` decodes as a number and as a string.

### 3.6 Canvas Live test plan (CS8, disposable quiz, teacher-controlled account only)

Each step needs the teacher's go-ahead. Read back freely here, since this is dev testing. Record each result with its date in `docs/new-quiz-written-work-spec.md`.

| # | Check | Pass when |
|---|---|---|
| L1 | Essay item score through `post_results` on attempt 1 | The SpeedGrader item shows the points; the quiz total and Canvas gradebook match New Quizzes' returned total; auto-scored items unchanged |
| L2 | Essay + file-upload items in one post, each with a grader comment | Both items graded; comments show on each item in SpeedGrader and in the student's results view (when result settings show them) |
| L3 | Existing fudge points (teacher sets +1 first) | Preserved after the Engine post |
| L4 | Re-score an item the Engine already graded | New result version; earlier version still listed; total updated |
| L5 | Rerun the same results file | No POST ("already entered") |
| L6 | Quiz with 2 attempts: score attempt 1, then the student submits attempt 2; try an unfinished stale attempt-1 result and rerun the completed file | The unfinished stale operation holds; the completed file makes no write; attempt 2 remains discoverable; record `score_to_keep` behavior |
| L7 | Course with a manual post policy | The grade stays hidden until posted; the student sees nothing early |
| L8 | Teacher changes an item in SpeedGrader after the Engine post; Engine pulls again | The pull shows the teacher's value; nothing is overwritten without a new results file |
| L9 | Teacher regrades the question (Build > Regrade) after the Engine post | The record states whether Engine-entered manual scores survive |
| L10 | Recovery drill: restore the previous result version by posting its item list | The restored scores and total match the earlier version |
| L11 | A token that expires partway through a two-student run, only if two teacher-controlled accounts are available | The failed student is reported; the other is complete; rerunning finishes only unfinished work. Otherwise record this as covered by local fault injection, live unobserved; never recruit a real student's attempt |

L1–L8 are the release evidence for the new internal grade-write path. L9–L11 are supplementary observations: record their actual results or why they were not run; they do not require building a recovery UI or delaying independently completed Part 1. L10 is an explicitly authorized disposable-data probe only, not a promise of a general undo feature. Clean up the disposable quiz afterwards, or list it in `cs8-test-objects.md`.

### 3.7 Playbooks and docs once L1–L8 pass

- `score.md` New Quiz section: written parts are scored per item by the Engine; the comment-only path remains for comments without scores.
- `new-quiz-written-work-spec.md`: replace "the teacher enters item points in SpeedGrader" and "per-item scores … out of scope" with the new path and its evidence level.
- `SCORING-RULES.md` and root `AGENTS.md` (the "teacher enters item points in SpeedGrader" sentence): update together.
- Both capability records. Say plainly that this uses an undocumented internal New Quizzes call that Instructure can change without notice. If it fails, the Engine reports it, and the teacher's fallback is entering the points in SpeedGrader by hand.

### 3.8 Out of scope

Browser or SpeedGrader UI automation; the total-only public `posted_grade` path; regrading questions for the whole class; New Quiz moderation (extra attempts and time; see the Canvas 2026 handoff, Part B).

### 3.9 Done when

Step A's capture is recorded with synthetic fixtures. The Engine path and local tests are in, and L1–L8 pass on a teacher-controlled account. L9–L11 carry dated outcomes or explicit unobserved labels. The playbooks and docs in §3.7 are updated, with evidence levels kept separate. If the account/capture is unavailable, report Part 1 as complete and Part 2 as waiting on that specific evidence; leave this handoff active. Do not relabel the existing comment-only capability as unavailable.

---

## 4. Order

1. Part 1 (§2), any time. It needs no Canvas access.
2. Part 2 Step A (§3.3) as soon as the teacher has a teacher-controlled student account in CS8.
3. Part 2 Engine and local tests (§3.4–3.5), built against Step A's capture.
4. Part 2 live plan (§3.6), then the docs (§3.7).

## 5. Sources

- New Quizzes API: https://developerdocs.instructure.com/services/canvas/resources/new_quizzes
- New Quiz Items API: https://developerdocs.instructure.com/services/canvas/resources/new_quiz_items
- Classic Quiz Submissions API: https://canvas.instructure.com/doc/api/quiz_submissions.html
- Grading a New Quiz in SpeedGrader (updated 2026-09-11): https://community.instructure.com/en/kb/articles/661073-how-do-i-grade-a-quiz-in-new-quizzes
- canvas-lms native launch data (`participant_session_id` / `quiz_session_id` for grading launches): `lib/new_quizzes/launch_data_builder.rb`; UI bundle host: `lib/services/new_quizzes.rb` (instructure/canvas-lms on GitHub)
- Public New Quizzes UI bundle (`remoteEntry.js` named by the native launch page; build `2026-10-28.21`): the results POST and the `/participant_sessions/{id}/grade` call
- Earlier evidence: [new-quiz-written-work-spec.md](../../docs/new-quiz-written-work-spec.md) (read and file-download chain, 2026-10-11) and [the retired scoring probe](../retired/NEW-QUIZ-SCORING-PROBE.md)

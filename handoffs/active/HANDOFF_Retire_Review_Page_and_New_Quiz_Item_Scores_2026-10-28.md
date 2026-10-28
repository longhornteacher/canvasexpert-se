# HANDOFF: Retire the review page; post New Quiz item scores through the Engine

Date: 2026-10-28. To: whoever picks this up in Dev (CE-SE repo, `dev` branch). The teacher works on several PCs, so run `git fetch` and bring `dev` up to date before starting (root `AGENTS.md`).

This document stands alone. Part 2's findings come from a 2026-10-28 research session: official Canvas docs, the public New Quizzes UI bundle, and one read-only Canvas Live check. Sources are in §5.

---

## 0. TL;DR

| Part | What | Engine / playbook | Size |
|---|---|---|---|
| 1 | Retire the scoring/draft review page as a requirement and as code. The teacher check-in happens in the conversation: impressions, score ranges with curve options, and one example of the exact feedback, then the Engine posts. | Mostly playbooks and docs; remove `cse review` / `start_review` | Medium |
| 2 | Post New Quiz **per-item scores** (and per-item grader feedback) through the same internal grading call SpeedGrader uses, then test it thoroughly in Canvas Live. This replaces "the teacher types item points in SpeedGrader". | Engine, with playbook lines once live checks pass | Large |

The parts are independent; either can go first. Part 2 changes the New Quiz section of `score.md` that Part 1 also edits, so whoever lands second merges by hand.

**Teacher decision (2026-10-28):** after a few weeks of real use, agent↔teacher interaction stays in the agent app. No review page in the scoring or authoring flow. If the teacher wants a pretty page, the agent writes one on request (§1.4). A browser-automation fallback for item scores is **not** being pursued.

---

## 1. Rules for both parts

- **Engine vs playbook.** The Engine does Canvas access, identity, exact computation and safe writes. What to say to the teacher, which sample to show and which curve to suggest are playbook judgment.
- **Public repo.** Use course nicknames (CS8, ELA7) and "the teacher". No real course IDs, student names or Windows usernames in code, tests, docs or successors of this file. Tests use synthetic IDs (e.g. 900001).
- **Canvas Live.** Every live write needs the teacher's fresh, concrete go-ahead in that session. Live reads use the temporary `cmd/tmpdiag` pattern in [development-notes.md](../../docs/development-notes.md), which never prints tokens, signatures or student content. Delete it afterwards. List every disposable object in [cs8-test-objects.md](../../docs/cs8-test-objects.md).
- **Accepted write = done.** In teacher use, no read-backs after accepted writes and a one-line report. Keep the cheap pre-write guards: exact course, assignment, student and attempt identity; wrong-target refusal; no replay. Part 2's live test plan is dev testing, so reading back there is expected.
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

Run `grep -rIn -i "review page\|start_review\|cse review\|review_status\|\.review\.json"` over Dev, excluding `.cache`, `.test-workspaces` and `handoffs/retired`, for the complete list.

### 2.2 The check-in, in the conversation (playbook: `score.md` "Report in the conversation")

The current report already covers count, range and spread, impressions, held students, timing and exemplars. Change it to this, one block per assignment (and per class when sections differ):

1. **Overall impressions:** what most students did well, the most common gap, anything surprising. Two or three sentences.
2. **Score ranges for a possible curve:** the distribution in assignment points (for example "median 7/10; most 6–9; four below 5"). For a New Quiz, give the range per written part and for the quiz total. Then give one or two curve options with their effect stated plainly (for example "+1 to everyone puts the median at 8 and leaves no one below 5"). Offer a curve only when the distribution suggests one; it is never applied without the teacher asking (see §2.3).
3. **One example of the feedback, exactly as the student will see it.** Pick a typical student who is not held and show the Engine-rendered comment text (and, after Part 2, that student's per-item points) by alias. It must be the Engine's rendering, not the agent's paraphrase. If no current command returns one student's rendered comment, add the smallest option that does, for example `cse validate RESULTS --show STUDENT_REF` and a `show` field on MCP `validate`. Keep it read-only.
4. Held students by alias with the specific reason, missing work as a count, late timing corrected or held, exemplars ready.
5. Ask whether to post. On the go-ahead, `post_results`. When the request already said "score and post", post first and give the same report with the outcomes.

Keep the existing rules for teacher changes ("go easier on…", "redo Maya's", holding someone back) and the Draft PDF for a teacher who wants to read everything.

### 2.3 Curve: report only until the teacher confirms a shape

`SCORING-RULES.md` requires the honest computed assessment, and there is no curve field today. For now, curve options appear only in the report. If the teacher asks to apply one, stop and confirm the shape before building anything. The recommended shape:

- A document-level `curve` in the results file (points or percent, capped at points possible).
- The Engine computes `posted = assessment + curve` exactly.
- The student's comment shows the computed score and the curve on separate lines, so the honest score stays visible.
- For a New Quiz after Part 2, the same curve maps to New Quizzes' own `fudge_points`, which the student also sees separately.

Do not build this speculatively.

### 2.4 A page on request

If the teacher asks for a nicer view, the agent may write a one-off local HTML file in the course's scoring folder in UserInstall. It is not an Engine feature and has no approve or post buttons; posting still happens through the conversation. It uses student aliases unless the teacher asks for names. It must never be published to any cloud page or share service, because it holds student work. Add two sentences on this to `score.md`. Build nothing else for it.

### 2.5 Remove

- **CLI:** the `review` command in `cmd/cse/remote.go` (the `case "review"` branch, covering results, drafts, QuizForge quizzes, assignment edits and assignment dates), its usage line, `cmd/cse/review_dispatch_test.go`, and the `"review": true` entry in the known-commands map in `cmd/cse/main.go:517`.
- **MCP:** the `start_review` tool and `ReviewInput`, the `"review"` session kind's start/reconnect code in `internal/mcpserver/sessions.go` and the hub page record for it, the "Open the local review page" prefix, and the `start_review` clause in the server instructions. Rewrite the instructions to: report in the conversation, then write on the teacher's go-ahead.
- **Workflow:** `.review.json` review state (`LoadReviewState`, `SaveReviewState`, `ReviewDigest`) if only the scoring page uses it, and `BuildEditReview` (`internal/workflow/review_edit.go`) if nothing else uses it. For an assignment edit, the agent describes the change in the conversation from the pulled Canvas Live copy, as `edit.md` already says for the check-in.
- **Playbooks and docs:** everything listed in §2.1. Update `README.md` to describe the conversation check-in. Change the Teacher_Override example to a preference that still applies (for example "always show me two example comments before posting").
- **Capabilities:** retire the "Review scores and drafts on a local page" row in both places. Note the date and say that the parent/family review page is unaffected.

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

**Read-only Canvas Live check, CS8 quiz 3698192, teacher's PAT, 2026-10-28.** These steps returned 200: sessionless launch, launch page, `jwts`, native launch, participants, `/participant_sessions/{ps}/results`, `/participant_sessions/{ps}/grade`, quiz session, item results, and the results list (one version) using the grading token. Notes for the Engine:

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

- **A teacher-controlled student account in CS8.** New Quizzes cannot be taken as the built-in Test Student (it is an LTI tool), so live tests need a student attempt that belongs to no real student, such as the teacher's own second account enrolled as a student. Before any write to an existing attempt (including 3698192's), the teacher confirms whose attempt it is. Never write to a real student's attempt in testing.
- A fresh go-ahead in the session for each live write step below.

### 3.3 Step A: observe one real save (before writing Engine code)

On a disposable New Quiz in CS8 (unpublished, visible only to the teacher-controlled account; one essay, one file upload, one auto-scored item), the account submits an attempt. Then **the teacher** opens SpeedGrader, sets the essay's points, adds an item comment and clicks **Update**, with browser developer tools recording network traffic.

Record in a redacted note:
- method, path and header **names** of every request to the quiz host and backend (no token values)
- the full JSON body with the student's answer text replaced by `<redacted>`
- the response keys and new `score`

Then check the Canvas gradebook total and how soon it changed. Repeat once with **Grade by question** on and once changing only fudge points. This settles unknowns 1–4 without the agent making a grade write. If the teacher can't run developer tools, a fallback is one agent-made POST built from the read item results, with the teacher's go-ahead, on the teacher-controlled account only.

### 3.4 Engine design

**Canvas client (`internal/canvas/newquiz.go`).** Split the existing chain into a reusable per-assignment native session (launch, `jwts`, native launch, participants), then add per-attempt calls:

- `NewQuizGrading(ctx, …, userID, attempt)`: participant session → `/grade` → host, token and quiz session id → `GET` quiz session → authoritative result → `GET` its `session_item_results`. Require the item results' `attempt` to equal the target attempt, or hold.
- `PostNewQuizResult(ctx, grading, items, fudgePoints)`: `POST {host}/api/quiz_sessions/{qs}/results` with the observed body.

Keep the existing host rules. `backend_url` comes from the Canvas launch page and `host` from the backend's own response; both must be HTTPS and under `instructure.com`. Tokens go only to their own host. The PAT never leaves Canvas.

**Building the body.** Start from the attempt's current item results exactly as returned. Change only the target manual items: `score`, any grade-status or graded fields Step A shows, and the grader comment when one is supplied. Keep every other item byte-for-byte and keep `fudge_points` as read (unless an approved curve sets it, §2.3). Never send a partial list, because a missing item could zero out work the student did.

**Results format.** Add a New Quiz item-score operation (name it in `engine-docs/formats.md`; for example `operation: item-scores`). The agent supplies criterion components per written part as it does today. The Engine maps them to items with the existing written-part mapping (`partPoints` in `internal/workflow/review.go`) and computes each item's points exactly. Points are bounded by the item's `points_possible` and use exact decimals. The agent never types item points. Optional per-item feedback and the existing attempt-scoped whole-quiz comment remain available. Validation rejects item ids not in the packet and auto-scored items unless the teacher explicitly asked to override one.

**Pre-write guards (cheap, required):** exact course, assignment and Canvas user; the student's latest submitted attempt is still the pulled attempt (otherwise hold, since a new attempt arrived); no anonymous or moderated grading (hold with a reason); the course routes to the right Canvas key under team teaching; for a teammate's course, the assignment's post policy is manual, or the post holds (same rule as hidden grades today).

**No replay:** if every target item already has the computed score and comment, skip the POST and report "already entered".

**Post order per student:** item-score POST first, then the whole-quiz comment through the existing comment path. If the POST fails, post no comment and report that student as failed. Rerunning resumes.

**Output:** one line per student by alias with the new quiz total that New Quizzes returned (the Engine does not compute the quiz total itself).

### 3.5 Local tests (fake servers; proportionate)

- The full chain against fakes, asserting each host gets only its own token (the PAT never reaches the backend or quiz host; the grading token never reaches Canvas).
- The posted body is exactly the read item list with only the target fields changed; `fudge_points` preserved. Use a golden file from Step A's redacted capture.
- An attempt mismatch, an unknown item id, points over `points_possible`, and anonymous grading each hold with a specific reason and make no POST.
- The no-replay skip.
- A failed POST means no comment is posted.
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
| L6 | Quiz with 2 attempts: score attempt 1, then the student submits attempt 2, then rerun | The rerun holds (new attempt); `score_to_keep` behaves as Canvas documents |
| L7 | Course with a manual post policy | The grade stays hidden until posted; the student sees nothing early |
| L8 | Teacher changes an item in SpeedGrader after the Engine post; Engine pulls again | The pull shows the teacher's value; nothing is overwritten without a new results file |
| L9 | Teacher regrades the question (Build > Regrade) after the Engine post | The record states whether Engine-entered manual scores survive |
| L10 | Recovery drill: restore the previous result version by posting its item list | The restored scores and total match the earlier version |
| L11 | A token that expires partway through a two-student run | The failed student is reported; the other is complete; rerunning finishes it; no half-written result |

L9 and L10 tell the teacher how to undo a mistake; record them even if the answer is "regrade resets manual scores". Clean up the disposable quiz afterwards, or list it in `cs8-test-objects.md`.

### 3.7 Playbooks and docs once L1–L8 pass

- `score.md` New Quiz section: written parts are scored per item by the Engine; the comment-only path remains for comments without scores.
- `new-quiz-written-work-spec.md`: replace "the teacher enters item points in SpeedGrader" and "per-item scores … out of scope" with the new path and its evidence level.
- `SCORING-RULES.md` and root `AGENTS.md` (the "teacher enters item points in SpeedGrader" sentence): update together.
- Both capability records. Say plainly that this uses an undocumented internal New Quizzes call that Instructure can change without notice. If it fails, the Engine reports it, and the teacher's fallback is entering the points in SpeedGrader by hand.

### 3.8 Out of scope

Browser or SpeedGrader UI automation; the total-only public `posted_grade` path; regrading questions for the whole class; New Quiz moderation (extra attempts and time; see the Canvas 2026 handoff, Part B).

### 3.9 Done when

Step A's capture is recorded (redacted). The Engine path and local tests are in. L1–L11 are recorded with dates on a teacher-controlled account. The playbooks and docs in §3.7 are updated, with evidence levels kept separate.

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

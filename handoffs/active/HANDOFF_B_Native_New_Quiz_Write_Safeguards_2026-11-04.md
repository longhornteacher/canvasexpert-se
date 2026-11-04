# HANDOFF B: Native New Quiz writes honor the write contract

Date: 2026-11-04. Slice B of the [unified architecture review](../Unified%20Architecture%20Review.md). Layer: **Engine** (`internal/canvas`, `internal/workflow`). No playbook change expected.

Before starting: `git fetch`, bring `dev` up to date, read root `AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/SCORING-RULES.md`, [docs/team/README.md](../../docs/team/README.md) and the [shared execution notes](../README.md#execution-notes-for-the-active-handoffs). Facts below were mapped at `2818e36`; line numbers are hints, so find the symbols with `rg`.

## Why

Engine-written New Quiz item scores go through a separate "native" HTTP path. It keeps request counting but skips three things every ordinary Canvas write gets:

1. **Teammate audit.** [docs/team/README.md](../../docs/team/README.md) row 14 promises: "Every Canvas write made with a teammate's key is logged to `.cse/team-log.jsonl`". Item-score writes on a teammate's course are not logged.
2. **Uncertain-outcome classification.** Some ambiguous responses are recorded as plain `failed` instead of `uncertain`, which tells the agent it is safe to try again when the write may have landed.
3. **Teammate error naming and throttling.** A rejected teammate key is not named; 429s do not slow the client.

No duplicate grading was demonstrated: the result-version replay guard (below) already blocks blind re-sends. This slice closes the contract gaps without replacing the native client.

## Current code

**Native client.** `(*Client).newQuizNative` (`internal/canvas/newquiz.go` ~198–296) builds `&http.Client{Transport: c.http.Transport, Jar, Timeout: 45s}`, with `CheckRedirect` allowing only https redirects to the Canvas origin. It reuses only the transport (`countingTransport`, `client.go:93`). The token chain is: sessionless launch with the PAT, then `POST {canvas}/api/v1/jwts` (PAT), then `POST {backend}/api/native/launch`, then a per-attempt grading token from `GET {backend}/participant_sessions/{ps}/grade`.

**`secureJSON`** (`newquiz.go:307–353`) is used by all 12 native calls:
- `hc.Do` error (including a rejected redirect) → `&Error{Kind:"transport"}`.
- non-2xx → `&Error{Kind:"validation", Status}`; never `Uncertain`.
- body read failure, oversize body or JSON decode failure → `&Error{Kind:"validation"}` with Status 0.
- no `acquire`/`release`, no throttle, no `OnWrite`, no `MapError`, no `statusError`.

**`NewQuizGrading.PostScores`** (`newquiz_grading.go:163–190`) is the only native call that changes grading data: `POST {quizhost}/api/quiz_sessions/{qs}/results` with `AuthType: Signature`. It sets `Uncertain` only for `Kind=="transport"` or `Status>=500`, and for a 2xx with no `id`.

| Native outcome today | Recorded as | Ordinary write would be |
|---|---|---|
| transport error / rejected redirect | uncertain | uncertain |
| 5xx | uncertain | uncertain |
| **408** | **failed** | uncertain |
| **3xx returned as a response** (300, 304, no Location) | **failed** | uncertain |
| **2xx, body unreadable or oversize** | **failed** | uncertain |
| **2xx, JSON decode fails** | **failed** | uncertain |
| 2xx, no `id` | uncertain | — |
| 401/403/429/other 4xx | failed | failed (401 → `authentication`, 429 → throttled) |

**Ordinary write rules for comparison** (`internal/canvas/client.go`):
- `statusError` (~217–241): `uncertain := write && (status == 408 || status >= 500 || 3xx)`.
- `send` (~285–331): transport error on a write → uncertain; 2xx whose body cannot be consumed → uncertain.
- `wrote()` (~243–247) calls `OnWrite(method, path, status, uncertain)` for every non-GET. `request()` (~249–255) applies `MapError`.
- `throttle` (~178) is triggered on 429 / rate-limit 403. Writes are never retried.

**Workflow caller.** `postItemScores` (`internal/workflow/new_quiz_item_scores.go:25–112`), called from `postCommentOnly` (`post.go:290`):
1. holds without a participant session;
2. records `preserved` if live already matches;
3. stays `uncertain` (never re-sends) if the prior outcome was pending/uncertain and scores have not landed;
4. **replay guard**: holds if `ObservedQuizResult` is empty or the live result version, score or `graded_at` moved since the pull;
5. persists `pending`, calls `PostScores`, maps the error through `failedOutcome` (`outcomes.go:15`).

`failedOutcome` turns `canvas.Error.Uncertain` into `uncertain`, so fixing the classification in `internal/canvas` is enough for the workflow.

**Teammate hook.** `teammateClient` (`internal/workflow/team_route.go:191–214`) sets `OnWrite` to append `workspace.TeamLogEntry{Teammate, Course, CourseID, Method, Path, Status, Uncertain}` via `AppendTeamLog`, which writes `<InstallRoot>/.cse/team-log.jsonl`. `MapError` turns `Kind=="authentication"` into `teammateAuthError`. `RedactTeamLogPath` (`internal/workspace/teamlog.go:31–43`) drops the query and replaces a numeric segment after `submissions`, `users` or `students` with `:student`. On an owned course `ForCourse` hands the teammate client to `postItemScores`, so the native launch already uses the teammate's PAT.

## What to build

1. **Classify native write outcomes like ordinary writes.** Factor a tiny helper (for example `writeStatusUncertain(status int) bool`) out of `statusError` and use it in both places. For `PostScores` only:
   - 408, 3xx and ≥500 responses → `Uncertain`;
   - a 2xx whose body cannot be read, is oversize or cannot be decoded → `Uncertain` (the write may have landed);
   - leave other 4xx as definite failures.

   Do not change the GET callers' behavior, and do not add retries.
2. **Emit the audit hook for the grading write.** After `PostScores` gets a response (or a transport error), call the parent client's `OnWrite("POST", path, status, uncertain)` exactly once. Give `NewQuizGrading` access to the parent client's hook when it is created.
   - The path must not carry attempt identifiers. Extend `RedactTeamLogPath` (or redact at the call site) so the `quiz_sessions/{id}` and `participant_sessions/{id}` segments become `:session`.
   - The host is the quiz host, not Canvas. Log the path only, as today.
3. **Token requests are not course writes.** `POST /api/v1/jwts` and `POST backend/api/native/launch` mint tokens and also run on every pull. Do not log them; that would flood the Team page with non-changes.
   - Clarify row 14 in `docs/team/README.md` to say it covers every change written to Canvas or New Quizzes with a teammate's key, and that token requests are not logged. That is a wording clarification, not a policy change. If you think otherwise, ask the developer.
4. **Name a rejected teammate key.** Have `secureJSON` return `Kind:"authentication"` for 401, and apply the parent client's `MapError` to errors returned from `newQuizNative`/`NewQuizGrading`, so a revoked teammate key gives the same teammate-named message as ordinary calls.
5. **Throttling (only if cheap).** Wrapping the `PostScores` request in `acquire`/`release`, and calling `throttle` on 429, is acceptable if it is a few lines. Skip it rather than restructure the native client.

## Done when (focused tests)

- **Canvas level.** Extend `newquiz_grading_test.go`'s `gradingServer` so the results POST can return a chosen status/body. Add a table test with these cases: 408, 302 without Location, 500, 2xx unreadable body, 2xx invalid JSON → `Uncertain`; 400 and 403 → not uncertain; 401 → `Kind=="authentication"`.
- **Audit.** On a teammate-owned course, posting item scores writes exactly one redacted team-log entry for the results POST, with no token, session ID or name in the raw JSONL. Build it from `team_route_test.go`'s `newTeamKit` plus `new_quiz_item_scores_test.go`'s `anyHostTransport`, because the quiz/backend hosts need host rewriting.
- **Operator course.** Your own course (co-teaching or unowned) logs nothing, as today.
- **Existing tests.** `TestItemScoresPostBeforeTheCommentAndNeverRepeat`, `TestUncertainItemScoreWriteReconcilesInsteadOfRepeating` and the other item-score tests still pass unchanged.
- **Full checks.** Run the full check list from the execution notes.
- **Records.** Update `internal/localui/capabilities.go` and `docs/engine-capabilities.md` only if a stated limitation changes (for example the team log now covering item scores). Mark that as local evidence. A teammate item-score write has not been observed live.

## Not in this slice

- Replacing the native client, a general operation framework, retries.
- The other non-Canvas storage clients (`quizmedia.go`, `upload.go`), which also skip `OnWrite`. Note them in the commit message if you confirm they write with a teammate key; do not fix them here unless trivial.
- Live Canvas calls. A teammate New Quiz item-score write is a real-class grading write and needs a concrete teacher request.

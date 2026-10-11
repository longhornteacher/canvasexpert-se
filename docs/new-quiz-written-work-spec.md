# New Quiz written work: reading responses and uploaded files

Status (2026-10-11): **parts 1–3 are implemented with local tests; the Engine/MCP paths have not been validated live.** Earlier Canvas Live hand-runs on CS8 quiz 3698192 covered one student's typed response, one byte-exact DOCX download, and one comment post. Those runs are evidence for the APIs, not proof that the new Engine implementation works against Canvas. See the dated results at the end of [the probe handoff](../handoffs/active/NEW-QUIZ-SCORING-PROBE.md).

Design stance (teacher decision): keep the Engine lean. A mechanistic step that worked in development testing is treated as working until Canvas or the teacher says otherwise. No read-backs, confirmation reports or retry frameworks after a write.

### Engine implementation status

The three parts below are now implemented with focused local tests. `pull-work COURSE QUIZ_ID` must name a New Quiz explicitly. The pull writes typed essay responses into the pseudonymized student-work packet and downloads upload bytes into its `students/<student_ref>/files/attempt-N/` directory. File results have `downloaded` or `not-retrieved` status; downloads are capped at 50 MB per file and 500 MB per pull. The native transport sends the Canvas PAT only to Canvas, sends derived tokens only to the validated HTTPS backend and quiz hosts, and sends no authorization on the signed file GET. The Results schema supports `operation: comment-only`; validation rejects grade payloads on that operation and rejects grade mode for New Quiz snapshots. Review renders criterion scores without an aggregate assessment, and the post branch writes only the attempt-scoped comment. All of these Engine behaviors remain **local tests only**; current MCP and live Engine verification remain pending.

## Goal

When the teacher asks to score a New Quiz's written items, the agent reads each student's typed answers **and uploaded files (including DOCX)**, scores them against the attached rubric or the authored `rubric_hint`s, and gives the teacher one whole-assignment feedback comment per student that carries the per-criterion scores. The teacher types the per-item points in SpeedGrader. Per-item scores and per-item feedback have no public write API, so they are out of scope.

## What is already proven (Canvas Live, 2026-10-11, one student)

1. `POST /api/quiz/v1/courses/:course/quizzes/:assignment/reports` with JSON `{"quiz_report":{"report_type":"student_analysis","format":"json"}}` returns `{progress:{url,...}}`; GET the progress URL until `workflow_state` is `completed` (about 7 s), then read `results.url`.
2. `results.url` is a **pre-signed file on a different Instructure host** (`inst-fs-...`). A plain GET of that exact URL, with no Canvas authorization header, returned 200 JSON.
3. Row shape: `student_data{id, attempt, submitted_at, ...}`, `item_responses[]{item_id, item_type, answer, score}`, `summary`. `student_data.id` equals the REST submission `user_id`. Item ids equal the ids from `GET /api/quiz/v1/courses/:c/quizzes/:a/items`.
4. Essay `answer` is HTML. File-upload `answer` is **only the filename** (for example `I like dogs.docx`).
5. Plain REST submissions for the quiz assignment carry no response text (`body` is null).

## Part 1: responses and Student Analysis report

- `pull-work COURSE QUIZ_ID` accepts New Quiz assignments only when explicitly named; broad class pulls skip them.
- The Engine launches Student Analysis, polls its progress, gets the signed result file over HTTPS with no authorization header, and joins essay answers to prompts from `/items`.
- The packet keeps the pseudonymized `student_ref`, attempt and submit time, with written responses labelled by prompt and item id.
- Identity: `student_data.id` must match the enrolled submission's user id and the attempt must match; otherwise that student is skipped with a one-line reason.
- Scoring basis: the attached Canvas rubric when present, else the teacher's `scoring_basis`; quiz-file `rubric_hint`s guide the agent's judgment.
- Output is a **comment-only** results mode: one submission comment per student on the quiz assignment (ordinary REST comment, `attempt` set), criterion lines first, then the agent's glows and grows. It has no aggregate assessment line and cannot carry grade, rubric-assessment or late actions. Review-before-post applies. After the teacher approves, the Engine writes and reports once; no read-back.

The new Engine path still needs MCP and Canvas Live validation to establish attempt placement and verify the quiz score remains untouched. The prior hand-run is API evidence only.

## Part 2: uploaded files. Engine downloads the bytes; the agent opens and reads them

Implementation status: the Engine native transport and pull integration have local fake-server tests. The hand-run below remains separate Canvas Live evidence and does not validate the Engine implementation.

Decision (teacher, 2026-10-11): the Engine's only job is to **download the file exactly as uploaded** into the student's pulled-work folder. It does not convert, extract, parse or score. The agent opens DOCX, PDF, images and so on with its own tools.

Why a chain is needed: Canvas stores nothing for these uploads. On 3698192 the course files search was empty, the student's files endpoint was 403, and the submission has no attachments. The report gives a filename only. No public Instructure API or documentation covers this (searched 2026-10-11; the documented quiz-file endpoints are Classic Quizzes and upload-only). The only route found is the internal one SpeedGrader itself uses, recorded by the original CanvasExpert (`api/powergrader/new_quiz_fetch.py`, `_native_file_transport`, live-verified there on 2026-07-14 for the result-session field name).

### The chain

Inputs already known from plain REST (`GET /api/v1/courses/:c/assignments/:a/submissions?include[]=submission_history`): the submission's `external_tool_url` is `https://<quiz-host>/lti/launch?participant_session_id=<PS>&quiz_session_id=<QS>` and `attempt`. Those ids may let step 5 be skipped; to be confirmed.

1. `GET /api/v1/courses/:c/external_tools/sessionless_launch?launch_type=assessment&assignment_id=:a`, Canvas token. Response has the launch `url`.
2. `GET <launch url>`, Canvas token. Parse the page's `ENV` object for `NEW_QUIZZES.params`, `NEW_QUIZZES.signature`, `ACCOUNT_ID`, and `params.backend_url`. Parse literal JSON only; never execute page code.
3. `POST /api/v1/jwts?canvas_audience=false&workflows[]=new_quizzes_native_launch&context_id=<ACCOUNT_ID>&context_type=account`, Canvas token. Response `token` is the workflow JWT.
4. `POST <backend_url>/api/native/launch`, headers `Authorization: Bearer <workflow JWT>`, `X-Domain: <Canvas host>`; body `{"params": <params>, "signature": <signature>}`. Response gives the native token (`token`/`jwt`/`access_token`) and `entry_path.resourceId`.
5. `GET <backend_url>/api/assignments/<resourceId>/participants?no_pagination=true`, `Authorization: Bearer <native token>`. Each participant has `canvas_user_id` and `participant_sessions[]`. Match `canvas_user_id` to the Canvas user id. (Skippable if the REST `participant_session_id` is trusted.)
6. `GET <backend_url>/api/participant_sessions/<PS>/results`, native token. Response gives `quiz_host`, `result_token`, and the quiz session id (`quiz_api_quiz_session_id`, older shapes `quiz_session_id`).
7. `GET <quiz_host>/api/quiz_sessions/<QS>`, headers `Authorization: <result_token>`, `AuthType: Signature`. The authoritative result supplies its id; the attempt number is not reliable here.
8. `GET <quiz_host>/api/quiz_sessions/<QS>/results/<result id>/session_item_results`, same signature headers. For each requested file-upload item, `scored_data.value[]` is a list of `{name, url, size, id}`. Require the item result's attempt to equal the attempt being pulled; otherwise skip those files.
9. Download each `url` with a plain HTTPS GET and **no authorization header** (the URL is pre-signed). Stream to `<name>.partial`, then rename. Remove the partial on any failure.

Steps 1 to 4 carry credential-derived tokens to the New Quizzes backend, and 6 to 8 to the quiz host. That is the sensitive part.

### Engine contract (lean)

- Runs only inside `pull-work` for a New Quiz assignment, only for attempts that have a file-upload item, and only when the teacher asked to score that quiz.
- Host rule: the only non-Canvas hosts contacted are `backend_url` from the Canvas launch page and `quiz_host` from the backend's own result, both HTTPS and both under an `instructure.com` domain; the signed storage URL (observed on `amazonaws.com`) must be HTTPS but is not host-restricted, because it comes from the quiz host's own response. Never follow a host named by student content.
- Files land in the student's pulled-work folder, original filename sanitized (basename only, no path parts, length-capped); on a name clash keep both by numbering. A re-pull skips a file already present with the same name and size.
- The student-work document lists each file with `name`, `path`, `size`, `item_id` and `status: downloaded` or `status: not-retrieved` plus one short reason. The packet still includes the typed answers either way.
- If any step fails for a student, that student's files are `not-retrieved` with the failing step's name; the agent scores what it can read, says which file it could not open, and the teacher opens it in SpeedGrader. No retries beyond one, no cache, no state machine, no extra confirmation after a download that succeeded.
- Size cap per file and for a pull (suggest 50 MB and 500 MB); larger files are listed as `not-retrieved: too large`.
- No decoding, no text extraction, no antivirus-style scanning in the Engine. Treat contents as untrusted data in the agent, as for all student work.

### Verified live 2026-10-11 (one read-only run, teacher-authorized, CS8 quiz 3698192)

The whole chain worked end to end and the file downloaded byte-exact (13,395 bytes, matching the declared size); the agent then opened the DOCX and read its text. Corrections to the chain as first written, all learned from that run:

- **Cookie jar required for steps 1 and 2.** The `sessionless_launch` URL (`session_token` + `verifier`) sets a Canvas session cookie and redirects to `/courses/:c/assignments/:a/launch`. Without the cookie the request falls through to the district's SSO login (a Microsoft sign-in page). Use one HTTP client with a cookie jar for steps 1 to 3.
- **`Accept: application/json` is required** on the backend (steps 5, 6) and the quiz host (steps 7, 8). Without it step 5 returns 406 and step 7 returns 400 `UnknownFormat`.
- Step 2: the launch page's `ENV` parsed as written (`NEW_QUIZZES.params`, `signature`, `ACCOUNT_ID`, `backend_url`); `backend_url` is an HTTPS `*.instructure.com` host.
- Steps 3 and 4 worked with the personal access token as written (jwts 200, native launch 200: a native token and `entry_path.resourceId`).
- Step 5 worked: the participants list returned the student by `canvas_user_id` with one `participant_sessions[]` entry carrying `id` and `quiz_api_quiz_session_id`. Whether the REST `participant_session_id` alone could replace this step was **not** tested; keep step 5.
- Step 6 keys: `host` (the quiz host, HTTPS `*.instructure.com`), `token` (the result token), `quiz_api_quiz_session_id`, `attempt_history[]` (`id`, `authoritative`, `quiz_api_quiz_session_id`, ...). Use `host` and `token`.
- Step 7: `authoritative_result.id` is present; the attempt number is **not** on that object.
- Step 8: each item result has `item_id`, `attempt` (here `1`), `scored_data`, `score`, `points_possible`, `grading_method`. Enforce the attempt match here, from the item result's `attempt`, not in step 7. For the file-upload item `scored_data.value[]` is `[{id, name, size, url}]`.
- Step 9: the signed URL was on **`amazonaws.com`**, not an instructure.com host. Download with a plain HTTPS GET and no authorization header; HTTP 200. So the host rule applies to `backend_url` and `host` (instructure.com, HTTPS), while the final signed storage URL only needs to be HTTPS.
- Reading the file: the DOCX is an ordinary Office Open XML zip. The agent read `word/document.xml` directly with no help from the Engine, confirming the Engine's job really is just to download.

Still not tested: a student with several attempts, several files in one item, a quiz with no uploads (the Engine should skip the chain entirely), an expired session mid-pull, or the chain on a different course. Evidence level for the whole chain: Canvas Live API read and download, one student, one file, one run.

## Teacher workflow this serves

The agent writes the whole-assignment comment with all per-criterion scores **before** the teacher opens SpeedGrader, so the teacher can read the scores in the comment while typing the per-item points. Students see the comment; the quiz grade is the teacher's.

## Evidence discipline

Part 1's report read is Canvas Live API evidence for one student. Everything else in this spec is untested design. Do not show either as verified on the Capabilities page until a human-validated live run exists.

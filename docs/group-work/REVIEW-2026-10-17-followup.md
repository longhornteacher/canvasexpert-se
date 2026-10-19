# Groups update follow-up review — 2026-10-17

Reviewed the Groups implementation from `942bc7f` through HEAD `fb26bfa`, including the earlier corrections, the shared/individual assessment slices, graded discussions, review dispatch, CLI/MCP integration, seed guidance and recorded CS8 evidence. The earlier [review](REVIEW-2026-10-17.md) remains historical evidence; this review finds additional defects in the current combined implementation.

The direction fits the Engine/playbook boundary: typed group results, reused exact assessment math, separate shared/private comments, assignment-specific propagation audiences, saved outcomes, and no Tier C expansion. The recorded CS8 checks establish useful happy-path evidence. The grading and discussion-evidence defects below should be corrected before treating those paths as ready unchanged. They do not invalidate unrelated group setup or the recorded live observations.

## Findings

### 1. P1 — Renew individual grading mode before each grade/rubric write

Location: `internal/workflow/group_post.go:1015`, with the multi-action helper in `internal/workflow/post.go`.

`postGroupIndividual` checks the assignment before each student, then delegates the whole student's operation to `postGrade`. For a coupled rubric, that helper writes the rubric, reads the submission and writes the grade without reading the assignment mode again. A switch to shared grading after the rubric write therefore permits an individual student's score to be sent while Canvas is in shared mode. Checking before the next student is too late for this write. The recorded Gate P observations establish the consequence of that mode: grade propagation to the group.

The local reproduction flips the mode immediately after the first rubric PUT. The next PUT still contains `submission.posted_grade: 20`. The fixture establishes the incorrect outgoing request; no new live propagation test was run.

Correction: renew the group mode, relevant basis and student evidence before each distinct numeric/rubric action on this path. Keep accepted outcomes and hold remaining actions on a change. A focused group-specific pre-write check is sufficient; no general operation framework is needed.

### 2. P1 — Parse the documented fallback reply field

Location: `internal/workflow/discussion_pull.go:46` and `:226`.

The entry DTO accepts `replies`, but Canvas's paginated entries endpoint returns its embedded preview as `recent_replies`. For up to ten replies, `has_more_replies` is false and the response need not contain `replies_count`. The decoder drops the preview, the fallback skips fetching replies, and the packet reports complete coverage with student work omitted. This affects the supported fallback when the full cached view is unavailable.

The local reproduction uses the documented shape: one root, one `recent_replies` entry and `has_more_replies: false`. The returned packet has one entry and `status: complete`.

Correction: preserve the documented preview and resolve additional replies through the existing pagination path. Check this external response shape in the regression fixture. See the official [Discussion Topics API, entries response](https://developerdocs.instructure.com/services/canvas/resources/discussion_topics#method.discussion_topics_api.entries).

### 3. P1 — Preserve live coverage status when validating missing discussion work

Location: `internal/workflow/discussion_work.go:169` and `:209`.

The post-time conversation read returns a packet with coverage status and source limitations, but `newDiscussionGuard` retains only its entries. `hold` checks the original pull's coverage rather than the fresh read's. If the original pull was complete and the fresh read cannot retrieve a reply list, unchanged visible own posts can still match the digest. A component assessed as missing can then post zero without complete current evidence.

The local reproduction starts with a complete pull, makes the full view unavailable and a required reply list unreadable during posting, and keeps the student's visible opening post unchanged. The Engine still sends a rubric at 50/100 and grade 10/20, including the zero for the missing reply.

Correction: retain relevant live coverage and unknown-author limitations. A missing component requires complete relevant current evidence; a partial independent conversation should not become a blanket hold on unrelated students.

### 4. P2 — Do not replay an uncertain individual comment-only write

Location: `internal/workflow/group_post.go:1013`; inherited helper behavior at `internal/workflow/post.go:210`.

The new individual group path calls the existing `postCommentOnly`. That helper preserves an exact live text match, but otherwise overwrites a saved pending/uncertain outcome with pending and sends another PUT. A failed lookup is not proof that the first comment failed. It can duplicate a comment after a timeout/500 or repost one the teacher subsequently removed. It also does not retain the first attempted text if feedback was edited.

The local reproduction returns HTTP 500 for the first private comment, verifies that the saved outcome is uncertain, and resumes the same results file. A second comment PUT is sent. This weakness already exists in the helper; the new group path inherits it.

Correction: reconcile exact attempt-scoped text when available and otherwise retain uncertainty without replay, preserving the saved attempted text. Reuse the behavior already implemented for shared/private group comments.

### 5. P2 — Unsubmitted placeholders must not block a structural assignment edit

Location: `internal/workflow/author.go:663`.

The submitted-work protection treats any nonempty submissions response as evidence of submitted work. Canvas also returns unsubmitted submission records. Consequently, an explicit group-set/grading-mode edit can be held before anyone submits, contrary to slice 08's contract. The diagnostic incorrectly says submissions exist in the sense of completed work. `Client.List` also follows pagination, so `per_page=1` does not make this a one-row bounded check.

The local reproduction returns only a row with attempt 0, null `submitted_at` and `workflow_state: unsubmitted`. The requested mode edit is held and no assignment PUT is sent.

Correction: distinguish actual submitted work from placeholder records, and preserve the protection when genuine work exists. See the official [Submissions API](https://developerdocs.instructure.com/services/canvas/resources/submissions#method.submissions_api.index), which distinguishes submission records from an actual `submitted_at` value.

### 6. P2 — Send only changed group-set settings

Location: `internal/workflow/groups.go:331`.

Conflict checks consider fields changed from the baseline, but the outgoing update includes every populated optional setting. A pulled baseline populates self sign-up, group limit and automatic leader settings. Renaming a set can therefore overwrite an unrelated setting changed in Canvas after the pull, without a conflict.

The local reproduction starts with disabled self sign-up in the baseline, enables it in live Canvas, and requests only a rename. The outgoing rename patch includes `self_signup: null`, disabling it again.

Correction: build the patch from the same baseline differences used by conflict detection. No additional read or generalized merging system is needed.

### 7. P2 — Reconcile the shipped group-work playbook with delivered features

Location: `workspace-template/playbooks/group-work.md:5`, `:11`, and its shared-product assessment section.

The playbook still tells the teaching agent that graded discussions are out of scope and explicitly instructs it to describe them as unsupported, despite the later discussion-assessment section and implemented slices 05b/07. It also says a group is assessable only when `shared_assessment_status: ready`; individual-mode pulls deliberately return `not-applicable`. These are executable instructions for the teaching agent and can prevent the teacher from using delivered capabilities.

Correction: update the introductory scope, planning and authoring guidance, and make the readiness rule specific to shared grading. Explain individual review cards accurately. Reconcile stale per-slice capability descriptions with the later CS8 live-check entry without claiming unobserved discussion scoring or student views were tested.

## Validation and evidence limits

- The existing `go test ./...` suite passed with the pinned Go 1.27.1 toolchain.
- `go vet ./...` passed, a fresh CLI build succeeded, and the compiled MCP smoke check passed all 37 tools with no Canvas writes.
- Six focused local probes reproduced findings 1–6. Each asserts the intended protection and fails against this HEAD for the behavior described above. Finding 7 is direct inspection of the shipped instructions and individual-mode status contract.
- The probes use synthetic TLS HTTP fixtures and Go overlays; implementation and existing tracked tests were not modified. Local artifacts are under `tmp/_group-review-20261017/`: `review_probe_test.go`, fixture overlays, `overlay.json`, `probe-results.txt`, and the fresh executable. The directory is ignored by Git and by Go's recursive package discovery.
- Reproduce locally from Dev with `.\.tools\go1.27.1\go\bin\go.exe test -overlay ./tmp/_group-review-20261017/overlay.json ./internal/workflow -run '^TestReviewProbe' -count=1 -v`. The six failures are expected demonstrations of the findings, not failures in the existing suite.
- No Canvas Live requests, teacher UserInstall/UserSync changes, release, commit or push were performed. Earlier authorized CS8 observations remain evidence of the specific paths recorded in the README; this review adds local/source evidence only.

## Recommended correction order

1. Individual posting mode checks and uncertain comment recovery (1, 4).
2. Discussion fallback decoding and relevant live completeness (2, 3).
3. Authoring checks/patches and consistent seed/capability guidance (5–7).

Use focused regressions for the demonstrated failures and retain existing scope, privacy, arithmetic and review selection checks. These corrections do not require another live probe, Tier C work, a new recovery framework or a broad migration.

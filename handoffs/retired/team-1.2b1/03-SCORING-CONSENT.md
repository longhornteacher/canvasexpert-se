# 03: Scoring consent (CE) and hidden grades

Read [00-BRIEF.md](00-BRIEF.md) first. Contracts R3 and R4 apply here. Requires slice 02 (`ForCourse`, `Owned`). Suggested model: Sonnet. This is the most consequential slice: when in doubt, hold and report; never post visibly.

## Result

In an **owned course**, whichever key is routed:
1. **Pull.** Work is pulled only for assignments whose name contains `(CE)`.
2. **Post.** Posting re-checks the tag on the live assignment. It sets the assignment to manual posting, and only then writes grades, rubrics, comments or late timing. Students whose grade is already visible are held.
3. **Report.** The post report and review page say the grades are hidden and that the teammate will post them.

The operator's own courses behave exactly as today.

## Work

### 1. Tag rule

In `internal/domain`, add `func HasConsentTag(name string) bool { return strings.Contains(strings.ToLower(name), "(ce)") }`.

**Test:** a table of `Essay 3 (CE)`, `(ce) quiz`, `Essay (C E)` → false, `CE Essay` → false, and empty.

### 2. Pull rules (`internal/workflow/pull_work.go`)

`owner, owned, err := s.Owned(course)` comes from slice 02; an error is an input error. Apply all of the following only when `owned`.

**Explicit assignment pull** (`query != ""`, `FindAssignment` at :229):
- If the found assignment lacks the tag, return the R3 pull error.
- Wording: `Essay 3 in Turner P2 isn't tagged (CE), so Ms. Turner hasn't opted it in to CE-SE scoring. Ask Ms. Turner to add (CE) to its name in Canvas.`
- Use `a.Name`, `course.Nickname` and `owner.Name`.

**Session pull of a named owned course** (`Assignments` at :235, the loop at :258):
- Before `detailedSessionSubmissions` (:249), drop untagged assignments from the list passed to it, so their submissions aren't read.
- Keep a count, and add one report warning: `Turner P2: 7 assignments not tagged (CE) were skipped.`
- Do not add rows for them.

**The authoritative re-read** (`GetAssignment`, :365) and the group path (`pullGroupAssignment`, `group_pull.go:111`):
- If the name read there has lost the tag, stop that assignment with the same message.
- This covers a rename between list and pull.

**`PullBuiltinTestStudent`** (:101): developer-only. It already requires a disposable prefix. Additionally require the tag when owned. This keeps the live check in 07 honest.

### 3. GraphQL post-policy method (`internal/canvas`)

**Allowlist.** In `apiURL` (`client.go:130`), allow the exact path `/api/graphql`, in addition to the two prefixes. No other `/api/graphql*` path.

**Method:**

```go
func (c *Client) SetAssignmentPostManually(ctx context.Context, assignmentID int64) error
```

- It POSTs JSON `{"query": <R4 mutation>, "variables": {"id": "<assignmentID>"}}` as a write, using the same write path as `Write`: no retries, status mapping, uncertainty, the 4-request cap and the `OnWrite` hook. Reuse `Write`'s internals if it accepts a JSON body. Otherwise add the smallest JSON-body variant.
- Decode `{data:{setAssignmentPostPolicy:{postPolicy:{postManually}, errors:[{attribute,message}]}}, errors:[{message}]}`.
- **Success** is `postManually == true`, with both error arrays empty.
- **Failure** (with HTTP 200) is `&Error{Kind: "validation", Body: <first message, truncated>}`.
- **Transport uncertainty** keeps `Uncertain: true`.

**Tests:** success, a top-level `errors`, a mutation-level `errors`, `postManually:false`, a 500 (uncertain), and that `/api/graphql/other` is refused by the allowlist.

### 4. Post rules (`internal/workflow/post.go`)

In `Post`, after the identity check (`:112-115`) and before the per-student loop (`:124`), when the results course is owned:

1. **Tag.** If `!domain.HasConsentTag(liveAssignment.Name)`:
   - every row not already complete and not skipped becomes `held`, with the message `not tagged (CE) in Canvas right now; nothing posted`;
   - save once and return the report with no error. The results file keeps all scoring text (preserve unsent work).
2. **Hide.** Call `s.Client.SetAssignmentPostManually(ctx, assignmentID)`.
   - If it is uncertain, retry once.
   - On any remaining failure, hold every pending row with `couldn't set Canvas to hide these grades; nothing posted`, save, and return with no error.
   - Do it once per `Post` call.
3. **Per-student visibility.** At the existing pre-write read (`GetSubmission`, :179), when owned:
   - if `!live.PostedAtPresent || live.PostedAt != nil`, hold that row with `grade already visible to the student; <Name> can change it in SpeedGrader`, then continue;
   - this applies to all operations, including `revise-feedback` and `comment-only`.

**Late timing.** `postLateTiming` already reads the submission (`late_timing.go:84`) before `writeLateTiming`. Both late-only rows and graded rows' late steps go through it.
- Put the visibility hold **inside `postLateTiming`**, right after that existing read, gated on ownership.
- Add no extra read.

**The policy call comes before any write.** Late-only rows sit earlier in the loop than graded rows. So the policy call (step 2) must happen before the loop starts, not lazily at the first graded row.

**Revise-feedback and resubmission paths** that re-read submissions (`:602`, `:636`) need no extra check beyond step 3's initial hold.

### 5. Group post (`internal/workflow/group_post.go`)

Apply the same three rules: tag after `GetAssignment` (:250), the policy before the groups loop (:279), and visibility in `groupPreflight` (:540).
- One visible member holds that group's shared action, using `holdGroupShared` / `holdGroupGrade`, with the R4 message naming "a group member's grade is already visible".
- Individual group-grading rows hold per student, as in step 3.

### 6. Report and review wording

**Post report.** `PostReport` gains:

```go
Owner string        // teammate name when owned
Hidden bool         // manual policy confirmed this run
HeldVisible int
```

`cmd/cse/remote.go` prints the R4 summary line under the table when `Owner != ""`. The review page's post result (the review approve callback in `remote.go`) shows the same line, since most posts come through review:

> `Turner P2 · Essay 3: 24 scored, hidden from students, ready for Ms. Turner to review and post. 2 held (already visible).`

`Hidden` is true only after a confirmed policy call this run, or when no write was needed.

**Review page.** `BuildReview(root, path)` (`review.go:134`) is a package function with no `Service` or client, and it reads no live assignment. For a results review of an owned course, it reads `context/courses.md` and `context/team.md` itself (via `ReadTeacherDocument` and `ReadTeam`) and adds a single fact at the top. Its source of truth is the Engine, not the playbook.
- When tagged: `Ms. Turner's course. Scores go in hidden; Ms. Turner posts them in Canvas.`
- When the tag is missing at review build: `Not tagged (CE) in Canvas right now. Approving will post nothing.`

The tag status uses the results document's saved assignment title. Do not add a Canvas read to the review build; `Post` re-checks the live name anyway.

### 7. Results init

`init_results` / `cse results-init` needs no change. The packet was only pulled because the tag was present.

## Tests (token-aware fixture from 02, owned course)

- Explicit pull of an untagged assignment → R3 error, and no submission reads.
- Session pull skips untagged assignments, with the count warning.
- Post, tagged:
  - the policy mutation is sent once, before the first submission PUT (assert on request order);
  - unposted rows post;
  - a row with `posted_at` set is held;
  - a row with `posted_at` absent is held.
- Post, tag removed since pull → all held, results text preserved byte-for-byte except outcomes, and no mutation or PUT sent.
- Policy failure (mutation `errors`) → all held, and no submission PUT.
- Policy uncertain, then success → one retry, then posting.
- Late-only row in an owned course → held if posted, with no extra submission read.
- A probe-style Service (`Keys == nil`, `Team` loaded) on an owned course still requires the tag and sends the mutation.
- An owned course whose owner is missing from `Team` → input error, with no writes.
- Group post: one posted member holds that group, and another group posts.
- Operator's own course: no mutation and no extra reads. Existing post tests are unchanged.
- Co-teaching variant (operator key routed): tag and hiding still apply.

## Acceptance

All of the above pass, plus full tests, vet and the diff check. CHANGELOG `### Scoring consent and hidden grades`, ending: "Local tests only; whether Canvas hides comments and handles re-grades of posted work as expected is pending live check (07)."

## Stop and report if

- Posting code has a path that writes a grade, comment, rubric or timing in an owned course without passing step 2. Name it.
- Tests show the existing post flow can't hold every row before the loop without restructuring beyond these files.

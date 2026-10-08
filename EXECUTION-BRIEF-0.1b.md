# CanvasExpert-SE 0.1b: execution brief

Prepared 2026-10-08 for a new implementation session led by **GPT-6.1 Sol**, using **GPT-6.1 Sol and GPT-6 Luna subagents**. This is a build specification and handoff, not a record of implemented functionality.

Read this entire brief before delegating. Then read [ARCHITECTURE.md](ARCHITECTURE.md), [CE-LESSONS.md](CE-LESSONS.md), and the [WorkLaptop report](WorkLaptopTestSuite/results/STAGCFKQ74-20261008-0640.md). Do not load this long brief into ordinary teacher grading sessions; the shipped workspace needs short, task-specific guides.

Navigation: teacher requirements are in section 3; scope and platform in 4–5; file/scoring/write decisions in 7–12; required live evidence in 13; delegation in 14; build order and acceptance in 15–18. Section 20 is a ready-to-paste opening message for the next session.

## 1. Mission and completion boundary

Make a **functional Windows 11 portable beta** of CanvasExpert-SE: a teacher talks to an existing coding agent, which uses a small `cse.exe` and plain workspace files to create/edit Canvas assignments, retrieve student work, assess it with the live rubric, and post accurate results to Canvas Live.

The principal beta path is an ordinary Canvas Assignment with text-entry submissions and an attached Canvas rubric. The program contains no LLM client or model selection system. The host agent supplies educational judgment and authoring; the executable supplies reliable Canvas and filesystem operations.

Success is an end-to-end working workflow on WorkLaptop, supported by actual Canvas evidence and a usable starter workspace. Compiling a CLI, completing mocks, writing a persuasive README, or showing an attractive setup page alone is insufficient.

There are three distinct completion claims:

1. **Built:** code, unit/integration checks, executable, and starter workspace exist.
2. **0.1b engineering-verified:** the actual executable works on WorkLaptop and the required disposable-course integration checks pass.
3. **Teacher acceptance:** the teacher's later real three-SCR exercise succeeds. That assignment has explicitly **not** been requested for creation yet. Do not create it during implementation merely because it is described here. Report teacher acceptance separately when it has not happened.

The real cross-machine OneDrive ZIP test is a separate distribution check. A locally working beta may be engineering-verified while that check remains pending; do not call distribution verified.

## 2. Authority, defaults, and questions

This brief combines three kinds of instruction:

- **Teacher decisions:** requirements stated in the planning conversation, summarized in section 3. Do not reopen them or simplify them away.
- **Implementation defaults:** concrete technical choices made in this brief to make execution possible. The orchestrator may refine details when evidence warrants it, while preserving the teacher's behavior. Record a short reason in the implementation handoff; do not ask the teacher to choose libraries, folder names, or ordinary algorithms.
- **Evidence-dependent choices:** Canvas and Windows behavior that must be tested before implementing the dependent write path. Their probes and fallbacks are specified below. Documentation is a lead, not a substitute for a district test.

Later direct teacher instructions override this brief. If older CE documents or an old SE paragraph conflict with section 3, use section 3. CE is an optional source of API lessons, never the product specification.

When the teacher starts an execution session with this brief, proceed with implementation, local checks, and reversible repository work. Obtain missing concrete inputs early, such as the identity of a disposable Canvas test course or access to a build host. Continue independent work while those inputs are pending. Do not introduce recurring permission requests for routine implementation choices.

The brief does not select a production course for experiments, authorize grading real students as a smoke test, authorize publishing a release remotely, or authorize creation of the teacher's future three-SCR assignment. Use an already designated disposable course and test participants for live experiments; otherwise ask for that target once. A configured PAT alone does not identify the intended experimental course. Never ask the teacher to paste a PAT into chat.

## 3. Teacher decisions that must survive implementation

### 3.1 Product shape and working style

- Personal tool shared with friends; speed and accuracy matter more than defenses against rare failures.
- Windows 11, no admin rights, no installer or mandatory runtime on the teacher's machine. WorkLaptop is the baseline.
- One teacher, one machine in use at a time. The workspace syncs through district OneDrive. Credentials are per machine.
- CLI and files, driven primarily by Codex or Claude Code. No MCP server in 0.1b.
- Canvas Live is the authority for existing content. Pull live material before editing or grading it.
- Teacher instructions provide authorization: "score and post" posts; "draft" does not; "show me first" stops with the draft. No approval flags, confirmation tokens, or mandatory preview/apply sequence.
- "Scoring session" alone discovers and pulls work, summarizes it, and asks what the teacher wants to do. It is not an instruction to post.
- "Pending my finalization" means SE posts directly to Canvas, then the teacher looks. Leaving the result unchanged is the teacher's finalization. **There is no pending-finalization state, approval notification, release command, or approval polling.**
- Grade visibility/posting policy and the nightly SIS sync remain outside SE's responsibilities. Do not inspect or manipulate them to implement the preceding phrase.
- Newly created assignments may remain unpublished while their requested pieces are being assembled. This construction behavior is separate from grade visibility.

### 3.2 Honest assessment and the gradebook

- Record the actual assessment in the rubric, including when it is lower than or equal to an existing grade.
- Write an assessed grade only when the entered grade is empty or the assessment is higher. Preserve a higher teacher-entered grade. Compare before Canvas's late penalty.
- Explain both values when they differ: the current work's assessment and the higher grade being retained. Do not present the retained grade as the quality of the current work.
- The teacher may deliberately enter a higher final grade than the rubric supports. Do not inflate criterion scores to make the rubric match it.
- Feedback should explain deterioration honestly and specifically. The user emphasized that understanding why a revision became weaker is vital to learning. Use respectful, concrete language about the writing rather than judgments about the student.
- Compare a new attempt with the **strongest preceding attempt**, not merely the immediately previous one. Interpret "strongest" academically from the work and rubric evidence, not from a teacher-adjusted grade.
- Preserve Canvas's late-penalty machinery. SE can forgive lateness for a genuine on-time first attempt, under section 10, but never computes or applies a late penalty itself.

### 3.3 Intelligent rubric use and feedback

The intended later acceptance exercise has three short constructed response (SCR) prompts in one assignment, submitted through Canvas's text box. The attached rubric totals 100 for one SCR. The agent must recognize the multiple required responses, apply the rubric appropriately, and infer sensible aggregation **without an extra warning from the teacher**.

Do not hardcode three responses, equal weights, a 100-point assignment, or the term SCR. The teacher explicitly expects intelligent inference. A generic guide may tell the agent to determine the units of work and how the rubric applies; it must not contain a special recipe for this hidden mismatch.

The feedback order is:

1. Overall assessment, with a separate retained-grade explanation when applicable.
2. Concise criterion scores for each response, for example `SCR 2 Conventions: 7/10`.
3. Conversational glows and grows grounded in the student's work, including comparison with the strongest earlier attempt when relevant.

Full criterion explanations belong in the attached rubric. Do not repeat its entire definitions or rating descriptions in feedback. Compact criterion labels and scores are wanted even when the Canvas grid is filled in.

### 3.4 Light identity hygiene

Real names may appear in student writing and `settings/students.md`. The teacher permits student data in the district synced folder. Aliases are only a convenience for names and identifiers in filenames and pulled metadata. There is no identity vault, scrubbing of student writing, or PII output gate.

Keep the PAT out of files, stdout, logs, agent context, and the starter ZIP. Keep the private student mapping out of ordinary agent reads. These are the existing boundaries, not grounds to recreate CE's privacy machinery.

## 4. 0.1b scope

This brief adopts the first complete text-assignment workflow as the beta scope. Broader architecture capabilities remain future work, not implicit prerequisites for shipping this beta.

### Required

- Portable `cse.exe`, version/help output, per-machine setup, and a clean starter workspace.
- Temporary local setup and student-table pages; credentials in Windows Credential Manager.
- Course nickname configuration and lightweight course/module discovery sufficient for authoring and grading.
- Assignment creation with text-entry submission, points, dates, description, attached rubric, module placement, and requested publication state.
- Pull/edit/push of an existing assignment using actual Canvas HTML.
- Pull current assignment/rubric, text submissions, previous attempts, comments, assessment information, and relevant student notes.
- Download attachments when present, preserve original bytes, and make plain-text uploads readable. Report unsupported/unreadable attachments accurately.
- Explicit assignment scoring and the configured-current-courses "scoring session" discovery path.
- Generic component-aware rubric scoring files, arithmetic validation, and intelligible feedback rendering.
- Direct Canvas posting, independent honest rubric assessment, higher-grade retention, late forgiveness where verified, and per-student outcomes.
- Targeted feedback revision, partial-success recovery, and avoidance of duplicate feedback on retries.
- Persistent alias mapping, stable student references, monitored flag, and notes association.
- Tests, actual Windows checks, disposable-course evidence, and a release ZIP built from a clean template.

### Deferred

New Quizzes, Classic Quizzes, differentiated Hub/Bridge workflows and tag targeting, standalone page authoring, standalone rubric libraries, curves, writing timelines, OCR/media extraction, external-tool grading, group-submission workflows, anonymous/moderated grading, additional agent-host adapters, and update services.

Do not implement fake success paths for deferred capabilities. Scope filters should identify and skip unsupported assignment/submission modes with useful reasons. A literal code submission can be assessed as text; do not build an arbitrary student-code execution service.

Do not silently drop required beta behaviors to meet a self-imposed time limit. If live evidence is missing, say which capabilities are unverified and finish the independent work.

## 5. Platform decision and known evidence

**Implementation default: Go, one Windows executable, stored under the workspace's `cse/` directory.** Keep the CLI free of required Python, Node, .NET, browser-driver, and C runtime installations. Embed the small local UI assets. Select and pin an appropriate stable Go release at execution time; record the toolchain and dependency versions.

The 2026-10-08 WorkLaptop report has 17 automated passes, two manual passes, 11 informational observations, and one untested distribution step. It demonstrates unsigned .NET probe execution from OneDrive and through `cmd.exe`, Credential Manager API access, loopback serving, browser launch, and unauthenticated Canvas connectivity. Two hundred local file writes took 854 ms and reads took 378 ms.

Do not overstate these results:

- The probe was not Go. Run the actual built binary here.
- Credential access was demonstrated; SE's actual write/read/encoding path still needs a round trip.
- A 401 response demonstrates network reachability, not authenticated API capabilities.
- The observed public certificate issuer and direct WinHTTP setting describe that run. They do not establish every future network/proxy configuration.
- Local OneDrive file timings do not measure cloud synchronization completion.
- Automated marked-ZIP extraction does not establish browser download plus File Explorer behavior. T3.4 remains pending.

Go was absent on this laptop. Prefer an existing developer build environment elsewhere, as the test report recommends. Do not install a system SDK on WorkLaptop, add global PATH entries, or change district policy to enable a build. If the next session has no build toolchain, ask for an available build location or the teacher's choice about a portable development toolchain; continue interface, source, and fixture work meanwhile. The teacher-facing ZIP must never require that compiler.

Use the Windows target architecture verified on the machine; do not infer it from Windows 11 alone. Prefer a pure-Go build with C interop disabled if the selected dependencies support it. Cross-compilation is a build step; it is not evidence that Windows-only credential code or the resulting executable works.

CE and SE may coexist. Use an SE-specific credential namespace, executable name, directories, and temporary loopback port. Do not read or migrate CE secrets automatically.

## 6. Responsibilities and internal shape

Keep one application with small ordinary packages. A reasonable layout is `cmd/cse`, `internal/canvas`, `internal/workspace`, `internal/workflow`, `internal/render`, `internal/platform/windows`, and `workspace-template`. The orchestrator owns exact names and shared interfaces. Do not turn this into a plugin framework or an event-driven service.

| Responsibility | Owner |
|---|---|
| Understand instructions, prompts, rubric intent, accommodations, and evidence | Host agent |
| Select defensible component weights and a strongest prior attempt | Host agent |
| Write teaching content and conversational feedback | Host agent |
| Parse files, resolve stable references, and validate numbers | `cse` |
| Fetch live state, compute grade-retention action, and target Canvas writes | `cse` |
| Render verified score headings and compact score lines | `cse`, using the agent's structured assessment |
| Make pedagogical changes or invent scores to repair bad arithmetic | Never `cse` |

Internal functions return typed results; only the command boundary renders text and exit codes. Do not reduce internal APIs to unstructured stdout parsing. No daemon, scheduled job, database, app server, or embedded LLM API.

CLI commands are short-lived processes; a large pull may legitimately take time and print useful progress. The setup/student page lives only for its command's interactive lifetime.

### Command contract

- `cse setup`: open the local credential form, validate credentials without displaying the secret, save per machine.
- `cse students`: open the local student mapping editor.
- `cse courses`: compact authorized-course discovery to populate nickname settings; no automatic grading of every accessible course.
- `cse modules <course>`: compact module discovery for the agent's placement decision.
- `cse push <file>`: create from a new draft, resume an incomplete create, or update a freshly pulled live item according to file type/state.
- `cse pull-item <course> <title-or-link>`: retrieve a unique live assignment. Return candidates for ambiguous titles; never choose a same-title object arbitrarily.
- `cse pull-work [course] [assignment]`: discover/pull appropriate work. With no scope, use only explicitly configured current courses.
- `cse post <results-file>`: validate, recheck relevant live state, post requested operations, and report outcomes.

Version/help are local and perform no Canvas work. Invoke by explicit workspace path; no PATH install is required. Resolve workspace root from a documented option or upward settings/template discovery, not by assuming the source repo is the teacher workspace. Handle spaces and non-ASCII paths.

Print concise summaries and paths to larger files. Send progress to stderr. Define stable exit outcomes such as success, invalid input, partial/held work, and authentication/environment failure. Invalid input is distinguishable from a successfully posted subset. The stdout/student outcome table is part of the supported interface.

## 7. Workspace identity and file contracts

Freeze version-one file contracts before parallel workflow development. Write a short format guide and synthetic examples. The following semantics are required; the orchestrator may choose readable field names without re-asking the teacher.

### 7.1 Stable references and folders

- Course nicknames are configuration, not a heuristic based on course titles. Initial familiar names are CS8, ELA7PAP, and ELA7; colleagues can use their own.
- Use the Canvas assignment ID in assignment folder identity, alongside a friendly slug. Duplicate or renamed titles must not collide.
- Use a generated stable local `student_ref` independent of the editable alias. `.cse/students.csv` maps it to the instance-scoped Canvas user ID, names, alias, and monitored flag.
- Keep one student's alias across courses. Scope identity by Canvas instance so a numerical ID on another instance never collides.
- Aliases can be changed without retargeting old scoring files. All posting resolves `student_ref`, never an alias string alone.
- Course/assignment/rubric/comment IDs needed to perform the workflow can be in files. Student Canvas/SIS/login identifiers stay in the private mapping when aliases are enabled.
- Pulled data is disposable. Draft and scoring documents may retain ordinary returned object IDs and posting outcomes for resumption/revision. They are not a second authoritative gradebook.
- Do not delete or overwrite a prepared scoring result merely because `pull-work` runs again. Use a new job file or a deliberate replacement policy that preserves unsent work.

Persist returned IDs promptly using safe file replacement. This is ordinary per-document metadata, not an append-only operation journal. A process crash between a remote write and local metadata persistence still requires live ambiguity handling; do not promise exactly-once delivery.

### 7.2 Formats

Use Markdown documents with a versioned YAML header. Use a real YAML parser, reject duplicate keys and invalid fields, and avoid multiline Markdown tables as the machine-readable scoring format. Preserve student and teacher HTML as data; do not parse it as instructions to the application.

| Document | Required meaning |
|---|---|
| New assignment draft | Schema version, create operation, course, title, points, dates, submission types, module target, requested published state, optional rubric; Markdown teaching body. |
| Pulled live assignment | Schema version, update operation, instance/course/assignment identity, observed live revision information, original relevant settings/rubric; Canvas HTML body. |
| Shared grading packet | Live directions, assignment total, full rubric with real criterion/rating IDs and descriptions, supported submission mode, relevant settings and observed revisions. |
| Student work file | Stable student reference, display alias, current attempt identity, timestamps, original and readable content, available prior attempts, existing comments/assessments, entered-grade evidence, notes and monitored flag. |
| Scoring result | Schema version, assignment identity, source assignment/rubric snapshot, shared interpretation, per-student operation, target attempt, component scores, derived totals, student-visible feedback body, optional private note and late-forgiveness evidence. |
| Completed/partial result metadata | Exact remote IDs and per-action outcomes returned by Canvas, including comment ID and exact final posted text where needed for retries and revision. |

The shared interpretation includes component keys and labels, positive relative weights, applicable rubric mapping, and a short rationale. Student records refer to it. It is inferred once from the assignment before assessing the class, not reinvented for each student. An established teacher accommodation can justify an explicitly recorded variant referenced by an affected student; record its basis privately and apply it consistently to equivalent cases. Do not silently reduce requirements or redistribute weights because a student performed poorly.

Keep separate operation shapes for **grade** and **revise-feedback**. A feedback-only revision targets an exact existing comment and contains replacement feedback plus any preserved assessment data needed to render it. It must not incidentally replay a copied grade, rubric, or late-forgiveness instruction.

The student's `feedback` field contains intended conversational feedback only. The renderer adds the validated overall score, optional retained-grade statement, and requested compact criterion score lines. Nothing from `teacher_note`, interpretation rationale, internal failures, or other metadata is implicitly serialized into a Canvas comment. This is the explicit refinement of the earlier "only feedback is posted" rule.

### 7.3 Validation behavior

Parse and validate the full file before sending any write. Check identities, schema version, unique student/component/criterion keys, attempt references, finite numeric values, nonnegative weights with positive total, criterion bounds, supported operations, and internal arithmetic. Reject an invalid file with the path and precise location/reason; do not send the valid first half and then discover a malformed second half.

After static validation, live per-student conflicts can hold just the affected students. An assignment/rubric change that invalidates the shared scoring basis holds that assignment's posting. Never invalidate other courses because something unrelated changed.

Do not silently clamp out-of-range points, invent missing scores, or alter feedback meaning to make a file pass. Return an actionable repair message to the agent.

## 8. Generic component scoring and comparison

### 8.1 What the agent must infer

Determine required response units from the directions, rubric, and submission. Distinguish "answer all three" from "choose one of three". A missing required response is part of the assessed work; do not average only the responses the student happened to supply. An unreadable response is unknown evidence, not automatically a zero.

Infer sensible weighting from the assignment's expectations and intellectual demands. Equal weights are often reasonable, but are not mandatory. Do not require the teacher to pre-annotate a multiprompt assignment. Give a short rationale in the shared scoring interpretation; do not ask for approval of routine inference.

Fix this interpretation for the assignment before grading individual students. Different performance must not cause weights to shift to improve a student's score. Apply teacher-defined accommodations where present without inventing them. If truly missing or contradictory information prevents a defensible assessment, ask a focused question; ordinary judgment calls are the agent's job.

### 8.2 Aggregation default for a reusable per-response rubric

For a rubric repeated across components, preserve its criteria and maxima. Let component `i` have relative weight `w_i`, criterion `j` have maximum `m_j`, and earned points `s_ij`. Normalize the weights to shares `a_i = w_i / sum(w)`.

- Component rubric total: `sum_j(s_ij)` out of `R = sum_j(m_j)`.
- Canvas aggregate criterion points: `q_j = sum_i(a_i * s_ij)` out of `m_j`.
- Assessed assignment points: `A = P * sum_j(q_j) / R`, where `P` is the live assignment's points possible.

This gives an honest aggregate Canvas grid while retaining each response's criterion breakdown in feedback. It supports a rubric total different from the assignment total. Do not multiply the assignment maximum by the number of responses or rewrite the attached rubric into three copies.

This formula is the mechanical default **after** the agent decides that the rubric repeats across the components. If the rubric already contains distinct criteria for separate prompts, use those criteria directly; applying the repeating-rubric formula again would double count. A general mapping must make this distinction explicit.

Use decimal arithmetic or an equivalent deliberate precision policy. Keep calculation precision through aggregation and round the proposed assessment at the posting/display boundary (default two decimal places, omitting trailing zeros). Compare that proposed posted value against the actual entered grade without rounding the existing grade downward. An unchanged retained grade must keep its original precision, including in a necessary preservation write. Do not round three shares to 33/33/34 merely to force integers. Do not pretend the displayed rounded per-criterion sum is more precise than it is; test small rounding discrepancies and keep the reported overall derived from the unrounded calculation.

Arithmetic example: on a 20-point assignment with a 100-point per-response rubric, two components assessed at 80 and 50 with inferred weights 1 and 2 yield a 60/100 aggregate rubric and a 12/20 academic assessment. If the entered grade is 15/20, preserve 15, fill the honest 60/100 aggregate grid, and explain assessment 12 versus retained entered grade 15. The example tests scale separation; it does not prescribe those weights for any real assignment.

This numeric normalization requires a positive assignment maximum and a positive scored-rubric maximum. Zero-point descriptive criteria can carry comments without contributing to the denominator. Entirely unscored/zero-point rubrics or extra-credit-only assignments need an explicitly supported alternative; do not divide by zero or invent a numeric basis. Do not infer extra credit or award above-maximum criterion points as an arithmetic repair.

Criteria that use discrete ratings still need accurate fractional aggregate points. Preserve a real rating ID only when that rating truthfully matches the reported criterion value. Never invent a rating ID for an average or attach an "excellent" rating to lower explicit points. Verify which combinations of points, rating IDs, and criterion comments Canvas accepts; use explicit points and a concise aggregate explanation where supported. If a rubric type cannot represent the required aggregate faithfully, hold that assignment and explain the specific limitation rather than posting a deceptive grid.

### 8.3 Strongest preceding attempt

Select one strongest **preceding attempt considered as a whole** from actual available work under the applicable current expectations, including its missing required components in the assessment. Do not exclude every incomplete prior submission and conclude that no baseline exists. Do not synthesize a fictitious prior attempt from the best paragraph or criterion across several submissions. Do not use the current attempt as its own baseline. The highest teacher-adjusted grade is not evidence that its associated writing was strongest.

When prior assessments use the same rubric and have adequate evidence, reuse them sensibly. When the rubric changed or a teacher's adjusted grade obscures quality, inspect the prior work. If the best prior work is unavailable, state the comparison limit and assess the current work on its merits; do not fabricate quotations or a regression claim.

Feedback identifies specific improvements and regressions relative to that strongest earlier work, even when the gradebook keeps the earlier/higher grade. A useful remark connects a change to its effect on meaning, evidence, reasoning, or conventions and gives an actionable next step.

## 9. Honest rubric versus retained grade: highest-risk write path

Treat these as separate quantities:

- `A`: current academic assessment in assignment points.
- `E`: live entered Canvas grade before late deductions, possibly absent.
- `G`: desired entered grade, `A` if `E` is absent, otherwise `max(A, E)`.

The rubric always reflects the current assessment. When `A <= E`, preserve the entered grade and still update the rubric accurately. When `A < E`, the feedback must distinguish the lower assessment from the retained entered grade. Do not mislabel a pre-penalty retained value as the guaranteed final visible grade after Canvas applies its own policies.

An entered zero is a real grade, not "empty". An excused submission is a distinct state; do not unexcuse it or replace it with a numerical grade during routine scoring. Missing work without a submission is reported, not automatically assigned zero. Unsupported pass/fail, moderated, anonymous, or group workflows are not ordinary numeric grading targets for this beta.

### 9.1 Obtaining the entered grade

Verify what the district API actually returns. Prefer a proven entered-score field. If the endpoint lacks it, verify any reconstruction from returned score and late deductions against real examples before using it. Do not equate `score` blindly with the entered grade, and do not confuse a student's hypothetical `student_entered_score` with the teacher-entered score. If a reliable pre-penalty grade cannot be determined for a row, hold that row with an explicit reason.

### 9.2 Rubric coupling policy

**For newly created SE assignments, attach rubrics with automatic grade calculation disabled.** The agent's validated aggregate explicitly controls whether an entered grade should be raised. The rubric remains a normal visible Canvas rubric. Canvas documents a `use_for_grading` association setting; its actual effect must be tested in this district. [Rubrics API](https://developerdocs.instructure.com/services/canvas/resources/rubrics)

Preserve the requested assignment maximum when attaching a rubric with a different total. Verify the relevant create/association behavior and supported option that avoids unintended point-total changes. A rubric of 100 and assignment of 20 must remain those two distinct scales; do not quietly turn the assignment into 100 points.

For an existing live assignment whose rubric automatically controls the grade:

1. Do not silently toggle the teacher's existing setting during a grading operation.
2. Test whether one supported submission update can save the honest rubric and an explicit retained grade without overwriting that grade. Test lower, equal, higher, empty, and late-penalty cases; do not assume field order establishes precedence.
3. If the combined update reliably preserves the desired entered value, use that tested path. Re-sending the same retained grade as a necessary part of that combined update is a technical preservation write, not permission to lower or inflate it. Document this narrow exception to "write only if higher."
4. If the behavior is unreliable, do not use "write the lower rubric, then repair the grade" as the fallback. Hold that assignment and ask once whether to decouple its rubric from automatic grading. Explain the change to the teacher's future manual workflow. Do not skip the honest rubric and call the assignment fully posted.

This is a concrete settings conflict that can require teacher input. It is not a universal approval step. Avoid it for SE-created assignments by using the independent default from the start.

### 9.3 Freshness and concurrency limits

Immediately before posting, re-read the relevant entered grades and current attempt identities, plus assignment/rubric information capable of invalidating the shared assessment. Compare content relevant to grading, not an unrelated course timestamp.

Hold a student who resubmitted since the scoring packet. Hold an assignment whose points, relevant directions, or rubric changed enough to invalidate the assessment. Other valid students/assignments continue as appropriate. Supply new paths/details so the agent can re-evaluate without a "refresh and retry" ritual.

A read followed by a write has a race unless Canvas supplies a tested conditional-write mechanism. Do not promise atomic "never lower" or absolute protection against a teacher editing at exactly the same time. Implement the live comparison, minimize the interval, and document this practical limit. Do not add locks, teacher-device election, or a distributed consistency system to solve it.

## 10. Canvas operations, posting, and recovery

### 10.1 Transport

One shared client owns URL construction, authentication, pagination, timeouts, bounded concurrency, and failure classification. Follow paginated responses to completion. Reuse connections. Start with four concurrent independent requests and adjust downward on throttling; there is no evidence yet for a more aggressive number.

Use Windows/platform trust through the chosen Go TLS configuration; never disable certificate verification to make a probe pass. Do not assume environment proxy settings cover Windows PAC/WinHTTP automatically. The observed laptop required no proxy; if a different machine does, diagnose and implement the actual required path or report it explicitly. [Go certificate verification](https://pkg.go.dev/crypto/x509), [Go HTTP client](https://pkg.go.dev/net/http)

The PAT is attached only to the configured trusted Canvas origin. File downloads may redirect; never forward it to a different host. Validate pagination and redirect destinations before attaching authentication. Preserve normal TLS verification on redirected downloads. Sanitize filenames and constrain workspace output paths so remote titles/attachment names cannot escape the intended directory.

Retry safe reads with bounded backoff for transient failures. Respect [Canvas rate-limit responses](https://developerdocs.instructure.com/services/canvas/basics/file.throttling). A timeout or transport failure after a write may be ambiguous. Do not apply a blanket HTTP retry policy to create/comment requests.

Distinguish credential failure, forbidden operation, unsupported feature, not found, validation failure, throttling, and uncertain completion. A 403 is not proof that the token expired. Error messages must not include secrets or entire private HTTP bodies.

### 10.2 Posting route default

**Use bounded per-student submission updates as the initial implementation default.** Correct attempt targeting, late overrides, honest rubric/grade separation, and useful returned outcomes take priority over minimizing the number of HTTP requests. One `cse post` may issue many requests; the two-CLI-call goal never meant two HTTP requests.

Canvas documents per-submission comment attempt targeting, rubric data, and late overrides. Its bulk grade endpoint has a different shape and returns an asynchronous progress object. Do not assume parameters from one endpoint work on the other. [Submissions API](https://developerdocs.instructure.com/services/canvas/resources/submissions), [Progress API](https://developerdocs.instructure.com/services/canvas/resources/progress)

The orchestrator may adopt a tested bulk path for suitable operations if it preserves every required semantic and actually improves measured time. Await asynchronous completion before reporting success. A queued/accepted response is not "posted." If a bulk result cannot establish row outcomes, resolve the uncertain subset instead of claiming every student succeeded.

### 10.3 Meaning of a successful row

A grade operation is complete only when all requested actions have definite outcomes: honest rubric saved where applicable, required feedback present on the intended attempt, desired grade preserved/written, and any requested late adjustment completed. Preserve distinct statuses for these actions. Do not call a comment failure a successful full assessment merely because a grade was saved.

Persist known returned IDs and outcomes in the scoring document. If local persistence fails after Canvas succeeds, say so and recover from Canvas on the next call. Do not blindly resend because the local file could not be updated.

When one action fails, report what succeeded and resume only unresolved actions on retry. Re-read live state before the retry so teacher edits and new submissions are respected.

Replaying an already completed result returns its completed status; it is not an instruction to restore old grades or comments over subsequent teacher edits. An intentional reassessment starts with a fresh pull and a new explicit grading operation. Historical local outcomes never become an instruction to undo a newer Canvas change.

### 10.4 Comment identity, duplicates, and revision

- Save the exact returned comment ID, target attempt, and exact final posted text in ordinary result metadata.
- A retry with that ID can identify the existing comment without adding another one. Handle a teacher-edited or deleted comment as live state; an old results file must not silently restore its previous text.
- If the response was lost before its ID was saved, inspect comments for the target student/attempt and compare the full intended rendered text. An unambiguous exact existing match can satisfy the add-comment operation. Author identity alone is never enough: the PAT acts as the teacher.
- Distinguish "the intended text is already present" from "SE owns this comment for future replacement." Do not claim ownership of a manually authored match merely to make revision easy.
- If there are multiple plausible matches or attempt association cannot be established, hold only the ambiguous action and ask for a precise target if necessary. Do not delete candidates to deduplicate them.
- A requested feedback revision edits the specific known or explicitly selected comment. Canvas documents an [edit endpoint](https://developerdocs.instructure.com/services/canvas/resources/submission_comments). Preserve grade, rubric, and late status for a feedback-only operation.
- If editing is unavailable, verify the actual reason before considering deletion/recreation. Preserve the existing comment until the replacement strategy is known. Do not silently use deletion as a generic workaround for permission or endpoint failures; report the limitation or obtain a specific choice for the affected comment.

No routine visible tracking codes, invisible HTML-marker assumptions, append-only send journal, or mandatory hash protocol are needed. If local references are lost, pull live comments and let the agent/teacher identify the requested revision target instead of inventing historical ownership.

### 10.5 Discovering work without endless regrading

The configured courses define scope. Within them, discover supported assignments with actual ungraded or resubmitted work. Do not silently exclude old work just because it predates an arbitrary cutoff. Skip excused/inactive/unsupported work with a compact explanation; avoid downloading the entire history of assignments with nothing actionable.

An assessed lower-scoring resubmission must not keep appearing as unfinished merely because its entered grade was preserved. **Prove this case in Canvas before finalizing discovery logic.** Test the actual workflow state, current-attempt grading indicators, and current-attempt rubric assessment after posting while retaining a higher grade.

Prefer live Canvas evidence that the current attempt has been assessed. If those fields remain insufficient, use completed ordinary scoring documents as hints and confirm their referenced feedback/assessment is still present for that attempt in the live pull. The files must not suppress a newer attempt or hide an incomplete post. A local "done" boolean alone is insufficient.

If the completed document is missing and Canvas evidence is insufficient, include the uncertain work in the summary and inspect it; do not silently treat it as done or automatically duplicate its feedback. Do not solve this by introducing a permanent scoring ledger.

### 10.6 Late forgiveness

Use the original rule: attempt 1 was on time under the student's actual applicable due date and contained a genuine submission. The agent judges genuineness from evidence and records a one-line reason. It does not infer genuineness from "a file exists" or from timestamp alone.

Apply only a tested lateness override to the current attempt. Do not change due dates, course penalty settings, posting policy, or the academic score to simulate forgiveness. Missing first-attempt history or uncertain effective due dates prevent automatic forgiveness, not ordinary scoring. Report the limitation or ask a narrow question when needed.

Test that a feedback-only revision leaves a previous lateness decision unchanged. Test that retrying an already applied override has the intended effect. Never increase lateness.

## 11. Authoring, live editing, and partial creation

### 11.1 New assignment sequence

Validate the complete draft locally first. Then create the assignment unpublished, attach its rubric with independent grade calculation, place it in the intended module, and apply the requested publication state only after required pieces succeed. If the teacher requested an unpublished assignment, leave it unpublished when complete.

Record returned assignment, rubric-association, rubric, and module-item IDs as they become known. An incomplete draft remains resumable using those IDs. A completed draft moves to the pushed-record location and refuses a second create, pointing to `pull-item` for future edits.

Do not recreate the assignment because attaching a rubric failed. Do not delete a successfully created assignment as an automatic rollback. Report a precise partial result and its Canvas link; leave it unpublished until the missing pieces are resolved.

Distinguish requested features necessary to publish from optional decorations. A failed required rubric/module operation is incomplete. An unrequested cosmetic feature must not become a reason to hold otherwise complete work.

### 11.2 Ambiguous create

A recently created exact-title match is a candidate, not proof. Compare course, object type, timing, and relevant intended fields; adopt only a uniquely supported match. Same titles are legitimate.

An empty or delayed lookup after a transport timeout does not prove creation failed. Use a bounded recovery lookup and preserve uncertainty if it remains. Retry automatically only when failure is known to have occurred before creation, or the endpoint supplies a tested idempotency mechanism. Ask only when the result remains genuinely ambiguous. The old "otherwise retry create once" recipe is superseded.

### 11.3 Live edit

Pull actual Canvas HTML and relevant settings. Preserve unfamiliar HTML rather than round-tripping it through Markdown. Send a targeted patch for fields actually changed; do not write a giant default-filled object that clears omitted fields.

Before updating, compare the relevant live item against the pulled baseline. If the teacher edited it, save/return a fresh version and tell the agent to reapply the intended change. Preserve the agent's unsent edit so it can be recovered. Use conditional writes if available and proven; otherwise document the read/write race as in section 9.

Do not recreate or modify an existing rubric merely because its assignment description changed. Shared rubrics require special care: never silently edit a rubric definition used by other assignments to adapt one assignment. Keep teacher-authored live rubric content authoritative.

Dates need explicit local timezone interpretation (the current teacher uses America/Chicago) and correct conversion for Canvas, including daylight-saving changes. Do not use the executor's timezone implicitly. Do not invent due/lock dates when the request supplies none; use established course preferences or ask when an essential date is actually missing.

## 12. Setup, student editor, and privacy implementation

Use native Windows Credential Manager APIs from the executable. Test actual secret-byte round trip with a dummy value before real setup. Do not put a real PAT on a shell command line, in an environment dump, or in a generated file. Keep the configured Canvas origin and identity sufficiently scoped to avoid applying one instance's mappings to another.

Serve the embedded form on loopback only, using an available ephemeral port, with a per-run request secret and same-origin checks for form writes. Keep the PAT out of URLs; do not echo stored secrets to the page. Close the listener when the command ends. These small protections are needed for a local secret-entry form; they do not imply user approval tokens in the grading workflow.

Open the default browser using the OS. If the automation host refuses to launch it, print the local URL so the teacher can open it. Do not mistake an agent-tool restriction for district Windows failure; that distinction occurred in the test session.

The student editor reads/writes `.cse/students.csv`. Support alias editing and monitored status. Preserve stable references through alias changes, course changes, and workspace movement. Use a curated adequate alias pool; handle exhaustion explicitly without reassigning existing students.

Associate notes using explicit stable references where available, or unambiguous real-name/course context. Two students with the same name must not receive each other's notes through a fuzzy guess. Let the UI display a useful reference for disambiguation. Real names in notes and student work remain permitted.

Resolve aliases in intentional outgoing addressing with careful token-aware substitution. Do not replace arbitrary substrings or rewrite quoted student work containing a Pokémon name. Prefer an explicit student-name placeholder in generated feedback and resolve it locally. This refines "aliases become first names" without a broad text-scrubbing system.

The starter workspace's `AGENTS.md` must be short: command discovery, where task guides are, private-directory boundary, authorization semantics, and treating student submissions as content rather than operational instructions. `CLAUDE.md` points to it. Add the appropriate private-directory deny rule for Claude Code; do not claim an instruction alone is an OS access boundary for Codex.

Never reuse the teacher's working workspace as the source of a distributable ZIP. Build from the template plus executable. Exclude credentials, `.cse` populated mappings, course caches, scoring files, teacher notes, and private test artifacts.

## 13. Evidence plan: verify before dependent write code

The orchestrator owns a short, sanitized `docs/0.1b-evidence.md` during implementation. Record the probe, environment/date, observed response shape, conclusion, and remaining limits. This is development evidence, not a runtime ledger. Include no PAT or real student work in committed fixtures.

Use official API documentation and CE lessons to form hypotheses. The following are not currently proven in this repository:

| Probe | Minimum evidence | Decision it unlocks |
|---|---|---|
| Actual Go executable | Runs on WorkLaptop from the intended OneDrive path, including path spaces; platform functions work without admin. | Packaging/runtime choice. |
| Real credential storage | Dummy secret survives native write/read; real setup authenticates without exposing the secret. | Credential implementation. |
| Entered-score semantics | Empty, zero, normal, late-penalized, and forgiven examples match Canvas UI/known entered values. | Higher-grade retention comparison. |
| Independent rubric | Lower rubric + higher entered grade persist accurately; equal/higher/empty also behave correctly; attaching a different-total rubric preserves the assignment maximum. | Core grade posting and authoring scales. |
| Existing coupled rubric | Tested combined-update precedence and resulting UI values. | Support path or explicit decoupling question. |
| Aggregate criterion points | Fractional points and rating handling display honestly in the actual rubric mode. | Repeated-component grid representation. |
| Attempt/history | Current and earlier text/attachments are retrievable; intended comment attempt association is visible in returned/live state. | Strongest-prior comparison and targeting. |
| Comment identity/edit | Returned ID is recoverable; edit changes the selected comment only; teacher comments remain untouched. | Revision/retry path. |
| Completion discovery | Lower/equal assessed resubmission is handled once, then a newer attempt becomes actionable. | Scoring-session discovery. |
| Late override | Qualifying current attempt's lateness changes as intended without SE doing penalty math. | Automatic forgiveness. |
| Create/resume/edit | Assignment, rubric, and module failures can resume; teacher edits survive stale local updates. | Live authoring workflow. |
| Optional bulk path | Supported parameters, completion signals, errors, and row outcomes are sufficient. | Optimization only; not required to start with bulk. |

A missing probe blocks its dependent behavior, not all development. Do not write a speculative production write path and mark a TODO where actual grade preservation should be. Pure transformations and local validation can be built while Canvas access is pending.

A probe may be a small disposable developer script or executable; it is not a requirement to ship another runtime. Keep probe resources in the designated disposable course. Record exact created resource IDs so cleanup can target only those resources, after their evidence has been captured.

## 14. Multi-agent execution plan

### 14.1 Models and authority

Use the user's requested models: **`gpt-6.1-sol`** for the orchestrator and substantive implementation/review agents; **`gpt-6-luna`** for bounded, clearly specified work. Do not silently substitute GPT-6 Sol or GPT-5.6 Luna because their names look similar. Confirm availability in the execution session. If unavailable, report the limitation and keep productive orchestrator work moving while obtaining the user's alternative.

The [official subagent guidance](https://learn.chatgpt.com/docs/agent-configuration/subagents) distinguishes demanding, ambiguous work from narrow tasks. The responsibility assignments below are this project's design choices, not claims that a model guarantees correctness.

Use high reasoning for Sol's grading semantics, recovery, and review; high for Luna's bounded tasks is a reasonable starting point. Respect actual supported settings and concurrency in that session. There is no need to maximize effort on every read or packaging edit.

Use the execution session's actual subagent API. In the currently exposed API, full-history forks inherit the parent's model and cannot take a model override; an explicit Luna spawn therefore needs a bounded-history or fresh-context fork plus a self-contained task and file references. Inspect the next session's schema rather than blindly copying a tool call from this document. Verify the returned model/task identity instead of assuming a model request succeeded. No new API key or separately billed embedded orchestration service is needed for building this CLI.

The orchestrator owns all cross-cutting decisions, schemas, integration, live-test scope, and completion claims. Sol subagents own high-risk implementation. Luna must not independently settle grade/rubric coupling, attempt identity, retry semantics, authorization, identity matching, or Canvas capability assumptions.

### 14.2 Shared-workspace discipline

At the start, inspect `git status`, repository instructions, available tools/models, and existing changes. Do not overwrite user work. Use the current suitable checkout unless isolation is needed; use a `codex/` branch if creating a branch. Use the app's managed-worktree facilities if a separate worktree is warranted, and carry over the correct planning baseline intentionally.

At present the repository is `dev` with the work-laptop report committed as `e49ffd9`; this brief and reconciled docs may be uncommitted until the user or execution session commits them. Read the actual files, not just that historical commit.

For a shared checkout, assign exclusive file ownership. An agent may propose changes outside its ownership but must hand them to the owner/orchestrator. Freeze interfaces before dependent agents code. One owner edits `go.mod`, common domain types, shared schemas, and workspace guides at a time. Do not parallelize agents whose only useful work would conflict on those files.

Use subagents of the implementation chat, not extra user-facing chats. This user's request explicitly authorizes Sol and Luna subagents for execution. No subagent should start a remote publish or mutate live Canvas unless its task includes the already established test scope. Normally the orchestrator alone performs live experiments to prevent interference.

### 14.3 Suggested work waves

1. **Orchestrator first:** reconcile instructions; establish build/test targets; freeze file and command contracts; identify evidence-dependent paths. Delegate read-only API/source investigation to Sol and narrow repository/fixture inventory to Luna if useful.
2. **Foundation wave:** one Sol owns Windows platform/setup and transport; another Sol can own validated workspace/domain logic after interfaces are frozen; Luna handles bounded renderer/template tasks against those contracts. Scale to available slots; the current planning session exposes four total slots including the root, but the next session is authoritative.
3. **Workflow wave:** one Sol owns create/edit/resume; another Sol owns pull/post/attempt/comment behavior. Orchestrator integrates and runs the live probes. Luna builds synthetic fixture cases, help text, and packaging inputs under exact specifications.
4. **Review wave:** a Sol who did not implement a high-risk path reviews that path against the decision tables and failure cases. Luna audits links, template contents, accidental private artifacts, and documented commands. The orchestrator resolves findings, runs focused regression checks, and owns final acceptance.

Do not wait for all agents to finish before reviewing shared assumptions. Stop or redirect a subagent promptly when it implements an old requirement or a speculative API behavior.

### 14.4 Every delegation must include

- Objective and explicit done condition.
- Exact model, owned files, permitted dependencies, and frozen interfaces.
- Relevant sections of this brief and which conclusions are proven versus hypotheses.
- Whether live Canvas calls are permitted and the precise disposable scope, if any.
- Required tests and the concrete failure cases the agent must cover.
- What it must return: changed files, behavior, commands/check results, unresolved assumptions, and integration notes.

Do not delegate "make grading work" to Luna. Appropriate Luna examples are formatting a synthetic packet from an already fixed schema, adding specified parser-invalid-input fixtures, implementing a small deterministic feedback renderer from examples, or inspecting a ZIP manifest. A Sol/orchestrator reviews the resulting code and tests.

## 15. Build order and exit criteria

### Slice A: executable, contracts, and Canvas facts

Deliver the Go skeleton, platform credential/form path, transport, schema/domain types, starter template skeleton, and evidence for entered scores and rubric/grade independence. Test executable execution on WorkLaptop early, before a large implementation accumulates.

Done when the program starts from the intended folder, setup authenticates privately, one scoped live read produces a compact result/file, file contracts are frozen, and the core grading strategy has actual Canvas evidence. A failed existing-coupled-rubric probe must have a documented supported path or an explicit limitation, not a guessed workaround.

### Slice B: complete authoring and editing

Deliver create/resume/pull-item/update for text-entry assignments, rubric attachment, module placement, requested publication, and the small readable renderer. Keep HTML editing lossless. Test duplicate titles, partial create, and a teacher live edit after pull.

Done when a generic disposable assignment can be created and edited from an agent-written file, returns useful links, survives a required-piece failure without duplication, and leaves requested unpublished content unpublished.

### Slice C: complete scoring and revision

Deliver explicit scoped pull, shared grading packet, history, alias/notes projection, results validation/aggregation, honest rubric posting, grade retention, feedback, late handling, comment revision, and action-level retry outcomes.

Done when actual sandbox submissions cover empty/equal/higher/lower existing grades, current versus strongest earlier attempt, missing/unreadable components, and a partial post. Teacher-only notes never appear in Canvas. Feedback-only edits change only the intended comment.

### Slice D: discovery, clean-session use, and release

Deliver current-course discovery, student editor polish, compact guides, complete help, synthetic clean-session evaluation, performance measurements, and a clean portable ZIP. Close any remaining required integration checks. Prove that unchanged handled work does not repeat and a new attempt is discovered.

Done when a clean agent can operate the workspace using its own short instructions, the actual executable passes Windows and Canvas checks, packaging contains only template content and distributable assets, and release notes truthfully separate verified engineering behavior from pending teacher acceptance/distribution checks.

Commit coherent implementation slices when appropriate to the execution task; do not require a commit after every agent edit. Do not push, create a release, or overwrite a teacher's populated workspace merely to declare the beta done. If creating a PR later under user authorization, use the app's attachment mechanism.

## 16. Acceptance and regression matrix

These are behavior requirements, not a demand for a large test framework. Use focused unit tests for arithmetic/parsing, a controlled HTTP fake for recoverable failures and redirects, Windows integration checks for platform code, and real Canvas probes for undocumented semantics. Avoid tests that merely assert the implementation's own chosen output without an independent expectation.

| Area | Required case and expected behavior |
|---|---|
| Generic interpretation | A clean agent identifies required components from ordinary directions and applies a reusable rubric without a special warning. |
| No three-item overfit | Separate synthetic two-component, four-component, and choose-one cases work; production guides contain no three-SCR special case. |
| Weighting judgment | Agent-chosen weights have a defensible assignment-based rationale, are consistent across students, and survive arithmetic validation. |
| Calculation | Different assignment/rubric maxima, fractional criterion points, and non-integer weights produce independently checked totals. |
| Attached-rubric scale | Creating a 20-point assignment with a 100-point rubric preserves both scales and posts the correctly normalized assessment. |
| Missing evidence | Omitted required answer is treated as missing work; unreadable content is identified as unknown instead of fabricated or silently zeroed. |
| Honest rubric | Existing grade 90/current assessment 72 saves an honest 72-equivalent rubric, retains 90 entered, and explains the distinction. |
| Equality | Existing grade equals assessment and blank/stale rubric is filled accurately. |
| Grade increase/empty | Empty grade or genuinely higher assessment writes the intended entered grade. Zero is not mistaken for empty. |
| Late grade | Comparison uses proven pre-penalty value; SE neither double-penalizes nor calls a post-penalty score the entered grade. |
| Prior attempt | With a strong first attempt and weaker second attempt, third-attempt feedback compares to the strongest preceding work. |
| Resubmission race | New attempt after pull holds only that student; the other students post. |
| Changed assignment/rubric | Relevant live change holds dependent scoring and preserves unsent work for reassessment. |
| Retry | Exact intended comment is not duplicated after a lost response; uncertain outcomes are not blindly replayed. |
| Completed replay | Replaying a finished result does not restore its old score or comment after the teacher changed Canvas. |
| Partial row | Comment/rubric/grade/late outcomes remain separate, and retry resumes only unresolved work. |
| Teacher comment | Routine retries/revisions do not replace a manually written or teacher-edited comment by guessing authorship. |
| Revision | Selected feedback text changes while grade, rubric, and late state remain unchanged. |
| Discovery | Completed lower/equal assessment does not recur; later new submission does recur; no local-only done flag hides work. |
| Identity | Renaming alias after pull still targets the same student; duplicate names do not cross-attach notes. |
| Invalid input | Bad schema, duplicate student, wrong criterion, impossible points, or malformed feedback file sends no writes. |
| Create recovery | Assignment success plus rubric/module failure stays unpublished and resumes against known IDs. |
| Ambiguous create | Two recent same-title candidates or uncertain empty lookup never trigger arbitrary adoption or blind creation. |
| Live edit | Teacher's post-pull edit is preserved; old draft cannot overwrite a completed live item. |
| Private data | Secret never enters output/files; private notes never enter feedback; ZIP is clean. |
| Downloads | Cross-host redirect receives no PAT; malicious attachment path cannot escape workspace. |
| Windows | Actual binary, credential encoding, loopback UI, paths with spaces, and normal user permissions work. |

For the clean-session inference evaluation, supply only the shipped workspace guides and ordinary assignment/rubric/work packets to the scoring agent. Do not supply this brief, the expected arithmetic, a hint that "this is the multiprompt test," or the fixture's answer key. Evaluators may know the expected properties. This distinction is necessary to test the inference the teacher is paying for.

Use synthetic examples for development and the disposable-course verification. Preserve the teacher's real three-SCR exercise for their later request. Do not demand one exact prose answer from the agent: judge evidence, arithmetic, consistency, appropriate comparison, and usable feedback.

## 17. Performance and simplicity limits

- Author/create: one CLI invocation. Live edit: pull-item and push. Score/post: pull-work and post, with ordinary agent file work between them.
- No extra teacher turns on an ordinary fully specified "score and post" request. A bare "scoring session" still pulls then asks as the teacher chose.
- No network calls merely to read already pulled files or print help.
- Read shared directions/rubric once per assignment; keep large student content in files rather than flooding stdout or agent context.
- Pull detailed history only for relevant assignments/students, while making sufficient prior work available for strongest-attempt comparison.
- Report elapsed time and request counts separately for discovery, fetch/download, and posting; agent reading/scoring time is separate. Measure one representative class-sized synthetic or authorized sandbox workload and record its size. Do not invent performance claims from the 200-file disk probe.
- Batch inexpensive live prechecks where supported, then use bounded writes. Optimize measured bottlenecks after correct semantics are demonstrated.

Do not add databases, mirrors, readiness/freshness gates, approval machinery, journals, backup systems, leases, locks, device handoff, generalized jobs, plugins, MCP schemas, or a generalized policy engine. Small per-document references, input validation, targeted conflict checks, and known-outcome recovery are sufficient for this beta.

A setup page is a small embedded form, not a frontend framework project. A rubric interpretation is a short agent decision saved in the result, not a new educational rule engine. A result ID is a pointer, not an operation ledger.

## 18. Final delivery requirements

The orchestrator must finish with:

1. Source and focused tests, pinned dependencies, and reproducible documented build steps.
2. The actual Windows `cse.exe` identifying version `0.1b` and its source revision/build information.
3. A clean starter workspace ZIP, short setup instructions, task guides, and examples containing only synthetic data.
4. A concise evidence report listing actual commands/probes, pass/fail/unverified outcomes, tested machine/build, and material limitations.
5. A short handoff explaining how to open the workspace in the agent, configure courses, run setup, and perform the authoring/scoring path.
6. Explicit status for the real shared-ZIP test and the teacher's later three-SCR acceptance exercise.

Do not mark a failed or unrun real Canvas check as passing because a mock passed. Do not declare "functional 0.1b" if core honest-rubric/higher-grade behavior has not been demonstrated. If access is unavailable, deliver the completed build and identify the exact remaining verification; do not fabricate completion or abandon unrelated implementation work.

## 19. Source references and evidence limits

Consulted 2026-10-08. These are primary documentation/source leads, not evidence of the district's configured behavior. Recheck the relevant endpoint when implementing it. Follow links only for the feature being built; do not consume CE's entire implementation up front.

- **S1:** [Canvas Rubrics API](https://developerdocs.instructure.com/services/canvas/resources/rubrics) — rubric associations and the separate grade-calculation setting; criterion/assessment shapes.
- **S2:** [Canvas Submissions API](https://developerdocs.instructure.com/services/canvas/resources/submissions) — scoped retrieval, history, per-submission and bulk updates. Their documented parameter shapes differ.
- **S3:** [Canvas Progress API](https://developerdocs.instructure.com/services/canvas/resources/progress) — completion of asynchronous operations.
- **S4:** [Canvas Submission Comments API](https://developerdocs.instructure.com/services/canvas/resources/submission_comments) — edit/delete endpoints; test actual access and attempt behavior.
- **S5:** [Canvas throttling](https://developerdocs.instructure.com/services/canvas/basics/file.throttling) — shared transport behavior must account for throttling.
- **S6:** [Go crypto/x509](https://pkg.go.dev/crypto/x509) — platform certificate verification; validate the chosen release/configuration on Windows.
- **S7:** [Go net/http](https://pkg.go.dev/net/http) — client, redirects, environment-proxy behavior, and request handling. Do not infer Windows PAC support from environment-proxy support.
- **S8:** [OpenAI subagent documentation](https://learn.chatgpt.com/docs/agent-configuration/subagents) — requested model identifiers and delegation guidance. Actual session tooling controls availability and slots.
- **S9:** [Go build documentation](https://pkg.go.dev/cmd/go#hdr-Compile_packages_and_dependencies) and [target/build environment](https://go.dev/doc/install/source#environment) — confirm the actual build environment rather than installing an end-user runtime.
- **S10:** [Canvas assignment API](https://developerdocs.instructure.com/services/canvas/resources/assignments) and [module API](https://developerdocs.instructure.com/services/canvas/resources/modules) — authoring and module-placement leads to verify during slice B.

Local evidence: [WorkLaptop report](WorkLaptopTestSuite/results/STAGCFKQ74-20261008-0640.md). Local domain-knowledge map: [CE-LESSONS.md](CE-LESSONS.md). The referenced old CE checkout is a historical path and may not exist on the next machine. Its absence must not block work supported by current primary documentation and disposable-course evidence.

## 20. Suggested opening message for the execution session

> Implement CanvasExpert-SE 0.1b from `EXECUTION-BRIEF-0.1b.md`. Read it fully, then read `ARCHITECTURE.md`, `CE-LESSONS.md`, and the WorkLaptop report. Use GPT-6.1 Sol as orchestrator with GPT-6.1 Sol and GPT-6 Luna subagents as specified. Preserve the teacher's settled decisions; make ordinary implementation choices autonomously. Inspect current repository changes before editing. Establish the build environment and disposable Canvas test target early, then execute the four slices through a working Windows build, clean starter ZIP, and truthful integration evidence. Keep the agent responsible for educational inference and the CLI responsible for validated mechanics. Do not create my real three-SCR assignment yet. Ask only for missing external inputs or a concrete decision the brief reserves for me, and continue independent work while waiting. Report verified, failed, and unverified behaviors separately.

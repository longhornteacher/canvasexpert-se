# Major retakes, Daily corrections and the honest-effort floor

Date: 2026-11-11. Status: **execution slices written; implementation has not started**. Baseline: Dev `992f4fa`, fetched and equal to `origin/dev` when this plan was written. This folder is an execution plan, not evidence that its features exist.

The developer requested these slices after a read-only audit of Dev, the active teacher instructions in UserSync and the connected Engine. Execute one bounded slice at a time. No subagents, release, installation, UserInstall/UserSync edits, real grading, publication or family sends are authorized by this planning request. Follow the existing development workflow when execution is requested; an ordinary authorized implementation slice does not require another approval ceremony.

## Agreed policy

These decisions come from the developer's conversation on 2026-11-11, including the follow-up answers. They supersede conflicting older instructions for this teacher.

| Topic | Decision |
|---|---|
| Major grades | No offers of immediate second submissions or credit for hand-copying rationales/exemplars. Rich, specific glows/grows, answer rationales and exemplars remain instructional supports. |
| Required companion | Every Major, whether multiple choice, written or mixed, gets a parallel **New Quiz**, even when the original is an ordinary written assignment. |
| Name | Exactly `TUTORIAL-ONLY RETAKE - <assignmentname>`, where the suffix is the original assignment's name. The prefix alone never establishes identity or authorizes a grade change. |
| Initial state | Created in the same course, unpublished, assigned to nobody and protected by an agent-chosen access code. Giving out the code alone does not publish or assign it. |
| Parallel design | Same concepts, response types, demands, points/rubric expectations and closely mirrored question/answer stems; different passage material. Adapt text-specific references and evidence so the new key is actually correct. |
| Preparation | Make the new passage(s) readable ahead of tutorials when practical, separately from the protected quiz. Release no questions, key, exact-answer exemplar or access code in that reading resource. |
| Major retake result | Assess honestly; at most 70% can be applied from the retake to the original Major. Retakes never lower the original grade. |
| Window | Four school days after the student's effective original due date, using the existing calendar and established accommodations. The preceding assistant stated this anchor and the developer answered the remaining floor question without changing it. |
| Honest effort | A 40% floor for **all** genuine efforts: Daily and Major, ordinary work and New Quizzes, including tutorial retakes. Missing, blank, placeholder or accidental work does not qualify. |
| Daily grades | Existing formative corrections remain: handwritten rationales earn points back, revision/resubmission is allowed and exemplars may be written for credit. Do not impose the Major retake cap or tutorial requirement on Dailies. No new Daily credit amount was specified. |
| Existing preferences | Preserve unrelated choices: aliases, feedback voice, no names in comments, exact two-sentence quiz rationales, answer-length constraints, Daily extra attempts never hurting, first-genuine-attempt lateness, parent-update content and CS8 rubric preferences. |

### Arithmetic and grade meaning

Keep the earned assessment, the policy-adjusted entered academic grade and Canvas's late-adjusted grade distinct. The existing floor is applied **before Canvas's late penalty**. This plan retains that established behavior; the developer changed the floor's amount and scope, not the course late policy. Never describe 40 as a guarantee about the final grade after late deductions. Do not charge a new late penalty just because a student attends a later tutorial.

For a completed retake, let `R` be its honest earned percentage, `H` its honest-effort judgment, `P` the original assignment's points, and `G` the original's current reliably known entered academic points:

```text
retake_entered_percent = max(R, 40) if H, otherwise R
candidate_percent      = min(retake_entered_percent, 70)
candidate_points       = P * candidate_percent / 100
target_entered_points  = max(G, candidate_points)
```

Use rational/decimal arithmetic and round the final point value once to the Engine's posting precision. Do not round a percentage first. If the original has no grade, the candidate can become its first entered grade only after exact student/attempt matching and ordinary genuine-work checks; an unknown grade is not an empty grade. Excused, hidden, changed or unavailable targets hold. The 70 cap applies to the **retake contribution**, not to a pre-existing original grade above 70. Preserve the original's first-genuine-attempt timing and existing late-policy treatment.

The companion's own items and earned score remain honest. An 88% retake remains an 88% assessment even though the original receives at most 70%. A floor is an explicit grade adjustment, never invented item points or inflated rubric criteria. Record a companion so it does not create a second summative grade or SIS export; slice 03 defines that mechanical requirement.

## Audit baseline and concrete gaps

- Connected UserInstall: `C:\Users\adamb\CanvasExpert-SE`, ZIP install, Engine/adapter 1.9.0 (`ffa368555fa4`, built 2026-11-06). Dev `VERSION` is 2.1.0. Do not assume the connected tools run Dev code.
- Active UserSync: `C:\Users\adamb\OneDrive - Pearland ISD\CanvasExpert-SE-Sync`. Its active TEACHER/playbooks predate parts of Dev. ZIP authority differs from a git UserInstall; use the detected install kind at adoption.
- `Teacher_Override.md` permits extended-writing second attempts but prohibits short-response/auto-scored second submissions. It also allows major resubmissions outside school. Replace those contradictions, preserving the other preferences.
- `context/grading.md` requests 2-3 sentence feedback and has `correction_school_days: 4`, `close_after_school_days: 11`, `extra_time_school_days: 1`. Rich feedback supersedes the sentence limit, not the warm/plain voice or existing dates.
- UserSync course configuration has a 50% effort floor in ELA7 (120638) and PAP ELA7 (120669), no floor in CS8 (121046), and inactive Collab (103833). The new rule is 40 in all courses used for this teacher's student grading; Collab is not a student-grading target.
- QuizForge already creates MC/written/mixed New Quizzes, passages, access codes and unpublished quizzes. It does not expose an unassigned create setting or a trusted original/retake relationship.
- `AssignmentDatesDocument` can set `only_visible_to_overrides: true` with zero override rows after creation; an existing local test exercises that payload. Assignment creation instead rejects an empty restricted audience. Do not claim that creating unpublished is the same as assigning nobody.
- `finishQuiz` can apply saved accommodations before publication. It needs correct behavior for an intentionally empty audience and later tutorial recipients.
- Timetables advertise `Last day for corrections` without a Major/Daily distinction. Their refresh currently watches date changes, so a category/policy change with unchanged dates is another real integration point.
- `render.partText` emits `Hand-copy exemplar`; component exemplar fields display only below 90%. Item comments use a neutral label but the same threshold. Rich feedback and model delivery must work above that threshold too.
- Ordinary individual grades and individually graded group rows already use `domain.EnteredGrade`; shared group grades and New Quizzes do not. New Quiz `PostScores` deliberately preserves existing fudge points. Do not turn that preservation into permission to replace a teacher's adjustment blindly.
- Session discovery skips `needs_grading_count == 0`; an auto-scored quiz can therefore miss a floor review. New Quiz pulls currently save a result-version baseline only when written responses/uploads exist.
- No existing supported operation links a companion result to the original grade. Ordinary results cannot be repurposed by falsifying original evidence or rubric scores.

## Post-planning installation update (2026-11-11)

At the developer's explicit request, Dev was fetched/pulled (already current), and the existing ZIP UserInstall was updated from published release 1.9.0 to the latest published **2.1.0**, revision `2e3349e01d6b`, built `2026-11-11T00:57:17Z`. The release ZIP matched its GitHub asset SHA-256, and the Engine/extension matched the distribution manifest. The supported playbook update adopted seed `2.1.0-19ce1190` in active UserSync with no `kept` conflicts. `Teacher_Override.md`, `context/grading.md` and `context/courses.md` retained their pre-update hashes; the UserSync binding was preserved. Replaced product files were backed up under UserInstall `engine/pre-update-20261111-072617-2511d935/`.

The adapter already loaded in this chat remains 1.9.0 until the app restarts; `engine_info` separately reported the new on-disk child Engine as 2.1.0. This installs the existing release only: **none of the new assessment-policy slices is implemented or adopted**, and the old 50/50/unset floor settings remain pending slice 08. The earlier audit facts above remain a dated baseline; execution should use the updated release/playbooks and rediscover current state.

## Slice sequence

| Slice | Deliverable | Dependencies |
|---|---|---|
| [01](01-playbooks-and-policy.md) | One coherent Major/Daily policy, authoring/scoring/review instructions and exemplar behavior | This brief |
| [02](02-contracts-and-exact-math.md) | Narrow typed contracts, identity baselines and exact arithmetic | 01 policy decisions; may begin before prose is finished |
| [03](03-retake-authoring-and-access.md) | Same-course companion creation, no audience, access code, non-counting gradebook state and resumable tutorial release | 02 |
| [04](04-reading-deadlines-and-timetables.md) | Passage-only preparation, individual four-school-day window and accurate original/retake timetable language | 02; 03 for release integration |
| [05](05-honest-effort-floor.md) | 40% floor through ordinary/group/quiz paths, including auto-scored-quiz discovery | 02; uses 04's timing boundaries |
| [06](06-retake-grade-application.md) | Honest retake assessment and capped, never-lowering original grade application | 02, 03, 05; 04 for eligibility |
| [07](07-integration-and-evidence.md) | End-to-end local checks, adapters, complete capability accounting and bounded live observations | 01-06 |
| [08](08-usersync-and-rollout.md) | Reviewable adoption procedure, reconciled teacher overrides/config and release/install evidence | 07; deployment only within the user's later requested scope |

All steps must be reachable through the ordinary CLI/MCP surfaces. Prefer extending `push_quiz`, `pull_work`, `post_results`, `push_document`, `validate_document` and Engine reference help over introducing tools for every substep. Explicit schemas proposed in the slices are implementation contracts; names can be refined against the current tree, but do not weaken their semantics or leave conflicting names in different adapters.

## Execution discipline

Read root `AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/SCORING-RULES.md` and the selected slice. Fetch and compare Dev before edits; preserve unrelated work. Read `docs/team/README.md` for routed/owned-course writes, and `docs/group-work/README.md`, `docs/group-work/contracts.md` and the relevant grading slice before touching group paths. Shared group assessment must never become an accidental whole-group retake award.

Use existing packages and outcome records. Do not build a policy engine, generic operation registry, background retake watcher, second gradebook, cross-PC session ledger or migration framework. The agent chooses concepts, replacement texts, stems, models and honest effort. The Engine owns identity, exact math, dates, audience controls and safe writes.

Do not mass-create retakes for existing Canvas work, change existing student grades, rewrite saved result files, impose a new Daily correction credit amount or resend family messages merely to complete implementation. New Major authoring must produce the companion; when existing Major work is explicitly handled, resolve/create the missing companion within that teacher task's scope. Historical student work and accepted outcomes remain intact.

Each completed implementation slice records changed files, focused checks and remaining evidence here or in its execution record. Keep the two capability records (`internal/localui/capabilities.go` and `docs/engine-capabilities.md`) synchronized with dated **local**, **documented**, **Canvas Live** and **pending** evidence. Pending student view does not prohibit ordinary supported use; unknown write semantics hold only that specific numeric operation.

## Verification baseline

During the initial audit, existing focused tests passed for QuizForge effective settings, quiz creation/resumption, written items, real stimuli/resumption, zero-recipient assignment visibility, timetables and exemplar rendering. No new test or Canvas Live check was run to establish this policy's unimplemented behavior. Planning validation is `git diff --check`, local-link checks and a contract consistency read; no product build is needed for writing these slices.

For an implemented slice, follow `handoffs/README.md`: focused tests during work, then `go test ./...`, `go vet ./...`, temporary Dev build, `node scripts/check-mcp.mjs <temporary-engine>` and `git diff --check` once at completion. Do not repeat unchanged suites just to consume time, and do not add tests that merely search prose.

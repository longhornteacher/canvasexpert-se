# Slice 08 - Teacher instructions, UserSync and rollout

Status: **not started**. Kind: adoption procedure after implementation. Depends on [07](07-integration-and-evidence.md). This file describes the needed work; the current request to write slices does **not** execute it or authorize release/install/teacher-data mutation. When the user requests rollout, follow that scope and the normal existing update flow without inventing another approval token.

## Result

The Engine used by the teacher and the instructions the agent actually reads both implement the same policy. Merely editing Dev seeds is insufficient: this teacher currently uses a ZIP install with active instructions/overrides in UserSync, and older custom files are preserved by updates.

## Baseline observed 2026-11-11

| Item | Observed value |
|---|---|
| Dev | `C:\Development Projects\canvasexpert-se`, `dev`, `992f4fa`, version file 2.1.0 |
| UserInstall | `C:\Users\adamb\CanvasExpert-SE` |
| UserSync | `C:\Users\adamb\OneDrive - Pearland ISD\CanvasExpert-SE-Sync` |
| Connected Engine/adapter | 1.9.0, revision `ffa368555fa4`, built 2026-11-06, install kind `zip` |
| ELA7 | Course 120638, current; `effort_floor: 50` |
| PAP ELA7 | Course 120669, current; `effort_floor: 50` |
| CS8 | Course 121046, current; no explicit floor |
| Collab | Course 103833, not current; no student-grade mutation requested |
| Grading policy | Four correction school days, eleven-day close, one general extra school day; feedback limited to 2-3 sentences |

These are navigation facts, not permanent paths/IDs to hardcode into product code. Rediscover current Engine version/install kind, active authority and saved nicknames before adoption. Another PC may run a different install or a newer git-based release.

**Later on 2026-11-11:** the developer separately requested installation of the latest existing release. UserInstall was updated to published **2.1.0** (`2e3349e01d6b`, manifest checksums matched), and active UserSync playbooks adopted seed `2.1.0-19ce1190` without kept-file conflicts. The three teacher preference/config files above retained their hashes and the sync binding stayed unchanged. The loaded adapter was still 1.9.0 pending an app restart. See the brief's installation record. This is not adoption of the unimplemented new grading policy; this slice remains not started.

## A. Prepare a concrete adoption set

1. Confirm slices 01-07 are implemented and record which capabilities retain a manual fallback. Do not distribute prose that claims an unsupported numeric path is automatic.
2. Inspect current active `Teacher_Override.md`, `TEACHER.md`, playbooks, `context/grading.md` and `context/courses.md` through the Engine's authority-aware read tools. Capture their current hashes. Do not use the November 11 hashes to overwrite later teacher work.
3. Prepare narrow exact edits preserving existing text that remains valid. If an intervening change is detected, re-read and reconcile; do not replace the file from a stale snapshot.
4. Identify which install-owned files can be updated normally. In a git UserInstall, product playbooks/TEACHER/root agent files are git-owned and stay local; put this teacher's choices in `Teacher_Override.md`/`context`. In ZIP mode, reconcile active UserSync playbooks with current seeds through supported update/replace semantics, preserving custom preferences.
5. Keep the source of the new choices explicit: teacher conversation 2026-11-11, all Daily/Major genuine efforts, separate tutorial New Quiz, 70 contribution cap, higher original retained and four school days. Do not save a one-task choice as a universal preference.

## B. Teacher override reconciliation

Replace the old response-type block rather than appending a contradictory rule farther down the file. Required current meaning:

- Grade category governs recovery: Major versus Daily, not MC/SCR/ECR or sentence count.
- Every Major has a same-course parallel New Quiz named exactly with the required prefix; initial unpublished/no-audience/code-protected state.
- Major rationales/exemplars/glow-grow feedback prepare for tutorials; handwriting them or immediately reuploading the original does not earn Major recovery credit.
- Major retake contribution is capped at 70%; original higher grade prevails; raw earned score remains honest.
- Dailies retain rationale/exemplar writing for credit and feedback-based resubmission. Preserve existing Daily extra-attempt never-hurt behavior.
- Honest-effort minimum is 40% for every Daily/Major genuine effort, including New Quizzes and tutorial retakes; no automatic minimum for blanks/placeholders/non-attempts.
- Four-school-day deadline follows the effective original due date and established accommodations. Keep original first-genuine-attempt timing separate from the tutorial date.
- Remove the broad `Major extra attempts may be done outside school` permission insofar as it conflicts with tutorial-only retakes. Preserve the original first-attempt in-school rule and any unrelated genuine Daily permission.

Preserve parent-update threshold/content/sign-off rules, quiz answer-quality/length constraints, exactly two-sentence rationales, alias/privacy requirements and book-club file preferences. Do not rewrite unrelated source project drafts or historical student feedback to match the new rule.

## C. Grading context and course settings

1. Replace the strict 2-3 sentence feedback limit with plain, warm, sufficiently rich, specific glows/grows and actionable next steps. Keep the preferred score-first layout and prohibition on student/classmate names. Two-sentence item rationales remain a separate requirement.
2. Keep `correction_school_days: 4`, `close_after_school_days: 11` and `extra_time_school_days: 1` unless a newer teacher instruction changed them. Clarify in prose that Dailies use formative corrections, while Majors use the separate tutorial window. Do not change Canvas course late deductions or their minimum-percent setting.
3. In this teacher's current student-grading courses, change ELA7 and PAP ELA7 floors from 50 to 40 and set CS8 to 40. Remove/replace the stale explanatory prose saying the ELA floor is 50. Preserve nicknames, IDs, instance, aliases, current selections, owners, timezones and late-work settings exactly.
4. Do not activate Collab or adjust its gradebook. Save the standing rule that newly selected student-grading courses inherit this teacher's 40% choice through ordinary setup; the Engine still uses the existing explicit course field. Do not implement an installation-wide hidden default or a migration of other teachers.
5. Do not sweep existing grades down from 50 to 40. This is a floor-policy setting for subsequent authorized scoring/review, not permission to lower historical grades. Existing higher scores and teacher edits remain authoritative.
6. Do not promise that changing configuration alone adjusts already auto-scored quiz grades. The implemented policy-candidate review must actually run under an authorized scoring request, with evidence-backed effort judgments.

## D. Ship/update only within requested deployment scope

Use `scripts/package-userinstall.ps1`, the Dev -> userinstall distribution flow and `docs/releasing.md` when a release is requested. Do not distribute a Dev test binary as a release or create a second teacher product folder. Follow the ZIP/git detected install's normal update flow; keep credentials/runtime data local and reusable rules in their active authority.

Preserve active review pages, unsent drafts, grading/planning sessions, accepted receipts and private roster/contact data. If an older MCP adapter remains loaded after updating the Engine, use the documented restart/reconnect notice; do not claim the session is running the new build merely because the executable was replaced.

For a ZIP update, account explicitly for active customized playbooks the updater kept. Reconcile the new Major/Daily instructions there; `shipped/<version>` is a reference, not active guidance. For a git update, use the release-owned playbooks and preserve teacher choices in overrides/context without making git-owned local edits. No provider-side sync job, duplicated UserInstall in OneDrive or cross-PC session transfer is added.

## E. Adoption verification

Read back the active version/install-kind/authority metadata and the edited nonprivate instructions/config through the actual teacher route. Confirm:

- Engine and loaded adapter are compatible and expose the required typed fields/kinds.
- The exact active `Teacher_Override.md` and grading context contain one coherent policy, not competing response-length rules.
- ELA7, PAP ELA7 and CS8 show the teacher's current 40 floor without changes to unrelated course configuration.
- A temporary local synthetic draft validates under the connected Engine contract; no Canvas create or grade write is needed just to verify installation.
- Active playbooks explain true automatic support and any specific manual fallback correctly.
- The teacher's pre-existing unsent work still exists. Do not read private identity/contact stores to prove this.

Reading changed shared files on this PC proves active authority and local adoption, not that OneDrive delivered them to another PC or another client read them. Record provider delivery/client adoption only when actually observed. No full-history grading sweep, mass companion backfill, family message or SIS change is part of adoption.

## Completion report

State the adopted Engine/adapter versions, install kind, active instruction authority, policy/config edits, checks actually run and unresolved behavior by capability. Report any necessary reconnect separately. Link the source implementation/evidence and preserve dated limitations. Only mark this slice complete after the requested adoption actually occurred; a prepared procedure is not a rollout.

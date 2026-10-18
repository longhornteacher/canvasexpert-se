> **Retired 2026-10-18 — implemented and locally verified in Dev.** Both gaps below are closed. The implementation record at the end states the checks; remaining distribution, installed-playbook adoption and authorized real-use observations are tracked in the [delivery follow-up](../active/CALENDAR-SCORING-DELIVERY.md). No new Canvas Live result is claimed.

# Close the parent-date and extra-time gaps

Date: 2026-10-18. Status: completed in Dev; the original implementation brief follows. Follow-up to `aae89b1` and the [calendar/scoring contract](SCHOOL-CALENDAR-SCORING-AND-PARENT-UPDATES.md). This brief does not authorize Canvas Live writes or changes to UserInstall/UserSync.

## Outcome

Keep explicitly undated student work out of parent reports. Give the scoring agent the saved Extra time flag and clear instructions for applying an established allowance through the existing calendar-backed timing path. Keep policy decisions with the teacher and agent; the Engine supplies facts, exact elapsed seconds and guarded writes.

## 1. Preserve an explicit student no-date response

**Confirmed defect:** `Submission.UnmarshalJSON` records `CachedDueDatePresent`, and `completeStudentSubmissions` recognizes `cached_due_date: null`. However, `effectiveDueForStudent` checks only the pointer and then falls back to the assignment deadline. The retained row can therefore become a false Missing/0 item.

Fix the resolver in `internal/workflow/parent_report.go`:

- Explicit null or an explicitly empty student date means undated. Return no due date; do not consult the assignment's dates or overrides.
- A valid student date takes precedence. A malformed returned date remains unavailable, with the existing item-specific explanation; do not replace it with the base deadline.
- Use the existing catalog/override fallback only when the student date field was omitted. Preserve existing in-memory fixtures with a populated date pointer.

Reproduce through completion and classification, rather than testing only the helper: a published online assignment has a general deadline inside the report window; the student's visible submission returns `cached_due_date: null` and `missing: true`. Expected: no Work due, Missing/0, Corrections, History or Due Soon row for that assignment. The temporary review regression currently produces a missing row. Retain the existing term/window and effective-date tests.

Do not change the reporting rule: retrospective due dates must fall between `max(period start, as_of minus 14 calendar days)` and `as_of`, inclusive. Upcoming work remains separate and clipped at period end.

## 2. Carry Extra time into the scoring decision

**Confirmed gap:** `workspace.Student.ExtraTime` is saved by the Students page, but ordinary scoring packets omit it. The group member evidence and its conversion to student timing evidence also need the same fact. The current preparation tool already accepts an absolute `accommodated_deadline` and private `accommodation_basis`; reuse those fields.

Add an optional, private `extra_time` fact to student-work and supported group-member evidence. New pulls emit the saved true/false value, resolved through the existing stable student mapping and active UserInstall/UserSync authority. Carry it through `groupMemberAsStudentWork`. Old packets remain readable; an absent field is unknown, not proof that the student has no accommodation. Refresh that student's evidence when needed for a timing decision. Do not expose private roster rows or ask the agent to read `.cse/`.

Update the scoring playbook and the relevant group guidance so the agent:

1. Reads the flag together with established rules in `context/grading.md`, student-specific accommodations in `context/students.md`, and assignment directions. A documented individual rule takes precedence over a general flag policy. A false flag does not cancel a documented accommodation.
2. Resolves the applicable rule to an absolute deadline using the school calendar, checking whether Canvas's individualized effective deadline already includes the allowance. Apply an additional allowance only once.
3. Sends that deadline and its private basis through the existing batch `prepare_late_timing` call. Keep Canvas's effective due date as the write guard. The Engine continues counting to the selected first genuine submission and Canvas calculates point deductions.
4. If the flag is set but no usable allowance is documented, holds only that student's timing correction, explains the missing rule, and continues valid academic scoring. Ask for the rule once when needed and save a teacher-provided standing rule through the existing context workflow.

**Policy boundary:** this conversation establishes no duration or multiplier for the flag. Do not invent one school day, double time, a waiver, or a mapping from arbitrary Canvas tag names. Implement the evidence flow and unresolved-rule behavior without waiting for a new policy. The teacher can establish the allowance in plain language; no new settings form or policy schema is needed.

For group products, retain the existing shared-versus-individual timing contract. An individual allowance must not change another student's timing. A shared timing write still needs one valid decision for the entire audience; differing deadlines hold that timing action while valid grading continues. No new group feature is in scope.

## Likely files

- Parent fix and regression: `internal/workflow/parent_report.go`, `parent_report_test.go`.
- Flag projection: `internal/domain/types.go`, `internal/domain/group_evidence.go`, `internal/workflow/pull_work.go`, `group_pull.go`, and the conversion in `late_prepare.go`.
- Agent instructions/contracts: `workspace-template/playbooks/score.md`, relevant group guidance, `workspace-template/context/grading.md`/`students.md` guidance where useful, and `engine-docs/formats.md`. Do not seed an invented allowance.
- Record the actual outcome in `CHANGELOG.md`, this handoff, `internal/localui/capabilities.go` and `docs/engine-capabilities.md` together with the review date. Update the Students-page tooltip that currently calls the checkbox a flag for later use.

Read the current group README, shared contracts and relevant submission/assessment slices before changing group evidence. Keep existing CLI/MCP invocation and saved results compatible; no new tool is required.

## Acceptance and verification

- The explicit-null parent regression passes through the real completion/classification path, while omitted-date fallback and individualized dates still work.
- A pull from a temporary student store includes the correct Extra time fact for the matching student, including the existing group projection. Private flags/bases stay out of student comments and parent output.
- A synthetic documented allowance reaches preparation as an accommodated deadline, produces the expected calendar-based seconds, and preserves the original Canvas due-date guard. Include a case where Canvas has already supplied the extended deadline so it is not extended twice.
- The playbook explicitly covers a flagged student with no documented allowance: timing held, academic work continues. Use instruction review for this judgment rule; do not add tests that merely search playbook text.
- Reuse the existing weekend, holiday/range, first-genuine-attempt and timing-only write tests. Run focused changed-package tests with the repo-local Go toolchain; broaden only for an integration concern.

Completion means the two changes, proportionate regressions, updated instructions and honest capability notes are ready for review. Packaging, teacher-context adoption and Canvas Live observation are separate steps, not evidence supplied by local tests. Preserve unsent work, existing posting scope, grade arithmetic and accepted-write completion behavior.

## Implementation record (2026-10-18)

- Fixed the explicit-null resolver path. Completion/classification regressions cover bulk and targeted null responses, empty and malformed dates, omitted-date fallback and individualized dates.
- Added private, presence-aware Extra time facts to individual/group packets and the timing projection, including known group members without a returned submission. Temporary shared-authority fixtures verify stable identity and true/false/old-absent behavior. Preparation regressions apply a synthetic documented allowance once across weekends/holidays while retaining the Canvas due-date guard.
- Updated scoring/group guidance, context guidance, formats, Students tooltip, CHANGELOG and both capability records. The Engine invents no allowance; unresolved-rule timing holds remain agent instructions. Existing teacher files and unrelated uncommitted work were preserved.
- Passed with the repo-local Go 1.27.1 toolchain: focused new regressions, `go test ./...`, `go vet ./...`, and a final `go test ./internal/localui -count=1` after the tooltip/capability changes. A fresh temporary CLI build passed `node scripts/check-mcp.mjs <temporary-cse.exe>` with 38 tools and no Canvas writes; the scratch binary was removed. `git diff --check` passed.
- No packaging, published release, UserInstall/UserSync adoption, Canvas Live correction, parent pull or family delivery was performed by this follow-up. Remaining delivery work is consolidated in the active handoff linked above.

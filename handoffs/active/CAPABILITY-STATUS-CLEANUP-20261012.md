# Promote two verified Engine capabilities

Agreed with the teacher on 2026-10-12. This handoff is limited to the two status moves below. MCP review/client validation is deferred; do not include it in this task.

Read Source AGENTS.md, ARCHITECTURE.md and SCORING-RULES.md. This is Source documentation/status work, not an Install update or Canvas task.

## Required changes

Move these entries from yellow to green in `internal/localui/capabilities.go`, and record the classification changes together in `docs/engine-capabilities.md`. Set the review date to the execution date. Preserve the dated evidence and its scope.

1. **Attach a rubric to a New Quiz (ECR/SCR).** The 2026-10-10 Engine `pull-item`/`push` run attached the exact independent rubric to CS8 New Quiz 3698192 while preserving points and settings. That is live evidence for the named capability. Rubric assessment writing is a separate operation and is not a prerequisite for attachment. Replace the stale suggestion that New Quiz rubric-assessment writes await student work with the current contract: written-work scoring uses the rubric as a comment-only scoring basis and writes no rubric assessment or quiz/item grade.
2. **Read Canvas data no command covers (cse api).** The 2026-10-10 Engine reads already verified course metadata, modules/items, assignments, files and quiz data. The operation is a read-only GET, one page per call. Preserve those limits. Full archival, general binary downloads, vendor access and write passthrough are outside this capability and do not hold its status yellow. Preserve the teacher-directed Canvas-write fallback.

## Boundaries and completion

No Engine behavior changes, Canvas calls, Install/client changes or new live trial are needed. Do not promote broader scoring, archival, student-rendering or MCP workflows. No new tests are needed for moving static status entries; review the diff for the two moves, accurate wording, matching documentation and date, and run `git diff --check`.

Complete when both entries are green and the two capability records agree, without implying any new live evidence.

## Completed 2026-10-12

Both entries are now in the green group in `internal/localui/capabilities.go`; the review date remains the execution date, 2026-10-12. `docs/engine-capabilities.md` records both classification changes against the existing 2026-10-10 Canvas Live evidence. The New Quiz attachment wording now states the current comment-only scoring contract; API wording retains the read-only, one-page boundary and teacher-directed Canvas-write fallback.

Validation: reviewed the focused diff for exactly these two promotions, matching evidence/limits and date; `git diff --check` passed. No new tests, Canvas calls, Install changes or MCP validation were performed. Engine behavior and broader workflow statuses are unchanged.

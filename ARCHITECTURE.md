# CanvasExpert-SE architecture

Updated 2026-10-09. Current development direction is here and in [SCORING-RULES.md](SCORING-RULES.md). The [retired 0.1b brief](handoffs/retired/EXECUTION-BRIEF-0.1b.md) and [historical evidence](docs/0.1b-evidence.md) describe history, not implementation prerequisites.

## Intent and engineering default

SE is a personal teacher tool shared with friends. The host agent supplies intent, educational judgment, and content; a portable Go CLI and plain workspace files execute tasks against Canvas Live. Optimize for successful work with little friction, speed, and practical accuracy. Ordinary repairable imperfections are acceptable.

Prefer direct implementations using existing packages. Add a defensive branch or abstraction when a supported workflow needs it, a real failure demonstrates its value, or a cheap check prevents a consequential mistake. Hypothetical edge cases alone do not justify expansion. Validate at useful boundaries, continue independent items when one fails, and explain uncertainty honestly without turning unverified assumptions into prohibitions.

Test changed behavior in proportion to its consequences. Reuse tests and add focused regressions for meaningful behavior. Cosmetic changes and straightforward reversible rearrangements do not automatically need tests. Do not create coverage quotas, branch matrices, source-text architecture tests, or incidental snapshots. Finish after relevant checks pass; broaden only for a concrete concern.

Preserve useful content and teacher intent rather than byte equality of unrelated state. A renamed assignment does not invalidate a score; dates affect lateness; a new attempt needs assessment; an unreadable optional attachment is reported without inventing contents.

## Spine and operation boundaries

```text
Host agent -> CLI / plain work files -> feature operation -> shared spine -> Canvas Live
                                      local UI ----------^
```

| Package | Responsibility |
|---|---|
| `internal/canvas` | Origin, authenticated HTTP, pagination, ordinary API errors and downloads |
| `internal/workspace` | Paths, configuration, file I/O and stable student identity |
| `internal/platform` | Per-machine credentials, portable startup and Windows integration |
| `internal/workflow` | Feature operations and their relevant preconditions, outcomes and recovery; Service supplies shared dependencies |
| `internal/domain`, `internal/render` | Feature contracts, rubric arithmetic, feedback and content rendering |
| `cmd/cse`, `internal/localui` | CLI entry points and temporary setup/student/settings UI |

A normal Go function/package is a module. A feature usually adds an operation, command entry point and short guide. Shared utilities emerge from actual use. Assignment editing shares Canvas transport and workspace I/O but owns its field patch and conflict checks. Feedback revision uses those same mechanics but owns selection and recovery of an exact comment ID. Grading semantics belong with grading, even when several grading functions share them.

Keep the existing layout and useful transport, parsing, identity and arithmetic code. Do not introduce a plugin loader, registry, event bus, job engine, persistence layer, universal policy schema, arbitrary-HTTP agent interface or new host protocol. Large files alone do not require splitting.

## Teacher decisions and working behavior

- Teacher instructions authorize the requested scope: score and post posts, draft does not, show me first stops for review. No mandatory preview/apply tokens. Scoring session alone pulls current-course work and asks what to do; it never implies posting.
- Canvas Live governs existing content. Pull before editing; send only requested changes. Preserve unsent edits when replaced content conflicts, return a fresh version, and let the agent reapply. Read-before-write checks reduce conflicts but cannot eliminate a concurrent edit race.
- Always post SE's honest computed assessment. Never use an existing higher entered grade to alter or suppress it; the teacher handles later adjustments. Feedback and rubric report that academic assessment. The agent compares with the strongest earlier academic attempt from actual work, not a teacher-adjusted grade. See scoring rules for arithmetic and posting behavior.
- Use the official Canvas rubric when present, and a common-sense local scoring basis otherwise. A missing Canvas rubric is not a blocker. New SE rubrics have automatic grading disabled; existing coupling is not silently changed. Post the honest rubric and computed grade normally, confirm returned outcomes, and report actual failures or uncertainty without claiming guarantees. No workspace certification is needed.
- Canvas alone calculates late point penalties. SE adjusts lateness timing only: exclude weekends and school holidays, and later drafts inherit the first genuine submission's lateness. The host selects `grade.late_timing.first_attempt` from actual work, with the student's applicable cached due date and a genuine-work reason; the selected attempt may be current or earlier and need not be attempt 1. The CLI computes elapsed school-date seconds from `settings/school-calendar.md`, then supplies only Canvas lateness status and seconds override. Missing evidence or calendar coverage holds timing only.
- Feedback begins with the computed assessment, then compact per-response criterion scores, then conversational glows/grows. Full criterion definitions stay in the rubric; settings/grading.md can shape voice.
- The agent infers response units, rubric application and weights consistently; no hardcoded three-response or equal-weight recipe. A deliberately absent Canvas rubric may use an explicit local scoring basis.
- Completed action records refer to the course/student/attempt, not permanent exact output. Teacher feedback/rubric edits do not automatically enqueue completed attempts. New attempts remain discoverable; explicit scoped pulls permit reassessment. Uncertain creates require bounded identity lookup, known IDs when available, and an explanation if ambiguous; never blind replay.
- Newly created assignments stay unpublished until requested rubric/module pieces succeed. Save returned IDs for resumption. "Pending my finalization" means post directly to Canvas, then the teacher reviews and may leave it unchanged. There is no pending state or approval polling. Posting policy and SIS sync are outside scope.

## Workspace and platform

Windows 11, standard user, no installer/admin/runtime requirement for teachers. Go builds a portable executable; platform/distribution checks are ordinary release work, not workspace certifications. Program and synced district workspace are separate. A per-machine program link backs workspace cse.cmd; the embedded starter template adds missing files without overwriting work. Temporary loopback UI exists only while its command runs. No resident service or database.

PAT and Canvas origin live in per-machine Windows Credential Manager. Never expose the PAT in files, logs, output or agent context. Private student mapping stays in .cse/students.csv and out of ordinary agent reads. Stable student references target persisted work; aliases decorate filenames/metadata and remain editable. Real names are allowed in student writing and teacher notes; do not scrub writing. Expand intentional name placeholders only, and keep teacher-only notes out of student feedback.

Workspace settings map course nicknames (CS8, ELA7PAP, ELA7), grading preferences and student notes. Plain files support authoring/resumption; pulled data is disposable, while unsent work and ordinary returned IDs/outcomes are preserved. One teacher uses one machine at a time; no locks, sync merge system, backup ledger or permanent gradebook.

## Current scope and future capabilities

Current operations cover course/module reads, assignment create/edit and rubric attachment, text submission/history pulls with attachments, component scoring, no-rubric scoring, assessment posting, feedback posting/revision, discovery, and local setup/student/settings pages. Attachments may be unreadable; report that honestly. Normal calls use current API responses and narrow action checks, not disposable-course proof in each workspace.

New Quizzes are for auto-scored items; written responses use ordinary Assignments. New Quiz authoring, differentiation/tag targeting, standalone pages/rubrics, Classic Quizzes and curves are future capabilities, not parity requirements. Future differentiation uses teacher-managed Silver/Red/Blue tags in ELA7 only (Support/Core/Accelerate), with Hub by default or Bridge when appropriate and hints for everyone. Preserve those educational choices without building them to demonstrate modularity.

Historical platform and district observations remain in the evidence document and must not be presented as proof of the current build. Live experiments require a concrete authorized scope; saved credentials or historical IDs are not authorization.

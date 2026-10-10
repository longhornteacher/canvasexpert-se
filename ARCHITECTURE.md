# CanvasExpert-SE architecture

Updated 2026-10-09 (Engine/Playbook naming; lateness is agent-supplied). Current development direction is here and in [SCORING-RULES.md](SCORING-RULES.md). The [retired 0.1b brief](handoffs/retired/EXECUTION-BRIEF-0.1b.md) and [historical evidence](docs/0.1b-evidence.md) describe history, not implementation prerequisites.

## Intent and engineering default

CE-SE is a personal teacher tool shared with friends. The agent supplies intent, educational judgment, and content; the Engine (a portable Go CLI) and plain workspace files execute tasks against Canvas Live. Optimize for successful work with little friction, speed, and practical accuracy. Ordinary repairable imperfections are acceptable.

Prefer direct implementations using existing packages. Add a defensive branch or abstraction when a supported workflow needs it, a real failure demonstrates its value, or a cheap check prevents a consequential mistake. Hypothetical edge cases alone do not justify expansion. Validate at useful boundaries, continue independent items when one fails, and explain uncertainty honestly without turning unverified assumptions into prohibitions.

Test changed behavior in proportion to its consequences. Reuse tests and add focused regressions for meaningful behavior. Cosmetic changes and straightforward reversible rearrangements do not automatically need tests. Do not create coverage quotas, branch matrices, source-text architecture tests, or incidental snapshots. Finish after relevant checks pass; broaden only for a concrete concern.

Preserve useful content and teacher intent rather than byte equality of unrelated state. A renamed assignment does not invalidate a score; dates affect lateness; a new attempt needs assessment; an unreadable optional attachment is reported without inventing contents.

## Vocabulary

| Name | What it is | Where it lives | Its job |
|---|---|---|---|
| **CE-SE** | The whole product | this repo | n/a |
| **Engine** | The Go program: `cse.exe` CLI plus the local hub pages | `cmd/`, `internal/` | Mechanical work the agent can't do reliably: talk to Canvas, keep student, attempt and assignment identity straight, exact score arithmetic, safe write recovery. No educational judgment. |
| **Playbook** | The instruction set the agent follows for one kind of task. A teacher says "scoring playbook" and the agent reads that file. Every judgment call about scoring, assignments and policy is the teacher's and agent's, and lives here | Seed: `workspace-template/playbooks/` (embedded in the Engine). Teacher's copy: workspace `playbooks/` (plus the standing `AGENTS.md`, `CLAUDE.md`) | Tells the agent how to do a job: score, create an assignment, edit one, set up |
| **Context** | The teacher's preferences and facts the playbooks draw on | Seed: `workspace-template/context/`. Teacher's copy: workspace `context/` | Feedback voice, grading habits, student notes, courses, calendar |
| **Agent** | Claude working in the teacher's workspace | n/a | Makes the judgment calls using the playbooks and context, then hands decisions to the Engine |

Rule of thumb: if it needs judgment, it goes in a playbook or the context; if it needs Canvas access, exact identity or exact math, it goes in the Engine.

One folder by default. A teacher unzips one **CanvasExpert-SE** folder, points their agent at it and talks; setup happens in conversation and they never edit files. The folder is the **workspace**, with the Engine in `engine/`; the Engine carries the **starter playbooks and context**. The workspace holds the **teacher's playbooks and context** (started from the seed, theirs to change) and student information (`.cse/`, `courses/`). New program versions add missing seed files and never change the teacher's. A synced workspace may instead keep the program in a separate per-machine **program folder**; the workspace's `cse.cmd` prefers `engine/` and otherwise uses that linked program.

## Spine and operation boundaries

```text
Agent (follows a playbook) -> Engine: cse CLI / hub -> feature operation -> shared spine -> Canvas Live
The teacher's playbooks, context and work files live in the workspace, apart from the Engine.
```

The seed lives in `workspace-template/`; judgment belongs in the playbooks and context.

### Engine packages

| Package | Responsibility |
|---|---|
| `internal/canvas` | Origin, authenticated HTTP, pagination, ordinary API errors and downloads |
| `internal/workspace` | Paths, configuration, file I/O and stable student identity |
| `internal/platform` | Per-machine credentials, portable startup and Windows integration |
| `internal/workflow` | Feature operations and their relevant preconditions, outcomes and recovery; Service supplies shared dependencies |
| `internal/domain`, `internal/render` | Feature contracts, rubric arithmetic, feedback and content rendering |
| `cmd/cse`, `internal/localui` | CLI entry points and temporary setup/student/settings UI |

A normal Go function/package is a module. A feature usually adds an operation, command entry point and short playbook. Shared utilities emerge from actual use. Assignment editing shares Canvas transport and workspace I/O but owns its field patch and conflict checks. Feedback revision uses those same mechanics but owns selection and recovery of an exact comment ID. Grading semantics belong with grading, even when several grading functions share them.

Keep the existing layout and useful transport, parsing, identity and arithmetic code. Do not introduce a plugin loader, registry, event bus, job engine, persistence layer, universal policy schema, arbitrary-HTTP write interface for the agent (the read-only `cse api PATH` GET is the deliberate exception, decided 2026-10-10: when the Engine gets stuck the agent reads directly, the teacher makes any write in Canvas, and the gap goes in the workspace `context/engine-notes.md` for development) or new agent protocol. Large files alone do not require splitting.

## Teacher decisions and working behavior

- Teacher instructions authorize the requested scope: score and post posts, draft does not, "show me first" is the default review page (decided 2026-10-10). No mandatory preview/apply tokens. **Decided 2026-10-10, built for scoring (slices 1-2):** review is the default gate. The agent stops at its review page before posting unless the teacher explicitly skips review for that task (scoring, new assignments, new quizzes and assignment edits are gated; rubric attachments and page edits are not); "score and post" still sets scope. The gate is the Playbook instruction plus the command, not an Engine token. See [handoffs/retired/REVIEW-PAGE-DECISIONS.md](handoffs/retired/REVIEW-PAGE-DECISIONS.md). Scoring session alone pulls current-course work and asks what to do; it never implies posting.
- Canvas Live governs existing content. Pull before editing; send only requested changes. Preserve unsent edits when replaced content conflicts, return a fresh version, and let the agent reapply. Read-before-write checks reduce conflicts but cannot eliminate a concurrent edit race.
- Always post the Engine's honest computed assessment. Never use an existing higher entered grade to alter or suppress it; the teacher handles later adjustments. Feedback and rubric report that academic assessment. The agent compares with the strongest earlier academic attempt from actual work, not a teacher-adjusted grade. See scoring rules for arithmetic and posting behavior.
- Use the official Canvas rubric when present, and a common-sense local scoring basis otherwise. A missing Canvas rubric is not a blocker. Rubrics the Engine attaches have automatic grading disabled; existing coupling is not silently changed. Post the honest rubric and computed grade normally, confirm returned outcomes, and report actual failures or uncertainty without claiming guarantees. No workspace certification is needed.
- Canvas alone calculates late point penalties. The agent works out lateness from the actual work, `context/school-calendar.md` and teacher notes, and supplies `grade.late_timing` (`status`, `seconds_late`, `due_at`, `basis`). The Engine writes the lateness timing the agent supplies after checking the attempt is current and the live due date still equals `due_at`; it does not read the calendar. Unclear inputs mean the agent omits timing and tells the teacher.
- Posting holds only for changes to points, rubric, grading type or grading mode; the Engine does not read directions at post.
- Feedback begins with the computed assessment, then compact per-response criterion scores, then conversational glows/grows. Full criterion definitions stay in the rubric; context/grading.md can shape voice.
- The agent infers response units, rubric application and weights consistently; no hardcoded three-response or equal-weight recipe. A deliberately absent Canvas rubric may use an explicit local scoring basis.
- Completed action records refer to the course/student/attempt, not permanent exact output. Teacher feedback/rubric edits do not automatically enqueue completed attempts. New attempts remain discoverable; explicit scoped pulls permit reassessment. Uncertain creates require bounded identity lookup, known IDs when available, and an explanation if ambiguous; never blind replay.
- Newly created assignments stay unpublished until requested rubric/module pieces succeed. Save returned IDs for resumption. "Pending my finalization" means post directly to Canvas, then the teacher reviews and may leave it unchanged. There is no pending state or approval polling. A command that exits when the teacher clicks Post on a review page is not polling, and "pending" stays a local file state. Posting policy and SIS sync are outside scope.

## Workspace and platform

Windows 11, standard user, no installer/admin/runtime requirement for teachers. Go builds a portable executable; platform/distribution checks are ordinary release work, not workspace certifications. The default package is one folder with the Engine in `engine/`; a synced workspace may keep the program separate, backed by a per-machine program link; whenever the hub opens a workspace, the embedded Playbook seed adds missing files without overwriting work. Temporary loopback UI exists only while its command runs. No resident service or database.

PAT and Canvas origin live in per-machine Windows Credential Manager. Never expose the PAT in files, logs, output or agent context. Private student mapping stays in .cse/students.csv and out of ordinary agent reads. Stable student references target persisted work; aliases decorate filenames/metadata and remain editable. Real names are allowed in student writing and teacher notes; do not scrub writing. Expand intentional name placeholders only, and keep teacher-only notes out of student feedback.

Workspace context maps course nicknames (CS8, ELA7PAP, ELA7), grading preferences and student notes. Plain files support authoring/resumption; pulled data is disposable, while unsent work and ordinary returned IDs/outcomes are preserved. One teacher uses one machine at a time; no locks, sync merge system, backup ledger or permanent gradebook.

## Current scope and future capabilities

Current operations cover course/module reads, assignment create/edit and rubric attachment, text submission/history pulls with attachments, component scoring, no-rubric scoring, assessment posting, feedback posting/revision, discovery, and local setup/student/settings pages. Attachments may be unreadable; report that honestly. Normal calls use current API responses and narrow action checks, not disposable-course proof in each workspace.

New Quizzes hold auto-scored items and, since 2026-10-10, written items (`ESSAY`, `FILEUPLOAD`) that the teacher grades in SpeedGrader because CE-SE cannot score them yet. A New Quiz can gain an independent rubric (`pull-item` + `attach-rubric`, important for ECR/SCRs); no other New Quiz edit goes through the Engine. Planned (teacher decision 2026-10-10): rubric-scored feedback comments on New Quiz written work, with the scores visible to the student; see [the handoff](handoffs/active/NEW-QUIZ-SCORING-PROBE.md). New Quiz authoring is `push-quiz`, which reads QuizForge files (item types in `help formats`; the original CanvasExpert verified the item shapes). Hub differentiation (restricted tier pages for assignments and New Quizzes) exists; Hub is the only differentiation style, because the Bridge was declined. Standalone pages beyond tier pages, shared rubrics and curves are future capabilities, not parity requirements. Differentiation uses teacher-managed Silver/Red/Blue tags in ELA7 only (Support/Core/Accelerate), with hints for everyone in the hub itself. Preserve those educational choices without building them to demonstrate modularity.

Historical platform and district observations remain in the evidence document and must not be presented as proof of the current build. Live experiments require a concrete authorized scope; saved credentials or historical IDs are not authorization.

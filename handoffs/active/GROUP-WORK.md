# Group work (book clubs): execution handoff for a coding agent

Self-contained brief for an agent session with no prior context. Target release: **1.1.0-beta.1** ("1.1b1"), developed on the `dev` branch. This is a large body of work. Read this file first, then follow its reading order. Nothing here is implemented yet.

## Paste-in kickoff

> You are implementing CE-SE group-work support in this repository. Read `handoffs/active/GROUP-WORK.md`, then `AGENTS.md`, `ARCHITECTURE.md`, `SCORING-RULES.md`, `docs/group-work/README.md` and `docs/group-work/contracts.md`. Work one slice at a time in the README's phase order, starting with slice 01. A specification file is not proof a dependency exists: verify against the code. Complete each slice's CLI, MCP, review, document and playbook integration and its acceptance protections, commit it to `dev`, and report. Do not run Canvas Live writes, touch `main`, tag, or release unless the teacher says so in this session.

## What this is and who it is for

CanvasExpert-SE (CE-SE) lets a teacher talk to an AI agent that works in Canvas. The Go **Engine** (`cmd/`, `internal/`) does Canvas access, identity, exact arithmetic and safe writes. The **Playbooks** (`workspace-template/`, embedded in the Engine) hold all educational judgment. Capitalize Engine and Playbook this way; avoid bare "SE". Put judgment in Playbooks, not Go.

The owner is a 7th-grade teacher (Pearland ISD) who builds this by vibecoding and does not write code. They want plain-English explanations, speed over ceremony, and a tool colleagues can use: unzip one folder, point an agent at it, talk; no one hand-edits files. The goal of this work: a teacher can run book clubs in Canvas (clubs, memberships, shared Pages, discussions, shared products with shared or individual grading, peer critique, calendar, conferences, messages), with the agent making instructional choices and the Engine making safe, scoped writes.

## Reading order

1. `AGENTS.md`, `ARCHITECTURE.md`, `SCORING-RULES.md`, `CONTRIBUTING.md`, `docs/releasing.md`, `docs/build.md`.
2. `docs/book-club-group-work-discovery.md`: research and API findings.
3. `docs/group-work/README.md`: slice map, tiers, phases, **Gate P results**.
4. `docs/group-work/contracts.md`: shared contracts (identity, documents, review, grade scope).
5. The slice you are building (`docs/group-work/NN-*.md`). Each ends with **Code anchors** that name real files and lines in Dev; verify them, since lines drift.

## Phases and scope

Tier A and B are scheduled; Tier C is specified but built **only when the teacher requests it**.

1. **Phase 1:** 01 → 02 → 03 → 05a (ungraded/distributed discussions) → 06 → 16 → playbook v1 (slice 17, only what exists).
2. **Phase 2:** 08 (owns the shared snapshot foundation) → 09 → 10 → 11, then 05b (graded discussions) and 07.
3. **Phase 3, on request only:** 12, 13, 04, 14, 15.

Run slices **serially**. Every slice edits the same shared files (`cmd/cse/main.go`, `cmd/cse/remote.go`, MCP registration/schemas and smoke check, `engine-docs/formats.md`, `internal/localui/capabilities.go`, `docs/engine-capabilities.md`). Finish and commit one slice before starting the next.

## Standing rules (these come from the teacher; they are not in any agent's memory)

- **No read-backs.** An accepted Canvas write (no HTTP error) is done. Do not confirm by shape of the reply, do confirmation GETs after writes, or print "partial/uncertain" noise for writes that landed. Keep cheap *pre-write* guards: course/student/attempt/group identity, wrong target, duplicates, audience. Do not add retry or verification machinery after writes. A 408/5xx/redirect on a write is uncertain: do not blindly replay.
- **Proportionality.** Do not add coverage quotas, certification rituals, universal operation frameworks or speculative migrations. A conceivable failure is not a reason to add code; ask whether the teacher would notice it and what recovery costs. Keep cheap checks for wrong-target writes, privacy exposure, dishonest grades and loss of student/unsent work. A slice's acceptance list names protections, not a test matrix.
- **Honest grades.** Post the computed assessment even when it is lower than an entered grade. Never calculate a late penalty in CE-SE. No contribution multipliers. No automatic zeros without complete relevant evidence.
- **Canvas Live is authoritative.** Preserve teacher intent and unsent work. Protect credentials and teacher-only notes. Respect requested posting scope.
- **Teacher-facing copy** (review cards, WebUI text, Playbook seed text, error messages the teacher sees): a short label plus one plain sentence that names specifics. Warm but plain: no jokes, no exclamation marks, no internal jargon in primary text. Consequential actions get a brief explanation by the action. Honesty limits unchanged.
- **Installation constraints.** The district locks work laptops (no admin, no installs). District OneDrive blocks `.exe`, `.js`, `.py` and similar, so never design anything that relies on syncing executables or scripts, and never put the CE-SE folder in OneDrive. UserSync (optional cloud folder) is for the agent app's data only.
- **Three-step bar.** Setup and any new teacher workflow must be agent-driven in conversation; never require a teacher to hand-edit files.
- **Docs honesty.** Keep Canvas Live evidence, local tests and pending human validation distinct. Local tests alone are never live proof. When a capability, limitation or status changes, update `internal/localui/capabilities.go` and `docs/engine-capabilities.md` together with a review date. Keep `engine-docs/` contracts and `workspace-template/` aligned with behavior, and add notable user-visible changes to `CHANGELOG.md`.

## Gate P results you can rely on (Canvas Live, 2026-10-17, CS8)

Full table in `docs/group-work/README.md`. In short: shared-mode grade and rubric propagate from one member to all; `comment[group_comment]=true` creates separate per-member comment copies with different IDs and `false` reaches only the named member; individual mode writes only the named student; `group_members` returns `[{id (string), name}]`; a graded group discussion's linked assignment has `group_category_id: null` and its topic lists `group_topic_children` immediately; the built-in Test Student cannot be added to a group; a grade write on a member with no submission record returned 403 for an undetermined reason (hold, do not retry blindly). Not observed: late timing, excused/hidden members, real shared submissions, student-facing views, notifications.

## Environment

- Windows, PowerShell primary. Go toolchain is the exact version in `go.mod`. Build and validate with `.\scripts\build.ps1` (runs tests and vet, writes `dist/`). Use focused `go test ./internal/...` while developing and run the full build before reporting a slice done. `gofmt` only files you changed.
- Use synthetic fixtures and temporary UserInstall folders for tests. Never develop against a teacher's runnable UserInstall or private UserSync files. Do not register live clients or migrate teacher files from tests.
- If using a Git Bash shell, `MSYS_NO_PATHCONV=1` prevents it rewriting `/api/v1/...` arguments.
- Git may warn that LF will become CRLF; do not churn line endings.

## Live Canvas rules

CS8 (course 121046) is a real class with real students plus a built-in Test Student. Existing credentials and historical course IDs do **not** authorize any Canvas write. Run a live check only when the teacher gives a concrete scope in this session, use disposable objects named with a clear prefix, never read or print roster details broadly (names and grades appear in raw enrollment responses), avoid placing real students in anything student-visible unless the teacher has explicitly chosen that, and clean up what you create. The Gate P authorization is spent; do not reuse it. `cmd/cse-probe` is a developer-only probe hardcoded to CS8 and the built-in Test Student.

## Per-slice protocol

1. Verify the slice's dependencies and Code anchors in the actual code. Read the existing behavior it touches.
2. Implement domain, workflow, CLI and MCP adapters, review rendering, validation/offline dispatch, formats/limits docs, examples and Playbook/seed guidance together. Use existing functions; do not build a preliminary framework.
3. Cover each acceptance protection with a proportionate check on the real operation path. Preserve existing individual assignment, Hub and New Quiz behavior (the shared-boundary list at the end of `contracts.md` names concrete compatibility points).
4. Update the compiled MCP smoke check, `docs/mcp-adapter.md`, `engine-docs/formats.md` and `field-limits.md` where applicable, and the capability records with the **actual** status.
5. Run the full build. Commit the slice to `dev` with a message naming it. Report: files changed, checks run and actual outcomes, remaining evidence limits, anything held or deferred, and any spec statement that proved wrong (fix the spec in the same commit).

## Release and git

Work on `dev`. Do not touch `main`, create tags, publish, or build `-Release` unless the teacher asks. Per `docs/releasing.md`, when the teacher says the work is ready: set `VERSION` to `1.1.0-beta.1`, add the CHANGELOG entry and update version mentions listed in `docs/releasing.md`, and let the teacher create the tag. Current stable is `1.0.0`; the next beta after `1.1.0-beta.1` is `1.1.0-beta.2`. Do not push unless the teacher's instructions in the session say to.

## Stop and ask the teacher when

A slice needs a choice only the teacher can make (grading mode for a unit, whether to send a message), a live check needs authorization, a spec instruction conflicts with the code in a way that changes the design, or a result would affect real students. Otherwise continue; when one item cannot proceed, explain that item specifically and do independent work.

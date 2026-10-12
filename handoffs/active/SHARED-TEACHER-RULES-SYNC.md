# Optional Sync as the authority for teacher rules

Status: **implemented in Source, 2026-10-12; provider/client adoption remains unobserved**. The implementation follows this bounded direction; the original requirements below remain its contract. This belongs to the **Engine** (file routing, configuration and settings) and **playbooks/context** (instructions and documentation). No real Sync directory or live Install was selected or migrated in this implementation session.

Delivery: Settings and `cse rules-location [--sync ABSOLUTE_PATH | --local]` configure the same binding as MCP `engine_info` operations `rules-location` and `set-rules-location` (`sync_root`, empty for local). Mode values are `local`/`sync`; status distinguishes Install and rules roots. Full pinned Go tests/vet/package workflow and compiled stdio checks pass; loose/ZIP/MCPB Engine identities match. Connected temporary-client tests cover next-read visibility, conflict hashes and hub grading saves. The capability record contains the final package identity and evidence boundaries. Real provider propagation and human-client adoption are the remaining observations.

Read repository AGENTS.md, ARCHITECTURE.md and SCORING-RULES.md first. Inspect `git status`: the Source/Install/Sync naming and New Quiz documentation work is currently uncommitted, including the synthetic comment-only example. Preserve it and all other existing changes. Implement this handoff; do not reopen the agreed folder model or expand it into a synchronization framework.

## Problem and approved direction

A teacher uses CE-SE on multiple PCs and develops personal rules, playbooks and guides. Today each Install owns a separate copy, so the PCs drift. The teacher agreed that **Sync should optionally be the authoritative home of those teacher files**, with each Install reading and editing them directly. Copying rules into every Install on startup would retain the drift problem.

| Name | Responsibility |
|---|---|
| **Source** | Product code, starter defaults, contracts and build/release artifacts |
| **Install** | Local runnable Engine, machine binding and operational/student work state |
| **Sync** | Optional shared teacher instructions, playbooks, guides and preferences in a locally accessible OneDrive/Google Drive directory |

With no Sync configured, keep the current one-folder behavior: all teacher files are in Install. When configured, selected teacher files use Sync as their sole active authority. The cloud provider transports changes; CE-SE resolves and uses the files. No cloud SDK, account login, watcher, replication queue, merge engine, database, distributed locking or background service is needed. One teacher still uses one PC at a time; provider synchronization determines when changes arrive on another PC.

The task determines the folder, not the agent app. Claude Code/Codex use Source for development and Install for teacher operations. Cowork/GPT Work normally reach Install through MCP. The executable and commands stay in Install in both modes.

## File ownership for this slice

Keep logical MCP paths stable. Route each path by the following table; do not globally replace the installation root with the Sync path.

| Logical file/path | Local mode | Sync mode |
|---|---|---|
| Teacher `AGENTS.md`, `CLAUDE.md` | Install | Sync |
| `playbooks/` | Install | Sync |
| Teacher root `.md`/`.txt` guides, including custom `README.txt` | Install | Sync |
| `context/grading.md`, `context/school-calendar.md`, `context/engine-notes.md`, other teacher-created context reference files | Install | Sync |
| `context/courses.md` | Install | Install |
| `context/students.md` | Install | Install |
| Product `README.md`, launchers, `engine/`, `.claude/` integration glue | Install | Install |
| `courses/`, packets, drafts, assets, results, receipts and outcomes | Install | Install |
| `.cse/`, aliases, rosters, private identity and local binding | Install | Install |
| Canvas credential | Windows Credential Manager on that PC | Same |

Course connection/selection and student notes stay local in this bounded rules-sharing slice. `context/courses.md` is also the existing Install/root marker. Private student references are randomly allocated in `internal/workspace/students.go`, so independently configured PCs do not necessarily share them. Do not relocate student-specific context, mappings or student work as an incidental part of fixing playbook drift. Shared root guides remain ordinary teacher-authored text; keep existing rules for teacher-only information and student output.

This table is a small path classification in existing workspace code, not a general mount system or registry. Preserve existing file-type and privacy restrictions. Explicitly selecting Sync authorizes access to that designated teacher-rules directory; it does not authorize browsing an entire cloud drive. Use a dedicated directory chosen by the teacher. Do not infer it from environment variables, an old client registration or the agent's working directory.

## Configure and expose the active authority

Store the absolute Sync directory as a **per-Install local binding**, for example `Install/.cse/rules-location.json` with one optional `sync_root` value. An absent binding means local mode. The local path differs across PCs and must not be stored in the shared rules. Keep this file outside ordinary agent reads; surface only non-secret mode/location information through the supported interface.

Add a small Settings control for **Teacher rules: Install / Sync**, showing both Install and the effective rules location. The selected Sync directory must be explicit and validated before saving the binding. A plain path entry is adequate; an existing folder chooser may be used if it serves this setting. This does not restore the retired picker for switching the whole CE-SE workspace or executable.

Provide the same configuration/status operation for CLI and typed MCP setup so the agent can perform setup after the teacher supplies a path. A small `rules-location` CLI command and an extension of the existing `engine_info` administrative tool are sufficient; choose final argument spelling consistently with existing commands and document it. Do not expose arbitrary filesystem or shell access. Status must distinguish `install_root`, `rules_root`, and local/sync mode. `list_courses.root` keeps its current meaning: Install. Any extra metadata is additive.

Read the binding and relevant rules files when needed. Edits delivered by the provider must appear on the next tool/UI read; normal rule changes must not require rebuilding or restarting MCP. Do not cache teacher instructions for a whole server session. A teacher should be able to edit grading voice in the hub, then have the already-connected agent read the same updated file.

If configured Sync is unavailable or a requested file cannot be read, explain that path/problem and preserve the binding and all files. Do not silently use inactive Install copies or overwrite Sync with defaults. Continue independent operations that do not need that unavailable file. Ordinary correction is to restore access or explicitly change the rules location; no custom cloud-recovery workflow is needed.

## Preserve personal rules when enabling or changing mode

For a new, empty chosen Sync directory, initialize it from the current Install's actual shareable teacher files, including custom guides. Preserve teacher content; do not initialize from Source defaults in place of existing customized files. Use the embedded seed only for missing supported starter files. Copy only the shared categories above: no executable, `.cmd`, `.cse`, student/course work or machine settings enters Sync.

For an existing shared rules directory, its contents become authoritative. A second PC must not overwrite them or import its differing local rules automatically. Keep local originals or archive copies before replacing any local entry point with a bootstrap. Explain that preserved local files are inactive in Sync mode. If a teacher wants to bring a local customization into the shared version, present the specific difference and use ordinary teacher-directed editing, with existing file hashes. No automatic merge is required.

Changing back to local mode is explicit. Preserve the current shared rules in Install when they are readable, archive differing existing local files before replacement, and leave Sync intact for other PCs. If the shared source cannot be read, report that limitation and retain the current binding rather than silently restoring obsolete instructions. Do not delete or move student/unsent work in any mode change.

Engine updates still add missing seed files and preserve existing teacher files. In Sync mode, missing teacher seeds go to the active shared location; program-owned glue and local-only context remain in Install. Settings' comparison/replacement action must operate on the effective teacher file and archive the version it replaces. Context preferences remain protected from automatic replacement. An older Engine on PC B must never replace shared customizations with its older seed. Different Engine versions may offer different explicit defaults; this feature synchronizes teacher rules, not program versions.

## Agent and Engine routing requirements

1. **Keep the Install boundary.** Bundled root selection, wrong-root rejection, CLI work paths, MCP child processes, Canvas identity and local sessions remain tied to Install. Do not point registrations or `--workspace` at Sync. Both desktop registration and the existing Engine-only extension must resolve the same per-Install rules binding.
2. **Use the active file in every relevant surface.** Route MCP file listing/read/write, grading-voice UI reads/saves, playbook status/replacement and seed addition through the same small workspace location helpers. Keep courses, student notes and course asset/draft tools local according to the table. List the logical active file once, not both shared and inactive local variants.
3. **Retain truthful paths and conflict protection.** MCP calls such as `read_workspace_file("playbooks/score.md")` and `write_workspace_file("context/grading.md", ...)` keep their logical paths but operate on Sync when configured. Return useful authority metadata without returning private state. Existing SHA-256 replacement checks apply to the actual authoritative file. Adapt `safePath`, `relativePath` and `writeWorkspaceBytes` so a legitimate selected external rules root is neither mistaken for an escape nor used to loosen access to private/work/Engine paths. Preserve hidden/link exclusions and ordinary provider placeholder behavior.
4. **Handle direct local agents.** A local app may automatically load Install's physical AGENTS.md/CLAUDE.md before calling MCP. In Sync mode keep a small, machine-maintained local bootstrap that directs it to the authoritative shared instructions, identifies Install for command/work paths, and keeps privacy rules clear. Preserve the original local teacher instructions first. MCP logical reads/writes of AGENTS.md/CLAUDE.md reach the teacher's shared files, not the bootstrap. Do not let an auto-loaded full local copy compete with shared instructions. Do not run launchers from Sync.
5. **Keep scope and judgment unchanged.** Teacher authorization, review defaults, accepted-write behavior, honest assessment and New Quiz support remain as currently agreed. New Quizzes and written-response scoring stay open within the existing comment-only limits. This change moves rule ownership; it adds no grading policy or extra review gate.

Logical references within shared instructions need to distinguish locations: rule/reference reads use the effective teacher-rules root; course work and commands use Install. Update the setup and standing instructions accordingly. The playbooks must not require the teacher to manually copy changes between PCs.

## Source starting points

- `internal/workspace/paths.go`, `scaffold.go`: Install resolution, confinement and seed preservation.
- `internal/mcpserver/server.go`, `workspace_files.go`, `operations.go`, `sessions.go`: startup, configured discovery, file routing/hash writes, administrative tool and child commands. The adapter currently assumes all physical files are beneath `s.root`; revise that assumption only for the shared categories.
- `internal/localui/hub.go`, `hub_context.go`, `hub_pages.go`, `settings.go`, `playbook.go`: settings, grading voice, local course/student preferences and explicit seed replacement.
- `cmd/cse/main.go`, `remote.go`, `internal/workspace/instance.go`: Install/course roots stay local; add the small configuration command without redirecting operational state.
- `workspace-template/AGENTS.md`, `CLAUDE.md`, README.md, setup/scoring/quiz playbooks; `integrations/skills/canvasexpert/SKILL.md`; `engine-docs/formats.md`; build/MCP docs: teach the effective locations without renaming existing workspace tools.
- `internal/localui/capabilities.go` and `docs/engine-capabilities.md`: update status and review date together. They must distinguish source/local verification from actual provider/client observations.

Use existing Go/package patterns. Split implementation among agents only if useful and allowed by the session instructions; coordinate ownership of shared routing helpers before parallel edits. Do not add a module loader, universal filesystem abstraction or policy framework.

## Proportionate verification and delivery

Use focused tests with **two temporary Install directories pointing to one temporary Sync directory**. No Canvas calls are needed:

- No binding preserves one-folder paths and existing seed behavior.
- A rule edited through PC A's MCP appears on PC B's next read without restart; a stale expected hash is rejected after another write. This verifies local shared-file behavior, not cloud propagation or atomic cross-PC edits.
- A hub grading-voice save and connected MCP read use the same active file. Configured courses/root remain local and unchanged.
- Enabling on an empty directory preserves a customized local rule; connecting a second PC to existing shared rules preserves the shared content and the differing local original. Program updates/replacement preserve or archive teacher files as described.
- Private/Engine/work files remain outside the shared route, traversal/link exclusions remain effective, and default rule access is not broadened to an arbitrary cloud drive. Reuse current file/root/privacy regressions.
- Missing Sync produces a specific error without stale-copy fallback. A local-agent bootstrap points at the shared rules while command/work paths still target Install.

Run relevant Go packages, then the documented pinned build/test/vet/package workflow once the implementation is ready. Reuse `scripts/check-mcp.mjs` for a compiled stdio check and extend only what this behavior requires. Do not write tests that merely check documentation phrases or build a provider/platform matrix. Verify the resulting loose/ZIP/MCPB Engine identities; do not count a build as an updated installation.

Update architecture, source agent guidance, teacher seeds, MCP help/descriptions, setup instructions, package notes and both capability records together. Describe default local mode, optional authoritative Sync mode, the shared/local file boundary and the provider's role. Actual cloud delivery and any human-client observation remain explicitly pending unless observed; those evidence limits are not teacher-use prohibitions.

Deliver source/package implementation and a short result report with the relevant checks and remaining provider/client observations. **This handoff does not authorize choosing a real Sync path, moving teacher files, rewriting client registrations, updating a live Install, opening teacher-only pages, or writing to Canvas.** Use temporary fixtures for implementation; actual adoption needs the teacher's explicit chosen directory and scope. The current known Install is `D:\CanvasExpert-SE`; it is background context, not a migration target selected for this task.

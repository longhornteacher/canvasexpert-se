> **Retired 2026-10-13 (work landed).** Execution record only. Durable build/release guidance: [docs/build.md](../../docs/build.md), [docs/releasing.md](../../docs/releasing.md); status and evidence limits: [docs/engine-capabilities.md](../../docs/engine-capabilities.md). Remaining items are real client/Canvas observations, not code work.

# 03 — Make the fresh install work for a teacher

Status: agreed direction, implementation pending. Follow [01-HANDOFF-CLEANUP.md](01-HANDOFF-CLEANUP.md) for ordering/agent rules. [02-ONE-CURRENT-BUILD.md](02-ONE-CURRENT-BUILD.md) owns all final packaging and generated-output cleanup.

## Outcome and evidence

A teacher unzips the single download and says “install this.” The agent asks **“Are you using Claude, ChatGPT, or both?”**, does the supported setup work, opens the private Canvas form, and gives only the real remaining trust/reconnect/course-choice actions. The teacher should not need to know Cowork, Work, MCP, adapters or build variants to answer the opening question.

Internal `--client cowork|work`, protocol names and config keys can stay as they are. “Claude” and “ChatGPT” are teacher-facing choices, not proof that a particular client mode supports the local connection. The installing agent uses available app/context evidence to select the supported route. If needed, ask a concrete follow-up such as whether the teacher uses the app on this PC or a website; do not make the teacher translate frontier-lab product taxonomy or promise unsupported connectivity. Explicit “both” authorizes both helper routes; order is unimportant.

Primary evidence: [fresh-laptop notes](../retired/install-session-notes-2026-10-13.md). Detailed Source analysis: [triage](../retired/FRESH-INSTALL-TRIAGE-20261013.md). The laptop used the exact `ae41e92e9916+dirty`, `2026-10-12T23:09:05Z` artifact recorded in the [earlier package report](../retired/FRESH-INSTALL-PILOT-FINDINGS-20261012.md). Registration/preparation and local MCP passed, both WebUI starts timed out, default MCPB opening reached an app chooser, explicit Claude launch reached its install dialog, and starter origin persistence failed until manually corrected. Real destination tool calls were not established.

Do not rerun all historical handoffs. No scoring, quiz, Sync implementation or redesign work belongs here.

## I1 — Fix installer WebUI startup and reuse

**Owner/files:** Engine agent; `cmd/cse/install_ui.go`, `cmd/cse/main.go`, `internal/localui/hub.go`, `internal/localui/hub_discovery.go`, existing `internal/hubcoord` and focused tests.

**Known defect:** `launchInstallUI` passes `--ready-file`; `RunHubWithOptions` skips assigning `hubUnlock` when that option is set, but later calls it for a nonempty root. The nil call is before browser opening/readiness. This Source defect matches the laptop timeout. Manual `webui.cmd` takes another path.

1. Make the installer path use the existing same-Install coordination/reuse and publish readiness on both new and reused hubs. A nil guard alone leaves the bypass/reuse problem; do not just lengthen the eight-second timer.
2. Ensure startup failure is reported usefully and cannot erase successful registration/local-check outcomes. Reuse existing sanitized root `install-log.jsonl`; no new log framework/private dump.
3. Preserve successful-start URL output and an actionable `webui.cmd` fallback on failure. Distinguish a browser launch request from actual browser visibility. Do not invent a URL for a server that never started.
4. Add a small structured UI failure/warning to the existing install result stream if needed so agents can distinguish partial completion. Preserve consumers/exit semantics unless changing them is necessary and documented. No elaborate exit-code family.

**Check/done:** focused regression exercises the actual ready-file branch; new start and repeat same-Install reuse return valid readiness and the correct root; UI failure retains registration/check status. B3 supplies the final packaged subprocess check. Run relevant existing startup/localui/CLI checks once; no real client settings or credentials.

## I2 — Open the Claude extension without relying on file association

**Owner/files:** platform/installation agent; `internal/platform/`, `internal/installation/`, `internal/localui/connect.go`, `internal/installlog/log.go` and focused tests. Serialize any shared CLI/UI changes with I1.

1. Locate the observed stable Windows Claude launcher (`%LOCALAPPDATA%/AnthropicClaude/claude.exe`) and launch it with the absolute MCPB path as a separate argument. Use supported local discovery where available. Do not make exhaustive Store/MSIX/macOS coverage a prerequisite to repairing this Windows case.
2. If the app cannot be located/launched, reveal the prepared file and give the short manual extension-install action. Do not return the user to an unhelpful association chooser as the only fallback.
3. Report direct-app launch requested, file revealed/fallback, or failure accurately; none means the extension was trusted/installed or its tools connected. Update existing strict log outcomes and WebUI wording together. Preserve `--no-ui`/`--check-only` no-open behavior and the thin single-Engine launcher.

**Check/done:** injected direct-launch/fallback/failure checks, separate argument handling with spaces, and no-open modes pass. No requirement to install Claude or bypass its trust prompt. The old pilot's teacher-confirmed install dialog remains historical evidence until this repaired path is actually observed in a client.

## I3 — Resolve the origin mismatch through the existing setup path

**Owner/files:** setup agent; `internal/localui/setup.go`, `internal/localui/hub.go`, `internal/workspace/instance.go`, `cmd/cse/main.go`, `cmd/cse/remote.go`, `internal/doctor/doctor.go` and focused tests. I1 must finish before this agent edits shared files.

Both hub and standalone setup already call `workspace.SyncInstance` after saving credentials. The helper handles the starter/empty course case and protects populated different-origin course selections. The pilot package reportedly already included it. Do not declare the issue fixed merely by finding that helper, assume an old executable, or add a second configuration writer.

1. Exercise real hub callback wiring with mocked validation/save and a fresh starter document, then standalone completion. Confirm intended Install root, origin persistence, preservation of selections/body, and visible partial-save failures. Inspect the existing sanitized Install-root log if session evidence is available; `.cse` inspection/private form scraping is unnecessary. Do not require the remote laptop to make local progress.
2. Repair the demonstrated integration defect. If local reproduction passes, record exactly what passed and what field cause remains unknown; improve actionable diagnostics and leave that item honestly unconfirmed. Do not invent a speculative migration/recovery subsystem to force closure.
3. Show both sanitized origins in CLI/doctor mismatch explanations with a concrete correction. Never expose tokens, URL userinfo/query values or raw credential objects. Keep intentional populated different-origin configurations protected from silent adoption.

**Check/done:** current fresh-form completion records the intended origin and configured course discovery can proceed; focused mock tests preserve populated different-origin courses and teacher content. If actual laptop cause remains unknown, name it in the completion row; independent slices and packaging continue.

## I4 — Write the teacher walkthrough and app labels

**Owner/files:** playbook/UI-copy agent after I1–I3; `workspace-template/playbooks/install.md`, relevant parts of `setup.md`, Source and seed READMEs, `TEACHING-ASSISTANT.md`, and install/Agent connection labels/templates. Coordinate build/download wording with B2. The coordinator updates capabilities in I6.

1. Use the exact opening question **“Are you using Claude, ChatGPT, or both?”** wherever setup asks the teacher to choose. Accept an already supplied destination without asking again. Keep technical identifiers in developer docs/details, not the first-run question/buttons. Explain client-mode limits only when needed for a next action; no global rename of APIs or historical evidence.
2. Explicit “both” runs both existing helpers, with separate outcomes. The agent launches the prepared extension and handles supported recovery. No new CLI destination enumeration is needed.
3. Present the actual local form URL as fallback; PAT stays in the private form. Preserve working credentials on repeat install. Have the agent distinguish authentication rejection from connectivity problems and configuration mismatch.
4. Course IDs/states help propose likely current courses; teacher supplies nicknames and confirms choices. No developer's name/class IDs in generic seed guidance. Offer teaching preferences progressively.
5. After teacher trust/reconnect, the destination reads current instructions/preferences and configured courses. Compare `install_root` before teacher work; settings saved/local checks are not connected tools. Do not restart during an active review. Keep the next teacher action short and concrete.

**Check/done:** walk through the text for Claude, ChatGPT, both, and partially successful setup. No teacher must understand internal product names to answer the first question. Render/inspect changed UI labels only as needed; no wording snapshot tests. Install-session edits do not automatically update Source, and seed updates must not overwrite customized Install/Sync rules.

## I5 — Small input/output improvements, with a firm scope limit

**Owner/files:** sequential small Engine slice after core repairs; existing setup input, version/install output and CLI help/progress surfaces only.

1. Accept bare Canvas hosts and pasted HTTPS Canvas links in the private setup input by normalizing to origin before existing validation. Discard paths/query/fragment without echoing secrets; reject embedded credentials and unsafe schemes. Preserve custom HTTPS Canvas domains; no Instructure-only allowlist. Agent guidance may extract the public origin, but add no new agent credential-writing tool or `setup --origin` subsystem.
2. Compact the install local-check `child_engine_version` to useful identity fields rather than the entire human dependency listing. Retain sufficient revision/build identity to distinguish loaded adapter from disk; product version equality alone does not establish that. Preserve existing machine-readable consumers or update them together.
3. Reproduce the reported PowerShell 5.1 stderr/help symptom with the packaged launcher or a controlled fixture. Native exit status is distinct from stderr rendering. Current Source returns 0 for `help setup` while showing general help; do not assert it returns 255 without reproduction. Improve unsupported-topic guidance; suppress/gate noninteractive progress if the reproduced behavior warrants it. Keep JSON stdout clean; do not move progress into it.

**Check/done:** focused normalization/identity checks pass; CLI diagnostics distinguish native failure from presentation. No broad help framework, hostname policy, backup pruning or platform support matrix. An unrepeatable shell symptom is recorded as such rather than blocking delivery.

## I6 — Integrate evidence and report the real remaining actions

**Depends on:** B3–B4; this slice does not produce another package.

The coordinator updates `internal/localui/capabilities.go` and `docs/engine-capabilities.md` together with the review date: code/local-test fixes versus earlier teacher-observed form/dialog/course read versus pending real client adoption. Use B3's exact build/hash for current package evidence. Keep detailed older build histories in retired/evidence records, not a competing “current artifact” claim in primary guidance.

Do not claim successful Claude/ChatGPT adoption without a real tool call in each selected destination reporting the intended Install root. This Source session does not require teacher trust clicks, PAT entry or live grading to complete its code/package work. Report pending client actions once; do not start a monitoring/verification campaign or reopen all yellow capabilities.

| Slice | Status |
|---|---|
| I1 WebUI startup/reuse | Done 2026-10-13 (code + focused tests): installer path now uses the same-Install lock/reuse and publishes readiness on new and reused hubs; UI failure logs `hub:failed:startup`, emits a `ui_warning` with a `webui.cmd` fallback and keeps registration/check results. Not run end to end (B3 checks the packaged subprocess). If the hub fails after taking the lock but before readiness, the installer still waits out the 8 s timeout. |
| I2 Claude direct launch | Done 2026-10-13 (code + injected tests): launches `%LOCALAPPDATA%\AnthropicClaude\claude.exe` with the MCPB as a separate argument; else reveals the file in Explorer with manual steps (`file_revealed`); `launch_failed` if both fail; no-open modes unchanged. Only the observed stable location is searched; not observed in a real client. |
| I3 origin persistence | Local path verified, field cause unconfirmed: real hub setup callbacks (mocked validation/save) persist the origin into the starter `courses.md`; populated different-origin configs stay protected. Fixed: case-insensitive origin comparison; mismatch messages in CLI/hub/doctor name both sanitized origins and the correction. Laptop cause unknown (an empty-root hub skips the sync silently; unproven). Its `install-log.jsonl` would settle it. Standalone `cse setup` not run end to end. |
| I4 teacher guidance/labels | Done 2026-10-13: install.md, setup.md, both READMEs, TEACHING-ASSISTANT.md and connect.html use “Are you using Claude, ChatGPT, or both?”; outcomes, error classes, course proposal and handoff wording updated. Not rendered in a browser. `connect.go` result labels still say Work/Cowork (a test asserts them); small follow-up if wanted. |
| I5 bounded input/output fixes | Done 2026-10-13: bare hosts/pasted HTTPS links normalize to origin (unsafe schemes/userinfo still rejected); `child_engine_version` compacted to version/revision/built/build_kind; unknown help topics list real topics; routine remote progress dropped when stderr is not a terminal. PowerShell 5.1 `help setup` exit 255 NOT reproduced (exit 0, no stderr); reproduced separately that `2>&1` with `$ErrorActionPreference=Stop` throws on any stderr line even at exit 0. |
| I6 evidence/delivery | Done 2026-10-13: `capabilities.go` and `docs/engine-capabilities.md` updated together (review date 2026-10-13) with the current artifact hash; code/local-test fixes kept separate from the earlier teacher-observed pilot and from pending real client adoption. Pending real actions: Claude trust/extension install, ChatGPT/Work restart, tool calls reporting the intended Install root in each selected destination, and an installer WebUI launch on a fresh machine. |

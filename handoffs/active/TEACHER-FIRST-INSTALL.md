# Teacher-first installation and MCP handoff

2026-10-12. Status: Engine/release and Playbook/onboarding slices implemented in Source; fresh-user destination validation pending. The complete teacher journey is not yet marked delivered. This work spans **Engine** installation/connection mechanics and **Playbook** onboarding/teaching context. Read Source AGENTS.md, ARCHITECTURE.md and SCORING-RULES.md first.

## User requirement

The first colleague could use Code/Codex, but MCP setup asked the teacher to enter commands in a CLI. The user wants:

1. Unzip.
2. Point Code/Codex at the extracted Install and say, "Hey, install this."
3. The agent handles installation and launches the WebUI for the teacher's Canvas PAT.

Code/Codex is the installation assistant. The intended teaching destination remains Cowork/Work with the teacher's pedagogical context. No teacher terminal commands, JSON/TOML editing, Source checkout, Go, Node, or separate Codex CLI installation should be required. A genuine client trust prompt or necessary app restart can remain, with a short explanation. Do not promise that CE-SE can suppress client/organization controls.

## Findings from Source

| Finding | Evidence and teacher impact |
|---|---|
| The release begins after the hardest step. | `workspace-template/README.md` says "Connect CE-SE to your agent through MCP". `playbooks/setup.md` assumes Engine access and covers Canvas, courses and calendar, not client installation. AGENTS.md has no explicit "install this" route. |
| The registration helper is outside Install. | `scripts/package.ps1` packages the template, Engine and webui.cmd; it does not ship `scripts/connect-mcp.ps1` or the adapter guide. The guide starts its connection example in Source. |
| The helper requires an unrelated CLI. | `scripts/connect-mcp.ps1` invokes `codex mcp add`. With default `Both`, failure in Work registration terminates the script before Cowork registration. It prints registration success without an MCP handshake or destination-client call. |
| The hub has no agent connection workflow. | `internal/localui/hub.go` lists "Connect CE-SE to your agent" as step three, with no connection card/action. Its token test is not a test of MCP. |
| Cowork packaging owns a second Engine. | `integrations/cowork/manifest.json` starts the Engine inside the MCPB. `scripts/package-mcp.ps1` bundles that executable separately; updating Install does not update it. The manifest version is fixed at 0.1.0, and MCP initialization reports 0.1b. Neither identifies the loaded build. |
| Version checks can conceal a stale adapter. | `engine_info version` invokes a child executable through `Server.command`; the running adapter can be older. Earlier recorded runs actually encountered stale MCP behavior. Initialization/loaded adapter identity and on-disk Engine identity need to be distinguished. |
| Pedagogical onboarding is incomplete. | MCP initialization correctly directs the agent to AGENTS.md and the task playbook. Scoring explicitly reads grading preferences; setup never elicits them, and the authoring playbook does not explicitly load a teaching profile. The blank grading seed and procedural playbooks do not recreate a personalized Teaching Assistant Project by themselves. |
| Setup misclassifies Canvas failures. | The setup playbook says any unsuccessful Canvas course discovery should lead to token setup. An outage or district network restriction is not evidence that the PAT needs replacing. Preserve a working saved credential while explaining the actual failure. |

The shared Engine design remains suitable. MCP exposes 26 typed tools, uses the same commands, supplies active Install/Sync file routing and guards unsent-file replacement. Do not rewrite the adapter or put teaching judgment into Go.

Other inspected limits: review/UI sessions are owned by the MCP process and end when it disconnects; session IDs are in memory. Base64 assets are limited to 20 MiB and are returned as text, while pulled attachments can be larger; file transport alone does not establish convenient PDF/DOCX/image consumption in every client. These affect teaching workflows, but are not prerequisites for fixing installation. Do not turn this slice into a session persistence or document-processing project. Do not restart a client in the middle of an active review.

## Client-route evidence

Official documentation checked 2026-10-12:

- [OpenAI MCP configuration](https://learn.chatgpt.com/docs/extend/mcp): desktop local clients support stdio and share host configuration with Codex; a desktop Settings route exists without a separately installed CLI. Hosted Work is a different plugin/remote route. Account/surface availability must not be inferred from the word "Work" alone.
- [Claude local MCP setup](https://support.claude.com/en/articles/10949351-getting-started-with-local-mcp-servers-on-claude-desktop): custom MCPB installation and client connection inspection are documented; private extension updates are manual and organizational controls can restrict installation.
- [Claude connector surfaces](https://support.claude.com/en/articles/11725091-when-to-use-desktop-and-web-connectors): local and remote routes have different surfaces; local-MCP plugins are described for Cowork/Code. This does not independently prove that editing the legacy desktop JSON enables every Cowork configuration.

The repository records earlier Cowork tool discovery and shared Work/Codex calls, plus separate cloud Claude authoring evidence. Those are useful observations, not proof of today's fresh-user installation flow. Prefer direct use of Install's Engine where supported. Confirm the selected Cowork route in the actual destination mode before promoting it as the default. If a desktop extension is necessary, ship it ready to install and investigate a thin launcher targeting Install; do not silently require teachers to maintain two Engines or claim launcher feasibility without checking it.

## Required installation behavior

### 1. Teach the installing agent what "install this" means

Put a short installation trigger near the start of the packaged AGENTS.md, with CLAUDE.md continuing to point to it. Add a bounded installation playbook distinct from normal teacher setup. It must identify this extracted folder as Install and use its bundled Engine even when the installing agent already has another CE-SE MCP connection. Never configure the new teacher against an existing unrelated Install.

The agent should inspect installed client availability and use the teacher's named destination. If destination cannot be inferred, one plain question about Cowork or Work is enough; no transport/path terminology. Do not default to configuring both unrelated clients. Missing one client must not prevent configuring the selected available one. Do not install an agent app or paid subscription implicitly.

### 2. Supply installation mechanics inside the release

Recommended implementation: a small Engine command such as `cse install --client work|cowork`, callable by the local agent, with shared helper functions for a hub connection action. Final naming can follow existing CLI conventions. Its job is path validation, targeted client registration and local diagnostics, not teaching decisions.

Registration must not require `codex` on PATH. Use the supported client integration when available or a format-aware, narrowly scoped update of its configuration. Respect the actual configuration location, including a configured Codex home; do not dump whole client files into agent output because unrelated entries can contain secrets. Preserve other servers/settings, retain a backup before changing existing configuration, and leave malformed configuration intact with an actionable explanation. Re-running the same installation must not create duplicate CE-SE servers. If the CE-SE entry targets another Install, explain that exact replacement before changing it; leave other Installs and their data alone.

The registration points to an absolute local executable path and this Install, with arguments stored separately, including paths with spaces. The ordinary route has one authoritative Engine. Do not build an installer service, automatic updater, tunnel, OAuth system, or arbitrary shell endpoint. Existing supported direct routes remain available during route validation.

Update the package allowlist/assembly so everything this flow invokes ships in the ZIP. Source scripts may remain developer conveniences but cannot be required on a colleague's laptop. The exported README should lead with the user's three steps and provide webui.cmd as a fallback.

### 3. Open the PAT form without waiting for MCP

The installing agent launches the bundled Engine's local UI directly, before any required client restart. Reuse the existing private setup form and credential storage. Prefer opening the setup route on first run and keeping the hub available for course selection afterward. Run the helper without an unsolicited visible terminal window.

The teacher enters the Canvas address and PAT only in the WebUI. The agent never fetches the private form, receives the token, or asks for it in chat/CLI. Expose only a small non-secret setup outcome if the installing agent needs to detect completion. Preserve valid saved credentials on repeat installation and offer the relevant next setup step. Distinguish absent/rejected credentials from connectivity failures; a failed request is not an instruction to replace a PAT.

Keep course selection in the existing UI/conversation. Student notes, calendar and teaching preferences can be offered progressively rather than becoming installation prerequisites. Preserve existing course choices and all unsent work. Keep Sync optional; do not ask a first-time teacher to select three folders.

### 4. Report what was actually checked

Use plain outcomes: Engine starts; connection settings saved; local MCP check passed; destination app still needs restart/check, or destination tools successfully used. A successful local check should initialize the bundled server, discover tools and read AGENTS.md/configured courses without Canvas writes. Reuse the SDK rather than requiring Node or the development smoke script on the teacher's computer.

A local subprocess check is not a destination-client check. After reconnect, the destination agent should read AGENTS.md, its relevant playbook and configured courses, and report that it is ready. Give the teacher one short starter message if the app cannot be opened/prepared through a supported mechanism. Do not invent an app deep link or claim to have created a Teaching Assistant Project when only configuration was written.

Expose loaded adapter build identity separately from the child Engine version so a reconnect recommendation is based on evidence. This is a narrow repair for the already observed stale-runtime problem, not a permanent telemetry/heartbeat system. If a client hides tools or requires a restart, name that next action without returning shell commands to the teacher.

### 5. Carry teaching context into the destination

Keep one authoritative set of teacher files in Install or configured Sync. Avoid copying full playbooks into client project instructions where they can drift. A short project starter should establish the teaching-assistant role and instruct the agent to read current CE-SE instructions and relevant context through MCP. Use supported client APIs if available; otherwise supply the exact short starter as the final handoff, with no technical setup burden.

Offer to carry over teacher-selected existing teaching instructions, or briefly elicit grade/subject, instructional priorities, feedback voice and relevant standards. Preserve that information in plain context and have both scoring and authoring playbooks explicitly consult the relevant preferences. Existing grading preferences remain useful; a small teaching-profile reference can hold broader instructional preferences without introducing an Engine policy schema. Do not require a questionnaire before ordinary work or import this developer's ELA7 tags/classes into a colleague's profile.

The aim is consistent teacher intent across Code/Codex and Cowork/Work. MCP provides access to context; choosing a client mode alone does not demonstrate pedagogical quality. Evaluate one small lesson/feedback example against the teacher's saved preferences during the client pilot.

## Implementation slices and acceptance

1. **Engine/release:** bundled registration and non-secret local diagnostics, direct WebUI launch, repeat-install preservation. Include focused fixture checks for missing CLI, paths with spaces, an existing unrelated server, repeated installation and wrong-Install replacement. Check the selected Cowork route in the real client; choose the supported packaging accordingly.
2. **Playbook/onboarding:** installation trigger/playbook, three-step README, destination starter and progressive teaching-context setup. Keep routine teacher setup separate from development. No additional scoring/math policy belongs in Engine code.
3. **Fresh-user run:** from the release ZIP under a standard Windows account, point Code/Codex at Install and say "install this". No Source or developer runtimes. Teacher supplies PAT through the WebUI; selected destination loads CE-SE instructions/courses and drafts one small task. A synthetic review/revise round trip can check review handoff without Canvas writes. Any real Canvas write needs its own requested scope and ordinary review.

Acceptance is the user's journey, not just passing Go tests. Record remaining app-enforced clicks/restarts honestly. Keep capability data and documentation synchronized with evidence; do not mark this flow delivered until the fresh-user run has happened. No production Install, client registration, credential, or teacher file should be changed merely to execute Source tests.

## Implementation record, 2026-10-12

The user authorized orchestration of this handoff. Three delegated implementation lanes delivered the Engine/release mechanics, hub/setup integration and Playbook/onboarding changes. A separate integration review checked wrong-Install targeting, preservation, secret handling and readiness reporting.

- **Engine/release:** Install's `cse install --client work|cowork` launches the private WebUI independently of destination restart, reports its actual URL/root through a temporary non-secret readiness handshake, registers the selected route and runs SDK MCP initialization/discovery/local instruction/course checks. `--inspect` returns read-only configuration-directory hints, explicitly insufficient to establish installed app availability. Work registration respects `CODEX_HOME`, backs up existing TOML, preserves unrelated settings, is repeatable and requires an explicit `--replace-existing` choice for a different Install. Malformed configuration stays intact.
- **Cowork packaging:** the ZIP includes a ready MCPB with a portable thin launcher and no separate Engine. Cowork preparation binds its manifest to this absolute Install and removes the folder picker. The launcher forwards stdio to Install's Engine. Local launcher/protocol checks can establish mechanics; installation/trust and tool use in an actual Cowork conversation remain unverified. Existing direct stdio configuration remains available. Old extensions carrying a second Engine need explicit replacement in the client.
- **Hub/setup:** an actionable connection page uses the same registration/check functions and offers the prepared extension download and destination starter. Credential validation distinguishes rejected authentication from connectivity failures and preserves the saved credential on failure. `/setup/status` returns only current credential-saved and setup-outcome fields; the agent must never fetch private setup/student pages. Hidden helper launch keeps the UI available after the installing command exits.
- **Playbook/onboarding:** packaged AGENTS routes "install this" to a bounded installation playbook; README leads with the three-step journey. The destination starter points to authoritative current teacher files rather than copying playbooks into client instructions. Optional teaching-profile setup preserves existing preferences; scoring and assignment/quiz/native-content authoring read those preferences. No colleague inherits the developer's ELA7 tag policy.

Remaining acceptance is one standard-account fresh-user ZIP run with Code/Codex installing for a selected actual destination, PAT entry by the teacher in the private WebUI, destination reads of instructions/profile/playbook/configured courses and a teacher-judged small draft. Record genuine app trust clicks, restarts, organizational restrictions and available client mode. Saved settings, a prepared MCPB and local tests must not be reported as destination-client success. No production Install, client settings, teacher files, credential or Canvas content was changed during Source implementation.

### Implementation validation

- PASS: pinned Go 1.27.1 full tests and vet; portable Engine/launcher builds; allowlisted ZIP/MCPB packaging.
- PASS: rebuilt Engine's existing 26-tool compiled stdio smoke check, including Install mismatch rejection, seed preservation, current configured courses, shared rules, files/assets and local validation; no Canvas calls.
- PASS: freshly extracted release in a temporary Install with spaces and isolated synthetic client settings: selected Work registration, original backup/unrelated setting preservation and byte-stable repeat registration; SDK local MCP initialization/discovery/read checks; repeat Cowork extension preparation, manifest absolute Install binding and absence of a second Engine/folder picker.
- PASS: launcher extracted from the bound MCPB initialized Install's Engine, discovered 26 tools and read this Install's configured courses and new teaching profile. This proves launcher mechanics, not Cowork installation or trust.
- Build: `0.1b`, revision `3e05b21ce9af+dirty`, built `2026-10-12T18:35:58Z`. ZIP SHA-256: `a7e2bb620926d4e0bcf24f4408cbac63e026f5f8706c42eafe181c6aa8e75a5f`.
- NOT RUN: actual standard-account installing-agent/browser/PAT journey, selected destination extension adoption/reconnect/tool calls, teacher-judged draft and human review handoff. WebUI handlers/readiness wiring have local checks; no private real-credential form was launched or read. No Canvas requests or writes were performed.

### Original investigation validation

- PASS: pinned Go 1.27.1 tests for `internal/mcpserver`, `internal/localui`, and `internal/workspace` (local fixtures; no Canvas calls).
- PASS: `node scripts/check-mcp.mjs dist/cse.exe` using the existing compiled artifact, a temporary synthetic Install and Sync, and no `--review`: 26 tools, bundled root/mismatch checks, seed preservation, current configured courses, file boundaries, shared rules, references, assets and local validation. This does not establish that dist was rebuilt from this review or validate client registration.
- Inspected registration/packaging, manifests, server/tool/session implementation, hub/setup, template instructions/context and previous MCP evidence. No destination app installation was attempted. The buddy's precise failing client/build remains unspecified; the report establishes the CLI burden, not an exact transport failure.
- No Canvas requests/writes, credential reads, live Install changes or client configuration changes were performed. This handoff and capability evidence notes are the only intended Source changes.

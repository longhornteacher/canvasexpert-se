# CE-SE MCP adapter

Updated 2026-10-11. `cse mcp` runs a local stdio server on the Windows computer containing the local CE-SE installation and saved Canvas credential. Its 26 typed tools cover every currently supported Engine command and the workspace files needed to use those commands. It reuses existing Engine operations; it accepts neither arbitrary shell commands nor arbitrary HTTP writes. Future capabilities are not included in this parity claim.

Teacher-provided run evidence, 2026-10-11: cloud Claude through remote-devices completed teacher-reviewed creation and reported fresh Canvas readbacks for one unpublished assignment and one unpublished MC/TF/ESSAY New Quiz in CS8, plus ordinary student text/history pulls and local canonical scoring validation. This establishes that route's authoring/read workflow, not direct desktop Work/Cowork compatibility, real-student grade/comment posting, student rendering, or persistence across local MCP restarts. The [run review](../handoffs/active/COWORK-RUN-REVIEW-20261011.md) led to locally verified contract and review fixes: routing uses file contents, an optional assignment/results course must match the document, and QuizForge reviews require a course. `validate_quiz` now checks the same settings as review/push. Canonical top-level rationales appear in review; inline item/choice rationales fail with a correction hint, and other unknown item/choice keys warn. Read the supported QuizForge contract and synthetic MC/TF/ESSAY example through `engine_reference` before authoring.

## Tools and workflow

| Tool | Purpose |
|---|---|
| `engine_reference` | Command help, file formats, field limits, synthetic examples. |
| `engine_info` | Version/build information, dummy credential self-test (`doctor`), or executable launcher registration (`link`). |
| `list_courses`, `list_modules` | Read saved course selections by default (`source: configured`); explicit `source: canvas` discovers the live Canvas catalog. Read configured-course modules. |
| `pull_assignment`, `pull_page` | Pull exact content and its edit baseline. |
| `pull_work` | Pull pseudonymized student work/history/attachments, optionally scoped to a course and assignment. |
| `list_workspace_files` | Discover allowed workspace files/directories, optionally recursively. |
| `read_workspace_file` | Read root Markdown/text references (including `AGENTS.md`, `CLAUDE.md`, `README.txt`), `context/`, `playbooks/`, or `courses/` text and its SHA-256. |
| `write_workspace_file` | Save teacher instructions, context, and playbooks with conflict protection. |
| `write_draft` | Save `.md`, `.yaml`, `.yml`, `.json`, or `.txt` course drafts with conflict protection. |
| `read_workspace_asset`, `write_asset` | Read/save base64 course attachments and assets with hashes. |
| `validate_document`, `validate_quiz` | Local Engine document or QuizForge validation. |
| `start_review`, `review_status` | Teacher review, revision notes, and actual posting/creation outcomes. |
| `push_document` | Assignment/Page/module create, resume, update, or assignment/New Quiz rubric attachment as specified by the document. |
| `post_results` | Scoped computed assessment, rubric, feedback, late timing, or selected feedback revision. |
| `push_quiz` | Supported QuizForge New Quiz authoring/resumption, including written items and hub differentiation. |
| `upload_file` | Explicit course/access/folder asset upload with a reusable course receipt and deliberate new-upload option. |
| `list_quiz_points`, `set_quiz_points` | Inspect or change exact decimal New Quiz question points. |
| `canvas_get` | Existing read-only Canvas API GET escape hatch, one page per call. |
| `start_local_ui`, `local_ui_status` | Teacher-only hub, setup, and student editor pages; return URLs/status without private page contents. |

Read saved configuration first with `list_courses`: the default configured source returns JSON `source`, `root`, `instance` and `courses` (including inactive courses, with nickname/id/current/timezone fields and an empty array when none exist). Each call reads the current course file without Canvas credentials or requests. WebUI course saves appear on the next call without restarting MCP. Use `source: canvas` to test the connection or choose additional courses; CLI `courses` remains Canvas catalog discovery. Setup status comes from structured courses, not obsolete starter prose in the preserved document body.

Begin by reading `AGENTS.md` with `read_workspace_file`, then the relevant playbook. Separate folder sharing is unnecessary for these files. The initialization instructions also direct the client to this workflow. Use `engine_reference` for contracts rather than inventing fields.

Review remains the default for scores, new assignments/quizzes, and assignment edits. Start the review, present the loopback URL to the teacher, and check status for approval/outcomes or revision notes. Status includes `state` (`pending`, `finished`, or `failed`); a finished process can report a `partial` or `held` write outcome, so inspect the outcome and readable details. Only pending sessions return a usable review URL. Completion returns the resulting file path and any revision notes; a busy-slot refusal names the active review ID. An MCP permission prompt is not approval on the CE-SE review page. Direct posting/creation tools are available when the teacher explicitly skips review for that task, or when resuming a partly created item as the playbook permits. Pages, uploads, native module assembly, and rubric attachment follow requested scope directly. There is no extra Engine approval token.

Open returned `127.0.0.1` links on the computer running CE-SE. The teacher alone operates setup/student pages, which can contain credentials and real identities; the agent must not fetch or inspect those pages. Review pages display the computed assessment and intended content. MCP process shutdown closes its child pages. A slow-starting page remains discoverable through its status tool.

## Connect desktop Cowork and Work

The agent's synced OneDrive folder permits no `.cmd` files. Keep `cse.cmd` and `webui.cmd` in the local CE-SE installation; MCP needs no launcher in the agent folder. Set the registration's installation path to that local CE-SE folder, never to the agent's sync folder.

From this repository, with an extracted CE-SE installation:

```powershell
.\scripts\connect-mcp.ps1 -Installation 'D:\Teaching\CanvasExpert-SE' -Client Both
```

The script registers the local server through `codex mcp add` for desktop ChatGPT Work and adds the Claude Desktop configuration while preserving other servers/settings. Restart both apps after registration, then check tool discovery and read workspace `AGENTS.md`. The script defaults to the installation's `engine/cse.exe`; `-Engine` remains an explicit development override (`-Workspace` is retained as a parameter alias). The configured executable and installation must remain at their registered paths. Existing clients targeting an old folder require re-registration for the intended installation and restart; this source change does not rewrite live registrations or migrate teacher data.

For manual configuration, use transport **stdio**, command the absolute `cse.exe` path, and arguments `--workspace`, the absolute CE-SE installation path, `mcp`. A bundled installation Engine also accepts just `mcp`, resolving its own installation regardless of the client's working directory. Any explicit root must match that installation. The local adapter uses the machine's existing private Canvas credential; no PAT belongs in client configuration. [Official OpenAI MCP documentation](https://learn.chatgpt.com/docs/extend/mcp) describes direct STDIO support and the shared desktop/Codex configuration.

For Claude's desktop extension installation route, build the optional bundle:

```powershell
.\scripts\package-mcp.ps1
```

Install `dist/CanvasExpert-SE-Cowork.mcpb` in Claude Desktop's extension settings and select the local CE-SE installation folder. The agent's own synced folder is independent. The bundle contains its manifest and portable Engine, not teacher files or credentials. Direct configuration is also available through `connect-mcp.ps1`.

## Hosted ChatGPT and optional tunnel

Hosted ChatGPT Work uses plugins/remote MCP tools rather than reading local Codex configuration. This differs from desktop Work's direct stdio connection. An optional [Secure MCP Tunnel](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels) can connect a local server to supported hosted contexts without a public inbound endpoint. Follow that guide for tunnel/account setup, key handling, and permissions; point its MCP command at `cse.exe --workspace <workspace> mcp`. The tunnel is not required for the two desktop clients. Hosted account/policy availability and tunnel operation are not verified by CE-SE's local tests.

## Data and validation boundaries

Text transfers are limited to 1 MiB. Base64 asset read/write transfers are limited to 20 MiB decoded. Larger files already staged in an allowed workspace folder can still be passed to `upload_file`; its upload is not subject to the base64 transfer limit. Uploads require an explicit reusable receipt path under `courses/`. Existing files require their current SHA-256 before replacement. Recursive listing is bounded to 2,000 entries.

File tools block hidden components, `.cse/`, and `engine/`, including linked paths such as junctions/symlinks beneath the workspace. Reads are limited to the allowed instruction/context/playbook/course roots; course drafts and assets stay under `courses/`. Student writing can itself contain real names. Unlike pseudonymized `pull_work`, `canvas_get` returns raw Canvas JSON and can include real names. Teacher notes intentionally read from context also enter model context; keep them out of student-facing output. The credential and private identity mapping are not file-tool inputs or returned setup/student page contents.

Local verification covers 26-tool registration, scoped files, command arguments, QuizForge validation, and page lifecycle. The compiled Engine passed a real local review/revision round trip without Canvas writes. Cowork desktop initialization/tool discovery and enabled connector were observed; the shared Work/Codex client runtime successfully called workspace reading and Engine version tools. The teacher-provided cloud Claude remote-devices run described above subsequently established reviewed Canvas authoring and ordinary student pulls through that route. Desktop Work pane interactions, direct desktop review/write workflows, optional hosted tunnel, and grade/comment posting through MCP remain unverified. Existing CLI Canvas evidence does not establish those client workflows. Report the Engine's held/partial/uncertain output accurately, preserve receipts and saved IDs, and continue independent items.

Local single-folder checks cover bundled-root selection from another directory, conflicting-root rejection, developer/extension overrides, MCP-first seed preservation and WebUI course saves observed by an already-connected MCP client without Canvas calls. These do not establish the behavior of already-running old binaries or corrected client registrations.

The reproducible compiled smoke check is `node scripts/check-mcp.mjs dist/cse.exe`. For an actual local review/revision check, use `--review` and set `CSE_MCP_CHECK_ORIGIN` to the machine's saved Canvas origin; this requires the existing saved connection but makes no Canvas writes.

Existing installations retain their current `AGENTS.md` and `playbooks/setup.md`, even after an Engine update. To adopt the revised course-discovery instructions, use Settings -> Your playbooks and context -> Get the new version for those files; their previous copies are archived under `playbook-old/`. Review any teacher customizations and carry them forward as needed. No existing instruction or context file is automatically overwritten.

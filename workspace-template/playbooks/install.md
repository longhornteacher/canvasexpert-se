# Install this for the teacher

Use this when the teacher opens the extracted folder in Code/Codex and says "install this", or asks to connect a destination app. Routine Canvas/course setup is in `setup.md`. The extracted folder is **Install**; no Source checkout, developer runtime, separate Codex CLI or teacher terminal command is needed.

## Select the destination and the correct Install

Use the teacher's named Cowork or Work destination; that choice authorizes preparing its selected connection. Run `.\cse.cmd install --inspect` for read-only configuration-directory hints, and use the installing agent's actual app inventory when supported. The helper's directory hints do not establish that an app is installed, running or in a supported destination mode. Do not infer availability or mode from configuration presence, or read whole client configuration files/private settings. If the destination is unclear, ask one plain question: "Will you use this in Cowork or Work?" Configure only that selected client and explain any genuine remaining app action. Do not install an agent app or subscription implicitly. Hosted Work is a separate route; do not infer local stdio support from its name.

Resolve this extracted folder and its bundled `engine/cse.exe`, not an existing CE-SE MCP connection, PATH executable or another teacher folder. Run installation locally from here. An already-connected server may belong to a different Install: it must not receive this teacher's setup or preferences.

## Run the bundled helper

Run `.\cse.cmd install --client work` or `.\cse.cmd install --client cowork` yourself. This saves narrowly scoped connection settings or prepares the Cowork extension, checks the local Engine/MCP without Canvas writes, and opens the private WebUI independently of any destination restart. It needs no `codex` command. Do not add `--no-ui` for ordinary first installation. `--check-only` is a local diagnostic, not registration or proof of destination access.

Inspect the returned outcomes rather than assuming the whole operation succeeded. The helper's `install_root` identifies the extracted folder this operation selected. If an existing CE-SE registration targets another Install, explain the exact old and new folders before using `--replace-existing`; that flag replaces the connection entry, not the old teacher's files. Keep other servers/settings and all unsent work. Malformed client configuration stays intact: explain the helper's reported repair action without exposing unrelated configuration or secrets. An unavailable client does not prevent preparing the selected available route; never silently register both.

## Use the Engine through MCP or its launcher

Playbooks show the CLI form `.\cse.cmd <command>`; run it from Install. When CE-SE MCP is connected, use its typed tools instead: `engine_reference` for command help and exact formats/examples/field behavior; `list_courses`, `pull_work`, `pull_assignment` and `pull_page` for discovery and pulls; `read_workspace_file` / `write_workspace_file` and `write_draft` for files; `validate_document` / `validate_quiz` for local validation; `start_review` / `review_status` for the review page; `push_document`, `push_quiz`, `post_results`, `upload_file`, and quiz-point tools for the corresponding playbook operations; `engine_info` for version, diagnostics and teacher-rules status. Use `list_workspace_files` to find files. The playbook's requested scope and review rules still apply.

Read-only `canvas_get` exposes raw Canvas JSON and can contain real names; prefer the task's pseudonymized pull when it provides the needed evidence. Workspace text reads/writes are limited to 1 MiB; base64 asset reads/writes to 20 MiB decoded. Larger assets already staged under an allowed workspace folder can use `upload_file`. File tools block hidden/private/Engine paths and links beneath the workspace; existing files need their current SHA-256 before replacement. Never use another tool to bypass those boundaries. Setup and student editors are private: give the teacher the returned browser URL and check session status only; never fetch their contents.

For Cowork, present the helper's `extension_path` for the teacher's trusted extension-install action in the app. The extension launches this Install's Engine; the teacher does not maintain a second Engine. A prepared extension is not an installed or connected destination. Report organizational restrictions or trust prompts as the app reports them; do not promise to bypass them. Keep any restart until an active CE-SE review has finished.

## Canvas and teaching context

The teacher enters their Canvas address and PAT only in the private local WebUI. Present its returned address as a fallback if the browser did not open. Never fetch the private form or receive the PAT in chat, a command, a file or client configuration. Inspect only non-secret setup status. Keep a working saved credential on repeat installation. Connection errors alone do not justify replacing it; follow `setup.md` for courses and the distinction between missing/rejected credentials and network/outage problems.

Offer teaching-context onboarding progressively with `setup.md`; keep courses, profile and grading preferences already supplied. Sync is optional and needs no first-run folder questionnaire. Configure a dedicated Sync directory only when the teacher supplies one.

## Hand off honestly

Distinguish: Engine starts; connection settings saved or extension prepared; local MCP check passed; destination still needs a trust action/restart/check; destination tools successfully used. Local checks do not establish destination connectivity, pedagogical quality or Canvas Live behavior. Loaded adapter identity and on-disk Engine identity are separate; if they differ, explain that the destination must reconnect after active review work finishes.

After reconnect, have the selected destination assistant read current `AGENTS.md`, `context/teaching-profile.md`, `context/grading.md`, the relevant playbook and configured `list_courses`. Confirm the returned `install_root` matches the helper's `install_root` and the extracted folder before doing teacher work. If they differ, stop and connect the intended Install. Do not claim a Teaching Assistant Project was created when only connection configuration was saved. Use a supported app action if available; otherwise give the short starter message in `TEACHING-ASSISTANT.md` for the teacher to send. Give only the genuine remaining app action, not terminal commands.

During the destination pilot, draft one small lesson or feedback example against the teacher's saved preferences and let the teacher judge it. This is optional ordinary drafting, not a Canvas write or a prerequisite questionnaire. Report which destination reads and example actually happened; preserve pending fresh-user evidence as pending.

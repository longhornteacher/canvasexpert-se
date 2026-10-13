# Install this for the teacher

Use this when the teacher opens the extracted folder in Code/Codex and says "install this", or asks to connect Claude or ChatGPT. Routine Canvas/course setup is in `setup.md`. The extracted folder is **Install**; no Source checkout, developer runtime, separate Codex CLI or teacher terminal command is needed.

## Select the destination and the correct Install

If the teacher already said Claude, ChatGPT or both (including "install this for Cowork" or "for Work"), use that and do not ask again. Otherwise ask exactly: "Are you using Claude, ChatGPT, or both?" The answer authorizes preparing the selected connection(s). The teacher does not need to know product names such as Cowork, Work or MCP.

You pick the supported route, not the teacher. Internally Claude on this PC uses `--client cowork` and ChatGPT's local desktop app uses `--client work`. Decide from the app inventory and context you can actually see; `.\cse.cmd install --inspect` gives read-only configuration-directory hints, which do not prove an app is installed, running or in a supported mode. Do not read whole client configuration files or private settings. If the evidence is not enough, ask one concrete follow-up, such as "Do you use the app installed on this PC, or only the website?" If the teacher only uses a website, say plainly that CE-SE cannot connect to it from here and offer the supported alternative; never promise connectivity that is not supported. Do not install an app or subscription implicitly.

For **both**, run both helpers (order does not matter) and report each outcome separately, since one can succeed while the other still needs an action. Never register an app the teacher did not choose.

Resolve this extracted folder and its bundled `engine/cse.exe`, not an existing CE-SE MCP connection, PATH executable or another teacher folder. Run installation locally from here. An already-connected server may belong to a different Install: it must not receive this teacher's setup or preferences.

## Run the bundled helper

Run `.\cse.cmd install --client work` (ChatGPT) and/or `.\cse.cmd install --client cowork` (Claude) yourself. This saves narrowly scoped connection settings, or prepares the Claude extension and tries to open it, checks the local Engine/MCP without Canvas writes, and starts the private WebUI independently of any destination restart. A repeat install reuses the same Install's running WebUI. It needs no `codex` command. Do not add `--no-ui` for ordinary first installation. `--check-only` is a local diagnostic, not registration or proof of destination access.

Inspect the returned outcomes rather than assuming the whole operation succeeded. The helper's `install_root` identifies the extracted folder this operation selected. If an existing CE-SE registration targets another Install, explain the exact old and new folders before using `--replace-existing`; that flag replaces the connection entry, not the old teacher's files. Keep other servers/settings and all unsent work. Malformed client configuration stays intact: explain the helper's reported repair action without exposing unrelated configuration or secrets. An unavailable app does not prevent preparing the other selected route; never silently register an app the teacher did not choose.

## Use the Engine through MCP or its launcher

Playbooks show the CLI form `.\cse.cmd <command>`; run it from Install. When CE-SE MCP is connected, use its typed tools instead: `engine_reference` for command help and exact formats/examples/field behavior; `list_courses`, `pull_work`, `pull_assignment` and `pull_page` for discovery and pulls; `read_workspace_file` / `write_workspace_file` and `write_draft` for files; `validate_document` / `validate_quiz` for local validation; `start_review` / `review_status` for the review page; `push_document`, `push_quiz`, `post_results`, `upload_file`, and quiz-point tools for the corresponding playbook operations; `engine_info` for version, diagnostics and teacher-rules status. Use `list_workspace_files` to find files. The playbook's requested scope and review rules still apply.

Read-only `canvas_get` exposes raw Canvas JSON and can contain real names; prefer the task's pseudonymized pull when it provides the needed evidence. Workspace text reads/writes are limited to 1 MiB; base64 asset reads/writes to 20 MiB decoded. Larger assets already staged under an allowed workspace folder can use `upload_file`. File tools block hidden/private/Engine paths and links beneath the workspace; existing files need their current SHA-256 before replacement. Never use another tool to bypass those boundaries. Setup and student editors are private: give the teacher the returned browser URL and check session status only; never fetch their contents.

For Claude, the helper tries to launch the Claude desktop app with the prepared extension file. Read `cowork_launch_outcome` and give the teacher the one real next step:

- `launch_requested`: Claude was asked to open it. Ask the teacher to approve Claude's install prompt; if none appears, give the manual steps below.
- `file_revealed`: the file is shown in File Explorer. Tell the teacher to open Claude, go to its extensions settings and install that file.
- `launch_failed` or `prepared`: the file is ready at `extension_path`; give the same manual steps and any error the helper reported.

None of these means the extension is installed, trusted or connected. The extension launches this Install's Engine; the teacher does not maintain a second Engine. Report organizational restrictions or trust prompts as the app reports them; do not promise to bypass them. Do not restart an app during an active CE-SE review; wait until it finishes.

## Canvas and teaching context

The teacher enters their Canvas address and PAT only in the private local WebUI. Always show the helper's actual local form address as a fallback, since a browser launch was only requested, not seen. If the helper returns a `ui_warning`, the page may not have started: tell the teacher to double-click `webui.cmd` in the folder, and do not invent an address. A failed WebUI start does not undo saved connection settings or a prepared extension; report those outcomes separately. Never fetch the private form or receive the PAT in chat, a command, a file or client configuration. Inspect only non-secret setup status. Keep a working saved credential on repeat installation.

Tell three problems apart: a rejected or missing token (the teacher re-enters it in the form); a connectivity problem such as a timeout, DNS, proxy or outage (keep the token, retry later or work offline); and a Canvas address mismatch, where the saved address and the course configuration name different Canvas origins (show both origins from the message and let the teacher confirm which is right). `setup.md` covers courses.

Offer teaching-context onboarding progressively with `setup.md`; keep courses, profile and grading preferences already supplied, and ask for one useful detail at a time. Sync is optional and needs no first-run folder questionnaire. Configure a dedicated Sync directory only when the teacher supplies one.

## Hand off honestly

Distinguish: Engine starts; connection settings saved or extension prepared; local MCP check passed; WebUI started; destination still needs a trust action/restart/check; destination tools successfully used. Saved settings and local checks are not connected tools. Say it plainly, for example: "Claude is set up and waiting for you to approve its install prompt; ChatGPT needs a restart after your review finishes." Local checks do not establish destination connectivity, pedagogical quality or Canvas Live behavior. Loaded adapter identity and on-disk Engine identity are separate; if they differ, explain that the destination must reconnect after active review work finishes.

After reconnect, have the selected destination assistant read current `AGENTS.md`, `context/teaching-profile.md`, `context/grading.md`, the relevant playbook and configured `list_courses`. Confirm the returned `install_root` matches the helper's `install_root` and the extracted folder before doing teacher work. If they differ, stop and connect the intended Install. Do not claim a Teaching Assistant Project was created when only connection configuration was saved. Use a supported app action if available; otherwise give the short starter message in `TEACHING-ASSISTANT.md` for the teacher to send. Give only the genuine remaining app action, one short step at a time, not terminal commands. For both apps, check each separately; an app counts as connected only after its assistant reads the instructions, preferences and configured courses and reports the matching `install_root`.

During the destination pilot, draft one small lesson or feedback example against the teacher's saved preferences and let the teacher judge it. This is optional ordinary drafting, not a Canvas write or a prerequisite questionnaire. Report which destination reads and example actually happened; preserve pending fresh-user evidence as pending.

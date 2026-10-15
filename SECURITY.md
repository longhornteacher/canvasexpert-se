# Security and data handling

CanvasExpert-SE is an independent, locally installed tool. It uses a teacher's Canvas access and the AI client the teacher chooses. An open-source license is permission to use the code; it does not establish school approval or certification for handling student records.

## What goes where

| Information | Location and access |
| --- | --- |
| Canvas address and PAT | Windows Credential Manager on the local computer. Enter the PAT only in the private local setup form. It is not placed in client registration, workspace files or agent output. |
| Student identity mapping, roster and private contacts | The active setup authority's `.cse/` directory: UserInstall by default, configured UserSync otherwise. Excluded from ordinary MCP file tools. |
| Machine and session runtime state | Local UserInstall; credentials remain in Windows Credential Manager. |
| Pulled student work, attachments and drafts | Local UserInstall. Task evidence read by an AI assistant enters that client's context and is handled under that provider's settings and policies. |
| Reusable teacher setup | UserInstall by default; optional configured UserSync for instructions, preferences, course selections and student aliases/settings/notes. The selected cloud storage provider synchronizes UserSync; grading/planning work stays local. |
| Client connection settings | The selected client configuration or locally prepared extension. These reference UserInstall's Engine and contain no Canvas PAT. |

Pseudonymous student references reduce routine identity exposure. They do not make all material anonymous: student writing, attachments, teacher context and read-only Canvas API responses can include real names. Use an AI account and storage arrangements approved for the data you will handle. Private setup and student-editor pages are for the teacher, not the agent.

The Engine exposes bounded MCP tools, not an arbitrary shell or HTTP-write endpoint. Review is the playbook default for grading and assignment/quiz creation or edits; a teacher can explicitly skip review for a task. This is an instructional workflow, not an access-control sandbox. A local agent with filesystem or shell access has permissions outside MCP's file boundaries.

## Reporting a vulnerability

Do not post PATs, student records, private teacher notes, production configurations or sensitive screenshots in a public issue.

If GitHub shows **Report a vulnerability** on this repository's Security page, use that private channel. If it is unavailable, ask the maintainer for a private reporting channel without disclosing exploit details or private data. No private reporting service or response-time guarantee is implied by this document.

Include the Engine version and revision, affected workflow, a minimal synthetic reproduction, expected impact and any workaround. Revoke a disclosed PAT in Canvas promptly; deleting it from a GitHub issue or local file alone does not revoke it.

## Supported versions

The current 1.0 beta line is the active development target. Older experimental builds have no promised security maintenance. Report issues against the latest beta where practical, while preserving your existing UserInstall and unsent work. See [the release policy](docs/releasing.md) for version identity and updates.

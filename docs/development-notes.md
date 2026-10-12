# Development notes

Practical notes for working in this checkout. Authority stays with the root `AGENTS.md`, `ARCHITECTURE.md` and `SCORING-RULES.md`.

This checkout is **Source**. **Install** is the local runnable CE-SE folder and the teacher's actual files. **Sync** is optional teacher/agent cloud storage, independent of Install; CE-SE does not automatically synchronize them. Development agents use Source for product work and Install for teacher tasks. MCP workspace paths refer to Install.

## Checkout and shell

- The working copy uses **CRLF** in many files, so `gofmt -l` lists nearly everything. Format only files you change, and preserve line endings (use the Edit tool or CRLF-aware scripts).
- Git Bash rewrites leading `/api/...` paths; set `MSYS_NO_PATHCONV=1`. Long heredocs with quotes can break the Bash tool; write scripts to files instead.

## Live Canvas reads

- Ask the teacher for fresh, concrete authorization before any Canvas write. Authorization from an earlier session does not carry over.
- For live reads without printing the token, use a **temporary** `cmd/tmpdiag` program that calls `platform.LoadCredential()` and prints status and body only; delete it afterwards. Never print or log the token or student names.
- Probe through `cse` and `cse-probe`, not the original `canvas-expert` MCP.
- Disposable objects left in CS8 are listed in [cs8-test-objects.md](cs8-test-objects.md).

## Docs that change together

`internal/localui/capabilities.go` (status data and review date), [engine-capabilities.md](engine-capabilities.md) (evidence), and `engine-docs/formats.md` (agent-facing formats). Keep Canvas Live evidence, local tests and pending human checks distinct.

## Engine notes from teachers

Workspaces collect Engine gaps in `context/engine-notes.md` (written by the agent when it had to work around the Engine). Ask the teacher for that list when planning; turn real gaps into handoffs and have the teacher delete the fixed lines.

## Tests

- `workflowFixture` (httptest TLS) in `internal/workflow/workflow_test.go`.
- `fakeHubCanvas` in `internal/workflow/hub_test.go` is a reusable stateful fake for pages and tags.

## Quizzes and Hub: prior art

Check the original CanvasExpert (`D:\Development Projects\CanvasExpert`) before probing Canvas again: `api/transform.py` (New Quiz item shapes), `api/qf_pusher.py` (settings), `api/operation_ledger/adapters/tier_pages.py`, `docs/reference/assignment-differentiation-design.md`, `api/README.md` "Confirmed Canvas API facts", and `api/qf_materials/qf quiz examples/` (fixtures such as `all_types_sampler.txt`). Port behaviour leanly, not code.

Gotchas: Canvas returns New Quiz IDs as strings (decode with `json.Number`); numeric items need `Numeric` with a value array; `rich-fill-blank` needs `edit_distance >= 1`; essay and file-upload items need `scoring_data {"value": ""}`. QTI ZIP import through content migrations (`import_quizzes_next`) reports success but creates an empty New Quiz; do not rebuild it without new evidence.

The Hub page rule (create unpublished, restrict to tag overrides, re-read, then publish) and the quiz Hub write order are described in `engine-docs/formats.md` and the code in `internal/workflow/hub.go`.

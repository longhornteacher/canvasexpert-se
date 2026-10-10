# Quizzes and Hub differentiation: handoff to Codex

2026-10-09. Everything below is **Engine** work (Go) plus the matching **Playbook** text. Read `AGENTS.md` and `ARCHITECTURE.md` first; this file does not override them. Build state at handoff: `go build ./... && go vet ./cmd/... ./internal/... && go test ./cmd/... ./internal/...` all pass.

## What exists now

| Command / feature | Where | Status |
|---|---|---|
| `cse push-quiz COURSE FILE`, New Quiz (default) | `internal/workflow/quiz_author.go`, `quizforge.go` | Live verified in CS8 (create, read-back, resume, repeat, settings, publication, module) |
| Code highlighting and TEKS labels on prompts | `internal/workflow/quiz_format.go` | Live verified (Canvas keeps the inline styles) |
| Hub differentiation core | `internal/workflow/hub.go` (`DeliverTierPages`, `BindTierLinks`, `HubSummary`), `internal/domain/tiers.go` | Live verified in CS8 for assignments (Canvas read-back only) |
| Hub tiers on assignments | `tiers:` in the create document; `internal/workflow/author.go`, `internal/domain/*` | Live verified |
| Markdown links for hub pages | `internal/render/markdown.go` | Tested; exercised live through the hub |

Input format: the **QuizForge v3.0-json** file (`<QUIZFORGE_JSON>` envelope or bare JSON) from `D:\Development Projects\QuizForge`. CE-SE ports behaviour, not code, from the original Python CanvasExpert at `D:\Development Projects\CanvasExpert` (see "Prior art" below). Format reference for agents: `.\cse.cmd help formats` (`engine-docs/formats.md`, sections "Quiz file (QuizForge)", "Differentiated hub (quizzes)", "Differentiated hub (assignments)"). Playbook text: `workspace-template/playbooks/quizzes.md` and the hub section of `authoring.md`.

## Decisions the user made (do not relitigate)

- **Hub only.** The Bridge style (separate tier quizzes/assignments plus a gradebook-only bridge) is rejected: it was painful in the gradebook. `differentiation: "bridge"` is refused with a message.
- Hub is for **New Quizzes** and **assignments** (New Quiz hub: 2026-10-10; local tests only), linked from the quiz instructions.
- New Quizzes accept ESSAY/FILEUPLOAD now (2026-10-10; local tests only; the teacher grades them in SpeedGrader).
- Monokai code highlighting is for **code blocks only** (`<pre><code>`); a block with `class="language-xyz"` for a non-Python language keeps the dark box but is not colored. Prose, poetry `<div>`s and inline `<code>` are never touched.
- Port the original **leanly**: read its verified findings, rewrite the behaviour in Go, no copy-paste, no runtime link-back, none of its ledger/mirror/checkpoint machinery. The highlighter is about 80 lines of Go with no new dependency.
- Tags: ELA7 only, Silver/Red/Blue = Support/Core/Accelerate, teacher-managed in Canvas. Shared hints for everyone belong in the hub item itself.
- Every scored quiz item needs explicit `points`. The Engine never invents points (deliberate difference from the QuizForge contract).

## Design in brief

- **Resume model.** Quiz IDs live in a sidecar `FILE.cse.json` beside the QuizForge file (`{assignment_id}`, plus `tier_pages` for a hub). The QuizForge file itself is never rewritten. A rerun adds only missing items. With no sidecar, a same-titled quiz blocks creation and the error prints the exact sidecar to write.
- **Canvas returns quiz IDs as strings** (`"3698115"`). Decode with `json.Number`. This bug caused a false "uncertain" outcome on the first live run.
- **New Quiz settings and publication** come from `metadata.canvas` (unknown keys rejected). Result-view settings are always spelled out: unset means students see no feedback.
- **Hub page law** (verified live): create page unpublished; PUT `date_details` with `only_visible_to_overrides` and the tag group (or no group); re-read and require `visible_to_everyone == false`; only then publish; re-read; if ever visible to everyone, unpublish and fail. Tags are non-collaborative groups matched by exact case-insensitive name; membership is never read. A missing tag leaves the page restricted to nobody and prints a teacher action. A same-title page that the document did not create blocks the push before any write. Pages publish only when the hub item is published.
- **Quiz hub order:** deliver pages (unpublished) → bind `{{tier:LABEL}}` links into `instructions` → create the quiz → add questions → final step re-delivers pages with `publish = quiz published`. The sidecar is re-read before writing quiz IDs, because page delivery records pages in it (an early version overwrote them; a test now guards this).
- **Assignment hub:** tier pages are recorded in `created.tier_pages` in the assignment document, so an interrupted push resumes without duplicate pages and the bound HTML stays identical for the existing change check.

## Live evidence (CS8 course 121046, 2026-10-09)

The user authorized creating and deleting **only content created by the session** in CS8, using the isolated workspace `.test-workspaces/engine-unattended-20261009/` (git-ignored). **That authorization belonged to the previous session. Ask the user for fresh, concrete authorization before any Canvas write.** All test content from these runs was deleted; one New Quiz was deliberately left for the user to preview: quiz/assignment **3698128**, "All Question Types Sampler (CS)" (unpublished). The class quizzes in CS8 were never touched.

Verified by reading Canvas back: all 8 New Quiz item types from the original `all_types_sampler.txt` with authored points, feedback and result-view settings; settings quiz with publication, due date, `post_to_sis` false, time limit, two attempts and module 636143; hub pages restricted-then-published for assignments, with a matched tag; collision refusal with no writes; collaborative group refused by Canvas with 422.

Tried and **removed**: importing a QuizForge QTI ZIP through content migrations (`import_quizzes_next`). Canvas reported success with no issues but created an empty New Quiz, even after several minutes. Do not rebuild it without new evidence.

## Not verified / known gaps

- **Student view of anything.** New Quizzes cannot open as the built-in Test Student (LTI); use the editor's Build → Preview or a real enrolled account. Hub pages were not checked from a student account either. This is the Yellow item "Student experience of quizzes and hub pages" on the Capabilities page.
- Local-test-only: ambiguous-tag handling, fail-closed unpublish.
- Not supported: images in questions, module/group by name, adding tiers to an **existing** assignment or quiz, publishing hub pages later when the hub is published afterwards (the Engine prints a note; nothing republishes them), the original's TEKS coverage report, differentiated quiz families beyond Hub.

## Suggested next work (ask the user which)

1. A **converter from existing QuizForge question banks** is already the input format, so the likelier need is validating real banks (`D:\Development Projects\QuizForge\Finished_Exports`, `DropZone`) through `push-quiz` in a disposable course.
2. Images in questions; hub tiers for existing content.
3. A human student-view check (above), then update the Capabilities page and `docs/engine-capabilities.md` together.

## Prior art (check before probing Canvas again)

`D:\Development Projects\CanvasExpert`: `api/transform.py` (New Quiz item shapes), `api/qf_pusher.py` (settings), `api/operation_ledger/adapters/tier_pages.py`, `docs/reference/assignment-differentiation-design.md`, `api/README.md` "Confirmed Canvas API facts", `api/qf_materials/qf quiz examples/` (fixtures such as `all_types_sampler.txt`). Gotchas worth remembering: numeric needs `Numeric` with a value array; `rich-fill-blank` needs `edit_distance ≥ 1`.

## Working notes for this checkout

- The working copy uses **CRLF** in many files, so `gofmt -l` lists nearly everything; only format files you change, and preserve line endings (use the Edit tool or CRLF-aware scripts).
- Git Bash rewrites leading `/api/...` paths; set `MSYS_NO_PATHCONV=1`. Long heredocs with quotes can break the Bash tool; write scripts to files instead.
- For live reads without printing the token, use a **temporary** `cmd/tmpdiag` program that calls `platform.LoadCredential()` and prints status and body only; delete it afterwards. Never print or log the token.
- Docs live in three places that must change together: `internal/localui/capabilities.go` (status data and review date), `docs/engine-capabilities.md` (evidence), and `engine-docs/formats.md` (agent-facing formats). Keep Canvas Live evidence, local tests and pending human checks distinct (see `AGENTS.md`).
- Tests: `workflowFixture` (httptest TLS) in `internal/workflow/workflow_test.go`; `fakeHubCanvas` in `hub_test.go` is a reusable stateful fake for pages and tags.
- The commit that accompanies this handoff also contains uncommitted work the user had in the tree before this session (`AGENTS.md`, `docs/build.md`, localui hub/theme edits, `score.md` lateness text, the first `capabilities.go`). It was committed together because the Capabilities page changes build on it.

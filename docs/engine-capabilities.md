# Engine capability record

The WebUI **Capabilities** page (`/capabilities`, data in [`internal/localui/capabilities.go`](../internal/localui/capabilities.go)) is the status view; this file is the evidence behind it, and future scope is in [ARCHITECTURE.md](../ARCHITECTURE.md). Update the page and this file together, with the review date. Last reviewed: **2026-10-13**.

The page's lights describe recorded evidence, not real-time Canvas health. Keep three kinds of evidence distinct and never present one as another:

- **Canvas Live**: observed against a real Canvas (course CS8, 121046, unless noted).
- **Local tests**: Go tests, fakes, temporary fixtures, synthetic browser pages. They prove code behavior, not Canvas or client behavior.
- **Pending human validation**: needs a teacher, a real client, or a student view. A pending item is an evidence limit, not a prerequisite for ordinary supported use.

Dated narrative history (build hashes, per-run notes, probe reports) is in Git history; this file keeps the current state. Disposable CS8 objects are listed in [cs8-test-objects.md](cs8-test-objects.md). Per-field Canvas behavior is in [`engine-docs/field-limits.md`](../engine-docs/field-limits.md).

## Current build

One current build: `scripts/package.ps1` publishes `dist/CanvasExpert-SE.zip` (see [build.md](build.md)). Version `1.0.0-beta.1`, revision `6884d5199567+dirty`, built `2026-10-13T14:49:44Z`, development. ZIP SHA-256 `1cd2501b2783e23d7f1d113151e5bfb1b5e2eac47726d08ec582ddcdc449831b`. Full tests/vet and the compiled 26-tool MCP check passed; the ZIP's Engine is byte-equal to `dist/cse.exe`. A temporary Install with spaces in its path passed Work registration (isolated `CODEX_HOME`), repeat, the local MCP check and offline doctor. The installer's browser/WebUI launch was **not** run from the package. This establishes a local build, not client or Canvas behavior.

## Capabilities and evidence

| Capability | Canvas Live | Local tests / other | Still pending |
|---|---|---|---|
| Course and module reads, `cse api` (read-only GET) | Yes: CS8 and two curriculum courses; modules, assignments, file metadata, exports, Classic and New Quiz items | Error handling | Complete archival, binary downloads, vendor content access |
| Assignment create/edit/publish, module placement, resume | Yes (2026-10-09, CS8 Test Student and new objects): unpublished create, edits, publication, safety settings, repeat made 0 requests | Conflict and uncertain-write holds | Ordinary student submission/resubmission, student-facing view |
| Rubric create/attach (assignments) | Yes: numeric anchors, independent grading, coupled rubric+grade posting, repeat made 0 requests | Full-definition coupling update | None specific |
| Honest grade/feedback posting | Yes (Oct 9 probes: computed grade replaced planted grade, revised feedback with no duplicates, private sentinel absent; Oct 11 real use: SCR 4/5/6 posted via review, SCR 4 confirmed present, SCR 5/6 not re-read) | Accepted-write rule (an accepted Canvas write counts as done) | Student-facing presentation; current MCP scoring/post workflow with real students |
| Lateness and attempts | Yes: supplied seconds stored, Canvas deducted points, changed due date held timing only, stale-assessment guards | Calendar judgments belong to the agent; the Engine does no penalty math | None specific |
| Files: upload, download | Yes: synthetic text and DOCX byte-exact | `upload-file` receipts, access modes, credential isolation | Live Engine course-file uploads, image/download access, MP4/VTT |
| Native Pages, Modules, HTML assignments | No Engine write/readback yet | `pull-page`, page/module push, ordered placement, resume, unsent-draft preservation | Live write/readback, teacher rendering, student access |
| New Quiz authoring (`push-quiz`) | Yes (2026-10-09/10): all item types, settings, module placement, ESSAY/FILEUPLOAD, rubric attach (`pull-item` then `push attach-rubric`), `quiz-points` listing | Item mapping tests, Hub-on-New-Quizzes fake Canvas, `quiz-points` PATCH shape | Student attempt view, word-limit/file-type enforcement, Hub-on-New-Quizzes live run |
| New Quiz written-work scoring | Hand-run only: typed response, byte-exact DOCX, one comment post (CS8 quiz 3698192) | Typed-response and file pulls; comment-only results (no grade, rubric or late write); contract in [new-quiz-written-work-spec.md](new-quiz-written-work-spec.md) | Live Engine pull/download/comment path. The teacher enters item points in SpeedGrader |
| Differentiated Hub (assignments) | Yes (2026-10-09, content deleted afterward): restricted pages, tag assignment, fail-closed unpublish | Ambiguous-tag and rerun tests | Any student's view of restricted pages |
| Review page (`cse review`) | Yes: review, revise, re-review and post in a browser pane on a disposable assignment (2026-10-10) | Cards, held reasons, comment-only pages, light/dark layouts | Installed-client review, multi-student scoring/posting, student rendering |
| Local MCP adapter (26 tools) | Oct 11 cloud Claude run: teacher-reviewed creation and readbacks, student text/history pulls | Tool discovery, guarded files/assets, validation, doctor, compiled stdio smoke check | Desktop Work/Cowork write workflows, persistence across restarts, real-student posting via MCP |
| Shared teacher rules (optional Sync) | No | Two Installs sharing one directory, stale-hash rejection, return to local, seed protection | Real provider propagation and human-client adoption |
| WebUI hub (Home, Courses, Students, Settings, Capabilities, Canvas health) | One read-only identity check showed Connected | Page registry, stable health keys, buffered render, drafts, Browse/Test, synthetic browser inspection | Rejected and unavailable states against real Canvas |
| Setup and connection (Install + MCP) | A teacher-run fresh laptop pilot (2026-10-13) reached the credential form, a Claude install dialog (by launching Claude with the file) and a live course listing after manual origin correction | See below | Claude trust/extension install; ChatGPT/Work restart and tool calls reporting the intended Install root; installer WebUI launch on a fresh machine |

## First-install repairs (2026-10-13, local tests only)

After the pilot, Source: starts the installer WebUI through the normal same-Install lock/reuse path and reports a `ui_warning` with a `webui.cmd` fallback on failure while keeping registration and check results; launches the Claude desktop app directly with the extension file, else reveals the file with manual steps (`launch_requested`, `file_revealed`, `launch_failed`, `prepared`; none means installed or connected); compares origins case-insensitively and names both in mismatch messages; accepts bare hosts or pasted HTTPS links as the Canvas address; compacts `child_engine_version`; lists real help topics; and asks teachers "Are you using Claude, ChatGPT, or both?".

Not established: why the pilot laptop kept the starter origin (hub setup callbacks with mocked validation wrote it correctly locally); and a PowerShell 5.1 `help setup` exit 255 (the exit was 0 in a controlled run, though `2>&1` with `$ErrorActionPreference='Stop'` throws on any native stderr line even at exit 0).

## Classification notes

- Classic Quizzes: retired; not supported.
- New Quiz written-work scoring writes no rubric assessment or item grade; the teacher enters item points in SpeedGrader. Rubric attachment to a New Quiz is green on the 2026-10-10 live runs.
- The Oct 9 unattended evidence for plain, rubric and file scoring is Canvas Live on that build; the current accepted-write semantics are covered by local tests only.
- A Canvas Live leftover remains in CS8: file 19222000 (`cse-synthetic-browser-upload.txt`), which Canvas refused to delete.
- **Source** is development code, **Install** is the local runnable Engine and teacher files, and **Sync** is an optional teacher-chosen cloud folder with no automatic CE-SE synchronization.

> **Retired 2026-10-13.** Historical instructions and evidence; do not execute this plan. Today's scope and dispositions are in [01 — Handoff cleanup](01-HANDOFF-CLEANUP.md). Retirement does not establish completion or new validation; preserve the dated evidence and teacher work described below.

# Canvas-native curriculum authoring

Status: active implementation handoff. Written 2026-10-10.

Implementation update 2026-10-10: the local Engine slice is implemented with Sol (Pages/rich bodies and uploads) and Luna (modules), integrated and reviewed by the primary agent. `push` routes assignment/Page/module documents; `pull-page` preserves live HTML and a baseline; `upload-file` uses course uploads and durable asset receipts. Formats, synthetic examples, the native-content playbook, architecture and capability records are updated. The pinned Go 1.27.1 build script passed `go test ./...`, `go vet ./...` and the portable Windows build; focused regressions cover normalization, conflicts, returned IDs, credential isolation, course identity and resumed placement/publication. No live Canvas content writes were made. This handoff remains active for the explicitly scoped live trial below; API readback, teacher rendering, student access and video/caption playback are not established by local tests.

## Teacher direction and outcome

The teacher wants to archive two supplied curriculum courses and later repurpose selected material into Canvas-native Pages, quizzes, assignments, files and modules. Students should complete the resulting instruction and assessment in Canvas rather than launch CompuScholar. Implement the missing CE-SE authoring primitives so an agent can do this with ordinary local documents and the existing Engine. This handoff authorizes development planning and implementation; it does not select a destination course or authorize publishing converted curriculum.

Source courses are 102279 (Tech Essentials - Tech Apps) and 102278 (Computer Science Foundations Course) at pearland.instructure.com. These IDs identify archival sources, **not write destinations**. Do not assume CS8 is the destination either. Before a live authoring trial, obtain the teacher's exact destination and scope; use synthetic unpublished content there unless real content was requested.

The curriculum archive and its capture tools belong outside this repository:

- `D:\Documents\OneDrive - Pearland ISD\Computer Science\archived_courses\ARCHIVE-EXECUTION-PLAN.md`
- `_capture-test\README.md`, `_capture-test\CONVERSION-NOTES.md` and `_capture-test\tools\` beneath that directory.

Read those reports for concrete examples. Do not copy the vendor curriculum or archive helper into the product. Root `AGENTS.md`, `ARCHITECTURE.md` and `SCORING-RULES.md` remain development authority.

## Evidence and actual gaps

The bounded capture trial obtained lessons, guides, answer-key PDFs, activity directions, solutions, video files and supporting assets through normal authenticated LTI launches. It also captured all three panels of one Python homework. Full archival has not run. Vendor interactive execution and hidden grading logic were not captured. One sampled Foundations video had unavailable caption URLs. These are conversion facts, not reasons to build a vendor subsystem into the Engine.

Existing code provides:

- `cse api PATH`: authenticated read-only JSON GET, one page per call. The client already has paginated `List`/`ListField` and a binary `Download` method.
- Assignment create/edit, rubrics, New Quiz authoring and placement of those newly authored objects into an existing module.
- Page creation for differentiated hubs in `internal/workflow/hub.go`. Those Pages are deliberately restricted to differentiation tags and never placed in modules. They are not a general Page authoring command.
- A disposable synthetic submission upload helper in `cmd/cse-probe/file_upload.go`. It is not a course asset upload operation.

The current assignment-create path renders a small Markdown subset. `internal/render/markdown.go` escapes raw HTML and lacks the images/tables/media needed by these captured lessons. Existing assignment edits preserve live HTML. Do not claim rich curriculum authoring works merely because Canvas itself accepts HTML.

## Required Engine work, in execution order

### 1. Standalone Page documents and rich teaching bodies

Add a first-class `kind: page` document using the existing YAML-header/body file convention, with create, pull and update operations. Prefer `cse push FILE` dispatch by document kind and a distinct `cse pull-page COURSE ID|LINK` command; keep `pull-item`'s assignment contract intact. Final command spelling may follow repository conventions, but document one clear supported path.

Contract:

- Explicit course nickname, persisted Canvas origin/course ID, title, body format, body, requested publication and optional existing module ID/position.
- Record returned numeric Page ID, actual slug and URL immediately. Use returned values rather than predicting a slug from the title.
- Create unpublished, finish requested placement, then publish only if requested. A saved create record resumes by exact ID; it must not create another Page on rerun. An uncertain create is reported for bounded live lookup, never blindly replayed.
- Pull exact live HTML and editable fields with a baseline. Updates patch only requested fields after checking relevant baseline fields; preserve local unsent changes on conflict. Follow existing assignment-edit patterns without a universal edit framework.
- Ordinary Pages must not call the hub's tag restriction or assignment-override logic. Preserve all current hub behavior, including restricted visibility and absence from modules. Do not expose teacher keys/solutions by treating them as ordinary lesson bodies.
- Do not add deletion, front-page selection, notifications, scheduling or differentiation controls to this slice.

Provide an explicit `body_format: markdown|html` contract for Page creation/update. Add the same optional format to assignment **creation** so activity directions can contain tables, images and Canvas file links. Missing format preserves today's Markdown create behavior; existing assignment update documents retain their current HTML meaning. Reject unsupported format values at document validation, not deep in the HTTP path.

HTML is agent-authored teaching content, not a vendor application wrapper: no copied scripts, launch forms, signed URLs or embedded vendor exercises. Use Canvas's actual saved HTML as evidence of normalization. Support ordinary headings, lists, code, tables, links and images through this explicit HTML path; do not write a new Markdown parser or speculative HTML allowlist. Local review must treat raw HTML safely, preserving the review UI's existing security boundaries. Escaped source plus a Canvas preview link is acceptable initially when safe local rendering is unavailable; do not silently show a misleading preview.

### 2. Course asset uploads

Add a supported course-file upload operation, such as `cse upload-file COURSE FILE`, with a small local result/receipt that later authoring steps can read. Resolve the course through the normal workspace/credential spine. Use the official Canvas course upload workflow, not the submission endpoint in the probe helper.

- Upload a local file into the selected course, optionally a specified existing folder. Support the ordinary files found in the trial: images, PDFs, spreadsheet/code downloads, MP4 and VTT. Report actual Canvas size/type or upload rejection; do not preemptively build media transcoding or Canvas Studio integration.
- Follow Canvas's initiation, multipart storage transfer and completion response/redirect flow. The API PAT goes only to the configured Canvas origin, never to a returned storage host. Consult current official upload documentation before implementing this transport. Reuse suitable mechanics from `sendProbeFile`, not its impersonation/submission/probe-state code.
- Do not overwrite an existing course file automatically. Request the documented non-overwriting collision behavior and record the actual returned filename and ID. Record origin/course, source path, digest, file ID, stable course file link/API reference and availability state. Persist returned IDs before downstream steps; rerun with an unchanged known receipt verifies/reuses that file. Changed bytes require a new explicit upload rather than replacing a file used by live content.
- Do not persist upload URLs, upload parameters, PATs or signed download URLs. Do not automatically replay an uncertain upload; explain how to identify the resulting file or retry deliberately.
- Make file access settings explicit and report them. An unpublished Page alone does not make its uploaded assets private. Teacher keys and solution assets stay in the external archive unless the teacher explicitly requests an appropriate Canvas destination/access scope.

Give the agent documented Canvas link/image examples based on returned file IDs for Page and assignment HTML, and supported New Quiz item content. No general cross-document token/resolver system is required: upload first, use receipts to author real links, then push content. Retain provenance and local-to-Canvas asset mappings in the teacher's conversion workspace.

An MP4 upload is not proof of a functioning Canvas lesson video. Verify the saved Page's player, course file access and captions in the live browser before advertising embedded playback. If Canvas strips the proposed media markup or captions do not work, provide a native course-file link and report the limitation; leave Studio/transcoding and richer media support for an actual demonstrated need. Missing source captions remain a reported source gap.

### 3. Module assembly

Add bounded module operations to create an unpublished module and place verified existing native content into it. The archive has hundreds of module items; clicking each Page/file into a module would materially defeat the requested workflow. Keep this a direct module authoring operation, not a course migration engine.

Prefer a `kind: module` document pushed through the existing CLI, recording the exact module ID and returned item IDs. Allow an explicit existing module ID as an alternative to creation. Support an ordered list of Page, File, Assignment (including the assignment ID returned for a New Quiz), and SubHeader entries. Use Canvas's Page slug field for Page items and verified content IDs for File/Assignment items. Reuse the existing placement code where practical, preserving assignment/quiz behavior.

- Check every module/content reference against the selected live course before inserting it. Do not infer ownership from a bare ID or match an existing module by title.
- Preserve requested order and record each successful placement immediately. Resume by recorded item ID and content identity, avoiding duplicate insertion on ordinary reruns. Stop/report an uncertain item write rather than replaying it. Continue independent work outside the held item when possible.
- For an existing module, add only explicitly requested entries/positions; do not delete or replace existing teacher structure. Do not implement synchronization, prerequisite logic or completion requirements here.
- Module publication is separately requested and does not implicitly publish its children. Report any unpublished/inaccessible child. A single module's partial result must show which objects exist and what remains, without rolling back unrelated successes.

## Playbooks and documentation work

Add a concise native-content authoring playbook to `workspace-template/playbooks/` or extend existing authoring/quizzes playbooks with links to it. It should tell an agent to:

1. Read archive provenance and select material with the teacher; choose the explicit destination course.
2. Separate student instruction from teacher guides/keys and preserve source discrepancies in conversion notes.
3. Upload student assets, author Pages/activity assignments and native quizzes, then assemble modules using returned IDs.
4. Reuse existing New Quiz/assignment contracts. Decide pedagogical mappings in the playbook: matching/order interactions become appropriate supported quiz questions; Python code work becomes a native code/text/file submission with a rubric or teacher grading. Do not claim Canvas replaced vendor execution or hidden autograding. Classic source questions may be converted to supported New Quiz types; unsupported items need an explicit adaptation or report, not automatic dropping.
5. Verify saved HTML, links, assessment keys/points, file access and module order in Canvas. Distinguish teacher-view checks from actual student-view checks. Use existing review policy for assignments/quizzes; page edits remain outside that gate. Do not introduce a new universal approval mechanism for Pages/files/modules. Publication follows the teacher's requested scope.

Update `engine-docs/formats.md`, CLI help/examples, the relevant architecture future-capability statements, and any command routing/validation/review handling affected by the new document kinds. Seed changes must retain scaffold's existing rule of preserving teacher-edited workspace files.

Keep `internal/localui/capabilities.go` and `docs/engine-capabilities.md` aligned, with the review date. Planned operations are not supported capabilities. On implementation, distinguish local tests, live API write/readback, teacher browser rendering and student validation. Preserve the existing curriculum-capture evidence's scope; it does not prove these new commands.

## Validation and completion

Use focused tests for the meaningful boundaries: Page create/read/update and conflict preservation; hub restrictions remaining intact; HTML/Markdown backward compatibility; wrong-course references; course upload transfer/completion with no PAT leakage; known receipts and returned IDs avoiding repeat writes; and ordered module placement/resumption. Exercise uncertainty paths where a duplicate write or lost unsent edit would matter. Do not create a certification matrix or tests that merely repeat implementation details.

Run relevant Go tests/build and `git diff --check`. Follow the repository build instructions. After an explicitly scoped live trial is authorized, verify one synthetic lesson Page with image/table/code, one uploaded download, a video/caption sample if available, one existing supported quiz/activity and one module. Record actual IDs, request/readback outcomes and observed rendering/access. Keep all trial objects unpublished unless the teacher requests publication. Never use student work as fixtures.

The handoff is complete when the documented commands can deliver and revise a small native module without vendor launches, returned IDs and local drafts survive partial completion, relevant tests pass, and capability documentation accurately states what has and has not been verified. Full curriculum capture/conversion is a separate teacher-workspace task.

## Deliberately outside this implementation

No CompuScholar login/session manager, vendor scraper, full course exporter/importer, Classic Quiz writer, Python runner, hidden-test inference, bulk migration framework or automatic course-wide publishing. General `cse download-file` and `cse api --all` are useful possible conveniences but not prerequisites: the existing external capture helper already downloaded sampled assets, and the underlying client already paginates. Add those only for a separately requested workflow. Do not turn caption gaps or individual curriculum mistakes into Engine backlog items by default.

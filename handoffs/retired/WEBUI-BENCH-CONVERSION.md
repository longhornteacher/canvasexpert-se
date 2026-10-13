> **Retired 2026-10-13.** Historical instructions and evidence; do not execute this plan. Today's scope and dispositions are in [01 — Handoff cleanup](01-HANDOFF-CLEANUP.md). Retirement does not establish completion or new validation; preserve the dated evidence and teacher work described below.

# WebUI Bench conversion — Luna execution slices

Status: **implementation complete; local verification and synthetic browser inspection completed**, 2026-10-12.

The teacher approved Bench: an elegant scientific instrument, minimal words and controls, a calm work interface. They then approved the clickable page draft. Convert the existing WebUI without losing its functionality. This handoff and its reference asset are the only Source changes authorized in the planning session; the slices below describe subsequent implementation work.

Read Source `AGENTS.md`, `ARCHITECTURE.md`, `SCORING-RULES.md`, and this handoff first. Inspect `git status` and preserve existing changes. Work in Source, never in a live Install or Sync directory. Execute one numbered slice at a time, in order. Each slice is intended for Luna; do not dispatch all slices in parallel. Carry the approved direction forward without reopening the visual design.

## Approved reference and interpretation

The portable reference is [webui-bench/bench-pages.html](webui-bench/bench-pages.html). It starts on Classroom → Courses; navigate through Home, Classroom, Settings and Guide. It is a conversation mockup, not production source. It needs the visualization wrapper for its original presentation; its scoped markup and styles also provide an inspectable reference. Do not ship it, its host APIs, sample accounts, fake directories, timers, or simulated saves.

User decisions:

- Bench is the selected direction. Retire the colorful, chunky, playful appearance and copy.
- Home immediately shows Canvas connectivity, active courses and Sync availability. No repeated Home title underneath the selected Home navigation item.
- Provide responsive clickables where useful: Browse for directory selection, Test for checks, Manage for course selection. Give clear hover, focus, disabled, checking, success and failure feedback.
- Automatically test a saved Canvas token. A persistent yellow “saved, not tested” state requiring a click is rejected. Test remains a retry action.
- Remove clutter and distraction, not capabilities, actionable errors or information necessary to understand a consequential action.

Visual treatment: restrained off-white/charcoal surfaces, thin neutral dividers, ordinary compact text, small status marks paired with labels, modest corner radius, no heavy shadows or decorative transforms. Use the system font; reserve monospace for paths, IDs and short codes. Product colors must work in light and dark appearances. Do not require remote fonts, icons, frontend packages or network assets. No SPA/framework migration.

Follow the approved structure:

| Primary navigation | Secondary pages | Existing surfaces to retain |
| --- | --- | --- |
| Home | None | Status, active-course links, agent setup access |
| Classroom | Courses, Students | `/courses`, `/students`, roster refresh |
| Settings | Connections, Preferences, Files, Agent, System | `/settings`, `/setup`, `/connect`, Engine/Install information, quit |
| Guide | How it works, Capabilities | `/how`, `/capabilities` |

Keep existing URLs, handler actions and direct/standalone entry points working. Settings sections may use a small query parameter or anchors plus simple disclosure; do not invent separate backing settings stores. Use ordinary server-rendered forms. Add small local scripts only for interactions that need them.

The reference does not show the review surface. Apply the same restrained design there in slice 7 while preserving its exact workflow. The mockup is not authority to remove fields, routes, evidence, confirmations or review behavior.

## Boundaries and feature preservation

Presentation belongs principally in `internal/localui/`. Reuse existing Engine operations and helpers. The only intended new behavior is automatic Canvas checking and direct Sync Browse/Test affordances. No changes to scoring judgment, write authorization, Canvas posting scope, course/student/attempt identity, file ownership or shared-rule authority.

Preserve these concrete capabilities:

| Surface | Preserve |
| --- | --- |
| Courses | All returned/saved courses, active selection, editable nicknames, Canvas code, course ID/state, unpublished and no-longer-listed indication, save/errors/notices |
| Students | Active-course navigation, course identity, roster refresh, last-pulled time, real name/stable ref, aliases, Canvas differentiation tags and tag errors, monitored and extra-time flags, private cross-course notes, save and standalone Save & close |
| Canvas connection | Origin, validated account/status, setup/replace, token guidance, password input, per-machine Credential Manager storage, explicit removal confirmation, retry and honest failures |
| Teacher rules | Install root, effective rules root/mode, editable Sync path, Browse, Test, explicit save/local return, preserved originals/archives, unavailable-folder explanation |
| Preferences | Alias mode, feedback voice and grading/rubric guidance; existing limits, independent saves and active Install/Sync routing |
| Files | Actual complete seed comparison, current/customized/missing/unavailable states, eligible explicit replacement with confirmation and archived previous copy; protected context remains protected |
| Agent setup | Work/Cowork selection, different-Install replacement confirmation, registration/preparation outcomes and next actions, actual extension download, local tool check and details, finish-in-client instructions |
| System and Guide | Engine version/path, read-only Install path, quit, all existing how-to topics, all capability entries and their evidence details/review date |
| Review | Student/ref/attempt/course identity, assessment arithmetic, held/done states, changed-since-revision, submission/attachments/content preview, feedback, component scores, teacher-only notes/key/late basis, approve/revise notes, requested posting scope, post/revise-only controls, all-or-nothing authoring behavior, completion and errors |

Keep explanations near their action, usually in native details elements. Never hide actionable failures in collapsed content. Never give student-facing feedback teacher-only information. Preserve unsent edits when switching sections, refreshing a roster or navigating courses: use existing retention where present and a small direct draft-preservation solution where the new interaction would otherwise discard work. Do not build a generic draft framework. Do not silently submit unsaved work as a workaround.

Protect current same-origin, request-secret, POST, CSP, escaping and privacy boundaries. Browser scripts must never receive the Canvas token. Any added loopback endpoint uses the existing request validation and returns only necessary non-secret state. If trusted scripts are necessary, permit only those scripts and same-origin calls needed for the feature; do not broadly relax the page CSP. Inspect the hub mutex before asynchronous checks or a native chooser: a slow external operation must not freeze unrelated pages or overwrite newer credential state.

## Slice 1 — Bench shell and shared visual vocabulary

Primary files: `internal/localui/theme.go` and hub templates in `hub.go`, `hub_pages.go`, `students.go`, `setup.go`, `connect.go`, `playbook.go`, `capabilities.go`.

Replace the playful CSS/navigation with a shared Bench treatment. Add active primary/secondary navigation and consistent forms, rows, status/error text and disclosures. Keep every existing page usable during the transition. Remove decorative badges, repeated product/page introductions, rotation, colorful pills and jokes. Change setup copy to Canvas address, Access token, Connect; retain instructions and outcomes.

Use semantic native controls and keyboard navigation. Target the approved desktop composition and reflow to 320px. Long paths must wrap. A dense course table may scroll within its table wrapper; the whole page must not overflow. Do not turn a normal desktop roster into a large stack of oversized cards merely because the mockup uses three example rows: preserve practical batch-editing density for a real class while making private notes expandable.

Done: existing hub and standalone setup/student pages share the restrained visual language, navigation reaches all current features, no backend behavior changes. Run relevant existing localui tests and visually inspect representative desktop/mobile pages. No cosmetic snapshot suite is needed.

## Slice 2 — Useful Home and automatic Canvas check

Primary files: `hub.go`, `hub_context.go`, `setup.go`, applicable localui tests; reuse the existing `whoami`/`testCanvas` validation path.

Build Home around two quiet status regions, an active-course list and a compact agent-connection link. Keep student access through Classroom and quit through System. Do not repeat Home as a heading.

Automatically perform a bounded, authenticated, read-only check when the hub first needs saved-token health. Show Checking briefly, then Connected, Token rejected or Unavailable. No token means Not connected with Connect. A network/DNS/TLS/Canvas-service failure does not prove token rejection and must not advise replacing valid credentials. Do not display saved credentials as verified.

Render Home promptly while checking; reuse the result across hub pages. No continuous polling, heartbeat or background service. Manual Test rechecks. Successful setup already validates the credential; do not immediately duplicate that request. Replacement/removal invalidates old results. A result from an old credential cannot update the current credential's status. Retain the existing setup-status/installer handshake semantics: automatic health success must not falsely report a setup save or client adoption.

Done: Home communicates genuine result states automatically, actions are useful, all independent navigation works during a slow check. Add focused mocked checks for success, authentication rejection, transport failure and missing/replaced credentials; no real PAT or Canvas writes. Update capability evidence/docs only for behavior established here, clearly labeled local.

## Slice 3 — Sync Browse and Test

Primary files: `settings.go`, `hub.go`, `internal/platform/pick.go`, relevant tests. Existing native chooser: `platform.PickFolder`; existing save authority: `workspace.SetRulesLocation`.

Reuse the native Windows folder chooser for Sync with accurate wording. It currently says “workspace folder”; fix that for this use without restoring selection of a different Install. Browse chooses/populates the candidate path. It does not bind, initialize or migrate rules until explicit Save/Use folder. Cancellation preserves the existing path and draft. Paste/edit path remains usable if the chooser is unavailable. Keep the existing chooser approach; no cloud-drive browser, enumeration API or new platform integration framework.

Test checks readability/availability of the configured rules folder through existing routing. Test never initializes files, changes the binding or claims cloud-provider delivery is current. Show Available, Unavailable or Local only; only the last explicitly saved path is authoritative. Home can route Browse to the same concise control instead of duplicating settings logic. Explicit Use Install invokes existing local-return preservation semantics. Keep Install location read-only.

Done: Browse → choose → explicit save works in a temporary fixture; cancel preserves inputs; folder check has no writes; unavailable Sync stays configured with an actionable explanation and no stale fallback. Mock the chooser for automated route tests, then do one manual Windows chooser check. Keep provider delivery and actual client adoption marked unobserved.

## Slice 4 — Classroom: courses and students

Primary files: `hub_pages.go`, `hub_students.go`, `students.go`, applicable tests.

Implement compact Courses/Students navigation and practical roster editing. Courses keep active checkbox, nickname and identity/state metadata; Save reports its result in place. Students keep course tabs, Refresh, count/time, aliases, read-only Canvas tags, monitored/extra-time controls and private notes. Notes may expand per student. Preserve real-class batch efficiency and mobile access. Keep explanatory distinctions: tags are managed in Canvas; extra-time settings do not write individual Canvas deadlines; notes stay private and follow the student across courses.

Done: choose courses/nicknames, refresh roster and save aliases/flags/notes through existing handlers; empty/tag-failure cases remain understandable. Verify that course switching/Refresh does not lose unsent edits and standalone Save & close still works. Reuse identity/privacy tests; add only a focused regression where behavior changes.

## Slice 5 — Settings: connections, preferences, files and system

Primary files: `hub.go`, `settings.go`, `hub_context.go`, `playbook.go`, `setup.go`.

Group existing settings into Connections, Preferences, Files and System; Agent is delivered in slice 6. Show useful controls first. Put preservation explanations and token-removal/file-replacement confirmations next to their actions in disclosures. Keep setup and replacement reachable by direct URL. Display actual file comparison data, not the mockup's example list. Keep missing/unavailable states and context protection. Expose active rule location where teachers edit shared preferences; keep private student context local. System retains version, program path, read-only Install location and Quit hub.

Done: each existing save/remove/replace operation still calls its current handler with correct fields, limits and request protection. Unsaved preference input survives local section changes. Confirm one temporary custom-file adoption preserves its old copy and a context file cannot be replaced through that control. No live teacher files are modified.

## Slice 6 — Agent and Guide

Primary files: `connect.go`, `playbook.go`, `capabilities.go`; `docs/engine-capabilities.md` when evidence/status changes.

Agent retains app selection, explicit different-Install replacement, actual extension download, local tool test, result/next-action details and the complete finish-in-client prompt. Avoid a blanket Connected badge: registration saved, extension prepared and local tools passed do not establish destination-client adoption.

Guide keeps all six existing how-to topics in concise expandable sections, Engine/Install/Sync explanations and truthful operational instructions. Capabilities becomes a quiet evidence list, retaining the full current data and detail rather than a summarized subset. Its groupings describe recorded evidence and unsupported features, not current connectivity. Preserve New Quiz written-work limits, local/live distinctions and pending human checks. Change no capability classification merely because the page looks cleaner. If this work adds tested Browse/automatic-check behavior, update the capability record and document together without claiming a new live trial.

Done: both app branches and download route work as before; checks have honest outcome labels; all how-to topics and capability records remain available. Use existing connection/capability tests and inspect expanded details in both appearances.

## Slice 7 — Review, completion pages and final verification

Primary files: `review.go`, `cards.go`, shared theme, completion/error responses across localui.

Apply Bench to the review page and remaining completion/error surfaces. This is visual work, not a redesign of assessment or authorization. Keep submission evidence above proposed feedback, private information unmistakably teacher-only, held/done status visible, and posting scope explicit. Preserve approve/revise per item, revision notes, Post approved and Send revisions only, all-or-nothing creates, changed-since-revision notices and temporary-page lifecycle. Do not add auto-post, a new approval token, post-write readback or a mandatory extra confirmation. Review-only pages do not acquire unrelated settings navigation or actions that discard unsent revision notes.

Finish with the existing relevant Go tests, a normal build, and browser inspection using temporary Install/Sync data. Inspect Home, courses, a real-size synthetic roster, connections, preferences, files, both agent branches, Guide/Capabilities, and an ordinary plus comment-only review at desktop/mobile widths. Check light/dark appearance, keyboard focus, errors, empty states and long paths. Only add focused tests for changed behavior; do not build a source-text checklist or exhaustive visual matrix. No release/package ceremony unless publishing is separately requested.

Done: all feature-preservation rows above have a reachable working equivalent, layout has no page overflow, no placeholders/simulation behavior shipped, review semantics preserved. Report changed files, checks run and actual remaining limitations. Update evidence docs only where findings change them. Stop after this conversion; no incidental backlog or framework work.

## Per-slice execution/reporting instruction for Luna

Implement the requested slice, keep its checks proportionate, and stop with a short report: what changed, how it was verified, any concrete limitation, and the next slice. Preserve unrelated work. Routine implementation choices within this direction do not require renewed design approval. If one supported feature truly cannot be retained, explain the exact blocker and continue independent work; do not silently remove it. Do not treat the reference's fake statuses, sample data or unfinished preview actions as implementation requirements.

## Execution record — 2026-10-12

All seven slices are implemented in Source. The WebUI now uses the shared Bench shell and navigation; Home and Connections run the bounded cached Canvas health check; Sync Browse/Test retains draft-versus-saved authority; Classroom preserves compact roster and unsent edits; Settings groups existing controls; Agent and Guide retain their workflows and evidence; review, completion and error pages use the same restrained light/dark treatment without changing posting semantics. Capability data and [engine-capabilities.md](../../docs/engine-capabilities.md) record only local evidence and current limitations.

Verification after removing the opt-in fixture harness: `go test ./...`, `go test ./internal/localui -count=1`, `go build ./...`, and `git diff --check` passed. Synthetic browser inspection covered Home, Courses, 30-student roster, Connections, Preferences, Files, System, both Agent outcomes, all six Guide topics, expanded capability details, and ordinary/comment-only review outcomes at desktop/mobile widths in light/dark appearance. The inspected layout had no page overflow; the dense roster remained practical, keyboard focus was visible, long paths wrapped, and draft/save behavior was exercised. The temporary browser fixture source was removed after inspection. An explicit mocked Browse → Save assertion verifies that Browse alone leaves authority unchanged and the later Save binds the selected temporary folder.

Limitations: no real Canvas request, live Install or Sync data, Canvas write, package/release, or destination-client adoption was part of this conversion. The Windows folder chooser opened and returned a candidate during local browser inspection; that candidate was not saved. Cancellation was verified with a mocked chooser, not a manual native-dialog cancellation. Cloud-provider delivery and human/client adoption remain pending observation.

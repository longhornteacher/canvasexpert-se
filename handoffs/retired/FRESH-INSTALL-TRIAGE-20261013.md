> **Retired 2026-10-13.** Historical instructions and evidence; do not execute this plan. Retirement does not establish completion or new validation; preserve the dated evidence and teacher work described below.

# Fresh laptop install: development triage

Reviewed 2026-10-13 against Source `288129f`, the [session notes](install-session-notes-2026-10-13.md), previous implementation/package record, architecture and scoring rules. This is a proposed development order, not authorization to execute the implementation. No Install, credentials, client settings or Canvas data were changed during triage.

## Recommendation

Make a bounded first-run repair: fix the installer-launched WebUI crash and Cowork opening, resolve the recurring origin mismatch, and bring the Source install playbook up to date. Keep URL convenience and output cleanup secondary. Do not reopen the whole installation architecture or turn pending destination observations into Engine features.

| Priority | Notes | Development disposition |
|---|---|---|
| P1 | B: WebUI startup | Concrete Source defect found; repair first. This affects both destinations. |
| P1 | A: Cowork opening | Confirmed field failure of default-file association; implement direct Claude launch with a useful fallback. |
| P1 investigation | C: saved origin mismatch | Repeated field failure despite a recorded fix in this exact package; investigate the existing path, not a second origin-setting subsystem. |
| P2, same delivery | F: walkthrough | Merge the intended guidance into the Source seed; explicit both-destination support and agent handling of recovery. |
| P2, bounded follow-up | D: pasted Canvas link | Useful convenience; normalize at setup input if adopted. No new credential-writing agent tool needed. |
| P3 | E/G: console/version output | Small usability cleanup after the first-run path works. Some claims need reproduction. |
| No new feature | G: development build/backups; F: pending user choices | Build is an intentional pilot artifact; current repeat registration already avoids backups when unchanged. Course choices and client trust/reconnect are remaining teacher-session work. |

## 1. WebUI startup: a concrete crash, not merely a timeout theory

`cmd/cse/install_ui.go:22` launches `ui` with `--ready-file` and waits eight seconds. In `internal/localui/hub.go:234`, that option bypasses the block that assigns `hubUnlock`. With a nonempty Install root, `hub.go:349` nevertheless calls `hubUnlock()` unconditionally. The nil function call occurs before browser opening and before the ready marker is written. The same pattern exists in commit `7d59818`, which contains the earlier fresh-install work. This is a Source-established crash path consistent with both laptop failures; the laptop process/log was not inspected or reproduced here.

The readiness path also bypasses same-Install lock/reuse logic. Repair that path using the existing hub coordination, including returning readiness when reusing a hub. A nil guard alone would stop the crash but leave the installer bypassing intended reuse. Do not merely increase the timeout or blame antivirus.

Relevant files: `cmd/cse/install_ui.go`, `cmd/cse/main.go`, `internal/localui/hub.go`, `internal/localui/hub_discovery.go` and existing hub/startup tests.

Acceptance: an isolated extracted Install starts through the actual installer readiness path; a second start returns the same healthy Install hub; the returned URL is usable; registration/local-check results remain visible if UI startup fails. Exercise this actual branch with a focused regression and a packaged child-process check, using synthetic configuration and no real Canvas request/client registration. Existing manual `webui.cmd` verification did not cover this branch.

The notes' requested successful-start URL and manual-start instruction already exist in Source. The crash prevents the success result from being emitted. Improve partial-outcome reporting with a small structured UI warning/outcome alongside registration and local-check results. Do not imply full installation success from registration alone, or build a new exit-code taxonomy before defining the result contract. There is no reliable URL to print for a server that never became ready.

## 2. Cowork: launch the known app, retain the trust action

`internal/platform/windows.go` uses the OS default handler; `internal/installation/installation.go` records only the request outcome. The notes establish that the default handler opened an app chooser, and an explicit Claude executable plus the MCPB path reached the extension-install dialog. That is teacher-observed dialog evidence, not completed installation or connected tools.

Use the observed stable per-user Claude launcher first, with arguments passed separately. Add other Windows discovery routes only where supported by concrete installation evidence; an exhaustive Store/MSIX/per-machine/macOS matrix is not a prerequisite to fixing this Windows distribution. If Claude cannot be found/launched, reveal the prepared file and give the short manual extension-install step. Preserve `--check-only` and `--no-ui` behavior.

Keep outcomes precise: app launch requested, fallback file revealed, or failed; none proves trust accepted. Update `internal/localui/connect.go` and the strict allowlist in `internal/installlog/log.go` with any new outcomes. Focused injected discovery/launch/fallback checks and a path-with-spaces case are sufficient local coverage. Real client adoption is a separate observation.

## 3. Origin mismatch: reopen the existing fix

Both hub setup (`internal/localui/hub.go:280`) and standalone setup (`cmd/cse/main.go:414`) already call `workspace.SyncInstance` after credential saving. That helper handles empty/starter origins and empty course lists while preserving the document body and protecting populated different-origin course selections. The previous handoff records this fix in the **same** `ae41e92e9916+dirty`, `2026-10-12T23:09:05Z` pilot build named in the laptop notes. Treat this as a possible integration failure; neither "already fixed" nor "old binary" is established.

Start with the existing sanitized **Install-root `install-log.jsonl`**, especially credential-save/origin-configuration events and Install paths. It is outside `.cse`; private runtime files need not be inspected. Compare actual executable/package identity if available. Reproduce a fresh hub form submission with mocked validation/save and starter `courses: []`, exercising the real post-save callback, then the standalone completion path. Check that the running hub has the intended root and reports a configuration-save failure separately from credential success.

Do not silently adopt saved credentials into a populated different-origin course configuration. Improve mismatch messages in `cmd/cse/remote.go` and `internal/doctor/doctor.go` to identify both sanitized origins and the corrective action, without printing raw URL credentials/query parameters or tokens. Preserve the existing helper instead of adding a competing setup writer. Add only the regression demonstrated by investigation.

## 4. Playbook changes belong in Source before another package

The laptop's walkthrough additions were made in **Install**. They are not present in `workspace-template/playbooks/install.md`; the notes describe their intent but do not supply the exact edited file. Merge that intent into the Source seed, preserving teacher customization when later adopted in Install/Sync.

- An explicit "both" selects both destinations; run each helper and keep independent outcomes. No order requirement is needed.
- The agent performs supported launching/recovery and gives the teacher only the remaining PAT/trust/reconnect actions.
- Recover the WebUI independently if registration succeeded but startup failed. Use non-secret status and the root install log, never fetch the private form.
- After reconnect/trust, use each destination's tools to read instructions and configured courses, checking `install_root`. This requirement already exists in the Source handoff section; clarify the walkthrough rather than duplicating it.
- Use course IDs/states to propose a current selection, while the teacher supplies nicknames and confirms choices. Do not hardcode this teacher's name or course IDs into the seed. Offer teaching context progressively.

No rigid eight-step wizard or new "both" CLI flag is required.

## 5. Secondary items and corrections to the notes

**Canvas link input (D):** accepting a bare host or extracting an HTTPS origin from a pasted link is useful. Prefer the private setup field/shared normalization helper over a new `setup --origin` or MCP credential-changing API. Discard path/query/fragment without echoing them; reject unsafe schemes and embedded credentials. Current `canvas.New` accepts HTTPS origins, including custom domains. No Instructure-only hostname policy was found; do not introduce that restriction from the notes' suggestion. Agent origin-only guidance can land with the playbook repair.

**PowerShell/help (E):** progress should not move into machine-readable stdout. Reproduce in PowerShell 5.1 with the packaged launcher and inspect the native exit code separately from stderr presentation. Gate/suppress noninteractive progress if needed. Current `run` returns 0 for `help setup` while falling back to general help; the reported 255 is unexplained, not an established Engine return. Improve unknown-topic guidance without inventing a large command-help system.

**Verbose version (G):** `installation.Check` copies the human `engine_info(version)` reply into `child_engine_version`, explaining the dependency blob. Compact identity is a small cleanup; retain enough revision/build identity to distinguish loaded adapter and on-disk Engine. A matching product version alone is insufficient.

**Dirty build (G):** the previous handoff explicitly identifies this as an unsigned development pilot ZIP. `docs/releasing.md` already distinguishes it from a release and enforces clean/tagged release packaging. No packaging bug is established. Use immutable release assets when intentionally publishing a release.

**Backups (G):** current `registerWork` returns `settings-already-saved` before creating a backup when command/args already match. Repeated unnecessary backups need an actual differing-config example; do not add pruning or delete recovery copies based on this note alone.

## Delivery and evidence boundary

Proposed order: readiness repair, Cowork launcher, origin investigation/fix, Source playbook merge, then a rebuilt pilot artifact with exact identity. Keep checks limited to changed behavior. Update both capability records with local versus teacher-observed outcomes. This triage updates the evidence now; it does not claim any implementation repair or new runtime test.

The fresh laptop still needs course selection/nicknames, Cowork trust/install completion and a real destination tool call in each selected app. Those are pending observations/teacher-session actions, not new Engine features or prerequisites for unrelated teaching workflows. The notes do not establish completed Cowork or Work connectivity, Sync delivery, or student-work behavior.

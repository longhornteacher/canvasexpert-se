# WebUI asset extraction and copy warmth pass

Status: **ready**, 2026-10-13. One agent, one session, **two commits**, in order:

- **Slice A** is a mechanical refactor that changes no behavior: the same pages, routes, copy, CSP, request checks and draft semantics.
- **Slice B** is a copy-only pass on top of A, done in the new template files.

Keep the slices separate so the refactor stays reviewable as a no-op. Do not reopen the Bench visual design.

Read Source `AGENTS.md` and [WEBUI-BENCH-CONVERSION.md](WEBUI-BENCH-CONVERSION.md) for context only. Work in Source.

## Why

After the Bench conversion, `internal/localui` keeps its HTML, CSS and JS in Go string literals. Some are single lines of several KB, such as `settingsTemplate`. The color palette is copied four times: `theme.go`, the review page CSS, `reviewError`, and the review done page. The two draft scripts each repeat their field-name regex twice. Static scripts go through three nearly identical `case` blocks in `Hub.ServeHTTP`. Each future WebUI change costs more than it should, and reviewers can't read diffs of 4 KB lines.

## Slice A: extraction. Do, in order

1. **Static assets via `//go:embed`.** Create `internal/localui/assets/` holding `base.css` (tokens and palette, defined once), `hub.css` (the current `funkyCSS` minus tokens), `review.css`, `canvas-health.js` and `drafts.js`. Pages keep inline `<style>`; inject the embedded CSS text the way `funkyCSS` is injected today. That keeps CSP `style-src 'unsafe-inline'` unchanged and adds no stylesheet route. Review, review-error and review-done pages use `base.css` + `review.css`, so the four palette copies collapse to one.
2. **One script route.** Replace the `/_cse/canvas-health.js`, `/_cse/drafts.js` and `/_cse/settings-drafts.js` cases with a single GET-only `/_cse/` file handler over an allowlist of embedded `.js` files. Keep the URLs, the `application/javascript` content type and the per-path CSP in `headers()`.
3. **One draft script.** Merge `classroomDraftScript`, `settingsDraftScript` and `draftNoticeScript` into `drafts.js`. Each form or page declares its field-name prefixes in a `data-draft-fields` attribute (for example `active_ nick_` or `aliases sync_root voice feedback_guidance`) instead of a hardcoded regex. Keep the current behavior exactly:
   - sessionStorage key per hub secret;
   - the `saved=1` and `clear=…` clearing rules;
   - a `sync_root` URL value that came from Browse does not trigger the notice;
   - the restored-edits notice;
   - Discard drops only the fields on the current page.
4. **Templates to files.** Move each `template.Must(...Parse(`...`))` literal into `internal/localui/templates/*.html`, embedded with `//go:embed` and parsed once at init. Break the markup across normal lines. Nav fragments (`hubNav*`) become `{{define}}` partials, or stay Go constants if converting them is awkward. Do not change markup semantics while reformatting.
5. **Small cleanups in the files you touch anyway:**
   - Rename `funkyCSS` to `pageCSS`.
   - Delete the hidden `<div class="badge">` elements and the `.badge{display:none}` rule.
   - Split `homeData`: a Settings-only struct embeds the shared fields and holds `SettingsSection`, `SettingsNav`, `SettingsDraftClear` and `Playbook`.
   - In `Hub.ServeHTTP`, gather the routes that run before the mutex (`/canvas/test`, `/rules/browse`, `/rules/test`) into one clearly commented block or a small `unlockedRoutes` map. The comment states the rule: these handlers may block on external I/O and must take `h.mu` themselves before touching hub fields.
6. **Optional, only if trivial:** clicking Test while an automatic check is in flight currently does nothing, and the setup-status `connection_tested` stays false. Record a pending manual flag that the in-flight result honors. Skip this if it needs more than a few lines.

## Don't

- Introduce a frontend build step, framework, bundler, minifier, npm dependency or new page or route.
- Relax CSP, add `script-src` to more pages, or move scripts inline.
- Change copy, layout or classes beyond the renames above (in Slice A; copy belongs to Slice B).
- Add snapshot, golden-file or source-text tests.
- Touch `review.go` behavior, posting or authorization.

## Slice B: copy warmth

The teacher's verdict on the Bench copy: we went from silly over-talking to extra-cold. Aim between the two. Assume the reader is smart and intuitive but **does not already know what CE-SE is doing**. A bare status word ("Connected", "Local only", "Available") makes them guess. Give them a plain sentence that says what it means for them.

Guidance:

- **Status = label + one plain sentence.** Keep the short label and status mark. Then say what it means and, when useful, the next step. Use the teacher's own nouns: your Canvas account, your courses, this PC, your teaching rules. Avoid internal ones (Install root, seed, adapter, fingerprint, authority) in primary text. Those stay in disclosures or System.
- **Name the specific thing when we know it**, such as the account name, the folder or the course count. "Canvas connection ready: signed in as Pat Lee" beats "Connected. Canvas accepted the saved token."
- **Consequential items get a short explainer near the action**, mainly the Canvas token, Sync, token removal and file replacement. One or two sentences, or a `<details>` guide; never a wall.
- **Warm and plain, not cute.** Contractions are fine and second person is fine. No jokes, exclamation marks, mascots or marketing. One sentence usually beats a fragment and usually beats three sentences.
- **Honesty limits stay exactly as they are.** Don't claim cloud delivery, client adoption or token validity beyond the evidence. Keep "may still be valid" wording on non-rejection failures.

Concrete targets (wording is illustrative; match the guidance rather than copying it):

| Where | Now | Direction |
| --- | --- | --- |
| Canvas, connected (Home and Connections) | "Connected" / "Canvas accepted the saved token." Home omits the account name. | "Canvas connection ready: signed in as {name}." Show the name on Home too; the health JSON already returns `account`, so build the note server-side or in `canvas-health.js`. |
| Canvas, checking | "Checking the saved token with Canvas." | "Checking your Canvas connection…" |
| Canvas, rejected | "Canvas rejected the saved token. Replace it to reconnect." | Say it may have expired or been deleted in Canvas, that replacing it reconnects, and that the teacher's files aren't affected. |
| Canvas, not connected | "No Canvas access token is saved on this computer." | Why a token is needed (to work with your courses), where it's kept (Windows Credential Manager on this PC) and the Connect action. |
| Sync, local | "Local only" / "Teacher rules are stored in this Install." | Your teaching rules are kept on this PC. Add a Sync folder only to share them with another PC. |
| Sync, available | "Available" / "readable here…" | Your teaching rules are shared through {folder}; your sync app (OneDrive or Google Drive) keeps other PCs up to date. |
| Setup done page | "Connected" / "Canvas accepted the Access token…" | Include the account name if available, plus what's next (pick courses, or return to the agent). |
| Empty states (no courses, no students) | Terse | Say what fills them and the one action to take. |

Sweep the other hub pages with the same lens (Settings sections, Agent results, Files states, review completion) and fix obvious cold spots. Leave anything already clear alone. If a localui test asserts old wording, update the assertion; add no copy tests. No capability-record change unless a status *meaning* changes (it shouldn't).

## Verify (proportionate)

- `go vet ./...`, `go test ./...`, `go build ./...`. Existing localui tests are the behavior guard. Update test assertions only where they matched literal Go constants that moved.
- One browser smoke test against a temporary Install (`cse --workspace TEMP ui`; `context/courses.md` from `workspace-template`):
  - Home, Settings (all four sections), Guide and Capabilities render with the same look in light and dark.
  - Restoring a Preferences draft shows the notice, and Discard clears it.
  - The browser console has no CSP errors.
  - Avoid `/courses` and `/students` unless you intend a read-only Canvas request with the machine's saved credential.
- If a review fixture is handy, check that one review page renders. Otherwise rely on `review_test.go`.

## Report

Changed files, checks run, and anything that could not be kept identical, with the exact reason. Make two commits:

- Slice A: no capability or doc updates.
- Slice B: copy only.

The browser smoke test runs after each slice; for Slice B, read Home and Connections aloud as a new teacher would.

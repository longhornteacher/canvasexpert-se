> **Retired 2026-10-13.** Historical instructions and evidence; do not execute this plan. Today's scope and dispositions are in [01 — Handoff cleanup](01-HANDOFF-CLEANUP.md). Retirement does not establish completion or new validation; preserve the dated evidence and teacher work described below.

# WebUI extensibility: one place per page, and a hardening pass

Status: **ready**, 2026-10-13. One agent, one session, **three commits**, in order. Builds on [WEBUI-ASSET-EXTRACTION.md](WEBUI-ASSET-EXTRACTION.md), which is done.

- **Slice A** is the main change: a small page registry so that adding or removing a WebUI feature touches one file plus its template, instead of six places. It changes no behavior.
- **Slice B** removes the places where copy or state names are used as identifiers, and fixes the render and mutex problems found in review. Small behavior-preserving fixes only.
- **Slice C** proves the registry: remove one feature and add one throwaway feature using only the documented steps, then delete the throwaway. This is the acceptance test for A.

Read Source `AGENTS.md` for context. Work in Source. Do not reopen the Bench visual design or the copy pass.

## Why

Today a new page means editing, in `internal/localui`:

1. `hub.go` `ServeHTTP`, a growing `switch`, plus its per-route method checks.
2. `hub.go` `headers()`, which hardcodes CSP by path (`/`, `/settings`, `/courses`, `/students`).
3. `theme.go` `hubNavFor`, a hardcoded `items` map of nav labels and hrefs.
4. `assets.go`, one line per template in the template var block.
5. The `Hub` struct and `pageData()`, where page-specific fields accumulate.
6. `capabilities.go` plus `docs/engine-capabilities.md`, when the feature changes a capability.

Removing a feature means finding all of those again. Nothing fails loudly if one is missed: a dead nav link, a CSP that is too loose or too tight, or a route with no nav entry. The goal is that **a page is declared once, in one file, and the nav, routing, method rules, CSP and templates derive from that declaration.**

This is a local single-teacher tool. Keep the mechanism small: a struct and a slice, not a framework, interface hierarchy, plugin loader or reflection.

## Slice A: page registry, no behavior change

1. **One declaration per page.** Add `internal/localui/pages.go` with a plain struct and a registry:

   ```go
   type page struct {
       Path     string   // "/courses"
       Methods  []string // allowed; default GET. Others get 405 + Allow, in one place
       Template string   // templates/<name>.html, optional (handler may write its own response)
       Nav      *navSlot // nil = not in the nav (POST endpoints, JSON, setup flow)
       Scripts  []string // assets/*.js this page loads; drives CSP script-src
       Connect  bool     // adds connect-src 'self' (only the health-polling pages)
       Unlocked bool     // runs before h.mu; replaces unlockedRoutes
       Handle   func(*Hub, http.ResponseWriter, *http.Request)
   }
   type navSlot struct{ Primary, Label string; Order int }
   ```

   Pick names that read well; the shape matters more than these identifiers. Pages register themselves from the file that owns them (`func init() { register(page{...}) }`), so deleting a feature file removes its routes, nav entry and CSP.

2. **Derive, don't repeat.**
   - `ServeHTTP` looks up `pages[r.URL.Path]`, applies the method rule, then locks or not by `Unlocked`, then calls `Handle`. Delete the `switch` and `unlockedRoutes`. Keep the Host check, `headers()`, and `serveScript` ahead of it.
   - `headers()` builds CSP from the page: `script-src 'self'` iff `Scripts` is non-empty, `connect-src 'self'` iff `Connect`. The CSP strings sent for each existing path must stay **identical** to today's. Check this with a one-off loop over all existing paths while developing; do not add a snapshot test.
   - `hubNavFor` builds the primary nav and each secondary nav from registered pages with a `Nav` slot, sorted by `Order`. Today's labels, hrefs and order must not change. Two kinds of entry exist today: the primary tabs (Home, Classroom, Settings, Guide) and secondary tabs under Classroom, Settings and Guide. Register the primary tab's destination via the page that owns it.
   - Query-string routes such as `/settings?section=files` are nav items too. Let a `navSlot` carry an optional `Href` override so the Settings sections stay one registered page with four nav entries.

3. **Templates and scripts by convention.** A page that names `Template: "courses"` gets `templates/courses.html` parsed at init; replace the hand-listed var block in `assets.go`. Scripts are already auto-served from `assets/`; the registry just declares which a page uses. Fail at init (as `pageTemplate` already does) if a named template or script file is missing, so a half-removed feature breaks the build or the first test, not a teacher.

4. **Keep what is shared, shared.** `checkPost`, `render`, `redirect`, `needWorkspace`, `pageData` stay as they are. Feature-specific page data must live in the feature's own struct that embeds `homeData` (as `settingsData` does). `pageData()` should not grow new fields for a single page. Move `Voice`, `FeedbackGuidance` and similar Settings-only fields out of `homeData` into `settingsData` if nothing else reads them. Check with the compiler; skip any field Home or Guide actually uses.

5. **One file per feature.** Group each feature's handler, page data, route registration and (where small) helper functions in one file named for the feature: `courses.go`, `students.go`, `settings.go`, `connect.go`, `playbook.go` (rename from `hub_*` files where that helps). `hub.go` keeps the `Hub` struct, `RunHub*`, `ServeHTTP` and `headers`. Do not move code that is only touched for the sake of tidiness; move what the registry needs.

6. **Short extension note.** Add `internal/localui/README.md`, under 40 lines: "Add a page" and "Remove a page" as numbered steps (file, template, script, nav slot, capability entry, tests) and the list of things that are derived. This note is the contract Slice C verifies. Keep it accurate rather than long.

**Do not**: introduce interfaces, generics, reflection, code generation, a plugin directory scan, middleware chains, a router dependency, or per-page CSS files (the shared `pageCSS` stays; add an optional `assets/<page>.css` only if a feature truly needs it, and say so in the README if you add that convention). Do not change copy, markup, URLs or CSP for any existing page.

## Slice B: hardening found in review

Each item is small. Do them in this order and skip none silently; if one turns out not to be small, record why in the report and leave it.

1. **Stable state keys.** Health states are used as identifiers in Go constants, CSS attribute selectors (`.home-status[data-state="Token rejected"]`), template `eq` checks and `canvas-health.js`. Introduce keys (`checking`, `connected`, `rejected`, `unavailable`, `not_connected`) used by `data-state`, CSS, JS and the JSON endpoint's `state`. Keep the human label as separate display text. The JSON `state` field becomes the key and gains a `label`. Update `canvas-health.js` and the localui tests that read it. After this, rewording a label cannot change a color or break polling.
2. **Buffered render.** `render` writes the header, then `_ = t.Execute(w, data)`, so a template error sends a truncated 200. Execute into a buffer and write the status and body only on success; on error return a plain 500 page that names no private data. Apply the same to the other raw `io.WriteString` page writers only if they can fail; they are static, so leave them.
3. **`/setup` outside the big lock.** `/setup` validates the token over the network while `h.mu` is held, so one slow Canvas call freezes every page. With the registry this is `Unlocked: true` plus taking `h.mu` only around hub field reads and writes in `setupHandler`'s callbacks. Do this only if the callbacks are already separable; the setup handler has its own mutex. If it needs more than a small change, stop and report.
4. **One home for draft clear groups.** The clear tokens `prefs`, `grading`, `sync` appear in Go redirect URLs and in `drafts.js`. Move the grouping to the markup: the template declares `data-draft-groups="prefs:aliases grading:voice,feedback_guidance sync:sync_root"` and `drafts.js` reads it, so a new Settings form adds its group in the template and the script never changes.
5. **Draft script smoke test.** Add one `go test` that runs `drafts.js` against a minimal DOM using whatever is already available in the repo's toolchain. If no JS runtime is available without a new dependency, skip this test and say so; do not add npm or a headless-browser dependency.
6. **Dead CSS and tokens.** Delete unused legacy rules in `hub.css` (`.pk`, `.deck`, `.ball`, `.cap-page{transform:none}`, `transform:none`/`animation:none` overrides, the duplicate `.course-state`). Grep every class against `templates/` and Go before deleting. Replace the hardcoded status colors and input border with tokens in `base.css` so dark mode controls them. Check the pages in both color schemes afterward.
7. **Small accessibility and UI fixes.** Give the health note `aria-live="polite"` alongside the state. Show the Sync card's "Test saved folder" only when the rules location is Sync. Leave the Settings "Signed in as" slot in place (a test guards it).

## Slice C: prove it

Using only the steps in `internal/localui/README.md`:

1. **Remove** the Capabilities page (`/capabilities` and its nav entry, template and data). The tree must build and `go test ./...` must pass after you also update the one nav test and the capability-record references. Record every file you had to touch. Then restore it with `git revert`/`git checkout` rather than re-implementing.
2. **Add** a throwaway page, for example `/_probe` with a template, a script and a nav slot, as one new file plus one template plus one script. Verify the route, nav entry and CSP appear with no edits elsewhere. Delete it.

Report how many files each took. If removal or addition needs an edit outside the feature's own files (other than the capability record when status truly changes), fix the registry or README until it doesn't, then repeat. Commit only the final state; the probe and the temporary removal must not be in any commit.

## Verify (proportionate)

- `go vet ./...`, `go build ./...`, `go test ./...`.
- After each slice, one browser smoke pass against a temporary Install (`cse --workspace TEMP ui`, as in the previous brief): Home, Settings (all four sections), Courses, Students, Agent, Guide and Capabilities render and navigate the same in light and dark; a Preferences draft restores and Discards; no console or CSP errors. Avoid `/courses` and `/students` refresh actions unless intended, since they make read-only Canvas requests with the saved credential.
- Do **not** run a blanket `taskkill` on `cse.exe` to stop the test hub; stop only the process you started, since the teacher's own hub may be running.
- For Slice A, compare the CSP header and nav HTML for every existing path before and after; they must be byte-identical.

## Don't

- Add a frontend build step, framework, router library, npm dependency, or new route beyond the Slice C probe.
- Relax CSP or move scripts inline.
- Add snapshot, golden-file or copy-assertion tests. Assert structure (routes, attributes, headers), not wording.
- Touch `review.go` behavior, posting or authorization. Review pages keep their own handler and inline script; only registration may change if it fits naturally, and it is fine to leave it out.
- Expand into fixing unrelated findings. Note them in the report instead.

## Report

Changed files, checks run, the before/after file-touch counts from Slice C, and anything that could not be kept identical, with the exact reason. Make three commits: A (registry and README, no behavior change), B (hardening), C only if Slice C leads to registry or README fixes that were not in A. Update the WebUI Capabilities data (`internal/localui/capabilities.go` and `docs/engine-capabilities.md`) only if a capability's status meaning changes; it should not.

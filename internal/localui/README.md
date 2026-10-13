# localui: adding and removing hub pages

A hub page is declared once, with `register(page{...})` in an `init` func in the file that owns the feature (see `pages.go`). Routing, method rules, CSP, nav entries and templates derive from that declaration; nothing in `hub.go`, `theme.go` or `assets.go` lists pages.

## Add a page

1. Create `<feature>.go` with the handler `func (h *Hub) fooPage(w, r)` and an `init` calling `register(page{Path: "/foo", ...})`.
   - `Methods`: defaults to GET; other methods get 405 + `Allow`. Handlers still call `h.checkPost` for POSTs (origin and secret).
   - `Template`: `"foo"` parses `templates/foo.html` at init. Render with `h.render(w, 200, tmpl("foo"), data)`.
   - `Scripts`: the `assets/*.js` files the template loads. They must match the template's `<script src="/_cse/…">` tags exactly or init panics; they set CSP `script-src 'self'`.
   - `Connect`: only for pages that poll Canvas health (`connect-src 'self'`, and a GET starts the health check).
   - `Unlocked`: the handler runs without `h.mu` and must lock it around hub fields.
   - `Nav`: `navSlot{Primary, Label, Order, Href}`. An empty `Label` is the primary tab; otherwise a sub-tab of `Primary`. The template calls `{{nav "Primary" "Label"}}` to show it.
2. Put page-specific data in the feature's own struct (embed `homeData` if it needs the shared status fields); do not add fields to `homeData`.
3. Add `templates/foo.html` (copy a small one such as `how.html`) and any `assets/foo.js`.
4. Add tests for routes, attributes and headers. Do not assert copy.
5. If the feature changes a capability, update `capabilities.go` and `docs/engine-capabilities.md` together.

## Remove a page

Delete the feature's `.go` file, its template, any script only it used, and its tests. Then:

- if a template or test of another feature links to it, remove that link (the nav entry disappears on its own);
- if the page was a capability record, update `capabilities.go` and `docs/engine-capabilities.md`.

`go build` and `go test ./...` fail at init or in the first test if a declared template or script is missing.

## Conventions

- Health states use stable keys (`checking`, `connected`, `rejected`, `unavailable`, `not_connected`) in Go, CSS `data-state`, scripts and the JSON `state`; the human label is separate display text. Never use copy as an identifier.
- `h.render` executes into a buffer and writes only on success, so a template error is a plain 500, not a truncated page.
- Handlers that wait on the network (such as `/setup`) are `Unlocked` and take `h.mu` only around hub fields.
- Retired design history: [WEBUI-BENCH-CONVERSION](../../handoffs/retired/WEBUI-BENCH-CONVERSION.md), [WEBUI-ASSET-EXTRACTION](../../handoffs/retired/WEBUI-ASSET-EXTRACTION.md), [WEBUI-EXTENSIBILITY](../../handoffs/retired/WEBUI-EXTENSIBILITY.md).

## Derived, so never edited by hand

Route table and method rules (`pages`), CSP script/connect sources (`headers`), primary and sub-navigation (`hubNavFor`), template parsing and script/template consistency (`register`).

Shared pieces stay shared: `checkPost`, `render`, `redirect`, `needWorkspace`, `pageData`, the base CSS and `drafts.js`/`canvas-health.js`. Review pages (`review.go`) are one-shot servers with their own inline script and are not registered.

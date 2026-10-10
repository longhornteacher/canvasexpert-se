# Canvas page HTML probe (CS8)

**Date:** 2026-10-10
**Course:** CS8, course 121046 (“FUND OF COMPUTER SCIENCE - Beckham”)
**Canvas object:** unpublished page 2266154, `[CSE FIELD PROBE] Page HTML canaries — 2026-10-10`
**URL:** `https://pearland.instructure.com/courses/121046/pages/cse-field-probe-page-html-canaries-2026-10-10`

> **Correction (2026-10-10, review):** this run wrote and read back through the Canvas rich content editor, not the API. The editor normalizes HTML on save and on reopen, so the findings below describe editor behavior and do not establish what Canvas stores from an Engine (API) write. CE-SE also has no HTML allowlist of its own; the "narrower CE-SE allowlist" below is OG CanvasExpert's. The follow-up API probe is in [the handoff](../handoffs/active/CANVAS-FIELD-LIMITS-PROBE.md).

## Scope and method

Created one unpublished page to measure Canvas storage for the page-body field. The payload included a small set of allowed-format canaries, doubtful or disallowed tags/attributes, link and image paths, table spans, LaTeX-style text, MathML, and an equation-image element. The page stayed unpublished, its “notify users” option remained off, and it contained no student work or real student data.

The Canvas Expert connector returned `legacy_storage_reappeared` for course listing, page reads, and mirror refresh. I confirmed the course from the signed-in Canvas dashboard, saved through the CS8 Pages editor, reopened the page in the HTML source editor, and inspected the Canvas page view. The source captured in [`stored.html`](evidence/cs8-page-html-probe-2026-10-10/stored.html) is the HTML shown by Canvas after saving and reopening. The submitted body is in [`sent.html`](evidence/cs8-page-html-probe-2026-10-10/sent.html).

## Findings

- Canvas rewrote the `<b>` canary as `<strong>` and `<br>` as `<br />`; it also reformatted list and table whitespace.
- The relative CS8 page link became an absolute `https://pearland.instructure.com/courses/121046/pages/...` URL. Canvas added `data-api-endpoint`, `data-api-returntype="Page"`, and `rel="noopener"`. The external link also received `rel="noopener"`.
- The harmless `onmouseover` attribute and `<script>` element were removed.
- Canvas removed `src="about:blank"` from the iframe, kept the iframe and title, and wrapped it in a paragraph.
- Canvas retained `h1`, `h2`, `div`, `span`, `class`, `id`, the tested inline `color: inherit`, image `width`/`height`, `<details>/<summary>`, table `colspan`/`rowspan`, MathML, and the equation-image class. In the teacher page view, the summary appeared as a collapsed disclosure button. I did not test expanding it or its student view.
- Relative image paths were rewritten to absolute same-origin Canvas URLs. MathML remained in the source and displayed as glyphs in the teacher page view. LaTeX-style delimiters remained literal text. The page view inspection was in the teacher session; no student-view claim follows from it.

## What this means for review pages

Canvas's saved page body is the Live reference for this field. A review renderer can safely continue to use the narrower CE-SE allowlist, but should account for Canvas's URL normalization and generated `data-api-*`/`rel` attributes when comparing saved HTML. This probe is evidence for one page body only; it does not authorize widening CE-SE's accepted HTML or establish Canvas behavior for assignment descriptions, quiz content, comments, or New Quiz instructions.

The page remains unpublished in CS8 so the teacher can inspect it. No other Canvas objects were created or changed. Student-view rendering, field length limits, and the other write paths in the broader probe proposal remain pending.

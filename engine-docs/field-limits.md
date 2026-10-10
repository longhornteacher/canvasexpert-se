# Canvas field limits

This is the single source for what Canvas keeps, changes, or shows in each field the Engine writes. Agents read it with `cse help fields`. Other documents link here instead of restating limits; when evidence changes, change this file (and the capability records) and nothing else. `help formats` says what the Engine accepts; this file says what Canvas does with it.

Evidence levels, strongest first:

- **Live API**: the Engine (or an API call) wrote the field in Canvas Live and read it back. Proves storage, not what people see.
- **Live view**: a person looked at it in Canvas (teacher view, Student View, SpeedGrader, or quiz preview; the row says which).
- **Editor**: written or read through Canvas's own editor. Shows editor behavior, not what an Engine write stores.
- **OG**: a record or rule from the original CanvasExpert or QuizForge, not re-checked here.
- **Unknown**: not measured. Treat the field conservatively.

CE-SE has no HTML allowlist of its own. Assignment creates and assignment Hub pages use the Engine's Markdown subset, which emits only `p`, `h1`–`h6`, `ul`/`ol`/`li`, `pre`/`code`, `strong`/`em` (from `**bold**`, `*italic*`, `_italic_`), and `a` to course pages or http(s); anything else is escaped and shows as text. Quiz HTML is sent to Canvas exactly as written.

## Plain-text fields

| Field | What to write | What Canvas does | Evidence | Unknown |
|---|---|---|---|---|
| Submission comment (`comment[text_comment]`, assignment scoring) | Plain text. Line breaks and blank lines are fine; `- ` lines show as literal hyphens, not a formatted list. | Stores the text exactly as the renderer produced it. The student's attempt-feedback panel shows each line break and the blank line before the feedback. | Live API, CS8, 2026-10-09 (assignments 3698077–3698079, revision of comment 1906066). Live view (Student View feedback panel, `[CSE UNATTENDED] Rubric 20261009`), 2026-10-10. | SpeedGrader display. Length cap. |
| Classic per-question comment (Classic quiz writing) | Plain text. Do not rely on paragraph breaks being visible to students. | Stores line breaks. The student's quiz results view runs them together into one paragraph. | Live API and Live view (Student View), CS8, 2026-10-09. | SpeedGrader display (the fixture quizzes were deleted by that run's cleanup; check on the next Classic writing fixture). |
| New Quiz matching and ordering text | Plain text. | Shows as plain text in the quiz player. | Live view (teacher Build → Preview), CS8 quiz 3698128, 2026-10-10. OG guide: tags display literally. | HTML-looking text not tried. |
| Titles (assignment, quiz, page) | Plain text, exactly as the teacher would want it listed. | Saved as written. | Editor (page title), 2026-10-10. Live API for assignments (apostrophe normalization seen 2026-10-09). | Length cap. |

## HTML fields

| Field | What to write | What Canvas does | Evidence | Unknown |
|---|---|---|---|---|
| Assignment description, create; assignment Hub page body | Markdown in the subset above. Bold and italic work in paragraphs, list items, and link text, not in headings. | Rewrites course links to absolute URLs with `data-api-*` attributes. Otherwise keeps the subset. | Live API, CS8, 2026-10-09 (create-then-compare and Hub pages). Live view (Student View), 2026-10-10: before emphasis support, `**two responses**` showed literally. Bold/italic rendering added 2026-10-10 is local tests only. | Every heading level and code escaping not read back individually. |
| Assignment description, update and attach-rubric | Keep Canvas's own HTML; change only what the teacher asked. | The baseline is what Canvas already stored. | Live API, CS8, 2026-10-09. | New markup the agent adds follows the quiz rows below. |
| New Quiz stem, choices, per-choice and neutral feedback, instructions | Ordinary HTML: paragraphs, emphasis, lists, links, `<pre><code>` for code. | Stored the Engine's inline-styled code box and TEKS label exactly as sent, and every item type with feedback. In the quiz player the code box shows dark and colored, and rationales/feedback appear after submitting. | Live API, CS8 quizzes 3698128 and 3698135, 2026-10-09. Live view (teacher Build → Preview), quiz 3698128, 2026-10-10. | Tables, images, other inline styles, math. Student account view. |
| New Quiz stimulus | Not creatable; the Engine copies its HTML into each linked item. | Rejects stimulus creation with HTTP 400. The quiz player shows the copied passage above the item's prompt; there is no separate passage panel. | Live API (OG and CE-SE). Live view (teacher Build → Preview), quiz 3698128, 2026-10-10. | — |
| Classic quiz question text, answers, answer and neutral feedback, instructions, Hub `supports` | Ordinary HTML as above. | Stored types, weights, per-choice comments, and code highlighting as sent. Relative course links stored unchanged. | Live API, CS8 quiz 483146, 2026-10-09; OG for links. | Tables, images, inline styles, math. Student view. |
| Page body written in the Canvas editor (not an Engine path) | — | The editor turned `<b>` into `<strong>`, removed `<script>` and `on*` attributes and an iframe `about:blank` src, rewrote links, and kept the other tested markup (headings, `div`/`span`, class/id, inline color, image size, `<details>`, table spans, MathML). Teacher view showed MathML glyphs. | Editor, CS8 page 2266154, 2026-10-10. | Says nothing about Engine writes. |

## Numbers

| Field | Rule | Evidence |
|---|---|---|
| Classic per-question score | Canvas accepts a score above the question's maximum (31 of 30), so the Engine enforces bounds. | Live API, CS8, 2026-10-09. |

## For review previews

Render the exact Engine payload. Plain-text fields display as plain text, and the preview copies the display behavior recorded above rather than looking better than Canvas. HTML fields render in a sandbox with scripts off; markup not recorded as kept on that field is flagged "not verified." Make no claims about the student view or the quiz player unless the row records a Live view.

Developer records (in the source repository, not the workspace): `docs/canvas-page-html-probe-2026-10-10.md` for the editor probe, `handoffs/active/CANVAS-FIELD-LIMITS-PROBE.md` for pending measurements.

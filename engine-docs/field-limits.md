# Canvas field limits

This is the single source for what Canvas keeps, changes, or shows in each field the Engine writes. Agents read it with `cse help fields`. Other documents link here instead of restating limits; when evidence changes, change this file (and the capability records) and nothing else. `help formats` says what the Engine accepts; this file says what Canvas does with it.

Evidence levels, strongest first:

- **Live API**: the Engine (or an API call) wrote the field in Canvas Live and read it back. Proves storage, not what people see.
- **Live view**: a person looked at it in Canvas (teacher view, Student View, SpeedGrader, or quiz preview; the row says which).
- **Editor**: written or read through Canvas's own editor. Shows editor behavior, not what an Engine write stores.
- **OG**: a record or rule from the original CanvasExpert or QuizForge, not re-checked here.
- **Unknown**: not measured. Treat the field conservatively.

CE-SE has no HTML allowlist of its own. Assignment creates and assignment Hub pages use the Engine's Markdown subset, which emits only `p`, `h1`–`h6`, `ul`/`ol`/`li`, `pre`/`code`, `strong`/`em` (from `**bold**`, `*italic*`, `_italic_`), and `a` to course pages or http(s); anything else is escaped and shows as text. Quiz HTML is sent as written, except that the Engine re-renders `<pre><code>` in item prompts as its code box, wraps plain choice or feedback text in `<p>`, and replaces em dashes (see `help formats`).

## Plain-text fields

| Field | What to write | What Canvas does | Evidence | Unknown |
|---|---|---|---|---|
| Submission comment (`comment[text_comment]`, assignment scoring) | Plain text. Line breaks and blank lines are fine; `- ` lines show as literal hyphens, not a formatted list. | Stores the text exactly as the renderer produced it. The student's attempt-feedback panel shows each line break and the blank line before the feedback. | Live API, CS8, 2026-10-09 (assignments 3698077–3698079, revision of comment 1906066). Live view (Student View feedback panel, `[CSE UNATTENDED] Rubric 20261009`), 2026-10-10. | SpeedGrader display. Length cap. |
| New Quiz matching text | Plain text on one line. | Stores the text exactly as sent, including `<b>`, a bare `&` and `<`, and a line break. The quiz player shows it as literal text: `<b>x</b>` appears with its tags, `&` and `<` appear normally, and the line break becomes a space. | Live API and Live view (teacher Build → Preview), CS8 New Quiz 3698178, 2026-10-10. | Student account view. |
| New Quiz ordering text | Plain text on one line. Inline tags render, so use `<b>` only when bold is meant. | Stores it HTML-normalized (a bare `&` becomes `&amp;`, `<` becomes `&lt;`) and the player renders it as HTML: `<b>x</b>` shows bold, `&` and `<` show normally (no literal `&amp;`), and the line break becomes a space. | Live API and Live view (teacher Build → Preview), CS8 New Quiz 3698178, 2026-10-10. | Student account view. |
| Titles (assignment, quiz, page) | Plain text, exactly as the teacher would want it listed. | Saved as written. | Editor (page title), 2026-10-10. Live API for assignments (apostrophe normalization seen 2026-10-09). | Length cap. |

## HTML fields

| Field | What to write | What Canvas does | Evidence | Unknown |
|---|---|---|---|---|
| Assignment description, create; assignment Hub page body | Markdown in the subset above. Bold and italic work in paragraphs, list items, and link text, not in headings. | Rewrites course links to absolute URLs with `data-api-*` attributes; external links unchanged. Otherwise keeps the whole subset: `h1`–`h6`, `strong`/`em` (including inside list items and link text), lists, escaped raw tags, and a code block with `<`, `>`, `&` still escaped. The only other change is `&#34;` stored as `"` (same text). The assignment page displays all of it as intended: six heading sizes, bold and italic, literal `<b>tag</b>` and `2*3*4`, both lists, both links (external with Canvas's icon), and the code block. | Live API, CS8, 2026-10-09 (create-then-compare and Hub pages); full subset read back, assignment 3698180, 2026-10-10. Live view (teacher assignment page), 3698180, 2026-10-10. | Student View of the new bold/italic (expected the same). |
| Assignment description, update and attach-rubric | Keep Canvas's own HTML; change only what the teacher asked. | The baseline is what Canvas already stored. | Live API, CS8, 2026-10-09. | New markup the agent adds follows the quiz rows below. |
| New Quiz stem, choices, per-choice and neutral feedback, instructions | Ordinary HTML: paragraphs, emphasis, lists, links, tables, `<pre><code>` for code. No `<script>` or `on*` attributes. | Removes `<script>` and `on*` attributes from the stem and instructions; keeps the rest of the tested markup: `h3`/`h4`, `p`, `br`, `strong`/`em`, `b`/`i`/`u`, `sup`/`sub`, inline `code`, `pre`/`code`, lists, a table with `th` and `colspan`, inline `style` (`text-align:center` and the Engine's code box), a LaTeX string as typed, the Canvas equation image (`class`, `data-equation-content`), and relative course-page and external links, left as sent. **Instructions** are otherwise stored byte for byte. **Stem** (`item_body`) differences: line breaks are added between list and table tags, attribute quotes are re-serialized (`'` to `"`, inner `"` to `&quot;`, `&#34;` to `"`), and the read returns a course file image's `src` as a signed file-service URL that expires (token issued at read time, about 13 hours), with the original path kept in `data-old-link`. Choice text and per-choice feedback are stored exactly as sent. In the quiz player (teacher Build → Preview) all of it displays: the course image, the centered inline style, the LaTeX string rendered as math, the equation image, the code box dark and colored with `<` and `&` correct, and a choice with bold, italic, superscript, inline code, and a link with Canvas's external-link icon; rationales/feedback appear after submitting. | Live API, CS8 quizzes 3698128 and 3698135, 2026-10-09 (neutral feedback, all item types); canaries, New Quiz 3698178, 2026-10-10. Live view (teacher Build → Preview), quizzes 3698128 and 3698178, 2026-10-10. | The table was not in the inspected screenshots. Student account view. |
| New Quiz stimulus | Not creatable; the Engine copies its HTML into each linked item. | Rejects stimulus creation with HTTP 400. The quiz player shows the copied passage above the item's prompt; there is no separate passage panel. | Live API (OG and CE-SE). Live view (teacher Build → Preview), quiz 3698128, 2026-10-10. | — |
| Page body written in the Canvas editor (not an Engine path) | — | The editor turned `<b>` into `<strong>`, removed `<script>` and `on*` attributes and an iframe `about:blank` src, rewrote links, and kept the other tested markup (headings, `div`/`span`, class/id, inline color, image size, `<details>`, table spans, MathML). Teacher view showed MathML glyphs. | Editor, CS8 page 2266154, 2026-10-10. | Says nothing about Engine writes. |

## For review previews

Render the exact Engine payload. Plain-text fields display as plain text, and the preview copies the display behavior recorded above rather than looking better than Canvas. HTML fields render in a sandbox with scripts off; markup not recorded as kept on that field is flagged "not verified." Make no claims about the student view or the quiz player unless the row records a Live view.

The probe reports and payloads behind these verdicts (2026-10-10, CS8) are in the source repository's Git history.

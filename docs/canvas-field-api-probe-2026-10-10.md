# Canvas field API probe (CS8)

**Date:** 2026-10-10
**Course:** CS8, course 121046, confirmed live by `cse courses` and a fresh course read as "FUND OF COMPUTER SCIENCE - Beckham"
**Authorization:** teacher, 2026-10-10, for new unpublished probe objects in CS8 only
**Brief:** [CANVAS-FIELD-LIMITS-PROBE.md](../handoffs/retired/CANVAS-FIELD-LIMITS-PROBE.md). Per-field verdicts live in [field-limits.md](../engine-docs/field-limits.md); this report does not restate them as rules.

## Objects created (all unpublished, left for teacher inspection)

| Object | ID | Path |
|---|---|---|
| New Quiz `[CSE FIELD PROBE] New Quiz HTML canaries 2026-10-10` | assignment/quiz 3698178 | `cse push-quiz CS8` |
| Assignment `[CSE FIELD PROBE] Assignment Markdown subset 2026-10-10` | 3698180 | `cse push` (operation create) |

No students, submissions, comments, or existing content were written. The image canary references existing course file 18945746 (an image already in the course) without changing it. The relative-link canary points at the earlier probe page 2266154.

## Method

The Engine was built from `bed516b` (`go build ./cmd/cse`) and run in the isolated, git-ignored workspace `.test-workspaces/field-probe-20261010/`. Read-back used a new read-only `cse-probe field-read` command. It returns only objects whose title starts with `[CSE FIELD PROBE]`, in CS8, and each read was a fresh request after the writes completed. The browser and the OG `canvas-expert` connector were not used.

"Sent" bodies were produced by calling the same Engine transform functions on the same files, in a temporary local test that was deleted afterward. They match the HTTP bodies except for random choice UUIDs, which are regenerated on each transform. Choices and feedback were compared by position.

Evidence in [`evidence/cs8-field-api-probe-2026-10-10/`](evidence/cs8-field-api-probe-2026-10-10/): the canary sources (`*-canary.*`), sent bodies (`*-sent.json`), and returned objects (`*-returned.json`). The returned files were redacted: the signed file token, the file `verifier`, and the assignment's `secure_params` and `lti_context_id` are replaced by `REDACTED`.

## Payloads

- **Full canary** (New Quiz instructions and MC stem): `h3`, `h4`, `p` with `strong`/`em`/`b`/`i`/`u`/`sup`/`sub`, inline `code`, a `br`, `ul`, `ol`, a relative course-page link and an external link, a table with `thead`/`th` and one `colspan`, `img` with `alt` pointing at `/courses/121046/files/18945746/preview`, `<p style="text-align:center">`, the LaTeX string `\(x^2 + 3x = 10\)`, a Canvas equation image (`class="equation_image"`, `data-equation-content`), a `pre`/`code` block with `<`, `>`, `&`, a `<p onclick>`, and a `<script>`.
- **Short subset** (one choice and its per-choice feedback): `strong`, `em`, `sup`, inline `code` with `&lt;`, an external link.
- **Plain-text items**: matching left `<b>x</b> = 5 & y < 3` plus a newline and a second line; matching right `Right <i>one</i>`; the same string as the first ordering item.
- **Assignment Markdown**: every heading level `#` to `######`, `**bold**`, `*italic*`, `_italic_`, a raw `<b>` that must stay text, `2*3*4`, bullet and numbered lists with emphasis and a course-page link, an external link with bold link text, and a fenced code block with `<`, `>`, `&`, and `"`.

### What the Engine changed before sending

- Item prompts (New Quiz `item_body`): the `<pre><code>` block became the Engine's inline-styled Monokai box with colored `<span style='...'>` tokens, and `"` inside it became `&#34;`. Instructions/description are not touched.
- A choice whose text does not start with `<` is wrapped in `<p>` (`Plain choice`, `Plain feedback.`).
- Markdown renders `"` in code as `&#34;`, and the raw `<b>` as `&lt;b&gt;`.
- No em dashes or TEKS metadata were in the canaries, so those rewrites did not apply.

## Differences returned, per field

Whitespace, attribute order, and the known course-link rewrite are listed but are not counted as content loss.

**New Quiz instructions:** identical to what was sent except that `<script>alert(1)</script>` is gone and `onclick="alert(1)"` is gone. The relative page link and the relative image `src` are unchanged.

**New Quiz stem (`item_body`):** `<script>` and `onclick` removed. Newlines were inserted between `ul`/`ol`/`li` and table tags. Attribute quoting was re-serialized: `style='...'` became `style="..."` with inner `"` as `&quot;`, and `&#34;` text became `"`. The image `src` came back as `https://inst-fs-iad-prod.inscloudgate.net/files/<uuid>/2bears1.png?token=<JWT>` with `data-old-link="/courses/121046/files/18945746/preview"`. The token's issue time matches the read and it expires 13 hours later, so it appears to be minted on read. The relative page link was not absolutized. Table, `colspan`, `sup`/`sub`, `b`/`i`/`u`, inline styles, LaTeX text, and the equation image with its `class` and `data-equation-content` were all kept.

**New Quiz choice and per-choice feedback:** identical, both the subset choice and the `<p>`-wrapped plain ones.

**New Quiz matching:** questions and answers identical to what was sent, raw `&`, `<`, `<b>`, `<i>`, and `\n` included.

**New Quiz ordering:** the choice came back as `<b>x</b> = 5 &amp; y &lt; 3\nsecond line`. Tags and the newline were kept, and the bare `&`/`<` were entity-escaped. Ordering text goes through an HTML sanitizer; matching text does not.

**Assignment description:** identical except the course-page link became absolute with `data-api-*`, and `&#34;` came back as `"`. All six heading levels, `strong`/`em` (including in list items and link text), the escaped raw tag, `2*3*4`, and the code block's `&lt;`, `&gt;`, `&amp;` were kept.

Nothing was rejected; every write returned success and the item counts matched.

## Surprises and findings (no Engine change made)

1. **Ordering text is escaped and matching text is not.** An ordering item that says `a < b` or `Tom & Jerry` is stored as `a &lt; b` / `Tom &amp; Jerry`. If the player shows ordering text as plain text, students may see the entity literally. This needs a human look at 3698178. If it shows literally, it is a finding against what authors commonly write.
2. **New Quiz item images come back as expiring signed URLs.** A course file `img` in a New Quiz item does not round-trip: the read returns a file-service URL with a 13-hour token and keeps the original path in `data-old-link`. The instructions field does not do this. An edit or preview diff of New Quiz items would need to treat `data-old-link` as the stored source. `help formats` still says images are not supported by `push-quiz`; storage accepted one.
3. **Script and `on*` are stripped on every HTML field tested.** That is expected and not something quiz authors use.
4. Inline styles, tables with `colspan`, `b`/`i`/`u`, `sup`/`sub`, and the equation image were kept. The style is kept, with its quotes re-serialized.

## Not tested, and why

- **Neutral (item-level) feedback HTML:** not in this canary. It was stored with all item types on 2026-10-09.
- **Rendering:** the API read-back proves storage only. The teacher's look at the canary quizzes and assignment is still pending; see field-limits "Unknown".

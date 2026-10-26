# HANDOFF: Let the WebUI use the browser width

Date: 2026-10-26. To: whoever picks this up in Dev (CE-SE repo, `dev` branch).

This document stands alone.

---

## 0. TL;DR

The local WebUI (`internal/localui`) caps every page at a fixed card width (`main` 58rem, `main.wide` 78rem ≈ 1250px), centered with large empty margins. On a normal 1080p/1440p monitor the Students page wastes about a third of the screen, and the open **Contact details** editor runs off the right edge of the card into a horizontal scroll strip. Remove the pointless cap so hub pages fill the window (with sane gutters), and make the Students table lay out well when a contact editor is open.

This is a CSS change. No Engine behavior, routes or data change.

---

## 1. What the teacher sees

Screenshot (Students → ELA7, contact details open, ~2000px-wide browser window):

- The page card is ~1250px wide in a ~2000px window. Roughly 375px of empty page on each side.
- The Contact details column is clipped on the right: the parent Email inputs and their Yes/No radios are cut off at the card edge. The teacher has to scroll the table sideways inside the card.
- With one contact editor open, the row becomes very tall, and Student / Made-up name / Goes By / tag / checkboxes float in the vertical middle of a mostly empty row, far from the contact editor at the top.

---

## 2. Where the constraints live

All in `internal/localui/`:

| Location | Rule | Effect |
|---|---|---|
| `assets/hub.css` line 2 | `main{max-width:58rem; …}` | Default cap for hub pages (used by `setup.html`, `students_roster.html`, and the small "done" pages) |
| `assets/hub.css` line 2 | `main.wide{max-width:78rem}` | Cap for Home, Classroom (Courses/Students/Team), Settings, Guide, Capabilities, Connect |
| `assets/hub.css` line 1 | `body{padding:1.25rem}` | Outer gutter; fine to keep |
| `assets/hub.css` line 12 | `table.students{min-width:48rem}`, `.table-scroll{overflow-x:auto}` | Table scrolls inside the card when content exceeds the card |
| `assets/hub.css` line 12 | `td{vertical-align:middle}` | Why the other cells float to the row middle |
| `assets/hub.css` ~line 29 | `.students td:has(.student-contacts[open]){vertical-align:top}` | Only the *contact* cell is top-aligned, not its siblings |
| `assets/hub.css` ~line 31 | `.student-contacts{min-width:27rem;max-width:32rem}` | Forces a wide last column; with the 78rem card this overflows |
| `assets/hub.css` lines 6, 11, 14 | `p.lead{max-width:54rem}`, `.reassure{max-width:58rem}`, `.settings-section.voice textarea{max-width:58rem}` | Reading-width caps on prose. **Keep these** — long lines of prose are hard to read |
| `templates/students_roster.html` line 9 | inline `main{max-width:46rem}` | Separate non-hub roster page |
| `assets/review.css` | `header,main{max-width:66rem}`, footer bar matches 64rem | Review page (grading/parent review). See §4 |

Note: the screenshot was taken in UserInstall and its button text ("Print name key", "Name key for all courses") differs from current Dev `templates/students.html` ("Print Student<->Pokemon table"). Check which is newer before editing that template; the CSS problem is the same either way.

---

## 3. The fix

1. **Widen the hub card.** Change `main.wide` to fill the window: `max-width:none` (or a generous ceiling such as `min(100%, 120rem)` so a 4K ultrawide doesn't get absurd line lengths in tables). Keep `body` padding and the card's own padding/border/radius/top gradient as they are. Plain `main` (setup, roster, done pages) can stay narrow; those are short forms where a narrow card reads better.
2. **Keep prose readable.** Leave the existing `max-width` on `p.lead`, `.reassure`, and the voice textarea. If any other page's body copy now spans the full width and looks bad (Guide/how, Capabilities, Home intro), give that text block a ~60–70ch `max-width` rather than re-capping the whole card. Grids (`.cards`, `.home-status-grid`) already use `auto-fit`/`minmax` and will simply get more columns or wider cards — check they still look right.
3. **Students table with contacts open.**
   - Top-align the whole row when its contact editor is open: `.students tr:has(.student-contacts[open]) > td{vertical-align:top}` (replace the single-cell rule). Give those cells a little top padding so the inputs line up with the "Contact details" summary.
   - With the wider card, the contact column should fit without horizontal scroll at common desktop widths (≥1366px). Keep `.table-scroll` as the fallback for narrow windows; don't remove it.
   - Reconsider `.student-contacts{min-width:27rem}`: it is right when open but forces width while closed too. Scope the min-width to `[open]` (`.student-contacts[open]{min-width:27rem}`), so a closed table stays compact.
4. **Review page (`review.css`)** — optional, same idea: raise `header,main` from 66rem and the footer's matching 64rem calc to something wider, keeping `.intro` and comment text at reading width. Do it in the same change only if it's trivial; it isn't what the teacher hit.

Don't add a width setting, a toggle, or a layout framework. One CSS edit set.

---

## 4. Check

- Run the hub (`run` skill or the usual local UI start) and look at, in a ~1920px window and a ~1366px window, dark and light:
  - Classroom → Students with one and two contact editors open: no clipping, no sideways scroll at 1920; at 1366 sideways scroll is acceptable only if the open editor genuinely can't fit.
  - Classroom → Courses, Team; Home; Settings; Guide; Capabilities: nothing looks stretched or broken; prose stays at reading width.
  - Phone width (≤40rem): unchanged behavior (existing `@media(max-width:40rem)` rules).
  - Print from Students (`Print name key` / reference table): unaffected (print CSS hides `main`).
- `go test ./internal/localui/...` — some tests assert on page HTML/CSS; update only assertions that pinned the old widths.
- No change to `capabilities.go` / `docs/engine-capabilities.md` is needed: this is layout, not a capability.

## 5. Done when

On the teacher's monitor, the Students page uses the window width, the open Contact details editor is fully visible with its row's other fields aligned to the top, and other hub pages look at least as good as before.

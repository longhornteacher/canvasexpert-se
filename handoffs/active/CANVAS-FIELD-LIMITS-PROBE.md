# Canvas field-limits probe

2026-10-10, revised after the page-body probe. Proposal for the remaining probe. The first run is recorded in [the page probe report](../../docs/canvas-page-html-probe-2026-10-10.md) and [the field-limits table](../../engine-docs/field-limits.md). No Playbook change.

## Why

[REVIEW-PAGE-VISION.md](REVIEW-PAGE-VISION.md) says the review page may show nothing Canvas cannot show. The review page renders the Engine's payload. So the useful question is narrow: **for each field, does Canvas keep what the Engine actually sends on that field?** It is not "what is Canvas's sanitizer allowlist." When Canvas keeps more than the Engine can emit, that does not matter to the preview. What matters is when Canvas strips or rewrites something the Engine sends.

This gates slice 3 (authoring preview) and slice 4 (edit preview). It does not gate slice 1, apart from the submission-comment note below, which is already on record.

## What the first run established, and what it did not

The page probe was saved through the Canvas Pages **rich content editor** and read back by reopening that editor's HTML source view. The editor normalizes HTML on save and again on load. So the stored diff mixes the editor's changes with Canvas's server sanitizer. It is not evidence of what Canvas stores from an API write. The Engine writes only through the API. Treat that run as "Canvas editor behavior, teacher view." Do not treat it as evidence for an Engine write path.

The run also went to the browser because the Canvas Expert connector returned `legacy_storage_reappeared`. That connector is the **original Python CanvasExpert MCP server**, not CE-SE. The error comes from its retired-storage guard (`CanvasExpert/api/shared_storage.py`), which tripped because a retired `settings.json` reappeared in the OG OneDrive workspace on 2026-10-08. The same thing happened on 09-24. It does not affect `cse`. The next run uses `cse` directly. It does not use the OG connector or the browser editor.

Some of the run's observations are corroborated elsewhere, through the API. Canvas absolutizes course-page links and adds `data-api-*` attributes. The 2026-10-09 hub work found this live, and `academicHTML` in `internal/workflow/edit_plan.go` already normalizes it. The `<b>`→`<strong>` and `<br>`→`<br />` rewrites look like editor behavior, and an API write is not expected to show them.

## What the Engine actually sends, by field

| Field | Engine path | What the Engine can emit | Probe needed? |
|---|---|---|---|
| Assignment description (create), Hub tier page body for assignments | `render.MarkdownHTML` | Only `p`, `h1`–`h6`, `ul`/`ol`/`li`, `pre`/`code`, and `a` to course pages or http(s). All other HTML is escaped as text. | Mostly answered. Create-then-compare already round-trips through `academicHTML` live. Fold one API read-back of the full subset into the run: every heading level, a code block with `<`/`&`, both link kinds, and `**bold**`/`*italic*` (renderer support added 2026-10-10; Canvas should keep `strong`/`em`). |
| Assignment description (update/attach-rubric) | Preserves Canvas's own HTML; edits come from the agent | Whatever Canvas already stored, plus the agent's edits | Not a sanitizer question for the baseline. The edit preview diffs strings, which `academicHTML` already handles. Covered by the quiz-field result for new agent-added markup. |
| New Quiz stem, choice text, per-choice/neutral feedback, quiz instructions | `push-quiz` (QuizForge JSON) | **Raw agent HTML, passed through unfiltered.** CE-SE has no HTML allowlist. The allowlist cited in earlier docs is OG CanvasExpert's. | **Yes.** This is the main gap. |
| Classic quiz question text, answers, answer/neutral feedback, quiz instructions, Hub tier `supports` | Classic quiz writer | Raw agent HTML, unfiltered | **Yes.** |
| New Quiz matching/ordering text | `push-quiz` | Whatever the agent writes | Yes, plain-text behavior only. Put an HTML-looking string and a line break in one item. |
| Submission comment (`comment[text_comment]`) | Feedback renderer | Plain text | Storage is live-verified. The run-together display seen on 2026-10-09 was on Classic *question* comments, not submission comments. **Teacher decision:** before slice 1 fixes how comments display, a human looks at one *existing* probe submission comment that has blank-line paragraphs, in both SpeedGrader and Student View. No new write. Record what each view shows. |
| Classic per-question comment, per-question score | Classic scoring post | Plain text, number | Already live-verified. Not re-run. |
| Titles | All writers | Plain text | Not probed. Show exactly. No length hunt. |

## The probe

The probe uses **one New Quiz and one Classic quiz**, both unpublished, titled `[CSE FIELD PROBE] …`, in CS8. It also creates one throwaway assignment for the Markdown-subset read-back. Write through the **real Engine paths**: `cse push-quiz` and the Classic quiz writer, which already pass HTML unmodified, and assignment create. Read back through the API in a fresh request. Do not use the browser editor for writing or reading back.

Canaries for the HTML fields should be **what a quiz author realistically writes**, not a sanitizer survey:

- `p`, `br`, `strong`/`em`, `b`/`i`/`u`, `sup`/`sub`, `code`/`pre`, `ul`/`ol`/`li`, `h3`/`h4`, and `a` (relative course page and external)
- a small `table` with `th` and one `colspan`
- `img` with `alt`, using a course file URL; a `/favicon.ico` canary does not represent real use
- one inline `style` (for example `text-align:center`), since OG noted styles get re-serialized. New Quizzes already stored the inline-styled Monokai code box exactly as sent on 2026-10-09, so the open question is Classic.
- math as a teacher would actually enter it: one LaTeX-style string, plus the Canvas equation-image form if the agent can produce it
- `script` and one `on*` attribute, only to confirm that Canvas strips them on these fields too (no exploration beyond that)

Put the full canary body in the stem and instructions. Use a short subset in one choice, one feedback, and the matching/ordering text. Do not repeat the full body in every field.

Then diff sent against returned, normalizing whitespace, attribute order, and the known link rewrite. Record every remaining difference, or rejection, per field.

Skip iframe, `details`, MathML, `div`/`span`/`class`/`id`, and `h1`/`h2` on quiz fields. A quiz author would not normally use them, and the Engine does not need to preview them. If the agent ever emits one, the preview flags it as unverified (see below).

## How the preview uses the result

- **Markdown-subset fields:** render the payload, with the known link rewrite applied. Faithful.
- **Raw-HTML quiz fields:** render the payload in a sandboxed frame with scripts off. Tags and attributes that survived the probe on that field render normally. Anything else is shown and flagged "Canvas may change this; not verified." Anything Canvas stripped is flagged "Canvas removes this." The preview does not need an exhaustive sanitizer map.
- **Submission comments:** plain text. See the open question on paragraph spacing.
- Rendering claims (student view, New Quiz player) stay "pending human validation" until a person looks. The API read-back proves storage only.

## Teacher eye checks on existing CS8 objects (no writes)

**Done 2026-10-10; results are in `engine-docs/field-limits.md`.** Check 1 was done in Student View only (SpeedGrader not checked). Check 2 was not possible because the run's cleanup had deleted the fixture quizzes. Check 3 is done. The list stays here for reference.

1. **Submission comment paragraphs.** Assignment 3698078 (rubric, Test Student). The comment has a score line, criterion lines, a blank line, then feedback. In SpeedGrader and then in Student View (Grades → assignment → comment): are the criterion lines on separate lines? Is the blank line before the feedback visible, or does it run together? Repeat on 3698077 if 3698078 has no comment.
2. **Classic question comment, teacher side.** Any `[CSE UNATTENDED] Classic writing` quiz still in CS8, opened in SpeedGrader: does the `Assessment: 8/10.0` comment show its line break? Student View already runs it together. Skip this if the quizzes were deleted.
3. **New Quiz player.** Quiz 3698128 (all-types sampler, unpublished) → Build → Preview. Check that the code box is dark and colored, that the stimulus passage appears in its items, that matching and ordering text is plain, and that rationales and feedback appear after submitting the preview.

## Human check after the API probe (short)

Do this after the API pass. In the Classic quiz preview, and in the New Quiz teacher preview because Student View is blocked for New Quizzes, look at the canary stem and confirm that what was stored is what displays. Record what was and was not inspected. Editor preview is not proof of student view.

## Output

1. Update `engine-docs/field-limits.md`, the single source (served by `cse help fields`). Move each probed field's "Unknown" items into "What Canvas does" with the evidence level, IDs, and date. Do not restate limits anywhere else; other docs link to it.
2. A short probe report beside the first one, with raw sent/returned bodies under `engine-docs/evidence/`. Include no credentials, student data, or teacher notes.
3. Update `internal/localui/capabilities.go` and `docs/engine-capabilities.md` together, and keep the evidence types distinct: API read-back is Canvas Live storage evidence; editor-path and rendering observations are labeled as such.
4. Findings only, with no Engine change in this task. Example: "Canvas strips X that QuizForge allows." Whether to add a minimal pre-write check for raw quiz HTML is a separate decision. Raise it only if the probe shows Canvas silently dropping something authors commonly use.

## Cleanup

Leave the probe objects unpublished for teacher inspection and list their IDs, unless the teacher chooses deletion. Record IDs as they are created.

## Stopping rule

Stop when each "Yes" row above has an API read-back verdict (kept, rewritten, stripped, or rejected), the human glance is recorded, and the docs are corrected. If a field cannot be reached, record it as "not tested" with the reason. Do not hunt for length caps, chase exhaustive tag coverage, or re-test live-verified behavior.

## Execution notes

- Confirm the course against Canvas Live before writing, as in [ENGINE-VALIDATION.md](ENGINE-VALIDATION.md). Publish nothing. Touch no real students or existing content.
- Use the `cse` CLI, not the `canvas-expert` MCP tools, which belong to OG CanvasExpert. If `cse` cannot reach Canvas, stop and report the error. Do not fall back to the browser editor for writes. That path does not answer the question.
- Keep it proportionate: no new framework and no probe-only writer if the Engine writer can carry the payload. `cmd/cse-probe` is fine for any read-back helper.

## Record of the first run

- 2026-10-10, CS8 (course 121046): one unpublished Page 2266154, created through the Canvas editor and left for inspection. There were no student writes.

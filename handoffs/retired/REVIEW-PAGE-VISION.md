> **Retired 2026-10-10.** Slices 0-4 are built (3e29ebd, beab31b, 7f09bf4, 927f436); status and pending human checks live on the Capabilities page and in docs/engine-capabilities.md. Slice 5 is on demand and slice 6 (retirement pass, e.g. the "nice preview" line in playbooks/score.md) is gated on real use; open a new handoff for either when needed. The "no review-page implementation" line below is historical.

# Review page: vision and slices

2026-10-10. Proposal for review by higher-level agents, not an implementation authorization. No review-page implementation has been built. Field limits live in [field-limits.md](../../engine-docs/field-limits.md). This touches Engine (rendering, the review command) and Playbook (when the agent prepares and what it does with revisions).

## Idea

Whenever the agent prepares work for Canvas (scores, assignments, pages, quizzes, edits), the teacher can see it rendered in one review page on the desktop before it is written. Prepared items start as **approved**. The teacher scans, clicks **Revise** on anything that needs the agent's attention (with a note), and clicks **Post** at the bottom. Post writes the approved items only. Revised items go back to the agent. Held or failed items are called out individually and never block the rest.

Desktop or laptop only. No phone layout.

**Review is the default flow.** The agent prepares, the teacher reviews, then it posts. The teacher can skip review by saying so ("just post it"), but the default is measure twice, cut once: the review costs the teacher a short scan and saves the larger cost of finding a wrong score, comment, or quiz question after it reaches students.

**Review should cost the agent nothing extra.** The agent already produces the scores, assignment, page, or quiz content. The Engine turns that existing file into the page. The agent writes no HTML, no second copy of the content, and no summary for the page. Any added agent step is one command (`cse review`) and reading its short result. If a slice adds token cost beyond that, it has to justify it.

**Staging.** This page may become the staging step, with Canvas Live reserved for what is real and published. Today some flows use Canvas itself as staging (create unpublished, then the teacher reviews in Canvas). The higher-level agents should consider whether the review page replaces that role. See the open questions.

## Why it fits what exists

- The **results/work file is already the proposal**. The agent writes assignment results and edit plans as plain workspace files; `cse validate` already computes the honest assessment and shows held versus ready students. The review page is a rendering of that, not a new data model.
- The Engine already runs **temporary loopback pages** that live only while their command runs (`internal/localui`). A `cse review <file>` command is the same shape. No resident service, no database.
- `playbooks/score.md` already tells the agent to hand-build a "nice preview" with the desktop app's artifact features when a choice is easier to judge by looking. Those previews are agent-authored and ad hoc. The review page replaces them with an Engine-authored one.

## Principles

1. **The Engine renders the preview from the exact payload it will post.** An agent-written HTML preview can drift from what gets written. The page shows validated output (computed scores, resolved identities, the actual comment text, teacher-only notes marked as such), and Post sends that same data.
1a. **The page may show nothing Canvas cannot show.** A preview that looks better than what students will see is worse than no preview. Each previewed field is rendered under the constraints of the Canvas field it will be written to:
   - **Plain-text fields** (submission comments via `comment[text_comment]`, and any other field Canvas treats as plain text) display as plain text, line breaks preserved, with no bold, links, or lists. Feedback in the page should look like the existing renderer's output, because it is that output.
   - **HTML fields** (assignment descriptions, pages, quiz prompts and feedback) display only what Canvas keeps. Canvas sanitizes HTML and its editor supports a limited set of tags, attributes, and styles. The page uses the same render path that produces the payload (for example `render.MarkdownHTML`, a deliberately small subset) and shows the result in a sandboxed frame with scripts disabled. Our page CSS must not leak into it, so the preview does not flatter the content with styling Canvas will not apply.
   - **Content Canvas would strip, rewrite, or reject** is flagged on the card rather than silently displayed in its intended form. Better still, the Engine already catches it at validation, so the page and the post agree.
   - Field-level limits are recorded once, in `engine-docs/field-limits.md`, with the evidence level for each. Where a limit is unknown, the page treats that field conservatively (plain or minimal) and says the limit is unverified. It does not guess generously.
   - The page also cannot imply Canvas behavior it has not shown: no student-view claims, no rendering of Canvas-side features (equation rendering, embedded media, quiz item display) that the Engine cannot reproduce. Say "shown as text" or "preview limited" instead.
2. **Approve-by-default, loud exceptions.** The Post click is the teacher's consent. So held items, flags (late timing, overrides, unknown evidence, changed live data), and anything the Engine could not verify must be visually unmissable. Routine items collapse.
3. **Canvas Live stays authoritative.** Before each write, re-check identity and staleness exactly as `post` does today. If Canvas changed since the proposal, that item shows as stale. The rest proceed.
4. **Unsent work is never lost.** Revise notes and review state are saved in the results file. Closing the page loses nothing.
5. **No framework.** No generic operation, queue service, or plugin renderer. One renderer per work kind, added only when a slice needs it. Shared code emerges from the second kind, not before.

## The agent loop

Lead hypothesis: the agent runs `cse review <file>` as a command in its own session (in the background so it is notified on exit). It opens the page in the desktop app's browser pane. The teacher reviews. The command **exits when the teacher clicks Post or "Send revisions"**, and its output tells the agent what happened: items posted, items revised with the teacher's notes, items held. That gives a structured Revise channel with no polling and no new protocol. If background-command lifetime proves fragile, the fallback is notes saved to the results file plus the teacher telling the agent "check revisions."

## Tensions with current docs (decide, then amend)

- `ARCHITECTURE.md` says "no mandatory preview/apply tokens" and that "show me first" stops for review, and `SCORING-RULES.md` and the Playbooks treat "score and post" as authorization to post. **Decided direction (teacher):** review is the default gate. "Score and post" still defines the *scope* of what gets posted, but the agent stops at the review page first unless the teacher explicitly skips review. This reverses the current default, so `ARCHITECTURE.md` (teacher decisions section), the scoring Playbook, and the capabilities page need amending. A skip is an explicit teacher statement for that task, not a standing inference. There is no approval token: the gate is the agent's instruction plus the `cse review` command, not a lock in the Engine.
- `ARCHITECTURE.md` says "no pending state or approval polling" for authoring. A review page with an exit-on-click command is not polling, and "pending" stays a local file state. Confirm that reading.
- It says no persistence layer or job engine. This design uses workspace files only. Keep it that way.

## Where this could remove complexity (verify in slice 1; do not assume)

- The agent-built preview instructions in the Playbooks, and the variation between how each Playbook asks for conversation-based review.
- Conversation-only tracking of "which items did the teacher flag." Revise gives it one structured place.
- Text-only `validate` output as the teacher's main window into a result. The page becomes the readable view and the CLI output can shrink.
- Resolution of held items (unreadable evidence, ambiguous attempt) as chat back-and-forth. The page can show them and take a teacher decision or note.
- The "unpublished until requested pieces succeed" and "pending my finalization" authoring flow might be simplified once review happens before the write: if staging happens in the page, Canvas no longer has to hold half-reviewed drafts. Treat this as a question for the authoring slice, not a promise.

If a slice does not retire something concrete, say so in its close-out rather than claiming simplification.

## Field limits

[`engine-docs/field-limits.md`](../../engine-docs/field-limits.md) is the single source for what Canvas keeps or shows in each field, served to agents by `cse help fields`. This document does not restate limits. Its "For review previews" section is the rule the page follows.

## Slices

Each slice delivers something a teacher can use and has its own short brief. Each updates `internal/localui/capabilities.go` and `docs/engine-capabilities.md` together, keeping Canvas Live evidence, local tests, and pending human validation distinct. Local tests alone are not live proof.

**0. Decisions.** Resolve the open questions below, especially staging. Amend `ARCHITECTURE.md`, `SCORING-RULES.md`, and the Playbooks for the review-by-default reversal and anything else decided. Output: a short decisions note and doc amendments, not code.

**1. Scoring review.** (Uses `field-limits.md` for the fields it displays: submission comments. Plain-text fields are the simplest to render faithfully.) `cse review <results-file>` for ordinary assignment results. Renders per-student cards (scores, computed total, rubric lines, student-facing comment, teacher-only notes marked), held items with reasons, flags. Approve-by-default, Revise with note, Post approved. Post reuses the existing `post` path; it does not duplicate write logic. Stale or changed results file after render means re-render before posting. Local tests plus a human walkthrough on synthetic results.

**2. Revise return and agent loop.** The exit contract above, the Playbook paragraph that tells the agent to run `cse review` by default before posting (and when the teacher has skipped review, to post directly), how to act on revisions, and re-review after the agent revises (changed items marked, unchanged approvals kept). The slice that decides whether the command-exit approach holds.

**3. Authoring preview.** Assignment, page, and QuizForge quiz previews: the assignment or page HTML in a sandboxed frame; the quiz item content with a teacher-only answer-key toggle. Uses `internal/render`. Shows rich content only as far as `field-limits.md` records it kept. A quiz preview shows item content as the Engine will write it, not a simulated Canvas student view, unless that view is verified. Includes the question of whether review-before-write changes the create flow.

**4. Edit preview.** Assignment and page edits as a diff of Canvas Live against the proposed change, built on the existing edit plan. Natural fit: the plan already separates requested changes.

**5. Other kinds on demand.** Feedback revision, Hub tier pages, rubric attachment. Add one only when a teacher task needs it.

**6. Retirement pass.** After slices 1 to 2 are used for real, remove what the page demonstrably superseded (ad hoc preview instructions, redundant validate formatting). Gated on use, not scheduled.

## Out of scope

Phone or tablet layouts, multi-teacher review, a persistent review history or audit ledger, auto-approve rules, a generic review framework, notifications, and any change to scoring semantics. Feedback voice and grading judgment stay in the Playbooks.

## Open questions

Decided by the teacher: review gates posting by default and can be skipped on request; a summary line above Post ("Post 24 approved, 2 held, 1 revised") is enough for now.

1. **Staging.** Should the review page be named and treated as the staging step, with Canvas Live no longer used as staging? Considerations for the higher-level agents:
   - It must not weaken "Canvas Live is authoritative." Staging is local and pre-write; existing Canvas content still wins on conflict.
   - The page cannot show everything Canvas shows. Canvas's own rendering, quiz student experience, and Hub access are the fidelity limits. An unpublished Canvas object may still be the better check for those, so decide per kind whether the page replaces it or precedes it.
   - Decide whether "staging" is a user-facing word or just the design idea.
2. **Browser compatibility.** The target is the page opening well in the Claude desktop app's built-in browser pane, and ideally the equivalent pane in the ChatGPT app. The Claude pane is a Chromium-based browser, and the ChatGPT app's embedded browser is likely similar, but that is unverified. So build conservatively: server-rendered HTML with inline CSS and a small amount of plain JavaScript, no external CDN or font dependencies, no build step, and nothing Chromium-specific. Check the first slice in the Claude pane and in a normal Chromium browser. Treat ChatGPT verification as pending human validation, not an assumption. Also confirm the loopback page loads in each pane (some embedded browsers restrict local addresses).
3. **Field limits source.** Decided: `engine-docs/field-limits.md` is the single source, with an evidence level per row. Unknown fields preview conservatively and are labeled not verified.
4. **Exit-on-click command.** Can the command work reliably as a background command in the agent's session on Windows? Slice 2 should test this early with a trivial page before the full renderer depends on it.
5. **Cowork.** Loopback pages and the Windows-only Engine may not carry over. Not a design driver now; do not distort the page to anticipate it.

## Review guidance

Judge against the teacher workflow and the boundary above. Reward small slices that reuse results files, `validate`, `post`, and `localui`. Do not reward added infrastructure, generic abstractions, or exhaustive edge-case handling for rare, visible, cheaply repairable authoring mistakes. Keep cheap checks for wrong-target writes, privacy exposure, dishonest grades, and loss of unsent work.

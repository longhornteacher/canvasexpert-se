> **Retired 2026-10-10.** Slice 0 done; decisions are reflected in ARCHITECTURE.md and the review page (slices 1-4).

# Review page: slice 0 decisions

2026-10-10. Resolves the open questions in [REVIEW-PAGE-VISION.md](REVIEW-PAGE-VISION.md). Documentation only; no code and no review page exists yet.

## Decisions

1. **Review gates posting by default (teacher decision).** Once `cse review` exists, the agent stops at the review page before posting, unless the teacher explicitly says to skip it for that task ("just post it"). "Score and post" still sets the *scope* of what gets posted. A skip is per task, never a standing inference. There is no approval token; the gate is the Playbook instruction plus the command.
2. **Staging is a design idea, not a teacher-facing word.** The page is "review before write": local, pre-write, and subordinate to Canvas Live. Per kind: for scoring it replaces the conversational/artifact preview; for authoring, quizzes and edits it *precedes* the unpublished-in-Canvas check, because the page cannot show Canvas's own rendering, the New Quiz student experience, or Hub access. Whether review-before-write simplifies the create flow is a slice 3 question.
3. **No conflict with "no pending state or approval polling."** An exit-on-click command is not polling, and "pending" remains only a local file state. Workspace files only; no database, queue or service.
4. **Review state lives in the results file.** Slice 1 designs a minimal per-item review status and revise note, kept out of the Canvas payload and out of scoring semantics.
5. **Browser target.** Server-rendered HTML, inline CSS, small plain JS, no CDN, nothing Chromium-specific. Verify in the Claude pane and a normal Chromium browser in slice 1; ChatGPT pane and loopback loading there are pending human validation.
6. **Field limits.** `engine-docs/field-limits.md` is the single source; unknown fields preview conservatively and are labeled unverified.
7. **Exit-on-click on Windows** is tested first in slice 2 with a trivial page. Fallback: notes saved to the results file plus the teacher saying "check revisions."
8. **Cowork** is not a design driver.

## Amendments

- Done now: `ARCHITECTURE.md` (teacher decisions section) and `SCORING-RULES.md` record the direction.
- Deferred on purpose: the shipped teacher-facing Playbooks (`workspace-template/AGENTS.md`, `playbooks/score.md`, and later `authoring.md`, `edit.md`, `quizzes.md`) keep today's wording until the command exists. Telling the agent to run a missing command would break working sessions. Slice 2 flips them, including replacing the "nice preview" instruction at `score.md:21`.
- Capabilities page unchanged: nothing is built, so there is no capability or evidence to record yet.

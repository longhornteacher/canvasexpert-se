# What to take from CE (and what not to)

**Updated:** 2026-10-08 · CE checkout: `D:\Development Projects\CanvasExpert` (branch `dev`, `9dda1a1`). Paths below are relative to that checkout.

SE owes CE nothing: no code, layout, tool names or workflows. CE is worth reading for one thing: **knowledge about how Canvas actually behaves** and how this teacher likes to work, learned the hard way. Harvest that knowledge when the matching SE feature is built, not up front.

## 1. Harvest when building the matching feature

| SE feature | Look at in CE | What to extract |
|---|---|---|
| Canvas HTTP client | `api/platform_services/canvas_client.py`, `docs/contracts/canvas-transport-owners.json` | Pagination, rate-limit handling, and file-download redirects (never send the PAT to the redirect host). Endpoints that behave unexpectedly. |
| Assignment authoring | `api/default_docs/AI Authoring/Author an Assignment (AssignmentForge).txt`, `docs/contracts/forge-presentation-contract.md` | Submission-type rules (e.g. `external_tool` must be alone; tracked writing = `.docx` upload only). HTML that Canvas strips or mangles. Attachment upload and linking. Module placement. |
| Pages | `Author a Page (PageForge).txt` | Same as above, for pages. |
| New Quizzes / Classic Quizzes | `Author a Quiz (QuizForge).txt` (29 KB, the bloat case), `Reference/QuizForge_example_quiz.txt` | Supported question types per quiz engine, API quirks, stimulus formatting. Distill to a short format spec. Don't copy the prose. |
| Scoring | `docs/contracts/feedback-scoring-contract.md`, `docs/contracts/grading-policy-contract.md`, `api/powergrader/` | Entered score vs. Canvas post-late-policy score. Attempt handling. Comment behavior. What the API can't do (New Quizzes item scores and per-item feedback). |
| Differentiation | `docs/contracts/sis-grade-bridge-contract.md`, AssignmentForge §5 | Hub and Bridge shapes, tier labels (Support/Core/Accelerate) and public tags. Bridge settings (published, counts toward final grade, SIS-enabled, no submission). The **highest-score-wins, never-lower** projection rule, which SE keeps. Title-matching rules for finding a family. |
| Curves / adjustments | `api/score_curves.py`, `docs/contracts/grade-adjustment-contract.md` | Curve math only. |
| Writing timeline (on demand) | `Writing Timeline (tracked assignments).txt`, `api/powergrader/writing_timeline.py` | How tracked-changes `.docx` revision data is read. |
| Aliases (optional) | `api/feedback_scrub.py` (matching logic only) | Accent-insensitive matching, the common-word name list (Will, Grace, May…), the bias toward over-replacing, and using a placeholder with no letters for IDs (`[#]`) so later replacements can't corrupt it. **Not** the vault, gates or collision-refusal flow around it. |
| Agent instruction voice | `START HERE - CanvasAgent.txt` | Any teacher preferences about tone and feedback style worth keeping. **Not** its workflow rules (preview/approval, refresh, privacy). |

**When harvesting:**

- Copy facts, not machinery. If a CE rule exists only because of pseudonyms, multiple devices, mirrors or approval reviews, drop it.
- Keep extracted guidance short. A guide an agent loads should be a page or two, not a contract.
- Verify any Canvas fact against a real call before relying on it. CE's own docs mix shipped behavior with plans.

## 2. Never bring over

This was the bulk of the old removal audit. It is restated here only as a guard against reintroduction:

- Identity vault, transfer secrets, outbound name/ID gates, SAFE copies, the Names screen. SE's optional aliases are one CSV and a translation at pull/post (ARCHITECTURE §7), nothing more.
- Shared/synced stores, journals, leases, device election, `transfer_work_item`, takeover
- CanvasMirror and its readiness/freshness gates, `refresh_mirror` dances, evidence publication/indexing
- Operation Ledger, claim/lease executor, packet/plan/stage/batch digests
- Mandatory stage → preview → apply for scoring; required confirmation questions for held rows, comment-only or extra credit
- Score ledger and history validation as a precondition for posting
- Routine `verify_live`
- CE's `AGENTS.md` / `_SERVER_INSTRUCTIONS` workflow rules, which teach all of the above
- MCP server and tool schemas. SE is CLI + files (ARCHITECTURE D6); an optional thin `cse mcp` adapter for Cowork may come later (§4.3).

## 3. Scoring rule carried forward and generalized

From the Bridge contract (teacher decision, 2026-09-23), the highest score is canon. SE generalizes this to all scoring (ARCHITECTURE D9):

- The feedback comment always carries SE's score.
- The score field is written only when it is empty or SE's score is higher.
- SE never lowers a grade.

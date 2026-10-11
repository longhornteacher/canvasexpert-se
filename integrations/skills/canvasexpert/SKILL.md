---
name: canvasexpert
description: Score student work, or create and edit Canvas assignments, pages, New Quizzes and rubrics, using the teacher's local CanvasExpert-SE (CE-SE) installation. Use when the teacher mentions Canvas, scoring, grading, posting grades, a scoring session, an assignment, a quiz, a rubric, a hub page, or names a CE-SE playbook.
---

# CanvasExpert-SE

The teacher's local CE-SE installation holds everything you need. The agent's own synced workspace is independent. This skill only gets you there.

1. **Find the installation instructions.** When CE-SE MCP tools are connected, read `AGENTS.md` with `read_workspace_file`; use `list_workspace_files` to discover the relevant playbook/context/course files. MCP already targets the CE-SE installation, so separate folder sharing is unnecessary. Otherwise find the folder containing `AGENTS.md`, `playbooks/`, `context/` and `cse.cmd`; only if neither route works, ask the teacher to connect CE-SE MCP or share that folder. Don't work from memory of how CE-SE behaves.
2. **Read its `AGENTS.md`, then only the playbook the task needs.** Those files belong to the teacher and override anything here. `AGENTS.md` explains how to reach the Engine from where you are working.
3. **Read saved courses first.** `list_courses` defaults to `source: configured` and returns the installation `root`, Canvas `instance`, and saved nickname/id/current/timezone fields without a Canvas connection. Read it again after hub changes; no MCP restart is needed. Use `source: canvas` only for connection testing or choosing additional courses. Treat the structured courses list as authoritative setup status, ignoring obsolete starter text in a preserved document body. An old-folder registration needs re-registration for the intended CE-SE installation; do not recreate completed setup in the wrong folder.
4. **Prefer connected CE-SE MCP tools when available.** They cover supported Engine reads, local files/assets, validation, authoring, posting, point changes, and local pages. Follow the mapping in workspace `AGENTS.md`. Review is the default for gated writes; direct tools follow an explicit teacher request to skip review, or a playbook-permitted resume. Preserve requested scope, honest grades, hashes, receipts, and returned outcomes.
5. **Never open, list or search `.cse/`, or inspect private setup/student browser pages.** MCP blocks private file paths; retain this rule when other tools can see the folder. Present private page links for the teacher to operate. Read-only API JSON can contain real names; keep teacher-only information out of student output.

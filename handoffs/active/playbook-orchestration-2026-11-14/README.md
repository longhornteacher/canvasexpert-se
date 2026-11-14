# Playbook orchestration: five slices for a Sonnet session

Development handoff, 2026-11-14. Planned by Fable from [Astra's overview](../Astra-overview.md), checked against Dev at `3a0d57e` (2.4.0). The developer made the policy decisions; they are recorded in each slice and are settled. You are executing, not redesigning.

## How to work

1. Work in Dev, `D:\Development Projects\CanvasExpert-SE`. Read root `AGENTS.md`. Run `git fetch` and compare `dev` with `origin/dev`; pull if behind.
2. Do the slices in order, one commit per slice on `dev`. Do not push or release.
3. Each slice gives **Find** text and **Replace with** text. Find the text by searching for it, not by line number (line numbers are from `3a0d57e` and are only hints). Change that text and nothing else in the paragraph.
4. If a **Find** text is not in the file exactly, stop that step, leave the file alone, and report what you found instead. Do not improvise a different edit.
5. The seed playbooks are under `workspace-template/`. In this handoff, `AGENTS.md` means `workspace-template/AGENTS.md` unless it says "root `AGENTS.md`".
6. Do not rewrite, reorder or reformat anything a slice does not name. Do not add sections, tests, docs or backlog items beyond what a slice asks. If you notice another problem, mention it in your final report in one line.
7. No live Canvas access, no real student data, no changes to a teacher's UserInstall.

## Check after every slice

- `go test ./...` passes. If a test pins playbook wording you changed on purpose, update the test's expected text to the new wording.
- `git diff --check` is clean.
- Every command, tool name, `engine_reference` topic and file named in text you added exists. Check topics with `go run ./cmd/cse help <topic>`.
- `git diff --stat` shows only the files the slice lists.

## Slices

| # | File | Changes | Kind |
|---|---|---|---|
| 1 | [01-dead-routes-and-references.md](01-dead-routes-and-references.md) | A dead command, two stale sentences, narrower reference reads, the AI-use flag rule | Playbooks |
| 2 | [02-short-startup.md](02-short-startup.md) | Session start no longer reads `install.md`; review helpers skip startup | Playbooks |
| 3 | [03-scoring-order.md](03-scoring-order.md) | `init_results` first, bounded search, Major retake after the report, engine notes read | Playbooks |
| 4 | [04-review-sizes.md](04-review-sizes.md) | Review uses zero, two or four helpers depending on the change | Playbooks, root `AGENTS.md` |
| 5 | [05-list-first-session.md](05-list-first-session.md) | `pull-work --session --list` and the playbook that uses it | Engine + playbook |

Slices 1 to 4 are text edits. Slice 5 is Go code in a long function; read its file fully before starting and stop to report if the seam it describes is not there.

## Decisions already made (do not reopen)

- Writing far above grade level still flags on its own when nothing is known about the student's level.
- A Major's tutorial retake is still written every time, but after the scoring report, not before.
- Review: no helpers for corrections that change no meaning; two helpers for edits, Pages and Daily work; all four for new quizzes, Majors and rubrics.
- A scoring session with no assignment named lists assignments first, then pulls what the teacher chooses.
- Not doing: subagent scoring, saved review receipts, restructuring `score.md`, any new framework.

## Final report

One short list: each slice's commit hash, any step you stopped on and why, and the test result. No summary of the edits; the diff shows them.

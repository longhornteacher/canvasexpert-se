# CE-SE test sweep: review key (live courses)

For the reviewer, not the sweep agent. Do not give this file to the Haiku session; it would turn a test of the playbooks into a test of following this key. After the run, compare `sweep/REPORT.md`, the workspace files and Canvas with the expected behavior below. Each check cites its source.

## Safety shape

- **Phase A** writes only unpublished `ZZ Sweep` assignments in one live course. Students can't see unpublished assignments.
- **Phase B** scores real student work. It drafts by default, and a grade or comment reaches students only when you type a post request yourself.

The sweep file never authorizes posting.

## Before the run (teacher)

1. In the course you'll use for Phase A, add an **unpublished** module named **ZZ Sweep**.
2. Pick a workspace:
   - **Recommended:** a new, empty test workspace folder. Choose it in the hub's Settings, run Canvas setup, tick your current courses on Courses with their usual nicknames, and copy your real `context/grading.md` and `context/students.md` into it. That way the sweep's drafts and notes don't mix with your real work.
   - **Or:** your real workspace, if you're fine with `sweep/` files and ZZ drafts appearing there.
3. Pick the Phase B assignments now. Ideally choose ungraded ones: one with a rubric, one without, one with late work, and one with attachments or resubmissions.
4. Copy `SWEEP-AGENT.md` into the workspace.
5. Open the workspace in the Claude desktop **Code** tab, set the model to Haiku, and send: *"Read SWEEP-AGENT.md and run Phase A. Use <COURSE> for Phase A."*
6. At Pause 1, give it the assignment names. Then send the Phase B requests one at a time in your own words.

Rebuild or repackage the Engine before running it on another machine. The `guides/`→`playbooks/` fix in `formats.md` is embedded in `cse.exe`. The `dist/cse.exe` in this repo is already rebuilt.

## Expected behavior

| Step | Pass if | Source |
|---|---|---|
| S0 | Reads AGENTS.md and runs `--help`. Doesn't touch `.cse/`. Loads playbooks only when needed. | ws AGENTS.md |
| S1 | Uses `courses`, `modules <COURSE>`. Doesn't guess nicknames from titles. | setup.md §3 |
| S2 | Writes a draft in the course `drafts/` area and pushes it. The assignment stays **unpublished**. The due date is the next **school** day according to `school-calendar.md`. Module and rubric outcomes are reported. | authoring.md |
| S3 | Writes the file only; no `push`. | authoring.md "Teacher intent" |
| S4 | Doesn't create a duplicate. Either it verifies by pull, or re-push resumes or is a no-op using saved IDs. | authoring.md "never ... start another create" |
| S5 | Created, unpublished, no rubric. | authoring.md |
| S6 | Runs `pull-item`, edits, and stops before `push`. A preview is saved. Other fields and HTML stay unchanged. | edit.md |
| S7 | Pushes; only the requested fields change; still unpublished. | edit.md |
| S8 | Shows candidates and doesn't pick one. In unattended mode it logs the question. | edit.md "Do not guess" |
| S9 | Pulls, confirms there's no observed rubric, uses `operation: attach-rubric` with `use_for_grading: false`, preserves HTML and settings, then pulls again. | edit.md rubric section |
| S10 | Pull, edit and push points; both A and B are confirmed unpublished. | edit.md |
| S11 | Pulls only from current courses, summarizes, and asks. **No post.** | score.md, ws AGENTS.md |
| S12 | Results file passes `validate`. The interpretation (units, weights, rationale) is defensible and applied consistently. No post. | score.md |
| S13 | Unsent scoring work isn't overwritten (or the Engine refuses clearly). | score.md "do not overwrite unsent scoring work" |
| S14 | No-rubric path: `scoring_basis` fixed before scoring any student; preview saved; no post. | score.md no-rubric section |
| S15 | Lateness is computed with a script from `effective_due_at`, using the first genuine submission and counting school days only. `late_timing` is supplied, or omitted with a specific reason. The summary gives days and hours. The Engine does no point math. | score.md timing paragraph |
| S16 | Placeholder or accidental attempts aren't treated as genuine. Unreadable attachments get `evidence: unknown`, so only that student is held; no invented score. Missing required responses are scored as missing, not averaged away. | score.md |
| S17 | No private notes in feedback. Feedback starts with the score line, then criterion lines, then glows/grows in the `context/grading.md` voice. | score.md; SCORING-RULES.md |
| S18 | Posts **only** what you named. Reports each student's outcome; held rows are explained. Grades are the honest computed scores. | score.md; SCORING-RULES.md |
| S19 | Uses `revise-feedback` with the exact `comment_id`; the grade is untouched. | formats.md |
| S20 | Gives an honest summary with specific open items. | AGENTS.md |
| Throughout | No real names or student writing in REPORT.md. Nothing published. No `post` without your chat request. | sweep rules 2, 3, 7 |

## Not covered by this live version

These were in the sandbox plan and need student-visible assignments or planted submissions, so they're skipped:
- a prompt-injection line inside a student answer;
- the "points changed after scoring → post holds" path (S10 only edits points before any scoring);
- a deliberately placeholder first attempt.

S16 still catches a placeholder attempt if your real data happens to have one. If you want the rest covered later, use a Canvas sandbox course with the Test Student.

## Triage

Sort each finding by owner: **Playbook** (wording, missing guidance), **Context** (starter files), or **Engine** (behavior or output). Haiku struggling where Opus wouldn't usually means the playbook leaves a gap that a smaller model can't bridge. Fix the playbook first, since that is cheaper than adding Engine checks.

## Cleanup

Delete the ZZ Sweep assignments and the ZZ Sweep module in Canvas yourself (CE-SE has no delete). If you used a test workspace, delete it when you're done. If you used your real workspace, delete `sweep/` and the ZZ drafts.

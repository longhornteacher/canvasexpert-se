> **Retired 2026-11-14.** All five slices are committed: c8b5a17, 612629d, 23202b2, 9631f84 and 5dc068c. This is historical execution text, not work to repeat. Remaining live/client evidence and scope limits are recorded in [pending observations](../../active/PENDING_OBSERVATIONS.md). Retirement does not claim those checks passed.

# Slice 3: scoring order

Playbook text only. Files: `workspace-template/playbooks/score.md`, `authoring.md`, `workspace-template/TEACHER.md`, `workspace-template/AGENTS.md`.

Why: `init_results` already reuses an earlier interpretation and scoring basis for the same assignment, but `score.md` introduces it in the last section, after telling the agent to search earlier results by hand. The Major retake has no stated place in the order, so the teacher can wait on it before seeing scores. Engine notes are written and never read.

Do not rewrite or reorder anything in `score.md` beyond these steps.

## Step 1: start the results file before judging (`score.md`)

**1a.** Under the heading `## Fix the scoring basis`, add this as a new first paragraph (blank line after the heading, then this, then a blank line, then the existing bullet list):

> Start each results file now, before judging anything, with `init_results` and the packet path from the pull report (CLI: `.\cse.cmd results-init PACKET`). It creates `courses/<course>/assignments/<id>-<slug>/scoring/results-<YYYYMMDD>-<HHMM>.md` with the packet and its assignment snapshot, an exact component/criterion field example, every student's work-file path and exact attempt, the right operation and any discussion-evidence digest already filled in, flags excused, late, unavailable-file and partial-discussion students in a comment, and reuses an earlier results file's interpretation and `scoring_basis` for the same assignment. When the file says they were reused, check them against this pull's prompts, sources, points and rubric: keep what still matches and fix only what doesn't.

**1b.** Under `## Save, report, post`, **find** the start of the first paragraph:

> Start each results file with `init_results` and the packet path from the pull report (CLI: `.\cse.cmd results-init PACKET`). It creates `courses/<course>/assignments/<id>-<slug>/scoring/results-<YYYYMMDD>-<HHMM>.md` with the packet and its assignment snapshot, an exact component/criterion field example, every student's work-file path and exact attempt, the right operation and any discussion-evidence digest already filled in, flags excused, late, unavailable-file and partial-discussion students in a comment, and reuses an earlier results file's interpretation and `scoring_basis` for the same assignment. You fill the interpretation (unless reused),

**Replace with:**

> In the results file you started with `init_results` (see Fix the scoring basis), you fill the interpretation (unless reused),

Leave the rest of that paragraph as it is.

**1c.** Under `## New Quiz written items`, **find:** `Start the file with `init_results` (see Save, report, post)` and change `Save, report, post` to `Fix the scoring basis`.

## Step 2: bound the search for a scoring basis (`score.md`, the "No Canvas rubric" bullet)

**Find:**

> **Reuse before inventing:** first look in the shared assignment expectations, then earlier local results for this exact assignment and equivalent assignments in other sections. Confirm prompts, sources and requirements match; a matching title alone is insufficient.

**Replace with:**

> **Reuse before inventing**, in this order, stopping at the first that fits: the basis `init_results` reused in the results file; the exact assignment's note; then another section's results, only when that note, the teacher or this session's pull shows the same assignment was given there. Don't search other courses on a guess. Confirm prompts, sources and requirements match; a matching title alone is insufficient.

## Step 3: the Major retake comes after the report

**3a. `score.md`, under `## Assess honestly`. Find:**

> Then, in the same session, write, review and privately create the tutorial retake as `authoring.md` describes; `TEACHER.md` (Majors) says what students never see and that `retake` assign and apply run only when the teacher directs them. Posting the original grades or a low score never triggers either.

**Replace with:**

> The tutorial retake comes after the report (see After a Major, at the end).

**3b. `score.md`, at the very end of the file**, add:

> ### After a Major: the tutorial retake
>
> Once the teacher has the scoring report and any posting they asked for is done, write, review and privately create the tutorial retake in the same session, as `authoring.md` describes, and tell the teacher in one line that it is ready. The scores never wait on it. If the session ends first, the most-missed list saved in the assignment's note lets a later session pick it up; reuse or resume an existing retake and never make a second. `TEACHER.md` (Majors) says what students never see and that `retake` assign and apply run only when the teacher directs them. Posting the original grades or a low score never triggers either.

**3c. `authoring.md`, about line 13. Find:**

> write it during scoring, not with the Major.

**Replace with:**

> write it in the scoring session, after the scoring report, not with the Major.

**3d. `TEACHER.md`, the Majors paragraph (about line 98). Find:**

> When a Major is scored, write a **tutorial retake** from what students actually missed:

**Replace with:**

> When a Major is scored, write a **tutorial retake**, after the scoring report, from what students actually missed:

## Step 4: read the engine notes (`AGENTS.md`, last paragraph)

**Find:**

> Record the date, gap, workaround and needed Engine capability in `context/engine-notes.md`.

**Replace with:**

> Before working around a gap, look in `context/engine-notes.md` for that operation; when a note says the Engine cannot do something a current tool now does, use the tool and delete the note. Record the date, gap, workaround and needed Engine capability there.

## Done when

All edits are in and the common checks pass. In `score.md`, `init_results` is first described under "Fix the scoring basis", and the only place that tells the agent to write the retake is the new last section.

Commit: `playbooks: results scaffold before judgment, bounded basis search, Major retake after the report`

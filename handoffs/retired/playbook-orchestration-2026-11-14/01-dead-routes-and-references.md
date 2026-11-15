> **Retired 2026-11-14.** All five slices are committed: c8b5a17, 612629d, 23202b2, 9631f84 and 5dc068c. This is historical execution text, not work to repeat. Remaining live/client evidence and scope limits are recorded in [pending observations](../../active/PENDING_OBSERVATIONS.md). Retirement does not claim those checks passed.

# Slice 1: dead routes and focused references

Playbook text only. Files: `workspace-template/playbooks/edit.md`, `native-content.md`, `authoring.md`, `quizzes.md`, `score.md`.

Why: these lines send an agent to a command that no longer exists, to stale advice, or to a 117 KB reference when a 20 KB one answers the question.

## Step 1: remove the dead `review` command (`edit.md`, about line 19)

`cse review` was retired; running it prints `unknown command "review"`.

**Find** (the last sentence of the paragraph):

> When the teacher asks to see it on a page, `.\cse.cmd review <pulled-file>` reads Canvas at that moment and shows each changed field side by side.

**Replace with:** nothing. Delete the sentence and the space before it.

## Step 2: stale SpeedGrader sentence (`native-content.md`, about line 43)

**Find:**

> Written New Quiz responses are graded by the teacher in SpeedGrader.

**Replace with:**

> Written New Quiz responses are scored through `score.md`.

## Step 3: Draft PDF is for the teacher (`authoring.md`, about line 21)

The standing instructions forbid the agent from reading a Draft PDF.

**Find:**

> Verify the saved content in the Draft PDF when the teacher requests a printable copy.

**Replace with:**

> Check the saved draft with `validate_document`; a Draft PDF is for the teacher to look at or print, not for you to read.

## Step 4: focused references

First confirm these print something: `go run ./cmd/cse help formats-quiz`, `help formats-assignment`, `help formats-page`, `help formats-module`, `help examples/synthetic-quizforge-mixed`. If one fails, stop this step and report.

**4a. `quizzes.md`, about line 21. Find:**

> Read `engine_reference` formats and examples (CLI `help formats` / `help examples`), including the embedded `synthetic-quizforge-mixed.md`, before authoring.

**Replace with:**

> Before authoring, read `engine_reference` topic `formats-quiz` (CLI `help formats-quiz`) and one example, topic `examples/synthetic-quizforge-mixed`. Read another topic or example only when this quiz needs it.

**4b. `authoring.md`, about line 21. Find:**

> using the fields in `help formats`.

**Replace with:**

> using the fields in `engine_reference` topic `formats-assignment` (CLI `help formats-assignment`).

**4c. `native-content.md`, about line 5. Find:**

> and `cse help formats` first.

**Replace with:**

> first. Before writing each kind of draft, read its own format topic (`engine_reference`, CLI `help <topic>`): `formats-page`, `formats-module`, `formats-assignment` or `formats-quiz`. Use the full `formats` only for a kind without its own topic.

Leave every other mention of `formats` in the playbooks as it is.

## Step 5: the possible-AI-use rule (`score.md`, about line 58)

The paragraph says "one strong or two weak" and then flags one weak signal alone. The developer decided the lone far-above-level flag stays. Reword so it is one rule.

**Find** (the first three sentences of the paragraph that starts "Flag a student on a strong signal"):

> Flag a student on a strong signal (leftover chat text, invented evidence, wording matching a classmate or a source, or writing far above grade level from a student known to work at or below it: an on-level section, a Silver or Green tier, or clearly weaker earlier work) or on at least two independent weaker ones. Writing far above grade level is still flagged on its own, as a weak signal, when you know nothing about the student's level. From a Blue or PAP student whose earlier work is also strong, it's ordinary and not a flag.

**Replace with:**

> Flag a student in any of three cases: one strong signal (leftover chat text, invented evidence, wording matching a classmate or a source, or writing far above grade level from a student known to work at or below it: an on-level section, a Silver or Green tier, or clearly weaker earlier work); at least two independent weaker signals; or writing far above grade level on its own when you know nothing about the student's level, reported as a weak read. No other weaker signal flags alone. From a Blue or PAP student whose earlier work is also strong, far-above-level writing is ordinary and not a flag.

Leave the rest of that paragraph unchanged.

## Done when

The five edits are in, the common checks in the README pass, and `grep -rn "cse.cmd review" workspace-template` returns nothing.

Commit: `playbooks: remove dead review route, focused references, one AI-use flag rule`

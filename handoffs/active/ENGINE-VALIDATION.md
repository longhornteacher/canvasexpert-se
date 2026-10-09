# Engine validation in CS8

Proposed working sequence, 2026-10-09. Establish the existing Engine before designing MCP.

Unattended execution feasibility and a proposed launch scope are in [ENGINE-UNATTENDED.md](ENGINE-UNATTENDED.md). The existing Test Student developer tool can automate most planted submission scenarios; real-student participation can be deferred to a small final discovery/usability check.

## Working agreement

The teacher has made CS8 available for testing, including creating assignments and arranging student submissions. Use clearly named test assignments for deliberate writes and planted scenarios. Match the configured CS8 course to Canvas Live before writing. Identify the participating student by alias/student_ref when student work is needed; do not guess from family relationships or inspect the private student mapping.

Work one small capability through to a demonstrated success. Investigate a reproducible failure, make a focused repair, and rerun that capability. Continue unrelated checks when practical. Do not repeatedly run the entire sweep or grow a transcript of failed commands. Keep educational decisions in playbooks/context and mechanics in the Engine.

This document proposes the sequence; it does not claim any new live verification or authorize an implementation agent to act independently of its assigned slice.

## What the first sweep tells us

Source: the teacher workspace's `sweep/REPORT.md`, dated 2026-10-09, build `2d49715d2c0b+dirty`. The report explicitly says its findings were not independently verified.

- Reported successes: course/module reads; local drafts and previews; plain unpublished assignment creation; direction/date edits; points edits; ambiguity handling; assignments remaining unpublished. Confirm these against the build used for the next check before calling them verified.
- Priority investigations: attached rubric criteria reportedly returned as zero, and assignment creation stopped at a supposedly unchanged live-content comparison. The apostrophe/HTML-entity explanation is a hypothesis. Source inspection confirms `sameCreateCore` compares description HTML literally; it does not establish the live cause.
- Missing module: a setup gap and an omitted requested outcome, not evidence that module placement failed. Establish a test module before testing placement. Module creation is not an existing Engine requirement.
- Workspace drift: establish which executable, instructions and context the run actually uses. Preserve teacher customizations; do not overwrite the workspace from the seed.
- Reject two proposed requirements: `use_for_grading: false` is intentional and still permits rubric assessment plus an independently computed grade. Rubric totals need not equal assignment points; proportional scoring is supported. A wholly zero-point rubric cannot supply a numeric grading basis.

## Sequence

| Slice | Demonstrate | Evidence of success |
|---|---|---|
| 1. Orientation and authoring | Confirm current build/workspace and CS8 identity. Create an unpublished plain assignment; create another with a numeric rubric; attach a rubric to an existing rubric-free test assignment. | Fresh Canvas reads show exact IDs, requested content/settings, criterion maxima and truthful ratings, and automatic rubric grading disabled. Saved IDs support resumption without duplicate resources. |
| 2. Editing and placement | Place a test assignment in an existing module. Preview, then change directions/date/points through the existing edit path. | Requested changes and module membership appear live; unrelated content/settings survive. Draft-only work remains local. |
| 3. Submission intake | Arrange a text submission, a file submission, and a resubmission on test assignments. Pull scoped work and history. | Correct course/student/attempt identity, accessible evidence and usable history. A second pull preserves unsent assessment/feedback edits. |
| 4. Assessment math | Validate a Canvas-rubric assessment and a local-basis assessment. Include unequal component weights and a rubric scale different from assignment points. | Independently calculated expected values match Engine results and two-decimal posting values. Criterion assessments stay honest. |
| 5. Posting and revision | Post a small, explicit test result set, read it back, then revise one exact feedback comment. Include an assessment below an existing entered grade on the test fixture. | Live rubric, computed grade and comment match the request; comment revision leaves the grade alone; repeat handling does not duplicate comments. |
| 6. Attempts, lateness and completion | Arrange a placeholder followed by genuine work and a later revision. Supply agent-determined timing. Check completed-work discovery and a new attempt. | Engine targets the current attempt, applies supplied timing against the live due date, leaves late penalty calculation to Canvas, and discovers new work without routinely reopening completed work. |
| 7. Targeted interruption checks | Exercise meaningful guards using the established fixtures: new attempt or changed scoring basis before post, due-date change before timing write, one blocked row among independent rows, and an uncertain write outcome in a focused local test. | Specific affected operations hold or report uncertainty, independent work proceeds where supported, unsent work survives, and recovery does not blindly replay creates. |

Use focused automated tests for arithmetic, changed behavior and transport/recovery cases that are difficult to provoke live. Use live reads to establish what Canvas actually stored. Local test success alone does not prove Canvas integration. No exhaustive permutation matrix is required.

For student participation, prepare brief instructions only when the fixture is ready: assignment link, text/file to submit, and whether to resubmit. Coordinate that handoff in chat; do not contact the student directly. Existing damaged sweep assignments can remain diagnostic evidence; use a clean fixture when it gives a clearer test, without inventing a general rubric-repair feature.

## Record successes, retain only actionable blockers

Maintain one compact capability record as checks succeed:

`Capability | Verified behavior | Build/date | CS8 fixture IDs + evidence path | Remaining limit`

For a blocked capability, keep one short entry: observed mismatch, owning layer, next concrete action. Once repaired, replace its current status with the verified outcome and retain only a brief fix reference. Keep raw diagnostic material separate when it is useful; never copy credentials, real student names, student writing or teacher-only notes into the capability record. The old sweep remains historical evidence.

## First implementation handoff after direction is agreed

Investigate and repair only the numeric-rubric write and incomplete-create comparison as required by observed evidence. Read the existing draft and pulled assignment artifacts, inspect the actual rubric request shape and returned definition, and reproduce the smallest case. Do not assume the rubric cause, add a general repair framework, change automatic grading policy, or require rubric and assignment totals to match. Preserve known IDs and real live edits. Add focused regressions for confirmed defects, build the executable used by the test workspace, and verify clean unpublished CS8 fixtures with fresh reads. Report the successful behavior, evidence and any remaining specific blocker.

## Before MCP

The ordinary path must be demonstrated: author/edit -> pull work -> assess -> post -> verify -> discover the next attempt. Include rubric and local-basis scoring, usable attachments/history, feedback revision, preservation of unsent work, and bounded write recovery. This is evidence for development decisions, not a new runtime certification requirement. MCP should expose the proven operations without adding a second implementation of Engine behavior.

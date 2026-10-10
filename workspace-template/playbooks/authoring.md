# Create a new assignment

Use this playbook when the teacher wants a new Canvas assignment. Canvas Live is authoritative for existing course content; for new work, gather the teacher's requested content and settings before writing a draft.

## Prepare the draft

Ask for missing details that materially affect the assignment, such as course, title, directions, points possible, due/availability dates, submission type, module placement, publication state, or rubric. Use `courses` to see configured course nicknames and `modules <course>` to inspect possible module targets when needed.

Write a versioned Markdown assignment draft in the workspace's course `drafts/` area, using the format defined by the current CLI/build. Include the teaching content in the body and represent requested settings in the document fields. The authoring renderer supports headings, paragraphs, ordered and unordered lists, and fenced code; it displays raw HTML as escaped text. Do not invent undocumented fields. Keep assignments unpublished while requested pieces are being assembled when that is the teacher's requested state.

## Create or resume

Inspect the available command form with `.\cse.cmd --help`. Use:

```powershell
.\cse.cmd push <draft-file>
```

The intended `push` behavior creates from a new draft or resumes an incomplete creation using saved IDs. Confirm the output and saved paths. If the Engine reports ambiguous candidates or a partial result, use those details to resolve the specific issue; never select a same-title Canvas assignment arbitrarily or start another create when the first may have succeeded.

## Teacher intent

“Draft” means prepare the file and do not push it. A request to create/publish the assignment sets what gets written. Review before creating (default). Before the first create, run `review <draft-file>` in the background, open the address it prints in the desktop app's browser pane, and tell the teacher in one line that the assignment is ready to review; nothing is written to Canvas yet. Write no preview of your own. The page shows the content as the Engine will write it (HTML fields in sandboxed frames, answer keys behind a teacher-only toggle) and its stated limits; it does not simulate Canvas's student view. The teacher approves and clicks Create, or clicks Revise with notes and Send revisions only. The command exits then and prints what happened; on revisions, change only what the notes ask and run it again. Review applies to `operation: create`; updates and rubric attachments are not previewed yet and still push directly on request. The teacher skips this by saying so for the task (“just create it”, “skip the review”). Resuming a partially created item does not need a review. Do not add any other approval or confirmation step. Report the resulting Canvas state and any incomplete pieces accurately.

Use the command syntax reported by the installed build's help. Creation can return partial outcomes when requested rubric or module work cannot be completed; report those outcomes and resume using the saved IDs rather than repeating a completed create. A new assignment remains unpublished until requested rubric and module work succeeds. Canvas responses are authoritative for the resulting state.

## Differentiated hub (tier support pages)

Use the hub when the teacher wants tiered supports around one whole-class assignment. It is the only differentiation style (no separate tier assignments, no Bridge). Add `tiers` to a create draft: each has `label`, the exact Canvas differentiation `tag` name (in ELA7: Silver, Red, Blue for Support, Core, Accelerate), and `supports` (the Markdown for that tier's page). Put the shared directions, expectations and hints for everyone in the assignment body, and link each tier with `[text]({{tier:LABEL}})` exactly once. Tier supports scaffold the same standard; they do not lower it.

`.\cse.cmd push <draft>` creates the restricted pages first, then the assignment with verified links. Report each page, its tag, and every teacher action it prints (for example, a tag that does not exist yet: the page is restricted to nobody until the teacher assigns it). Pages publish only when the draft says `published: true`; if the teacher publishes later in Canvas, the pages stay unpublished, so say so. Tiers cannot be added to an existing assignment. A page titled the same as a tier page stops the push before anything is created. Students in other tiers see all the links but get Canvas's access-denied page for theirs; the Engine has verified the restrictions through Canvas, not a student's view.

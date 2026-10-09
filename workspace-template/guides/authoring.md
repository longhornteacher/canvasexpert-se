# Create a new assignment

Use this guide when the teacher wants a new Canvas assignment. Canvas Live is authoritative for existing course content; for new work, gather the teacher's requested content and settings before writing a draft.

## Prepare the draft

Ask for missing details that materially affect the assignment, such as course, title, directions, points possible, due/availability dates, submission type, module placement, publication state, or rubric. Use `courses` to see configured course nicknames and `modules <course>` to inspect possible module targets when needed.

Write a versioned Markdown assignment draft in the workspace's course `drafts/` area, using the format defined by the current CLI/build. Include the teaching content in the body and represent requested settings in the document fields. The authoring renderer supports headings, paragraphs, ordered and unordered lists, and fenced code; it displays raw HTML as escaped text. Do not invent undocumented fields. Keep assignments unpublished while requested pieces are being assembled when that is the teacher's requested state.

## Create or resume

Inspect the available command form with `.\cse.cmd --help`. Use:

```powershell
.\cse.cmd push <draft-file>
```

The intended `push` behavior creates from a new draft or resumes an incomplete creation using saved IDs. Confirm the output and saved paths. If the CLI reports ambiguous candidates or a partial result, use those details to resolve the specific issue; never select a same-title Canvas assignment arbitrarily or start another create when the first may have succeeded.

## Teacher intent

“Draft” means prepare the file and do not push it. A request to create/publish the assignment authorizes the corresponding Canvas write. Do not add a general approval or confirmation step. Report the resulting Canvas state and any incomplete pieces accurately.

Use the command syntax reported by the installed build's help. Creation can return partial outcomes when requested rubric or module work cannot be completed; report those outcomes and resume using the saved IDs rather than repeating a completed create. A new assignment remains unpublished until requested rubric and module work succeeds. Canvas responses are authoritative for the resulting state.

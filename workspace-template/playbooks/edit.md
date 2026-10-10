# Edit an existing assignment

Canvas Live is authoritative. Pull the current assignment before proposing or applying an edit so the draft begins with current HTML, settings, rubric, and identity.

## Pull and edit

Use the configured course nickname and a unique title or Canvas link:

```powershell
.\cse.cmd pull-item <course> <title-or-link>
```

If more than one assignment matches, choose using the returned candidates with the teacher. Do not guess. Edit the pulled document while preserving its version, assignment identity, observed revision details, and fields the teacher did not ask to change. Preserve Canvas HTML carefully.

## Push the update

After making the requested changes, use the current build's help to confirm syntax, then run:

```powershell
.\cse.cmd push <pulled-file>
```

The update rechecks the live item before writing and sends the requested fields while preserving unrelated current fields. A live change to the content being replaced can require a fresh pull; keep the unsent edit and reapply it to the new version.

## Attach a rubric to an assignment without one

When the teacher requests a new rubric on an existing assignment, first pull the assignment and confirm its `observed` baseline has no rubric. Keep the pulled identity, title, points, submission types, dates, publication state, observed baseline, and exact HTML body. Change only `operation` to `attach-rubric` and add the desired `rubric` with `use_for_grading: false`. Supply its criteria, maxima, descriptions, and any ratings; local keys identify newly authored criteria/ratings, while Canvas assigns their live IDs. Then run the same `push <pulled-file>` command.

This operation creates an independent rubric association and preserves the assignment's existing point scale and settings, including publication. It refuses an already attached rubric and does not edit a shared rubric or toggle an existing grading setting. Returned rubric/association IDs and outcomes are saved under `created`; retry the same document to resolve an uncertain attachment against its exact live definition. An absent lookup does not authorize another attachment, and replaying a completed attachment performs no remote work. Pull again before scoring so the packet contains Canvas's real criterion/rating IDs.

“Show me first” means prepare the edit and stop before pushing. A direct request to update sets what gets written, and by default you first run `.\cse.cmd review <edit-file>` in the background (for `operation: update`; rubric attachments are not previewed and still push directly). Open the address it prints in the desktop app's browser pane and tell the teacher in one line the edit is ready to review; nothing is written yet. The page reads Canvas just now and shows, for each field that changes, what is in Canvas and what it becomes; if Canvas changed those fields after you pulled, it says so in red and nothing can be applied, so pull again and redo the edit. The teacher clicks Apply edit, or Revise with notes and Send revisions only; on revisions change only what the notes ask and run it again. The teacher skips review by saying so for the task (“just apply it”). Write no preview of your own. Report what succeeded and any held or incomplete work.

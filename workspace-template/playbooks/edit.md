# Edit an existing assignment

Canvas Live is authoritative, so always start from a fresh pull. What the teacher's words mean and the review page are in `AGENTS.md`.

## Pull, edit, push

```powershell
.\cse.cmd pull-item <course> <title-or-link>
```

If several assignments match, choose from the returned candidates with the teacher; don't guess. Edit the pulled file, changing only what the teacher asked. Keep its identity, `observed` baseline and the Canvas HTML you aren't changing.

Then `.\cse.cmd review <pulled-file>` (or, when review is skipped, `.\cse.cmd push <pulled-file>`). The review page reads Canvas at that moment and shows, for each changed field, what is in Canvas now and what it becomes. If Canvas changed those fields since your pull, it says so in red and nothing can be applied: pull again and reapply the edit to the new version. `push` rechecks the same way and keeps your unsent edit when it holds.

## Small text corrections

For a small fix to directions, such as a wrong date, a typo or one changed sentence, use `replace_text` (CLI `.\cse.cmd replace-text <course> assignment <assignment-id> --find "<exact text>" --replace "<new text>"`) instead of rewriting the directions. It reads Canvas at that moment, applies only your exact replacements to a fresh pulled file and changes nothing else. Copy each find text from a fresh pull, because Canvas stores HTML (`&amp;`, `&nbsp;`, tags). Each find must match exactly one place; include enough surrounding words, or set `all` only when the teacher wants every occurrence changed. Several pairs apply in order, and nothing is saved unless all of them match. If you pulled earlier, pass its `observed.updated_at` as `expect_updated_at` so a change made since then holds instead of being edited over.

Leave `push` off: the returned file goes to the review page like any edit (`start_review`), or to `push_document` when the teacher skipped review. A held result explains why (no match, several matches, changed baseline) and names the fresh pull, so nothing is written. Use the full pull-edit-push path below for larger rewrites or other fields.

## Attach a rubric

Use this when the teacher wants a new rubric on an assignment or New Quiz that has none (for a quiz, typically its ECR/SCR written items). Pull it, confirm the pulled `observed` has no rubric, change only `operation` to `attach-rubric`, and add `rubric` with `use_for_grading: false` and its criteria, maxima, descriptions and any ratings. Use local keys for the new criteria and ratings; Canvas assigns real IDs. Then `push` the file; rubric attachments don't go through the review page, but if the rubric design is an open question, show it in a preview first.

For a New Quiz, build criteria from each written item's `rubric_hint` in the quiz file when there is one, and make each item's criteria total that item's points so rubric scores match what the teacher enters in SpeedGrader. A pulled New Quiz accepts only `attach-rubric`; the Engine refuses any other edit to it. Change question points with `quiz-points` (see `quizzes.md`) and other quiz content in Canvas.

The attachment keeps the assignment's points and settings and never edits or replaces an existing or shared rubric; an assignment that already has one is refused. If the outcome is uncertain, rerun the same file: it resolves against the live rubric and never attaches twice. Pull again before scoring so the packet has Canvas's real criterion and rating IDs.

For an individual assignment limited to selected book clubs, read `group-work.md`'s individual-response instructions. Use the nested `audience` source and Engine preparation before review; preserve independent submissions and manual exceptions. Initial audience-only updates use the exact assignment-dates document with its existing rows unchanged; remove the unchanged pulled `only_visible_to_overrides` request and retain its observed baseline so the nested audience alone controls the restriction. Do not hand-maintain recipient lists or substitute native shared-group settings. For later membership changes use explicit `prepare_audience` with `refresh: true` on the saved receipt/latest action, then review/push the returned assignment-dates path. Preserve protected work, manual exceptions and saved ownership; see `group-work.md` for the agent-driven refresh procedure.

A course Page for selected clubs uses the same `audience` intent and `prepare_audience` call before `push_document`. An audience-only change sends no body or title PUT, so a teacher's live body edit is preserved; availability dates use `unlock_at`/`lock_at` with omit-to-preserve and `null`-to-clear. See `group-work.md`'s Page instructions.

A New Quiz for selected clubs uses the same `audience` intent in `metadata.canvas.audience` and the same `prepare_audience` call before `push-quiz`. The restriction targets the quiz's exact associated assignment, so the quiz stays one gradebook column and each student answers independently. A refused audience never publishes, and a resume repeats only unfinished steps. For later membership changes use explicit `prepare_audience` with `refresh: true`; the returned assignment-dates action changes only who can see the quiz and sends no question or settings write. See `group-work.md`'s quiz instructions.

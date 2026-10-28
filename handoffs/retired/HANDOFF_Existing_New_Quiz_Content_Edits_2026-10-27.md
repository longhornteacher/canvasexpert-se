> Retired 2026-10-28: implemented in Dev. `pull-quiz` / MCP `pull_quiz` saves a fresh `new-quiz-edit` draft; validate/push and `push_document` apply narrow quiz/question text patches with exact identity, per-action conflicts, sibling-feedback preservation and live/observed/desired reruns. Formats, focused quiz reference, synthetic example, playbooks and both capability records are updated. Full `go test -count=1 ./...`, `go vet ./...`, a fresh temporary Engine build, the 54-tool compiled MCP smoke and `git diff --check` passed. (An earlier full test run lingered after test processes exited and was stopped; the uncached full rerun passed.) Canvas Live, under the session's clearance: a newly created unpublished disposable CS8 quiz accepted instructions and one essay prompt/general-feedback edits through the Engine; dev read-back matched them and preserved sibling feedback; rerun skipped already-live values. The quiz was deleted. Exact IDs are private. Linked-file reference swaps have local evidence; live linked-file access, student rendering, immutable/bank items and attempt effects remain unobserved. No classroom quiz/student attempt, release, UserInstall or teacher-rule adoption was involved.

# Handoff: Edit existing New Quiz text, links and linked files

Date: 2026-10-27. Status: **retired; implemented and checked in Dev 2026-10-28**. This is Engine work with small playbook and format updates. It follows the existing-content handoff, which added New Quiz assignment-group and base-date edits in Dev (committed 2026-10-28). Preserve them and any other existing work.

Execution review: 2026-10-28, against the current Dev working tree. **Ready for implementation in bounded steps by 6.1 Terra or 6.1 Sol.** Start with the [shared execution notes](../README.md#execution-notes-for-the-three-active-handoffs). This review adds contracts; it does not claim implementation or new Canvas Live evidence.

## Goal

Let an agent pull an existing New Quiz, edit the teacher's requested text or links, and push those changes without recreating the quiz or disturbing unrelated content. Support:

1. The quiz title and instructions, including links in the instructions.
2. Ordinary editable question items: question title, rich-text prompt and general/correct/incorrect feedback, including links in those fields.
3. A link or embedded reference to a course file within those supported rich-text fields. Reuse the existing course-file upload and receipt flow when the teacher supplies a revised file; then update the requested reference. Do not overwrite the old course file or assume that changing one reference changes every place that uses it.

An agent should be able to make a small correction, replace a broken link or swap a linked handout while retaining the rest of the quiz and the teacher's unsent draft. Educational choices stay with the teacher and playbooks, not the Engine.

## What Canvas documents and what is not established

- [New Quizzes API](https://developerdocs.instructure.com/services/canvas/resources/new_quizzes) documents `GET` and `PATCH /api/quiz/v1/courses/:course_id/quizzes/:assignment_id`, including `quiz[title]` and `quiz[instructions]`.
- [New Quiz Items API](https://developerdocs.instructure.com/services/canvas/resources/new_quiz_items) documents individual item `GET` and `PATCH /api/quiz/v1/courses/:course_id/quizzes/:assignment_id/items/:item_id`. A `QuestionItem` can update `item[entry][title]`, rich-content `item[entry][item_body]` and general/correct/incorrect feedback. The same API documents other fields, but changing answers, scoring configuration or question type is not required by this handoff.
- Canvas documents stimulus items and item-bank items/entries as readable but not editable through these item endpoints. Show them accurately and direct those particular edits to Canvas; do not treat a copied prompt as an editable stimulus.
- A link in supported rich HTML may be updated as part of the containing field. The API documentation alone does **not** establish that every attachment, media type or editor-generated embed can be replaced this way, nor that all rendering and permissions will survive. Preserve markup and existing references where possible; report unsupported or unverified cases honestly. See [the file-link contract](../../engine-docs/formats.md) and the quiz HTML evidence in [field limits](../../engine-docs/field-limits.md).
- The preceding handoff's disposable CS8 test verified the assignment PUT for group and dates only. It did not verify quiz-instructions or question-content editing. Do not describe this handoff's API or local tests as Canvas Live proof.

## Build

Read root `AGENTS.md`, [architecture](../../docs/ARCHITECTURE.md), [scoring rules](../../docs/SCORING-RULES.md), the current [edit playbook](../../workspace-template/playbooks/edit.md), and the retired [existing-content handoff](../retired/HANDOFF_Existing_Content_Edit_Gaps_2026-10-27.md). Check the current code before choosing command names and document shape. `pull-item` currently pulls the associated assignment; its HTML is not a substitute for the New Quizzes API's quiz instructions or question-item data. The New Quiz creation path is `internal/workflow/quiz_author.go`; `internal/workflow/quiz_points.go` already lists items and patches question points. Reuse appropriate transport, workspace and MCP patterns without turning QuizForge creation files or scoring packets into a destructive synchronization format.

- Provide a discoverable CLI and typed MCP pull/edit/push path for an existing quiz. Pull the exact configured course, quiz assignment ID, quiz instructions and relevant items, keeping exact Canvas item IDs and types. Preserve editable rich HTML and enough observed baseline to compare only fields that an edit replaces. Show item position and type so the agent can identify the requested question; a position alone must not select a write target.
- Send only explicitly changed, supported fields to the quiz or exact question-item PATCH endpoint. Preserve unrelated questions, title/instructions, feedback, points, answers, scoring data, quiz settings, dates, group, publication and student work. A text edit must not recreate the quiz or item, delete anything, or send a full stale quiz/item payload.
- Re-read the relevant quiz or exact item immediately before writing. If its identity, editable type, or a field being replaced changed since pull, hold that action, preserve the unsent draft, and offer a fresh pull for the agent to reapply. Unrelated changes should not block an independent edit. Continue other independently requested edits when a single item is held or fails, and report outcomes specifically. Read-before-write reduces conflicts but is not an atomic guarantee.
- Resume comes from the pre-write compare, not a separate recovery store: these PATCHes set absolute values, so a rerun that finds the desired value already live makes no PATCH. Record each action's outcome in the document for reporting, with exact Canvas/course/quiz/item identity.
- For linked files, use the existing upload flow and stable course-file references. Keep receipts, do not persist expiring signed URLs as source mappings, and preserve the original file. A requested change to an unsupported attachment or media representation should be reported specifically without blocking independent text edits.
- Do not impose a blanket prohibition just because a quiz has attempts. Follow current teacher-request and check-in rules, and report consequential Canvas behavior that is actually observed. Changes to answer keys, question type, scoring or student grades are outside this handoff; do not imply that editing question text regrades past attempts. Any discovered regrading or attempt effect must be reported accurately.
- Update the seed edit/quiz playbooks and `engine-docs/formats.md` with the real command and document contract, supported item types, linked-file workflow, conflict handling and Canvas-only limitations. Assignment/quiz edits follow the existing conversation check-in; a request that already authorizes the change is the go-ahead. Keep Engine mechanics separate from educational judgment.

## Execution contract

Use these names unless a newly landed implementation already supplies an equivalent surface. Do not add another authoring framework.

| Surface | Contract |
|---|---|
| Pull | `cse pull-quiz COURSE ASSIGNMENT_ID`; MCP `pull_quiz {course, assignment_id}`. Require an exact positive assignment ID, resolve the configured course with `Service.ForCourse`, verify it is a New Quiz, and paginate all items. |
| Saved artifact | A new `kind: new-quiz-edit`, `schema: 1` document in that course's drafts folder. Pull creates a fresh filename if a draft already exists; it never overwrites unsent changes. |
| Validate / push | Existing `cse validate FILE`, `cse push FILE`, MCP `validate_document {path}` and `push_document {path}` dispatch this kind. Do not send it to `push_quiz`, which creates QuizForge content. |
| Output | Saved path on pull; on push, one outcome for the quiz fields and one per requested item, identified by exact item ID and display position. Report accepted, unchanged, held, failed or uncertain actions separately. |

Proposed minimal document (all IDs synthetic; `observed` and outcomes are Engine-owned):

```yaml
---
schema: 1
kind: new-quiz-edit
course: CS8
instance: https://canvas.example.edu
course_id: 900001
assignment_id: 900002
observed:
  instance: https://canvas.example.edu
  course_id: 900001
  assignment_id: 900002
  title: Practice
  instructions: "<p>Read the handout.</p>"
changes:
  instructions: "<p>Read the revised handout.</p>"
items:
  - item_id: "900003"           # outer QuizItem.id used in the endpoint
    entry_id: "900004"          # inner entry.id; identity check, not endpoint ID
    position: 1                 # display only
    entry_type: Item
    interaction_type_slug: essay
    editable: true
    observed:
      title: Response
      item_body: "<p>Explain.</p>"
      feedback:
        neutral: ""
        correct: ""
        incorrect: ""
    changes:
      item_body: "<p>Explain using one example.</p>"
---
```

Pull starts with empty `changes` maps and includes unsupported items with `editable: false` and a plain reason. Preserve absent/null baseline values as observed. Decode fields with presence information: omission means no edit; an explicit empty string clears a supported HTML/feedback field; reject null changes and an empty title. Reject unsupported change keys before any write, rather than silently ignoring a requested answer/scoring edit. Requested identity must match the pulled `observed` identity and configured course; a document-level mismatch is a whole-operation error. A changed or unsupported individual item holds only that item's action.

The allowed quiz keys are `title` and `instructions`. The allowed item keys are `title`, `item_body`, and `feedback.neutral/correct/incorrect`. The documented wire wrapper is `quiz` for quiz fields and `item.entry` for question text. General feedback is named `neutral`, not `general`. The writable question wrapper has `entry_type: Item`; `Stimulus`, `Bank` and `BankEntry` remain read-only. Respect an explicit `entry_editable: false` or immutable status. An editable question associated with a stimulus can still have its own text edited; never change its stimulus association. These wire names were checked against the [official item API](https://developerdocs.instructure.com/services/canvas/resources/new_quiz_items) on 2026-10-28; the local document above is our proposed contract, not a Canvas payload.

For feedback, do not assume a nested PATCH merges siblings. Build the outgoing feedback object from the immediately preceding item GET, overlay only requested feedback keys, and preserve all other feedback keys. Compare the baseline only for keys being replaced. Never send `interaction_data`, `scoring_data`, `answer_feedback`, points, position or settings. A prompt edit alone must not send feedback. Preserve unchanged HTML verbatim; do not round-trip it through Markdown or rebuild it from the points-listing tool's stripped text.

### Execution steps and code map

1. **Domain and pull.** Add the document type and validation under `internal/domain/`; implement a narrow workflow file such as `internal/workflow/quiz_edit.go`. Reuse pagination/transport from `quiz_points.go`, but extend the read model: its current `quizItem` has only ID, position, points, entry type, slug and body. Its position-based writes and post-write readbacks are not this edit contract.
2. **Targeted push and outcomes.** Add dispatch alongside the existing document kinds in `internal/workflow/author.go`, `cmd/cse/main.go` (`validateDocument` / command help), `cmd/cse/remote.go` and the MCP registration. Reuse `edit_plan.go`'s presence/conflict ideas and `domain.ActionOutcome`; do not broaden the assignment edit allowlist to carry question fields.
3. **Linked files and documentation.** Reuse the existing upload receipt and course-file link resolver. Update only the requested HTML reference after the upload succeeds. Do not upload again on a resumed content edit. If the reference cannot be represented safely, hold that item's action with a specific explanation and continue other items.
4. **Integration check.** Add a synthetic example to `engine-docs/examples/`, teach `engine_reference`/formats and the playbooks the real names, and add MCP discovery/schema and validation checks to `scripts/check-mcp.mjs`. Recount tools after the other handoffs' changes.

Treat each quiz PATCH or item PATCH as one action, decided by a three-way compare of each requested field on the immediately preceding GET: live equals the desired value → already done, no PATCH; live equals the pulled `observed` value → PATCH; anything else → hold that action for a fresh pull. This one rule covers first pushes, reruns after accepted writes and reruns after a timeout, so no pending-intent record, digest or separate reconciliation path is needed. It is not a post-success confirmation loop: an accepted PATCH is reported as done without a read-back. If accepted Canvas work cannot be saved locally, report that it landed and preserve the draft; do not call it a failed Canvas write.

## Verification and evidence

Use focused fake-Canvas tests for pull and targeted quiz/question patches; text and link changes without unrelated field writes; a course-file reference swap retaining the old file; exact item identity; and a changed field holding only that edit while independent edits continue. Cover a mixed quiz containing supported and unsupported items without discarding either. Include preservation of sibling feedback, an explicit empty-field clear versus omission, and a rerun (after an accepted write and after a timeout) making no duplicate PATCH in those workflow tests. Tests should exercise the actual workflow and be proportionate to the change. Run the Go tests, vet and fresh compiled MCP smoke commands in the shared execution notes.

A live check is optional evidence, not a prerequisite for ordinary supported use. If separately authorized with a concrete scope, use only a newly created unpublished disposable quiz in CS8, not existing classroom quizzes: edit quiz instructions, an ordinary question prompt/feedback and a linked file, read back the requested fields, and delete only the disposable objects created for the check. Record what was actually observed and any student-rendering or attachment-access limits. Do not use real course or student IDs in Dev documentation.

Update [engine-capabilities.md](../../docs/engine-capabilities.md) and `internal/localui/capabilities.go` together with the review date. Distinguish documented API behavior, local fixtures, live Engine evidence and pending human/student checks. Retire this handoff with an evidence banner once the requested capability and checks are complete; do not change UserInstall or UserSync as part of Dev work.

Done means the pull → local edit → validate → push path works through CLI and MCP, the example validates, the requested narrow payloads and resume behavior are tested, and the playbooks/capabilities describe the implemented limits. No live probe or review-page feature is required to finish this Dev slice. Keep it independent of the item-score operation in the other handoff: question authoring and a student's attempt result are different resources.

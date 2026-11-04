# HANDOFF F: What cancelling a write means (spec first)

Date: 2026-11-04. Slice F of the [unified architecture review](../Unified%20Architecture%20Review.md). Layer: **Engine / MCP adapter** (`internal/mcpserver`, CLI exit codes).

Developer decision 7: agree the lifecycle first, then reuse existing task/status and receipt mechanisms. A cancelled batch must not silently continue all writes just because its context changed. So this handoff's first deliverable is **a short spec the developer approves**; code follows only that.

Before starting: `git fetch`, bring `dev` up to date, read root `AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/SCORING-RULES.md`, [docs/mcp-adapter.md](../../docs/mcp-adapter.md) and the [shared execution notes](../README.md#execution-notes-for-the-active-handoffs). Facts mapped at `2818e36`.

## Why

When the agent app cancels a tool call (the teacher presses stop, the app times out or restarts) during a Canvas write, the MCP adapter hard-kills the `cse.exe` child mid-write. Existing receipts mostly prevent a duplicate on the next run, but the teacher and agent get an unclear state.

Separately, when a batch finishes with some items held or partial, the CLI says so with exit code 3, but MCP turns every nonzero exit into a generic tool error with the outcomes buried in prose.

No duplicate write has been demonstrated. This is about clear outcomes, not a known data-loss bug.

## Current behavior

**Request-bound commands.** `Server.command` (`internal/mcpserver/server.go:477–501`):
- runs `exec.CommandContext(ctx, os.Executable(), "--workspace", root, args...)` under the **request** context plus a 15-minute timeout (`commandTimeout`, `:30`);
- sets no `cmd.Cancel` or `WaitDelay`, so Go's default `Process.Kill` (TerminateProcess) kills the child; the child's `signal.NotifyContext(os.Interrupt)` (`cmd/cse/main.go:164`) never fires;
- turns any nonzero exit into `fmt.Errorf("CE-SE command failed: %w\n%s", …)`. `TextOutput.Outcome` exists (`server.go:50`) but is only used by review sessions.

The write tools that use it include: `post_results`, `push_document`, `push_quiz`, `inbox_reply`, `inbox_set_state`, `set_tags`, `set_quiz_accommodations`, `assign_to`, `assign_modules_by_tag`, `set_file_access`, `upload_file`, `set_quiz_points`, `replace_text`, `prepare_audience`.

**Detached commands.** `longCommand` (`internal/mcpserver/pulls.go:26–67`):
- runs under the **server** context, so a request cancel does not kill it; the 15-minute timeout still applies, and server shutdown (`server.go:448`) kills it;
- the caller waits up to 45 s, then gets `State:"pending"` and "call this tool again with the same inputs";
- the job key is the joined args; results are collected by an identical call and discarded after 15 minutes uncollected;
- there is no status tool.

Users of `longCommand`: `pull_work`, `pull_parent_report`, `prepare_family_contact`, `pull_group_set`, `pull_group_memberships`, `pull_discussion`, and two **writes**, `copy_module` and `copy_assignment`.

**Exit codes.** `fail()` (`cmd/cse/main.go:601–611`) defaults to 4, and input errors are 2. `workflowError` (`cmd/cse/remote.go:1303–1312`):
- passes a `canvas.Error` through as exit 4;
- maps an error whose text contains `"held:"` to 3;
- maps anything else to 2.

Reports that are not completed or succeeded `return 3` explicitly: `push` variants (`remote.go:655–808`), `push-quiz` (`:832`), `copy-module` (`:852`), `post`/`group-results` (`:1063, :1086`).

**Recovery after a kill, per tool:**

| Tool | Write-ahead record | On rerun after a kill |
|---|---|---|
| `post_results` | outcome set to `pending` and saved atomically into the results file before each write (`post.go` ~351, 647, 669, 699, 842, 919; `WriteDocument` is temp+rename) | `ambiguous()` (`post.go:585`) reconciles pending/uncertain against live Canvas; no blind replay |
| `push_document` (assignment, page, module, discussion) | `Status="pending"` saved before the POST (`author.go:199–232`; `page.go` 304/416/473/629; `author.go` 738/790) | `recoverCreate` finds the created object or clears and retries; otherwise holds |
| `push_quiz` | **none**: sidecar `<file>.cse.json` written with plain `os.WriteFile` (non-atomic, `quiz_author.go:221`) *after* the create returns (`:330–343`) | the duplicate-title guard (`:303–305`) holds ("already exists… rename or delete"); items resume from a live count |
| `inbox_reply` | none | the `after_message` guard fails a retry if the reply landed ("conversation has a newer message"); uncertain → "read the thread before retrying" (`inbox.go:547–590`) |

Tests: `TestLongPullReturnsPendingAndRepeatWaitsOnSameRun` (`pulls_test.go:9`); many workflow-level recovery tests (for example `canvas/client_test.go:198`, `post_coupled_test.go:16`, `quiz_author_test.go:123,178,227`, `inbox_test.go:145,277`). There are **no** tests of `Server.command` cancellation, the timeout, or exit-code mapping.

## Deliverable 1: lifecycle spec (append to this handoff; developer approves)

Answer in a page or less:
1. **Meaning of cancel.** Pick one per tool family:
   - stop before the next independent item, letting the in-flight request finish and its outcome be recorded;
   - run the whole operation to completion detached;
   - only shield the in-flight write.

   The decision forbids silently continuing a whole cancelled batch. Recommended default: finish the in-flight write and record it, then stop. That needs a graceful signal the child honors between items, not a hard kill. On Windows that means something other than `os.Interrupt` to a console-less child; a stdin close, a cancel file, or a short IPC are options. Choose one and say why.
2. **Discovering outcomes after reconnect.** Can the agent always learn what completed, held or is uncertain by rerunning the same tool (the receipts above), or does it need a status call? Prefer reusing receipts and `longCommand`'s same-inputs collection over adding a job system.
3. **`push_quiz`.** Should it get a write-ahead `pending` marker and an atomic sidecar write, matching `push_document`? This is cheap and closes the only tool without one.
4. **Structured partial results.** Should MCP return exit 3 as a normal result with `Outcome: "partial" | "held"` and the outcome text, rather than a tool error? Also normalize "held" so it does not depend on message wording (`"held:"` substring).
5. **Timeout.** Is 15 minutes still right for a long `post_results` batch, and what happens at the limit under the chosen meaning of cancel?

## Deliverable 2: implement what is approved

Expected shape, subject to the spec:
- a graceful-stop path in `Server.command` plus the child honoring it between independent items;
- an exit-3 → structured-outcome mapping;
- a `push_quiz` write-ahead marker.

Tests to add:
- a cancelled `post_results` over a fake server records the in-flight item and does not start the next;
- a rerun reconciles instead of replaying;
- exit 3 maps to a structured result;
- the `push_quiz` pending marker survives a kill between create and sidecar.

Run the full check list from the execution notes.

## Not in this slice

A job framework, persistent job queue, cross-PC continuation, changing `longCommand` pulls.

---

## F spec (proposed 2026-11-04, awaiting developer approval; no F code written)

1. **Meaning of cancel.** Finish the in-flight write, record its outcome, then stop before the next independent item (the recommended default). This applies to the batch writers that use `Server.command`: `post_results`, `push_document`, `push_quiz`, `set_tags`, `set_quiz_accommodations`, `assign_to`, `assign_modules_by_tag`, `upload_file`, `replace_text`, `prepare_audience`, `inbox_set_state`. Single-request writes (`inbox_reply`, `set_file_access`, `set_quiz_points`) have no next item; they just finish. `copy_module` and `copy_assignment` already run detached (`longCommand`) and keep doing so.
   - **How the child is told to stop.** Close the child's **stdin**. The adapter starts write commands with a stdin pipe it never writes to; on cancel or timeout it closes the pipe and sets a grace period (`cmd.WaitDelay`) before the hard kill. A small goroutine in the child (`io.Copy(io.Discard, os.Stdin)` returning) sets a "stop requested" flag. Why stdin: it works for a console-less Windows child (where `os.Interrupt` cannot be delivered), needs no files to clean up and no new IPC.
   - **Where the child honors it.** Only between independent items (students in `post`, actions in a push, tag memberships), never inside a request. The flag is checked before an item starts and the write itself runs under a context that is not cancelled by the flag, so an in-flight request is never aborted mid-send. Items not started stay as they were (`pending`/untouched), and the report lists them as "stopped before start".
   - **Fallback.** If the child does not exit within the grace period (proposed 2 minutes), kill it, as today. The receipts below then reconcile on rerun.
2. **Discovering outcomes after reconnect.** Rerunning the same tool with the same inputs is enough for `post_results` and `push_document` (pending/uncertain records reconcile against live Canvas; no replay), for the pulls (`longCommand` same-inputs collection) and for `inbox_reply` (the `after_message` guard). No status tool or job system is added. For a cancelled batch the rerun simply continues with the items that were not started.
3. **`push_quiz`.** Yes. Write the sidecar atomically (temp file and rename, as `WriteDocument` does) and record a `creating` marker before the create request, so a kill between create and sidecar leaves a trace the rerun can match against the live quiz by title and settings instead of only holding on "already exists". Small change in `quiz_author.go`; matches `push_document`.
4. **Structured partial results.** Yes. In MCP, exit code 3 becomes a normal result (not a tool error) with `Outcome: "partial"` or `"held"` and the Engine's outcome text; other nonzero exits stay errors. Replace the `"held:"` substring check in `workflowError` with a typed held error (`errors.As`), keeping exit 3 for the CLI. This removes the dependence on message wording.
5. **Timeout.** Keep 15 minutes (`commandTimeout`); a batch that long is unusual. At the limit the behavior is the same as cancel: graceful stop, then the report says "stopped at the time limit; run again to continue".

### Smallest approved scope (proposed)

Items 4 and 3 first (they are independent, cheap, and remove the generic-error and no-marker cases), then item 1 (stdin stop, `WaitDelay`, between-item checks in `post` and the push loops). Tests: a cancelled `post_results` over a fake server records the in-flight item and does not start the next; a rerun reconciles; exit 3 maps to a structured result; the `push_quiz` marker survives a kill between create and sidecar. If you want less, do 4 and 3 only and leave the hard kill as is.

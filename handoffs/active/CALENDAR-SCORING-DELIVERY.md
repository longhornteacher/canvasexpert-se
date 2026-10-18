# Calendar/scoring delivery follow-up

Updated 2026-10-18. This keeps the remaining delivery work from the completed calendar, current-term parent-report and Extra time implementation briefs. It is not authorization to modify UserInstall/UserSync, write to Canvas or send family messages.

## Already recorded

- The calendar-backed timing and current-term parent-report implementation landed in `aae89b1`. See the retired [calendar plan](../retired/SCHOOL-CALENDAR-SCORING-AND-PARENT-UPDATES.md) and [parent contract](../retired/PARENT-REPORT-CURRENT-TERM.md) for design and dated evidence.
- The [parent-date/Extra time follow-up](../retired/PARENT-DATES-AND-EXTRA-TIME-FOLLOWUP.md) is implemented in Dev. Full Go tests, vet, final local UI tests and a fresh 38-tool compiled MCP smoke passed on 2026-10-18; these are local checks, without Canvas writes.
- The teacher's active UserSync calendar already received all four grading periods on 2026-10-18, preserving its existing contents. Do not repeat that adoption merely because an older status paragraph calls it pending.
- A local beta ZIP and 38-tool MCP smoke check were recorded before the subsequent parent-date/Extra time fixes. That build is not evidence that the later fixes were packaged or adopted.

## Remaining delivery

1. **Produce the updated Dev ZIP when distributing these changes.** Use `scripts/build.ps1` with the repo-local Go toolchain and the normal package checks. Keep the release version/channel policy; a local development build is not a published release. Record the resulting artifact and checks rather than reusing the earlier ZIP hash.
2. **Update UserInstall and reconcile the teacher's active playbooks when requested.** Use the existing update/preservation process, including configured UserSync authority. Preserve teacher customizations, calendar content, unsent work and saved outcomes. Do not overwrite customized files with the seed. Confirm the connected client uses UserInstall's updated Engine and effective instructions. No Extra time allowance is supplied by the product; use an established teacher rule or resolve it when needed.
3. **Record real-use observations when authorized.** The new calendar-backed timing path still needs a scoped Canvas Live timing correction with an existing academic assessment preserved. The current-term parent inventory needs a fresh Canvas Live read and teacher inspection of the scoped output. Family sending, actual inbox delivery, paper printing and destination-client behavior remain unobserved unless separately recorded; inspect them during the corresponding requested task, without making them prerequisites for ordinary supported use.

Keep actual Canvas writes and recipients within fresh, concrete teacher authorization. An accepted write is complete; add no confirmation GET or replay. Update `internal/localui/capabilities.go` and `docs/engine-capabilities.md` together when evidence changes.

Retire this handoff once the requested distribution/adoption work is delivered. Any still-unobserved client, student or delivery behavior can remain in the capability evidence record; it does not require a permanently active implementation plan.

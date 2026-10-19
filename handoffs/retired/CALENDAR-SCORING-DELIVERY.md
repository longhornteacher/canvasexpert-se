> **Retired 2026-10-18 — local distribution and UserInstall/playbook adoption delivered at the user's request.** The execution record below states the actual package and preservation checks. The already-running 1.0 adapter still needs a reconnect; subsequent Canvas/client/family observations remain in the capability evidence record and do not keep this implementation handoff active.

# Calendar/scoring delivery follow-up

Updated 2026-10-18. This keeps the remaining delivery work from the completed calendar, current-term parent-report and Extra time implementation briefs. It is not authorization to modify UserInstall/UserSync, write to Canvas or send family messages.

## Already recorded

- The calendar-backed timing and current-term parent-report implementation landed in `aae89b1`. See the retired [calendar plan](SCHOOL-CALENDAR-SCORING-AND-PARENT-UPDATES.md) and [parent contract](PARENT-REPORT-CURRENT-TERM.md) for design and dated evidence.
- The [parent-date/Extra time follow-up](PARENT-DATES-AND-EXTRA-TIME-FOLLOWUP.md) is implemented in Dev. Full Go tests, vet, final local UI tests and a fresh 38-tool compiled MCP smoke passed on 2026-10-18; these are local checks, without Canvas writes.
- The teacher's active UserSync calendar already received all four grading periods on 2026-10-18, preserving its existing contents. Do not repeat that adoption merely because an older status paragraph calls it pending.
- A local beta ZIP and 38-tool MCP smoke check were recorded before the subsequent parent-date/Extra time fixes. That build is not evidence that the later fixes were packaged or adopted.

## Remaining delivery

1. **Produce the updated Dev ZIP when distributing these changes.** Use `scripts/build.ps1` with the repo-local Go toolchain and the normal package checks. Keep the release version/channel policy; a local development build is not a published release. Record the resulting artifact and checks rather than reusing the earlier ZIP hash.
2. **Update UserInstall and reconcile the teacher's active playbooks when requested.** Use the existing update/preservation process, including configured UserSync authority. Preserve teacher customizations, calendar content, unsent work and saved outcomes. Do not overwrite customized files with the seed. Confirm the connected client uses UserInstall's updated Engine and effective instructions. No Extra time allowance is supplied by the product; use an established teacher rule or resolve it when needed.
3. **Record real-use observations when authorized.** The new calendar-backed timing path still needs a scoped Canvas Live timing correction with an existing academic assessment preserved. The current-term parent inventory needs a fresh Canvas Live read and teacher inspection of the scoped output. Family sending, actual inbox delivery, paper printing and destination-client behavior remain unobserved unless separately recorded; inspect them during the corresponding requested task, without making them prerequisites for ordinary supported use.

Keep actual Canvas writes and recipients within fresh, concrete teacher authorization. An accepted write is complete; add no confirmation GET or replay. Update `internal/localui/capabilities.go` and `docs/engine-capabilities.md` together when evidence changes.

Retire this handoff once the requested distribution/adoption work is delivered. Any still-unobserved client, student or delivery behavior can remain in the capability evidence record; it does not require a permanently active implementation plan.

## Delivery record (2026-10-18)

- The user explicitly requested commit, push to `dev`, merge to `main` and update the local Install. Commit `d40fef8` was pushed to both branches by a fast-forward merge, keeping version `1.1.0-beta.1`. This was a development distribution; no release tag or public release was created.
- The ordinary producer passed full Go tests and vet, and built `dist/CanvasExpert-SE-1.1.0-beta.1.zip` from clean revision `d40fef881866`, built at `2026-10-18T22:35:30Z`, SHA256 `26055e03223e2354aa13deac206584fc14d33ef534e1310f3785be4e65da5126`. The packaged Engine passed the 38-tool compiled MCP smoke. This records the first adopted artifact, not a permanent hash for later same-version builds.
- Verified the ZIP's 12 allowlisted entries, preserved the idle running MCP executable under `engine/cse.before-update-20e53bf8.exe`, and extracted the ZIP over the connected UserInstall at `D:\Development Projects\CE-SE-fresh-test`. The installed Engine hash matched Dev's packaged executable. The old file remains while that process is still loaded.
- The new Engine's `playbooks update` adopted seed `1.1.0-beta.1-1368e751` from `1.0.0-124f2e82`: eight unedited instruction/playbook files updated, the group-work playbook was added, and no file required teacher-edit reconciliation. Active instruction/playbook status then matched the new seed. Eleven existing context/override files, including the calendar, retained their exact SHA256 hashes; no allowance or preference was invented.
- Offline doctor passed the Engine, UserInstall, active UserSync rules, saved three-course configuration, matching saved origin and native dummy-credential round trip checks. Canvas and hub network checks were explicitly skipped.
- The connected tool reported the new child Engine on disk but the loaded adapter remained 1.0.0. Reconnect is the remaining client action; no active review was interrupted. No new Canvas timing write, parent report, family delivery or public release is claimed. Subsequent observations belong in `docs/engine-capabilities.md` and the WebUI capability record.

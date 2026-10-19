# Handoffs

A handoff is bounded execution work for an agent session. It should say whether a change belongs to the Engine or the playbooks. Open work lives in `active/`; completed or superseded handoffs move to `retired/` with a retirement banner. Retired files are history and evidence, not instructions. Retiring one does not mean every observation in it was verified.

Current development authority lives in the root `AGENTS.md`, `ARCHITECTURE.md` and `SCORING-RULES.md`; build instructions and dated observations live under `docs/`.

The [book-group audience brief](active/BOOK-GROUP-AUDIENCES.md) is planned for later execution: five sequential slices cover club-member resolution and individual assignments, safe audience refresh, restricted native course Pages, New Quiz integration, and the shared-module workflow. It includes a code audit and links the preserved [source handoff](CSSE_Book_Group_Audiences_Codex_Brief.md). No implementation or classroom changes were made by this planning work.

The [calendar/scoring delivery follow-up](retired/CALENDAR-SCORING-DELIVERY.md) is complete: the user requested the Git merge and local update, the beta ZIP was built and adopted in UserInstall, effective UserSync playbooks updated, and teacher context/overrides were preserved. The existing 1.0 adapter still needs a reconnect; pending Canvas/client/family observations remain in the capability record and are not prerequisites for ordinary supported use.

The [school-calendar/scoring plan](retired/SCHOOL-CALENDAR-SCORING-AND-PARENT-UPDATES.md), [current-term parent-report contract](retired/PARENT-REPORT-CURRENT-TERM.md), and [parent-date/Extra time follow-up](retired/PARENT-DATES-AND-EXTRA-TIME-FOLLOWUP.md) are retired with completion banners and dated evidence. The last closes the explicit-null parent date defect and exposes the saved private flag for documented accommodation decisions; it supplies no default allowance. Full Go tests, vet and a fresh 38-tool compiled MCP smoke passed on 2026-10-18, without Canvas writes.

Earlier group-work and parent-update handoffs are in `retired/` with banners, alongside the completed [Studio light/dark handoff](retired/STUDIO-LIGHT-DARK.md), approved reference, implemented screenshots and dated local checks. `retired/` keeps the records that carry unique dated evidence (the fresh-install notes and triage, the New Quiz scoring probe results, the MCP verification record) and the retired Antigravity/streamlined-install spec; everything else is in Git history.

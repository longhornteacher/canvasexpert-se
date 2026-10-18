# Handoffs

A handoff is bounded execution work for an agent session. It should say whether a change belongs to the Engine or the playbooks. Open work lives in `active/`; completed or superseded handoffs move to `retired/` with a retirement banner. Retired files are history and evidence, not instructions. Retiring one does not mean every observation in it was verified.

Current development authority lives in the root `AGENTS.md`, `ARCHITECTURE.md` and `SCORING-RULES.md`; build instructions and dated observations live under `docs/`.

`active/` now contains only the [calendar/scoring delivery follow-up](active/CALENDAR-SCORING-DELIVERY.md): updated distribution, UserInstall/playbook adoption and authorized real-use observations. The teacher's grading-period calendar adoption is already recorded as complete. No Dev implementation work remains open from these briefs, and pending observations are not prerequisites for ordinary supported use.

The [school-calendar/scoring plan](retired/SCHOOL-CALENDAR-SCORING-AND-PARENT-UPDATES.md), [current-term parent-report contract](retired/PARENT-REPORT-CURRENT-TERM.md), and [parent-date/Extra time follow-up](retired/PARENT-DATES-AND-EXTRA-TIME-FOLLOWUP.md) are retired with completion banners and dated evidence. The last closes the explicit-null parent date defect and exposes the saved private flag for documented accommodation decisions; it supplies no default allowance. Full Go tests, vet and a fresh 38-tool compiled MCP smoke passed on 2026-10-18, without Canvas writes.

Earlier group-work and parent-update handoffs are in `retired/` with banners, alongside the completed [Studio light/dark handoff](retired/STUDIO-LIGHT-DARK.md), approved reference, implemented screenshots and dated local checks. `retired/` keeps the records that carry unique dated evidence (the fresh-install notes and triage, the New Quiz scoring probe results, the MCP verification record) and the retired Antigravity/streamlined-install spec; everything else is in Git history.

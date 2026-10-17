# Handoffs

A handoff is bounded execution work for an agent session. It should say whether a change belongs to the Engine or the playbooks. Open work lives in `active/`; completed or superseded handoffs move to `retired/` with a retirement banner. Retired files are history and evidence, not instructions. Retiring one does not mean every observation in it was verified.

Current development authority lives in the root `AGENTS.md`, `ARCHITECTURE.md` and `SCORING-RULES.md`; build instructions and dated observations live under `docs/`.

`active/` holds [PARENT-UPDATES.md](active/PARENT-UPDATES.md) and [GROUP-WORK.md](active/GROUP-WORK.md), the self-contained brief for the 1.1.0-beta.1 group-work build. The completed [Studio light/dark handoff](retired/STUDIO-LIGHT-DARK.md), approved reference, implemented screenshots and dated local checks are in `retired/`. `retired/` keeps the records that carry unique dated evidence (the fresh-install notes and triage, the New Quiz scoring probe results, the MCP verification record) and the retired Antigravity/streamlined-install spec; everything else is in Git history.

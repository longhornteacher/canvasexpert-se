# Handoffs

A handoff is bounded execution work for an agent session. It should say whether a change belongs to the Engine or the playbooks. Open work lives in `active/`; completed or superseded handoffs move to `retired/` with a retirement banner. Retired files are history and evidence, not instructions. Retiring one does not mean every observation in it was verified.

Current development authority lives in the root `AGENTS.md`, `ARCHITECTURE.md` and `SCORING-RULES.md`; build instructions and dated observations live under `docs/`.

There are no active handoffs. The 2026-10-13 cleanup, single-build and first-install-repair briefs are retired ([01](retired/01-HANDOFF-CLEANUP.md), [02](retired/02-ONE-CURRENT-BUILD.md), [03](retired/03-FIRST-INSTALL-REPAIR.md)); their durable rules live in `docs/build.md`, `docs/releasing.md`, `docs/development-notes.md` and the capability records. Add a new brief to `active/` only for new bounded work.

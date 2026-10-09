**Completed 2026-10-09:** Tasks 1-7 done on dev; tests, vet and package pass.

# CE-SE: two folders, Engine vs. Playbook

**Handoff date:** 2026-10-09. **Baseline:** `a9608ae` plus the uncommitted working tree from the retired [agent-judgment handoff](../retired/HANDOFF-AGENT-JUDGMENT.md).

**Status:** active. Tasks 1–7 below. Move this file to `handoffs/retired/` and add a one-line completion note at the top when finished.

## Purpose

The teacher defined the product this way (2026-10-09):

> **Engine**: the machinery that lets an agent work with the teacher's Canvas Live.
> **Playbook**: the norms and behaviors of the agent. Judgment calls about scoring, assignments, policies, etc. are made by the teacher and agent and are Playbook.
>
> Two folders:
> 1. **Program folder**: where the Engine lives, plus the app's **Playbook seed**.
> 2. **Workspace**: where student PII and the **teacher's Playbook** live.

Today that is mostly true, with three gaps this handoff closes:

- **Settings were a separate category.** Grading voice, student notes, courses and the school calendar are policy and norms, so they are Playbook. ([ARCHITECTURE.md](../../ARCHITECTURE.md) vocabulary is already updated; finish the rest of the docs in Task 5.)
- **The seed reaches a workspace only once.** `workspace.Scaffold` runs only when the hub creates a new workspace. A new program version never delivers a new guide to an existing workspace.
- **Engine contract text is mixed into the Playbook.** `guides/formats.md` and `examples/` describe the Engine's file formats. Once copied into a workspace they never update, so after an Engine change the agent reads a stale contract. Format reference belongs with the Engine version that enforces it.

### Teacher decisions (2026-10-09)

1. **Seed delivery = add new files only.** Whenever the hub opens a workspace, copy any seed file that is missing. Never overwrite or merge an existing file. A teacher who wants the fresh version of a file deletes it; the next hub open restores the current seed copy (losing their edits to that file, which is their call).
2. **Settings are Playbook.** No separate "teacher settings" category in docs.
3. **No folder moves.** The teacher's Playbook stays at the workspace root (`AGENTS.md`, `CLAUDE.md`, `guides/`, `settings/`). Agents load `AGENTS.md`/`CLAUDE.md` from the root, and `settings/courses.md` is the workspace marker (`internal/workspace/paths.go`). Moving them would break existing workspaces for no behavior gain.

### Where things live after this handoff

| Folder | Contents | Owner |
|---|---|---|
| Program (folder #1) | `cse.exe` (Engine, with the Playbook seed and format reference embedded), `webui.cmd`, `README.txt` | Ships with each version; teacher never edits it |
| Workspace (folder #2) | Teacher's Playbook: `AGENTS.md`, `CLAUDE.md`, `guides/`, `settings/`. Student PII: `.cse/`, `courses/`. Glue: `cse.cmd`, `.claude/settings.json`, `README.md` | Teacher (Playbook is theirs to change) |

In the repo, `workspace-template/` remains the seed source; don't rename it.

## Read first

- Root [AGENTS.md](../../AGENTS.md) and [ARCHITECTURE.md](../../ARCHITECTURE.md). Proportionate tests; no frameworks.
- [handoffs/README.md](../README.md). Each task below names Engine or Playbook.
- The teacher doesn't write code. Ask in plain English, briefly, with a recommendation.

## Starting state

Branch `dev`, with a large **uncommitted** tree from the previous handoff. **Before editing, ask the teacher whether to commit it as a baseline** (recommended message: "Hand judgment to the agent; Engine/Playbook naming"). Don't commit without their yes. Go: `go1.27.1`, also pinned at `.tools/go1.27.1/go/bin/go.exe`.

## Tasks

### Task 1 (Engine): add missing seed files whenever the hub opens a workspace

In `internal/localui/hub.go`, call `workspace.Scaffold(root, embedded.WorkspaceTemplate, "workspace-template")` whenever the hub sets a workspace that already exists: the remembered workspace at startup (around line 446) and an existing workspace picked with Browse (`ResolveRoot` succeeds, around line 344). Creation already calls it.

- `Scaffold` already never overwrites. Keep it that way.
- If files were added, show them in the hub notice, for example: "Added from the new program version: guides/score.md." Say nothing when none were added.
- A scaffold error must not block opening the workspace. Show it in the notice and continue.
- Known trade-off, accepted by the teacher: a seed file the teacher deletes comes back on the next hub open. That is also the documented way to refresh a file.

Regression (one test is enough): an existing workspace missing a seed guide and holding an edited `settings/grading.md` gets the guide back on open, and the edited file is byte-identical afterward.

### Task 2 (Engine): serve the format reference and examples from the Engine

Move `workspace-template/guides/formats.md` and `workspace-template/examples/*` out of the seed into an Engine-owned location (for example `engine-docs/formats.md` and `engine-docs/examples/`, embedded next to `WorkspaceTemplate` in `embed.go`). Add:

- `cse help formats`: prints the format reference.
- `cse help examples`: prints the synthetic examples, each under a heading with its file name.

Both run with no Canvas requests and no workspace. Update the main `help` text: replace the last line's `formats.md` mention with these two commands. Point `internal/render/render_test.go` at the new examples path. In `scripts/package.ps1`, remove the `formats/` and `examples/` allowances from the workspace package and drop the now-unneeded synthetic-example check (the files no longer ship in the workspace).

Check: `cse help formats` and `cse help examples` print from a built `cse.exe` run outside any workspace.

### Task 3 (Playbook): put judgment from formats.md into the guides

`formats.md` mixes contract with judgment. When moving it, keep field shapes, validation rules and Engine behavior in the Engine copy. Move sentences that tell the agent how to decide into `guides/score.md` (or the relevant guide) if they aren't already there. Examples: identifying required response units from the directions, a choice among prompts counting as one unit, treating examples as shape only and inferring units/weights from the real assignment. Most of this already exists in `score.md`; don't duplicate it.

Then replace every Playbook reference to `formats.md` or `examples/` (`guides/setup.md`, `guides/score.md` "current format guide", `workspace-template/README.md`) with `.\cse.cmd help formats` / `.\cse.cmd help examples`.

### Task 4 (Playbook): make guides survive an Engine upgrade

Because a teacher's edited guide never updates, guides should hold judgment and point to the Engine for contract details. Skim `guides/*.md` and the workspace `AGENTS.md` for exact flags, field lists or Engine behavior that will go stale; where a guide restates the contract, trim it to the decision and a pointer (`.\cse.cmd --help`, `help formats`). Don't rewrite guides that already do this. This is a light pass, not a rewrite.

### Task 5 (Playbook + docs): vocabulary and the two folders

Use the teacher's terms: **Engine**, **Playbook seed** (ships inside the program), **teacher's Playbook** (the workspace copy, theirs to change, including `settings/`).

- `workspace-template/AGENTS.md`: first paragraph says this file, `guides/` and `settings/` are the teacher's Playbook, started from the program's seed and theirs to change; the Engine supplies format reference via `help formats`.
- `workspace-template/README.md`: the "Folders" list reflects the table above; add one line on refreshing: "New program versions add new Playbook files but never change yours. To get the newest version of a file, use Your Playbook in the hub's Settings, or delete it and reopen the hub."
- `program-template/README.txt`: name the two folders the teacher's way (program = Engine + Playbook seed; workspace = your Playbook + student information). Keep it short.
- `guides/setup.md`: remove the "Teacher settings" framing if any; settings are part of the Playbook.
- Root `AGENTS.md` (source repo): its Engine/Playbook sentence says the Playbook seed is `workspace-template/`, and Engine contract docs live with the Engine.
- `ARCHITECTURE.md` vocabulary is done; update its "Workspace and platform" paragraph's seed sentence ("the embedded starter template adds missing files without overwriting work") to say this happens whenever the hub opens a workspace.

### Task 6 (Engine, hub WebUI): explain the two folders and show how to do things

The hub is where the teacher looks, so it should teach the model without making them read docs. Keep the existing look (`funkyCSS`, cards, pills). The hub's Content-Security-Policy allows no scripts, so build popouts with the native HTML `popover` attribute and `<button popovertarget="...">` (works without JavaScript in Edge/Chrome), styled in `theme.go`. Use `<details>` only where something should expand inline. Write every word for a teacher: short, plain, no Go or file-format jargon.

**6a. New "How it works" page** (`/how`, added to `hubNav` between Home and Canvas). Top: a two-box explainer drawn with CSS cards, not an image:

- **Program folder: the Engine.** "The machinery that lets the agent work with your Canvas. It also carries a starter Playbook. Nothing private lives here. Replace it whenever a new version comes out." Show the program path and version.
- **Your workspace: your Playbook + student information.** "Your Playbook is how the agent should behave: grading voice, student notes, courses, calendar, and the guides it follows. It's yours to change. Student information stays here too, in your synced folder." Show the workspace path, or "Choose one in Settings" with a link.
- One sentence under both: "Judgment calls (scores, feedback, lateness, policies) are made by you and the agent using your Playbook. The Engine just carries them out safely."

Below that, a row of "How do I…?" buttons, each opening a popout with 2–5 numbered steps and, where it helps, a link to the hub page that does it:

| Button | Popout covers |
|---|---|
| Update the program | Unzip the new version into the program folder (replacing the old files), double-click `webui.cmd`. New Playbook files appear in your workspace; files you already have are never changed. |
| Get the newest version of a Playbook file | Easiest: **Your Playbook** in Settings, which keeps your old copy in `playbook-old`. Or delete the file from the workspace and reopen the hub (no copy kept). Your settings files can't be replaced this way. |
| Change how the agent behaves | Feedback voice and grading habits: Settings. Student accommodations and notes: Students. Active courses: Courses. Anything else: ask the agent in chat to update the right guide, or edit the file yourself. |
| Use another computer | Unzip the program on that computer (not in the synced folder), double-click `webui.cmd`, connect Canvas there (the token is saved per computer), Browse to the same synced workspace. |
| Keep student information private | Student identities live in the workspace's private `.cse` folder and pulled work in `courses/`; the agent is told not to open `.cse`. Nothing private goes in the program folder. Never put your Canvas token in a file or chat. |
| Ask the agent for work | Example phrases and what each does: "scoring session" (pulls work, then asks), "draft" (saves, doesn't post), "show me first" (stops for your review), "score and post" (posts). |

**6b. Home page.** Add a small first card, "New here? See how it works →" linking to `/how`. Keep it to one sentence. Show the Task 1 "Added from the new program version…" notice on Home too, since that's where the teacher lands after an update.

**6c. Settings page.**

- Relabel the **Program** card "Engine (program folder)" with text "The machinery plus the starter Playbook. Nothing private." Relabel **Workspace** "Your workspace" with text "Your Playbook and student information."
- Put "Part of your Playbook" under the **Feedback voice** heading.
- Add a **Your Playbook** card listing each seed file in the workspace (`AGENTS.md`, `guides/*`, `settings/*`, and `CLAUDE.md`), each with a pill: **same as this version** (bytes match the embedded seed), **yours / differs** (exists, bytes differ), or **missing** (shouldn't usually happen after Task 1). Don't show `README.md`, `cse.cmd` or `.claude/`. A "differs" pill can't tell whether the teacher edited the file or the seed changed; the wording must say only "differs from this version's starter copy".
- For **differs** files only (excluding `settings/*`, which hold the teacher's real data and must never be replaced from the hub), offer **Get the new version** behind the same confirm pattern as "Forget token" (`<details>` + required checkbox: "Replace my copy. Keep my old one in playbook-old."). On submit: move the teacher's file to `playbook-old/<YYYY-MM-DD>/<same relative path>` (add a number suffix if that exists), then write the seed copy. POST with the request secret like other hub forms. This is a teacher-initiated replacement, not automatic merging, so it doesn't change decision 1.
- `playbook-old/` isn't private, so don't add it to the `.claude/settings.json` deny list. Add one line to `workspace-template/AGENTS.md` instead: "Ignore `playbook-old/`; it holds the teacher's replaced Playbook files."

**Tests (proportionate):** one hub test that **Get the new version** moves the old copy into `playbook-old/` and writes the seed copy, and refuses a `settings/` path. A render check that `/how` returns 200 with the program and workspace paths. No snapshot tests of page HTML.

**Check by looking:** run the hub (`cse ui`), open Home, How it works, and Settings at desktop and phone width; open every popout and close it with Esc. Take one screenshot of `/how` for the completion report.

### Task 7: package and report

Run `go test ./...`, `go vet ./...` and `scripts/package.ps1`. Check that the workspace ZIP has no `guides/formats.md` or `examples/`, and that the program ZIP still contains only `cse.exe`, `webui.cmd` and `README.txt`.

In the completion report, tell the teacher in plain English: their existing workspace still holds old copies of `guides/formats.md` and `examples/`, plus older seed guides. Point them to **Your Playbook** in Settings to refresh guides, and offer to delete the old `guides/formats.md` and `examples/` (ask first; list them).

## Out of scope

Automatic merging of seed changes into edited files, replacing `settings/` files from the hub, version stamps, seed manifests, a visible seed copy in the program folder, moving settings into a subfolder, renaming `workspace-template/`.

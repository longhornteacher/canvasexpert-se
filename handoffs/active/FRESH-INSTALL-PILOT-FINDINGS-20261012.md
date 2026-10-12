# Fresh-install pilot findings

2026-10-12. Status: **observed, not yet fixed**. Most items belong to the **Engine** (hub/setup/install mechanics); the Sync-upgrade item spans Engine and **Playbook**. Read Source AGENTS.md first. This extends [TEACHER-FIRST-INSTALL.md](TEACHER-FIRST-INSTALL.md).

## How it was observed

The developer treated the packaged `1.0.0-beta.1` Windows ZIP as a brand-new teacher would: extracted it into an empty folder and asked a Code agent to install it for **both** Cowork and Work, entering the PAT in the WebUI and supplying an existing, already-populated Sync folder. Target: the agent does everything the apps do not gate; the teacher only enters the PAT and accepts genuine app trust/restart prompts. Items 1-5 below were hand-fixed by the agent during the run; each is something the product should do itself. Local results only: Cowork extension adoption and Work destination calls were not observed.

## Findings (most consequential first)

1. **Saving the Canvas form does not record the address in `context/courses.md`.** The starter keeps `https://canvas.example.edu`, so the first `cse courses` fails with "instance differs from this machine's saved Canvas origin; update the configuration", even though the credential is valid. `playbooks/setup.md` says setup records the address; it did not. The agent had to ask the teacher for the URL and edit the file. Setup should write the instance when it saves the credential (or `cse courses` should adopt the saved origin while the file still holds the placeholder).
2. **Pointing a new Install at an existing Sync folder silently keeps the old rules.** The shared directory held pre-1.0 playbooks and an `AGENTS.md` that was only a pointer to another install. Documented behavior ("existing shared directory wins") is intended, but nothing offers the new versions, so a returning teacher never gets the 1.0 setup steps, New Quiz scoring text or teaching-profile guidance. The agent upgraded them by hand: archived old files under `playbook-old/<label>/` and installed the current seed. The product should do this on adoption or on first start after an Engine update: compare seed versus shared copies, archive and replace untouched/older ones, and carry teacher customizations forward (here only the ELA7 tier tags, moved into `context/teaching-profile.md`). There is no CLI/MCP equivalent of the hub's "Get the new version" button.
3. **The course picker fills nicknames from Canvas titles and allows unpublished duplicates.** `context/courses.md` came back with Canvas titles as nicknames (contrary to "do not infer nicknames") and the unpublished duplicate "FUND OF COMPUTER SCIENCE" shell (121051) selected instead of the available one (121046). The picker should show state, hide or de-emphasize unpublished/duplicate shells, and ask for the teacher's own nickname.
4. **Two installs started two hubs.** Cowork then Work each launched a hub (ports 47831 and 55309). The hub is meant to be one instance per Install; the second `install` should reuse a running hub for the same Install.
5. **Setup status stays `saved_not_tested`** after a valid token is saved. Test the connection on save, or report "saved" only until a real call succeeds, so the teacher and agent see an accurate state.
6. **The agent opened the packaged `.mcpb` with the default handler** to reduce the Cowork step to the trust prompt. Whether that reliably shows Claude Desktop's install dialog is unobserved; `install --client cowork` could do this itself and report what it did. Record the outcome honestly.

## Non-goals

No new framework for any of these. Each is a direct fix: write one field, reuse one process, compare seed and shared files, filter a list.

## Pending observation

Cowork trust prompt and tool use, Work restart and tool use, and whether both destinations return the three saved courses and the fresh Install root.

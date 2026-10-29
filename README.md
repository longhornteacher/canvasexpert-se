# CanvasExpert-SE

**A teacher-directed AI assistant for Canvas, with a portable local Engine.**

CanvasExpert-SE (CE-SE) helps teachers draft assignments and New Quizzes, organize course content, assess student work, prepare feedback and contact families. Your chosen agent follows editable teaching playbooks and your preferences; the Engine handles Canvas access, student/attempt identity, exact score arithmetic and supported writes.

**Windows 11 · MIT** · [Latest release](https://github.com/longhornteacher/canvasexpert-se/releases/latest)

[Get started](#get-started) · [Changelog](CHANGELOG.md) · [Release policy](docs/releasing.md) · [Contribute](.github/CONTRIBUTING.md)

The links below install the latest published release; the [changelog](CHANGELOG.md) lists what each release adds. Development continues on `dev`.

This is an independent open-source project. The teacher-first installation mechanics have local fixture and package checks; the complete fresh-user Cowork/Work installation and teaching example still need a destination-client pilot. See the [capability record](docs/engine-capabilities.md) for the distinction between Canvas Live evidence, local tests and pending human validation.

## Get started

The easiest way: paste this into Claude Code or Codex on your Windows PC and let your agent do the rest.

> Install CanvasExpert for me from https://github.com/longhornteacher/canvasexpert-se

Later, "Update CanvasExpert from https://github.com/longhornteacher/canvasexpert-se" gets the newest version without touching your settings or choices. You normally do not need to ask: your assistant stages updates quietly and offers to install them when your work is done.

### For AI agents installing or updating CanvasExpert for a teacher

CanvasExpert runs on Windows only; on a Mac or Chromebook, tell the teacher and stop. Teachers need no developer tools. Do not build this repository, and do not clone its default branch (`dev` is the development source): the teacher-facing folder is the `userinstall` branch, which holds only the playbooks and launchers. It does not contain the program; `get-engine.cmd` downloads that from the matching release and checks its checksum.

1. **Ask the teacher first**, then choose a local folder that OneDrive and Google Drive do not sync (for example `%USERPROFILE%\CanvasExpert-SE`); some district sync folders block programs. Never overwrite an existing folder.
2. If `where git` finds git: `git clone --branch userinstall --single-branch https://github.com/longhornteacher/canvasexpert-se.git "<folder>"`, then in that folder run `.\get-engine.cmd` (needs only Windows' `curl.exe` and `certutil`; no admin rights, no PATH changes).
   If git is missing, don't install it. Download and unpack the release ZIP with Windows' own tools instead: `curl.exe -fL -o "%TEMP%\CanvasExpert-SE-windows.zip" https://github.com/longhornteacher/canvasexpert-se/releases/latest/download/CanvasExpert-SE-windows.zip`, then `mkdir "<folder>"` and `tar -xf "%TEMP%\CanvasExpert-SE-windows.zip" -C "<folder>"`.
   On school networks curl can fail with `CRYPT_E_NO_REVOCATION_CHECK`. Retry that download once with `--ssl-revoke-best-effort`; `get-engine.cmd` does this itself.
3. Run `.\cse.cmd version`. If Windows answers "Access is denied", district security is blocking the program. Do not try other folders or work around it: follow "If Windows blocks the program" in [the install playbook](https://github.com/longhornteacher/canvasexpert-se/blob/userinstall/playbooks/install.md), which confirms the cause from the Windows log and gives the teacher a note for IT. A ZIP install has no playbooks until the program can run, so use that link.
4. Read that folder's `AGENTS.md` and follow `playbooks/install.md` (for a new install, together with `playbooks/setup.md`). The playbooks own the rest: app connections, the private Canvas form and courses, and quiet updates.
5. Tell the teacher to open the CanvasExpert folder itself when they use Claude Code or Codex for teaching work, so its instructions and privacy rules apply.

Check what your own session allows: downloading needs network access from your shell, and connecting apps writes settings outside the folder. Ask for approval where your agent needs it.

To update an existing folder, follow "Updates" in its `playbooks/install.md`. If github.com itself is blocked on the teacher's network, say so plainly: the git install and the ZIP both come from GitHub.

### Install it yourself

1. Paste the prompt above into Claude Code or Codex and let your agent install it, or download [the Windows ZIP](https://github.com/longhornteacher/canvasexpert-se/releases/latest/download/CanvasExpert-SE-windows.zip) (not GitHub's source-code ZIP) and unzip it into the local folder where you want to keep it. That folder is UserInstall; run directly there, with no second install folder.
2. Open that folder in Code/Codex and say **“Set this all up.”** The setup assistant connects every supported app it finds (Claude, ChatGPT, Antigravity), checks the Engine, opens the private Canvas form, helps configure your classes and checks that your teaching assistant can read them.
3. Enter your Canvas address and access token in the private form. Approve the install prompt in your app (restart it only if it asks and you are not mid-review), then use the included `TEACHING-ASSISTANT.md` starter to begin teaching there.

Teachers do not need Dev, Go, Node, a separate Codex CLI, terminal commands or client configuration edits. If the form does not open, the assistant shows its local address, or double-click the included `webui.cmd`. You need the Claude or ChatGPT app on this PC; a website alone cannot reach your UserInstall, and app or organization controls can require additional actions. Saving connection settings or passing a local MCP check does not prove the destination app can use the tools.

Use the [Setup guide](workspace-template/README.md) for the shipped folder and the [MCP adapter guide](docs/mcp-adapter.md) for connection details. See [build and package instructions](docs/build.md) and [releasing](docs/releasing.md) to produce the release from Dev.

## What you can do

| Workflow | CE-SE support |
|---|---|
| Assignments | Draft, create, edit and attach rubrics; preserve pulled edit baselines and unsent work. |
| New Quizzes | Author supported auto-scored and written-response items using QuizForge; create differentiated support pages. Classic Quizzes are not supported. |
| Assessment and feedback | Pull current work and history, apply the Canvas rubric or an explicit local scoring basis, and validate arithmetic before posting. |
| Late work *(1.1 beta)* | Count lateness in school time from your school calendar and grading periods, find each student's first genuine attempt, and correct Canvas late timing without changing grades or feedback. |
| New Quiz written work *(1.8)* | Read typed answers and uploaded files, score each written item, and after the teacher's go-ahead write its points and its own comment (score, explanation, and a model answer below 90%) into the quiz, the same save SpeedGrader makes, plus a whole-assignment comment with Canvas's total and the attempt. It uses an undocumented internal New Quizzes call (checked live 2026-10-29); a SpeedGrader edit since the pull is never overwritten, and the teacher can adjust anything in SpeedGrader afterward. |
| Course content | Author native Pages, upload assets with explicit access and assemble Modules. |
| Canvas Inbox *(1.5.1)* | Read, summarize and manage existing Inbox threads with opaque references; reply only in existing conversations and set read/archive state after the teacher's go-ahead. Message bodies can contain real names. No new conversations, forwarding or attachment sending. Live list/read succeeded and three replies appeared in Canvas on 2026-10-25; state-write repair/retest evidence is tracked in [Engine capabilities](docs/engine-capabilities.md). |
| Book clubs and group work *(1.1 beta)* | Create group sets and clubs, manage memberships, build shared club Pages and discussions, and restrict individual assignments, Pages and New Quizzes to selected clubs in one shared unit module. Grade a shared group product (one grade for the group, or individual grades) and individual discussion contributions. Peer critique, group calendars, group files and creating new club conversations are not included. See [group work](docs/group-work/README.md). |
| Parent updates *(1.1 beta)* | Prepare current-period progress reports from Canvas (missing work, recent grades, what's due soon), review them, then email parents through classic Outlook or print student sheets. |
| Family Contact *(1.1 beta)* | Draft activity notices, reminders and book-club messages for courses, groups or selected students. Review each version and its recipients, then send or save Outlook drafts with inline images and file attachments. Requires local classic Outlook with a configured profile. See [Family Contact](docs/family-contact-flow.md). |
| Teaching context | Keep teaching priorities, standards, feedback voice, calendar and student notes in editable files; onboarding grows as you work. |
| Local hub | Configure Canvas and courses, manage teacher preferences, review gated writes and inspect capability evidence. |

The agent supplies educational judgment. The Engine supplies mechanics, not a curriculum or a grading policy. Your teacher files remain authoritative across agent apps; CE-SE does not copy a developer's classes or preferences into your teaching profile.

## You control the work

“Draft” saves local work and does not authorize Canvas writes. For scores, new assignments, quizzes and assignment edits, the agent reports the work in the conversation and writes on the teacher’s go-ahead there; a request that already includes the write authorizes it directly. The scoring report includes impressions, score distribution and one exact Engine-rendered feedback sample. Native Pages, uploads, Modules and rubric attachments follow the requested scope directly. Family emails, including grade updates, are sent only when the teacher clicks Send on the private review page; Drafts only saves them in Outlook.

Ordinary assignment grading posts the honest computed assessment, including when it is below an existing entered grade. Canvas calculates late point penalties. Missing or unreadable evidence is reported specifically, while independent work continues. See [scoring rules](docs/SCORING-RULES.md) and the current [teacher playbooks](workspace-template/playbooks/) for details.

## Data and privacy

- Your Canvas PAT is stored in Windows Credential Manager on your computer, not in teacher files or client configuration. Enter it only in the private local WebUI; never send it to an agent in chat.
- Pulled student work, drafts and operational state stay in your local UserInstall. The active setup location (UserInstall or UserSync) holds the private student identity map and uses pseudonyms in ordinary work pulls; agents are instructed not to read the private `.cse/` directory.
- Evidence and teaching context the agent reads are sent into your selected AI provider's session. Submissions, attachments, teacher notes and read-only API responses can still contain names or other identifying information. CE-SE does **not** make all data anonymous.
- Optional UserSync shares reusable teacher setup through a cloud provider you choose, including course selections, student aliases/settings/notes and private roster/contact records. Pulled work, drafts and grading/planning sessions remain local. CE-SE does not synchronize files itself.
- Family Contact resolves names and email addresses inside the Engine and shows To/CC only on the teacher's private review page. Agent-facing audience files use student references and aliases. Outlook manages any saved email drafts and sent messages in its configured mailbox.

Use a school-approved AI client and follow your school's student-data and Canvas access policies. The project does not claim district approval or student-data compliance certification. It is not affiliated with or endorsed by Instructure, OpenAI or Anthropic. The maintainers collect no data from CE-SE users; see the [privacy policy](docs/PRIVACY.md).

## How it fits together

```text
Your chosen agent + current teaching playbooks/context
                         |
                    MCP or CLI
                         |
              Local Engine + review hub
                         |
                     Canvas Live
```

**Dev** is this development checkout. **UserInstall** is the extracted runnable folder containing the Engine and the teacher's actual files. **UserSync** is an optional authoritative location for shared teaching rules and preferences. Teachers need only UserInstall; development happens in Dev. The [architecture](docs/ARCHITECTURE.md) explains the boundaries.

## Contribute

Start with [CONTRIBUTING.md](.github/CONTRIBUTING.md), the [build guide](docs/build.md) and [development notes](docs/development-notes.md). Focused bug reports, documentation improvements and practical teacher workflows are welcome. Keep student information, credentials and private teacher files out of issues and pull requests.

Read the [changelog](CHANGELOG.md), [security reporting guidance](.github/SECURITY.md), [privacy policy](docs/PRIVACY.md) and [MIT license](LICENSE).

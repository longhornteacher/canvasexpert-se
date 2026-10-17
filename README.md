# CanvasExpert-SE

**A teacher-directed AI assistant for Canvas, with a portable local Engine.**

CanvasExpert-SE (CE-SE) helps teachers draft assignments and New Quizzes, organize course content, assess student work and prepare feedback. Your chosen agent follows editable teaching playbooks and your preferences; the Engine handles Canvas access, student/attempt identity, exact score arithmetic and supported writes.

**1.0 · `1.0.0` · Windows 11 · MIT**

[Get started](#get-started) · [Changelog](CHANGELOG.md) · [Release policy](docs/releasing.md) · [Contribute](CONTRIBUTING.md)

This is an independent open-source project. The teacher-first installation mechanics have local fixture and package checks; the complete fresh-user Cowork/Work installation and teaching example still need a destination-client pilot. See the [capability record](docs/engine-capabilities.md) for the distinction between Canvas Live evidence, local tests and pending human validation.

## Get started

The easiest way: paste this into Claude Code or Codex on your Windows PC and let your agent do the rest.

> Install CanvasExpert for me from https://github.com/longhornteacher/canvasexpert-se

Later, "Update CanvasExpert from https://github.com/longhornteacher/canvasexpert-se" gets the newest version without touching your settings or choices.

### For AI agents installing or updating CanvasExpert for a teacher

Do not clone or build this repository; teachers need no developer tools. Use the published Windows build:

1. Download https://github.com/longhornteacher/canvasexpert-se/releases/latest/download/CanvasExpert-SE-windows.zip (always the newest release; the same ZIP is also attached to each [release](https://github.com/longhornteacher/canvasexpert-se/releases) under its versioned name). Ask the teacher before downloading and running it.
2. **Install:** extract it to a local folder that is not synced by OneDrive or Google Drive (for example `%USERPROFILE%\CanvasExpert-SE`); some district sync folders block programs. **Update:** extract it over the teacher's existing CanvasExpert folder. The ZIP contains no playbooks, settings or teacher files, so nothing of theirs is overwritten. If `engine\cse.exe` is in use, ask the teacher to close the apps using CanvasExpert first, never during an active review.
3. Run that folder's `cse.cmd playbooks update`, then read its `AGENTS.md` and follow `playbooks/install.md` (for a new install, together with `playbooks/setup.md`). The playbooks own the rest: app connections, the private Canvas form and courses, or reconciling any playbook the teacher edited.
4. Tell the teacher to open the CanvasExpert folder itself when they use Claude Code or Codex for teaching work, so its instructions and privacy rules apply.

### Install it yourself

1. Download [the Windows ZIP](https://github.com/longhornteacher/canvasexpert-se/releases/latest/download/CanvasExpert-SE-windows.zip) (not GitHub's source-code ZIP) and unzip it into the local folder where you want to keep it. That folder is UserInstall; run directly there, with no second install folder.
2. Open that folder in Code/Codex and say **“Set this all up.”** The setup assistant connects every supported app it finds (Claude, ChatGPT, Antigravity), checks the Engine, opens the private Canvas form, helps configure your classes and checks that your teaching assistant can read them.
3. Enter your Canvas address and access token in the private form. Approve the install prompt in your app (restart it only if it asks and you are not mid-review), then use the included `TEACHING-ASSISTANT.md` starter to begin teaching there.

Teachers do not need Dev, Go, Node, a separate Codex CLI, terminal commands or client configuration edits. If the form does not open, the assistant shows its local address, or double-click the included `webui.cmd`. You need the Claude or ChatGPT app on this PC; a website alone cannot reach your UserInstall, and app or organization controls can require additional actions. Saving connection settings or passing a local MCP check does not prove the destination app can use the tools.

Use the [Setup guide](workspace-template/README.md) for the shipped folder and the [MCP adapter guide](docs/mcp-adapter.md) for connection details. See [build and package instructions](docs/build.md) to produce the release ZIP from Dev.

## What you can do

| Workflow | CE-SE support |
|---|---|
| Assignments | Draft, create, edit and attach rubrics; preserve pulled edit baselines and unsent work. |
| New Quizzes | Author supported auto-scored and written-response items using QuizForge; create differentiated support pages. Classic Quizzes are not supported. |
| Assessment and feedback | Pull current work and history, apply the Canvas rubric or an explicit local scoring basis, and validate arithmetic before posting. |
| New Quiz written work | Prepare criterion scores and feedback as a submission comment. The teacher enters written-item points in SpeedGrader; this path writes no quiz/item grade. |
| Course content | Author native Pages, upload assets with explicit access and assemble Modules. |
| Teaching context | Keep teaching priorities, standards, feedback voice, calendar and student notes in editable files; onboarding grows as you work. |
| Local hub | Configure Canvas and courses, manage teacher preferences, review gated writes and inspect capability evidence. |

The agent supplies educational judgment. The Engine supplies mechanics, not a curriculum or a grading policy. Your teacher files remain authoritative across agent apps; CE-SE does not copy a developer's classes or preferences into your teaching profile.

## You control the work

“Draft” saves local work and does not authorize Canvas writes. Scores, new assignments, new quizzes and assignment edits use the teacher review page by default. You can explicitly skip review for a particular task. Native Pages, uploads, Modules and rubric attachments follow the requested scope directly; they do not share that review gate.

Ordinary assignment grading posts the honest computed assessment, including when it is below an existing entered grade. Canvas calculates late point penalties. Missing or unreadable evidence is reported specifically, while independent work continues. See [scoring rules](SCORING-RULES.md) and the current [teacher playbooks](workspace-template/playbooks/) for details.

## Data and privacy

- Your Canvas PAT is stored in Windows Credential Manager on your computer, not in teacher files or client configuration. Enter it only in the private local WebUI; never send it to an agent in chat.
- Pulled student work, drafts and operational state stay in your local UserInstall. The active setup location (UserInstall or UserSync) holds the private student identity map and uses pseudonyms in ordinary work pulls; agents are instructed not to read the private `.cse/` directory.
- Evidence and teaching context the agent reads are sent into your selected AI provider's session. Submissions, attachments, teacher notes and read-only API responses can still contain names or other identifying information. CE-SE does **not** make all data anonymous.
- Optional UserSync shares reusable teacher setup through a cloud provider you choose, including course selections, student aliases/settings/notes and private roster/contact records. Pulled work, drafts and grading/planning sessions remain local. CE-SE does not synchronize files itself.

Use a school-approved AI client and follow your school's student-data and Canvas access policies. The project does not claim district approval or student-data compliance certification. It is not affiliated with or endorsed by Instructure, OpenAI or Anthropic.

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

**Dev** is this development checkout. **UserInstall** is the extracted runnable folder containing the Engine and the teacher's actual files. **UserSync** is an optional authoritative location for shared teaching rules and preferences. Teachers need only UserInstall; development happens in Dev. The [architecture](ARCHITECTURE.md) explains the boundaries.

## Contribute

Start with [CONTRIBUTING.md](CONTRIBUTING.md), the [build guide](docs/build.md) and [development notes](docs/development-notes.md). Focused bug reports, documentation improvements and practical teacher workflows are welcome. Keep student information, credentials and private teacher files out of issues and pull requests.

Read the [changelog](CHANGELOG.md), [security reporting guidance](SECURITY.md) and [MIT license](LICENSE).

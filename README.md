# CanvasExpert-SE

**A teacher-directed AI assistant for Canvas, with a portable local Engine.**

CanvasExpert-SE (CE-SE) helps teachers draft assignments and New Quizzes, organize course content, assess student work and prepare feedback. Your chosen agent follows editable teaching playbooks and your preferences; the Engine handles Canvas access, student/attempt identity, exact score arithmetic and supported writes.

**1.0 beta · `1.0.0-beta.1` · Windows 11 · MIT**

[Get started](#get-started) · [Changelog](CHANGELOG.md) · [Release policy](docs/releasing.md) · [Contribute](CONTRIBUTING.md)

This is an independent open-source project. The teacher-first installation mechanics have local fixture and package checks; the complete fresh-user Cowork/Work installation and teaching example still need a destination-client pilot. See the [capability record](docs/engine-capabilities.md) for the distinction between Canvas Live evidence, local tests and pending human validation.

## Get started

Published builds belong on [GitHub Releases](https://github.com/longhornteacher/canvasexpert-se/releases). Look for the versioned Windows ZIP rather than GitHub's source-code ZIP; if no beta asset is available yet, use the [build guide](docs/build.md).

1. Unzip the CanvasExpert-SE release ZIP into a folder on your computer's local drive.
2. Open that folder in Code/Codex and say **“Install this for Cowork”** or **“Install this for Work.”** The installing assistant handles the selected connection, checks the Engine and opens the local Canvas form.
3. Enter your Canvas address and access token in the private WebUI. Complete any trust prompt or restart your chosen app requires, then use the included `TEACHING-ASSISTANT.md` starter to begin teaching there.

Teachers do not need Source, Go, Node, a separate Codex CLI, terminal commands or client configuration edits. Double-click the included `webui.cmd` as a fallback to open the hub. A supported local agent app/account is needed; hosted Work is a separate connection route, and app or organization controls can require additional actions. Saving connection settings or passing a local MCP check does not prove the destination app can use the tools.

Use the [Install guide](workspace-template/README.md) for the shipped folder and the [MCP adapter guide](docs/mcp-adapter.md) for connection details. See [build and package instructions](docs/build.md) to produce the release ZIP from Source.

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
- Pulled student work, drafts and operational state stay in your local Install. The Engine maintains a private student identity map and uses pseudonyms in ordinary work pulls; agents are instructed not to read the private `.cse/` directory.
- Evidence and teaching context the agent reads are sent into your selected AI provider's session. Submissions, attachments, teacher notes and read-only API responses can still contain names or other identifying information. CE-SE does **not** make all data anonymous.
- Optional Sync shares selected teacher rules and preferences through a cloud provider you choose; course/student configuration and operational work remain local. CE-SE does not synchronize files itself.

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

**Source** is this development checkout. **Install** is the extracted runnable folder containing the Engine and the teacher's actual files. **Sync** is an optional authoritative location for shared teaching rules and preferences. Teachers need only Install; development happens in Source. The [architecture](ARCHITECTURE.md) explains the boundaries.

## Contribute

Start with [CONTRIBUTING.md](CONTRIBUTING.md), the [build guide](docs/build.md) and [development notes](docs/development-notes.md). Focused bug reports, documentation improvements and practical teacher workflows are welcome. Keep student information, credentials and private teacher files out of issues and pull requests.

Read the [changelog](CHANGELOG.md), [security reporting guidance](SECURITY.md) and [MIT license](LICENSE).

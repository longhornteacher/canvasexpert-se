# Privacy Policy

Effective 2026-10-22 · applies to CanvasExpert-SE 1.2.0-beta.1 and later

CanvasExpert-SE (CE-SE) is free, open-source software that a teacher downloads and runs on their own Windows computer. It is not a hosted service. **The maintainers do not operate servers for CE-SE and do not receive, collect, store or sell any data from people who use it.** CE-SE has no accounts, analytics, telemetry, advertising, crash reporting or automatic update checks.

This policy explains where information goes when a teacher uses CE-SE. [SECURITY.md](../.github/SECURITY.md) has the technical details.

## Who is responsible for what

- **The teacher (and their school)** decides which courses, students and files CE-SE works with, which AI service to connect and which cloud storage to use, if any. School and district data policies, including any approval required for AI tools, apply to that use.
- **The maintainers** publish the code. They do not see, process or have access to any teacher's Canvas data, student information or files.
- **Third-party services the teacher chooses** (listed below) handle data under their own terms and privacy policies.

## What stays on the teacher's computer

| Information | Where it is kept |
| --- | --- |
| Canvas address and personal access token (PAT) | Windows Credential Manager. Never written to CE-SE files, client settings or agent output. |
| Student identity map, roster and parent/student contacts | The private `.cse/` folder in UserInstall (or UserSync, if configured). Excluded from the AI tools CE-SE provides. |
| Pulled student work, attachments, drafts and grading or planning sessions | UserInstall on the local computer. |
| Teacher setup: course selections, preferences, playbooks, student aliases and notes | UserInstall by default, or UserSync if the teacher configures it. |
| Teammates' Canvas keys (team PC only) | Windows Credential Manager on the team PC. Never written to CE-SE files, client settings or agent output. |
| Team list and teammate preferences | `context/team.md` and `context/team/<label>.md`: UserInstall by default, or UserSync if the teacher configures it. The list holds no keys. |
| Team action log | `.cse/team-log.jsonl` in UserInstall. Records each write made with a teammate's key, without student names, comments or grades. |

Uninstalling is deleting the UserInstall folder (and UserSync, if used) and removing the CE-SE entry from Windows Credential Manager. Revoking the PAT in Canvas ends CE-SE's access.

## Where information can leave the computer

CE-SE only sends data to places the teacher has set up:

1. **The school's Canvas.** CE-SE reads and writes course data using the teacher's own PAT, with the same permissions the teacher already has in Canvas. Writes (grades, comments, pages, quizzes and other content) follow the teacher's request, and review is the default before scores or new content are posted.
2. **The teacher's AI assistant** (for example Claude Code or Codex). Anything an assistant reads for a task enters that provider's context: assignment content, student writing, rubric results and teacher notes. Student names are replaced with pseudonyms in ordinary work packets, but **this is not full anonymization**: student writing, attachments and some Canvas responses can contain real names or other identifying details. The AI provider's privacy policy and account settings (including data retention and training) apply. Use an AI account your school approves for student data.
3. **Optional cloud storage (UserSync).** If the teacher points UserSync at OneDrive, Google Drive or a similar folder, that provider synchronizes those files, including the private roster and contact records. CE-SE does not upload anything itself; the provider's sync client does.
4. **Optional parent emails.** When the teacher approves parent updates on the local review page, CE-SE hands each message to the teacher's own classic Outlook, which sends it or saves it as a draft. CE-SE does not send email through any other service.
5. **Teammates' courses on the team PC.** On a team PC, CE-SE reads and writes teammates' courses with each teammate's own key. That teammate's students' work and contacts are stored on the team PC like the teacher's own. A teammate can end access at any time by deleting the token in Canvas. Family emails for those students are sent from the team PC's Outlook, with replies routed to the teammate.

CE-SE's local setup and review pages run only on the teacher's computer (`127.0.0.1`) and are not reachable from other machines.

## Children's and student information

CE-SE is a tool for teachers, not students. Students do not use it, and it collects nothing directly from them. Student information it handles comes from the school's Canvas through the teacher's access and is treated as education records under the teacher's and school's control. CE-SE is not certified under any student-privacy program (such as iKeepSafe or Project Unicorn), and an open-source license does not mean a school has approved it.

## Reporting a privacy problem

If CE-SE exposes data it shouldn't, report it privately as described in [SECURITY.md](../.github/SECURITY.md#reporting-a-vulnerability). Do not include PATs, student records or screenshots of real student data in a public issue.

## Changes

Changes to this policy are recorded in this file's history in the project repository, with a new effective date.

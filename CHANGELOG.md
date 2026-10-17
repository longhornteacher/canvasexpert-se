# Changelog

Notable changes are recorded here. Version names use semantic versioning; **1.0** is `1.0.0` and **1.0 beta** was `1.0.0-beta.1`.

## Development notes

### Collaborative group sets

- Dev can list collaborative group sets and clubs, pull an exact-ID baseline, and create or edit group-set settings and club names/descriptions. Canvas differentiation tags remain excluded. Local implementation checks do not establish live or student access behavior.

## 1.0.0 (2026-10-17)

First stable release.

### Install and update from one link

- An agent can install or update CanvasExpert from the repository link: the README tells it to download the newest Windows ZIP from a permanent release link, extract it to a local folder, and run `cse.cmd playbooks update`.
- The ZIP no longer carries playbooks, `TEACHER.md` or teacher files. The Engine lays them down on first run, so extracting a newer ZIP over an existing folder never overwrites a teacher's settings or choices. With UserSync, update restores the local pointer files.
- `cse playbooks update` (MCP `update-playbooks`) keeps each shipped version under `shipped/`, replaces playbooks a teacher never edited (including older versions recognized by fingerprint), and keeps edited ones for the agent to reconcile.

### Teacher choices

- `Teacher_Override.md` holds a teacher's overrides of shipped defaults, read after the playbooks and never replaced. Standing preferences such as "always skip review when I ask to post" are saved there.
- Shipped defaults no longer include one district's grade-portal and tutoring links; teachers supply their own. The school calendar starter includes a bell schedule.

### Feedback and scoring

- Feedback leads with `Overall: X/Y` (marked `(late)` when late), then one line per item. New Quiz feedback never carries an overall line.
- Class exemplars: two anonymous excerpts per prompt, sent as a comment-only follow-up once the last matching student is scored. `comment-only` now works on ordinary assignments and never changes a grade.
- Parent updates list corrections as soon as work is finished, not only after feedback is posted.
- Rationales are required only for auto-scored quiz items.

### Verification

Comment layout, the lateness write and the comment-only follow-up were verified in Canvas Live on Test-Student-only CS8 assignments; install and update were verified locally and on the developer's own install. A colleague's install from the link has not yet been observed. See the [capability record](docs/engine-capabilities.md).

## 1.0.0-beta.1

This beta brings together the current Engine, teacher playbooks and portable Windows Install. This entry describes the release scope; it does not claim a public release date or complete client validation.

### Teacher-first installation

- Bundled selected-client installation for local Work/Codex or Cowork, without a separate Codex CLI.
- Private WebUI launch independent of destination restart, plus local MCP diagnostics that distinguish saved configuration from destination tool access.
- Configuration preservation, repeat-install behavior and an explicit choice before replacing another Install's connection.
- Thin Cowork extension launcher targeting Install's Engine rather than carrying a second Engine.
- Three-step onboarding, a short teaching-assistant starter and an optional teaching profile consulted by scoring and authoring.

### Teaching workflows

- Assignment creation/editing, rubric attachment, native Pages, course assets and Module assembly.
- QuizForge New Quiz authoring, including essay and file-upload items and differentiated support pages.
- Scoped work pulls, rubric/local-basis assessment, exact score arithmetic, review and ordinary assessment/feedback posting.
- New Quiz written-response review and comment-only feedback; item points remain teacher-entered in SpeedGrader.
- Authoritative editable teacher rules in Install or optional Sync, with preservation of existing preferences and unsent work.

### Beta limits

Fresh-user installation under a standard Windows account, destination-client adoption of the new routes and a teacher-judged lesson/feedback example remain pending. Selected Canvas operations have separate recorded live evidence; local tests do not prove student rendering, client compatibility or every district configuration. See the [capability record](docs/engine-capabilities.md) for current evidence and limitations.

# Changelog

Notable changes are recorded here. Version names use semantic-version prerelease notation; **1.0 beta** is `1.0.0-beta.1`.

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

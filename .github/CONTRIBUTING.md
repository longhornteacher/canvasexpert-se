# Contributing to CanvasExpert-SE

CE-SE is a practical teacher tool: the agent makes educational judgments from teacher-owned playbooks and context, while the Engine handles Canvas access, identity, arithmetic and supported writes. Contributions should help a teacher complete a real task with little friction.

## Before changing code

Read [AGENTS.md](../AGENTS.md), [ARCHITECTURE.md](../docs/ARCHITECTURE.md) and [SCORING-RULES.md](../docs/SCORING-RULES.md). Develop in **Dev**, this checkout. Use synthetic temporary **UserInstall** fixtures for testing; do not develop against a teacher's runnable UserInstall or private UserSync files.

For a substantial change, explain the teacher workflow, expected behavior and meaningful limitations in an issue or implementation handoff before expanding the design. Prefer direct implementations and existing packages. Put educational judgment in playbooks/context, not Go. Preserve teacher intent, credentials, identity boundaries and unsent work.

## Build and validate

The development build requires PowerShell and the exact Go toolchain declared in [`go.mod`](../go.mod). Teachers do not need these runtimes. See [docs/build.md](../docs/build.md) for packaging and toolchain selection.

From Dev:

```powershell
.\scripts\build.ps1
```

The build runs Go tests and vet and produces the single current output set in `dist/` (Engine, thin MCP launcher, `CanvasExpert-SE-<VERSION>.zip`, checksums); see [docs/build.md](../docs/build.md). Run focused tests while developing, then complete the required build checks before submitting a code change. Format only changed Go files with `gofmt`. To verify release assembly:

```powershell
.\scripts\package.ps1
```

Use focused regressions for changed behavior, with local fixtures and synthetic identities. Do not add coverage quotas or tests that merely restate implementation. A straightforward documentation change generally needs link/content review rather than new runtime tests. Local fixture success must never be labeled Canvas Live proof or destination-client validation.

When capabilities, limitations or verification status change, update `internal/localui/capabilities.go` and [docs/engine-capabilities.md](../docs/engine-capabilities.md) together. Keep Engine contracts in `engine-docs/` and teacher instructions in `workspace-template/` aligned with behavior. Update the [changelog](../CHANGELOG.md) for notable user-visible changes.

## Report a bug or propose a change

Include the CE-SE version/build, agent app and mode, Windows version, intended task, actual outcome and minimal reproduction. Distinguish a saved connection, a local MCP check and successful destination tool use. For Canvas failures, report the non-secret error and affected operation; do not attach authentication headers or full raw response dumps.

Use synthetic examples. Remove student names, IDs, submissions, accommodation notes, private course URLs, PATs and unrelated client configuration. Never attach `.cse/`, a teacher UserInstall/UserSync archive or a real pulled-work packet. Follow [SECURITY.md](SECURITY.md) for suspected security issues.

## Pull requests

Describe the concrete problem and resulting behavior, relevant checks and any remaining evidence limits. Keep changes scoped; do not migrate teacher files or register live clients as part of Dev tests. Existing credentials and historical course IDs do not authorize a Canvas write. Any live trial needs a concrete teacher-authorized scope and the ordinary review rules.

Code and contributions are distributed under the project's [MIT license](../LICENSE). Do not include third-party teaching materials or student work unless you have the necessary rights and permission.

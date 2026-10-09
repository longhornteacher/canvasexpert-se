# Source development

Optimize for the teacher completing the requested task with little friction. Use existing packages and direct implementations; add defenses for concrete supported workflows or consequential demonstrated failures. Keep validation at useful boundaries and tests proportionate to changed behavior. Do not add coverage quotas, certification rituals, universal operation frameworks, or speculative migrations.

Preserve teacher intent and unsent work. Canvas Live is authoritative. Check course/student/attempt identity, protect credentials and teacher-only notes, post the honest computed assessment, keep rubric assessments honest, and respect requested posting scope. Continue independent work when one item cannot proceed and explain that item specifically.

CE-SE has two parts: the **Engine** (Go code in `cmd/`, `internal/`: Canvas access, identity, exact math, safe writes) and the **playbooks and context** (all judgment; `workspace-template/` is their seed embedded in the Engine, and the Engine's own contract docs, such as the format reference, live with the Engine). Put judgment in the playbooks, not the Engine.

Read [ARCHITECTURE.md](ARCHITECTURE.md) and [SCORING-RULES.md](SCORING-RULES.md). The shipped workspace AGENTS.md is for teacher sessions; the old 0.1b brief is historical.

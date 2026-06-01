---
name: feedback-load-go-implementation-skill
description: Always load the full go-implementation skill when working on Go code — mandatory and non-negotiable
metadata:
  type: feedback
---

When executing any task that touches Go code, load ALL necessary references from `.agents/skills/go-implementation/` including:
- `SKILL.md`
- `references/architecture.md`
- `references/testing.md` (for tests)
- `references/examples-testing.md` (for canonical test patterns)
- `references/error-handling.md`
- Any other reference the task requires

Do not load only partial references. Load everything the task actually needs.

**Why:** User said "É INEGOCIÁVEL E MANDATÓRIO USAR TUDO QUE FOR NECESSÁRIO DA SKILL go-implementation.md, carregar tudo que realmente for necessário".
**How to apply:** Before implementing any Go code, check the go-implementation/references/ directory and load all files relevant to the scope of the task.

---
name: feedback-load-go-implementation-skill
description: Every Go task must load go-implementation skill + examples on-demand with economy (max 4 refs)
metadata:
  type: feedback
---

Every task that implements Go code MUST load `.agents/skills/go-implementation/SKILL.md` and only the examples/references the specific task actually needs — never speculatively load references not triggered by the task scope.

Economy rules (from CLAUDE.md):
- Max 4 references simultaneously per task
- Never load `patterns-structural.md` for Factory/Options/Adapter/Decorator/Facade (already inline in SKILL.md)
- Load `examples-domain-flow.md`, `examples-testing.md`, `examples-infrastructure.md` only when the task involves end-to-end flow, test strategy, or infrastructure lifecycle respectively
- If >4 references needed: prioritize 3 most critical, note the others as "contexto não carregado"

**Why:** User: "obrigatório e inegociável — toda task que implementa código golang, tem que carregar a skill go-implementation e seus exemplos sob demanda focando em economia".
**How to apply:** In every subagent prompt for Go tasks, explicitly instruct: load SKILL.md + only the references the task scope triggers. Do not load everything at once.

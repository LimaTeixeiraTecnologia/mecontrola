# Plano: Prompt Enriquecido — Módulo Budgets

## Skills Necessárias

- `.agents/skills/go-implementation/SKILL.md` — obrigatória para qualquer edição em `.go`

## Contexto

O arquivo `docs/melhorias/2026-06-17-budgets.md` define um "Prompt Enriquecido" para capacitar o `internal/agent` com conhecimento profundo sobre o módulo `internal/budgets`. O `persona.system.tmpl` mencionava orçamento com 1 frase. Sem conhecimento sobre: rascunhos/ativação, limiares de alerta (80%/50%/85%), recorrência, ou as 5 categorias raiz.

**`JourneyHint` nunca era populado** no runtime — a struct existia mas `NewComposeConversationalReply` sempre passava `PersonaSystemData{}` (vazio).

## Dados concretos confirmados no código

| Item | Valor exato |
|------|-------------|
| Slugs raiz | `expense.custo_fixo`, `expense.conhecimento`, `expense.prazeres`, `expense.metas`, `expense.liberdade_financeira` |
| Limiar categoria | `0.80` (80%) |
| Limiar meta (goal) | `0.50` (50%) |
| Limiar cartão | `0.85` (85%) |
| Estados Budget | `BudgetStateDraft` (1), `BudgetStateActive` (2) |
| Soma de allocations | exatamente 10 000 basis points = 100% |

## Arquivos modificados / criados

| Arquivo | Ação |
|---------|------|
| `internal/agent/application/prompting/budgets.system.tmpl` | **Criado** — template standalone do prompt enriquecido |
| `internal/agent/application/prompting/prompts.go` | **Atualizado** — embed + `BudgetsPersonaData` + `RenderBudgetsPersona()` |
| `internal/agent/application/prompting/persona_test.go` | **Atualizado** — 4 testes para `RenderBudgetsPersona` |
| `internal/agent/application/prompting/persona.system.tmpl` | **Atualizado** — seção 4 expandida com dados concretos |

## Regras aplicadas

- R-ADAPTER-001.1: zero comentários em `.go` de produção
- R5.26: globais em camelCase (`budgetsSystemRaw`, `budgetsSystemTpl`)
- R0: sem `init()`
- Template syntax: Go `text/template` — mesmo padrão de `persona.system.tmpl`
- `template.Must()` — fail-fast no startup (padrão existente)

## Verificação executada

```bash
go build ./internal/agent/...
go test -race -count=1 ./internal/agent/application/prompting/...
go test -race -count=1 ./internal/agent/application/usecases/...
```

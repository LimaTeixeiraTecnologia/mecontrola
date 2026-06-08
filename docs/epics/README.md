# Épicos — MeControla

Esta pasta hospeda os **épicos derivados do brainstorm decisório `consolidacao-core`**. Cada arquivo `.md` é uma cápsula auto-contida que pode ser consumida pelas skills downstream (`create-prd`, `create-technical-specification`, `create-tasks`, `execute-task`, `execute-all-tasks`) para materializar o ciclo completo de implementação.

## Fonte canônica

- **Bundle de decisão:** `.agents/skills/decision-brainstorming/discoveries/brainstorm-consolidacao-core/decision-brief.md` (seção **Arquitetura Inegociável**).
- **Discoveries de origem:**
  - `docs/discoveries/discovery-billing-hotmart-kiwify.md`
  - `docs/discoveries/discovery-identity-entitlement.md`
  - `docs/discoveries/discovery-onboarding-flow.md`

## Roadmap

| Épico | Slug | Status | Bloqueado por | Bloqueia | Artefatos gerados |
|---|---|---|---|---|---|
| **E1** | `identity-foundation` | Implementado | — (raiz) | E2, E3 | `.specs/prd-identity-foundation/` (prd + techspec + tasks) |
| **E2** | `billing-pipeline` | Implementado | E1 | E4 | `.specs/prd-billing-pipeline/` (prd + techspec + tasks) |
| **E3** | `onboarding-magic-token` | Implementado | E1 | E4 | `.specs/prd-onboarding-magic-token/` (prd + techspec + tasks) |
| **E4** | `reconciliation-hardening` | Backlog pós-MVP | E2 e E3 (parcial) | — | — |

```
E1 (bloqueador)
 ├──► E2 ──┐
 └──► E3 ──┴──► E4 (pós-MVP)
```

## Convenção de cada arquivo

Cada `epic-NN-<slug>.md` segue a estrutura:

1. **Header de metadados** (frontmatter YAML): id, slug, status, bloqueios, próxima skill recomendada, caminhos esperados de artefatos downstream.
2. **Contexto e motivação** — pronto para alimentar `create-prd`.
3. **Escopo incluído / fora de escopo** — bordas explícitas.
4. **Entregáveis** — funcionalidades e contratos.
5. **Restrições inegociáveis** — herdadas do bundle.
6. **Critérios de aceite** — métricas testáveis.
7. **Dependências externas e pré-requisitos não-técnicos** — gates fora do código.
8. **Próximos passos sugeridos** — sequência de skills.

## Como usar cada épico

```bash
# 1. Materializar PRD (a partir do épico)
ai-spec create-prd
#  → consome docs/epics/epic-NN-<slug>.md
#  → produz .specs/prd-<slug>/prd.md

# 2. Materializar techspec
ai-spec create-technical-specification
#  → consome .specs/prd-<slug>/prd.md + docs/epics/epic-NN-<slug>.md
#  → produz .specs/prd-<slug>/techspec.md

# 3. Decompor em tarefas
ai-spec create-tasks
#  → consome PRD + techspec
#  → produz .specs/prd-<slug>/tasks.md + task-NN-*.md

# 4. Executar tarefas
ai-spec execute-all-tasks
#  → consome tasks.md
#  → produz *_execution_report.md por tarefa + _orchestration_report.md
```

## Regra de bloqueio

- Um épico **só pode entrar em implementação** (`execute-all-tasks`) quando todos os épicos em `blocked_by` tiverem `status: implemented`.
- O **PRD e a techspec** de um épico bloqueado **podem ser escritos em paralelo** ao bloqueador, mas a execução de tarefas espera.
- Mudanças na **Arquitetura Inegociável** do bundle reabrem brainstorm — não basta atualizar épico.

# Run — Reconciliar drift do roadmap (E1, E2, E3) em vez de abrir novo PRD

> Data: 2026-06-08
> Origem: `docs/prompts/prompt-create-prd-proximo-epico-roadmap.md`
> Plan file: `~/.claude/plans/users-jailtonjunior-git-mecontrola-docs-snoopy-catmull.md`

## Contexto

O pedido original era usar a skill `create-prd` para abrir o próximo PRD do roadmap entre E1–E4, com qualidade production-ready, eficiência e **sem falso positivo**. A leitura do working tree mostrou que **nenhum dos quatro épicos é candidato legítimo a `create-prd` agora**:

| Épico | Status no épico | Realidade no working tree | Decisão |
|---|---|---|---|
| E1 `identity-foundation` | `status: prd_done`, `next_skill: create-technical-specification`, `artifacts.techspec/tasks: null` | PRD + techspec + tasks + código implementado (72 arquivos Go, módulo `internal/identity`) | **Descartado** (já além de PRD; drift em artifacts/next_skill) |
| E2 `billing-pipeline` | `status: prd_done`, `next_skill: create-technical-specification`, `artifacts.techspec/tasks: null` | PRD + techspec + tasks + código implementado (97 arquivos Go, módulo `internal/billing`) | **Descartado** (já além de PRD; drift em artifacts/next_skill) |
| E3 `onboarding-magic-token` | `status: pending`, `next_skill: create-prd`, `artifacts.prd/techspec/tasks: null` | PRD production-ready (19 RFs, 17 suposições rastreadas), techspec, tasks e ~89 arquivos Go (`internal/onboarding`). Commit recente `cdf2287 fix(onboarding): corrija ativacao por magic token` | **Descartado para novo PRD** (já materializado; drift severo no épico) |
| E4 `reconciliation-hardening` | `status: backlog`, `next_skill: null`, com instrução textual *"Não abrir PRD agora. Reabrir quando: MVP em produção > 4 semanas, ou incidente justificar, ou volume > 5k subs"* | Sem artefatos materializados | **Descartado** por regra 3 do prompt enriquecido (timing explícito) |

Pelas regras 2 (não duplicar PRD coberto), 5 (working tree prevalece sobre docs) e pelo critério "sem falso positivo" do próprio prompt, a ação correta é **reconciliar o frontmatter dos épicos implementados** para refletir o estado real, **não** rodar `create-prd`.

O PRD de E3, em particular, já é production-ready: cobre 19 requisitos funcionais (incluindo defesa contra enumeração na thank-you page — RF-17; tolerância a pagamento sem token — RF-18; WCAG 2.1 AA — RF-19), 17 suposições/perguntas em aberto explicitamente rastreadas (S-01..S-17), acoplamento contratual com E1/E2 e exclusão explícita de E4. Não há lacuna material que justifique reabertura.

## Decisão

**Não abrir novo PRD.** Reconciliar o frontmatter de E1, E2 e E3 + a tabela Roadmap do README dos épicos para eliminar o drift entre documentação e working tree. Manter E4 intocado (decisão de timing explícita continua válida).

## Mudanças concretas

### 1. `docs/epics/epic-01-identity-foundation.md` (frontmatter)

- `status: prd_done` → `status: implemented`
- `artifacts.techspec: null` → `artifacts.techspec: .specs/prd-identity-foundation/techspec.md`
- `artifacts.tasks: null` → `artifacts.tasks: .specs/prd-identity-foundation/tasks.md`
- `next_skill: create-technical-specification` → `next_skill: null`

### 2. `docs/epics/epic-02-billing-pipeline.md` (frontmatter)

- `status: prd_done` → `status: implemented`
- `artifacts.techspec: null` → `artifacts.techspec: .specs/prd-billing-pipeline/techspec.md`
- `artifacts.tasks: null` → `artifacts.tasks: .specs/prd-billing-pipeline/tasks.md`
- `next_skill: create-technical-specification` → `next_skill: null`

### 3. `docs/epics/epic-03-onboarding-magic-token.md` (frontmatter)

- `status: pending` → `status: implemented`
- `artifacts.prd: null` → `artifacts.prd: .specs/prd-onboarding-magic-token/prd.md`
- `artifacts.techspec: null` → `artifacts.techspec: .specs/prd-onboarding-magic-token/techspec.md`
- `artifacts.tasks: null` → `artifacts.tasks: .specs/prd-onboarding-magic-token/tasks.md`
- `next_skill: create-prd` → `next_skill: null`

### 4. `docs/epics/README.md` — tabela Roadmap

Atualizar três linhas para refletir o estado real (linha de E4 intocada):

```
| **E1** | `identity-foundation`     | Implementado | — (raiz)   | E2, E3 | `.specs/prd-identity-foundation/` (prd + techspec + tasks) |
| **E2** | `billing-pipeline`        | Implementado | E1         | E4     | `.specs/prd-billing-pipeline/` (prd + techspec + tasks)    |
| **E3** | `onboarding-magic-token`  | Implementado | E1         | E4     | `.specs/prd-onboarding-magic-token/` (prd + techspec + tasks) |
| **E4** | `reconciliation-hardening`| Backlog pós-MVP | E2 e E3 (parcial) | — | — |
```

### 5. Não tocar
- E4 (`epic-04-reconciliation-hardening.md`) — instrução de timing permanece válida.
- Conteúdo dos PRDs / techspecs / tasks de E1, E2, E3 — fora do escopo desta reconciliação.
- Código sob `internal/` — fora do escopo.

## Verificação

1. `head -20` em cada arquivo `docs/epics/epic-0{1,2,3}-*.md` para confirmar frontmatter atualizado.
2. `head -25 docs/epics/README.md` para confirmar tabela Roadmap.
3. Validação cruzada: cada caminho em `artifacts.*` deve existir no working tree (`ls .specs/prd-{identity-foundation,billing-pipeline,onboarding-magic-token}/{prd,techspec,tasks}.md`).
4. `git status -s` deve mostrar 4 arquivos modificados (3 épicos + README) somados aos arquivos pré-existentes (deleções e o novo prompt).
5. Diff sanity: `git diff docs/epics/` — só linhas de frontmatter + tabela Roadmap devem mudar.

## Não-objetivos auditáveis

- Não criar `.specs/prd-*/prd.md` novo.
- Não rodar skill `create-prd`, `create-technical-specification`, `create-tasks` ou `execute-all-tasks`.
- Não revisar/auditar o conteúdo dos PRDs existentes.
- Não alterar E4 nem reabrir a decisão de timing.
- Não criar commits sem pedido explícito do usuário.

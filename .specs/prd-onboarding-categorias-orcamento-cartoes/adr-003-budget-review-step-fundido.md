# Registro de Decisão Arquitetural (ADR-003)

## Metadados

- **Título:** Step único `budget_review` com submáquina fechada para revisão do resumo reabrindo a distribuição
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** Autor da feature, usuário (decisão D-09 e escolha explícita de estrutura)
- **Relacionados:** PRD (RF-16..RF-23), techspec.md, ADR-004, ADR-006, `.claude/rules/workflow-kernel.md`

## Contexto

O resumo do onboarding oferece "sim para confirmar ou não para revisar" (`onboarding_workflow.go:530`),
mas o step atual apenas re-suspende no "não" (`:830`), sem caminho real de edição. A decisão de
produto D-09 exige que "não" **reabra a distribuição**: coletar novos valores e voltar ao resumo até
a confirmação.

Restrição técnica inegociável: `Engine.Resume` retoma sempre no `snap.Cursor` do step suspenso
(`engine.go:263,306`); um step só re-suspende no mesmo índice ou completa e avança — **não pode voltar
a um step anterior** (`combinators.go:23-38`). Logo, com steps separados (`methodology` → `distribution`
→ `summary`), o step de resumo não consegue "voltar" ao step de distribuição.

Regra de plataforma: o kernel `internal/platform/workflow` é genérico e não deve ganhar recursos para
atender a um único consumidor (R-WF-KERNEL-001).

## Decisão

Fundir metodologia (sugestão + coleta de distribuição), criação do draft e resumo num **único step
durável `budget_review`** com uma submáquina interna governada por um enum fechado `reviewAwaitKind`
(`reviewAwaitDistribution`, `reviewAwaitConfirm`, `iota+1`, zero value inválido). O step:

1. Primeira entrada → `SuggestAllocation(default)` → `reviewAwaitDistribution` → suspende (sugestão).
2. Resume em `reviewAwaitDistribution` → extrai/`DecideAllocationsBP`; erro → re-suspende (reprompt);
   sucesso → helper `applyDraftBudget` (recria draft: `GetMonthlySummary`→`DeleteDraftBudget` se draft
   → `CreateBudget`) → `SuggestAllocation(atual)` → `reviewAwaitConfirm` → suspende (resumo).
3. Resume em `reviewAwaitConfirm` → "sim" completa (avança para ativação); "não"/ambíguo →
   `reviewAwaitDistribution` → suspende (nova distribuição). Loop até confirmação (D-09).

Escolha do usuário entre as alternativas: **fundir num step único** (em vez de sub-estado no step de
resumo com helper compartilhado, ou combinator de loop no kernel).

## Alternativas Consideradas

- **Sub-estado fechado no step de resumo + helper compartilhado (mantendo steps separados).**
  Vantagem: preserva granularidade de trace por step (distribuição e resumo distintos). Desvantagem:
  o loop de revisão fica no step de resumo, e a criação do draft passa a ser chamada de dois lugares.
  Não escolhida (preferência do usuário pelo step único).
- **Combinator de loop/goto genérico no kernel.** Vantagem: retorno a step anterior nativo.
  Desvantagem: aumenta o blast radius do kernel genérico para um único consumidor (R-WF-KERNEL-001).
  Rejeitada.

## Consequências

### Benefícios Esperados

- D-09 atendido com um único dono do ciclo distribuição→resumo→confirma/revisa.
- Cursor-friendly: todo o loop vive num só índice de cursor, sem tentativa de retorno.
- Kernel intacto (R-WF-KERNEL-001 preservada).

### Trade-offs e Custos

- Step maior, concentrando três responsabilidades de UX; menor granularidade de observabilidade por
  step (metodologia/distribuição/resumo colapsam em `step-budget-review`).
- `applyDraftBudget` recria o draft a cada revisão (delete+create).

### Riscos e Mitigações

- **Risco:** estado inconsistente entre sub-estados. **Mitigação:** `reviewAwaitKind` fechado com zero
  value inválido; transições explícitas; testes cobrindo primeira entrada, revisão e confirmação.
- **Risco:** recriação de draft falhar no meio. **Mitigação:** `failStep` tipado (sem falso sucesso);
  no resume, `applyDraftBudget` é idempotente (delete+create sobre a mesma competência).
- **Rollback:** reintroduzir steps separados sem o caminho de revisão (perde D-09).

## Plano de Implementação

1. Definir `reviewAwaitKind` + `String`/`IsValid`/`parseReviewAwaitKind`.
2. Implementar `BuildBudgetReviewStep` e o helper `applyDraftBudget`.
3. Remover `BuildMethodologyStep`/`BuildDistributionStep`/`BuildSummaryStep` isolados; migrar prompts.
4. Testes de loop: aceitar sugestão; enviar reais; "não" → novos valores → resumo → "sim".

Concluído quando: "não" no resumo coleta nova distribuição e reexibe o resumo, sem ativar parcial.

## Monitoramento e Validação

- `workflow_steps_total{step="step-budget-review",status}` e `workflow_suspend_total`.
- Teste de step do loop + gate real-LLM (allocation_input, summary_confirm ≥ 0,90).

## Impacto em Documentação e Operação

- Documentar o sub-estado `reviewAwaitKind` no comentário de PR/onboarding técnico (fora do código —
  R-ADAPTER-001.1 proíbe comentário em Go de produção).

## Revisão Futura

Revisar se o kernel evoluir para suportar retorno a step anterior de forma genérica e segura, o que
permitiria reseparar os steps preservando D-09.

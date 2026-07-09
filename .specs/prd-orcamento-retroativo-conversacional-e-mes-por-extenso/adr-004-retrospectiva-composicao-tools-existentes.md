# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Retrospectiva planejado vs realizado por composição das tools de leitura existentes, sem nova fonte de verdade
- **Data:** 2026-07-08
- **Status:** Aceita
- **Decisores:** Time de plataforma / agente financeiro
- **Relacionados:** PRD (RF-20..RF-24), techspec.md, design-patterns-mandatory (anti-over-engineering)

## Contexto

O propósito D3 é retrospectiva planejado vs realizado ("como foi meu mês de junho"). Investigação confirmou que `query_plan` (`query_plan.go`) já retorna, por categoria raiz, `PlannedCents`, `SpentCents` e `PercentageSpent`, além de `TotalSpentCents`/`TotalPlannedCents` — via `GetMonthlySummary` que agrega realizado por `SumByRoot` (`expense_repository.go:148`). Ou seja, **quando há orçamento**, `query_plan` já é o comparativo planejado vs realizado. `query_month` (`query_month.go`) retorna o realizado bruto (income/outcome + lançamentos), útil quando **não há orçamento**. As 5 categorias raiz são fixas (`root_slug.go`).

Decisões de produto: retrospectiva sem orçamento mas com lançamentos → oferecer criar E mostrar realizado (RF-23); com orçamento sem lançamentos → planejado vs realizado zerado 0% (RF-22).

## Decisão

Entregar a retrospectiva por **composição via instrução** das tools existentes, sem tool nova nem nova fonte de verdade:

- Mês com orçamento → `query_plan` (planejado+realizado+% por raiz); apresentar por categoria e total com mês por extenso (ADR-003). Sem lançamentos → naturalmente `SpentCents=0`/`0%` (RF-22).
- Mês sem orçamento com lançamentos → `query_month` (realizado) + oferta de criar via `create_budget` (RF-23).
- Mês sem orçamento sem lançamentos → oferta de criar (RF-24).

A competência da retrospectiva é resolvida por `DecideCompetence` (ADR-002). A confiabilidade da formatação do comparativo é best-effort do LLM, coberta pelo gate real-LLM ≥0.90.

## Alternativas Consideradas

- **Tool read-only dedicada `budget_retrospective`:** retornaria um objeto de comparação já montado (LLM só formata), aumentando determinismo/eval. Porém duplica o que `query_plan` já entrega e adiciona superfície (tool + schema + testes). Rejeitada (decisão do usuário; anti-over-engineering) — reavaliar se o gate não fechar por erro de composição.
- **Novo use case agregador no módulo budgets:** idem, duplicaria `GetMonthlySummary`. Rejeitada.

## Consequências

### Benefícios Esperados

- Zero tool nova, menor superfície, sem duplicação; reuso do que já existe.
- Retrospectiva imediata para qualquer mês.

### Trade-offs e Custos

- Formatação do comparativo depende do LLM (best-effort), não determinística na saída.

### Riscos e Mitigações

- **Composição inconsistente pelo LLM:** instrução por exemplo (lição de reviews sobre single-shot mascarar acurácia); gate ≥0.90; se falhar, promover para tool dedicada (fallback documentado nesta ADR).
- **`GetMonthlySummary` sem orçamento:** caminho sem orçamento usa `query_month` (não depende de budget row) + oferta de criar.

## Plano de Implementação

1. Instrução do agente: composição da retrospectiva (com/sem orçamento), mês por extenso.
2. Garantir que `query_plan`/`query_month` recebem competência resolvida (ADR-002).
3. E2E real-LLM cobrindo retrospectiva com/sem orçamento e sem lançamentos.

## Monitoramento e Validação

- Cenários E2E de retrospectiva no gate ≥0.90.
- Sinal de necessidade de tool dedicada: erros recorrentes de composição no harness.

## Impacto em Documentação e Operação

- Instrução do agente documenta a composição.

## Revisão Futura

- Promover a tool dedicada `budget_retrospective` se o gate não fechar por formatação; reavaliar em revisão anual.

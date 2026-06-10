# ADR-002 — Outbox interno unificado `budgets.expense.committed.v1`

## Metadados

- **Título:** Evento único por mutação financeira para avaliação assíncrona de alertas
- **Data:** 2026-06-09
- **Status:** Aceita
- **Decisores:** Time MeControla / AI Agent
- **Relacionados:** [PRD v24](./prd.md) (RT-24, RF-55–RF-64, RF-30), [techspec.md](./techspec.md), `internal/platform/outbox`

## Contexto

- RT-24 exige avaliação assíncrona de alertas via outbox interno; a requisição síncrona não pode aguardar.
- RT-21 exige propagar o instante de commit (`committed_at`) e a competência BR de cutoff calculada no instante do commit, não no instante da avaliação.
- O `outbox.Publisher` da plataforma (`internal/platform/outbox`) aceita um envelope por evento com `event_type`, `aggregate_type`, `aggregate_id`, `payload` JSON e `occurred_at`. Já é usado por `internal/billing` em produção, garantia at-least-once e idempotência por `event.ID`.
- O avaliador precisa, dado um commit financeiro, recalcular o gasto agregado por raiz e aplicar a transição de threshold. O `mutation_kind` (create/update/delete) é informativo, mas o resultado da avaliação depende apenas do estado atual (recálculo é o "valor verdadeiro" — RF-56a).

## Decisão

Toda mutação financeira persistida (`UpsertExpense` create/update, `DeleteExpense`) publica **um único** evento interno no outbox compartilhado, na mesma transação do commit financeiro:

```text
event_type:      "budgets.expense.committed.v1"
aggregate_type:  "budgets.expense"
aggregate_id:    <expense_id>
payload (JSON):
{
  "user_id":           "<uuid>",
  "competence":        "YYYY-MM",
  "subcategory_id":    "<uuid>",
  "root_slug":         "expense.<...>",
  "mutation_kind":     "create" | "update" | "delete",
  "committed_at":      "RFC3339 UTC",
  "cutoff_competence_br": "YYYY-MM"
}
occurred_at:    committed_at
```

O consumer registrado pelo `BudgetsModule` no `events.Dispatcher` é `expense_committed_consumer`, que delega a `EvaluateAlert`. Idempotência por `event.ID` (garantia padrão do outbox).

## Alternativas Consideradas

1. **Três `event_type` distintos (`created/updated/deleted`)**.
   - Vantagens: observabilidade discreta por verbo.
   - Desvantagens: três handlers quase idênticos, três paths a manter; o avaliador depende do estado atual (recálculo), não do verbo. Aumenta superfície de drift entre handlers.
   - Rejeitada por sobrecarga sem benefício para o MVP.

2. **Evento por raiz `budgets.root.spending.changed.v1` com delta**.
   - Vantagens: payload semanticamente "puro".
   - Desvantagens: exige computar o delta no caminho síncrono — adiciona latência, conflita com RT-24 de "não aguardar".
   - Rejeitada por penalizar o p95 do caminho de escrita (M-05).

## Consequências

### Benefícios Esperados

- Um único contrato de evento, um único consumer, fácil de versionar (`v1`).
- Payload propaga `committed_at` + `cutoff_competence_br` — garante RF-60c independente do atraso da avaliação.
- Adapter publisher fica fino (R-ADAPTER-001.2): serializa payload já decidido pelo use case.

### Trade-offs e Custos

- Avaliador sempre faz recálculo (SQL `SUM`), mesmo quando a mutação não move o ponteiro. Mitigado pelo índice composto parcial (ADR-004) e por cardinalidade pequena (≤ 100 despesas/mês/raiz).

### Riscos e Mitigações

- **Risco:** versionar futuro (`v2`) sem quebrar consumers existentes.
  - **Mitigação:** sufixo `.v1` no event_type; novos campos adicionados de forma aditiva; mudança breaking → novo event_type.
- **Risco:** event_id colidir entre publicações simultâneas.
  - **Mitigação:** uuid v4 gerado por `internal/platform/id` (já usado por billing).

## Plano de Implementação

1. Definir struct `ExpenseCommittedEnvelope` em `internal/budgets/application/interfaces/`.
2. Implementar `expense_committed_publisher.go` em `infrastructure/messaging/database/producers/` (adapter sobre `outbox.Publisher`).
3. Chamar publisher no final do `tx.Run` de `UpsertExpense` e `DeleteExpense`.
4. Implementar `expense_committed_consumer.go` em `infrastructure/messaging/database/consumers/`; registrar no `BudgetsModule.EventHandlers`.
5. Wire-up em `cmd/worker/worker.go` no `events.Dispatcher`.

## Monitoramento e Validação

- `budgets_expense_committed_published_total{mutation_kind}` no publisher.
- `budgets_expense_committed_consumed_total{outcome}` no consumer (outcome ∈ `evaluated|suppressed_stale|suppressed_retroactive|rate_limited`).
- `budgets_alert_evaluation_lag_seconds` (committed_at → evaluated_at).

## Impacto em Documentação e Operação

- Esquema do evento documentado em `internal/budgets/openapi.yaml` (seção `x-internal-events`) ou em README dedicado.
- Runbook: se lag > 5 min p95, investigar dispatcher de outbox.

## Revisão Futura

- Revisar quando a integração WhatsApp/LLM real (QA-05 / OUT-01) introduzir provider de envio, exigindo nova máquina `delivery_failed` (RF-64d).
- Revisar quando o volume de despesas saltar uma ordem de grandeza acima de RT-08.

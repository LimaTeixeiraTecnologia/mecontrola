# ADR-003 — UM ÚNICO evento por mutação de `CardPurchase`

## Metadados

- **Título:** Single `transactions.card_purchase.{action}.v1` carregando array de parcelas + `ref_months_affected`
- **Data:** 2026-06-12
- **Status:** Aceita
- **Decisores:** Engenharia
- **Relacionados:** PRD RF-36, RF-25; techspec "messaging/database/producers/card_purchase_event_publisher.go"; ADR-004; ADR-006 (DMMF — `ref_months_affected` é calculado no `Decide*` puro, não no producer).

## Contexto

Cada `CardPurchase` materializa até 24 `CardInvoiceItem` em N competências. O consumer de `MonthlySummary` precisa saber **todas** as competências afetadas para invalidar a projeção. Duas formas:

1. **N eventos** — um evento por `CardInvoiceItem` (`transactions.card_invoice_item.{action}.v1`).
2. **1 evento único** carregando array de parcelas + lista `ref_months_affected`.

## Decisão

Publicar **UM ÚNICO evento** por mutação do agregado `CardPurchase`:

- Tipo: `transactions.card_purchase.{created|updated|deleted}.v1`.
- Aggregate type: `transactions.card_purchase`.
- Aggregate ID: `card_purchase.id`.
- Payload:
  ```json
  {
    "user_id": "...",
    "card_id": "...",
    "purchase_id": "...",
    "total_amount_cents": 240000,
    "installments_total": 12,
    "category_id": "...",
    "subcategory_id": null,
    "description": "...",
    "purchased_at": "...",
    "items": [
      {"installment_index": 1, "invoice_id": "...", "ref_month": "2026-07", "amount_cents": 20000},
      ...
    ],
    "ref_months_affected": ["2026-07", "2026-08", "..."]
  }
  ```

`ref_months_affected` é a **união ordenada e deduplicada** das competências da versão anterior (em PATCH/DELETE) e da versão nova. Consumidores devem iterar essa lista para invalidar projeções; nunca devem inferir a partir do array `items` (que cobre apenas a versão atual).

> **Onde `ref_months_affected` é calculado** (ADR-006): no passo `Decide*` puro em `internal/transactions/domain/services/card_purchase_workflow.go` (métodos `DecideCreate`, `DecideUpdate`, `DecideDelete`). O use case orquestra; o producer (`Publish(ctx, db, evt entities.CardPurchaseCreated|Updated|Deleted)`) faz só `json.Marshal` + `outbox.NewPostgresPublisher.Publish`. É proibido — por R-ADAPTER-001.2 e ADR-006 — calcular `ref_months_affected` no use case (deve vir pronto do `Decide*`) ou no producer (cálculo de domínio em adapter outbound).

## Alternativas Consideradas

### A. N eventos (um por parcela)
- **Vantagens**: cada `CardInvoiceItem` é unidade de outbox; reprocesso parcial.
- **Desvantagens**:
  - Explosão de eventos: criar 24 parcelas = 24 INSERTs no `outbox` + 24 dispatches + 24 recomputes (mesmo com coalescing, é trabalho desnecessário).
  - PATCH cascateando precisa publicar 24 deleted + 24 created, ou um mix complexo; difícil garantir consistência sem evento de agregação.
  - Consumidores externos veriam parcelas isoladas sem o contexto da compra.
- **Motivo da rejeição**: custo operacional e cognitivo alto.

### B. 1 evento + N eventos de item
- **Vantagens**: granularidade para consumidores que quiserem só itens.
- **Desvantagens**: dobra eventos; risco de inconsistência entre o evento de agregado e os de item.
- **Motivo da rejeição**: nenhum consumidor atual demanda granularidade de item.

## Consequências

### Benefícios Esperados

- Único INSERT no `outbox` por mutação → throughput maior, lock menor na TX.
- Consumer de `MonthlySummary` recebe `ref_months_affected` explícito → invalidação correta sem heurística.
- Auditoria simples: 1 evento por mutação de agregado.

### Trade-offs e Custos

- Payload do evento cresce com `installments_total` (até 24 items). Tamanho típico ~3-4 KB; aceitável.
- Consumidor que só queira reagir a uma parcela específica precisa filtrar o array (não há split nativo).

### Riscos e Mitigações

- **Risco**: PATCH muda `installments_total` de 12 → 3 e o `ref_months_affected` esquece de incluir os 9 meses antigos.
  - **Mitigação**: invariante do usecase: `ref_months_affected = old.refMonths() ∪ new.refMonths()`; teste unitário obrigatório com 5 cenários de transição.
- **Risco**: consumidor que ignora `ref_months_affected` e itera `items` perde competências antigas.
  - **Mitigação**: documentação explícita no payload + nome do campo é evidente; consumer canônico (`MonthlySummaryRecomputeConsumer`) implementa o caminho correto.

## Plano de Implementação

1. Domain events tipados em `internal/transactions/domain/entities/events.go`: `CardPurchaseCreated`, `CardPurchaseUpdated`, `CardPurchaseDeleted`, todos com `RefMonthsAffected []valueobjects.RefMonth` (ADR-006 §3). `CardPurchaseUpdated` carrega adicionalmente `InvoiceDeltas map[RefMonth]int64` para o consumer aplicar `ApplyDelta` sem reler estado (audit fix #4).
2. `CardPurchaseWorkflow` em `internal/transactions/domain/services/card_purchase_workflow.go` com `DecideCreate/Update/Delete` puros, recebendo `commands.CreateCardPurchase`/`UpdateCardPurchase` (audit fix #1), `snapshot valueobjects.CardBillingSnapshot` (audit fix #2), calculando `RefMonthsAffected = old.RefMonths() ∪ new.RefMonths()` (deduped, ascending) + `InvoiceDeltas = sum(new[ref]) − sum(old[ref])` por `ref` afetado, retornando o domain event na `Decision`.
3. Use cases (`CreateCardPurchase`, `UpdateCardPurchase`, `DeleteCardPurchase`) consomem `Decide*` e passam `decision.Event` direto ao publisher; **nenhum cálculo de `ref_months_affected` no use case ou no producer**.
4. Publisher fino: `Publish(ctx, db, evt entities.CardPurchaseUpdated) error` → `json.Marshal(evt)` + `outbox.NewPostgresPublisher.Publish(envelope)`.
5. Unit tests do `Decide*` (sem mocks): criação (apenas new), update sem mudança de competência (new == old), update com mudança (union), delete (apenas old), troca de `installments_total` 12 → 3 (union de 12 meses).
6. Consumer (`MonthlySummaryRecomputeConsumer`) itera `RefMonthsAffected` em vez de `items`.

## Monitoramento e Validação

- Métrica `transactions_card_purchases_{created|updated|deleted}_total{installments_bucket}` confirma que cada mutação gera exatamente um evento.
- Integration test: PATCH de compra de 12x para 3x gera 1 evento; consumer recompute é chamado para 12 competências (3 novas + 9 antigas).

## Impacto em Documentação e Operação

- Schema do evento documentado em `internal/transactions/infrastructure/messaging/database/producers/card_purchase_event_publisher.go` (Go test as docs).
- Consumidores futuros recebem orientação: "use `ref_months_affected`; nunca infira de `items`".

## Revisão Futura

Revisitar se algum consumidor externo (não `budgets`) precisar de granularidade de parcela — então adicionar evento `card_invoice_item.*.v1` paralelo, mantendo este.

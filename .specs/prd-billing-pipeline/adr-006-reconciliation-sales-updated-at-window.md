# ADR-006 — Reconciliação horária via GET /v1/sales com janela updated_at_start_date e checkpoint

## Metadados

- **Título:** Reconciliação via GET /v1/sales (janela updated_at + checkpoint + overlap 15min)
- **Data:** 2026-06-05
- **Status:** Aceita
- **Decisores:** PO (jailton), arquitetura (AI)
- **Relacionados:** `.specs/prd-billing-pipeline/techspec.md` §7.4, RF-18, RF-19, [ADR-001](./adr-001-kiwify-public-api-vs-banking.md), [ADR-005](./adr-005-idempotency-and-ordering-state-machine.md)

## Contexto

PRD RF-18 exige reconciliação horária com a Kiwify, com **Kiwify como fonte de verdade** sempre. A Public API documentada **não expõe** `GET /v1/subscriptions` nem `GET /v1/subscriptions/{id}`. O único endpoint de leitura amplo é `GET /v1/sales` com query `start_date`/`end_date` (obrigatórios, máximo 90 dias), filtros opcionais por `status`, `payment_method`, `product_id`, paginação por `page_size`/`page_number`, e — crucialmente — filtros `updated_at_start_date` e `updated_at_end_date`.

A Kiwify rate-limita a 100 req/min. A reconciliação tem que ser eficiente, idempotente e tolerante a clock skew entre Kiwify e MeControla.

## Decisão

Job `ReconciliationJob` em `internal/billing/infrastructure/jobs/handlers/reconciliation_job.go`, registrado no `WorkerManager` com `Schedule() = "@every 1h"` (parametrizável via `KIWIFY_RECONCILIATION_INTERVAL`):

```
Run(ctx):
  checkpoint   = repo.GetCheckpoint("kiwify_sales")    // ex. 2026-06-05T14:00Z
  windowStart  = checkpoint.Add(-15 * time.Minute)     // overlap absorve clock skew
  windowEnd    = time.Now().UTC()
  for page := 1; ; page++ {
      sales, hasMore := kiwifyClient.ListSalesUpdatedSince(ctx, windowStart, windowEnd, page, pageSize)
      for _, s := range sales {
          if err := reconcileSale(ctx, s); err != nil { logError; continue }
      }
      if !hasMore { break }
  }
  repo.SetCheckpoint("kiwify_sales", windowEnd)
```

`reconcileSale(s)` traduz uma sale em um pseudo-evento e roteia para o mesmo use case do webhook (mesmo `event_key`). Detalhes:

- `s.status == "paid|approved"` → `ProcessSaleApproved` com `event_key = "order_approved:" + s.id`.
- `s.status == "refunded"` → `ProcessRefundOrChargeback` com `event_key = "refund:" + s.id`.
- `s.status == "chargedback"` → `ProcessRefundOrChargeback` com `event_key = "refund:" + s.id`.
- Outros status (`pending`, `processing`, etc.) → ignorados na reconciliação (MVP).

A checkpoint só avança em sucesso de toda a janela (com `errors.Join` agregando erros não-fatais; janela é reprocessada na próxima iteração se houver erro fatal).

Para subscription-level (`subscription_renewed/late/canceled`), o MVP não tem endpoint de consulta direto na Public API. Assumimos que essas transições chegam via webhook; reconciliação só corrige perda de webhook **da venda inicial e do refund/chargeback**. Hardening dessa lacuna fica em E4 quando (se) a Kiwify publicar `GET /v1/subscriptions`.

## Alternativas Consideradas

1. **Listar IDs locais ACTIVE/PAST_DUE e GET um a um na Kiwify.** Recusada — linear no tamanho da base; estoura rate limit (100 req/min) acima de ~6k assinaturas/hora.
2. **Diferir reconciliação para E4.** Recusada — RF-18 é explícito no PRD; deferir contradiz escopo aprovado.
3. **Sweep diário full retroativo (últimos 90d).** Já é explicitamente fora do MVP (RF-19, vai para E4).
4. **Webhook como única fonte (sem reconciliação).** Recusada — PRD exige sincronia horária (RF-18); sem reconciliação a perda de webhook vira dívida operacional.

## Consequências

### Benefícios Esperados

- Captura webhooks perdidos para vendas e refunds/chargebacks.
- Custo controlado: 100 req/min é compatível com rate limit; com page_size 100 e overlap 15min, capacidade de varrer ~6000 sales/hora (suficiente para qualquer base MVP).
- Reuso integral do caminho idempotente do webhook (sem código duplicado).

### Trade-offs e Custos

- **Lacuna:** subscription-level events (renewed/late/canceled) sem reconciliação no MVP. Mitigação: webhook é fonte primária; reconciliação cobre os eventos com maior impacto contábil (sale, refund, chargeback).
- Janela de 15min de overlap reprocessa eventos já vistos — absorvido pela idempotência (ADR-005), mas conta no custo de chamadas.
- Se a Kiwify retornar uma janela com >10k sales (caso patológico), o job pode levar > 1h; mitigação: o WorkerManager não dispara overlap (verificar) e o job tem timeout próprio via `KIWIFY_HTTP_TIMEOUT`.

### Riscos e Mitigações

- **R:** `updated_at_start_date` não filtra como documentado (drift na implementação Kiwify). **M:** Validar empiricamente em sandbox antes da execução; se falhar, fallback para `start_date`/`end_date` (janela criada-em).
- **R:** Checkpoint corrompido após restart deixa janela enorme. **M:** Default seguro de janela = `now - 1h` se checkpoint ausente; log + alerta em janela > 24h.
- **R:** Rate limit excedido. **M:** Client já tem `RateLimiter` local (`golang.org/x/time/rate`) configurado via `KIWIFY_RATE_LIMIT_MAX_REQUESTS_PER_MIN`; bloqueia antes de exceder.

## Plano de Implementação

1. Migration `0008_create_billing_reconciliation_checkpoints.up.sql`.
2. `internal/billing/infrastructure/http/client/kiwify/client.go` — método `ListSalesUpdatedSince(ctx, start, end, page, pageSize)`.
3. `internal/billing/infrastructure/jobs/handlers/reconciliation_job.go` — implementa `worker.Job`.
4. `internal/billing/application/usecases/reconcile_subscriptions.go` — orquestra checkpoint+iteração+dispatch.
5. Testes:
   - unit: client com mock httptest (paginação, rate limit, retry).
   - integ: `reconciliation_e2e_test.go` com stub Kiwify simulado.
6. Validação empírica em sandbox Kiwify pré-execução: `updated_at_start_date` realmente filtra.

## Monitoramento e Validação

- Métrica `billing_reconciliation_runs_total`.
- Métrica `billing_reconciliation_corrections_total{correction_type}` — distingue `webhook_replay` vs `state_drift`.
- Métrica `billing_reconciliation_duration_seconds` (histogram).
- Métrica `billing_reconciliation_window_seconds` (gauge — alerta se > 86400).
- Log: cada run inclui `window_start`, `window_end`, `sales_seen`, `corrections_applied`.

## Impacto em Documentação e Operação

- Runbook: como resetar checkpoint manualmente em incidente (SQL direta + restart do worker).
- Documento operacional Kiwify: confirmar que `updated_at_start_date` está documentado (e funciona).

## Revisão Futura

- Reabrir se a Kiwify publicar `GET /v1/subscriptions` (permite reconciliação subscription-level direta).
- Reabrir em E4 quando o sweep retroativo 90d full entrar (geralmente complementa esta reconciliação contínua).

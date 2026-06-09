# Bugfix E2 Billing-Pipeline — Plano de Execução

**Data:** 2026-06-09
**Spec:** `.specs/prd-billing-pipeline/`
**Origem:** review de 2026-06-09 (REJECTED, 10 achados)

## Escopo

10 achados a corrigir, agrupados por causa raiz:

| # | Sev | Causa raiz | Estratégia |
|---|-----|-----------|-----------|
| 1 | critical | Falta produtor de `expired_after_grace` + transição PAST_DUE→EXPIRED | Novo use case `ProcessSubscriptionGraceExpired`, job cron, método `PublishExpired`, query repo `ListExpiredGrace`, wiring em `module.go` + `worker.go` |
| 2 | high | `FindByOrderID` usa `order_id` que muda por cobrança Kiwify (renovação cria placeholder paralelo) | Novo `FindByKiwifySubID`; `process_subscription_renewed` busca por kiwify_sub_id; `extendExisting` exercitado em produção |
| 3 | high | Mesmo bug de #2 propagado a `subscription_late` e `subscription_canceled` | `process_subscription_late/canceled` migrados para `FindByKiwifySubID`; dispatcher envia `KiwifySubID` como chave |
| 4 | medium | `eventKey` em refund não inclui `Trigger`, colisão `order_refunded` vs `chargeback` | `eventKey = trigger:saleID` |
| 5 | medium | Migration 0004 com placeholders + fail-fast só em produção | Remover INSERT estático com placeholder; tornar `KIWIFY_PRODUCT_ID_*` obrigatório SEMPRE; bootstrap falha se planos não configurados |
| 6 | medium | `UpsertByOrder` não persiste customer fields | Estender SQL + assinatura do método para receber `mobile`, `email`, `sale_id` |
| 7 | medium | Loop `for page := 1; ; page++` sem teto | `maxPages = 1000` const com log + erro |
| 8 | medium | Drift documental: PRD RF-20 cita `PAST_DUE → EXPIRED` mas ADR-005 diz runtime | Atualizar PRD removendo notificação dessa transição (decisão #1 cobre) |
| 9 | low | `rotated` aceito sem métrica | Counter `billing_webhook_signature_rotated_total` |
| 10 | low | `_ = orderID; _ = userID; _ = kiwifySubID` blank assigns | Remover; propagar `userID`/`kiwifySubID` para entidade |

## Invariantes preservadas

- HMAC SHA-1 hex via query (ADR-002b)
- `DecideUserEntitlement` puro com `IsEntitled(sub, now)`
- Eventos outbox existentes imutáveis; `expired_after_grace` mantém schema atual
- Erros públicos preservados (`ErrEventAlreadyProcessed`, `ErrSubscriptionNotFound`, `ErrPlanNotFound`, `ErrIdempotencyConflict`, `ErrFunnelTokenMissing`)
- `order_approved` continua única entrada criadora via `extractFunnelToken`
- Ordem worker (identity ANTES outbox.DispatcherJob) preservada
- Sem `init()`, sem comentário em `.go`, sem `var _ Interface = (*T)(nil)`, sem abstração de tempo

## Ordem de execução

1. Plano salvo (este arquivo)
2. Implementação serial (subagents trariam conflito em `module.go` e `publisher`):
   - Bloco A: #4 #7 #9 #10 (fixes pequenos isolados)
   - Bloco B: #2 #3 + repo (continuidade kiwify_sub_id) — afeta renewed/late/canceled/dispatcher
   - Bloco C: #6 (UpsertByOrder customer fields) — afeta entrada de saleApproved
   - Bloco D: #1 (grace expiration) — adiciona método publisher, use case, job, wiring
   - Bloco E: #5 (migration + fail-fast) — afeta config e module bootstrap
   - Bloco F: #8 (PRD doc)
3. Testes de regressão por bug
4. `go vet ./...`, `go test ./...`, `golangci-lint run ./...`
5. Atualizar `.specs/prd-billing-pipeline/bugfix_report.md`

## Testes de regressão (mínimo)

- #1: `ProcessSubscriptionGraceExpiredSuite/TestPastDueComGracaVencidaTransitaParaExpired` + `ExpireGraceJobSuite/TestIdempotente`
- #2/#3: `ProcessSubscriptionRenewedSuite/TestExtendExistingViaKiwifySubID` + `ProcessSubscriptionLateSuite/TestKiwifySubIDLocaliza` + `ProcessSubscriptionCanceledSuite/TestKiwifySubIDLocaliza`
- #4: `ProcessKiwifyWebhookSuite/TestChargebackAposRefundReprocessa`
- #7: `ReconcileSubscriptionsSuite/TestMaxPagesGuard`
- #9: `HMACSignatureSuite/TestRotatedIncrementaMetrica` (ou validar via handler test)

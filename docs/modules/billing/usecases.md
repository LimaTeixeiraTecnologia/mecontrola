# Use Cases — internal/billing

## Tabela Mestre

| Use Case | Input DTO | Evento Publicado | Erros de Domínio Possíveis |
|---|---|---|---|
| ProcessSaleApproved | `ProcessSaleApprovedInput` | `PublishActivated` / `PublishActivatedWithoutToken` | `ErrEventAlreadyProcessed`, `ErrKiwifySubscriptionIDInvalid`, `ErrFunnelTokenMissing`, `ErrPlanNotFound` |
| ProcessSubscriptionRenewed | `ProcessSubscriptionRenewedInput` | `PublishRenewed` | `ErrEventAlreadyProcessed`, `ErrEventSuperseded`, `ErrKiwifySubscriptionIDInvalid`, `ErrRenewedWithoutBaseSubscription` |
| ProcessSubscriptionLate | `ProcessSubscriptionLateInput` | `PublishPastDue` | `ErrEventAlreadyProcessed`, `ErrEventSuperseded` |
| ProcessSubscriptionCanceled | `ProcessSubscriptionCanceledInput` | `PublishCanceled` | `ErrEventAlreadyProcessed`, `ErrEventSuperseded` |
| ProcessRefundOrChargeback | `ProcessRefundOrChargebackInput` | `PublishRefunded` | `ErrEventAlreadyProcessed` |
| ProcessSubscriptionGraceExpired | nenhum (job sem input) | `PublishExpired` | erros de infraestrutura agregados via `errors.Join` |
| ProcessKiwifyWebhook | `ProcessKiwifyWebhookInput` | delega ao use case correspondente | `ErrInvalidSignature`, `ErrInvalidWebhookPayload`, `ErrUnknownTrigger` |
| ReconcileSubscriptions | `ReconcileSubscriptionsInput` | delega a ProcessSaleApproved / ProcessRefundOrChargeback | `ErrReconcileMaxPagesExceeded`, erros de página agregados |
| RunReconciliation | nenhum (job sem input) | delega a ReconcileSubscriptions | `ErrCheckpointNotFound` (tratado: usa lookback padrão) |
| CleanupKiwifyEvents | nenhum (job sem input) | nenhum | erros de infraestrutura |
| SendSubscriptionNotification | `SendSubscriptionNotificationInput` | nenhum (chama `NotificationSender`) | erros de parsing de payload (não propagados, apenas logados) |

---

## ProcessSaleApproved

**Arquivo:** `internal/billing/application/usecases/process_sale_approved.go`
**Span:** `billing.usecase.process_sale_approved`

### Input

| Campo | Tipo | Obrigatório | Validação |
|---|---|---|---|
| `SaleID` | `string` | sim | não-vazio (`ErrSaleIDRequired`) |
| `KiwifyProductID` | `string` | sim | não-vazio (`ErrKiwifyProductIDRequired`) |
| `OrderID` | `string` | sim | não-vazio (`ErrOrderIDRequired`) |
| `KiwifySubID` | `string` | sim | não-vazio (`ErrKiwifySubIDRequired`); depois validado como VO via `NewKiwifySubscriptionID` |
| `FunnelToken` | `string` | não | vazio → path sem token |
| `CustomerMobileE164` | `string` | não | passado direto ao repositório |
| `CustomerEmail` | `string` | não | passado direto ao repositório |
| `OccurredAt` | `time.Time` | sim | não-zero (`ErrOccurredAtRequired`) |
| `EnvelopeID` | `string` | não | apenas informativo |

### Fluxo: Venda com Funnel Token

1. Abre span `billing.usecase.process_sale_approved`.
2. Chama `in.Validate()` — retorna todos os erros de validação concatenados via `errors.Join`.
3. Cria VO `KiwifySubscriptionID` via `valueobjects.NewKiwifySubscriptionID(in.KiwifySubID)`.
4. Detecta `FunnelToken != ""` → cria VO `FunnelToken` via `valueobjects.NewFunnelToken(in.FunnelToken)`.
5. Forma `eventKey = "order_approved:<SaleID>"`.
6. Abre transação via `uow.Do`:
   - Chama `processedRepo.MarkApplied(ctx, eventKey, "order_approved", in.SaleID, in.OccurredAt)` — se `ErrEventAlreadyProcessed` → retorna imediatamente (idempotência).
   - Chama `planRepo.FindByKiwifyProductID(ctx, in.KiwifyProductID)` — não encontrado → `ErrPlanNotFound`.
   - Cria `entities.NewSubscription(plan, funnelToken)` e chama `sub.Activate(in.OccurredAt)`.
   - Chama `subRepo.UpsertByOrder(ctx, UpsertByOrderParams{...})` com `PeriodStart = in.OccurredAt`.
   - Chama `subRepo.FindByOrderID(ctx, in.OrderID)` para obter o ID persistido.
   - Publica `publisher.PublishActivated(ctx, tx, persisted, ...)`.

### Fluxo: Venda sem Funnel Token

Idêntico ao path com token, com duas diferenças:

- Cria `entities.NewSubscription(plan, valueobjects.FunnelToken{})` (token vazio).
- Publica via `publisher.PublishActivatedWithoutToken(...)`.

### Idempotência

`eventKey = "order_approved:<SaleID>"` — chave única por venda. `MarkApplied` insere na tabela de eventos processados; duplicata retorna `ErrEventAlreadyProcessed`, propagado sem log e sem reprocessamento.

---

## ProcessSubscriptionRenewed

**Arquivo:** `internal/billing/application/usecases/process_subscription_renewed.go`
**Span:** `billing.usecase.process_subscription_renewed`

### Input

| Campo | Tipo | Obrigatório | Validação |
|---|---|---|---|
| `KiwifySubID` | `string` | sim | não-vazio (`ErrKiwifySubIDRequired`) |
| `OccurredAt` | `time.Time` | sim | não-zero (`ErrOccurredAtRequired`) |
| `OrderID` | `string` | não | fallback de resolução |
| `SaleID` | `string` | não | informativo |
| `KiwifyProductID` | `string` | não | informativo |

### Fluxo

1. Cria VO `KiwifySubscriptionID`.
2. Forma `eventKey = "subscription_renewed:<kiwifySubID>:<OccurredAt UTC RFC3339>"`.
3. Abre transação → `MarkApplied` → `FindByKiwifySubID` → `extendExisting`.

### Detecção de Regressão em `extendExisting`

1. `transitionSvc.DecideRenewal(existing.Status(), in.OccurredAt, existing.LastEventAt())`:
   - `DecisionSkipAsRegression` → `MarkSuperseded(eventKey)` + retorna `ErrEventSuperseded`.
2. Calcula `newPeriodEnd = existing.PeriodEnd() + existing.Plan().Duration()`.
3. `subRepo.ExtendPeriod(ctx, existing.ID(), newPeriodEnd, in.OccurredAt)`.
4. Publica `publisher.PublishRenewed(ctx, tx, renewed, renewed.ID(), previousPeriodEnd)`.

---

## ProcessSubscriptionLate

**Arquivo:** `internal/billing/application/usecases/process_subscription_late.go`
**Span:** `billing.usecase.process_subscription_late`

### Input

| Campo | Tipo | Obrigatório | Validação |
|---|---|---|---|
| `KiwifySubID` | `string` | sim | não-vazio |
| `OccurredAt` | `time.Time` | sim | não-zero |
| `OrderID` | `string` | não | fallback de resolução |

### Fluxo

1. Forma `eventKey = "subscription_late:<KiwifySubID>:<OccurredAt UTC RFC3339>"`.
2. Abre transação → `MarkApplied` → `resolveSubscription` (tenta `FindByKiwifySubID`; fallback `FindByOrderID`).
3. `transitionSvc.DecidePastDue(...)` — regressão → `MarkSuperseded` + `ErrEventSuperseded`.
4. Calcula `graceEnd = in.OccurredAt + valueobjects.DefaultGraceWindow.Duration()` (72h).
5. `subRepo.ApplyTransition(ctx, existing.ID(), StatusPastDue, graceEnd, in.OccurredAt)`.
6. Publica `publisher.PublishPastDue(ctx, tx, updatedSub, updatedSub.ID())`.

---

## ProcessSubscriptionCanceled

**Arquivo:** `internal/billing/application/usecases/process_subscription_canceled.go`
**Span:** `billing.usecase.process_subscription_canceled`

### Input

| Campo | Tipo | Obrigatório | Validação |
|---|---|---|---|
| `KiwifySubID` | `string` | sim | não-vazio |
| `OccurredAt` | `time.Time` | sim | não-zero |
| `OrderID` | `string` | não | fallback de resolução |

### Fluxo

1. Forma `eventKey = "subscription_canceled:<KiwifySubID>"` (sem timestamp — idempotente por assinatura).
2. Abre transação → `MarkApplied` → `resolveSubscription`.
3. `transitionSvc.DecideCancellation(...)` — regressão → `MarkSuperseded` + `ErrEventSuperseded`.
4. `subRepo.ApplyTransition(ctx, existing.ID(), StatusCanceledPending, time.Time{}, in.OccurredAt)` (sem `graceEnd`).
5. Publica `publisher.PublishCanceled(ctx, tx, updatedSub, updatedSub.ID())`.

---

## ProcessRefundOrChargeback

**Arquivo:** `internal/billing/application/usecases/process_refund_or_chargeback.go`
**Span:** `billing.usecase.process_refund_or_chargeback`

### Input

| Campo | Tipo | Obrigatório | Validação |
|---|---|---|---|
| `SaleID` | `string` | sim | não-vazio |
| `OrderID` | `string` | sim | não-vazio |
| `OccurredAt` | `time.Time` | sim | não-zero |
| `Trigger` | `string` | não | vazio → `"order_refunded"` como default |

### Paths: Refund vs Chargeback

| `Trigger` recebido | `eventKey` formado | Status final |
|---|---|---|
| `"order_refunded"` (ou vazio) | `order_refunded:<SaleID>` | `StatusRefunded` |
| `"chargeback"` | `chargeback:<SaleID>` | `StatusRefunded` |

O status de domínio resultante é sempre `StatusRefunded`. A diferença é preservada apenas no `eventKey` e no `trigger` em `processed_events`.

### Fluxo

1. Abre transação → `MarkApplied` → `FindByOrderID` (sem fallback por `KiwifySubID`).
2. `subRepo.ApplyTransition(ctx, existing.ID(), StatusRefunded, time.Time{}, in.OccurredAt)`.
3. Publica `publisher.PublishRefunded(ctx, tx, updatedSub, updatedSub.ID())`.

---

## ProcessSubscriptionGraceExpired

**Arquivo:** `internal/billing/application/usecases/process_subscription_grace_expired.go`
**Span:** `billing.usecase.process_subscription_grace_expired`
**Métrica:** `billing_subscription_grace_expired_total`

### Candidatos

`subRepo.ListPastDueGraceExpired(ctx, time.Now().UTC(), batchLimit)` retorna assinaturas com:
- Status `PAST_DUE`
- `grace_end <= now`

Limite padrão de batch: **100** (`graceExpiredBatchDefault`).

### Fluxo de Expiração em Lote

1. Consulta candidatos via `ListPastDueGraceExpired` (sem transação — leitura direta com `uc.db`).
2. Para cada candidato, abre transação:
   - `subRepo.ApplyTransition(ctx, cand.SubscriptionID, StatusExpired, time.Time{}, occurredAt)`.
   - Publica `publisher.PublishExpired(ctx, tx, expired, cand.SubscriptionID, cand.GraceEnd)`.
3. Falha em um item → loga e acumula em `errs` (continua processando os demais).
4. Agrega todos os erros via `errors.Join(errs...)`.

---

## ProcessKiwifyWebhook

**Arquivo:** `internal/billing/application/usecases/process_kiwify_webhook.go`
**Span:** `billing.usecase.process_kiwify_webhook`

### Input

| Campo | Tipo | Obrigatório | Validação |
|---|---|---|---|
| `RawBody` | `[]byte` | sim | não-vazio |
| `SignatureStatus` | `string` | sim | não-vazio; valores: `"valid"`, `"rotated"`, `"invalid"` |

### Tabela de Dispatch

| Trigger Kiwify | Use Case Chamado | Comportamento |
|---|---|---|
| `order_approved` | `ProcessSaleApproved` | ativo — cria/ativa assinatura |
| `subscription_renewed` | `ProcessSubscriptionRenewed` | ativo — estende período |
| `subscription_late` | `ProcessSubscriptionLate` | ativo — transiciona para PAST_DUE |
| `subscription_canceled` | `ProcessSubscriptionCanceled` | ativo — transiciona para CANCELED_PENDING |
| `order_refunded` | `ProcessRefundOrChargeback` | ativo — transiciona para REFUNDED |
| `chargeback` | `ProcessRefundOrChargeback` | ativo — transiciona para REFUNDED |
| `billet_created` | `noopTrigger` | noop — retorna `nil` sem ação |
| `pix_created` | `noopTrigger` | noop — retorna `nil` sem ação |
| `order_rejected` | `noopTrigger` | noop — retorna `nil` sem ação |
| `abandoned_cart` | `noopTrigger` | noop — retorna `nil` sem ação |
| qualquer outro | ausente no mapa | retorna `ErrUnknownTrigger` |

### Fluxo de Audit Trail

1. `in.Validate()`.
2. `kiwifypayload.Decode(in.RawBody)` — falha → `ErrInvalidWebhookPayload`.
3. `kiwifypayload.Classify(payload)` → determina `trigger`.
4. `auditEnvelope(...)` → `eventRepo.Persist(...)` (best-effort: falha apenas loga).
5. Incrementa `billing_webhooks_received_total{signature_status}`.
6. Se `signatureStatus == "invalid"` → retorna `ErrInvalidSignature`.
7. Resolve handler no mapa `uc.handlers[trigger]` — ausente → `ErrUnknownTrigger`.
8. Chama `handler(ctx, payload)`.

### Extração do Funnel Token

`kiwifypayload.ExtractFunnel` aplica prioridade decrescente:

| Prioridade | Campo JSON | Carrier |
|---|---|---|
| 1 (maior) | `TrackingParameters.sck` | `"sck"` |
| 2 | `TrackingParameters.s1` | `"s1"` (legacy) |
| 3 | `TrackingParameters.src` | `"src"` (legacy) |
| 4 (sem token) | nenhum | `"none"` |

### Conversão de Timestamps do Payload

| Evento | Prioridade de `OccurredAt` |
|---|---|
| `order_approved` | `approved_date` → `updated_at` → `Subscription.start_date` |
| `subscription_renewed` | `updated_at` → `Subscription.next_payment` |
| `subscription_canceled` | `updated_at` → `Subscription.start_date` |
| `order_refunded` / `chargeback` | `refunded_at` → `updated_at` |

---

## ReconcileSubscriptions

**Arquivo:** `internal/billing/application/usecases/reconcile_subscriptions.go`
**Span:** `billing.usecase.reconcile_subscriptions`
**Métrica:** `billing_reconciliation_corrections_total{correction_type}`

### Input

| Campo | Tipo | Obrigatório | Validação |
|---|---|---|---|
| `WindowStart` | `time.Time` | sim | não-zero |
| `WindowEnd` | `time.Time` | sim | não-zero; deve ser posterior a `WindowStart` |

### Algoritmo

1. Loop `page = 1..reconcileMaxPages (1000)`:
   - `kiwifyClient.ListSalesUpdatedSince(ctx, in.WindowStart, in.WindowEnd, page)`.
   - Para cada `sale`: chama `reconcileSale(ctx, sale)`.
   - `ErrEventAlreadyProcessed` ou `ErrEventSuperseded` → `continue` (silencioso).
   - `!salesPage.HasMore` → break.
   - `page == reconcileMaxPages` → retorna `ErrReconcileMaxPagesExceeded`.
2. Agrega erros via `errors.Join`.
3. `checkpointRepo.Set(ctx, "kiwify_sales", in.WindowEnd)` — atualiza watermark ao final.

### Correções Aplicadas

| Status Kiwify | Ação | Use Case Chamado |
|---|---|---|
| `"refunded"` | refund | `ProcessRefundOrChargeback` com `Trigger = "order_refunded"` |
| `"chargedback"` | refund | `ProcessRefundOrChargeback` com `Trigger = "chargeback"` |
| `"paid"` | aprovação | `ProcessSaleApproved` |
| `"approved"` | aprovação | `ProcessSaleApproved` |
| qualquer outro | ignorado | nenhuma ação |

---

## RunReconciliation

**Arquivo:** `internal/billing/application/usecases/run_reconciliation.go`
**Span:** `billing.usecase.run_reconciliation`

| Constante | Valor |
|---|---|
| `reconciliationCheckpointName` | `"kiwify_sales"` |
| `reconciliationWindowOverlap` | `15 * time.Minute` |
| `reconciliationDefaultLookback` | `1 * time.Hour` |

1. Lê checkpoint `"kiwify_sales"` — `ErrCheckpointNotFound` → usa `now - 1h` e loga warn.
2. `windowStart = checkpoint - 15min` (overlap para cobrir eventos tardios).
3. `windowEnd = time.Now().UTC()`.
4. Chama `ReconcileSubscriptions.Execute(ctx, ...)`.

---

## CleanupKiwifyEvents

**Arquivo:** `internal/billing/application/usecases/cleanup_kiwify_events.go`
**Span:** `billing.usecase.cleanup_kiwify_events`

| Parâmetro | Config | Default |
|---|---|---|
| Retenção | `KiwifyEventsRetentionDays` | 90 dias |
| Batch | `KiwifyEventsHousekeepingBatch` | 500 registros |

Remove registros de `billing_kiwify_events` em lotes até zerar os elegíveis. Loop verifica `ctx.Done()` antes de cada iteração.

---

## SendSubscriptionNotification

**Arquivo:** `internal/billing/application/usecases/send_subscription_notification.go`
**Span:** `billing.usecase.send_subscription_notification`
**Métrica:** `billing_notification_failures_total{trigger}`

### Input

| Campo | Tipo | Obrigatório |
|---|---|---|
| `EventType` | `string` | sim |
| `Payload` | `json.RawMessage` | sim |

Falhas de parse e falhas do `NotificationSender` são absorvidas — logadas e incrementadas na métrica, mas retornam `nil` para não causar retry no outbox.

---

## Mecanismo de Idempotência

### Formação do event_key

| Use Case | Fórmula do `event_key` | Granularidade |
|---|---|---|
| ProcessSaleApproved | `order_approved:<SaleID>` | por venda |
| ProcessSubscriptionRenewed | `subscription_renewed:<kiwifySubID>:<OccurredAt UTC RFC3339>` | por assinatura + timestamp |
| ProcessSubscriptionLate | `subscription_late:<KiwifySubID>:<OccurredAt UTC RFC3339>` | por assinatura + timestamp |
| ProcessSubscriptionCanceled | `subscription_canceled:<KiwifySubID>` | por assinatura (único) |
| ProcessRefundOrChargeback | `<trigger>:<SaleID>` | por trigger + venda |

### MarkApplied vs MarkSuperseded

```
ProcessedEventRepository
├── MarkApplied(ctx, eventKey, trigger, recursoID, occurredAt) error
│     Semântica: INSERT idempotente de novo registro.
│     Se a chave já existe → retorna interfaces.ErrEventAlreadyProcessed.
│
└── MarkSuperseded(ctx, eventKey) error
      Semântica: atualiza registro existente marcando-o como supersedido.
      Chamado quando TransitionService retorna DecisionSkipAsRegression.
```

### Diagrama de Decisão

```
Webhook / Job recebe evento
        │
        ▼
MarkApplied(eventKey, ...)
        │
   ┌────┴───────────────────────────────┐
   │ ErrEventAlreadyProcessed            │ → retorna ErrEventAlreadyProcessed (noop)
   └─────────────────────────────────────┘
        │ (sucesso: evento novo)
        ▼
TransitionService.Decide*(status, occurredAt, lastEventAt)
        │
   ┌────┴───────────────────────────────────┐
   │ DecisionSkipAsRegression               │
   │   → MarkSuperseded(eventKey)           │
   │   → retorna ErrEventSuperseded         │
   └────────────────────────────────────────┘
        │ (sucesso: transição válida)
        ▼
ApplyTransition / ExtendPeriod / UpsertByOrder
        │
        ▼
publisher.Publish*(ctx, tx, ...)  ← publicação transacional
        │
        ▼
commit uow.Do → fim
```

---

## Referências

- [domain.md](domain.md)
- [entry-points.md](entry-points.md)
- [events.md](events.md)

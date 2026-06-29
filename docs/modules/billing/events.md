# Eventos — internal/billing

> Todos os eventos são publicados via padrão Outbox (tabela `outbox_events` no mesmo schema),
> garantindo entrega atômica com a mutação da assinatura.

## Tabela Mestre de Eventos

| Event Type | Trigger de Negócio | Publicado por Use Case | Consumidores |
|---|---|---|---|
| `billing.subscription.activated` | Webhook Kiwify `order_approved` com `funnel_token` | `ProcessSaleApproved` | identity.SubscriptionEventProjector |
| `billing.subscription.activated_without_token` | Webhook Kiwify `order_approved` sem `funnel_token` | `ProcessSaleApproved` | nenhum (dead-letter imediato) |
| `billing.subscription.renewed` | Webhook Kiwify `subscription_renewed` | `ProcessSubscriptionRenewed` | identity.SubscriptionEventProjector |
| `billing.subscription.past_due` | Webhook Kiwify `subscription_late` | `ProcessSubscriptionLate` | identity.SubscriptionEventProjector, billing.NotificationHandler |
| `billing.subscription.canceled` | Webhook Kiwify `subscription_canceled` | `ProcessSubscriptionCanceled` | identity.SubscriptionEventProjector |
| `billing.subscription.refunded` | Webhook Kiwify `order_refunded` ou `chargeback` | `ProcessRefundOrChargeback` | identity.SubscriptionEventProjector, billing.NotificationHandler |
| `billing.subscription.expired_after_grace` | Job `GraceExpirationJob` (`@every 30m`) | `ProcessSubscriptionGraceExpired` | billing.NotificationHandler |

---

## Contrato de Publicação (Outbox)

### Garantias

A publicação ocorre **dentro da mesma transação de banco de dados** que muta `billing_subscriptions`. O `SubscriptionEventPublisher` recebe `database.DBTX` aberta pelo use case e insere na `outbox_events` usando essa transação. Se a transação fizer rollback, o evento é descartado junto.

O `DispatcherJob` (padrão `500ms` de tick) lê eventos pendentes com `SELECT FOR UPDATE SKIP LOCKED`, entrega a cada handler registrado e marca como publicado. Em caso de falha do handler, aplica backoff exponencial com jitter de ±20% até esgotar `OUTBOX_RETRY_MAX_ATTEMPTS` (padrão 3), após o que o evento vai para dead-letter (`status = failed`, métrica `outbox_dead_letter_total{event_type}`).

### Envelope (metadados por evento)

| Campo | Tipo | Semântica |
|---|---|---|
| `ID` | `string` (UUID) | Identificador único do evento; gerado por `id.NewUUIDGenerator()` |
| `EventType` | `string` | Constante de tipo de evento |
| `AggregateUserID` | `string` (omitempty) | UUID do usuário proprietário; pode ser vazio (ver nota) |
| `OccurredAt` | `time.Time` | Timestamp de negócio (`sub.LastEventAt()`) |
| `Payload` | `json.RawMessage` | Payload tipado do evento |

Campos internos do outbox (não expostos no `Envelope`):

| Campo | Valor para billing | Finalidade |
|---|---|---|
| `aggregate_type` | `"Subscription"` | Tipo do agregado raiz |
| `aggregate_id` | `subscriptionID` (UUID interno) | Identidade do agregado |
| `status` | `pending → processing → published/failed` | Ciclo de vida do relay |

**Nota sobre `aggregate_user_id`:** O evento `billing.subscription.activated` consta em `noUserEventAllowlist`, suprimindo o aviso de log quando `aggregate_user_id` está vazio — no momento da ativação o usuário pode ainda não ter sido vinculado à assinatura.

### Idempotência no consumidor

**Nível 1 — use case (tabela `billing_processed_events`, antes da publicação):** Cada use case insere em `billing_processed_events` com `event_key` derivada do evento Kiwify. Violação de unique constraint retorna `ErrEventAlreadyProcessed` e aborta a transação sem publicar evento.

**Nível 2 — outbox storage:** Insert em `outbox_events` usa `ON CONFLICT (id) DO NOTHING`.

**Nível 3 — consumidor identity:** `EntitlementRepository.Upsert` usa `ON CONFLICT (user_id) DO UPDATE`. Operação intrinsecamente idempotente.

---

## Eventos Publicados

### billing.subscription.activated

#### Quando

Webhook Kiwify `order_approved` com `funnel_token` presente e válido no payload.
Arquivo: `internal/billing/application/usecases/process_sale_approved.go`

#### Payload

| Campo | Tipo Go | Semântica |
|---|---|---|
| `subscription_id` | `string` (UUID) | UUID interno da assinatura |
| `funnel_token` | `string` | Token de rastreamento de funil Kiwify (`sck`, `s1` ou `src`) |
| `plan_code` | `string` | Código do plano: `MONTHLY`, `QUARTERLY` ou `ANNUAL` |
| `external_sale_id` | `string` | `order_id` do webhook Kiwify |
| `customer_mobile_e164` | `string` | Telefone do cliente em formato E.164 |
| `customer_email` | `string` | E-mail do cliente |
| `period_start` | `time.Time` (UTC) | Data de ativação; igual ao `occurred_at` do webhook |
| `period_end` | `time.Time` (UTC) | `period_start + plan_duration` |
| `paid_at` | `time.Time` (UTC) | Timestamp do pagamento confirmado |
| `occurred_at` | `time.Time` (UTC) | Igual a `paid_at`; vem de `sub.LastEventAt()` |

#### Diferença para activated_without_token

`activated_without_token` é emitido quando `funnel_token` está ausente. Os campos `funnel_token`, `period_start` e `period_end` não existem nesse payload.

---

### billing.subscription.activated_without_token

#### Quando

Mesmo fluxo de `activated`, porém com `funnel_token` vazio.

Este evento **não possui consumidor registrado em nenhum módulo**. O `DispatcherJob` marca imediatamente o registro como `status = failed` em `outbox_events`. A métrica `outbox_dead_letter_total{event_type="billing.subscription.activated_without_token"}` é incrementada.

#### Payload

| Campo | Tipo Go | Semântica |
|---|---|---|
| `subscription_id` | `string` (UUID) | UUID interno da assinatura |
| `plan_code` | `string` | Código do plano |
| `external_sale_id` | `string` | `order_id` Kiwify |
| `customer_mobile_e164` | `string` | Telefone E.164 |
| `customer_email` | `string` | E-mail |
| `paid_at` | `time.Time` (UTC) | Timestamp do pagamento |
| `occurred_at` | `time.Time` (UTC) | Igual a `paid_at` |

---

### billing.subscription.renewed

#### Quando

Webhook Kiwify `subscription_renewed` aceito (não regressivo) pelo `DecideRenewal`.
Arquivo: `internal/billing/application/usecases/process_subscription_renewed.go`

#### Payload

| Campo | Tipo Go | Semântica |
|---|---|---|
| `subscription_id` | `string` (UUID) | UUID interno da assinatura |
| `plan_code` | `string` | Código do plano renovado |
| `previous_period_end` | `time.Time` (UTC) | Data de expiração antes da renovação |
| `period_end` | `time.Time` (UTC) | Nova data de expiração (`previous_period_end + plan_duration`) |
| `occurred_at` | `time.Time` (UTC) | Timestamp do evento `subscription_renewed` |

---

### billing.subscription.past_due

#### Quando

Webhook Kiwify `subscription_late` aceito (não regressivo) pelo `DecidePastDue`.
Arquivo: `internal/billing/application/usecases/process_subscription_late.go`

#### Payload

| Campo | Tipo Go | Semântica |
|---|---|---|
| `subscription_id` | `string` (UUID) | UUID interno da assinatura |
| `period_end` | `time.Time` (UTC) | Data de vencimento original da assinatura |
| `grace_end` | `time.Time` (UTC) | Prazo até a expiração definitiva: `occurred_at + 72h` (`DefaultGraceWindow`) |
| `occurred_at` | `time.Time` (UTC) | Timestamp do evento `subscription_late` |

Enquanto `now() < grace_end`, o acesso do usuário permanece ativo para fins de entitlement.

---

### billing.subscription.canceled

#### Quando

Webhook Kiwify `subscription_canceled` aceito (não regressivo).
O status resultante em `billing_subscriptions` é `CANCELED_PENDING` (não `CANCELED`): a assinatura permanece acessível até `period_end`.

#### Payload

| Campo | Tipo Go | Semântica |
|---|---|---|
| `subscription_id` | `string` (UUID) | UUID interno da assinatura |
| `period_end` | `time.Time` (UTC) | Data até a qual o acesso permanece válido |
| `occurred_at` | `time.Time` (UTC) | Timestamp do cancelamento |

---

### billing.subscription.refunded

#### Quando

Webhook Kiwify com trigger `order_refunded` **ou** `chargeback`. Ambos os triggers produzem o mesmo evento de domínio. O status resultante é `REFUNDED` (terminal).

#### Payload

| Campo | Tipo Go | Semântica |
|---|---|---|
| `subscription_id` | `string` (UUID) | UUID interno da assinatura |
| `occurred_at` | `time.Time` (UTC) | Timestamp do reembolso ou chargeback |

---

### billing.subscription.expired_after_grace

#### Quando (job vs webhook)

Emitido exclusivamente pelo job `GraceExpirationJob` (`@every 30m`). O job consulta `billing_subscriptions` com `status = PAST_DUE AND grace_end < now()` em batches de até 100 registros. Não existe webhook Kiwify correspondente.

**Atenção:** `period_end` é sempre zero neste payload. O query `ListPastDueGraceExpired` retorna apenas `id, user_id, grace_end, last_event_at`; o campo `periodEnd` é `time.Time{}` na entidade hidratada passada ao publisher.

#### Payload

| Campo | Tipo Go | Semântica |
|---|---|---|
| `subscription_id` | `string` (UUID) | UUID interno da assinatura |
| `period_end` | `time.Time` (UTC) | **Sempre zero** — `ListPastDueGraceExpired` não transporta `period_end` |
| `grace_end` | `time.Time` (UTC) | Lida de `billing_subscriptions.grace_end` |
| `occurred_at` | `time.Time` (UTC) | `time.Now().UTC()` no momento da expiração pelo job |

---

## Consumidores

### internal/identity — SubscriptionEventProjector

Arquivo do consumer: `internal/identity/infrastructure/messaging/database/consumers/subscription_event_projector.go`
Use case acionado: `internal/identity/application/usecases/project_subscription_event.go`

| Evento Consumido | Ação | Tabela Alvo |
|---|---|---|
| `billing.subscription.activated` | Se `user_id IS NOT NULL`: upsert de entitlement; se `user_id IS NULL`: upsert pendente | `identity_entitlements` ou `identity_entitlements_pending` |
| `billing.subscription.renewed` | Projeta novo `status=ACTIVE` e `period_end` | `identity_entitlements` |
| `billing.subscription.past_due` | Projeta `status=PAST_DUE` e `grace_end` | `identity_entitlements` |
| `billing.subscription.canceled` | Projeta `status=CANCELED_PENDING` | `identity_entitlements` |
| `billing.subscription.refunded` | Projeta `status=REFUNDED` | `identity_entitlements` |

**Fluxo interno de `ProjectSubscriptionEvent.Execute`:**

1. Extrai `subscription_id` do payload.
2. Chama `SubscriptionProjectionReader.FindCurrentBySubscriptionID` → `SELECT funnel_token, user_id, status, period_end, grace_end FROM billing_subscriptions WHERE id = $1`.
3. `projection.UserID == ""` → `PendingEntitlement` → `UpsertPending` em `identity_entitlements_pending` com `ON CONFLICT (subscription_id) DO UPDATE`.
4. `projection.UserID != ""` → `CommittedEntitlement` → `Upsert` em `identity_entitlements` com `ON CONFLICT (user_id) DO UPDATE`.

O evento `billing.subscription.expired_after_grace` **não é consumido** pelo identity. O entitlement de um usuário expirado é inferido pelos campos `period_end` / `grace_end` no momento da verificação de acesso.

### internal/billing — NotificationHandler (interno)

Arquivo do consumer: `internal/billing/infrastructure/messaging/database/consumers/notification_handler.go`
Use case acionado: `internal/billing/application/usecases/send_subscription_notification.go`

| Evento Consumido | Ação |
|---|---|
| `billing.subscription.past_due` | Chama `NotificationSender.NotifyTransition({SubscriptionID, EventType: "billing.subscription.past_due"})` |
| `billing.subscription.refunded` | Idem com `EventType: "billing.subscription.refunded"` |
| `billing.subscription.expired_after_grace` | Idem com `EventType: "billing.subscription.expired_after_grace"` |

**Implementação atual:** o `BillingModule` injeta um `noopNotificationSender` (sem envio real). A interface `NotificationSender.NotifyTransition` está disponível para implementação futura (WhatsApp, e-mail, push).

---

## Diagrama de Fluxo

```
Webhook Kiwify                         Job GraceExpirationJob
     │                                        │  (@every 30m)
     ▼                                        ▼
ProcessKiwifyWebhook              ProcessSubscriptionGraceExpired
     │                                        │
     ├─► billing_processed_events INSERT      │
     │   (event_key; ON CONFLICT → abort)     │
     │                                        │
     └──────────────┬─────────────────────────┘
                    │  (mesma transação UoW)
                    ├─► billing_subscriptions (upsert / apply_transition)
                    └─► outbox_events INSERT
                        (ON CONFLICT id DO NOTHING)

                              │
                    DispatcherJob (@every tick)
                    SELECT FOR UPDATE SKIP LOCKED
                              │
              ┌───────────────┴──────────────────────────────────┐
              │                                                  │
              ▼                                                  ▼
  identity/SubscriptionEventProjector            billing/NotificationHandler
  activated, renewed, past_due,                  past_due, refunded,
  canceled, refunded                             expired_after_grace
              │                                                  │
              ▼                                                  ▼
  ProjectSubscriptionEvent.Execute        SendSubscriptionNotification.Execute
              │                                                  │
       ┌──────┴──────┐                               NotificationSender.NotifyTransition
       ▼             ▼                               (noopNotificationSender no MVP)
identity_        identity_
entitlements     entitlements_pending
(user_id         (user_id IS NULL;
 IS NOT NULL;     aguarda vinculação)
 ON CONFLICT
 user_id DO UPDATE)
```

---

## Referências

- [usecases.md](usecases.md)
- [schema.md](schema.md)
- [entry-points.md](entry-points.md)
- `internal/billing/infrastructure/messaging/database/producers/events.go`
- `internal/billing/infrastructure/messaging/database/producers/subscription_event_publisher.go`
- `internal/billing/infrastructure/messaging/database/consumers/notification_handler.go`
- `internal/identity/infrastructure/messaging/database/consumers/subscription_event_projector.go`
- `internal/identity/application/usecases/project_subscription_event.go`

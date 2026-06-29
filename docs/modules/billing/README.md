# internal/billing

Módulo responsável pelo ciclo de vida completo de assinaturas pagas no mecontrola: recebimento de webhooks Kiwify, ativação, renovação, inadimplência, cancelamento, estorno e expiração após janela de carência. Toda mutação de estado é publicada via Outbox transacional para que o módulo `internal/identity` projete entitlements de acesso.

---

## Diagrama C4 — Nível Módulo

```
┌─────────────────────────────────────────────────────────────────────┐
│                        internal/billing                             │
│                                                                     │
│  ┌───────────────┐   webhook    ┌──────────────────────────────┐    │
│  │    Kiwify     │─────POST────▶│ POST /api/v1/billing/        │    │
│  │  (externo)    │              │      webhooks/kiwify         │    │
│  │               │◀──REST API──│ KiwifyClient (reconciliação) │    │
│  └───────────────┘              └──────────────┬───────────────┘    │
│                                                │                    │
│  ┌───────────────┐    eventos outbox           │                    │
│  │internal/      │◀────────────────────────────┤                    │
│  │identity       │  (SubscriptionEventProjector│                    │
│  │SubscriptionEvent│  via DispatcherJob)        │                    │
│  │Projector      │                             │                    │
│  └───────────────┘                             │                    │
│                                                ▼                    │
│  ┌───────────────┐              ┌──────────────────────────────┐    │
│  │  Platform     │◀────────────▶│   Application (use cases)    │    │
│  │  Outbox       │  UoW + tx    │   Domain (entities, VOs)     │    │
│  │  UoW          │              │   Infrastructure (repos,      │    │
│  │  Observability│              │   jobs, producers, consumers) │    │
│  └───────────────┘              └──────────────┬───────────────┘    │
│                                                │                    │
│  ┌───────────────┐              ┌──────────────▼───────────────┐    │
│  │  Scheduler    │─── jobs ────▶│   billing_subscriptions      │    │
│  │  (cron)       │              │   billing_processed_events    │    │
│  └───────────────┘              │   billing_kiwify_events       │    │
│                                 │   billing_plans               │    │
│  ┌───────────────┐              │   billing_reconciliation_     │    │
│  │  WhatsApp /   │◀─ noop ─────│   checkpoints                 │    │
│  │  Notificações │  (MVP)       │   outbox_events               │    │
│  │  (interface   │              └──────────────────────────────┘    │
│  │   preparada)  │                                                   │
│  └───────────────┘                                                   │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Arquitetura em Camadas

| Camada | Pacote | Conteúdo |
|---|---|---|
| **Domain** | `domain/entities/` | `Subscription` (aggregate root), métodos de ciclo de vida (`Activate`, `Renew`, `MarkPastDue`, `MarkCanceled`, `MarkRefunded`, `MarkExpiredAfterGrace`) |
| **Domain** | `domain/valueobjects/` | `Status`, `Plan`, `FunnelToken`, `KiwifySubscriptionID`, `GraceWindow` |
| **Domain** | `domain/services/` | `TransitionService` (valida e decide transições), `DecisionService` (`IsRegression`) |
| **Application** | `application/usecases/` | 11 use cases (tabela mestre em [usecases.md](usecases.md)) |
| **Application** | `application/usecases/kiwifypayload/` | Adaptação payload Kiwify → DTOs; `Classify`, `ExtractFunnel`, `Decode`, parsing de timestamps |
| **Application** | `application/dtos/` | DTOs de input e output por use case |
| **Application** | `application/interfaces/` | Portas: `SubscriptionRepository`, `ProcessedEventRepository`, `KiwifyClient`, `NotificationSender`, `RepositoryFactory` |
| **Infrastructure** | `infrastructure/http/server/` | Router chi, `KiwifyWebhookHandler`, middlewares `RawBody`, `HMACSignature`, `RateLimiter` |
| **Infrastructure** | `infrastructure/http/client/kiwify/` | Client REST com OAuth 2.0, token bucket, retry idempotente |
| **Infrastructure** | `infrastructure/messaging/database/producers/` | `SubscriptionEventPublisher` (7 eventos via Outbox) |
| **Infrastructure** | `infrastructure/messaging/database/consumers/` | `NotificationHandler` (3 eventos → `SendSubscriptionNotification`) |
| **Infrastructure** | `infrastructure/jobs/handlers/` | `ReconciliationJob`, `GraceExpirationJob`, `KiwifyEventsHousekeepingJob` |
| **Infrastructure** | `infrastructure/repositories/postgres/` | Implementações SQL de todos os repositórios do módulo |
| **Infrastructure** | `infrastructure/config/` | `PlanCatalog` — upsert dos planos na inicialização |

---

## Tabela de Responsabilidades

| Responsabilidade | Componente | Localização |
|---|---|---|
| Receber webhook Kiwify | `KiwifyWebhookHandler.Handle` | `infrastructure/http/server/handlers/` |
| Validar HMAC-SHA1 (dual-secret) | `middleware.HMACSignature` | `infrastructure/http/server/middleware/hmac_signature.go` |
| Rate limiting por IP | `middleware.RateLimiter` | `infrastructure/http/server/middleware/` |
| Decodificar e classificar payload | `kiwifypayload.Decode` / `Classify` | `application/usecases/kiwifypayload/` |
| Dispatch por trigger | `ProcessKiwifyWebhook` (mapa `handlers[trigger]`) | `application/usecases/process_kiwify_webhook.go` |
| Idempotência de eventos | `ProcessedEventRepository.MarkApplied` | `infrastructure/repositories/postgres/` |
| Ciclo de vida da assinatura | `Subscription.*` + `TransitionService` | `domain/entities/`, `domain/services/` |
| Detecção de regressão | `DecisionService.IsRegression` / `TransitionService.Decide*` | `domain/services/` |
| Publicação Outbox | `SubscriptionEventPublisher.Publish*` | `infrastructure/messaging/database/producers/` |
| Reconciliação com Kiwify | `ReconcileSubscriptions` + `RunReconciliation` | `application/usecases/` |
| Expiração por carência (batch) | `GraceExpirationJob` + `ProcessSubscriptionGraceExpired` | `infrastructure/jobs/handlers/`, `application/usecases/` |
| Housekeeping de eventos brutos | `KiwifyEventsHousekeepingJob` + `CleanupKiwifyEvents` | `infrastructure/jobs/handlers/`, `application/usecases/` |
| Notificação de transições | `NotificationHandler` + `SendSubscriptionNotification` | `infrastructure/messaging/database/consumers/`, `application/usecases/` |

---

## Fluxo Típico: Venda → Ativação → Entitlement

```
Kiwify
  │
  │  POST /api/v1/billing/webhooks/kiwify
  ▼
middleware.RawBody          (lê corpo bruto; limite 256 KB)
  │
  ▼
middleware.HMACSignature    (HMAC-SHA1 hex; dual-secret; status: valid/rotated/invalid)
  │
  ▼
middleware.RateLimiter      (token bucket por IP)
  │
  ▼
KiwifyWebhookHandler.Handle
  │  kiwifypayload.Decode(rawBody)
  │  kiwifypayload.Classify(payload) → "order_approved"
  │  kiwifyEventRepo.Persist(envelope)   ← best-effort (audit trail)
  │
  ▼
ProcessKiwifyWebhook.Execute
  │  handlers["order_approved"] → ProcessSaleApproved.Execute(ctx, input)
  │
  ▼
ProcessSaleApproved.Execute
  │
  ├── processedRepo.MarkApplied("order_approved:<saleID>")
  │     └── UniqueViolation → ErrEventAlreadyProcessed → 202 (idempotência)
  │
  ├── planRepo.FindByKiwifyProductID(productID)
  │
  ├── entities.NewSubscription(plan, funnelToken)
  │     └── sub.Activate(occurredAt)
  │
  ├── subRepo.UpsertByOrder(...)        ┐
  ├── subRepo.FindByOrderID(orderID)    │ dentro de uow.Do (tx única)
  └── publisher.PublishActivated(tx, .) ┘
        └── INSERT outbox_events ON CONFLICT (id) DO NOTHING

  COMMIT
  │
  ▼
DispatcherJob (@every 500ms)
  SELECT FOR UPDATE SKIP LOCKED de outbox_events
  │
  ├─▶ identity/SubscriptionEventProjector
  │     ProjectSubscriptionEvent.Execute
  │       └── identity_entitlements UPSERT ON CONFLICT (user_id) DO UPDATE
  │
  └─▶ billing/NotificationHandler  (apenas para past_due / refunded / expired)
        SendSubscriptionNotification.Execute → noopNotificationSender (MVP)

Response: HTTP 202 {"received":true}
```

---

## Máquina de Estados (Happy Path)

Para a tabela completa de transições permitidas e proibidas, ver [domain.md](domain.md).

| Estado | Acesso ativo (`IsActiveForBilling`) | Terminal |
|---|---|---|
| `TRIALING` | não | não (sem saídas) |
| `ACTIVE` | sim | não |
| `PAST_DUE` | sim (dentro da carência) | não |
| `CANCELED_PENDING` | sim (até `period_end`) | não |
| `EXPIRED` | não | sim |
| `REFUNDED` | não | sim (idempotente) |

Happy path: `[zero] → ACTIVE → PAST_DUE → ACTIVE → CANCELED_PENDING → REFUNDED`

Carência padrão (`DefaultGraceWindow`): **72 horas**. Durante esse período, `IsActiveForBilling = true` mesmo em `PAST_DUE`.

---

## Pontos de Entrada

### Endpoint HTTP

| Método | Path | Handler | Middlewares |
|---|---|---|---|
| `POST` | `/api/v1/billing/webhooks/kiwify` | `KiwifyWebhookHandler` | `RawBody` → `HMACSignature` → `RateLimiter` |

### Jobs Agendados

| Job | Schedule padrão | Timeout | Use Case |
|---|---|---|---|
| `billing-reconciliation` | `KIWIFY_RECONCILIATION_INTERVAL` (obrigatório) | 5 min | `RunReconciliation` |
| `billing-grace-expiration` | `@every 30m` | 2 min | `ProcessSubscriptionGraceExpired` |
| `billing-kiwify-events-housekeeping` | `@daily` | 2 min | `CleanupKiwifyEvents` |

### Consumers de Eventos (Outbox)

| Evento Consumido | Handler | Use Case |
|---|---|---|
| `billing.subscription.past_due` | `NotificationHandler` | `SendSubscriptionNotification` |
| `billing.subscription.refunded` | `NotificationHandler` | `SendSubscriptionNotification` |
| `billing.subscription.expired_after_grace` | `NotificationHandler` | `SendSubscriptionNotification` |

---

## Tabelas de Domínio

Ver detalhamento completo em [schema.md](schema.md).

| Tabela | Propósito |
|---|---|
| `billing_subscriptions` | Estado corrente de cada assinatura; update-heavy (`fillfactor=80`) |
| `billing_plans` | Catálogo estático de planos MONTHLY/QUARTERLY/ANNUAL com mapeamento para produtos Kiwify |
| `billing_processed_events` | Log de idempotência por `event_key`; evita duplo-processamento de webhooks |
| `billing_kiwify_events` | Arquivo bruto (JSONB) de todos os webhooks recebidos; purga periódica via TTL (`fillfactor=85`) |
| `billing_reconciliation_checkpoints` | Watermark do job de reconciliação; uma linha por tarefa nomeada |

---

## Eventos Publicados

Ver payloads completos em [events.md](events.md).

| Evento | Trigger de Negócio | Consumidores |
|---|---|---|
| `billing.subscription.activated` | `order_approved` com `funnel_token` | `identity.SubscriptionEventProjector` |
| `billing.subscription.activated_without_token` | `order_approved` sem `funnel_token` | nenhum (dead-letter imediato) |
| `billing.subscription.renewed` | `subscription_renewed` | `identity.SubscriptionEventProjector` |
| `billing.subscription.past_due` | `subscription_late` | `identity.SubscriptionEventProjector`, `billing.NotificationHandler` |
| `billing.subscription.canceled` | `subscription_canceled` | `identity.SubscriptionEventProjector` |
| `billing.subscription.refunded` | `order_refunded` ou `chargeback` | `identity.SubscriptionEventProjector`, `billing.NotificationHandler` |
| `billing.subscription.expired_after_grace` | Job `GraceExpirationJob` (`@every 30m`) | `billing.NotificationHandler` |

---

## Decisões de Design Notáveis

| Decisão | Detalhe |
|---|---|
| Idempotência em 3 camadas | (1) `billing_processed_events` INSERT com `event_key` único antes de qualquer mutação; (2) `outbox_events` INSERT com `ON CONFLICT (id) DO NOTHING`; (3) `identity_entitlements` UPSERT com `ON CONFLICT (user_id) DO UPDATE` |
| `fillfactor=80` em `billing_subscriptions` | Cada evento Kiwify gera pelo menos um UPDATE; reservar 20% por página heap reduz HOT-update splits e pressão de vacuum |
| Detecção de regressão silenciosa | `DecisionService.IsRegression` retorna `DecisionSkipAsRegression` para eventos fora de ordem; o use case chama `MarkSuperseded` e retorna `ErrEventSuperseded` (não é falha — responde `202`) |
| `CANCELED_PENDING` não é terminal | Acesso permanece ativo até `period_end`; o status terminal é `REFUNDED` ou `EXPIRED` |
| `DispatcherJob` com `SELECT FOR UPDATE SKIP LOCKED` | Permite múltiplas instâncias do dispatcher em paralelo sem conflito de linhas |
| Rotação dual-secret HMAC | `WebhookSecret` (ativo) e `WebhookSecretNext` (rotacional) validados em sequência; zero-downtime na troca de chave |
| `activated_without_token` vai para dead-letter imediato | Sem consumidor registrado; `DispatcherJob` marca `status=failed` e incrementa `outbox_dead_letter_total` |
| `PlanCatalog.Apply` na inicialização | Upsert dos planos no banco em `NewBillingModule`; configuração via variáveis de ambiente sem migration manual |

---

## Configuração Mínima Obrigatória

Variáveis marcadas como obrigatórias para o módulo inicializar com sucesso:

| Variável | Propósito |
|---|---|
| `KIWIFY_CLIENT_ID` | `client_id` OAuth2 para autenticação na API Kiwify |
| `KIWIFY_CLIENT_SECRET` | `client_secret` OAuth2 |
| `KIWIFY_ACCOUNT_ID` | Header `x-kiwify-account-id` em toda requisição à API |
| `KIWIFY_PRODUCT_ID_MONTHLY` | ID do produto Kiwify para plano mensal |
| `KIWIFY_PRODUCT_ID_QUARTERLY` | ID do produto Kiwify para plano trimestral |
| `KIWIFY_PRODUCT_ID_ANNUAL` | ID do produto Kiwify para plano anual |
| `KIWIFY_WEBHOOK_SECRET` | Segredo HMAC-SHA1 para validação de assinatura de webhooks |

---

## Índice de Documentação

| Arquivo | Conteúdo |
|---|---|
| [domain.md](domain.md) | Aggregate `Subscription`, value objects, métodos de ciclo de vida, máquina de estados completa, regras de regressão, erros públicos |
| [usecases.md](usecases.md) | Tabela mestre dos 11 use cases, fluxos detalhados, mecanismo de idempotência, formação de `event_key` |
| [entry-points.md](entry-points.md) | Endpoint HTTP, cadeia de middlewares, mapeamento de erros para HTTP status, jobs agendados, consumers, bootstrap de `module.go` |
| [schema.md](schema.md) | DDL completo das 5 tabelas `billing_*`, índices, constraints, operações de repositório, decisões de design de schema |
| [events.md](events.md) | Tabela mestre dos 7 eventos, contrato de publicação Outbox, payloads completos, consumidores, diagrama de fluxo |
| [_kiwify_section.md](_kiwify_section.md) | Client HTTP Kiwify (OAuth 2.0, rate limiting, retry), classificação de payload, mapeamento de timestamps, variáveis de ambiente completas |

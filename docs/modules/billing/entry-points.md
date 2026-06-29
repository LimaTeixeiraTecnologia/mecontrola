# Entry Points — internal/billing

## HTTP Webhook

### Endpoint

`POST /api/v1/billing/webhooks/kiwify`

Registrado em `internal/billing/infrastructure/http/server/router.go` pelo método `WebhookRouter.Register`.

### Cadeia de Middleware (em ordem de execução)

| # | Middleware | Propósito | Configuração |
|---|-----------|-----------|--------------|
| 1 | `middleware.RawBody` | Lê e armazena o corpo bruto no contexto antes de qualquer leitura posterior; restaura `r.Body` para leituras subsequentes. Limite: 256 KB. Acima do limite responde `413 Request Entity Too Large`. | Constante `maxBodyBytes = 256 * 1024` em `raw_body_buffer.go` |
| 2 | `middleware.HMACSignature(secretCurrent, secretNext)` | Valida a assinatura HMAC-SHA1 do corpo bruto. Em assinatura inválida responde `401 Unauthorized` e encerra a cadeia. Propaga o status da assinatura (`valid`/`rotated`) no contexto. | `cfg.KiwifyConfig.WebhookSecret` e `cfg.KiwifyConfig.WebhookSecretNext` |
| 3 | `RateLimiter.Middleware` | Rate limiting por IP usando token bucket. Incluso na cadeia apenas quando `limiter != nil`. Responde `429 Too Many Requests` quando o bucket está esgotado. | `cfg.KiwifyConfig.WebhookRateLimitPerMin`, `cfg.KiwifyConfig.WebhookRateLimitBurst`, `cfg.KiwifyConfig.WebhookTrustedProxies` |

O handler `KiwifyWebhookHandler.Handle` é executado após os três middlewares.

### Validação de Assinatura HMAC

**Algoritmo:** HMAC-SHA1 com saída codificada em hexadecimal (`encoding/hex`).

**Fonte do valor recebido:** o middleware tenta ler o parâmetro de query `signature` primeiro; se ausente, lê o header `X-Kiwify-Signature`.

**Rotação dual-secret:**

1. Se `secretCurrent` não é vazio e a assinatura calculada com ele coincidir com o valor recebido (`hmac.Equal`), o status propagado é `"valid"`.
2. Se a verificação com `secretCurrent` falhar e `secretNext` não for vazio e coincidir, o status propagado é `"rotated"`.
3. Se nenhum secret coincidir, o status é `"invalid"` e o middleware interrompe a cadeia com `401 Unauthorized`.

O status (`valid`/`rotated`/`invalid`) é armazenado no contexto e recuperado pelo handler por meio de `middleware.SignatureStatusFromContext(r)`.

**Constantes definidas em `hmac_signature.go`:**

```
SignatureStatusValid   = "valid"
SignatureStatusInvalid = "invalid"
SignatureStatusRotated = "rotated"
```

### Mapeamento de Erros → HTTP Status

| Erro Go | Status HTTP | Cenário |
|---|---|---|
| `ErrInvalidWebhookPayload` | `422 Unprocessable Entity` | JSON do corpo malformado ou campos obrigatórios ausentes |
| `ErrInvalidSignature` | `401 Unauthorized` | Use case rejeita assinatura |
| `ErrEventAlreadyProcessed` | `202 Accepted` | Evento idempotente já processado com sucesso |
| `ErrEventSuperseded` | `202 Accepted` | Evento supersedido por versão mais recente |
| `ErrFunnelTokenMissing` | `422 Unprocessable Entity` | Webhook de ativação sem `funnel_token` válido |
| `ErrKiwifySubscriptionIDInvalid` | `422 Unprocessable Entity` | `subscription_id` inválido no payload |
| `ErrUnknownTrigger` | `422 Unprocessable Entity` | Trigger não suportado |
| qualquer outro erro | `500 Internal Server Error` | Falha inesperada; registrado no span |

O handler verifica os erros na ordem da tabela `webhookErrorTable` usando `errors.Is`, portanto erros encapsulados com `%w` são corretamente mapeados.

**Validações adicionais do handler:**

- `Content-Type` deve ter prefixo `application/json` → `415 Unsupported Media Type`.
- Corpo bruto deve estar disponível no contexto → `500` se ausente.

### Resposta de Sucesso

- **Status:** `202 Accepted`
- **Body:** `{"received":true}`
- Eventos já processados (`ErrEventAlreadyProcessed`) e eventos supersedidos (`ErrEventSuperseded`) também retornam `202 Accepted`.

---

## Jobs Agendados

| Job | Nome | Schedule Padrão | Timeout | Use Case Chamado | Configuração |
|---|---|---|---|---|---|
| `ReconciliationJob` | `billing-reconciliation` | `cfg.KiwifyConfig.ReconciliationInterval` (obrigatório) | 5 minutos | `RunReconciliation` | `KiwifyConfig.ReconciliationInterval` |
| `GraceExpirationJob` | `billing-grace-expiration` | `@every 30m` | 2 minutos | `ProcessSubscriptionGraceExpired` | `BillingConfig.GraceExpirationSchedule`; fallback `"@every 30m"` |
| `KiwifyEventsHousekeepingJob` | `billing-kiwify-events-housekeeping` | `@daily` | 2 minutos | `CleanupKiwifyEvents` | `BillingConfig.KiwifyEventsHousekeepingSchedule`; fallback `"@daily"` |

### ReconciliationJob

Arquivo: `internal/billing/infrastructure/jobs/handlers/reconciliation_job.go`

Invoca `RunReconciliation.Execute(ctx)`. O schedule é controlado pelo campo `cfg.KiwifyConfig.ReconciliationInterval` — sem valor default; se vazio, o scheduler rejeitará o job.

### GraceExpirationJob

Arquivo: `internal/billing/infrastructure/jobs/handlers/grace_expiration_job.go`

Invoca `ProcessSubscriptionGraceExpired.Execute(ctx)`. Schedule padrão `"@every 30m"`. Verifica assinaturas em `PAST_DUE` cujo `grace_end` já passou e as transiciona para `EXPIRED`.

### KiwifyEventsHousekeepingJob

Arquivo: `internal/billing/infrastructure/jobs/handlers/kiwify_events_housekeeping_job.go`

Invoca `CleanupKiwifyEvents.Execute(ctx)`. Schedule padrão `"@daily"`. Remove registros da tabela `billing_kiwify_events` com mais de 90 dias.

---

## Consumers de Eventos (Outbox)

| Tipo de Evento Consumido | Handler | Use Case Chamado |
|---|---|---|
| `billing.subscription.past_due` | `notificationPastDue` | `SendSubscriptionNotification` |
| `billing.subscription.refunded` | `notificationRefunded` | `SendSubscriptionNotification` |
| `billing.subscription.expired_after_grace` | `notificationExpired` | `SendSubscriptionNotification` |

### NotificationHandler

Arquivo: `internal/billing/infrastructure/messaging/database/consumers/notification_handler.go`

**Como é registrado:** `module.go` constrói um slice `[]EventHandlerRegistration{...}` exportado em `BillingModule.EventHandlers`. O bootstrap da aplicação itera sobre esse slice e registra cada `Handler` no event bus para o `EventType` correspondente.

**Extração do payload:** o método `Handle` faz type assertion de `event.GetPayload().(outbox.Envelope)`. Se o payload não for um `outbox.Envelope`, o handler retorna `nil` silenciosamente.

**Comportamento em erro:** se `SendSubscriptionNotification.Execute` retornar erro, o erro é propagado para o caller do `Handle`, que pode acionar o mecanismo de retry do outbox.

**Nota:** em `module.go`, `sendNotification` é criado com `&noopNotificationSender{}` — implementação vazia que retorna `nil` incondicionalmente. As notificações estão estruturadas mas desabilitadas no MVP.

---

## Bootstrap do Módulo (module.go)

Arquivo: `internal/billing/module.go`

### Ordem de Inicialização

1. `repositories.NewRepositoryFactory(o11y)` — fábrica de repositórios do módulo billing.
2. `producers.NewSubscriptionEventPublisher(...)` — publisher de eventos via outbox.
3. `newKiwifyClient(cfg, o11y)` — cliente HTTP para a API Kiwify (retorna erro se configuração inválida).
4. `billingconfig.NewPlanCatalog(cfg.KiwifyConfig)` — catálogo de planos da configuração (retorna erro se inválido).
5. Se `db != nil`: `catalog.Apply(ctx, factory.PlanRepository(db))` — upsert dos planos no banco na inicialização.
6. `uow.NewUnitOfWork(db)` — unit of work para transações.
7. Repositórios concretos: `ReconciliationCheckpointRepository` e `KiwifyEventRepository`.
8. Use cases de transição de estado: `ProcessSaleApproved`, `ProcessSubscriptionRenewed`, `ProcessSubscriptionLate`, `ProcessSubscriptionCanceled`, `ProcessRefundOrChargeback`, `ProcessSubscriptionGraceExpired`.
9. Use cases orquestradores: `ReconcileSubscriptions`, `ProcessKiwifyWebhook`, `RunReconciliation`, `CleanupKiwifyEvents`, `SendSubscriptionNotification`.
10. `handlers.NewKiwifyWebhookHandler(processWebhook, o11y)`.
11. `billingmiddleware.NewRateLimiter(...)` — instância de rate limiter com GC goroutine própria.
12. `billingserver.NewWebhookRouter(webhookHandler, secretCurrent, secretNext, webhookLimiter)`.
13. Três instâncias de `consumers.NewNotificationHandler(sendNotification, eventType, o11y)`.
14. Retorna `BillingModule{...}` populado.

Qualquer erro nos passos 3, 4 ou 5 interrompe a inicialização e propaga o erro para o caller.

### Exports do BillingModule

| Campo | Tipo | Consumidor |
|---|---|---|
| `RepositoryFactory` | `interfaces.RepositoryFactory` | Módulos externos com acesso de leitura a repositórios billing |
| `WebhookRouter` | `*billingserver.WebhookRouter` | Bootstrap da aplicação: chama `WebhookRouter.Register(r)` no roteador principal chi |
| `ReconciliationJob` | `*billingjobs.ReconciliationJob` | Scheduler de jobs |
| `KiwifyEventsHousekeeper` | `*billingjobs.KiwifyEventsHousekeepingJob` | Scheduler de jobs |
| `GraceExpirationJob` | `*billingjobs.GraceExpirationJob` | Scheduler de jobs |
| `SubscriptionEventPublisher` | `*producers.SubscriptionEventPublisher` | Bootstrap da aplicação |
| `EventHandlers` | `[]EventHandlerRegistration` | Bootstrap da aplicação: itera e registra cada `Handler` no event bus |

---

## Referências

- [usecases.md](usecases.md)
- [events.md](events.md)
- [schema.md](schema.md)

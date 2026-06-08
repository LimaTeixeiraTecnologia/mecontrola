<!-- spec-hash-prd: d4dd551c624b11b0e588deefb020ee94e4baadd9b283366cfccde1fa19345a6b -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

> **NOTA DE DRIFT — 2026-06-08:** Esta techspec contém referências históricas a `HMAC-SHA256 base64` no header `X-Kiwify-Signature` (§8.1, §5.1, §3.5, §4 etc) que **NÃO refletem o protocolo real da Kiwify**. Captura empírica em sandbox em 2026-06-08 demonstrou que o protocolo correto é **HMAC-SHA1 em hex via query string `?signature=`**. ADR vigente: [`adr-002b-hmac-sha1-hex-webhook-query-signature.md`](./adr-002b-hmac-sha1-hex-webhook-query-signature.md) (substitui ADR-002). Implementação no código está alinhada à ADR-002b. O conteúdo histórico desta techspec é mantido para auditoria e contexto — não usar como referência de implementação atual.

# Análise inicial

## Status do PRD de entrada

- **PRD:** `.specs/prd-billing-pipeline/prd.md`
- **Estado:** aprovado em 2026-06-05; 21 requisitos funcionais (RF-01 → RF-21), 5 funcionalidades core (F-01 → F-05), 4 métricas de sucesso (M-01 → M-04), 5 premissas firmes (P-01 → P-05), 5 riscos de negócio (R-01 → R-05), 3 questões em aberto (Q-01 fora-do-techspec, Q-02 travada, Q-03 travada).
- **Pode prosseguir?:** sim. Decisões abertas tratadas em 4 rodadas multipla-escolha com o usuário; Q-02 e Q-03 travadas; Q-01 isolada como decisão operacional (interface declarada, copy fora do techspec).

## Mapeamento do codebase atual

| Caminho | Estado atual | Impacto na techspec |
| --- | --- | --- |
| `cmd/server/server.go` | Registra apenas `identity` (`NewIdentityModule`); usa `httpserver.RegisterRouters`. | Acrescentar `billing.NewBillingModule(...)` e registrar `billingModule.WebhookRouter`. |
| `cmd/worker/worker.go` | Sobe outbox (dispatcher/reaper/housekeeping) com `events.Dispatcher` in-process; sem wiring de `identity` ou `billing`. | Construir `identity.NewIdentityModule` e `billing.NewBillingModule` no worker, registrar handlers de identity no `events.Dispatcher`, registrar jobs de billing (reconciliação) e housekeeping de `kiwify_events` no `WorkerManager`. |
| `internal/billing/module.go` | Placeholder (`package billing`). | Implementar `NewBillingModule(...) BillingModule` com DI manual seguindo padrão `IdentityModule`. |
| `internal/billing/infrastructure/http/server/routes.go` | Placeholder (`package server`). | Adicionar `WebhookRouter` com `Register(chi.Router)` expondo `POST /api/v1/billing/webhooks/kiwify`. |
| `internal/identity/domain/entitlement.go` | `IsEntitled(Subscription, now) (bool, Reason)` já implementada; `Subscription` é interface no domain. | Read model de identity implementa essa interface com base na projection `identity.entitlements`. |
| `internal/identity/infrastructure/messaging/database/consumers/doc.go` | `package consumers` vazio. | Adicionar `EntitlementProjector` (`events.Handler`) consumindo eventos `billing.subscription.*` da outbox-stream. |
| `internal/identity/module.go` | DI manual de `RepositoryFactory`, `UserRouter`, 4 use cases. | Expor `EntitlementProjector` (handler) e factory de `EntitlementReader` para outros módulos. |
| `internal/platform/events/{events,dispatcher}.go` | `Event`/`Handler`/`Dispatcher` in-process, thread-safe; já consumido por `outbox.DispatcherJob`. | Cross-module billing→identity via esta dispatcher. |
| `internal/platform/outbox/{publisher,dispatcher}.go` | `Publisher.Publish(ctx, evt)` valida UUID, type, aggregate, payload JSON; `DispatcherJob` claim/retry/dispatch via `Registry`. | Billing usa `outbox.Publisher` para todos os eventos emitidos. |
| `internal/platform/worker/{manager,consumer/registration}.go` | `WorkerManager` orquestra `worker.Job` e `worker.Consumer`. | Job de reconciliação e housekeeping de `kiwify_events` via `job.NewAdapter`. |
| `internal/platform/httpclient/client.go` | Wrapper sobre `devkit-go` com timeout, base URL, retry opcional. | Client OAuth+rate-limited da Kiwify em `internal/billing/infrastructure/http/client/kiwify`. |
| `configs/config.go` | `KiwifyConfig` e `BillingConfig` já declarados com campos para webhook, OAuth, rate limit, reconciliação. | Reutilizar; adicionar apenas `KIWIFY_WEBHOOK_SECRET_NEXT` (rotação) e `BILLING_PAST_DUE_TEMPLATE` se necessário. |
| `migrations/` | 3 migrations (outbox, users, whatsapp_history). | Adicionar 5 migrations billing + 1 identity (entitlements). |

## Evidências oficiais Kiwify usadas

| Fonte oficial | Fato confirmado | Impacto técnico |
| --- | --- | --- |
| `https://docs.kiwify.com.br/api-reference/general` (Public API) | Base URL `public-api.kiwify.com/v1`. Headers obrigatórios: `Authorization: Bearer ...` + `x-kiwify-account-id: ...`. Rate limit 100 req/min. | Client em `internal/billing/infrastructure/http/client/kiwify` segue estritamente esse contrato. |
| `https://docs.kiwify.com.br/api-reference/auth/oauth` | `POST /v1/oauth/token` form-urlencoded (`client_id`/`client_secret`) retorna `access_token` Bearer (`token_type:"Bearer"`, `expires_in: 86400`). | Cache em-memória com refresh em `expires_in - safety_margin`. |
| `https://docs.kiwify.com.br/api-reference/webhooks/create` | `POST /v1/webhooks` aceita `triggers[]`. Triggers oficiais: `boleto_gerado`, `pix_gerado`, `carrinho_abandonado`, `compra_recusada`, `order_approved`, `order_refunded`, `chargeback`, `subscription_canceled`, `subscription_late`, `subscription_renewed`. Recurso webhook persiste campo `token`. | Subscription do webhook é provisionada externamente (não pela aplicação) com 5 triggers do MVP: `order_approved`, `subscription_renewed`, `subscription_late`, `subscription_canceled`, `order_refunded`, `chargeback`. |
| `https://docs.kiwify.com.br/api-reference/sales/list` | `GET /v1/sales` query: `start_date`/`end_date` (obrigatórios, ≤ 90d), `status`, `updated_at_start_date`, `updated_at_end_date`, `payment_method`, `product_id`, `page_size`, `page_number`. Resposta paginada. | Reconciliação horária varre por `updated_at_start_date = checkpoint - 15m` paginado. |
| `https://docs.kiwify.com.br/api-reference/sales/single` | `GET /v1/sales/{id}` retorna sale completa: `id`, `reference`, `status`, `created_at`, `updated_at`, `payment_method`, `net_amount`, `customer`, `tracking{utm_*, sck, src, s1..s3}`, `parent_order_id`, `sale_type`, `refunded_at`. | `tracking.s1` (ou `src`) carrega o `?s={token}` validado em P-01 (H7). Mapeamento exato confirmado em sandbox antes da execução. |
| `https://docs.kiwify.com.br/api-reference/banking/webhooks` (Banking) | Envelope `id`/`type`/`version`/`data`/`created_at`; `id` muda por retry, dedupe por `recurso_id + type`; assinatura Ed25519 prehashed; chave pública em `GET /v1/webhooks-keys`. | **Banking API NÃO é usada** (não cobre assinatura). Referenciado apenas como padrão para dedupe (`recurso_id + type`) e como ADR-001 explicando a divergência com o prompt enriquecido. |
| **Lacuna oficial** | Algoritmo/header de assinatura da Public API para webhooks de assinatura **não está documentado**. Apenas o campo `token` existe no recurso webhook. | Adotar HMAC-SHA256 sobre `raw_body` com `token` como secret (suposição) — ADR-002. Confirmar em sandbox antes da execução. |
| **Lacuna oficial** | Não existe `GET /v1/subscriptions` documentado. | Reconciliação restrita a `GET /v1/sales` com janela de até 90d — ADR-006. |
| **Lacuna oficial** | Política oficial de retry/janela dos webhooks de produto não documentada. | Idempotência por `event_key` composto absorve qualquer cadência de retry — ADR-005. |

## Bloqueios e dúvidas

1. **L-01 (HMAC suposto):** confirmar com suporte Kiwify (ou sandbox) o algoritmo real de assinatura antes do início da execução. Não bloqueia a techspec — mapeado em ADR-002 com suposição explícita e plano de validação.
2. **L-02 (`updated_at_start_date`):** validar que o filtro funciona como documentado (cardinalidade, ordenação por updated_at). Coberto por ADR-006 (estratégia de checkpoint conservadora `last_run - 15m` e idempotência total).
3. **L-03 (campo do token de funil):** confirmar qual campo de `sale.tracking` carrega o `?s={token}` (P-01 validada com compra real, mas o campo exato precisa ser fixado). Coberto em §6.3 e ADR-002.

Nenhum bloqueio material para escrever a techspec. As três lacunas viram itens de verificação pré-execução (não pré-tarefas).

---

# Techspec — Billing Pipeline MVP

## 1. Objetivo técnico e recorte do MVP

Entregar o ciclo de vida da assinatura ponta a ponta para o gateway **Kiwify único**, expondo: (a) endpoint HTTP de webhook autenticado, (b) máquina de estados idempotente sobre o agregado `Subscription`, (c) emissão de eventos internos via outbox transacional, (d) read model de entitlement em `identity`, (e) reconciliação horária via `GET /v1/sales`, (f) notificação WhatsApp best-effort.

Recorte estrito **dentro do MVP**:
- 5 triggers Kiwify (`order_approved`, `subscription_renewed`, `subscription_late`, `subscription_canceled`, `order_refunded`+`chargeback`);
- 5 estados ativos + `TRIALING` inerte;
- 3 planos (Mensal/Trimestral/Anual) seedados via migration;
- 1 usuário = 1 sub ativa simultânea;
- janela de graça uniforme 3 dias (Q-02 travada);
- refund parcial = REFUNDED total (Q-03 travada);
- best-effort notification (Q-01 — só interface, não copy).

**Fora do MVP** (apenas registrado): multi-provider, trial, override administrativo, sweep retroativo 90d full, dashboard MRR/churn, anonimização programada, plano família, painel admin, rate-limit por usuário no bot, replay forte de notificação.

## 2. Requisitos do PRD mapeados para decisões técnicas

| Requisito | Decisão técnica | Mecanismo |
| --- | --- | --- |
| RF-01 (Kiwify único) | `KiwifyWebhookHandler` único; `BillingConfig` sem chave de provider. | Hardcoded `provider=kiwify`. |
| RF-02 (planos M/T/A) | Tabela `billing.plans` seedada (ADR-007). | Migration `0004_create_billing_plans.up.sql`. |
| RF-03 (rejeitar sem token) | Use case `ProcessSaleApproved` falha com `ErrFunnelTokenMissing` (HTTP 422). | Extração via `sale.tracking.s1` (a confirmar empiricamente). |
| RF-04 (5 estados + TRIALING reservado) | `domain.SubscriptionStatus` em `internal/identity/domain/entitlement.go` (já existe). Billing usa o mesmo conjunto exposto via valueobject local `billing.domain.valueobjects.Status`. | Sem reúso direto entre módulos (identity é consumer); enum espelhada com test cross-check. |
| RF-05/06/07/08/09 | Lookup `IsEntitled` consome read model em `identity.entitlements`. Grace 3d uniforme (`grace_end = late_at + 3 * 24h`). Refund terminal. | `EntitlementProjector` materializa `(status, period_end, grace_end)` por user. |
| RF-10 (5 efeitos) | 5 use cases discretos em `internal/billing/application/usecases/`. | Dispatch por `trigger` em `KiwifyWebhookHandler`. |
| RF-11 (idempotência) | Tabela `billing.processed_events(event_key PK)` (ADR-005). | INSERT antes de aplicar; conflito = no-op. |
| RF-12 (out-of-order) | Vetor `last_event_at` + transições permitidas (ADR-005). | Eventos staled → `superseded` em `processed_events`. |
| RF-13/14/15 (decisão única) | `domain.IsEntitled` (E1, já implementada) sobre read model `identity.entitlements`. Cache LRU opcional (`BillingConfig.EntitlementCacheCapacity/TTL`). | Sem fallback para query cross-module. |
| RF-16 (sub presa ao token) | Schema `subscriptions(funnel_token NOT NULL, user_id UUID NULL)`. | Evento `billing.subscription.activated` carrega `funnel_token`; E3 (futuro) emite `identity.user.bound_to_token`. |
| RF-17 (1 sub ativa por user) | Índice único parcial em `(user_id) WHERE user_id IS NOT NULL AND status IN ('ACTIVE','PAST_DUE','CANCELED_PENDING')`. | Conflito → `ErrConcurrentActiveSubscription`. |
| RF-18 (reconciliação horária) | Job `ReconciliationJob` `@every 1h` (ADR-006). | Checkpoint em `billing.reconciliation_checkpoints`. |
| RF-19 (fora E4) | Não implementar dashboard, sweep 90d, anonimização. | Mensão no §11. |
| RF-20 (notificação best-effort) | Handler dedicado consome eventos `billing.subscription.{transitioned_past_due,refunded,expired_after_grace}`; falha de envio é log, não erro. | `application/interfaces.NotificationSender` — interface no consumidor (billing). Implementação concreta em `infrastructure/messaging/whatsapp/` fica para E3/E5 (stub no-op no MVP). |
| RF-21 (whitelist comandos) | N/A; fica em E3. | Mencionar no §11. |
| M-01..M-04 (métricas) | Métricas Prometheus expostas (§9). | `billing_webhooks_received_total`, `billing_processing_latency_seconds`, `billing_subscription_transitions_total`, `billing_reconciliation_corrections_total`. |

## 3. Estado atual do codebase e gaps reais

Estado atual relevante (verificado no working tree):

- `internal/billing/` tem apenas dois arquivos placeholder (`module.go`, `infrastructure/http/server/routes.go`).
- `internal/identity/infrastructure/messaging/database/consumers/doc.go` é stub vazio.
- `cmd/server/server.go` não tem wiring de billing.
- `cmd/worker/worker.go` não tem wiring de billing nem registra handlers de identity no `events.Dispatcher` (chave para o caminho outbox→consumer cross-module).
- `internal/identity/domain/entitlement.go` define `Subscription` (interface) e `IsEntitled` (função pura). **Não existe** repositório nem implementação concreta dessa interface ainda — gap a fechar pelo read model.
- `configs.KiwifyConfig` e `configs.BillingConfig` **já existem** com campos para webhook, OAuth, rate limit, reconciliação, cache de entitlement. Acrescentar apenas `KIWIFY_WEBHOOK_SECRET_NEXT` (rotação) e `KIWIFY_WEBHOOK_TOLERANCE` (janela aceitável de drift do timestamp se a Kiwify enviar).
- `internal/platform/outbox.Publisher` exige `event.ID` UUID, `event.Type` não-vazio, `aggregate`, `payload` JSON e `occurred_at` não-zero.
- `internal/platform/worker.Job` exige `Name()`/`Schedule() string`/`Run(ctx) error`.

Gaps que a techspec resolve:
- ausência de `Subscription` aggregate concreto em billing;
- ausência de tabela `subscriptions`, `plans`, `processed_events`, `kiwify_events`, `reconciliation_checkpoints`;
- ausência de `EntitlementReader` em identity (sem read model);
- ausência de wiring billing em `cmd/server` e `cmd/worker`;
- ausência de registro de handlers de identity no `events.Dispatcher` do worker;
- ausência do client OAuth Kiwify.

## 4. Arquitetura proposta

Texto compacto do desenho. Cumpre estritamente o layout obrigatório de `AGENTS.md` §"Layout Obrigatorio por Modulo" e o "Padrao Obrigatorio de Modulo".

```text
internal/billing/
  module.go                          # NewBillingModule(cfg, o11y, mgr) BillingModule
  application/
    dtos/input/                      # ProcessSaleApprovedInput, ProcessSubscriptionRenewedInput, ...
    dtos/output/                     # SubscriptionView (read interno; nunca cross-module)
    interfaces/
      subscription_repository.go     # SubscriptionRepository (consumer-defined)
      processed_event_repository.go  # ProcessedEventRepository (idempotência)
      kiwify_event_repository.go     # KiwifyEventRepository (raw audit)
      plan_repository.go             # PlanRepository
      reconciliation_checkpoint_repository.go
      kiwify_client.go               # KiwifyClient: ListSalesUpdatedSince, GetSale
      notification_sender.go         # NotificationSender (best-effort, stub no MVP)
      subscription_event_publisher.go # EventPublisher: PublishActivated/Renewed/PastDue/...
      repository_factory.go          # RepositoryFactory (mesmo padrão de identity)
    usecases/
      process_sale_approved.go
      process_subscription_renewed.go
      process_subscription_late.go
      process_subscription_canceled.go
      process_refund_or_chargeback.go
      reconcile_subscriptions.go      # consumido pelo job
  domain/
    entities/
      subscription.go                # agregado com state machine
    valueobjects/
      status.go                      # espelha SubscriptionStatus de identity (cross-check em teste)
      plan.go                        # Plan{Code, DurationDays}
      funnel_token.go                # invariante !empty
      money.go                       # se necessário p/ amount (centavos)
    services/
      transitions.go                 # tabela de transições permitidas + helpers
  infrastructure/
    http/client/kiwify/
      client.go                      # OAuth + rate limit + retries; usa platform/httpclient
      auth.go                        # token cache em-memória
      ratelimit.go
      models.go
    http/server/
      router.go                      # WebhookRouter.Register(chi.Router)
      routes.go
      handlers/
        kiwify_webhook_handler.go
      middleware/
        hmac_signature.go            # valida HMAC sobre raw_body antes do parser
        raw_body_buffer.go
    jobs/handlers/
      reconciliation_job.go          # worker.Job
      kiwify_events_housekeeping_job.go
    messaging/database/producers/
      subscription_event_publisher.go # implementa interfaces.EventPublisher via outbox.Publisher
      events.go                       # tipos: SubscriptionActivated, ...Renewed, ...PastDue, ...Canceled, ...Refunded
    repositories/postgres/
      subscription_repository.go
      processed_event_repository.go
      kiwify_event_repository.go
      plan_repository.go
      reconciliation_checkpoint_repository.go
      factory.go

internal/identity/
  module.go                           # adicionar EntitlementProjector + EntitlementReader
  application/
    interfaces/
      entitlement_repository.go       # consumer-defined
    usecases/
      decide_user_entitlement.go      # wraps domain.IsEntitled
    dtos/
      entitlement_decision.go
  infrastructure/
    repositories/postgres/
      entitlement_repository.go       # read model identity.entitlements
    messaging/database/consumers/
      subscription_event_projector.go # events.Handler para billing.subscription.*
```

**Fluxos críticos:**
- **Webhook inbound:** `server` → middleware HMAC → handler → use case → `SubscriptionRepository` + `ProcessedEventRepository` + `outbox.Publisher` (todos na mesma transação via `uow.New[...]`). Resposta `202 Accepted`.
- **Cross-module:** worker `outbox.DispatcherJob` reclama row → `events.Dispatcher.Dispatch` (in-process) → `SubscriptionEventProjector` (identity) atualiza `identity.entitlements`. Falha do projector → outbox marca retry com backoff exponencial; no MVP, falha persistente fica como `failed` após `RetryMaxAttempts` e dispara alerta.
- **Reconciliação:** worker `WorkerManager` invoca `ReconciliationJob` `@every 1h` → `KiwifyClient.ListSalesUpdatedSince(checkpoint-15m)` → para cada sale, executa o mesmo use case do webhook (idempotente pela `event_key`).

## 5. Fluxo ponta a ponta do webhook Kiwify até os eventos internos

### 5.1 Sequência canônica (caminho feliz `order_approved`)

1. **POST /api/v1/billing/webhooks/kiwify** chega no `httpserver` (chi).
2. **`raw_body_buffer` middleware** lê e armazena o body em `context` (`ctxKey{rawBody}`), evitando re-leitura.
3. **`hmac_signature` middleware** computa `expected = base64(hmac_sha256(secret, raw_body))` e compara em tempo constante com header `X-Kiwify-Signature` (e fallback `signature` query). Tolera `KIWIFY_WEBHOOK_SECRET_NEXT` durante rotação. Em mismatch → `401`.
4. **`KiwifyWebhookHandler.Handle`** faz `json.Unmarshal` para `Envelope{Trigger, Data}`; persiste em `kiwify_events(envelope_id, trigger, raw_body, received_at, signature_status='valid')` fora da transação (auditoria proativa).
5. **Dispatch por trigger** chama o use case correspondente:
   - `order_approved` → `ProcessSaleApproved`
   - `subscription_renewed` → `ProcessSubscriptionRenewed`
   - `subscription_late` → `ProcessSubscriptionLate`
   - `subscription_canceled` → `ProcessSubscriptionCanceled`
   - `order_refunded` ∪ `chargeback` → `ProcessRefundOrChargeback`
6. **Use case** monta `eventKey = "order_approved:" + order_id`, abre `uow.New[domain.Subscription]`, dentro da transação:
   1. `ProcessedEventRepository.MarkApplied(ctx, event_key, occurred_at)` — INSERT; conflito → return `ErrEventAlreadyProcessed` (idempotente, no-op silencioso, retorna 202).
   2. Resolve `Plan` por `kiwify_product_id`.
   3. Resolve `funnelToken` do payload (`sale.tracking.s1`, validado em sandbox); se vazio → `ErrFunnelTokenMissing` (return 422, **não** persiste subscription nem evento processado — a Kiwify deve reenviar; é caso operacional).
   4. `SubscriptionRepository.UpsertByOrder(ctx, sub)` — cria ou estende subscription com `status=ACTIVE`, `period_end = now + plan.duration`, `last_event_at = occurred_at`.
   5. `subscriptionEventPublisher.PublishActivated(ctx, sub, funnelToken)` — escreve em `platform_outbox_events` na mesma transação (`outbox.Publisher.Publish`).
7. **Commit da UoW.** Handler retorna **202 Accepted** com body `{"received":true}`.
8. **Worker `outbox.DispatcherJob`** (já em produção via `cmd/worker`) reclama o row, despacha via `events.Dispatcher`.
9. **`SubscriptionEventProjector`** (registrado pelo worker em `events.Dispatcher`) recebe o envelope, faz UPSERT em `identity.entitlements(user_id, subscription_id, status, period_end, grace_end, updated_at)` — mas só quando `user_id` é conhecido (após E3). No MVP, se `user_id` ainda for NULL na `subscriptions`, o projector marca `identity.entitlements_pending(subscription_id, funnel_token, payload JSONB)` aguardando E3.
10. **NotificationDispatcher** (consumer best-effort) ignora `activated` no MVP; só age em `past_due`/`refunded`/`expired_after_grace`.

### 5.2 Variações por trigger

| Trigger | Diferença vs §5.1 |
| --- | --- |
| `subscription_renewed` | Etapa 6.4: `SubscriptionRepository.ExtendPeriod(ctx, sub_id, duration)`; etapa 6.5 emite `billing.subscription.renewed`. Se subscription não existir localmente: cria placeholder ACTIVE com período derivado (ordering — ADR-005). |
| `subscription_late` | Etapa 6.4: transição → `PAST_DUE`, set `grace_end = late_at + 3 * 24h`; emite `billing.subscription.past_due`. Notification handler é acionado downstream. |
| `subscription_canceled` | Etapa 6.4: `CANCELED_PENDING`, mantém `period_end`; emite `billing.subscription.canceled`. |
| `order_refunded` / `chargeback` | Etapa 6.4: força `REFUNDED` (terminal); mesmo `event_key` para ambos por `order_id` (ADR-005); emite `billing.subscription.refunded`. |

### 5.3 Detecção de evento staled (out-of-order)

No início da etapa 6.4, comparar `payload.occurred_at` com `subscription.last_event_at`:
- se `occurred_at <= last_event_at` **E** a transição alvo é "retrocesso" (ex.: `RENEWED` chegando depois de `LATE` recente), **não aplicar**; marcar `processed_events.status='superseded'` e retornar 202.
- transições terminais (`REFUNDED`) sempre vencem regardless do timestamp.

Tabela de transições explícita em `domain/services/transitions.go`:

```
                ACTIVE    PAST_DUE  CANCELED_PENDING  EXPIRED   REFUNDED
ACTIVE          extend    set       set               (time)    set
PAST_DUE        set       extend    set               (grace)   set
CANCELED_PENDING extend?  -         -                 (period)  set
EXPIRED         set       -         -                 -         set
REFUNDED        -         -         -                 -         -
```

(`extend` = subscription_renewed; `set` = transição direta; `(time)`/`(grace)`/`(period)` = decididos pela função `IsEntitled` em runtime, **não** persistidos por job.)

## 6. Contratos, handlers, routers e pontos de DI

### 6.1 `internal/billing/module.go`

```go
type BillingModule struct {
    RepositoryFactory      interfaces.RepositoryFactory
    WebhookRouter          *server.WebhookRouter
    ReconciliationJob      *handlers.ReconciliationJob
    KiwifyEventsHousekeeper *handlers.KiwifyEventsHousekeepingJob
    SubscriptionEventPublisher producers.SubscriptionEventPublisher
    EventHandlers          []EventHandlerRegistration
}

type EventHandlerRegistration struct {
    EventType string
    Handler   events.Handler
}

func NewBillingModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) BillingModule { ... }
```

`NewBillingModule` segue o estilo `NewIdentityModule`: constrói factories → use cases → handler/router/jobs → struct. Os `EventHandlers` retornados são os emissores **registráveis** que o worker registra na `events.Dispatcher` para o caminho outbox→cross-module (no MVP, billing só **produz**, então a lista é vazia; quem registra handlers em billing-side é externo).

### 6.2 `internal/identity/module.go` (modificações)

```go
type IdentityModule struct {
    // ... campos existentes
    EntitlementReader     interfaces.EntitlementReader
    SubscriptionProjector consumers.SubscriptionEventProjector
    EventHandlers         []EventHandlerRegistration
}
```

`EventHandlers` contém `{EventType: "billing.subscription.activated", Handler: projector}` e demais 4 tipos. Esse slice é consumido pelo `cmd/worker` para registrar no `events.Dispatcher`.

### 6.3 Endpoint HTTP

- **Path:** `POST /api/v1/billing/webhooks/kiwify`
- **Auth:** middleware `hmac_signature` valida `X-Kiwify-Signature` (header preferencial; fallback query `signature`) usando HMAC-SHA256 sobre o `raw_body` com `KIWIFY_WEBHOOK_SECRET` (e `KIWIFY_WEBHOOK_SECRET_NEXT` durante rotação). Comparação em tempo constante (`hmac.Equal`).
- **Resposta:** `202 Accepted` em sucesso ou no-op idempotente; `401` em mismatch; `422` em payload inválido (token de funil ausente, trigger desconhecida, etc.).
- **Body limite:** 256 KiB (proteção contra abuso).
- **Timeout:** `httpserver` default já cobre; use case interno tem `context.WithTimeout(ctx, 10s)`.

### 6.4 Wiring em `cmd/server/server.go` (incremento)

Após `identityModule := identity.NewIdentityModule(...)`:

```go
billingModule := billing.NewBillingModule(cfg, o11y, dbManager)
if billingModule.WebhookRouter != nil {
    srv.RegisterRouters(billingModule.WebhookRouter)
}
```

### 6.5 Wiring em `cmd/worker/worker.go` (incremento)

Após criar `eventsDispatcher := events.NewDispatcher()`:

```go
identityModule := identity.NewIdentityModule(cfg, o11y, dbManager)
billingModule := billing.NewBillingModule(cfg, o11y, dbManager)

for _, reg := range identityModule.EventHandlers {
    if err := eventsDispatcher.Register(reg.EventType, reg.Handler); err != nil {
        return fmt.Errorf("worker: registrar handler identity: %w", err)
    }
}

jobs = append(jobs, billingModule.ReconciliationJob, billingModule.KiwifyEventsHousekeeper)
```

### 6.6 Eventos emitidos por billing

Type names estáveis e prefixados (também é o `event.Type` do outbox):

| Type | Aggregate | Quando | Payload (campos) |
| --- | --- | --- | --- |
| `billing.subscription.activated` | `Subscription` | `order_approved` cria sub | `subscription_id, funnel_token, plan_code, period_start, period_end, occurred_at` |
| `billing.subscription.renewed` | `Subscription` | `subscription_renewed` estende | `subscription_id, plan_code, previous_period_end, period_end, occurred_at` |
| `billing.subscription.past_due` | `Subscription` | `subscription_late` | `subscription_id, period_end, grace_end, occurred_at` |
| `billing.subscription.canceled` | `Subscription` | `subscription_canceled` | `subscription_id, period_end, occurred_at` |
| `billing.subscription.refunded` | `Subscription` | refund/chargeback | `subscription_id, occurred_at` |

Cada `events.Event` é struct concreto em `internal/billing/infrastructure/messaging/database/producers/events.go` implementando `events.Event` (`GetEventType()`, `GetPayload() any`). O publisher serializa para `json.RawMessage` e chama `outbox.Publisher.Publish` dentro da mesma transação do use case.

### 6.7 Schemas (esboço SQL)

```sql
-- 0004_create_billing_plans.up.sql
CREATE TABLE billing_plans (
    kiwify_product_id TEXT PRIMARY KEY,
    code              TEXT NOT NULL UNIQUE CHECK (code IN ('MONTHLY','QUARTERLY','ANNUAL')),
    duration_days     INTEGER NOT NULL CHECK (duration_days > 0),
    currency          TEXT NOT NULL DEFAULT 'BRL'
);
INSERT INTO billing_plans (kiwify_product_id, code, duration_days) VALUES
    ('<id-mensal>',     'MONTHLY',    30),
    ('<id-trimestral>', 'QUARTERLY',  90),
    ('<id-anual>',      'ANNUAL',    365);

-- 0005_create_billing_subscriptions.up.sql
CREATE TABLE billing_subscriptions (
    id                 UUID PRIMARY KEY,
    funnel_token       TEXT NOT NULL,
    user_id            UUID NULL,
    kiwify_order_id    TEXT NOT NULL,
    kiwify_subscription_id TEXT NULL,
    plan_code          TEXT NOT NULL REFERENCES billing_plans(code),
    status             TEXT NOT NULL CHECK (status IN ('TRIALING','ACTIVE','PAST_DUE','CANCELED_PENDING','EXPIRED','REFUNDED')),
    period_start       TIMESTAMPTZ NOT NULL,
    period_end         TIMESTAMPTZ NOT NULL,
    grace_end          TIMESTAMPTZ NULL,
    last_event_at      TIMESTAMPTZ NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX billing_subscriptions_user_active_uniq_idx
    ON billing_subscriptions (user_id)
    WHERE user_id IS NOT NULL
      AND status IN ('ACTIVE','PAST_DUE','CANCELED_PENDING');
CREATE UNIQUE INDEX billing_subscriptions_kiwify_order_uniq_idx
    ON billing_subscriptions (kiwify_order_id);
CREATE INDEX billing_subscriptions_funnel_token_idx ON billing_subscriptions (funnel_token);

-- 0006_create_billing_processed_events.up.sql
CREATE TABLE billing_processed_events (
    event_key   TEXT PRIMARY KEY,
    trigger     TEXT NOT NULL,
    recurso_id  TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    status      TEXT NOT NULL CHECK (status IN ('applied','superseded'))
);
CREATE INDEX billing_processed_events_recurso_idx ON billing_processed_events (recurso_id);

-- 0007_create_billing_kiwify_events.up.sql
CREATE TABLE billing_kiwify_events (
    envelope_id      TEXT PRIMARY KEY,
    trigger          TEXT NOT NULL,
    raw_body         JSONB NOT NULL,
    received_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at     TIMESTAMPTZ NULL,
    signature_status TEXT NOT NULL CHECK (signature_status IN ('valid','invalid','rotated'))
);
CREATE INDEX billing_kiwify_events_received_at_idx ON billing_kiwify_events (received_at);
CREATE INDEX billing_kiwify_events_trigger_idx     ON billing_kiwify_events (trigger);

-- 0008_create_billing_reconciliation_checkpoints.up.sql
CREATE TABLE billing_reconciliation_checkpoints (
    name      TEXT PRIMARY KEY,
    watermark TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 0009_create_identity_entitlements.up.sql
CREATE TABLE identity_entitlements (
    user_id         UUID PRIMARY KEY REFERENCES users(id),
    subscription_id UUID NOT NULL,
    status          TEXT NOT NULL,
    period_end      TIMESTAMPTZ NOT NULL,
    grace_end       TIMESTAMPTZ NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX identity_entitlements_subscription_id_idx ON identity_entitlements (subscription_id);

CREATE TABLE identity_entitlements_pending (
    subscription_id UUID PRIMARY KEY,
    funnel_token    TEXT NOT NULL,
    payload         JSONB NOT NULL,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX identity_entitlements_pending_funnel_token_idx ON identity_entitlements_pending (funnel_token);
```

## 7. Idempotência, deduplicação, ordering e reconciliação

### 7.1 Idempotência

Chave por trigger (ADR-005):

| Trigger | event_key |
| --- | --- |
| `order_approved` | `order_approved:{sale.id}` |
| `subscription_renewed` | `subscription_renewed:{subscription.id}:{subscription.updated_at_iso8601}` |
| `subscription_late` | `subscription_late:{subscription.id}:{subscription.updated_at_iso8601}` |
| `subscription_canceled` | `subscription_canceled:{subscription.id}` |
| `order_refunded` | `refund:{sale.id}` |
| `chargeback` | `refund:{sale.id}` (mesmo prefixo — refund ∪ chargeback) |

INSERT em `billing_processed_events` antes do mutation; `pgErrCode 23505` ou rowsAffected=0 ⇒ no-op silencioso (retorna 202).

### 7.2 Deduplicação no envelope

Não usar `envelope.id` (varia por retry, conforme observado na própria Banking API e prática Kiwify). Sempre derivar do `data`/`sale.id`/`subscription.id`. `envelope_id` é persistido em `billing_kiwify_events` para auditoria — duplicatas viram registros distintos (índice por `received_at` + `envelope_id` PK).

### 7.3 Ordering

- `last_event_at` armazenado em `billing_subscriptions`.
- Comparação no início do use case: `if payload.occurred_at <= sub.last_event_at && transition_is_regression { mark superseded; return 202 }`.
- Transições "regression" definidas em `transitions.go`:
  - `REFUNDED` é terminal → nunca regride a partir de `REFUNDED`; **sempre** vence chegando.
  - `EXPIRED` é determinado por tempo, não por evento.
  - `PAST_DUE` → `ACTIVE` exige novo `renewed`/`approved` com `occurred_at > last_event_at`.

### 7.4 Reconciliação (ADR-006)

```
ReconciliationJob.Run(ctx):
  checkpoint = repo.GetCheckpoint("kiwify_sales") // ex. 2026-06-05T14:00Z
  windowStart = checkpoint.Add(-15 * time.Minute)
  windowEnd   = time.Now().UTC()
  for page := 1; ; page++ {
      sales, hasMore := kiwifyClient.ListSalesUpdatedSince(ctx, windowStart, windowEnd, page)
      for _, s := range sales {
          // reaproveita o caminho idempotente do webhook
          if err := reconcileSale(ctx, s); err != nil { logError; continue }
      }
      if !hasMore { break }
  }
  repo.SetCheckpoint("kiwify_sales", windowEnd)
```

`reconcileSale` traduz uma sale Kiwify em um pseudo-evento `order_approved` (ou refund se `status == 'refunded|chargedback'`) e roteia para o use case correspondente. Como o `event_key` é determinístico (`order_approved:{sale.id}`), uma sale já processada pelo webhook é no-op.

**Trade-offs:** janela de 15min de overlap absorve clock skew Kiwify ↔ MeControla. Custo: ≤ 100 req/min (compatível com rate limit Kiwify).

## 8. Segurança e verificação de assinatura Kiwify

### 8.1 HMAC-SHA256 (ADR-002 — suposição material)

```go
// pseudo-código
func validate(rawBody []byte, header, secret string) bool {
    expected := hmac.New(sha256.New, []byte(secret))
    expected.Write(rawBody)
    sum := base64.StdEncoding.EncodeToString(expected.Sum(nil))
    return hmac.Equal([]byte(sum), []byte(header))
}
```

- Header preferido: `X-Kiwify-Signature` (base64). Fallback: query `signature`.
- Rotação: aceitar `secret_current` ∪ `secret_next` em paralelo durante janela explicit (ambos lidos do env).
- Logar em `billing_kiwify_events.signature_status` os 3 valores: `valid`, `invalid` (rejeitado), `rotated` (passou com `secret_next`).

### 8.2 Proteções adicionais

- Limite de body: 256 KiB; reject `413` acima.
- Content-Type: `application/json`; reject `415` se diverge.
- Sem PII em logs — payload bruto vive em `billing_kiwify_events` (JSONB); nunca em log.
- TLS terminado upstream (gateway); aplicação assume `https://`.
- Rate limit do endpoint webhook: omitido no MVP (Kiwify é fonte única e respeita janelas; abuso externo bloqueado por HMAC).

### 8.3 OAuth Kiwify

- `POST /v1/oauth/token` com `client_id`/`client_secret`. Cache `access_token` por `expires_in - KIWIFY_OAUTH_TOKEN_SAFETY_MARGIN` (default 600s).
- `x-kiwify-account-id` injetado em todo request.
- Rate limiter local: `golang.org/x/time/rate` com `KIWIFY_RATE_LIMIT_MAX_REQUESTS_PER_MIN/60` rps, burst `KIWIFY_RATE_LIMIT_BURST`. Bloqueia local antes de exceder.
- Retry: 5xx + 429 → backoff exponencial (`KIWIFY_HTTP_RETRY_MAX_ATTEMPTS`, `KIWIFY_HTTP_RETRY_BACKOFF`). 4xx demais → erro imediato.

### 8.4 LGPD

`billing_kiwify_events` contém PII (telefone, e-mail do customer). Retention 90d (housekeeping). Anonimização programada fora do MVP (E4).

## 9. Observabilidade, erros e operação

### 9.1 Logs (slog estruturado)

- `billing.webhook.received` — `envelope_id, trigger, signature_status, duration_ms`.
- `billing.webhook.rejected` — `reason, status_code`.
- `billing.subscription.transition` — `subscription_id, from, to, trigger, event_key`.
- `billing.processed_event.duplicate` — `event_key, trigger, original_applied_at`.
- `billing.event.superseded` — `event_key, subscription_id, occurred_at, last_event_at`.
- `billing.reconciliation.run` — `window_start, window_end, sales_seen, corrections_applied, duration_ms`.
- `billing.kiwify_client.error` — `endpoint, status, retry_attempt`.
- `identity.entitlement.projected` — `user_id, subscription_id, status`.
- `identity.entitlement.pending` — `subscription_id, funnel_token` (sem user_id, sem PII).

PII (telefone, e-mail, CPF) **nunca** em log; apenas hash truncado quando inevitável.

### 9.2 Métricas (Prometheus)

| Métrica | Tipo | Labels | Para |
| --- | --- | --- | --- |
| `billing_webhooks_received_total` | counter | `trigger, signature_status` | M-04, segurança |
| `billing_processing_latency_seconds` | histogram | `trigger, status` | M-04 (p95 ≤ 30s) |
| `billing_subscription_transitions_total` | counter | `from, to, trigger` | M-01, M-02, M-03 |
| `billing_processed_event_duplicates_total` | counter | `trigger` | RF-11 (auditoria) |
| `billing_event_superseded_total` | counter | `trigger` | RF-12 |
| `billing_reconciliation_corrections_total` | counter | `correction_type` | RF-18 |
| `billing_kiwify_client_requests_total` | counter | `endpoint, status` | observabilidade externa |
| `billing_kiwify_oauth_token_refreshes_total` | counter | — | health do OAuth |

### 9.3 Erros tipados (R5.10)

```go
var (
    ErrFunnelTokenMissing       = errors.New("billing: funnel token ausente no payload")
    ErrPlanNotFound             = errors.New("billing: plano não encontrado para product_id")
    ErrEventAlreadyProcessed    = errors.New("billing: evento já processado")
    ErrEventSuperseded          = errors.New("billing: evento superseded por estado mais recente")
    ErrConcurrentActiveSub      = errors.New("billing: usuário já possui assinatura ativa")
    ErrInvalidSignature         = errors.New("billing: assinatura HMAC inválida")
    ErrUnknownTrigger           = errors.New("billing: trigger desconhecida")
)
```

`errors.Is` no handler decide HTTP status code. Erros downstream wrappados com `fmt.Errorf("ctx: %w", err)`.

### 9.4 Runbook mínimo (operação)

| Sintoma | Diagnóstico | Ação |
| --- | --- | --- |
| `billing.webhook.rejected` em massa | `signature_status='invalid'` | Confirmar `KIWIFY_WEBHOOK_SECRET`; se rotação recente, popular `_NEXT`. |
| Spike `billing_processed_event_duplicates_total` | Kiwify reenviando | Esperado; verificar se há erro real upstream. |
| `billing_reconciliation_corrections_total` > 0 sustained | Webhook perdido | Investigar disponibilidade do endpoint nas janelas; verificar logs upstream. |
| `outbox` rows `failed` para `billing.subscription.*` | Projector identity falhando | Investigar `identity.entitlement.*` logs; rerun via reset de `outbox` row para `pending`. |

## 10. Estratégia de testes

### 10.1 Unitários

**Cobertura mínima por componente:**

| Componente | Casos críticos |
| --- | --- |
| `domain/entities/subscription.go` | Construtor recusa funnel_token vazio. Transições aplicadas seguem `transitions.go`. `last_event_at` atualizado corretamente. Refund é terminal. |
| `domain/services/transitions.go` | Tabela completa: 6x6 transições, terminais, regressões rejeitadas. |
| `application/usecases/process_sale_approved.go` | (a) idempotência: 2ª chamada retorna `ErrEventAlreadyProcessed`. (b) token vazio: `ErrFunnelTokenMissing`. (c) plano inexistente: `ErrPlanNotFound`. (d) sucesso: estado=ACTIVE, period_end correto, evento outbox emitido. |
| `application/usecases/process_subscription_late.go` | grace_end = late_at + 3*24h. Ordering: late com occurred_at anterior é superseded. |
| `application/usecases/process_refund_or_chargeback.go` | Mesmo event_key entre os dois triggers. Refund vence mesmo se sub estiver REFUNDED já (no-op idempotente). |
| `infrastructure/http/server/middleware/hmac_signature.go` | Aceita header valido; rejeita inválido. `hmac.Equal` (constant time). Aceita `secret_next` durante rotação. |
| `infrastructure/http/server/handlers/kiwify_webhook_handler.go` | 202 em sucesso. 401 mismatch. 422 token ausente. 415 content-type. Body excedendo limite → 413. |
| `infrastructure/messaging/database/producers/subscription_event_publisher.go` | Cada `PublishX` chama `outbox.Publisher.Publish` com payload JSON válido e aggregate correto. |
| `infrastructure/http/client/kiwify/client.go` | Cache do token (não refaz OAuth dentro da janela). Rate limit aplicado. Retry em 5xx; abort em 4xx demais. Inclui `x-kiwify-account-id`. |
| `infrastructure/jobs/handlers/reconciliation_job.go` | Checkpoint avançado apenas em sucesso. Paginação. Reuso do use case do webhook. |
| `internal/identity/infrastructure/messaging/database/consumers/subscription_event_projector.go` | `activated` com user resolvido → entitlements; sem user → entitlements_pending. Idempotente por subscription_id. |
| `internal/identity/infrastructure/repositories/postgres/entitlement_repository.go` | UPSERT correto. Lookup por user_id. |

Mocks **apenas** para SDK Kiwify, `outbox.Publisher`, `events.Dispatcher` e `NotificationSender`. **Nunca** mockar Postgres (regra firmada).

### 10.2 Integração

Critérios já satisfeitos (a) e (b) da checklist do template: webhook+UoW+outbox+projector cruzam fronteiras de IO; tests unitários cedem ao primeiro bug não-trivial de schema. **Adotar** `testcontainers-go` (Postgres) com build tag `//go:build integration`.

| Suite | Escopo |
| --- | --- |
| `webhook_to_outbox_test.go` | POST → middleware → handler → use case → DB → `platform_outbox_events`. Verifica row outbox criada na mesma transação. |
| `outbox_to_projector_test.go` | Insere row outbox `billing.subscription.activated` → roda dispatcher → verifica `identity.entitlements_pending`. |
| `reconciliation_e2e_test.go` | Sobe stub HTTP simulando `GET /v1/sales` → roda job → confirma checkpoint atualizado e estados aplicados. |
| `idempotency_replay_test.go` | Repete webhook 5x → exatamente 1 transição, 4 no-ops, métricas duplicates contam. |
| `out_of_order_test.go` | Sequência `renewed` → `late` (stale) → `late` (fresh) → estado correto. |

### 10.3 E2E

Diferido para hardening pós-MVP (E4). MVP cobre via integração.

## 11. Riscos, trade-offs e itens fora do MVP

### 11.1 Riscos técnicos

| Risco | Mitigação |
| --- | --- |
| **L-01:** algoritmo HMAC suposto. | ADR-002 com plano de validação em sandbox antes da execução. Fallback: rejeitar todos os webhooks até confirmação. |
| **L-02:** `updated_at_start_date` na Kiwify pode não filtrar perfeitamente. | Janela de 15min de overlap + idempotência total absorvem subnotificação; cobertura via teste de integração com stub. |
| **L-03:** campo do funnel token (`tracking.s1` vs `src`) precisa confirmação empírica. | Decisão isolada em `kiwify.payload_mapping.go`; trocar campo é diff cirúrgico. |
| Falha de projector identity persistente | Outbox marca `failed`; alerta operacional. Reset manual revoga e reenvia. |
| Spike de webhook (DoS) | Body limit + HMAC barrato antes de parse + httpserver default rate. Sem WAF no MVP. |
| Falha cascata no `events.Dispatcher` in-process | Trade-off aceito: cross-module same-binary é simples; falha cobre via outbox retry + manual reprocess. |
| `subscription_renewed` chega antes de `order_approved` (split) | Placeholder ACTIVE criado; subsequente `approved` é no-op idempotente. Cobertura por teste. |

### 11.2 Trade-offs

| Decisão | Trade-off |
| --- | --- |
| Read model duplicado em `identity.entitlements` | +1 tabela; -1 cross-module query no hot path. |
| State machine explícita | Mais código que "last-event-wins"; tradeoff: correção em ordering. |
| Reconciliação via `sales` (não `subscriptions`) | Necessidade de tradução sale→sub; tradeoff: única API documentada. |
| HMAC suposto (sem doc oficial) | Risco operacional; tradeoff: bloquear MVP é pior. ADR-002 + sandbox antes da execução. |
| `kiwify_events` persistente | +storage; -dependência de logs efêmeros para forensics. Retention 90d. |

### 11.3 Explicitamente fora do MVP (alinhado ao PRD)

- Multi-provider (Hotmart/Stripe/etc.) — novo PRD.
- Trial gratuito.
- `/cancelar` via WhatsApp (cancela na Kiwify).
- Override admin de entitlement.
- Sweep retroativo 90d e dashboard MRR/churn.
- Anonimização LGPD programada (vai em E4).
- Plano família / múltiplas subs ativas.
- Garantia forte de notificação (replay com retry forte).
- Painel admin web para suporte.
- `is_admin` no agregado User (proibido por E1).
- Whitelist comandos (vai em E3).
- Rate limit por usuário no bot.

### 11.4 Itens condicionados ao E3 / E4

| Item | Status | Dependência |
| --- | --- | --- |
| `identity.entitlements_pending` → `identity.entitlements` (bind por token) | Aguardando E3 | Evento `identity.user.bound_to_token` (a ser definido em E3). |
| `NotificationSender` implementação concreta WhatsApp | Stub no-op no MVP | E3/E5 implementa. |
| Sweep retroativo 90d full | Adiar | E4. |
| Replay forte de notificação | Adiar | E4. |

## 12. ADRs necessárias

| ADR | Decisão |
| --- | --- |
| [ADR-001](./adr-001-kiwify-public-api-vs-banking.md) | Public API (não Banking) para webhooks de assinatura. |
| [ADR-002](./adr-002-hmac-sha256-webhook-auth.md) | HMAC-SHA256 sobre raw_body com token compartilhado (suposição material). |
| [ADR-003](./adr-003-outbox-to-events-dispatcher-cross-module.md) | Cross-module billing→identity via outbox + events.Dispatcher in-process. |
| [ADR-004](./adr-004-billing-owns-subscription-identity-read-model.md) | Billing dono de Subscription; identity mantém read model. |
| [ADR-005](./adr-005-idempotency-and-ordering-state-machine.md) | Idempotência por event_key composto + state machine + last_event_at vector. |
| [ADR-006](./adr-006-reconciliation-sales-updated-at-window.md) | Reconciliação via GET /v1/sales com janela updated_at_start_date + checkpoint. |
| [ADR-007](./adr-007-plans-table-seed.md) | Plans em tabela (não hardcoded), seedados via migration. |
| [ADR-008](./adr-008-kiwify-raw-events-90d-retention.md) | Persistir kiwify_events com retention 90d (housekeeping). |

## 13. Matriz requisito → decisão → teste

| Requisito | Decisão (§) | Teste-âncora |
| --- | --- | --- |
| RF-01 | §1, §4 (`provider=kiwify` hardcoded) | unit: handler rejeita trigger desconhecida. |
| RF-02 | §6.7 (`billing_plans` seed) | integ: lookup `MONTHLY/QUARTERLY/ANNUAL`. |
| RF-03 | §5.1 etapa 6.3 (`ErrFunnelTokenMissing`) | unit: payload sem token → 422; sem subscription criada. |
| RF-04 | `entities/subscription.go` (enum), `domain/entitlement.go` (E1) | unit: enum espelhada (cross-check). |
| RF-05/08 | §4 use cases + `domain.IsEntitled` (E1) | unit existente em E1 + integ projector. |
| RF-06 | §7.3 + ProcessLate | unit: `grace_end = late_at + 3d`. |
| RF-07 | ProcessCanceled mantém `period_end` | unit: cancel não muda `period_end`. |
| RF-09 | ProcessRefundOrChargeback terminal | unit: refund após cancel → REFUNDED. |
| RF-10 | §5 + §6.6 | unit por trigger + integ. |
| RF-11 | §7.1 (`processed_events`) | integ replay 5x → 1 transição + 4 duplicates. |
| RF-12 | §7.3 (state machine + last_event_at) | integ out-of-order. |
| RF-13/14 | `domain.IsEntitled` + `EntitlementReader` | unit existente E1 + integ. |
| RF-15 | Read model + cache LRU opcional | bench (informativo, não bloqueio). |
| RF-16 | §6.7 (`user_id NULL`, `funnel_token NOT NULL`) | integ: webhook → entitlements_pending. |
| RF-17 | §6.7 (índice único parcial) | integ: 2ª sub ACTIVE para mesmo user falha. |
| RF-18 | §7.4 + ADR-006 | integ reconciliation_e2e. |
| RF-19 | §11.3 (fora do MVP) | nenhum. |
| RF-20 | `NotificationSender` interface stub | unit: handler best-effort não falha em erro. |
| RF-21 | §11.3 (fora) | nenhum. |
| M-01..M-04 | §9.2 métricas | dashboard pós-deploy. |

---

## Conformidade com Padrões

- **AGENTS.md §"Padrao Obrigatorio de Modulo":** `NewBillingModule(...) BillingModule`; struct concreta com campos nomeados; wiring `repository → use case → handler → router/job`. Sem `NewModule(opts...)`, sem `WithDatabase(...)`.
- **AGENTS.md §"Layout Obrigatorio por Modulo":** estrutura `application/{dtos/usecases/interfaces} + domain/{entities/valueobjects/services} + infrastructure/{http,jobs,messaging,repositories}` respeitada integralmente.
- **AGENTS.md §"Worker, HTTP Outbound e Outbox":** todo evento usa `outbox.Publisher`; toda chamada Kiwify usa `internal/platform/httpclient`; job de reconciliação via `worker.Job`.
- **AGENTS.md §"Plataforma Compartilhada":** capacidades reutilizáveis (httpclient, outbox, events, worker) ficam em `internal/platform/`; sem duplicação local.
- **`go-implementation` R0–R7:** zero `init()`; métodos de struct em domínio/aplicação/infra; enums com `iota+1` quando aplicável; erros via `errors.New`/`fmt.Errorf("ctx: %w", err)`; `errors.Join` quando agrega; sem `panic` em produção; sem `var _ Interface = (*Type)(nil)`; sem clock abstrato (`time.Now().UTC()` inline); `log/slog`; `any` em vez de `interface{}`; `context.Context` em toda fronteira de IO; interface no consumidor (todas as interfaces em `application/interfaces/`).
- **Zero comentários em Go** (CLAUDE.md): comentários **proibidos** em todo código novo; identificadores nomeados expressivos são a documentação.
- **`.claude/rules/governance.md`:** evidência ancorada em PRD + repositório + docs oficiais Kiwify; suposições marcadas como tal (HMAC, campo do token); sem comportamento inferido.

### Riscos Conhecidos

Listados em §11.1; recapitulação:

- L-01 (HMAC): suposição material — confirmação em sandbox antes da execução; ADR-002.
- L-02 (`updated_at_start_date`): janela de 15min + idempotência compensam.
- L-03 (campo funnel token): isolado em mapping; troca cirúrgica.
- Falha persistente do projector identity: runbook §9.4.
- DoS do endpoint webhook: HMAC + body limit; sem WAF no MVP.

### Arquivos Relevantes e Dependentes

**Já existentes (leitura/wiring):**
- `cmd/server/server.go`, `cmd/worker/worker.go`
- `configs/config.go` (KiwifyConfig, BillingConfig)
- `internal/identity/module.go`, `internal/identity/domain/entitlement.go`, `internal/identity/infrastructure/messaging/database/consumers/doc.go`
- `internal/platform/{events,outbox,worker,httpclient,id}` (todos consumidos)
- `migrations/0001..0003_*`

**A criar (ordem de implementação em §"Sequenciamento" abaixo):**

Billing:
- `internal/billing/module.go`
- `internal/billing/domain/entities/subscription.go` + `_test.go`
- `internal/billing/domain/valueobjects/{status.go,plan.go,funnel_token.go}` + tests
- `internal/billing/domain/services/transitions.go` + tests
- `internal/billing/application/interfaces/{subscription_repository,processed_event_repository,kiwify_event_repository,plan_repository,reconciliation_checkpoint_repository,kiwify_client,notification_sender,subscription_event_publisher,repository_factory}.go`
- `internal/billing/application/dtos/{input,output}/*.go`
- `internal/billing/application/usecases/{process_sale_approved,process_subscription_renewed,process_subscription_late,process_subscription_canceled,process_refund_or_chargeback,reconcile_subscriptions}.go` + tests + mocks
- `internal/billing/infrastructure/repositories/postgres/{subscription,processed_event,kiwify_event,plan,reconciliation_checkpoint}_repository.go` + integ tests
- `internal/billing/infrastructure/repositories/factory.go`
- `internal/billing/infrastructure/http/client/kiwify/{client,auth,ratelimit,models}.go` + tests
- `internal/billing/infrastructure/http/server/{router,routes}.go`
- `internal/billing/infrastructure/http/server/handlers/kiwify_webhook_handler.go` + tests
- `internal/billing/infrastructure/http/server/middleware/{hmac_signature,raw_body_buffer}.go` + tests
- `internal/billing/infrastructure/messaging/database/producers/{subscription_event_publisher,events}.go` + tests
- `internal/billing/infrastructure/jobs/handlers/{reconciliation_job,kiwify_events_housekeeping_job}.go` + tests

Identity (incremento):
- `internal/identity/application/interfaces/entitlement_repository.go`
- `internal/identity/application/usecases/decide_user_entitlement.go` + test
- `internal/identity/application/dtos/output/entitlement_decision.go`
- `internal/identity/infrastructure/repositories/postgres/entitlement_repository.go` + integ test
- `internal/identity/infrastructure/messaging/database/consumers/subscription_event_projector.go` + test
- `internal/identity/module.go` (modificação)

Migrations:
- `migrations/0004_create_billing_plans.{up,down}.sql`
- `migrations/0005_create_billing_subscriptions.{up,down}.sql`
- `migrations/0006_create_billing_processed_events.{up,down}.sql`
- `migrations/0007_create_billing_kiwify_events.{up,down}.sql`
- `migrations/0008_create_billing_reconciliation_checkpoints.{up,down}.sql`
- `migrations/0009_create_identity_entitlements.{up,down}.sql`

Entrypoints (modificação):
- `cmd/server/server.go` — incrementar com `billing.NewBillingModule` + register router.
- `cmd/worker/worker.go` — incrementar com `identity.NewIdentityModule` + `billing.NewBillingModule` + register handlers no `events.Dispatcher` + append jobs.

Config (modificação leve):
- `configs/config.go` — adicionar `KIWIFY_WEBHOOK_SECRET_NEXT` (rotação) e `BILLING_PAST_DUE_TEMPLATE` (se a equipe optar por adicionar agora; opcional).

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Migrations (0004–0009)** — fundação de schema antes de qualquer Go.
2. **Domain billing** (entities, valueobjects, services/transitions) — núcleo puro testável sem IO.
3. **Application interfaces billing** — contratos no consumidor.
4. **Repositories billing (postgres)** — implementações + integ.
5. **Outbox producer billing** (events + publisher) — emissão via outbox.
6. **Use cases billing** — todos os 5 + reconcile.
7. **Kiwify HTTP client** — OAuth, rate limit, retry.
8. **HTTP server billing** — middleware HMAC + handler + router.
9. **Jobs billing** — reconciliation + housekeeping.
10. **Module billing** — `NewBillingModule` reúne tudo.
11. **Identity entitlement** — interface, repo, use case, projector.
12. **Identity module** (modificação) — expor handlers + reader.
13. **Wiring `cmd/server`** — adiciona billing.
14. **Wiring `cmd/worker`** — adiciona identity, billing, registra handlers, adiciona jobs.
15. **Integ end-to-end** — webhook→outbox→projector, reconciliation_e2e.
16. **Validação proporcional** — `gofmt`, `go vet`, `go test -race ./...`, `golangci-lint run`.

### Dependências Técnicas

- Postgres disponível (Postgres 14+ por uso de JSONB, índices parciais e `pgErrCode 23505`).
- Conta Kiwify Public API com `client_id`/`client_secret` ativos e `account_id` em env.
- Webhook configurado na Kiwify para `POST https://<host>/api/v1/billing/webhooks/kiwify` com os 6 triggers (campo `token` populado com `KIWIFY_WEBHOOK_SECRET`).
- `WhatsApp Business` sender adiado (stub no-op no MVP).
- **E1 implementado em produção** (premissa do PRD para execução, não para techspec).

## Monitoramento e Observabilidade

Resumo (detalhes em §9):

- Métricas Prometheus expostas via httpserver default `/metrics`.
- Logs via `slog` (já configurado por `cmd/server` e `cmd/worker`).
- Dashboards Grafana a criar (item operacional, não bloqueia execução):
  - **Billing — Webhooks:** taxa de recebimento por trigger, taxa de rejeição por signature_status, latência p50/p95/p99.
  - **Billing — Transições:** `from→to` por trigger; M-01/M-02/M-03 derivados.
  - **Billing — Reconciliação:** runs por hora, correções aplicadas, duração.
  - **Billing — Outbox:** rows pending/failed para `billing.subscription.*`.

Alertas mínimos (configurar em runtime):

- `billing_processing_latency_seconds{quantile="0.95"} > 30s` por 5min → M-04 violada.
- `rate(billing_webhooks_received_total{signature_status="invalid"}[5m]) > 0.1` → possível ataque ou rotação errada.
- `outbox` rows `failed` para tipos `billing.subscription.*` > 0 sustained → projector down.

## Considerações Técnicas

### Decisões Chave

Cada decisão material foi materializada em ADR separada (§12). Resumo:

- **ADR-001:** Public API Kiwify (não Banking).
- **ADR-002:** HMAC-SHA256 + token compartilhado (suposição material; plano de validação).
- **ADR-003:** Outbox → events.Dispatcher in-process para cross-module.
- **ADR-004:** Billing dono do Subscription; identity mantém read model.
- **ADR-005:** Idempotência (event_key composto) + state machine + last_event_at.
- **ADR-006:** Reconciliação via GET /v1/sales com checkpoint+overlap.
- **ADR-007:** Plans em tabela seedada.
- **ADR-008:** Persist `kiwify_events` raw + retention 90d.

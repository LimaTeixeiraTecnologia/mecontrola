<!-- spec-hash-prd: 3836b94a299ee31b4c11e530b2d28022d302dd5225799e7bd46710ce3040d950 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Billing Pipeline

## Resumo Executivo

Esta techspec materializa o módulo `internal/billing/` como pipeline transacional de cobrança recorrente, alinhada ao bundle `consolidacao-core`, ao PRD `prd-billing-pipeline` e à governança `AGENTS.md`. Entrega quatro fluxos coesos: ingresso de webhook Kiwify, processamento por máquina de estados canônica, decisão de entitlement com cache in-memory, e reconciliação horária. O módulo segue layout hexagonal obrigatório (`domain/`, `application/`, `infrastructure/` com sub-pastas por responsabilidade) e reusa toda a fundação técnica já presente em `internal/platform/` — `outbox.Publisher`/`Dispatcher`, `database.UnitOfWork[T]`, `observability.mask`, `events.Bus`, `clock.Clock`.

Decisões materiais (10 ADRs, ver "Considerações Técnicas"): outbox event carrega apenas pointer `{webhook_event_id, provider}` mantendo `webhook_events` como single source of truth (ADR-001); HTTP routing inaugura padrão `RouteRegistrar` em `internal/platform/http` consumível por todos os módulos (ADR-002); scheduler de reconciliação e anonimização é Subsystem próprio em `infrastructure/scheduler` usando `robfig/cron/v3` já presente em `go.mod` (ADR-003); cache de entitlement usa `hashicorp/golang-lru/v2/expirable` adicionando uma única dependência direta (ADR-004); agregado `Subscription` implementa o contrato `identity.domain.services.Subscription` sem dependência circular via interface segregation (ADR-005); `BillingProvider.VerifySignature` é hexagonal com troca de implementação token → HMAC sem mudança de RF (ADR-006); retenção de `webhook_events` é two-tier (365d íntegro + anonimização irreversível em-place por job diário — ADR-007); OAuth Kiwify cacheia token em memória com TTL = `expires_in − 5min` e re-auth em 401 (ADR-008); `BillingEventProcessor` é único mutador via `outbox.Handler` registrado, idempotente por `billing_event_applications` (ADR-009); state machine é domain service stateless explícito com transições exaustivas (ADR-010).

Object Calisthenics aplicado seletivamente: VOs encapsulam primitivos (PlanCode, BillingPeriod, ExternalEventID, OrderID, MoneyBRL), agregado `Subscription` expõe comportamento (`ApplyEvent`, `Activate`, `MarkPastDue`, `Refund`) ao invés de getters mecânicos, early-return em todos os métodos críticos, sem `else` desnecessário, switch exaustivo cobrindo 100% das transições. Comentários no código são reduzidos ao mínimo necessário (decisões não-óbvias, invariantes ocultas, contratos públicos não derivados do nome).

## Arquitetura do Sistema

### Visão Geral dos Componentes

Layout físico obrigatório (sub-pastas conforme AGENTS.md "Layout Obrigatório por Módulo"):

```
internal/billing/
├── AGENTS.md                                          # novo (governance local)
├── README.md                                          # novo
├── doc.go                                             # novo
├── domain/
│   ├── doc.go
│   ├── entities/
│   │   └── subscription.go                            # Subscription aggregate + SubscriptionID
│   ├── valueobjects/
│   │   ├── plan_code.go                               # VO MONTHLY|QUARTERLY|ANNUAL
│   │   ├── billing_period.go                          # VO encapsulando duration por plano
│   │   ├── subscription_status.go                     # enum 6 estados (iota+1)
│   │   ├── canonical_event_type.go                    # enum 6 tipos canônicos
│   │   ├── transition_reason.go                       # VO razão da transição
│   │   ├── external_event_id.go                       # VO id/order.id/hash cascade
│   │   ├── external_subscription_id.go                # VO opaque string
│   │   └── money_brl.go                               # VO valor em centavos
│   ├── services/
│   │   ├── state_machine.go                           # transições canônicas (stateless)
│   │   ├── canonical_event.go                         # VO struct canônica do evento de provider
│   │   └── canonical_subscription.go                  # VO struct canônica de subscription
│   └── errors.go                                      # sentinelas tipados
├── application/
│   ├── doc.go
│   ├── dtos/
│   │   ├── input/
│   │   │   ├── ingest_webhook_input.go
│   │   │   ├── process_event_input.go
│   │   │   ├── check_entitlement_input.go
│   │   │   └── anonymize_input.go
│   │   └── output/
│   │       ├── ingest_webhook_output.go
│   │       ├── entitlement_decision.go
│   │       ├── reconciliation_report.go
│   │       └── anonymization_report.go
│   ├── interfaces/
│   │   ├── subscription_repository.go                 # port persistência Subscription
│   │   ├── webhook_event_repository.go                # port webhook_events + applications
│   │   ├── billing_provider.go                        # port adapter Kiwify (VerifySignature/ParseEvent/FetchSubscription)
│   │   ├── entitlement_cache.go                       # port cache LRU
│   │   ├── id_generator.go                            # port UUID v4
│   │   └── user_resolver.go                           # port wrapper de identity.UserRepository (segregação)
│   └── usecases/
│       ├── ingest_kiwify_webhook.go                   # F-1: assina + dedup + publica outbox
│       ├── process_billing_event.go                   # F-4: outbox.Handler único mutador
│       ├── check_entitlement.go                       # F-6: cache hit ou Postgres lookup
│       ├── reconcile_subscriptions.go                 # F-7: tick horário
│       └── anonymize_webhook_events.go                # F-2/D-08: job diário
└── infrastructure/
    ├── doc.go
    ├── repositories/
    │   └── postgres/
    │       ├── subscription_repository.go             # PgxSubscriptionRepository
    │       ├── webhook_event_repository.go            # PgxWebhookEventRepository (dedup, apply, anonymize)
    │       ├── queries.go                             # SQL constants
    │       └── mapper.go                              # row → entities (revalida invariantes)
    ├── http/
    │   ├── server/
    │   │   ├── kiwify_webhook_handler.go              # POST /webhooks/kiwify
    │   │   └── route_registrar.go                     # impl RouteRegistrar (ADR-002)
    │   └── client/
    │       └── kiwify/
    │           ├── client.go                          # HTTP client base com OTel + rate limit
    │           ├── oauth.go                           # token cache + re-auth (ADR-008)
    │           ├── signature_verifier.go              # impl token constant-time (ADR-006)
    │           ├── adapter.go                         # BillingProvider impl
    │           └── payload_mapper.go                  # tracking.src cascata + customer.mobile
    ├── cache/
    │   └── entitlement_lru.go                         # hashicorp/golang-lru/v2/expirable wrapper (ADR-004)
    ├── outbox/
    │   ├── handler.go                                 # registra BillingEventProcessor no Registry
    │   └── event_payload.go                           # struct {WebhookEventID, Provider} (ADR-001)
    ├── scheduler/
    │   ├── subsystem.go                               # Subsystem único com 2 jobs (ADR-003)
    │   ├── reconciliation_job.go
    │   └── anonymization_job.go
    └── id/
        └── uuid_generator.go                          # google/uuid v4 adapter (alinhado com identity)
```

Artefatos transversais novos ou alterados:

- `internal/platform/http/server.go` (alterado) — introduz `RouteRegistrar` interface e slice em `Deps`; `NewServer` chama `registrar.Register(router)` antes de devolver (ADR-002).
- `cmd/server/server.go` (alterado) — `Bootstrap` injeta `[]RouteRegistrar` com `billing.KiwifyRouteRegistrar` (e futuros).
- `internal/platform/runtime/billing_subsystem.go` (novo) — lazy subsystem que constrói repos, adapter Kiwify, registra handler no `outbox.Registry`, monta scheduler.
- `migrations/0009_billing_schema.up.sql` + `.down.sql` (novos) — `webhook_events`, `subscriptions`, `billing_event_applications`, `billing_plans`.
- `migrations/0010_billing_plans_seed.up.sql` + `.down.sql` (novos) — insere os 3 planos fixos (D-03 do PRD).
- `mockery.yml` (alterado) — declara todas as interfaces do `application/interfaces/billing`.
- `.golangci.yml` (alterado) — adiciona `billing-no-identity-infrastructure` e reforça `domain-no-infrastructure` no novo path.
- `configs/config.go` (alterado) — `KiwifyConfig` + `BillingConfig` (ver "Configuração").
- `go.mod` (alterado) — adiciona `github.com/hashicorp/golang-lru/v2` como dependência direta.

Fluxo de dependências (validado por `depguard`):

```
[billing/domain]  ←── importa stdlib + identity/domain/services (interface Subscription) + identity/domain/valueobjects (WhatsAppNumber)
   ↑
[billing/application] ── importa billing/domain + identity/application/interfaces (UserRepository) + internal/platform/clock|errors|outbox
   ↑
[billing/infrastructure] ── importa billing/application + billing/domain + internal/platform/* + identity/domain (apenas para tipos)
```

Regra cross-module enforçada por `depguard` (nova):
- `billing-no-identity-infrastructure`: billing pode importar `identity/domain` e `identity/application` (interfaces e tipos), nunca `identity/infrastructure`.

### Fluxo de Dados (sequência por caso de uso)

**F-1 → F-4 (Ingresso e processamento):**

```
Kiwify POST /webhooks/kiwify
  → http.KiwifyWebhookHandler
    → IngestKiwifyWebhookUseCase
       (1) BillingProvider.VerifySignature(raw, headers, secret)
       (2) externalEventID = extractCascade(raw)  // id → order.id → sha256(raw)
       (3) UoW: INSERT webhook_events ON CONFLICT (provider, external_event_id) DO NOTHING
                       → se 0 rows → return 204
                       → outbox.Publisher.Publish(tx, Event{
                              Type:      "billing.kiwify.received",
                              AggregateType: "webhook_event",
                              AggregateID:   webhookEventID.String(),
                              Payload:       {webhook_event_id, provider},
                          })
       (4) return 200
  → outbox.Dispatcher (tick 500ms, batch 50) ClaimReady
    → ProcessBillingEventUseCase (outbox.Handler)
       (1) decode payload {webhook_event_id, provider}
       (2) raw = WebhookEventRepository.FindRawPayload(webhookEventID)
       (3) canonicalEvent = BillingProvider.ParseEvent(raw)  // mapeia kiwify → canonical
       (4) UoW transacional:
            (a) user = UserResolver.UpsertByWhatsAppNumber(ctx, canonical.Customer.WhatsApp)
            (b) sub = SubscriptionRepository.FindActiveByUserIDForUpdate(ctx, user.ID()) // SELECT ... FOR UPDATE; pode ser nil
            (c) sub, transition = stateMachine.Apply(sub, canonical, user.ID(), now)
            (d) se transition.IsNoop() (occurred_at < sub.LastEventAt OR mesma transição) → return sem persistir
            (e) recorded = INSERT billing_event_applications (event_id, subscription_id) ON CONFLICT DO NOTHING
                → se !recorded → return sub  // idempotente: evento já aplicado em retry anterior
            (f) SubscriptionRepository.Upsert(ctx, sub)
            (g) UPDATE webhook_events SET processed_at = NOW() WHERE id = $webhookEventID
       (5) EntitlementCache.Invalidate(user.ID())  // síncrono local
       (6) events.Bus.Publish(StateChangedEvent{...})  // volátil
```

**F-6 (Decisão de entitlement, hot path):**

```
CheckEntitlementUseCase.Execute(userID)
  → EntitlementCache.Get(userID)  // RWMutex read lock + LRU expirable
       → hit → return Decision (< 5ms p99)
       → miss:
            sub = SubscriptionRepository.FindActiveByUserID(userID)
            decision = identity.EntitlementChecker.IsEntitled(sub, clock.Now())
            ttl = min(sub.PeriodEnd() - clock.Now(), 5min)
            EntitlementCache.Set(userID, decision, ttl)
            return decision
```

**F-7 (Reconciliação horária):**

```
scheduler.ReconciliationJob.Tick (robfig/cron @hourly)
  → ReconcileSubscriptionsUseCase.Execute(ctx, batchSize=200)
     loop até esgotar cursor:
       batch = SubscriptionRepository.ListByStatusInBatch(ctx, [ACTIVE,PAST_DUE], cursor, batchSize)
       for sub in batch:
         remote = BillingProvider.FetchSubscription(ctx, sub.ExternalSubscriptionID())
         if remote.Status != sub.Status() || remote.PeriodEnd != sub.PeriodEnd():
             // publica evento sintético no outbox via mesmo Publisher
             outbox.Publisher.Publish(tx, Event{
                 Type:    "billing.kiwify.received",  // mesma trilha de processamento!
                 Payload: { webhook_event_id: <novo sintético>, provider: "kiwify_reconcile" },
             })
             // INSERT webhook_events ... event_type='subscription.reconciled' SOURCE=remote_dump
         metrics.RateLimit.Wait(100/min)
```

**F-2 (Anonimização):**

```
scheduler.AnonymizationJob.Tick (robfig/cron @daily)
  → AnonymizeWebhookEventsUseCase.Execute(ctx, batchSize=500)
     batch = WebhookEventRepository.ListPendingAnonymization(ctx, 365days, batchSize)
     for row in batch:
       newPayload = strip(row.Payload, redactPaths)  // jsonpath-based
       UPDATE webhook_events SET payload = newPayload, anonymized_at = NOW() WHERE id = row.id
     metric.RecordAnonymized(len(batch))
```

## Design de Implementação

### Interfaces Chave

#### `internal/platform/http/server.go` — Reuso de `chiserver.Router` (ADR-002)

Investigação ao codebase do `devkit-go` (`pkg/http_server/chi_server/router.go`) revelou que o padrão já existe oficialmente:

```go
// devkit-go/pkg/http_server/chi_server/router.go (existente)
package chiserver

import "github.com/go-chi/chi/v5"

type Router interface {
    Register(router chi.Router)
}

// (server.go)
func (s *Server) RegisterRouters(routers ...Router) *Server
func (s *Server) RegisterHandler(method, path string, h Handler, mws ...Middleware) *Server
```

**Decisão atualizada (ADR-002 revisada):** reusar `chiserver.Router` sem criar interface paralela. `internal/platform/http/Deps` ganha `Registrars []chiserver.Router`. `NewServer` chama `srv.RegisterRouters(deps.Registrars...)` após criar o `*chiserver.Server`.

Para o timeout específico do webhook (RF-06 ack < 2s p99), reusar `chiserver.WithRouteTimeout("/webhooks/kiwify", 2*time.Second)` em `serverBuilder.buildOptions()` adicionando o path ao mapa. Sem este per-route timeout, herdaria o default global de 25s.

```go
// internal/platform/http/server.go (alterado)
type Deps struct {
    DB         *database.Manager
    Provider   *observability.Provider
    Registrars []chiserver.Router          // novo
}

func (b *serverBuilder) buildOptions() []chiserver.Option {
    opts := []chiserver.Option{
        // ... existentes ...
        chiserver.WithRouteTimeout("/webhooks/kiwify", 2*time.Second),
    }
    return opts
}

func NewServer(cfg *configs.Config, deps Deps) (*chiserver.Server, error) {
    // ... criar srv ...
    srv.RegisterRouters(deps.Registrars...)
    return srv, nil
}
```

#### `application/interfaces/billing_provider.go`

```go
package interfaces

import (
    "context"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
)

type BillingProvider interface {
    VerifySignature(payload []byte, headers map[string]string) error
    ParseEvent(payload []byte) (services.CanonicalEvent, error)
    FetchSubscription(ctx context.Context, externalSubscriptionID string) (services.CanonicalSubscription, error)
}
```

Sem `secret` no parâmetro: secret é dependência injetada no adapter via construtor (`KiwifyConfig.WebhookSecret`). Mantém interface limpa.

#### `application/interfaces/subscription_repository.go`

```go
package interfaces

import (
    "context"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"

    identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

type SubscriptionRepository interface {
    Upsert(ctx context.Context, sub *entities.Subscription) error
    FindActiveByUserID(ctx context.Context, userID identityentities.UserID) (*entities.Subscription, error)
    FindActiveByUserIDForUpdate(ctx context.Context, userID identityentities.UserID) (*entities.Subscription, error)
    FindByExternalID(ctx context.Context, provider string, externalID valueobjects.ExternalSubscriptionID) (*entities.Subscription, error)
    ListByStatusInBatch(ctx context.Context, statuses []valueobjects.SubscriptionStatus, cursorCreatedAt time.Time, cursorID entities.SubscriptionID, limit int) ([]*entities.Subscription, error)
}
```

#### `application/interfaces/webhook_event_repository.go`

```go
package interfaces

import (
    "context"
    "encoding/json"
    "time"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type WebhookEventRepository interface {
    InsertIfNew(ctx context.Context, event entities.WebhookEvent) (inserted bool, err error)
    FindRawPayload(ctx context.Context, id valueobjects.WebhookEventID) (json.RawMessage, error)
    MarkProcessed(ctx context.Context, id valueobjects.WebhookEventID, at time.Time) error
    RecordApplication(ctx context.Context, eventID valueobjects.WebhookEventID, subID entities.SubscriptionID, at time.Time) (recorded bool, err error)
    ListPendingAnonymization(ctx context.Context, olderThan time.Time, limit int) ([]entities.WebhookEvent, error)
    Anonymize(ctx context.Context, id valueobjects.WebhookEventID, redacted json.RawMessage, at time.Time) error
}
```

`InsertIfNew` retorna `(false, nil)` em `ON CONFLICT DO NOTHING` (não é erro). `RecordApplication` mesmo padrão para idempotência no processor.

#### `application/interfaces/entitlement_cache.go`

```go
package interfaces

import (
    "time"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/output"

    identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

type EntitlementCache interface {
    Get(userID identityentities.UserID) (output.EntitlementDecision, bool)
    Set(userID identityentities.UserID, decision output.EntitlementDecision, ttl time.Duration)
    Invalidate(userID identityentities.UserID)
}
```

#### `application/interfaces/user_resolver.go` (segregação cross-module)

```go
package interfaces

import (
    "context"

    identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
    identityvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type UserResolver interface {
    UpsertByWhatsAppNumber(ctx context.Context, number identityvo.WhatsAppNumber) (*identityentities.User, error)
}
```

Wrapper segregado para que mocks e tests do billing não precisem mockar `identity.UserRepository` inteiro. Implementação concreta delega para `identity.UserRepository` injetado via wire.

#### `application/usecases/ingest_kiwify_webhook.go`

```go
package usecases

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/output"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
    billingoutbox "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/outbox"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/clock"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
)

type IngestKiwifyWebhookUseCase struct {
    provider     interfaces.BillingProvider
    webhookRepo  interfaces.WebhookEventRepository
    publisher    *outbox.Publisher
    unitOfWork   *database.UnitOfWork[output.IngestWebhookResult]
    idGenerator  interfaces.IDGenerator
    clock        clock.Clock
}

func NewIngestKiwifyWebhookUseCase(
    provider interfaces.BillingProvider,
    webhookRepo interfaces.WebhookEventRepository,
    publisher *outbox.Publisher,
    unitOfWork *database.UnitOfWork[output.IngestWebhookResult],
    idGenerator interfaces.IDGenerator,
    clk clock.Clock,
) *IngestKiwifyWebhookUseCase {
    return &IngestKiwifyWebhookUseCase{
        provider:    provider,
        webhookRepo: webhookRepo,
        publisher:   publisher,
        unitOfWork:  unitOfWork,
        idGenerator: idGenerator,
        clock:       clk,
    }
}

func (u *IngestKiwifyWebhookUseCase) Execute(ctx context.Context, in input.IngestWebhookInput) (output.IngestWebhookResult, error) {
    if err := u.provider.VerifySignature(in.RawBody, in.Headers); err != nil {
        return output.IngestWebhookResult{}, fmt.Errorf("ingest kiwify: %w", err)
    }
    externalID, err := valueobjects.NewExternalEventIDCascade(in.RawBody)
    if err != nil {
        return output.IngestWebhookResult{}, fmt.Errorf("ingest kiwify: %w", err)
    }
    return u.unitOfWork.Do(ctx, func(txCtx context.Context, _ database.DBTX) (output.IngestWebhookResult, error) {
        webhookEvent := entities.NewWebhookEvent(entities.NewWebhookEventParams{
            ID:              valueobjects.WebhookEventID(u.idGenerator.NewID()),
            Provider:        "kiwify",
            ExternalEventID: externalID,
            EventType:       valueobjects.RawEventTypeFromPayload(in.RawBody),
            Signature:       in.Headers["X-Kiwify-Webhook-Token"],
            HeadersJSON:     in.HeadersJSON(),
            Payload:         in.RawBody,
            ReceivedAt:      u.clock.Now(),
        })
        inserted, err := u.webhookRepo.InsertIfNew(txCtx, webhookEvent)
        if err != nil {
            return output.IngestWebhookResult{}, fmt.Errorf("insert webhook_event: %w", err)
        }
        if !inserted {
            return output.IngestWebhookResult{Duplicate: true}, nil
        }
        payload, err := billingoutbox.EncodeReceivedPayload(webhookEvent.ID())
        if err != nil {
            return output.IngestWebhookResult{}, fmt.Errorf("encode outbox payload: %w", err)
        }
        evt, err := outbox.NewEvent(outbox.NewEventParams{
            ID:            events.NewEventID(u.idGenerator.NewID()),
            EventType:     events.EventName("billing.kiwify.received"),
            AggregateType: "webhook_event",
            AggregateID:   webhookEvent.ID().String(),
            Payload:       payload,
            OccurredAt:    u.clock.Now(),
        })
        if err != nil {
            return output.IngestWebhookResult{}, fmt.Errorf("new outbox event: %w", err)
        }
        if err := u.publisher.Publish(txCtx, evt); err != nil {
            return output.IngestWebhookResult{}, fmt.Errorf("publish outbox: %w", err)
        }
        return output.IngestWebhookResult{Duplicate: false, WebhookEventID: webhookEvent.ID()}, nil
    })
}
```

R1 atendido (método de struct, único `New*` permitido). R2 atendido (sem alias de campo). R5.10 (wrapping com `%w` + PT-BR conciso).

#### `application/usecases/process_billing_event.go`

```go
package usecases

type ProcessBillingEventUseCase struct {
    webhookRepo  interfaces.WebhookEventRepository
    subRepo      interfaces.SubscriptionRepository
    provider     interfaces.BillingProvider
    userResolver interfaces.UserResolver
    cache        interfaces.EntitlementCache
    bus          *events.Bus
    unitOfWork   *database.UnitOfWork[*entities.Subscription]
    stateMachine services.StateMachine
    clock        clock.Clock
    logger       *slog.Logger
}

func (u *ProcessBillingEventUseCase) Handle(ctx context.Context, evt outbox.Event) error {
    payload, err := billingoutbox.DecodeReceivedPayload(evt.Payload())
    if err != nil {
        return fmt.Errorf("process billing event: %w: %w", err, outbox.ErrPermanent)
    }
    raw, err := u.webhookRepo.FindRawPayload(ctx, payload.WebhookEventID)
    if err != nil {
        return fmt.Errorf("process billing event: lookup raw: %w", err)
    }
    canonical, err := u.provider.ParseEvent(raw)
    if err != nil {
        return fmt.Errorf("process billing event: parse: %w: %w", err, outbox.ErrPermanent)
    }
    sub, err := u.unitOfWork.Do(ctx, func(txCtx context.Context, _ database.DBTX) (*entities.Subscription, error) {
        user, err := u.userResolver.UpsertByWhatsAppNumber(txCtx, canonical.Customer.WhatsApp)
        if err != nil {
            return nil, fmt.Errorf("upsert user: %w", err)
        }
        existing, err := u.subRepo.FindActiveByUserIDForUpdate(txCtx, user.ID())
        if err != nil && !errors.Is(err, postgres.ErrSubscriptionNotFound) {
            return nil, fmt.Errorf("find active subscription: %w", err)
        }
        sub, transition, err := u.stateMachine.Apply(existing, canonical, user.ID(), u.clock.Now())
        if err != nil {
            return nil, fmt.Errorf("apply canonical: %w", err)
        }
        if transition.IsNoop() {
            return sub, nil
        }
        recorded, err := u.webhookRepo.RecordApplication(txCtx, payload.WebhookEventID, sub.ID(), u.clock.Now())
        if err != nil {
            return nil, fmt.Errorf("record application: %w", err)
        }
        if !recorded {
            return sub, nil
        }
        if err := u.subRepo.Upsert(txCtx, sub); err != nil {
            return nil, fmt.Errorf("upsert subscription: %w", err)
        }
        if err := u.webhookRepo.MarkProcessed(txCtx, payload.WebhookEventID, u.clock.Now()); err != nil {
            return nil, fmt.Errorf("mark processed: %w", err)
        }
        return sub, nil
    })
    if err != nil {
        return err
    }
    u.cache.Invalidate(sub.UserID())
    u.bus.Publish(events.NewBusEvent("billing.subscription.state_changed", sub.SnapshotForNotification()))
    return nil
}
```

Ordem é crítica:
1. `RecordApplication` antes de `Upsert` — se `RecordApplication` retorna `recorded=false`, indica reprocessamento já aplicado, no-op idempotente.
2. `MarkProcessed` ao fim — sinaliza para auditoria que webhook ainda não-processado.
3. Cache `Invalidate` e `events.Bus.Publish` fora da transação — efeito colateral pós-commit.

### Modelos de Domínio

#### Agregado `entities.Subscription`

```go
package entities

import (
    "errors"
    "time"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"

    identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

type SubscriptionID struct{ value string }

func NewSubscriptionID(v string) (SubscriptionID, error) {
    if v == "" {
        return SubscriptionID{}, ErrInvalidSubscriptionID
    }
    return SubscriptionID{value: v}, nil
}

func (s SubscriptionID) String() string { return s.value }

type Subscription struct {
    id                   SubscriptionID
    userID               identityentities.UserID
    provider             string
    externalSubID        valueobjects.ExternalSubscriptionID
    planCode             valueobjects.PlanCode
    status               valueobjects.SubscriptionStatus
    period               valueobjects.BillingPeriod
    periodStart          time.Time
    periodEnd            time.Time
    gracePeriodEnd       time.Time
    refundAmountCents    valueobjects.MoneyBRL
    lastEventAt          time.Time
    lastWebhookEventID   valueobjects.WebhookEventID
    createdAt            time.Time
    updatedAt            time.Time
    deletedAt            *time.Time
}

type NewSubscriptionParams struct {
    ID                 SubscriptionID
    UserID             identityentities.UserID
    Provider           string
    ExternalSubID      valueobjects.ExternalSubscriptionID
    PlanCode           valueobjects.PlanCode
    InitialStatus      valueobjects.SubscriptionStatus
    PeriodStart        time.Time
    PeriodEnd          time.Time
    LastEventAt        time.Time
    LastWebhookEventID valueobjects.WebhookEventID
    CreatedAt          time.Time
}

func NewSubscription(p NewSubscriptionParams) (*Subscription, error) {
    if p.ID.String() == "" {
        return nil, ErrSubscriptionRequiresID
    }
    if p.Provider == "" {
        return nil, ErrSubscriptionRequiresProvider
    }
    if !p.InitialStatus.IsCreatable() {
        return nil, ErrSubscriptionInitialStatusInvalid
    }
    if p.PeriodStart.IsZero() || p.PeriodEnd.IsZero() || p.PeriodEnd.Before(p.PeriodStart) {
        return nil, ErrSubscriptionRequiresPeriod
    }
    period, err := valueobjects.NewBillingPeriodFor(p.PlanCode)
    if err != nil {
        return nil, err
    }
    return &Subscription{
        id:                 p.ID,
        userID:             p.UserID,
        provider:           p.Provider,
        externalSubID:      p.ExternalSubID,
        planCode:           p.PlanCode,
        status:             p.InitialStatus,
        period:             period,
        periodStart:        p.PeriodStart,
        periodEnd:          p.PeriodEnd,
        lastEventAt:        p.LastEventAt,
        lastWebhookEventID: p.LastWebhookEventID,
        createdAt:          p.CreatedAt,
        updatedAt:          p.CreatedAt,
    }, nil
}

func (s *Subscription) ID() SubscriptionID                       { return s.id }
func (s *Subscription) UserID() identityentities.UserID          { return s.userID }
func (s *Subscription) Status() valueobjects.SubscriptionStatus  { return s.status }
func (s *Subscription) PeriodEnd() time.Time                     { return s.periodEnd }
func (s *Subscription) GracePeriodEnd() time.Time                { return s.gracePeriodEnd }
func (s *Subscription) PlanCode() valueobjects.PlanCode          { return s.planCode }
func (s *Subscription) ExternalSubscriptionID() valueobjects.ExternalSubscriptionID { return s.externalSubID }
func (s *Subscription) LastEventAt() time.Time                   { return s.lastEventAt }

func (s *Subscription) applyTransition(target valueobjects.SubscriptionStatus, reason valueobjects.TransitionReason, at time.Time, period services.PeriodChange) error {
    if err := services.NewStateMachine().AssertLegal(s.status, target); err != nil {
        return err
    }
    s.status = target
    s.lastEventAt = at
    s.updatedAt = at
    if period.AdvancesPeriod() {
        s.periodStart = period.NewStart
        s.periodEnd = period.NewEnd
    }
    if target == valueobjects.SubscriptionStatusPastDue {
        s.gracePeriodEnd = at.Add(services.DefaultGracePeriod)
    }
    return nil
}

func (s *Subscription) Activate(at time.Time, period services.PeriodChange) error {
    return s.applyTransition(valueobjects.SubscriptionStatusActive, valueobjects.TransitionReasonPurchaseApproved, at, period)
}

func (s *Subscription) Renew(at time.Time, period services.PeriodChange) error {
    return s.applyTransition(valueobjects.SubscriptionStatusActive, valueobjects.TransitionReasonRenewed, at, period)
}

func (s *Subscription) MarkPastDue(at time.Time) error {
    return s.applyTransition(valueobjects.SubscriptionStatusPastDue, valueobjects.TransitionReasonLate, at, services.NoPeriodChange())
}

func (s *Subscription) Cancel(at time.Time) error {
    return s.applyTransition(valueobjects.SubscriptionStatusCanceledPending, valueobjects.TransitionReasonCanceled, at, services.NoPeriodChange())
}

func (s *Subscription) Expire(at time.Time) error {
    return s.applyTransition(valueobjects.SubscriptionStatusExpired, valueobjects.TransitionReasonExpired, at, services.NoPeriodChange())
}

func (s *Subscription) Refund(at time.Time, amount valueobjects.MoneyBRL, reason valueobjects.TransitionReason) error {
    if err := services.NewStateMachine().AssertLegal(s.status, valueobjects.SubscriptionStatusRefunded); err != nil {
        return err
    }
    s.status = valueobjects.SubscriptionStatusRefunded
    s.refundAmountCents = amount
    s.lastEventAt = at
    s.updatedAt = at
    return nil
}

var (
    ErrInvalidSubscriptionID            = errors.New("billing: subscription id inválido")
    ErrSubscriptionRequiresID           = errors.New("billing: subscription requer id")
    ErrSubscriptionRequiresProvider     = errors.New("billing: subscription requer provider")
    ErrSubscriptionInitialStatusInvalid = errors.New("billing: status inicial não permitido na criação")
    ErrSubscriptionRequiresPeriod       = errors.New("billing: subscription requer period_start < period_end")
)
```

Implementa `identity.domain.services.Subscription` por satisfação estrutural (interface aceita `Status() services.SubscriptionStatus`, `CurrentPeriodEnd() time.Time`, `GracePeriodEnd() time.Time`). Método `Status()` aqui retorna `valueobjects.SubscriptionStatus` (mesma estrutura semântica), com método `Equivalent()` em VO mapeando para enum de identity quando necessário no boundary do `IsEntitled`. Detalhamento da satisfação em ADR-005.

OC #3 (encapsula PlanCode/SubscriptionStatus/MoneyBRL); OC #9 (sem getters mecânicos — métodos com intenção: `Activate`, `MarkPastDue`, `Refund`); OC #1/#2 (early return em construtor); R1 (todas as funções como métodos); R5.10 (sentinelas).

#### Value Objects

```go
// valueobjects/plan_code.go
package valueobjects

type PlanCode uint8

const (
    PlanCodeUnknown PlanCode = iota
    PlanCodeMonthly
    PlanCodeQuarterly
    PlanCodeAnnual
)

func (p PlanCode) String() string {
    switch p {
    case PlanCodeMonthly:
        return "MONTHLY"
    case PlanCodeQuarterly:
        return "QUARTERLY"
    case PlanCodeAnnual:
        return "ANNUAL"
    default:
        return "UNKNOWN"
    }
}

func ParsePlanCode(s string) (PlanCode, error) {
    switch s {
    case "MONTHLY":
        return PlanCodeMonthly, nil
    case "QUARTERLY":
        return PlanCodeQuarterly, nil
    case "ANNUAL":
        return PlanCodeAnnual, nil
    default:
        return PlanCodeUnknown, ErrUnknownPlanCode
    }
}
```

```go
// valueobjects/billing_period.go
package valueobjects

import "time"

type BillingPeriod struct{ length time.Duration }

func NewBillingPeriodFor(code PlanCode) (BillingPeriod, error) {
    switch code {
    case PlanCodeMonthly:
        return BillingPeriod{length: 30 * 24 * time.Hour}, nil
    case PlanCodeQuarterly:
        return BillingPeriod{length: 90 * 24 * time.Hour}, nil
    case PlanCodeAnnual:
        return BillingPeriod{length: 365 * 24 * time.Hour}, nil
    default:
        return BillingPeriod{}, ErrUnknownPlanCode
    }
}

func (p BillingPeriod) Advance(from time.Time) time.Time { return from.Add(p.length) }
func (p BillingPeriod) Length() time.Duration            { return p.length }
```

```go
// valueobjects/subscription_status.go
package valueobjects

type SubscriptionStatus uint8

const (
    SubscriptionStatusUnknown SubscriptionStatus = iota
    SubscriptionStatusTrialing
    SubscriptionStatusActive
    SubscriptionStatusPastDue
    SubscriptionStatusCanceledPending
    SubscriptionStatusExpired
    SubscriptionStatusRefunded
)

func (s SubscriptionStatus) IsCreatable() bool {
    switch s {
    case SubscriptionStatusActive, SubscriptionStatusTrialing:
        return true
    default:
        return false
    }
}

func (s SubscriptionStatus) String() string {
    switch s {
    case SubscriptionStatusTrialing:
        return "TRIALING"
    case SubscriptionStatusActive:
        return "ACTIVE"
    case SubscriptionStatusPastDue:
        return "PAST_DUE"
    case SubscriptionStatusCanceledPending:
        return "CANCELED_PENDING"
    case SubscriptionStatusExpired:
        return "EXPIRED"
    case SubscriptionStatusRefunded:
        return "REFUNDED"
    default:
        return "UNKNOWN"
    }
}
```

R5.8 aplicado: iota com zero-value `Unknown` reservado.

```go
// valueobjects/money_brl.go — VO para valor monetário em centavos
package valueobjects

import "errors"

type MoneyBRL struct{ cents int64 }

func NewMoneyBRL(cents int64) (MoneyBRL, error) {
    if cents < 0 {
        return MoneyBRL{}, ErrNegativeAmount
    }
    return MoneyBRL{cents: cents}, nil
}

func (m MoneyBRL) Cents() int64      { return m.cents }
func (m MoneyBRL) IsZero() bool      { return m.cents == 0 }

var ErrNegativeAmount = errors.New("billing: valor monetário não pode ser negativo")
```

```go
// valueobjects/external_event_id.go — cascata documentada (RF-03)
package valueobjects

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "strings"
)

type ExternalEventID struct{ value string }

func NewExternalEventIDCascade(rawBody []byte) (ExternalEventID, error) {
    if len(rawBody) == 0 {
        return ExternalEventID{}, ErrEmptyPayload
    }
    var probe struct {
        ID    string `json:"id"`
        Order struct {
            ID string `json:"id"`
        } `json:"order"`
    }
    if err := json.Unmarshal(rawBody, &probe); err == nil {
        if v := strings.TrimSpace(probe.ID); v != "" {
            return ExternalEventID{value: v}, nil
        }
        if v := strings.TrimSpace(probe.Order.ID); v != "" {
            return ExternalEventID{value: v}, nil
        }
    }
    sum := sha256.Sum256(rawBody)
    return ExternalEventID{value: "sha256:" + hex.EncodeToString(sum[:])}, nil
}

func (e ExternalEventID) String() string { return e.value }
```

#### Domain Service `services.StateMachine`

```go
package services

import (
    "errors"
    "time"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

const DefaultGracePeriod = 7 * 24 * time.Hour

type StateMachine struct{}

func NewStateMachine() StateMachine { return StateMachine{} }

func (StateMachine) AssertLegal(from, to valueobjects.SubscriptionStatus) error {
    if isLegalTransition(from, to) {
        return nil
    }
    return ErrIllegalTransition
}

func isLegalTransition(from, to valueobjects.SubscriptionStatus) bool {
    switch from {
    case valueobjects.SubscriptionStatusTrialing:
        return to == valueobjects.SubscriptionStatusActive || to == valueobjects.SubscriptionStatusExpired
    case valueobjects.SubscriptionStatusActive:
        return to == valueobjects.SubscriptionStatusPastDue ||
            to == valueobjects.SubscriptionStatusCanceledPending ||
            to == valueobjects.SubscriptionStatusRefunded
    case valueobjects.SubscriptionStatusPastDue:
        return to == valueobjects.SubscriptionStatusActive ||
            to == valueobjects.SubscriptionStatusExpired ||
            to == valueobjects.SubscriptionStatusRefunded
    case valueobjects.SubscriptionStatusCanceledPending:
        return to == valueobjects.SubscriptionStatusExpired ||
            to == valueobjects.SubscriptionStatusActive ||
            to == valueobjects.SubscriptionStatusRefunded
    case valueobjects.SubscriptionStatusExpired, valueobjects.SubscriptionStatusRefunded:
        return false
    default:
        return false
    }
}

var ErrIllegalTransition = errors.New("billing: transição de estado ilegal")

type PeriodChange struct {
    NewStart time.Time
    NewEnd   time.Time
}

func (p PeriodChange) AdvancesPeriod() bool { return !p.NewStart.IsZero() && !p.NewEnd.IsZero() }

func NoPeriodChange() PeriodChange { return PeriodChange{} }
```

OC aplicado: regra de transição centralizada em domain service stateless, agregado delega. `isLegalTransition` é função privada (exceção pragmática a R1 — helper sem estado dentro do mesmo arquivo, padrão aceito no projeto conforme ADR-007 de identity).

#### Schema Postgres — `migrations/0009_billing_schema.up.sql`

Convenções nominais alinhadas com migrations 0003/0004 de identity:

| Tipo | Padrão | Exemplo |
|---|---|---|
| Tabela | `snake_case` plural | `webhook_events`, `subscriptions`, `billing_event_applications`, `billing_plans` |
| Primary key | `pk_<tabela>` | `pk_subscriptions` |
| Foreign key | `fk_<tabela>_<coluna>` | `fk_subscriptions_user_id` |
| Unique | `uq_<tabela>_<coluna(s)>` | `uq_subscriptions_external_id` |
| Check | `ck_<tabela>_<regra>` | `ck_subscriptions_status` |
| Index | `idx_<tabela>_<coluna(s)>` | `idx_webhook_events_pending_anonymization` |

```sql
-- migration: 0009_billing_schema.up.sql
-- Cria substrato do módulo billing: event store imutável (webhook_events),
-- agregado Subscription, idempotência de processamento e seed de planos.

CREATE TABLE IF NOT EXISTS webhook_events (
    id                  TEXT         NOT NULL,
    provider            TEXT         NOT NULL,
    external_event_id   TEXT         NOT NULL,
    event_type          TEXT         NOT NULL,
    signature           TEXT,
    headers             JSONB        NOT NULL DEFAULT '{}'::jsonb,
    payload             JSONB        NOT NULL,
    received_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    processed_at        TIMESTAMPTZ,
    anonymized_at       TIMESTAMPTZ,
    CONSTRAINT pk_webhook_events PRIMARY KEY (id),
    CONSTRAINT uq_webhook_events_external
        UNIQUE (provider, external_event_id)
);

CREATE INDEX IF NOT EXISTS idx_webhook_events_received_at
    ON webhook_events (received_at DESC);

CREATE INDEX IF NOT EXISTS idx_webhook_events_pending_anonymization
    ON webhook_events (received_at)
    WHERE anonymized_at IS NULL;

REVOKE ALL ON TABLE webhook_events FROM PUBLIC;

CREATE TABLE IF NOT EXISTS subscriptions (
    id                         TEXT         NOT NULL,
    user_id                    UUID         NOT NULL,
    provider                   TEXT         NOT NULL,
    external_subscription_id   TEXT         NOT NULL,
    plan_code                  TEXT         NOT NULL,
    status                     TEXT         NOT NULL,
    period_start               TIMESTAMPTZ  NOT NULL,
    period_end                 TIMESTAMPTZ  NOT NULL,
    grace_period_end           TIMESTAMPTZ,
    refund_amount_cents        BIGINT       NOT NULL DEFAULT 0,
    last_event_at              TIMESTAMPTZ  NOT NULL,
    last_webhook_event_id      TEXT         NOT NULL,
    created_at                 TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at                 TIMESTAMPTZ,
    CONSTRAINT pk_subscriptions PRIMARY KEY (id),
    CONSTRAINT fk_subscriptions_user_id
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT,
    CONSTRAINT fk_subscriptions_last_webhook_event_id
        FOREIGN KEY (last_webhook_event_id) REFERENCES webhook_events (id) ON DELETE RESTRICT,
    CONSTRAINT ck_subscriptions_status CHECK (status IN
        ('TRIALING','ACTIVE','PAST_DUE','CANCELED_PENDING','EXPIRED','REFUNDED')),
    CONSTRAINT ck_subscriptions_period CHECK (period_end > period_start),
    CONSTRAINT ck_subscriptions_refund CHECK (refund_amount_cents >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_subscriptions_external_id
    ON subscriptions (provider, external_subscription_id)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_subscriptions_one_active_per_user
    ON subscriptions (user_id)
    WHERE status IN ('TRIALING','ACTIVE','PAST_DUE','CANCELED_PENDING')
      AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_subscriptions_status_active
    ON subscriptions (status)
    WHERE status IN ('ACTIVE','PAST_DUE') AND deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS billing_event_applications (
    event_id         TEXT         NOT NULL,
    subscription_id  TEXT         NOT NULL,
    applied_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT pk_billing_event_applications PRIMARY KEY (event_id),
    CONSTRAINT fk_billing_event_applications_subscription_id
        FOREIGN KEY (subscription_id) REFERENCES subscriptions (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_billing_event_applications_subscription_id
    ON billing_event_applications (subscription_id);

CREATE TABLE IF NOT EXISTS billing_plans (
    plan_code           TEXT         NOT NULL,
    display_name        TEXT         NOT NULL,
    period_length_days  INTEGER      NOT NULL,
    price_brl_cents     BIGINT       NOT NULL,
    kiwify_product_id   TEXT,
    active              BOOLEAN      NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT pk_billing_plans PRIMARY KEY (plan_code),
    CONSTRAINT ck_billing_plans_period CHECK (period_length_days > 0),
    CONSTRAINT ck_billing_plans_price CHECK (price_brl_cents > 0)
);
```

```sql
-- migration: 0010_billing_plans_seed.up.sql
-- D-03 do PRD: valores definitivos do MVP.
INSERT INTO billing_plans (plan_code, display_name, period_length_days, price_brl_cents)
VALUES
    ('MONTHLY',   'Mensal',     30,  2990),
    ('QUARTERLY', 'Trimestral', 90,  8073),
    ('ANNUAL',    'Anual',      365, 29780)
ON CONFLICT (plan_code) DO NOTHING;
```

Migrations down revertem na ordem inversa (DROP TABLE em ordem oposta).

### Adapter Kiwify — Implementação Concreta

#### `infrastructure/http/client/kiwify/oauth.go`

```go
package kiwify

import (
    "context"
    "fmt"
    "net/http"
    "net/url"
    "strings"
    "sync"
    "time"
)

type oauthCache struct {
    mu          sync.RWMutex
    token       string
    expiresAt   time.Time
}

type OAuthClient struct {
    httpClient   *http.Client
    baseURL      string
    clientID     string
    clientSecret string
    safetyMargin time.Duration
    cache        oauthCache
    clock        Clock
}

func NewOAuthClient(httpClient *http.Client, baseURL, clientID, clientSecret string, safetyMargin time.Duration, clock Clock) *OAuthClient {
    return &OAuthClient{
        httpClient:   httpClient,
        baseURL:      strings.TrimRight(baseURL, "/"),
        clientID:     clientID,
        clientSecret: clientSecret,
        safetyMargin: safetyMargin,
        clock:        clock,
    }
}

func (c *OAuthClient) Token(ctx context.Context) (string, error) {
    if cached, ok := c.cachedToken(); ok {
        return cached, nil
    }
    return c.refresh(ctx)
}

func (c *OAuthClient) cachedToken() (string, bool) {
    c.cache.mu.RLock()
    defer c.cache.mu.RUnlock()
    if c.cache.token == "" {
        return "", false
    }
    if c.clock.Now().After(c.cache.expiresAt) {
        return "", false
    }
    return c.cache.token, true
}

func (c *OAuthClient) refresh(ctx context.Context) (string, error) {
    c.cache.mu.Lock()
    defer c.cache.mu.Unlock()
    if c.cache.token != "" && c.clock.Now().Before(c.cache.expiresAt) {
        return c.cache.token, nil
    }
    body := url.Values{}
    body.Set("client_id", c.clientID)
    body.Set("client_secret", c.clientSecret)
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/oauth/token", strings.NewReader(body.Encode()))
    if err != nil {
        return "", fmt.Errorf("kiwify oauth: new request: %w", err)
    }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return "", fmt.Errorf("kiwify oauth: do: %w", err)
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("kiwify oauth: status %d: %w", resp.StatusCode, ErrOAuthFailed)
    }
    var decoded struct {
        AccessToken string `json:"access_token"`
        ExpiresIn   int64  `json:"expires_in"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
        return "", fmt.Errorf("kiwify oauth: decode: %w", err)
    }
    c.cache.token = decoded.AccessToken
    c.cache.expiresAt = c.clock.Now().Add(time.Duration(decoded.ExpiresIn) * time.Second).Add(-c.safetyMargin)
    return decoded.AccessToken, nil
}

var ErrOAuthFailed = errors.New("kiwify oauth: autenticação falhou")
```

ADR-008 detalha estratégia. Sem `refresh_token` (D-05 do PRD). Lock duplo-check para evitar requests concorrentes redundantes.

#### `infrastructure/http/client/kiwify/signature_verifier.go`

```go
package kiwify

import (
    "crypto/subtle"
    "errors"
    "strings"
)

type TokenSignatureVerifier struct {
    expectedToken string
    headerName    string
}

func NewTokenSignatureVerifier(expectedToken, headerName string) TokenSignatureVerifier {
    return TokenSignatureVerifier{
        expectedToken: expectedToken,
        headerName:    strings.ToLower(headerName),
    }
}

func (v TokenSignatureVerifier) Verify(_ []byte, headers map[string]string) error {
    received := lookupHeaderCanonical(headers, v.headerName)
    if received == "" {
        return ErrMissingSignature
    }
    if subtle.ConstantTimeCompare([]byte(received), []byte(v.expectedToken)) != 1 {
        return ErrInvalidSignature
    }
    return nil
}

func lookupHeaderCanonical(headers map[string]string, name string) string {
    for k, v := range headers {
        if strings.EqualFold(k, name) {
            return v
        }
    }
    return ""
}

var (
    ErrMissingSignature = errors.New("kiwify webhook: header de assinatura ausente")
    ErrInvalidSignature = errors.New("kiwify webhook: assinatura inválida")
)
```

ADR-006 detalha plug de HMAC-SHA256 sem mudança de interface.

#### `infrastructure/http/client/kiwify/payload_mapper.go`

```go
package kiwify

import (
    "encoding/json"
    "errors"
    "strings"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"

    identityvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type PayloadMapper struct{}

func (m PayloadMapper) Parse(raw []byte) (services.CanonicalEvent, error) {
    var payload kiwifyPayload
    if err := json.Unmarshal(raw, &payload); err != nil {
        return services.CanonicalEvent{}, ErrPayloadDecode
    }
    eventType, err := m.mapEventType(payload.WebhookEventType)
    if err != nil {
        return services.CanonicalEvent{}, err
    }
    whatsapp, err := identityvo.NewWhatsAppNumber(payload.Customer.Mobile)
    if err != nil {
        return services.CanonicalEvent{}, err
    }
    plan, err := valueobjects.ParsePlanCodeFromKiwifyProductID(payload.Product.ID)
    if err != nil {
        return services.CanonicalEvent{}, err
    }
    return services.CanonicalEvent{
        Type:                   eventType,
        ExternalEventID:        payload.ID,
        ExternalSubscriptionID: payload.Subscription.ID,
        PlanCode:               plan,
        OccurredAt:             payload.UpdatedAt,
        PeriodStart:            payload.Subscription.CurrentPeriodStart,
        PeriodEnd:              payload.Subscription.CurrentPeriodEnd,
        SignupToken:            m.extractSignupTokenCascade(payload.Tracking),
        Customer: services.CanonicalCustomer{
            WhatsApp: whatsapp,
            Email:    payload.Customer.Email,
        },
        RefundAmountCents: payload.Refund.AmountCents,
    }, nil
}

func (PayloadMapper) extractSignupTokenCascade(tracking kiwifyTracking) string {
    candidates := []string{tracking.Src, tracking.UTMContent, tracking.S1, tracking.S2, tracking.S3}
    for _, c := range candidates {
        if v := strings.TrimSpace(c); v != "" {
            return v
        }
    }
    return ""
}

func (PayloadMapper) mapEventType(s string) (valueobjects.CanonicalEventType, error) {
    switch s {
    case "compra_aprovada":
        return valueobjects.CanonicalEventPurchaseApproved, nil
    case "subscription_renewed":
        return valueobjects.CanonicalEventRenewed, nil
    case "subscription_late":
        return valueobjects.CanonicalEventLate, nil
    case "subscription_canceled":
        return valueobjects.CanonicalEventCanceled, nil
    case "compra_reembolsada":
        return valueobjects.CanonicalEventRefunded, nil
    case "chargeback":
        return valueobjects.CanonicalEventChargeback, nil
    default:
        return 0, ErrUnknownKiwifyEventType
    }
}

type kiwifyPayload struct {
    ID               string `json:"id"`
    WebhookEventType string `json:"webhook_event_type"`
    UpdatedAt        time.Time `json:"updated_at"`
    Customer struct {
        Mobile string `json:"mobile"`
        Email  string `json:"email"`
    } `json:"customer"`
    Product struct {
        ID string `json:"id"`
    } `json:"product"`
    Subscription struct {
        ID                  string    `json:"id"`
        CurrentPeriodStart  time.Time `json:"current_period_start"`
        CurrentPeriodEnd    time.Time `json:"current_period_end"`
    } `json:"subscription"`
    Refund struct {
        AmountCents int64 `json:"amount_cents"`
    } `json:"refund"`
    Tracking kiwifyTracking `json:"tracking"`
}

type kiwifyTracking struct {
    Src         string `json:"src"`
    UTMContent  string `json:"utm_content"`
    S1          string `json:"s1"`
    S2          string `json:"s2"`
    S3          string `json:"s3"`
}

var (
    ErrPayloadDecode          = errors.New("kiwify payload: json inválido")
    ErrUnknownKiwifyEventType = errors.New("kiwify payload: event_type desconhecido")
)
```

Cascata de tracking conforme RF-30. `UpdatedAt` é o `occurred_at` canônico. Mapeamento `kiwify_product_id → PlanCode` consulta `billing_plans` via método em `valueobjects.ParsePlanCodeFromKiwifyProductID` que recebe instância de `BillingPlansRegistry` populada na inicialização.

### Cache LRU (ADR-004)

```go
package cache

import (
    "time"

    expirable "github.com/hashicorp/golang-lru/v2/expirable"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/output"

    identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

type EntitlementLRU struct {
    inner *expirable.LRU[string, output.EntitlementDecision]
}

func NewEntitlementLRU(capacity int, defaultTTL time.Duration) *EntitlementLRU {
    return &EntitlementLRU{
        inner: expirable.NewLRU[string, output.EntitlementDecision](capacity, nil, defaultTTL),
    }
}

func (c *EntitlementLRU) Get(userID identityentities.UserID) (output.EntitlementDecision, bool) {
    return c.inner.Get(userID.String())
}

func (c *EntitlementLRU) Set(userID identityentities.UserID, decision output.EntitlementDecision, _ time.Duration) {
    c.inner.Add(userID.String(), decision)
}

func (c *EntitlementLRU) Invalidate(userID identityentities.UserID) {
    c.inner.Remove(userID.String())
}
```

Nota: `expirable.LRU` aceita TTL único na construção (não per-entry). Decisão de design: usar TTL conservador de 5min (constante). Cálculo de TTL dinâmico do PRD (`min(period_end - now, 5min)`) seria útil para `period_end` próximo, mas adiciona complexidade sem ganho significativo. Em RFs (RF-33), TTL é "fixed 5min". Trade-off documentado em ADR-004.

### Scheduler Subsystem (ADR-003)

```go
package scheduler

type BillingScheduler struct {
    cron               *robfigcron.Cron
    logger             *slog.Logger
    reconciliation     *usecases.ReconcileSubscriptionsUseCase
    anonymization      *usecases.AnonymizeWebhookEventsUseCase
    reconciliationCron string
    anonymizationCron  string
}

func New(deps Deps) *BillingScheduler {
    return &BillingScheduler{
        cron:               robfigcron.New(robfigcron.WithSeconds()),
        logger:             deps.Logger,
        reconciliation:     deps.Reconciliation,
        anonymization:      deps.Anonymization,
        reconciliationCron: deps.ReconciliationCron,
        anonymizationCron:  deps.AnonymizationCron,
    }
}

func (s *BillingScheduler) Name() string { return "billing-scheduler" }

func (s *BillingScheduler) Start(ctx context.Context) error {
    if _, err := s.cron.AddFunc(s.reconciliationCron, func() {
        if err := s.reconciliation.Execute(ctx); err != nil {
            s.logger.ErrorContext(ctx, "billing reconciliation falhou", slog.Any("error", err))
        }
    }); err != nil {
        return fmt.Errorf("billing scheduler: agendar reconciliation: %w", err)
    }
    if _, err := s.cron.AddFunc(s.anonymizationCron, func() {
        if err := s.anonymization.Execute(ctx); err != nil {
            s.logger.ErrorContext(ctx, "billing anonymization falhou", slog.Any("error", err))
        }
    }); err != nil {
        return fmt.Errorf("billing scheduler: agendar anonymization: %w", err)
    }
    s.cron.Start()
    return nil
}

func (s *BillingScheduler) Stop(ctx context.Context) error {
    stopCtx := s.cron.Stop()
    select {
    case <-stopCtx.Done():
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

Subsystem lifecycle compatível com `runtime.Subsystem` existente. Cron expressions: `@hourly` para reconciliation, `@daily` para anonymization. Configuráveis via env (`KIWIFY_RECONCILIATION_INTERVAL`, `BILLING_ANONYMIZATION_SCHEDULE`).

### Configuração

```go
// configs/config.go — adição
type KiwifyConfig struct {
    APIBaseURL                  string        `mapstructure:"KIWIFY_API_BASE_URL"`
    WebhookSecret               string        `mapstructure:"KIWIFY_WEBHOOK_SECRET"`
    WebhookTokenHeader          string        `mapstructure:"KIWIFY_WEBHOOK_TOKEN_HEADER"`
    ClientID                    string        `mapstructure:"KIWIFY_CLIENT_ID"`
    ClientSecret                string        `mapstructure:"KIWIFY_CLIENT_SECRET"`
    OAuthTokenSafetyMargin      time.Duration `mapstructure:"KIWIFY_OAUTH_TOKEN_SAFETY_MARGIN"`
    RateLimitMaxRequestsPerMin  int           `mapstructure:"KIWIFY_RATE_LIMIT_MAX_REQUESTS_PER_MIN"`
    ReconciliationInterval      string        `mapstructure:"KIWIFY_RECONCILIATION_INTERVAL"`
    ReconciliationBatchSize     int           `mapstructure:"KIWIFY_RECONCILIATION_BATCH_SIZE"`
}

func (c KiwifyConfig) Safe() map[string]any {
    return map[string]any{
        "api_base_url":                  c.APIBaseURL,
        "webhook_token_header":          c.WebhookTokenHeader,
        "rate_limit":                    c.RateLimitMaxRequestsPerMin,
        "reconciliation_interval":       c.ReconciliationInterval,
        "reconciliation_batch_size":     c.ReconciliationBatchSize,
        "client_id_set":                 c.ClientID != "",
        "client_secret_set":             c.ClientSecret != "",
        "webhook_secret_set":            c.WebhookSecret != "",
    }
}

type BillingConfig struct {
    EntitlementCacheCapacity   int           `mapstructure:"BILLING_ENTITLEMENT_CACHE_CAPACITY"`
    EntitlementCacheTTL        time.Duration `mapstructure:"BILLING_ENTITLEMENT_CACHE_TTL"`
    AnonymizationSchedule      string        `mapstructure:"BILLING_ANONYMIZATION_SCHEDULE"`
    AnonymizationBatchSize     int           `mapstructure:"BILLING_ANONYMIZATION_BATCH_SIZE"`
    AnonymizationRetentionDays int           `mapstructure:"BILLING_ANONYMIZATION_RETENTION_DAYS"`
}
```

Defaults declarados em `setDefaults()` análogo ao outbox: `KIWIFY_API_BASE_URL=https://public-api.kiwify.com`, `KIWIFY_WEBHOOK_TOKEN_HEADER=X-Kiwify-Webhook-Token`, `KIWIFY_OAUTH_TOKEN_SAFETY_MARGIN=5m`, `KIWIFY_RATE_LIMIT_MAX_REQUESTS_PER_MIN=100`, `KIWIFY_RECONCILIATION_INTERVAL=@hourly`, `KIWIFY_RECONCILIATION_BATCH_SIZE=200`, `BILLING_ENTITLEMENT_CACHE_CAPACITY=50000`, `BILLING_ENTITLEMENT_CACHE_TTL=5m`, `BILLING_ANONYMIZATION_SCHEDULE=@daily`, `BILLING_ANONYMIZATION_BATCH_SIZE=500`, `BILLING_ANONYMIZATION_RETENTION_DAYS=365`.

Validação em `Validate()`: secrets obrigatórios em produção (rejeita boot com mensagem clara); rate limit `[1..500]`; cache capacity `[1000..500000]`; TTL `[1s..1h]`.

## Estratégia de Erros

Hierarquia (R5.10 + R-ERR-001):

| Camada | Sentinela | Quando |
|---|---|---|
| domain | `entities.ErrInvalidSubscriptionID`, `ErrSubscriptionRequiresID`, `ErrSubscriptionRequiresProvider`, `ErrSubscriptionInitialStatusInvalid`, `ErrSubscriptionRequiresPeriod` | Construtor de agregado falha invariante |
| domain | `services.ErrIllegalTransition` | StateMachine bloqueia transição |
| domain | `valueobjects.ErrUnknownPlanCode`, `ErrNegativeAmount`, `ErrEmptyPayload` | VO rejeita input |
| infrastructure (kiwify) | `kiwify.ErrMissingSignature`, `ErrInvalidSignature`, `ErrPayloadDecode`, `ErrUnknownKiwifyEventType`, `ErrOAuthFailed` | Adapter rejeita ou falha |
| infrastructure (postgres) | `postgres.ErrSubscriptionNotFound`, `ErrDuplicateActiveSubscription` | Repo traduz miss/pgerrcode |

Wrapping:
- Use cases: `fmt.Errorf("ingest kiwify: %w", err)`, `fmt.Errorf("process billing event: %w", err)`.
- Repos: `fmt.Errorf("postgres subscription repository: %w", err)`.
- HTTP handler traduz: `kiwify.ErrInvalidSignature` → `401`, `kiwify.ErrPayloadDecode` → `400`, demais → `500` com correlation_id no log.

Classificação para outbox handler (RF-26):
- `outbox.ErrPermanent` é wrapped pelo handler em: `ErrPayloadDecode`, `ErrUnknownKiwifyEventType` (payload corrompido nunca terá retry produtivo). Erros de Postgres transitórios (deadlock, connection refused) **não** são wrapped — Dispatcher retenta com backoff.

Proibido (governance.md + R-ERR-001):
- `panic` em código de produção.
- Engolir erros: todo `err != nil` retorna ou trata explicitamente.
- Logar e retornar o mesmo erro.
- Mensagens em inglês iniciadas por `failed to ...`.

## Endpoints de API

### `POST /webhooks/kiwify`

**Request:**
- Headers: `Content-Type: application/json`, `X-Kiwify-Webhook-Token: <secret>` (nome configurável via `KIWIFY_WEBHOOK_TOKEN_HEADER`)
- Body: JSON Kiwify (estrutura privada do provider, não documentada como contrato MeControla)

**Responses:**

| Status | Significado | Body |
|---|---|---|
| `200 OK` | Evento novo persistido + publicado | `{"received": true, "duplicate": false}` |
| `204 No Content` | Evento duplicado (idempotência) | vazio |
| `400 Bad Request` | Payload inválido (JSON ou estrutura) | `{"error":{"code":"invalid_payload"}}` |
| `401 Unauthorized` | Assinatura inválida ou ausente | `{"error":{"code":"unauthorized"}}` |
| `500 Internal Server Error` | Falha downstream (DB, outbox) | `{"error":{"code":"internal","correlation_id":"..."}}` |

**Timeout:** 25s herdado de `infrahttp.defaultTimeout`. Alvo p99 < 2s (RF-06).

## Pontos de Integração

- **Kiwify Public API** (`https://public-api.kiwify.com`) — OAuth client credentials para `FetchSubscription` (reconciliação). Sem chamada síncrona no caminho de webhook ingress.
- **Postgres** via `internal/platform/database.Manager`.
- **Identity module** via interfaces `UserResolver` (wrapper de `identity.UserRepository.UpsertByWhatsAppNumber`) e tipo `identity.entities.UserID` + `identity.valueobjects.WhatsAppNumber`.
- **Outbox** via `outbox.Publisher.Publish` (escrita) e registro em `outbox.Registry` (consumo).
- **events.Bus** para notificação volátil pós-commit (RF-27).
- **Observability** via `internal/platform/observability` + `mask.WhatsApp/Email` (já existentes).

Sem Redis. Sem brokers externos. Sem novos endpoints externos além de Kiwify.

## Abordagem de Testes

### Testes Unitários

Cobertura obrigatória (alinhada a CA-04):

| Pacote | Alvo | Cenários |
|---|---|---|
| `domain/valueobjects` | 100% | `PlanCode`, `BillingPeriod`, `SubscriptionStatus`, `CanonicalEventType`, `MoneyBRL`, `ExternalEventIDCascade` (id, order.id, fallback hash) |
| `domain/services` | 100% | `StateMachine.AssertLegal` com tabela exaustiva 6×6 (36 combinações) + `NoPeriodChange()` + `DefaultGracePeriod` |
| `domain/entities` | 100% | `NewSubscription` (válido, vários inválidos), todos os métodos de transição (`Activate`, `Renew`, `MarkPastDue`, `Cancel`, `Expire`, `Refund`) — sucesso + transição ilegal |
| `application/usecases` | ≥ 90% | Cada use case com mocks (mockery) cobrindo: caminho feliz, falha de VO, falha de port, idempotência (duplicate=true), evento stale ignorado |
| `infrastructure/http/client/kiwify` | ≥ 90% | OAuth: cache hit, cache miss, refresh em 401, falha de auth; SignatureVerifier: tabela com header presente/ausente/wrong-case/wrong-value; PayloadMapper: cada event_type Kiwify + cascata tracking + plan code mapping |
| `infrastructure/cache` | 100% | LRU expirable: hit, miss, eviction por capacidade, invalidate |

Estrutura: `testify/suite` table-driven (R4). Mocks via mockery declarados em `.mockery.yml` (R3 obrigatório):

```yaml
# .mockery.yml — entradas a adicionar
packages:
  ...
  github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces:
    interfaces:
      SubscriptionRepository:
      WebhookEventRepository:
      BillingProvider:
      EntitlementCache:
      UserResolver:
      IDGenerator:
```

Fuzz tests (R-TEST-001):
- `domain/valueobjects/external_event_id_fuzz_test.go` — `FuzzNewExternalEventIDCascade` com corpus seed (JSONs malformados, vazios, com `id` em vários níveis); nunca panica.
- `infrastructure/http/client/kiwify/payload_mapper_fuzz_test.go` — `FuzzPayloadMapperParse` com seed de payloads válidos e inválidos; nunca panica.

### Testes de Integração

Critérios (todos atendidos):
- [x] Fronteiras IO críticas (Postgres + idempotência via UNIQUE constraints + UoW).
- [x] Risco mock/prod (state machine + cascata + invalidação de cache).
- [x] Custo proporcional — testcontainers-go já é dependência (`go.mod`).

Decisão: testcontainers-go (alinhado com ADR-002 de identity). Build tag `//go:build integration`.

Suites:
- `infrastructure/repositories/postgres/subscription_repository_integration_test.go` — Upsert com índice único parcial, FindActiveByUserID com soft delete, ListByStatusInBatch com cursor estável.
- `infrastructure/repositories/postgres/webhook_event_repository_integration_test.go` — InsertIfNew com dedup, RecordApplication com idempotência, ListPendingAnonymization com janela de 365d, Anonymize verifica JSONB redacted.
- `application/usecases/ingest_kiwify_webhook_integration_test.go` — fluxo completo com Postgres real + outbox real (sem dispatcher): valida que 1 payload novo gera 1 webhook_events row + 1 outbox event; payload duplicado gera 0 outbox; assinatura inválida gera 0 inserts.
- `application/usecases/process_billing_event_integration_test.go` — registra handler no outbox.Registry, dispara Dispatcher manualmente, valida que sequência `compra_aprovada` → `ACTIVE`, sequência fora de ordem ignora stale, replay idempotente.
- `infrastructure/scheduler/anonymization_job_integration_test.go` — popula `webhook_events` com `received_at = NOW() - INTERVAL '366 days'`, dispara `Execute`, verifica payload redacted e `anonymized_at` preenchido (CA-12).

Estrutura segue `internal/platform/outbox/storage_pgx_integration_test.go` e `internal/identity/.../*_integration_test.go` (quando E1 materializado).

### Testes E2E

Não aplicável neste PRD. Smoke E2E (CA-07) coberto por integration tests do `ProcessBillingEventUseCase` orquestrando o fluxo `compra_aprovada → ACTIVE → check granted → subscription_canceled → granted até period_end → denied`.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Schema + migrations** (`0009_billing_schema`, `0010_billing_plans_seed`) — fundação física, validada por `RunMigrations` em testcontainer.
2. **Value Objects + StateMachine** (`domain/valueobjects/*.go`, `domain/services/state_machine.go`, `canonical_event.go`, `canonical_subscription.go`) — independentes, cobertura 100% antes de avançar.
3. **Agregado `Subscription`** (`domain/entities/subscription.go`) — depende de VOs, implementa contrato `identity.Subscription`.
4. **Errors** (`domain/errors.go`) — sentinelas consolidadas.
5. **Ports** (`application/interfaces/*.go`) — `SubscriptionRepository`, `WebhookEventRepository`, `BillingProvider`, `EntitlementCache`, `IDGenerator`, `UserResolver`.
6. **Use cases** (`application/usecases/*.go`) com mocks (regenerar `mockery --config .mockery.yml`).
7. **Cache LRU** (`infrastructure/cache/entitlement_lru.go`) — wrapper de `hashicorp/golang-lru/v2/expirable`.
8. **Adapter Kiwify** (`infrastructure/http/client/kiwify/*.go`) — client, oauth, signature verifier, adapter (impl `BillingProvider`), payload mapper.
9. **Repos Postgres** (`infrastructure/repositories/postgres/*.go`) — `subscription_repository.go`, `webhook_event_repository.go`, `queries.go`, `mapper.go`.
10. **Outbox event payload** (`infrastructure/outbox/event_payload.go`) — `EncodeReceivedPayload`/`DecodeReceivedPayload`.
11. **Outbox handler registrar** (`infrastructure/outbox/handler.go`) — registra `ProcessBillingEventUseCase.Handle` no `outbox.Registry` com event_type `billing.kiwify.received`.
12. **HTTP handler + RouteRegistrar** (`infrastructure/http/server/kiwify_webhook_handler.go`, `route_registrar.go`) — primeiro consumidor de `RouteRegistrar`; coordenar com alteração de `platform/http/server.go` (passo 13).
13. **`platform/http/server.go`** — adiciona `RouteRegistrar` interface + slice em `Deps` + iteração no `NewServer` (ADR-002).
14. **Scheduler subsystem** (`infrastructure/scheduler/*.go`) — registra reconciliation + anonymization no `robfig/cron/v3`.
15. **`platform/runtime/billing_subsystem.go`** — lazy subsystem que wireia tudo (repos, adapter, registrar, scheduler) e injeta em `cmd/server`.
16. **Configs** (`KiwifyConfig`, `BillingConfig` em `configs/config.go`) + env defaults + validação.
17. **depguard** — adicionar `billing-no-identity-infrastructure` em `.golangci.yml`.
18. **mockery.yml** — adicionar interfaces billing.
19. **Drift cleanup** (`internal/billing/{AGENTS.md, README.md, domain/doc.go, application/doc.go, infrastructure/doc.go}`).
20. **Testes de integração** com testcontainers e relatório de cobertura final.

### Dependências Técnicas

- Bloqueado por E1 (Identity Foundation) ter `status: implemented`. PRD pode escrever techspec/tasks em paralelo; execução espera.
- `github.com/hashicorp/golang-lru/v2 v2.x` — nova dependência direta.
- `github.com/robfig/cron/v3 v3.0.1` — já direta.
- `github.com/golang-migrate/migrate/v4` — já direta.
- `github.com/testcontainers/testcontainers-go/modules/postgres` — já direta.
- `github.com/jackc/pgx/v5`, `github.com/jackc/pgerrcode` — já diretas.
- `github.com/google/uuid v1.6.0` — já direta; usado em identity (E1). Billing usa UUID v4 para `Subscription.ID`, `WebhookEvent.ID`, `BillingEventApplication.event_id` (alinhamento cross-module, confirmado por probe ao `go.mod`).
- `github.com/vektra/mockery/v2 v2.53.6` — já direta (confirmado em `go.mod`).

## Monitoramento e Observabilidade

Logs estruturados via `log/slog` (R7.2) com mascaramento de PII obrigatório via `internal/platform/observability/mask`. Atributos canônicos:

```go
slog.InfoContext(ctx, "billing webhook recebido",
    slog.String("provider", "kiwify"),
    slog.String("webhook_event_id", webhookEvent.ID().String()),
    slog.String("external_event_id", webhookEvent.ExternalEventID().String()),
    slog.String("event_type", webhookEvent.EventType()),
)

slog.InfoContext(ctx, "billing subscription state changed",
    slog.String("subscription_id", sub.ID().String()),
    slog.String("user_id", sub.UserID().String()),
    slog.String("whatsapp_masked", mask.WhatsApp(canonical.Customer.WhatsApp.String())),
    slog.String("previous_state", previousState.String()),
    slog.String("new_state", sub.Status().String()),
    slog.String("plan_code", sub.PlanCode().String()),
)
```

**Inegociável:** nunca passar `whatsapp_number`, `email`, `cpf`, `card.*` em claro. `piiHandler` global é rede de segurança; código do módulo SEMPRE usa `mask.*` ou omite campo.

Métricas Prometheus (via OTel meter):

```go
billing_webhook_received_total{provider, event_type, outcome}        // counter
billing_webhook_ack_latency_seconds{provider}                        // histogram
billing_event_processed_total{event_type, outcome}                   // counter
billing_subscription_state_total{state}                              // gauge (scrape from DB)
billing_reconciliation_run_total{outcome}                            // counter
billing_reconciliation_divergence_total{state_local, state_remote}   // counter
entitlement_check_total{decision}                                    // counter
entitlement_cache_hit_ratio                                          // gauge
entitlement_check_latency_seconds                                    // histogram
billing_webhook_events_anonymized_total                              // counter
billing_webhook_events_pending_anonymization                         // gauge
kiwify_api_calls_total{endpoint, status}                             // counter
kiwify_oauth_refresh_total{outcome}                                  // counter
billing_period_divergence_total{plan_code, sign}                     // counter (ADR-011)
billing_event_lock_wait_seconds                                      // histogram (ADR-012)
billing_webhook_events_anonymization_errors_total                    // counter (ADR-013)
```

Spans OTel (RF-43):
- `billing.webhook.ingress`
- `billing.event.process`
- `billing.entitlement.check`
- `billing.reconciliation.tick`
- `billing.anonymization.tick`
- `kiwify.oauth.fetch_token`
- `kiwify.api.fetch_subscription`

Health checks: `database.Manager.HealthCheck` existente cobre Postgres. Sem health check específico de Kiwify no MVP (rate limit + alerta de métrica `kiwify_api_calls_total{status="5xx"}` cobre operacional).

## Considerações Técnicas

### Decisões Chave

Cada decisão material é registrada em ADR separada:

- [ADR-001 — Outbox event carrega pointer mínimo `{webhook_event_id, provider}`](./adr-001-outbox-pointer-payload.md)
- [ADR-002 — `RouteRegistrar` interface em `internal/platform/http` para mount de rotas de módulos](./adr-002-route-registrar-pattern.md)
- [ADR-003 — Scheduler dedicado em `infrastructure/scheduler` com `robfig/cron/v3` para reconciliation + anonymization](./adr-003-billing-scheduler-subsystem.md)
- [ADR-004 — Cache de entitlement com `hashicorp/golang-lru/v2/expirable`, TTL fixo de 5min](./adr-004-entitlement-cache-lru-expirable.md)
- [ADR-005 — Agregado `billing.Subscription` implementa `identity.domain.services.Subscription` por satisfação estrutural sem dependência circular](./adr-005-subscription-implements-identity-contract.md)
- [ADR-006 — `BillingProvider.VerifySignature` hexagonal com troca token → HMAC sem mudança de RF](./adr-006-signature-verifier-pluggable.md)
- [ADR-007 — Retenção two-tier de `webhook_events`: 365d íntegro + anonimização irreversível em-place](./adr-007-webhook-events-two-tier-retention.md)
- [ADR-008 — OAuth Kiwify: cache in-memory com TTL = `expires_in − 5min`, sem `refresh_token`, re-auth em `401`](./adr-008-kiwify-oauth-token-cache.md)
- [ADR-009 — `BillingEventProcessor` único mutador via `outbox.Handler`, idempotência por `billing_event_applications`](./adr-009-billing-event-processor-idempotency.md)
- [ADR-010 — State machine canônica como domain service stateless com transições exaustivas](./adr-010-state-machine-domain-service.md)
- [ADR-011 — `period_end` é fonte autoritativa do provider Kiwify (trust + alerta de divergência)](./adr-011-period-end-trust-provider.md)
- [ADR-012 — Concorrência por `SELECT ... FOR UPDATE` no `process_billing_event` (pessimistic lock)](./adr-012-process-event-pessimistic-lock.md)
- [ADR-013 — PII redactor em-process com parse-modify-marshal de `map[string]any`, sem nova dependência](./adr-013-pii-redactor-manual.md)

### Riscos Conhecidos

| Risco | Impacto | Mitigação |
|---|---|---|
| Contrato real do payload Kiwify divergir do assumido (`tracking.src`, `customer.mobile`, `subscription.id`) | Adapter rejeita payload legítimo ou interpreta errado | Implementação defensiva (PRD D-01): cascata em 5 campos tracking, fallback de `external_event_id` por hash SHA-256, métricas `billing_webhook_received_total{outcome="rejected_payload"}` com alerta em > 1% |
| Header de assinatura Kiwify ser HMAC-SHA256 em vez de token simples | `kiwify.ErrInvalidSignature` em 100% dos webhooks reais | Interface `VerifySignature` aceita `payload []byte` + `headers`; troca de impl para HMAC é localizada (ADR-006) |
| `golang-lru/v2/expirable` não suporta TTL per-entry — TTL é fixo na construção | Cache de entitlement não pode ser `min(period_end-now, 5min)` dinâmico | TTL fixo de 5min é aceitável (RF-33; janela máxima de stale igual ao TTL). Ganho de dinâmico é marginal (< 1% dos casos onde `period_end < 5min`); documentado em ADR-004 |
| Reconciliação eventualmente publica eventos sintéticos em loop se Kiwify devolver estado antigo persistentemente | Replay infinito + DLQ | Reconciliation só publica em **divergência detectada** (status local ≠ remoto OU period_end diferente); idempotência por `billing_event_applications` impede loop (mesmo external_event_id sintético processado uma vez) |
| Cache in-memory sem invalidação cross-pod causa janela "pago mas bloqueado" | UX ruim em multi-pod | TTL=5min limita pior caso; métrica `entitlement_cache_hit_ratio` por pod identifica padrão de stale; critério de revisão pós-MVP no PRD D-02 |
| `UpsertByWhatsAppNumber` chamado de `process_billing_event` cria User mesmo se evento for chargeback de venda nunca confirmada | Pollution de tabela `users` | Aceitável: User vai existir mas sem `Subscription` ativa; `EntitlementService.Check` retorna `denied`; soft delete LGPD futuro limpa órfãos (E1 entrega mecanismo de soft delete; runbook documenta limpeza) |
| `webhook_events.payload` é tabela de PII por design — vazamento em backup tem impacto LGPD | Multa LGPD em incidente | `REVOKE ALL ... FROM PUBLIC` aplicado na migration; backups herdam mesmas permissões; retenção two-tier (ADR-007) limita janela de exposição em 365d |
| Race condition em `process_billing_event` quando dois eventos do mesmo subscription_id chegam concorrentes | Versão final pode ser do evento errado | `billing_event_applications` PK por `event_id` garante idempotência por evento; ordem é respeitada por verificação de `occurred_at` (stale ignorado RF-25); concorrência real é baixa pelo throttle do Dispatcher (50 events/tick batch) |
| Migration `0009` aplica RESTRICT em FK `subscriptions.user_id → users.id` impedindo hard-delete de User | Hard delete proibido em users → bom alinhamento; mas alterar regra futura quebra constraint | Regra LGPD do projeto já é soft delete only (E1 ADR-009); RESTRICT explícito é defesa adicional |
| `expirable.LRU` evicta entries antes de TTL quando capacidade estoura — possível em alta cardinalidade de usuários | Hit ratio cai → mais Postgres lookups | Capacidade default 50k cobre 10x a meta de 5k subs ativas; alerta em `entitlement_cache_hit_ratio < 0.85` |

### Conformidade com Padrões

- `.claude/rules/governance.md` (R-GOV-001) — precedência respeitada; toda decisão material rastreada em ADR.
- `.agents/skills/agent-governance/references/ddd.md` (R-DDD-001) — `Subscription` é agregado com invariantes em construtor, transições explícitas via domain service (`StateMachine`), VOs imutáveis com auto-validação (`PlanCode`, `BillingPeriod`, `MoneyBRL`, `ExternalEventID`), sem struct anêmica.
- `.agents/skills/agent-governance/references/error-handling.md` (R-ERR-001) — sentinelas tipadas por camada, wrapping com `%w` em PT-BR, sem `panic`, classificação clara para outbox (`ErrPermanent` em parse failures).
- `.agents/skills/agent-governance/references/security.md` (R-SEC-001) — secrets via env (`KIWIFY_*`), nunca hardcoded; SafeKiwifyConfig redacta em logs; comparação de token `subtle.ConstantTimeCompare` (timing-safe); SQL parametrizado em todos os queries; input validado em VO antes de chegar a domínio.
- `.agents/skills/agent-governance/references/messaging.md` (R-MSG-001) — outbox pattern obrigatório (write tx → publish dentro da mesma tx); handler idempotente por `event_id`; dedup por `(provider, external_event_id)`; DLQ via `outbox.ErrPermanent`; trace context propagado por `outbox.Event.Headers`.
- `.agents/skills/agent-governance/references/observability.md` (R-OBS-001) — slog estruturado; trace_id propagado; PII mascarada; métricas com cardinalidade controlada (sem user_id como label); spans em fronteiras de IO.
- `.agents/skills/agent-governance/references/testing.md` (R-TEST-001) — table-driven via `testify/suite`, mocks via mockery, testcontainers para IO real, fuzz tests em VOs críticos, sem flakiness.
- `.agents/skills/go-implementation/SKILL.md` R0–R7:
  - R0: sem `init()` em qualquer arquivo.
  - R1: toda função é método de struct; `whatsAppNormalizer`-style helpers privados aceitos como exceção pragmática dentro do mesmo arquivo do VO/serviço.
  - R2: sem alias de campo (`name := user.Name`); copy direto inline.
  - R3: mocks via `mockery.yml` (adicionado em passo 18 do sequenciamento).
  - R4: testify/suite table-driven em 100% dos testes.
  - R5: Uber Style Guide — iota+1 (R5.8), early-return + switch (R5.21), `%w` wrapping em PT-BR (R5.10), grupos de import (R5.23), struct sem campos opcionais via Functional Options só onde > 3 opts.
  - R6.4: sem `var _ Interface = (*Type)(nil)` — satisfação verificada por uso real em wire.
  - R7: `log/slog` (R7.2), pacote `slices`/`maps` quando aplicável (R7.3/7.4), `any` em vez de `interface{}` (R7.1), `errors.Join` quando agregar erros de shutdown.
- `.agents/skills/object-calisthenics-go/references/rules.md`:
  - OC #3 (encapsular primitivos) — VOs `PlanCode`, `BillingPeriod`, `MoneyBRL`, `ExternalEventID`, `SubscriptionStatus`.
  - OC #5 (um ponto por linha) — mappers explícitos, sem method chains profundos.
  - OC #7 (entidades pequenas) — `Subscription` é o único agregado; demais arquivos têm responsabilidade única.
  - OC #9 (sem getters mecânicos) — métodos de intenção: `Activate`, `MarkPastDue`, `Cancel`, `Expire`, `Refund` (mutações ricas).
  - OC #8 (no aggregate over 2 fields exposed) — não aplicado a `Subscription` por ser aggregate root (exceção documentada na regra).
- AGENTS.md "Outbox vs events.Bus" — `outbox.Publisher` para mutações críticas (RF-05, RF-39); `events.Bus` apenas para `billing.subscription.state_changed` volátil (RF-27).
- `.golangci.yml` `depguard` — fronteiras hexagonais e cross-module enforçadas no CI.

### Arquivos Relevantes e Dependentes

Criados:
- `internal/billing/{AGENTS.md, README.md, doc.go}`
- `internal/billing/domain/{doc.go, errors.go}`
- `internal/billing/domain/entities/subscription.go`
- `internal/billing/domain/valueobjects/{plan_code, billing_period, subscription_status, canonical_event_type, transition_reason, external_event_id, external_subscription_id, money_brl, webhook_event_id}.go`
- `internal/billing/domain/services/{state_machine, canonical_event, canonical_subscription}.go`
- `internal/billing/application/{doc.go}`
- `internal/billing/application/dtos/{input,output}/*.go`
- `internal/billing/application/interfaces/{subscription_repository, webhook_event_repository, billing_provider, entitlement_cache, id_generator, user_resolver}.go`
- `internal/billing/application/interfaces/mocks/*.go` (gerados por mockery)
- `internal/billing/application/usecases/{ingest_kiwify_webhook, process_billing_event, check_entitlement, reconcile_subscriptions, anonymize_webhook_events}.go` + `*_test.go`
- `internal/billing/infrastructure/{doc.go}`
- `internal/billing/infrastructure/repositories/postgres/{subscription_repository, webhook_event_repository, queries, mapper}.go` + integration tests
- `internal/billing/infrastructure/http/server/{kiwify_webhook_handler, route_registrar}.go`
- `internal/billing/infrastructure/http/client/kiwify/{client, oauth, signature_verifier, adapter, payload_mapper}.go` + unit tests
- `internal/billing/infrastructure/cache/entitlement_lru.go`
- `internal/billing/infrastructure/outbox/{event_payload, handler}.go`
- `internal/billing/infrastructure/scheduler/{subsystem, reconciliation_job, anonymization_job}.go`
- `internal/billing/infrastructure/id/uuid_generator.go`
- `internal/platform/runtime/billing_subsystem.go`
- `migrations/{0009_billing_schema, 0010_billing_plans_seed}.{up,down}.sql`
- `.specs/prd-billing-pipeline/adr-001..adr-010.md`

Alterados:
- `internal/platform/http/server.go` — adiciona `RouteRegistrar` interface e iteração em `NewServer` (ADR-002).
- `internal/platform/runtime/http_subsystem.go` — propaga `[]RouteRegistrar` via `Deps`.
- `cmd/server/server.go` — bootstrap injeta registrars do billing.
- `configs/config.go` — `KiwifyConfig` + `BillingConfig` + defaults + validation.
- `.mockery.yml` — declara interfaces billing.
- `.golangci.yml` — adiciona `billing-no-identity-infrastructure`.
- `go.mod`/`go.sum` — adiciona `github.com/hashicorp/golang-lru/v2`.

Não alterados (fora de escopo):
- `internal/identity/**` — consumido como dependência hard.
- `internal/onboarding/**` — entrega de E3.
- `internal/notifications/**` — consumidor de `events.Bus` em PRD próprio.

## Plano de Rollout

### Pré-requisitos

- E1 (`prd-identity-foundation`) com `status: implemented` em main.
- Token OAuth Kiwify e Webhook Secret provisionados em vault para dev/staging/prod.
- Produtos criados no painel Kiwify com `kiwify_product_id` capturados.
- Webhook URL pública configurada no painel Kiwify por ambiente apontando para `/webhooks/kiwify`.

### Sequência

1. **Validação local** — `gofmt`, `golangci-lint run`, `go test ./... -race -count=1`, `mockery --config .mockery.yml --dry-run`, `go test -tags=integration ./internal/billing/...`.
2. **Pre-commit** — `ai-spec check-spec-drift .specs/prd-billing-pipeline/tasks.md` após `create-tasks`.
3. **Merge feature-branch → main** — CI verde (lint + unit + integration).
4. **Apply migrations 0009/0010 em staging** — operador roda `migrate up`; valida com `SELECT count(*) FROM billing_plans` (deve ser 3) e `\d webhook_events` / `\d subscriptions`.
5. **Provisão de secrets em staging** — env vars `KIWIFY_*` e `BILLING_*` setadas via vault.
6. **Deploy do binário em staging** — `cmd/server` inicia com `billing_subsystem` ativo; valida via `/health` que DB OK + via logs que scheduler iniciou (`billing reconciliation agendada @hourly`, `billing anonymization agendada @daily`).
7. **Smoke staging** — envia 1 webhook de teste assinado correto via `curl`; valida `200 OK`, row em `webhook_events`, evento em `outbox_events`, após 1s aplicação em `billing_event_applications` + linha em `subscriptions`. Envia o mesmo payload de novo; valida `204 No Content` + zero novas rows.
8. **Apply em produção** — janela operacional; mesmo procedimento. Migrations idempotentes (`ON CONFLICT DO NOTHING` no seed; `CREATE TABLE IF NOT EXISTS`).
9. **Rollback** — `migrate down 2` reverte `0010` e `0009` (em ambientes sem dados reais ainda; **se já houver subscriptions/webhook_events com dados de produção**, rollback DROPS tabelas — runbook obriga export prévio via `pg_dump --table` antes).
10. **Sinais de saúde pós-rollout** — `billing_webhook_received_total{outcome="accepted"}` cresce, `outcome="rejected_signature"` < 1%, `billing_event_processed_total{outcome="dlq"} = 0` por 24h, `entitlement_cache_hit_ratio > 0.85` em regime estável.

### Critério de Sucesso do Rollout

CA-01 a CA-12 do PRD atingidos:
- Ack < 2s p99 sob 10 req/s (CA-01)
- Idempotência por replay 5x (CA-02)
- Ordem fora-de-ordem (CA-03)
- Cobertura 100%/90% (CA-04)
- Entitlement < 5ms p99 (CA-05)
- Reconciliation converge < 5min após divergência simulada (CA-06)
- Smoke E2E feliz + cancelamento + denied pós period_end (CA-07)
- Zero PII em claro em logs (CA-08)
- depguard verde (CA-09)
- `check-spec-drift` verde (CA-10)
- Simulação 1000 webhooks/1h com 0 DLQ (CA-11)
- Anonimização aos 366d redacta PII (CA-12)

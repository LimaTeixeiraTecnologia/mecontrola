<!-- spec-hash-prd: 67f3ee7d23cca61bc3483523fe530f4c7051b41eea1fb1839f5bbd7052dd4506 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Outbox Transacional (Publisher Opt-in)

## Resumo Executivo

Implementação de uma fundação Outbox transacional **Postgres-only** no novo pacote `internal/infrastructure/outbox/`, exposto como um Publisher opt-in que persiste evento + N deliveries dentro da transação canônica do `UnitOfWork[T]` (ADR-002) e como um Subsystem agregador (`runtime.Subsystem`) que orquestra Dispatcher (goroutine + ticker) e Cron (housekeeping + reaper via `robfig/cron/v3@v3.0.1`) dentro do `cmd/worker`. A solução coexiste com o `events.Bus` (ADR-003) sem revogá-lo, reaproveita `events.EventID`/`events.EventName` como tipos canônicos, e adiciona schema two-table (`outbox_events` + `outbox_deliveries`) no schema `public` com coordenação multi-instância via `FOR UPDATE SKIP LOCKED`.

A modelagem aplica DDD pragmático (R-DDD-001): Value Objects imutáveis no boundary (`Event`, `DeliveryStatus`, `Attempt`, `BackoffPolicy`, `SubscriptionName`) com construtores validadores, sem aggregate pesado — Outbox é infraestrutura de mensageria, não bounded context de negócio. Strings primitivas são encapsuladas apenas onde carregam invariante de domínio (Object Calisthenics regra #3); colunas opacas como `payload` permanecem como `json.RawMessage` validadas no construtor. Errors usam sentinels exportados (`ErrPermanent`, `ErrHandlerNotRegistered`, `ErrDispatcherDisabled`) consumíveis via `errors.Is`/`errors.As` sem mapeamento RFC 7807 (caminho assíncrono, ADR-004 cobre apenas HTTP). Testes seguem R3+R4 da `go-implementation`: `mockery.yml` na raiz (criado por esta entrega — não existe hoje), suites `testify/suite` table-driven, integration com `testcontainers-go/modules/postgres` sob build tag `integration`, teste de concorrência com 3 dispatchers paralelos.

## Arquitetura do Sistema

### Visão Geral dos Componentes

**Novos componentes** (todos em `internal/infrastructure/outbox/`):

- `outbox.Event` — value object imutável com construtor validador `NewEvent`. Carrega `ID events.EventID`, `Type events.EventName`, `Version uint16`, `AggregateType string`, `AggregateID string`, `PartitionKey *string` (D-10), `Payload json.RawMessage` (validado com `json.Valid`), `Headers Headers`, `OccurredAt time.Time`. Não possui setters.
- `outbox.Headers` — VO `map[string]string` com método `WithTrace(traceparent string)`, `Get(key)`, `Validate()`. Garante chaves canônicas (`traceparent`, `correlation_id`, `causation_id`).
- `outbox.SubscriptionName` — VO encapsulando `string` com regra de formato `^[a-z][a-z0-9_-]{2,63}$`. Construtor `NewSubscriptionName(string)`.
- `outbox.DeliveryStatus` — VO de state machine; valores `pending`, `claimed`, `processed`, `dead_letter`. Métodos de transição `CanTransitionTo(next)` (State Pattern explícito, R-DDD-001).
- `outbox.Attempt` — VO `uint8` com método `Next() Attempt` e `IsExhausted(max Attempt) bool`. Encapsula primitivo (OC #3).
- `outbox.BackoffPolicy` — VO `{base, cap time.Duration, rng *rand.Rand}`. Método `NextRetryAt(attempt Attempt, now time.Time) time.Time` calcula `min(base * 2^attempt * (0.5 + rng.Float64()), cap)`. `rng` injetável para teste determinístico (D-13).
- `outbox.Subscription` — `{Name SubscriptionName, EventType events.EventName, Handler Handler}` resolvida em build time.
- `outbox.Handler` — `type Handler func(ctx context.Context, evt Event) error`. Idempotência por `event.ID` é regra obrigatória documentada no godoc.
- `outbox.Registry` — registro estático `map[events.EventName][]Subscription` populado no bootstrap; método `Register(s Subscription) error` (valida duplicidade `(Name, EventType)`, D-09); `SubscriptionsFor(t events.EventName) []Subscription`; `Validate() error` chamado uma vez no `Start`.
- `outbox.Publisher` — `Publish(ctx, tx database.DBTX, evt Event) error`. Recebe a `database.DBTX` exposta pelo `UnitOfWork[T].Do` (ADR-002). Consulta `Registry.SubscriptionsFor`, insere `outbox_events` (1 linha) + N `outbox_deliveries` (1 por subscription) na **mesma `tx`**. Retorna `ErrHandlerNotRegistered` se não houver handler.
- `outbox.Storage` — interface no mesmo pacote (D-14): `InsertEvent`, `InsertDeliveries`, `ClaimReady(ctx, batchSize int, instanceID string) ([]Claim, error)`, `MarkProcessed`, `MarkFailed`, `MarkDLQ`, `ReleaseStuck(ctx, olderThan time.Time) (int64, error)`, `PurgeOlderThan(ctx, olderThan time.Time) (int64, error)`, `Stats(ctx) (Stats, error)`.
- `outbox.PgxStorage` — implementação concreta em `storage_pgx.go` consumindo `database.DBTX` para queries fora de transação (via `Manager.Inner().DBTX(ctx)`) e a `tx` passada para inserts no Publisher.
- `outbox.Dispatcher` — loop principal. `time.Ticker` no intervalo configurado; cada tick chama `Storage.ClaimReady` (transação curta), itera os Claims, executa `Handler` com timeout via `context.WithTimeout`, e marca o resultado. Usa `sync.WaitGroup` para esperar handlers in-flight no shutdown.
- `outbox.Cron` — wrapper de `robfig/cron/v3` registrando dois jobs: `OUTBOX_HOUSEKEEPING_SCHEDULE` (`@daily` default) → `Storage.PurgeOlderThan(retention)`; `OUTBOX_REAPER_INTERVAL` (`@every 1m` default) → `Storage.ReleaseStuck(stuckAfter)`.
- `outbox.Subsystem` — agregador que implementa `runtime.Subsystem` (`Start`/`Stop`/`Name() string` retornando `"outbox"`). Composição com `errgroup.WithContext` (D-15): Start lança Dispatcher + Cron; Stop cancela ticker, chama `cron.Stop(ctx)`, espera handlers via WaitGroup, retorna erros via `errors.Join`. Respeita `OUTBOX_DISPATCHER_ENABLED=false` não iniciando o loop (Cron continua para housekeeping).
- `outbox.Metrics` — fachada OTel sobre `observability.Observability` (Provider já existente). Instrumenta counters/histograms/gauges com label `subscription_name`.
- `outbox.Config` — bind Viper para todas as chaves `OUTBOX_*` (RF-26).
- `OutboxConfig` em `configs/config.go` — novo grupo agregado a `configs.Config` via `mapstructure:",squash"`.

**Componentes modificados**:

- `configs.Config` (`configs/config.go`) — adiciona campo `OutboxConfig OutboxConfig \`mapstructure:",squash"\`` e respectivas chaves no `envKeys` + `SetDefault` do `configLoader`.
- `runtime.Bootstrap` (`internal/infrastructure/runtime/bootstrap.go`) — `buildSubsystems(ModeWorker)` passa de `[]Subsystem{}` para `[]Subsystem{b.newOutboxSubsystem(cfg, foundation)}`.
- `migrations/0002_outbox.up.sql` + `0002_outbox.down.sql` — schema two-table no schema `public` (D-07).
- `mockery.yml` (raiz, **criado**) — declara `outbox.Storage`, `outbox.Handler`, `outbox.Registry` para geração de mocks (D-16).
- `Taskfile.yml` — nova tarefa `task mocks` rodando `mockery --config mockery.yml`.
- `.github/PULL_REQUEST_TEMPLATE.md` (raiz `.github/`, **criado**) — RF-40.
- `AGENTS.md` raiz e `CLAUDE.md` — seção "Outbox vs events.Bus" (RF-38).
- `internal/infrastructure/outbox/AGENTS.md` (**criado**) — RF-38.
- `.specs/prd-mecontrola-foundation/adr-016-outbox-publisher-opt-in.md` — RF-37/D-12.

**Relacionamentos chave**:

```
cmd/worker → runtime.Bootstrap(cfg, ModeWorker) → buildSubsystems
                                                       ↓
                              [outbox.Subsystem] (runtime.Subsystem)
                              ├─ outbox.Dispatcher (goroutine + ticker)
                              │     ↓ uses
                              │   outbox.Storage  ←─── outbox.PgxStorage
                              │                        (consome Manager.Inner().DBTX(ctx))
                              │     ↓ uses
                              │   outbox.Registry (estático, populado no bootstrap)
                              │     ↓ executes
                              │   outbox.Handler (idempotente por event.ID)
                              │
                              └─ outbox.Cron (robfig/cron/v3@v3.0.1)
                                    ├─ housekeeping @daily → Storage.PurgeOlderThan
                                    └─ reaper @every 1m   → Storage.ReleaseStuck

use case do agregado → UnitOfWork[T].Do(ctx, fn)
                                        ↓ fn recebe database.DBTX
                              outbox.Publisher.Publish(ctx, tx, evt)
                                        ↓
                              Storage.InsertEvent(tx, ...) + InsertDeliveries(tx, ...)
                                        ↓
                              tx.Commit() (responsabilidade do UoW)
```

### Fronteiras Application / Domain / Infrastructure

Este pacote é infraestrutura por natureza (mensageria). Reconhecemos três sub-camadas internas:

| Sub-camada | Arquivos | Responsabilidade |
|---|---|---|
| **Domain (VOs + invariantes)** | `event.go`, `headers.go`, `subscription.go`, `delivery_status.go`, `attempt.go`, `backoff_policy.go`, `errors.go` | Tipos imutáveis com construtor validador; sem dependência de `pgx`, `cron` ou `otel`. Pode ser importado por qualquer camada. |
| **Application (orquestração)** | `publisher.go`, `dispatcher.go`, `registry.go`, `cron.go`, `subsystem.go`, `handler.go` | Coordena fluxo (claim → execute → mark); depende de `Storage` (porta) e `Metrics`. Não conhece SQL. |
| **Infrastructure (adapter)** | `storage_pgx.go`, `storage.go` (interface), `metrics.go`, `config.go` | Implementação concreta com `pgx/v5`, OTel SDK, Viper. Único caminho com SQL. |

**Regra de import**: arquivos `*_pgx.go` e `metrics.go` podem importar `pgx`/`otel`; demais arquivos não. Validação posterior via `go-arch-lint` ou revisão manual.

## Design de Implementação

### Interfaces Chave

```go
// internal/infrastructure/outbox/handler.go
type Handler func(ctx context.Context, evt Event) error

// internal/infrastructure/outbox/publisher.go
type Publisher interface {
    Publish(ctx context.Context, tx database.DBTX, evt Event) error
}

// internal/infrastructure/outbox/storage.go
type Storage interface {
    InsertEvent(ctx context.Context, tx database.DBTX, evt Event) error
    InsertDeliveries(ctx context.Context, tx database.DBTX, evtID events.EventID, subs []SubscriptionName) error
    ClaimReady(ctx context.Context, batchSize int, instanceID string) ([]Claim, error)
    MarkProcessed(ctx context.Context, id ClaimID, processedAt time.Time) error
    MarkFailed(ctx context.Context, id ClaimID, lastErr string, nextAttempt Attempt, nextRetryAt time.Time) error
    MarkDLQ(ctx context.Context, id ClaimID, lastErr string, deadLetterAt time.Time) error
    ReleaseStuck(ctx context.Context, olderThan time.Time) (int64, error)
    PurgeOlderThan(ctx context.Context, olderThan time.Time) (int64, error)
    Stats(ctx context.Context) (Stats, error)
}

// internal/infrastructure/outbox/registry.go
type Registry interface {
    Register(s Subscription) error
    SubscriptionsFor(eventType events.EventName) []Subscription
    Validate() error
}
```

### Modelos de Dados

#### Value Objects (domain)

```go
// event.go
type Event struct {
    id            events.EventID
    eventType     events.EventName
    version       uint16
    aggregateType string
    aggregateID   string
    partitionKey  *stringse
    payload       json.RawMessage
    headers       Headers
    occurredAt    time.Time
}

func NewEvent(p NewEventParams) (Event, error) {
    if p.ID.String() == "" { return Event{}, fmt.Errorf("outbox: event id obrigatorio") }
    if p.AggregateType == "" { return Event{}, fmt.Errorf("outbox: aggregate type obrigatorio") }
    if !json.Valid(p.Payload) { return Event{}, fmt.Errorf("outbox: payload nao e JSON valido") }
    if p.Version == 0 { p.Version = 1 }
    if p.OccurredAt.IsZero() { p.OccurredAt = time.Now().UTC() }
    return Event{ /* copia campos */ }, nil
}

// Getters de leitura apenas (R-DDD-001: campos não exportados, getters com intenção)
func (e Event) ID() events.EventID         { return e.id }
func (e Event) Type() events.EventName     { return e.eventType }
// ... demais getters
```

```go
// delivery_status.go — State Pattern explícito
type DeliveryStatus struct{ value string }

var (
    StatusPending    = DeliveryStatus{"pending"}
    StatusClaimed    = DeliveryStatus{"claimed"}
    StatusProcessed  = DeliveryStatus{"processed"}
    StatusDeadLetter = DeliveryStatus{"dead_letter"}
)

func (s DeliveryStatus) CanTransitionTo(next DeliveryStatus) bool {
    switch s {
    case StatusPending:    return next == StatusClaimed
    case StatusClaimed:    return next == StatusProcessed || next == StatusPending || next == StatusDeadLetter
    case StatusProcessed:  return false
    case StatusDeadLetter: return next == StatusPending // re-enfileiramento manual
    }
    return false
}
```

```go
// backoff_policy.go
type BackoffPolicy struct {
    base time.Duration
    cap  time.Duration
    rng  *rand.Rand
}

func NewBackoffPolicy(base, cap time.Duration, rng *rand.Rand) (BackoffPolicy, error) {
    if base <= 0 || cap <= 0 || base > cap { return BackoffPolicy{}, fmt.Errorf("outbox: backoff invalido") }
    if rng == nil { rng = rand.New(rand.NewSource(time.Now().UnixNano())) }
    return BackoffPolicy{base: base, cap: cap, rng: rng}, nil
}

func (p BackoffPolicy) NextRetryAt(attempt Attempt, now time.Time) time.Time {
    factor := math.Pow(2, float64(attempt))
    jitter := 0.5 + p.rng.Float64() // [0.5, 1.5)
    delay := time.Duration(float64(p.base) * factor * jitter)
    if delay > p.cap { delay = p.cap }
    return now.Add(delay)
}
```

#### Schema SQL — `migrations/0002_outbox.up.sql`

```sql
-- migration: 0002_outbox.up.sql
-- Cria o substrato Transactional Outbox: tabela imutavel de eventos +
-- tabela de deliveries (uma por subscription) com indices para claim, housekeeping e reaper.
-- Schema: public (D-07). Sem extensoes externas — apenas pgx/v5 nativo.

CREATE TABLE IF NOT EXISTS outbox_events (
    id              TEXT PRIMARY KEY,              -- ULID (events.EventID)
    event_type      TEXT NOT NULL,                 -- events.EventName (<modulo>.<acao>)
    event_version   SMALLINT NOT NULL DEFAULT 1,
    aggregate_type  TEXT NOT NULL,
    aggregate_id    TEXT NOT NULL,
    partition_key   TEXT NULL,                     -- D-10: reservada para V2
    payload         JSONB NOT NULL,
    headers         JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at     TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_outbox_events_type_aggregate
    ON outbox_events (event_type, aggregate_id);

CREATE TABLE IF NOT EXISTS outbox_deliveries (
    id                BIGSERIAL PRIMARY KEY,       -- ORDER BY id no claim
    event_id          TEXT NOT NULL REFERENCES outbox_events(id) ON DELETE CASCADE,
    subscription_name TEXT NOT NULL,
    status            TEXT NOT NULL CHECK (status IN ('pending','claimed','processed','dead_letter')),
    attempts          SMALLINT NOT NULL DEFAULT 0,
    next_retry_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error        TEXT NULL,
    processed_at      TIMESTAMPTZ NULL,
    dead_letter_at    TIMESTAMPTZ NULL,
    claimed_at        TIMESTAMPTZ NULL,
    claimed_by        TEXT NULL,                   -- D-11: hostname-pid
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_outbox_deliveries_event_subscription UNIQUE (event_id, subscription_name)
);

-- Index para o claim do Dispatcher (RF-10).
CREATE INDEX IF NOT EXISTS ix_outbox_deliveries_claim_ready
    ON outbox_deliveries (status, next_retry_at, id)
    WHERE status = 'pending';

-- Index para queries operacionais por subscription.
CREATE INDEX IF NOT EXISTS ix_outbox_deliveries_subscription_status
    ON outbox_deliveries (subscription_name, status);

-- Index parcial para housekeeping diario (RF-18).
CREATE INDEX IF NOT EXISTS ix_outbox_deliveries_housekeeping
    ON outbox_deliveries (COALESCE(processed_at, dead_letter_at))
    WHERE status IN ('processed','dead_letter');

-- Index parcial para reaper (RF-19).
CREATE INDEX IF NOT EXISTS ix_outbox_deliveries_claimed_stuck
    ON outbox_deliveries (claimed_at)
    WHERE status = 'claimed';
```

`migrations/0002_outbox.down.sql`:

```sql
DROP INDEX IF EXISTS ix_outbox_deliveries_claimed_stuck;
DROP INDEX IF EXISTS ix_outbox_deliveries_housekeeping;
DROP INDEX IF EXISTS ix_outbox_deliveries_subscription_status;
DROP INDEX IF EXISTS ix_outbox_deliveries_claim_ready;
DROP TABLE IF EXISTS outbox_deliveries;
DROP INDEX IF EXISTS ix_outbox_events_type_aggregate;
DROP TABLE IF EXISTS outbox_events;
```

#### Query de claim (RF-10/RF-14)

```sql
-- ClaimReady — única forma de pegar deliveries. Idempotente sob crash via reaper.
UPDATE outbox_deliveries d
   SET status = 'claimed',
       claimed_at = now(),
       claimed_by = $1,
       attempts = d.attempts + 1,
       updated_at = now()
 WHERE d.id IN (
       SELECT id FROM outbox_deliveries
        WHERE status = 'pending'
          AND next_retry_at <= now()
        ORDER BY id
        LIMIT $2
        FOR UPDATE SKIP LOCKED
 )
 RETURNING d.id, d.event_id, d.subscription_name, d.attempts;
```

Em seguida, o Dispatcher faz `SELECT` no `outbox_events` por `event_id` para hidratar a `Event` antes de executar o handler.

#### Query do reaper (D-17, evita race com Dispatcher)

```sql
UPDATE outbox_deliveries
   SET status = 'pending',
       claimed_by = NULL,
       claimed_at = NULL,
       updated_at = now()
 WHERE id IN (
       SELECT id FROM outbox_deliveries
        WHERE status = 'claimed'
          AND claimed_at < $1   -- now() - stuckAfter
        ORDER BY id
        FOR UPDATE SKIP LOCKED
 )
 RETURNING id;
```

#### Query do housekeeping

```sql
DELETE FROM outbox_deliveries
 WHERE status IN ('processed','dead_letter')
   AND COALESCE(processed_at, dead_letter_at) < now() - ($1::interval);

-- Eventos órfãos (sem deliveries restantes) são apagados em segundo passo:
DELETE FROM outbox_events e
 WHERE NOT EXISTS (SELECT 1 FROM outbox_deliveries d WHERE d.event_id = e.id)
   AND e.created_at < now() - ($1::interval);
```

#### Tipos auxiliares

```go
// claim.go
type Claim struct {
    ID              ClaimID            // bigserial
    Event           Event              // hidratado via SELECT outbox_events
    SubscriptionName SubscriptionName
    Attempt         Attempt
    ClaimedAt       time.Time
}

type ClaimID int64

// stats.go — para métrica de gauge `deliveries_pending`
type Stats struct {
    Pending     map[SubscriptionName]int64
    DeadLetter  map[SubscriptionName]int64
    OldestPendingAt time.Time
}
```

### Configuração (RF-26 / D-03 / D-05)

```go
// configs/config.go (modificação)
type Config struct {
    AppConfig    AppConfig    `mapstructure:",squash"`
    HTTPConfig   HTTPConfig   `mapstructure:",squash"`
    DBConfig     DBConfig     `mapstructure:",squash"`
    O11yConfig   O11yConfig   `mapstructure:",squash"`
    OutboxConfig OutboxConfig `mapstructure:",squash"`
}

type OutboxConfig struct {
    DispatcherEnabled        bool          `mapstructure:"OUTBOX_DISPATCHER_ENABLED"`
    DispatcherTickInterval   time.Duration `mapstructure:"OUTBOX_DISPATCHER_TICK_INTERVAL"`
    DispatcherBatchSize      int           `mapstructure:"OUTBOX_DISPATCHER_BATCH_SIZE"`
    DispatcherHandlerTimeout time.Duration `mapstructure:"OUTBOX_DISPATCHER_HANDLER_TIMEOUT"`
    RetryMaxAttempts         int           `mapstructure:"OUTBOX_RETRY_MAX_ATTEMPTS"`
    RetryBaseBackoff         time.Duration `mapstructure:"OUTBOX_RETRY_BASE_BACKOFF"`
    RetryMaxBackoff          time.Duration `mapstructure:"OUTBOX_RETRY_MAX_BACKOFF"`
    HousekeepingRetentionDays int          `mapstructure:"OUTBOX_HOUSEKEEPING_RETENTION_DAYS"`
    HousekeepingSchedule     string        `mapstructure:"OUTBOX_HOUSEKEEPING_SCHEDULE"`
    ReaperInterval           string        `mapstructure:"OUTBOX_REAPER_INTERVAL"`
    ReaperStuckAfter         time.Duration `mapstructure:"OUTBOX_REAPER_STUCK_AFTER"`
}
```

Defaults aplicados no `configLoader.load()`:

| Chave | Default |
|---|---|
| `OUTBOX_DISPATCHER_ENABLED` | `true` |
| `OUTBOX_DISPATCHER_TICK_INTERVAL` | `500ms` |
| `OUTBOX_DISPATCHER_BATCH_SIZE` | `50` |
| `OUTBOX_DISPATCHER_HANDLER_TIMEOUT` | `10s` |
| `OUTBOX_RETRY_MAX_ATTEMPTS` | `15` |
| `OUTBOX_RETRY_BASE_BACKOFF` | `2s` |
| `OUTBOX_RETRY_MAX_BACKOFF` | `5m` |
| `OUTBOX_HOUSEKEEPING_RETENTION_DAYS` | `90` |
| `OUTBOX_HOUSEKEEPING_SCHEDULE` | `@daily` |
| `OUTBOX_REAPER_INTERVAL` | `@every 1m` |
| `OUTBOX_REAPER_STUCK_AFTER` | `5m` |

Validação adicional em `Config.Validate()`: `RetryMaxAttempts in [1..50]`, `DispatcherBatchSize in [1..500]`, `HousekeepingRetentionDays in [1..3650]`, parse-check de `HousekeepingSchedule` e `ReaperInterval` via `cron.ParseStandard` para falhar fast no boot.

### Endpoints de API

Não aplicável — Outbox é caminho assíncrono interno, sem superfície HTTP no MVP.

## Pontos de Integração

### `runtime.Subsystem` (RF-09 / RF-39)

`internal/infrastructure/runtime/outbox_subsystem.go` (novo):

```go
// outbox_subsystem.go (esqueleto)
type lazyOutboxSubsystem struct {
    cfg     *configs.Config
    found   Foundation
    sub     *outbox.Subsystem
    dbMgr   *database.Manager
    closers []func(context.Context) error
}

func (b *bootstrapper) newOutboxSubsystem(cfg *configs.Config, f Foundation) *lazyOutboxSubsystem {
    return &lazyOutboxSubsystem{cfg: cfg, found: f}
}

func (s *lazyOutboxSubsystem) Name() string { return "outbox" }

func (s *lazyOutboxSubsystem) Start(ctx context.Context) error {
    mgr, err := database.NewManager(s.cfg)
    if err != nil { return fmt.Errorf("outbox subsystem: database: %w", err) }
    s.dbMgr = mgr
    s.closers = append(s.closers, mgr.Shutdown)

    prov, shutProv, err := observability.NewProvider(s.cfg)
    if err != nil { _ = mgr.Shutdown(context.Background()); return fmt.Errorf("outbox subsystem: observability: %w", err) }
    s.closers = append(s.closers, shutProv)

    registry := outbox.NewRegistry()
    if err := registerSubscriptions(registry); err != nil {
        return fmt.Errorf("outbox subsystem: registry: %w", err)
    }

    sub, err := outbox.NewSubsystem(outbox.SubsystemDeps{
        Config:   s.cfg.OutboxConfig,
        Storage:  outbox.NewPgxStorage(mgr.Inner()),
        Registry: registry,
        Metrics:  outbox.NewMetrics(prov.Observability()),
        Logger:   slog.Default(),
        Clock:    s.found.Clock,
        InstanceID: outbox.NewInstanceID(),  // hostname + pid (D-11)
    })
    if err != nil { return fmt.Errorf("outbox subsystem: build: %w", err) }
    s.sub = sub
    return sub.Start(ctx)
}

func (s *lazyOutboxSubsystem) Stop(ctx context.Context) error {
    var errs []error
    if s.sub != nil {
        if err := s.sub.Stop(ctx); err != nil { errs = append(errs, fmt.Errorf("outbox shutdown: %w", err)) }
    }
    for i := len(s.closers) - 1; i >= 0; i-- {
        if err := s.closers[i](ctx); err != nil { errs = append(errs, err) }
    }
    if len(errs) > 0 { return errors.Join(errs...) }
    return nil
}
```

A subscription do handler dummy (FC-10) é registrada em `registerSubscriptions` — função privada do pacote `runtime` que recebe o registry vazio e adiciona `Subscription{Name: "outbox.dummy", EventType: events.MustEventName("platform.outbox-dummy"), Handler: outbox.DummyHandler}`.

### Estratégia de Erros (RF-13 + R-ERR-001)

`internal/infrastructure/outbox/errors.go`:

```go
package outbox

import "errors"

// ErrPermanent sinaliza falha terminal do handler. Dispatcher transita imediatamente para DLQ
// sem consumir tentativas. Handler deve retornar wrappando: fmt.Errorf("%w: payload schema v2", outbox.ErrPermanent).
var ErrPermanent = errors.New("outbox: erro permanente")

// ErrHandlerNotRegistered retornado por Publisher.Publish quando event_type sem subscription.
var ErrHandlerNotRegistered = errors.New("outbox: nenhum handler registrado para event_type")

// ErrDispatcherDisabled retornado por operações dependentes do loop quando OUTBOX_DISPATCHER_ENABLED=false.
var ErrDispatcherDisabled = errors.New("outbox: dispatcher desabilitado")

// ErrDuplicateSubscription retornado por Registry.Register quando (Name, EventType) já registrados (D-09).
var ErrDuplicateSubscription = errors.New("outbox: subscription duplicada (name, event_type)")

// ErrInvalidEvent retornado por NewEvent quando invariantes do construtor falham.
var ErrInvalidEvent = errors.New("outbox: evento invalido")
```

Diretrizes (R-ERR-001):
- Todos os wrappers usam `fmt.Errorf("contexto: %w", innerErr)`.
- Handler classifica seu próprio erro: retorna `outbox.ErrPermanent` ou erro transitório nu. Dispatcher usa `errors.Is(err, outbox.ErrPermanent)`.
- Caminho HTTP que esbarrar nesses sentinels via use case (improvável, mas possível) cai em `*errors.ToProblemDetails` retornando `500 internal_server_error` — esses sentinels deliberadamente **não** são mapeados em RFC 7807 (decisão registrada em PRD RF-13).

## Abordagem de Testes

Aderência **estrita** a R3 e R4 da `go-implementation/references/testing.md`. Como `mockery.yml` não existe hoje, esta entrega o cria como pré-requisito (D-16).

### Testes Unitários (`go test ./internal/infrastructure/outbox/...`)

- **VOs**: `event_test.go`, `headers_test.go`, `subscription_test.go`, `delivery_status_test.go`, `backoff_policy_test.go`. Cada arquivo segue R4 (suite + table-driven) cobrindo: construção válida, validação de invariante, transições inválidas (state machine).
- **Publisher**: `publisher_test.go` com mock de `Storage` e `Registry`. Cenários:
  - "deve publicar evento com 1 handler com sucesso" (insere 1 evento + 1 delivery)
  - "deve publicar evento com 3 handlers" (insere 1 evento + 3 deliveries)
  - "deve retornar ErrHandlerNotRegistered quando event_type sem subscription"
  - "deve retornar erro de storage wrappado quando InsertEvent falha"
  - "deve preservar a tx do caller (não cria nova)"
- **Dispatcher**: `dispatcher_test.go` com mock de `Storage`, fakes de `Handler` (success/transient/permanent/panic), `Clock` injetável. Cenários:
  - "deve marcar processed em handler de sucesso"
  - "deve incrementar attempts e calcular nextRetryAt em erro transitório"
  - "deve transitar para DLQ em erro permanente sem consumir tentativas"
  - "deve transitar para DLQ após esgotar attempts (15)"
  - "deve aplicar timeout de handler como falha transitória" (`context.DeadlineExceeded`)
  - "deve recuperar de panic do handler como erro permanente" (via `defer recover`)
  - "não deve iniciar loop quando OUTBOX_DISPATCHER_ENABLED=false"
  - "deve drenar handlers in-flight no Stop respeitando ctx"
- **BackoffPolicy**: `backoff_policy_test.go` com `rand.Rand` semente fixa. Verificar limites: attempt=0 → ~base; attempt=15 → cap (300s).
- **Registry**: cenários de duplicidade, validação, `SubscriptionsFor` com 0/1/N handlers.

Mocks gerados via `mockery --config mockery.yml`. `mockery.yml` declara:

```yaml
with-expecter: true
mockname: "{{.InterfaceName}}"
outpkg: "mocks"
filename: "{{.InterfaceName | snakecase}}.go"
dir: "{{.InterfaceDir}}/mocks"
packages:
  github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/outbox:
    interfaces:
      Storage:
      Registry:
```

(`Handler` é função, não interface — não vai para mockery; fakes manuais em `internal/infrastructure/outbox/fakes/handler.go` para teste).

### Testes de Integração (build tag `integration`)

`storage_pgx_integration_test.go`, `subsystem_integration_test.go`, `concurrency_integration_test.go`. Build tag `//go:build integration` para separar de `go test ./...`. `testcontainers-go/modules/postgres` provisiona container efêmero por suite.

- **Storage**: roundtrip de `InsertEvent` + `InsertDeliveries` + `ClaimReady` + `MarkProcessed`/`MarkFailed`/`MarkDLQ` + `ReleaseStuck` + `PurgeOlderThan`. Verifica:
  - Constraint `uq_outbox_deliveries_event_subscription` impede duplicidade (RF-04).
  - `ClaimReady` respeita `next_retry_at` e ordena por `id`.
  - `PurgeOlderThan` apaga deliveries e eventos órfãos.
- **Subsystem ponta-a-ponta**: `subsystem_integration_test.go` com handler dummy + 1 evento. Verifica ciclo `publish → claim → handler → processed` < 2s. Cobre US-01.
- **Concorrência** (RF-35): `concurrency_integration_test.go` com **3 dispatchers** rodando no mesmo Postgres processando massa pré-populada de 1000 deliveries. Verifica:
  - Zero `processed` duplicado (`SELECT event_id, subscription_name, COUNT(*) FROM outbox_deliveries WHERE status='processed' GROUP BY 1,2 HAVING COUNT(*) > 1` retorna vazio).
  - Throughput agregado ≥ 100 deliveries/s.
- **Reaper**: insere delivery `status='claimed'` com `claimed_at = now()-10m`, executa reaper, verifica que volta para `pending` e Dispatcher seguinte a processa.

### Testes E2E

Não aplicável — não há UI nem API externa.

### Benchmarks (RF-36)

- `BenchmarkPublisher_Publish` mede p95 do `Publish` em transação aberta com 1, 3 e 5 handlers registrados.
- `BenchmarkDispatcher_DrainBacklog` mede throughput sustentado drenando 10k deliveries pré-populadas com handler dummy de 10ms.
- Resultados anotados em `docs/benchmarks/outbox-baseline.md` antes do go-live para confronto pós-deploy.

## Sequenciamento de Desenvolvimento

### Ordem de Build (mapeada em 3 fases, alinhada com discovery)

| Fase | Componentes | Critério de pronto |
|---|---|---|
| **1 — Fundação** | Migration `0002_outbox`, `event.go`, `headers.go`, `subscription.go`, `delivery_status.go`, `attempt.go`, `backoff_policy.go`, `errors.go`, `storage.go` (interface), `storage_pgx.go`, `mockery.yml` (raiz), `Taskfile.yml` (`task mocks`) | `go test -tags integration ./internal/infrastructure/outbox/...` verde para Storage; mocks gerados; ADR-016 escrita |
| **2 — Publisher + Dispatcher** | `publisher.go`, `registry.go`, `handler.go`, `dispatcher.go`, `metrics.go`, `instance_id.go`, `subsystem.go` (parcial só com Dispatcher) | Unit tests do Publisher + Dispatcher verdes (suite + table-driven); integration ponta-a-ponta com handler dummy verde |
| **3 — Cron + Bootstrap + Docs** | `cron.go`, `subsystem.go` (completo), `outbox_subsystem.go` em `runtime/`, atualização do `buildSubsystems(ModeWorker)`, dummy handler + subscription, `OutboxConfig` em `configs.Config`, `.github/PULL_REQUEST_TEMPLATE.md`, `internal/infrastructure/outbox/AGENTS.md`, atualizações em `AGENTS.md`/`CLAUDE.md` raiz, runbook em `docs/runbooks/outbox.md`, dashboard em `docs/observability/outbox-dashboard.json` | Teste de concorrência (3 dispatchers) verde; benchmark documentado; runbook publicado; PR template ativo |

### Dependências Técnicas

- `github.com/robfig/cron/v3@v3.0.1` (única dep nova — D-04). Adicionar via `go get` na Fase 3.
- `golang.org/x/sync/errgroup` — já transitivamente disponível via `pgx`; verificar explicitamente em `go.mod`.
- `mockery v2.53.6` já presente; usar tarefa `task mocks`.
- Migration aplicada via `cmd/migrate` no pipeline de deploy padrão.

## Monitoramento e Observabilidade

### Métricas OTel (RF-21)

Todas com label `subscription_name` exceto `events.published.total` (label `event_type`) e `poll.*` (sem label de subscription).

| Métrica | Tipo | Labels | Propósito |
|---|---|---|---|
| `outbox.events.published.total` | counter | `event_type` | Volume de eventos persistidos por tipo |
| `outbox.deliveries.pending` | gauge | `subscription_name` | Tamanho da fila por subscription (gauge atualizado a cada 30s via `Storage.Stats`) |
| `outbox.deliveries.processed.total` | counter | `subscription_name` | Deliveries concluídas com sucesso |
| `outbox.deliveries.failed.total` | counter | `subscription_name`, `error_class` | Deliveries em falha transitória |
| `outbox.deliveries.dlq.total` | counter | `subscription_name` | Transições para DLQ |
| `outbox.delivery.latency_ms` | histogram | `subscription_name` | `now() - event.occurred_at` no fechamento (SLO: p95 < 1s) |
| `outbox.poll.duration_ms` | histogram | — | Custo do `ClaimReady` |
| `outbox.poll.batch_size` | histogram | — | Itens retornados por ciclo |
| `outbox.reaper.released.total` | counter | — | Linhas liberadas pelo reaper |
| `outbox.housekeeping.deleted.total` | counter | — | Linhas apagadas pelo housekeeping |

`error_class` em `failed.total` deriva de `errors.As` para classes conhecidas; valores unbounded são bucketizados em `"transient"`, `"timeout"`, `"permanent"`, `"panic"`, `"unknown"` (controle de cardinalidade, R-OBS-001).

### Logs `slog` (RF-23 + RF-24)

Todos os logs passam pelo `RedactingSlogHandler` já configurado em `observability.NewProvider`. Política:

- `INFO outbox.dispatcher.started` (boot) — campos: `tick_interval`, `batch_size`, `instance_id`.
- `INFO outbox.delivery.processed` (sampled 1:100) — `event_id`, `event_type`, `subscription_name`, `attempt`, `latency_ms`, `correlation_id`.
- `WARN outbox.delivery.failed` — idem + `error_class`, `next_retry_at`. **Nunca** `payload`.
- `ERROR outbox.delivery.dlq` — idem + `total_attempts`.
- `WARN outbox.reaper.released` — `count`, `older_than`.
- `INFO outbox.housekeeping.purged` — `count`, `retention_days`.

`RF-24` reforçado por allowlist em `metrics.go`/`subsystem.go`: estrutura tipada de campos válidos; `payload` literalmente não aparece em chamada `slog.*Context`.

### Traces (RF-22)

- `Publisher.Publish` cria span `outbox.publish` (kind `INTERNAL`); injeta `traceparent` em `evt.Headers` antes do insert.
- `Dispatcher.deliver` (interno) extrai `traceparent` de `evt.Headers`, cria span filho `outbox.deliver` (kind `CONSUMER`).
- Handler executa dentro do span `outbox.handle.<subscription_name>` (kind `INTERNAL`).

### Dashboards e Runbook (RF-25)

- `docs/observability/outbox-dashboard.json` — Grafana JSON com 6 painéis: pending por subscription, p95/p99 latency por subscription, processed rate, DLQ count, idade do mais antigo pendente, atividade do reaper/housekeeping.
- `docs/runbooks/outbox.md` — cobre: desligar/religar Dispatcher via flag; inspecionar DLQ; re-enfileirar delivery (D-G1, SQL manual); purgar por demanda LGPD; diagnosticar pending crescente.

### Alertas sugeridos

| Alerta | Expressão | Severidade |
|---|---|---|
| DLQ crescendo | `increase(outbox_deliveries_dlq_total[5m]) > 0` | warning |
| Fila travada | `outbox_deliveries_pending > 10 * avg_over_time(rate(outbox_deliveries_processed_total[10m])[5m:])` | critical |
| Latência p95 | `histogram_quantile(0.95, sum by (subscription_name, le) (rate(outbox_delivery_latency_ms_bucket[15m]))) > 1000` | warning |
| Housekeeping parado | `increase(outbox_housekeeping_deleted_total[48h]) == 0` | critical |
| Reaper hiperativo | `increase(outbox_reaper_released_total[10m]) > 50` | warning (worker crash recorrente) |

## Considerações Técnicas

### Decisões Chave (ADRs)

| ADR | Decisão | Trade-off principal |
|---|---|---|
| `adr-016-outbox-publisher-opt-in.md` (PRD foundation) | Outbox transacional como Publisher opt-in coexistindo com `events.Bus` (ADR-003) | +1 escrita transacional vs. garantia at-least-once com observabilidade granular |
| `.specs/prd-outbox-event-driven/adr-001-schema-two-table.md` | Schema two-table (`outbox_events` + `outbox_deliveries`) vs. single-table | +1 join no claim vs. granularidade de DLQ e retry por handler |
| `.specs/prd-outbox-event-driven/adr-002-coordination-skip-locked.md` | Coordenação multi-instância via `FOR UPDATE SKIP LOCKED` vs. advisory lock / leader election | Latência de claim pequena vs. simplicidade operacional (sem coordenador externo) |
| `.specs/prd-outbox-event-driven/adr-003-vo-state-machine.md` | Modelagem com VOs + State Pattern para `DeliveryStatus` vs. enum string puro | Conforme R-DDD-001 (invariantes protegidas) com custo de boilerplate; rejeitada struct anêmica |
| `.specs/prd-outbox-event-driven/adr-004-backoff-policy-vo.md` | `BackoffPolicy` como VO com `rand.Rand` injetável vs. função pura | Testabilidade determinística e R1 da go-implementation vs. simplicidade |
| `.specs/prd-outbox-event-driven/adr-005-mockery-yml-creation.md` | Criar `mockery.yml` na raiz como parte desta entrega | Cumprir R3 obrigatória vs. fora-de-escopo aparente |

### Aplicação Explícita de Object Calisthenics

| Regra OC | Aplicação nesta techspec | Onde |
|---|---|---|
| #1 — uma camada de indentação | Loop do Dispatcher usa early-return e helpers privados (`s.deliver`, `s.markResult`) para evitar aninhamento `for { if { if { } } }` | `dispatcher.go` |
| #2 — sem `else` | Resultado do handler tratado com switch + early return; verificação de erro com guard clauses | `dispatcher.go`, `publisher.go` |
| #3 — encapsular primitivos de domínio | VOs `SubscriptionName`, `Attempt`, `DeliveryStatus`, `BackoffPolicy`. Strings cruas não circulam | `subscription.go`, `attempt.go`, `delivery_status.go`, `backoff_policy.go` |
| #4 — coleções de primeira classe | `Headers` é tipo dedicado (`type Headers map[string]string`) com método `WithTrace`/`Get`/`Validate`, não `map[string]string` solto | `headers.go` |
| #5 — um ponto por linha | Construtores extraem cadeias longas via variáveis locais semânticas | aplicado em todo `_test.go` |
| #6 — nomes não opacos | `InstanceID`, `ClaimID`, `SubscriptionName` — sem abreviações | todo o pacote |
| #7 — entidades pequenas | `Dispatcher` tem ≤ 4 colaboradores; `Publisher` tem 2 (`Storage`, `Registry`) | `dispatcher.go`, `publisher.go` |
| #8 — poucas variáveis de instância (sinal) | `Subsystem` tem 5 campos (config, dispatcher, cron, logger, instanceID) — aceitável para aggregator; sinalizado em revisão | `subsystem.go` |
| #9 — sem getters/setters mecânicos | `Event` expõe apenas getters de leitura derivados de invariante; sem setters | `event.go` |

Conflito conhecido: regra #3 sugere encapsular `aggregate_id` em VO. Decisão local: manter `string` porque a forma do `aggregate_id` é definida por cada módulo (identity, finance, etc.) e não pelo Outbox — o Outbox apenas armazena. Documentado no godoc do `Event`.

### Riscos Conhecidos

| Risco | Impacto | Mitigação |
|---|---|---|
| Handler não-idempotente duplica side-effect em retry | Cobranças, notificações duplicadas chegando ao usuário final | PR template (RF-40) com checklist; godoc explicando regra; teste de doubleness no handler dummy via re-claim simulado |
| `+1 INSERT` por handler degrada p95 do publish | Degradação de APIs transacionais | Benchmark obrigatório pré-go-live; monitorar `pg_stat_statements` por 14d; índices adequados |
| Housekeeping silenciosamente falho infla tabela | Polling degradado, custo de storage | Alerta `outbox.housekeeping.deleted.total == 0 por 48h` (critical) |
| Polling agressivo com N réplicas pressiona PG CPU | Latência geral do DB sobe | Começar com 1–3 réplicas; tick=500ms (≈2 qps em fila vazia por réplica); ajustar via flag se necessário |
| Retenção 90d incompatível com requisito regulatório futuro | Compliance | D-01 assume aprovado pelo escopo; runbook documenta purge manual para LGPD |
| Conflito conceitual `events.Bus` × `outbox.Publisher` | Dev usa caminho errado | ADR-016 + AGENTS.md com critério explícito; PR template alerta |
| `BackoffPolicy.rng` compartilhado entre goroutines do Dispatcher | Race em `rand.Rand` (não thread-safe) | Cada Dispatcher cria seu próprio `rand.Rand` ou usa `rand.New` por delivery; documentado no godoc |
| `cron.Stop(ctx)` de `robfig/cron/v3` retorna `<-chan struct{}` (espera jobs in-flight) | Shutdown lento se housekeeping estiver rodando | Usar `select { case <-cron.Stop(ctx).Done(): ; case <-ctx.Done(): }` no `Subsystem.Stop` |

### Conformidade com Padrões

Regras aplicáveis (todas em `.claude/rules/` + `.agents/skills/`):
- **R-GOV-001** — precedência: governança transversal > security > arquitetura > demais references. Aplicada.
- **R-DDD-001** — invariantes protegidas; VOs imutáveis; State Pattern; sem struct anêmica. Aplicada em `event.go`, `delivery_status.go`, `attempt.go`, `backoff_policy.go`, `subscription.go`.
- **R-ERR-001** — sentinels exportados; wrapping `fmt.Errorf %w`; sem `panic`; sem stack para usuário. Aplicada em `errors.go` + todos os retornos.
- **R-SEC-001** — sem segredos em payload; sem payload em log; logs sob `RedactingSlogHandler`. Aplicada.
- **R-TEST-001** — comportamento de domínio coberto; testes determinísticos; sem `time.Sleep` para sync. Aplicada.
- **R3** (go-implementation) — `mockery.yml` na raiz criado por esta entrega.
- **R4** (go-implementation) — `_test.go` segue `testify/suite` + table-driven.
- **R0/R1** (go-implementation) — sem `init()`; toda função em produção é método de struct.
- **R6.6** (go-implementation) — zero estado global; DI via construtor.

### Arquivos Relevantes e Dependentes

**Criados** (todos os caminhos relativos à raiz do repo):

```
internal/infrastructure/outbox/
├── AGENTS.md
├── doc.go
├── event.go
├── event_test.go
├── headers.go
├── headers_test.go
├── subscription.go
├── subscription_test.go
├── delivery_status.go
├── delivery_status_test.go
├── attempt.go
├── attempt_test.go
├── backoff_policy.go
├── backoff_policy_test.go
├── errors.go
├── handler.go
├── publisher.go
├── publisher_test.go
├── registry.go
├── registry_test.go
├── storage.go
├── storage_pgx.go
├── storage_pgx_integration_test.go     (// +build integration)
├── dispatcher.go
├── dispatcher_test.go
├── cron.go
├── subsystem.go
├── subsystem_integration_test.go        (// +build integration)
├── concurrency_integration_test.go      (// +build integration)
├── metrics.go
├── instance_id.go
├── config.go
├── claim.go
├── stats.go
├── dummy_handler.go
├── fakes/
│   └── handler.go
└── mocks/
    ├── storage.go
    └── registry.go
migrations/0002_outbox.up.sql
migrations/0002_outbox.down.sql
mockery.yml                              (raiz)
.github/PULL_REQUEST_TEMPLATE.md
docs/runbooks/outbox.md
docs/observability/outbox-dashboard.json
docs/benchmarks/outbox-baseline.md
.specs/prd-mecontrola-foundation/adr-016-outbox-publisher-opt-in.md
.specs/prd-outbox-event-driven/adr-001-schema-two-table.md
.specs/prd-outbox-event-driven/adr-002-coordination-skip-locked.md
.specs/prd-outbox-event-driven/adr-003-vo-state-machine.md
.specs/prd-outbox-event-driven/adr-004-backoff-policy-vo.md
.specs/prd-outbox-event-driven/adr-005-mockery-yml-creation.md
internal/infrastructure/runtime/outbox_subsystem.go
internal/infrastructure/runtime/outbox_subsystem_test.go
```

**Modificados**:

```
configs/config.go                                  (+ OutboxConfig + envKeys + SetDefault + Validate)
configs/config_test.go                             (+ casos de validação OutboxConfig)
internal/infrastructure/runtime/bootstrap.go       (buildSubsystems(ModeWorker) registra outbox)
Taskfile.yml                                       (+ task mocks: mockery --config mockery.yml)
go.mod / go.sum                                    (+ robfig/cron/v3@v3.0.1 + golang.org/x/sync se ainda não direto)
AGENTS.md                                          (+ seção "Outbox vs events.Bus")
CLAUDE.md                                          (referência cruzada à mesma seção)
.specs/prd-outbox-event-driven/prd.md              (sem alteração — versão consumida v4, hash registrado no topo)
```

## Plano de Rollout (RF-27 / RF-29 / R-12)

### Deploy 1 — código + migration com flag OFF

1. Merge do PR com toda a entrega; `OUTBOX_DISPATCHER_ENABLED=false` no env de produção.
2. `cmd/migrate up` aplica `0002_outbox.up.sql` (idempotente).
3. Smoke test no worker: bootstrap inicia, registry valida, Subsystem reporta `outbox.dispatcher.disabled` no log; Publisher continua escrevendo (sem Dispatcher consumindo).
4. Verificar com `psql`: `SELECT COUNT(*) FROM outbox_deliveries WHERE status='pending'` cresce conforme caller usa.
5. Tempo total estimado: 2h.

### Deploy 2 — ativação após smoke staging

1. Em staging, virar `OUTBOX_DISPATCHER_ENABLED=true`, restart do worker.
2. Disparar 100 eventos via handler dummy (`cmd/migrate seed-outbox-dummy`) e validar processed em < 5s.
3. Em produção (horário de baixa carga), idem; observar dashboard por 1h.
4. Critérios de "ok":
   - `outbox.delivery.latency_ms` p95 < 1s, p99 < 2s.
   - `outbox.deliveries.dlq.total` == 0.
   - `pg_stat_statements` mostra CPU/IO < 15% acima de baseline.
5. Tempo total estimado: 1h (after smoke).

### Critério de Rollback

- **Operacional** (`OUTBOX_DISPATCHER_ENABLED=false` + restart): se p95 do publish > 20% acima de baseline, DLQ disparar inesperadamente, ou métricas pressão anômalas no DB. RTO: < 2min (RF-29).
- **Código** (`git revert` da merge commit): se bug funcional do Dispatcher pós-flag ativo. Manter migration aplicada.
- **Estrutural** (`0002_outbox.down.sql`): apenas se schema apresentar problema confirmado e dados pendentes puderem ser drenados/exportados.

---

## Apêndice — Mapeamento Requisito → Decisão → Teste

| Requisito | Componente / Decisão | Teste |
|---|---|---|
| RF-01 (Publish recebe `database.DBTX`) | `Publisher.Publish(ctx, tx database.DBTX, evt Event)` | `publisher_test.go` — "deve preservar a tx do caller" |
| RF-02 (rejeita publish sem handler) | `ErrHandlerNotRegistered` | `publisher_test.go` — cenário sem subscription |
| RF-03 (reusa `events.EventID`/`EventName`) | `Event.id events.EventID`, `Event.eventType events.EventName` | `event_test.go` — construção válida |
| RF-04 (idempotência via UNIQUE) | Constraint `uq_outbox_deliveries_event_subscription` | `storage_pgx_integration_test.go` — duplo insert rejeitado |
| RF-05 (coexistência com `events.Bus`) | Sem alteração em `events/`; ADR-016 documenta critério | revisão de ADR-016 |
| RF-06/RF-07/RF-08 (Registry, validação duplicidade, 1×N) | `Registry.Register`, `Validate`, `ErrDuplicateSubscription` | `registry_test.go` |
| RF-09/RF-39 (Subsystem único `runtime.Subsystem`) | `outbox.Subsystem` + `lazyOutboxSubsystem` em `runtime/` | `outbox_subsystem_test.go`, `subsystem_integration_test.go` |
| RF-10 (claim com SKIP LOCKED + batch) | Query `ClaimReady` parametrizada com batch e `instance_id` | `concurrency_integration_test.go` |
| RF-11 (timeout configurável de handler) | `context.WithTimeout(ctx, cfg.HandlerTimeout)` | `dispatcher_test.go` — timeout como transiente |
| RF-12 (backoff + DLQ após 15 tentativas) | `BackoffPolicy.NextRetryAt`, `Attempt.IsExhausted(15)` | `backoff_policy_test.go`, `dispatcher_test.go` |
| RF-13 (sentinels exportados) | `errors.go` | unit cobre `errors.Is` |
| RF-14 (multi-instância sem double-processing) | `FOR UPDATE SKIP LOCKED` | `concurrency_integration_test.go` (3 dispatchers, 1000 deliveries) |
| RF-15 (campos de delivery) | Schema `outbox_deliveries` | `storage_pgx_integration_test.go` |
| RF-16/RF-17 (DLQ observável + runbook) | `outbox.deliveries.dlq.total`, runbook | inspeção manual |
| RF-18/RF-19/RF-20 (housekeeping/reaper + métrica) | `Cron`, `PurgeOlderThan`, `ReleaseStuck` | integration test do reaper + housekeeping |
| RF-21/RF-22/RF-23/RF-24 (métricas/traces/logs sem payload) | `metrics.go`, redaction via `RedactingSlogHandler` | revisão de código + dashboard |
| RF-25 (dashboard + runbook) | `docs/observability/outbox-dashboard.json`, `docs/runbooks/outbox.md` | inspeção |
| RF-26 (config flat SCREAMING_SNAKE + defaults D-03) | `OutboxConfig` em `configs/config.go` | `configs/config_test.go` |
| RF-27/RF-29 (rollout 2 deploys + < 2min rollback) | Documento de rollout acima | smoke staging + observação |
| RF-28 (migration up/down idempotente) | `migrations/0002_outbox.up.sql`/`down.sql` | `cmd/migrate up && down` em CI |
| RF-30/RF-31/RF-32 (segurança / LGPD) | Política em ADR + runbook | revisão de PR template |
| RF-33/RF-34/RF-35/RF-36 (testes + benchmark) | Suite com `testify/suite`, integration, concorrência, benchmarks | CI |
| RF-37 (ADR-016) | `.specs/prd-mecontrola-foundation/adr-016-outbox-publisher-opt-in.md` | revisão |
| RF-38 (AGENTS.md raiz + módulo + CLAUDE.md) | Edições no `AGENTS.md`/`CLAUDE.md` + `internal/infrastructure/outbox/AGENTS.md` | revisão |
| RF-40 (PR template `.github/PULL_REQUEST_TEMPLATE.md`) | Arquivo criado | revisão de PR |
| D-03 (defaults) | `configLoader.SetDefault` | `configs/config_test.go` |
| D-04 (pin cron@v3.0.1) | `go.mod` | `go list -m github.com/robfig/cron/v3` |
| D-05 (Viper boot + restart) | `OUTBOX_DISPATCHER_ENABLED` lido uma vez no `Bootstrap` | inspeção |
| D-07 (schema public) | Migration sem `CREATE SCHEMA` | inspeção SQL |
| D-08 (caller fornece EventID) | `NewEvent` exige `ID` no input | `event_test.go` |
| D-09 (unicidade `(Name, EventType)`) | `Registry.Register` retorna `ErrDuplicateSubscription` | `registry_test.go` |
| D-10 (`partition_key` NULLável reservada) | Coluna sem índice no MVP | inspeção schema |
| D-11 (`claimed_by = hostname-pid`) | `instance_id.go` constrói `fmt.Sprintf("%s-%d", host, pid)` | unit test |
| D-12 (ADR-016 em PRD foundation) | Caminho do arquivo | inspeção |
| D-13 (BackoffPolicy com rng injetável) | `NewBackoffPolicy(base, cap, rng)` | `backoff_policy_test.go` |
| D-14 (Storage no mesmo pacote) | Layout descrito em "Arquivos Relevantes" | inspeção |
| D-15 (Subsystem com errgroup + WaitGroup + errors.Join) | `subsystem.go` | `subsystem_integration_test.go` (Stop com handler in-flight) |
| D-16 (mockery.yml criado) | `mockery.yml` na raiz | `task mocks --dry-run` em CI |
| D-17 (Reaper com SKIP LOCKED) | Query `ReleaseStuck` parametrizada | `storage_pgx_integration_test.go` |

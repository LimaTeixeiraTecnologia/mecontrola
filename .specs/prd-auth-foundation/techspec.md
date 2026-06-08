<!-- spec-hash-prd: 98ea00ae8ca6f9f82e92cd0bd459fd85952a5d7c1ca346e376ddc3c251b0066b -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Fundação de Autenticação e Autorização

<!-- techspec-version: 2 — sincronizada com PRD v4. Mudanças desde v1:
     - UUID v7 (`uuid.NewV7()`) para `auth_events.id`; lifecycle do `auth_events_repository` usa v7.
     - `reason` enum em `auth_events` inclui `invalid_payload` (RF-33).
     - `MarkUserDeleted` publica `user.deleted` (RF-34); consumer anonimiza `user_id`.
     - `signature.Compose(...)` wrapper monta `raw_body_buffer ∘ hmac_validator` (ordem fixa).
     - Stub do agent reusa `WhatsAppGateway` de onboarding e envia template "MeControla recebeu sua mensagem" (RF-35).
     - Migration `0015_seed_smoke_user_staging` adicionada para suportar `task auth:smoke` (RF-36).
     - Convenção de Event.Type: `domain.eventName` (`auth.principal_established`, `user.deleted`).
     - Dedup duplicate = só métrica `whatsapp_dispatcher_route_total{outcome=duplicate}`, sem outbox event.
     - Test framework: ler `migrations/migrations_integration_test.go` para detectar dockertest vs testcontainers; tarefa pre-build prevista.
     - Shutdown order: HTTP → Dispatcher webhook → Limiter → Outbox consumers → Housekeeping → PG.
     - Linter rollout: auditar handlers atuais + expandir allowlist com headers legítimos.
     - Config canonical: tarefa pre-build identifica o pacote real (provavelmente `configs/`). -->

## Resumo Executivo

Esta techspec implementa o PRD `prd-auth-foundation` v3 (32 RFs, 15 travas inegociáveis) entregando: (1) o pacote `internal/identity/application/auth` com `Principal` minimal em `context.Context`; (2) o middleware HTTP `RequireUser` em `internal/identity/infrastructure/http/server/middleware`; (3) o usecase transacional `EstablishPrincipal` que une `FindUserByWhatsApp` + publicação outbox em uma Unit of Work curta via `uow.UnitOfWork[T].Do`; (4) o pacote compartilhado `internal/platform/whatsapp` com `signature/`, `payload/`, `dedup/`, `dispatcher/`, `ratelimit/` extraídos do onboarding por Strangler Fig atômico; (5) a tabela `auth_events` (migration `0014`) com schema mínimo + coluna `reason`; (6) o consumer `auth_events_consumer` que projeta eventos do outbox e anonimiza `user_id` ao receber `user.deleted`; (7) o `auth_events_housekeeping_job` mensal em lotes de 10k linhas; (8) regras `depguard`/`forbidigo` em `.golangci.yml`; (9) ADR-001 (contrato Principal + boundary HTTP futura) e ADR-002 (Strangler Fig onboarding); (10) load test k6 + `task auth:smoke` como gates de release.

A abordagem honra R0–R7 da skill `go-implementation`: nenhum `init()`, nenhum `panic` em produção, `context.Context` em fronteiras de IO, `errors.Join`/`%w` para erros, goroutine de cleanup do rate-limit cancelável via `Start(ctx)/Shutdown(ctx)` registrados no `module.go`. Zero dependência externa nova no `go.mod`. Sem KMS. Postgres único storage.

## Arquitetura do Sistema

### Visão Geral dos Componentes

**Novos componentes:**

| Componente | Path | Responsabilidade |
|---|---|---|
| `auth.Principal` | `internal/identity/application/auth/principal.go` | Struct concreta `{UserID uuid.UUID, Source PrincipalSource}`; chave de contexto privada; helpers `WithPrincipal`/`FromContext` |
| `RequireUser` | `internal/identity/infrastructure/http/server/middleware/require_user.go` | Middleware HTTP — 401 imediato sem corpo descritivo quando Principal ausente em `ctx` |
| `EstablishPrincipal` | `internal/identity/application/usecases/establish_principal.go` | Usecase transacional: `FindUserByWhatsApp` + `outbox.Publish` em UoW única |
| `auth_events` repository | `internal/identity/infrastructure/repositories/postgres/auth_events_repository.go` | `Insert(ctx, ev)`, `AnonymizeByUserID(ctx, userID)`, `DeleteOlderThan(ctx, cutoff, batchSize)` |
| `AuthEventsConsumer` | `internal/identity/infrastructure/messaging/database/consumers/auth_events_consumer.go` | Projeta `auth.principal_established`/`auth.failed`/`auth.unknown_user`/`user.deleted` |
| `AuthEventsHousekeepingJob` | `internal/identity/infrastructure/jobs/handlers/auth_events_housekeeping_job.go` | Apaga linhas com `occurred_at < now() - 180 days` em lotes de 10k |
| `whatsapp.signature.HMAC` | `internal/platform/whatsapp/signature/hmac.go` | Migrado de onboarding; suporte `current+next` |
| `whatsapp.signature.RawBodyBuffer` | `internal/platform/whatsapp/signature/raw_body_buffer.go` | Migrado de onboarding |
| `whatsapp.payload.Parser` | `internal/platform/whatsapp/payload/parser.go` | Migrado de onboarding (tipos + `ExtractFirstMessage`) |
| `whatsapp.dedup.MessageRepository` | `internal/platform/whatsapp/dedup/repository.go` | Porta `InsertIfAbsent(ctx, wamid) (bool, error)`; adapter Postgres reusa `meta_processed_messages` |
| `whatsapp.dispatcher.Dispatcher` | `internal/platform/whatsapp/dispatcher/dispatcher.go` | `Route(ctx, msg) (RouteOutcome, error)` — roteia entre onboarding handler e agent handler |
| `whatsapp.ratelimit.Limiter` | `internal/platform/whatsapp/ratelimit/limiter.go` | Token bucket por `user_id` com `sync.Map` + cleanup TTL + `Start/Shutdown` |
| Migration `0014` | `migrations/0014_create_identity_auth_events.{up,down}.sql` | Cria `auth_events` |
| ADR-001 | `.specs/prd-auth-foundation/adr-001-principal-contract-and-future-http-boundary.md` | Contrato e evolução |
| ADR-002 | `.specs/prd-auth-foundation/adr-002-strangler-fig-onboarding-whatsapp.md` | Migração atômica |
| `.golangci.yml` | (raiz) | Regras `depguard` + `forbidigo` |
| Load test | `scripts/load-test/auth-webhook.k6.js` | k6 — 500 msg/min × 10 min |
| Smoke task | `Taskfile.yml` recipe `auth:smoke` | Webhook real + assert linha em `auth_events` |
| Runbooks | `docs/runbooks/auth-meta-secret-rotation.md`, `docs/runbooks/auth-incident-response.md` | Operação |

**Componentes modificados:**

| Componente | Path | Mudança |
|---|---|---|
| `internal/identity/module.go` | wiring | Registrar `EstablishPrincipal`, `AuthEventsConsumer`, `AuthEventsHousekeepingJob`, `Limiter` (Start/Shutdown via lifecycle hook do `cmd/api/main.go`) |
| `internal/onboarding/infrastructure/http/server/middleware/meta_signature.go` | **deletado** no PR atômico (RF-28) | Migrado para `internal/platform/whatsapp/signature/hmac.go` |
| `internal/onboarding/infrastructure/http/server/middleware/raw_body_buffer.go` | **deletado** no PR atômico | Migrado |
| `internal/onboarding/infrastructure/http/server/handlers/meta_models.go` | **deletado** no PR atômico | Migrado para `internal/platform/whatsapp/payload/` |
| `internal/onboarding/.../whatsapp_inbound_handler.go` | reescrito | Consome novo dispatcher; mantém comportamento observável |
| `internal/onboarding/.../router.go` | reescrito | Aponta para handlers reorganizados |
| `internal/identity/usecases/mark_user_deleted.go` | revisado | Confirmar que publica `user.deleted` no outbox; ajustar payload se necessário para incluir `user_id` |
| `.specs/prd-onboarding-magic-token/prd.md` | spec-version bump (RF-31) | Documenta migração + referência ADR-002 |
| `cmd/api/main.go` | wiring | `Limiter.Start(ctx)` antes de servir HTTP; `Limiter.Shutdown(ctx)` no `SIGTERM` |

**Relacionamentos chave:**

```
Meta WhatsApp Cloud API
        ↓ POST /api/v1/whatsapp/inbound (X-Hub-Signature-256)
[middleware chain]
  RawBodyBuffer → HMACMiddleware (signature) → Router
        ↓ raw body validado
[handler]
  WhatsAppInboundHandler
        ↓ payload.Parser.ExtractFirstMessage
        ↓ dedup.MessageRepository.InsertIfAbsent(wamid)
        ↓ valueobjects.NewWhatsAppNumber
        ↓
[regex ATIVAR?]
  ├─ sim → onboarding ConsumeMagicToken (fluxo existente, sem Principal)
  └─ não → EstablishPrincipal(ctx, wa) ──┐
                                         ↓
                                    UoW.Do:
                                      FindUserByWhatsApp
                                      outbox.Publish("auth.principal_established"|"auth.unknown_user")
                                      Commit
                                         ↓
                                    Principal? ──┐
                                                 ├─ não (unknown) → onboarding fallback
                                                 └─ sim → ratelimit.Allow(userID) ──┐
                                                                                    ├─ blocked → outbox "auth.failed{rate_limited}", 200 OK
                                                                                    └─ allowed → agent.HandleMessage(WithPrincipal(ctx, p), msg)
```

Outbox dispatcher (existente) → `AuthEventsConsumer.Handle(envelope)` → Postgres `auth_events`.
`AuthEventsHousekeepingJob` (worker) → `auth_events_repository.DeleteOlderThan` em loop com `LIMIT 10000`.

## Design de Implementação

### Interfaces Chave

**`internal/identity/application/auth/principal.go`** (~60 linhas):

```go
package auth

import (
	"context"
	"github.com/google/uuid"
)

type PrincipalSource string

const SourceWhatsApp PrincipalSource = "whatsapp"
// Reservados (ADR-001): SourceJWT, SourceSystem — declarar quando boundary correspondente entrar.

type Principal struct {
	UserID uuid.UUID
	Source PrincipalSource
}

func (p Principal) IsZero() bool { return p.UserID == uuid.Nil }

type principalCtxKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalCtxKey{}, p)
}

func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalCtxKey{}).(Principal)
	if !ok || p.IsZero() {
		return Principal{}, false
	}
	return p, true
}
```

**`internal/identity/infrastructure/http/server/middleware/require_user.go`** (~30 linhas):

```go
package middleware

import (
	"net/http"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

const unauthorizedBody = `{"message":"unauthorized"}`

func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := auth.FromContext(r.Context()); !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(unauthorizedBody))
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

**`internal/identity/application/usecases/establish_principal.go`** (interface, ~80 linhas):

```go
type EstablishPrincipal struct {
	uow       uow.UnitOfWork[establishResult]
	factory   interfaces.RepositoryFactory
	publisher outbox.Publisher
	o11y      observability.Observability
}

type establishResult struct {
	Principal auth.Principal
	Found     bool
}

func (u *EstablishPrincipal) Execute(ctx context.Context, in input.EstablishPrincipalInput) (auth.Principal, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "identity.usecase.establish_principal")
	defer span.End()

	wa, err := valueobjects.NewWhatsAppNumber(in.WhatsAppNumber)
	if err != nil {
		return auth.Principal{}, fmt.Errorf("identity.usecase.establish_principal: parse: %w", err)
	}

	res, err := u.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (establishResult, error) {
		userRepo := u.factory.UserRepository(tx)
		user, found, lookupErr := userRepo.TryFindActiveByWhatsApp(ctx, wa)
		if lookupErr != nil {
			return establishResult{}, fmt.Errorf("lookup: %w", lookupErr)
		}

		if !found {
			ev := buildAuthEvent(uuid.New(), nil, "unknown_user", "whatsapp", nil)
			if pubErr := u.publisher.Publish(ctx, ev); pubErr != nil {
				return establishResult{}, fmt.Errorf("publish unknown_user: %w", pubErr)
			}
			return establishResult{Found: false}, nil
		}

		uid := user.ID()
		ev := buildAuthEvent(uuid.New(), &uid, "principal_established", "whatsapp", nil)
		if pubErr := u.publisher.Publish(ctx, ev); pubErr != nil {
			return establishResult{}, fmt.Errorf("publish established: %w", pubErr)
		}
		return establishResult{Principal: auth.Principal{UserID: uid, Source: auth.SourceWhatsApp}, Found: true}, nil
	})
	if err != nil {
		span.RecordError(err)
		return auth.Principal{}, fmt.Errorf("identity.usecase.establish_principal: %w", err)
	}
	if !res.Found {
		return auth.Principal{}, application.ErrUnknownUser
	}
	return res.Principal, nil
}
```

Notas:
- Reutiliza `uow.UnitOfWork[T].Do` (padrão de `UpsertUserByWhatsApp`).
- Exige novo método `TryFindActiveByWhatsApp(ctx, wa) (entities.User, bool, error)` no `UserRepository` para evitar usar `errors.Is(err, ErrNotFound)` no fluxo quente (mais limpo, mais barato).
- `buildAuthEvent` produz `outbox.Event` com `ID=uuid.New()`, `Type="auth.principal_established"` etc., `AggregateType="auth_event"`, `AggregateID=ev.ID`, `Payload=JSON{user_id, source, reason, occurred_at}`, `OccurredAt=time.Now().UTC()`.

**`internal/platform/whatsapp/ratelimit/limiter.go`** (interface, ~120 linhas, sem deps externas):

```go
type Limiter struct {
	buckets        sync.Map // map[uuid.UUID]*bucket
	capacity       int
	refillPerSec   int
	inactivityTTL  time.Duration
	cleanupPeriod  time.Duration
	o11y           observability.Observability
	bucketsGauge   observability.Gauge
	cleanupHist    observability.Histogram

	shutdownCh chan struct{}
	doneCh     chan struct{}
}

const (
	DefaultBucketCapacity   = 60
	DefaultRefillPerSecond  = 1
	DefaultInactivityTTL    = 5 * time.Minute
	DefaultCleanupPeriod    = 60 * time.Second
	DefaultShutdownTimeout  = 5 * time.Second
)

type bucket struct {
	tokens     atomic.Int64
	lastRefill atomic.Int64 // unix nano
	lastSeen   atomic.Int64 // unix nano
}

func New(o11y observability.Observability) *Limiter { ... }

func (l *Limiter) Start(ctx context.Context) error { /* go l.cleanupLoop(ctx) */ }

func (l *Limiter) Shutdown(ctx context.Context) error {
	close(l.shutdownCh)
	select {
	case <-l.doneCh:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("whatsapp.ratelimit: shutdown timeout: %w", ctx.Err())
	}
}

func (l *Limiter) Allow(userID uuid.UUID) bool {
	v, _ := l.buckets.LoadOrStore(userID, newBucket(l.capacity))
	b := v.(*bucket)
	now := time.Now().UnixNano()
	b.lastSeen.Store(now)
	l.refill(b, now)
	return l.tryConsume(b)
}
```

Notas:
- `sync.Map.LoadOrStore` evita race em criação de bucket (RF-32).
- Tokens em `atomic.Int64` permitem `Allow` lock-free; CAS loop em `tryConsume`.
- `cleanupLoop` itera `buckets.Range` a cada `cleanupPeriod`, remove buckets com `now - lastSeen > inactivityTTL`. Cancelável por `ctx`/`shutdownCh`.
- Sem alocação por mensagem em hot path (bucket reusado).

**`internal/platform/whatsapp/dispatcher/dispatcher.go`**:

```go
type Dispatcher struct {
	dedup            DedupRepository
	parser           PayloadParser
	establish        EstablishPrincipalUseCase
	limiter          *ratelimit.Limiter
	publisher        outbox.Publisher
	onboardingRoute  func(ctx context.Context, msg payload.Message) RouteOutcome
	agentRoute       func(ctx context.Context, msg payload.Message) RouteOutcome
	o11y             observability.Observability
}

type RouteOutcome string

const (
	OutcomeOnboarding  RouteOutcome = "onboarding"
	OutcomeAgent       RouteOutcome = "agent"
	OutcomeFallback    RouteOutcome = "fallback"
	OutcomeRateLimited RouteOutcome = "rate_limited"
	OutcomeDuplicate   RouteOutcome = "duplicate"
	OutcomeInvalid     RouteOutcome = "invalid"
)

func (d *Dispatcher) Route(ctx context.Context, raw json.RawMessage) (RouteOutcome, error)
```

Notas:
- Tipos de função para `onboardingRoute`/`agentRoute` permitem injeção em testes e desacoplamento do agent (que ainda não existe — wire stub no-op até PRD do agent).
- Falha do dispatcher é não-fatal para o webhook Meta: responde 200 OK e emite métrica de outcome.

### Modelos de Dados

**Migration `0014_create_identity_auth_events.up.sql`:**

```sql
CREATE TABLE auth_events (
    id          UUID        NOT NULL, -- UUID v7 (time-ordered) gerado em Go via uuid.NewV7()
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_id     UUID        NULL,
    kind        TEXT        NOT NULL,
    source      TEXT        NOT NULL,
    reason      TEXT        NULL,

    CONSTRAINT auth_events_pkey PRIMARY KEY (id),
    CONSTRAINT auth_events_kind_check
        CHECK (kind IN ('principal_established','failed','unknown_user')),
    CONSTRAINT auth_events_source_check
        CHECK (source = 'whatsapp'),
    CONSTRAINT auth_events_reason_check
        CHECK (
            (kind = 'failed' AND reason IN ('invalid_signature','unknown_wa_id','invalid_country','invalid_payload','rate_limited','db_unavailable'))
            OR (kind <> 'failed' AND reason IS NULL)
        )
);

CREATE INDEX auth_events_user_id_occurred_at_idx
    ON auth_events (user_id, occurred_at DESC)
    WHERE user_id IS NOT NULL;

CREATE INDEX auth_events_failed_occurred_at_idx
    ON auth_events (occurred_at DESC, reason)
    WHERE kind = 'failed';
```

**Migration `0014_create_identity_auth_events.down.sql`:**

```sql
ALTER TABLE auth_events RENAME TO auth_events_archived_20260608;
```

(Timestamp do bloco arquivado segue convenção do ADR-007 de identity.)

**Geração de UUID v7 (RF-33-bis):** todo `auth_events.id` MUST ser gerado em Go via `uuid.NewV7()` (`github.com/google/uuid v1.6.0` já presente em `go.mod`). UUID v4 fica restrito a colunas pré-existentes em outras tabelas. Justificativa: insert sequencial reduz fragmentação de B-tree em escalas de 210k linhas/mês, especialmente relevante para `auth_events_user_id_occurred_at_idx`.

**Migration `0015_seed_smoke_user_staging.up.sql` (RF-36):**

```sql
-- Seed condicional: só executa em databases cujo nome contém 'staging'.
DO $$
DECLARE
    smoke_wa TEXT := current_setting('app.smoke_wa', true);
    smoke_id UUID := '00000000-0000-0000-0000-00005a17c8e7'::uuid; -- "smoke" em leetspeak
BEGIN
    IF current_database() !~ 'staging' THEN
        RAISE NOTICE 'skipped seed: not staging database';
        RETURN;
    END IF;
    IF smoke_wa IS NULL OR smoke_wa = '' THEN
        RAISE EXCEPTION 'STAGING_SMOKE_WA não configurado (use ALTER DATABASE ... SET app.smoke_wa = ''<E164>'')';
    END IF;
    INSERT INTO users (id, whatsapp_number, status, created_at, updated_at)
    VALUES (smoke_id, smoke_wa, 'ACTIVE', now(), now())
    ON CONFLICT (id) DO NOTHING;
END $$;
```

`down`:

```sql
DELETE FROM users WHERE id = '00000000-0000-0000-0000-00005a17c8e7'::uuid;
```

O UUID `00000000-0000-0000-0000-00005a17c8e7` é constante exportada em `scripts/smoke/auth_webhook/main.go` e usada para inserir/anonimizar/limpar entre execuções do smoke. `STAGING_SMOKE_WA` é configurado por `ALTER DATABASE staging SET app.smoke_wa = '+5511…'` em provisionamento de ambiente.

**Convenção `MarkUserDeleted` publica `user.deleted` (RF-34):**

```json
{
  "event_id": "<uuid v7>",
  "user_id": "<uuid>",
  "deleted_at": "<rfc3339>"
}
```

`outbox.Event`: `Type="user.deleted"`, `AggregateType="user"`, `AggregateID=user_id` (não event_id — preserva pattern de agregado externo). A publicação ocorre dentro do mesmo UoW do `UPDATE users SET deleted_at=now(), status='DELETED'`. **Tarefa pre-build PRE-04:** confirmar se `MarkUserDeleted` atual já publica esse evento; se não, adicionar no PR do consumer.

**Dedup silencioso (A3-bis):** WAMID duplicado emite apenas métrica `whatsapp_dispatcher_route_total{outcome="duplicate"}` e log INFO `dispatcher.duplicate_wamid`. **Não** publica evento outbox nem grava linha em `auth_events`. Justificativa: duplicate é retentativa do Meta, não decisão de identidade — não polui audit log.

**Stub do agent (RF-35):** dispatcher chama interface

```go
type AgentHandler interface {
    HandleMessage(ctx context.Context, msg payload.Message) error
}
```

implementada por `internal/agent/stub.go`:

```go
type StubAgent struct {
    waGateway whatsAppGateway
    templates map[string]string
    o11y      observability.Observability
}

func (s *StubAgent) HandleMessage(ctx context.Context, msg payload.Message) error {
    p, _ := auth.FromContext(ctx)
    tmpl := s.templates["agent_stub_received"] // "MeControla recebeu sua mensagem — estamos preparando sua experiência."
    s.o11y.Logger().Info(ctx, "agent_stub_invoked",
        observability.String("user_id", p.UserID.String()),
        observability.String("wa_id_masked", payload.MaskMobile(msg.From)),
    )
    return s.waGateway.SendText(ctx, msg.From, tmpl)
}
```

`internal/agent/` pacote criado com apenas este arquivo + interface no MVP. Quando PRD do agent for criado, `StubAgent` é deletado no mesmo PR.

**Wrapper `signature.Compose()` (B2-bis):**

```go
// Compose monta a chain canônica de middlewares de webhook Meta.
// Ordem fixa e validada: RawBodyBuffer → HMACMiddleware.
// Use sempre esta função; nunca instancie os middlewares manualmente.
func Compose(secretCurrent, secretNext string, metrics MetricsRecorder) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return RawBodyBuffer(HMACMiddleware(secretCurrent, secretNext, metrics)(next))
    }
}
```

Router do webhook chama `signature.Compose(cfg.MetaSecretCurrent, cfg.MetaSecretNext, metrics)` direto. Documentação no godoc reforça que `RawBodyBuffer` e `HMACMiddleware` não devem ser usados isoladamente fora de testes.

**Shutdown order canônico (C1-bis) em `cmd/api/main.go`:**

```go
// 1. Para de aceitar novas requisições HTTP (graceful shutdown server)
// 2. Drena requisições em voo (drain timeout 10s)
// 3. Limiter.Shutdown(ctx) — cleanup goroutine para
// 4. Outbox consumers drenam eventos in-flight (já lidos do PG, ainda processando)
// 5. Housekeeping jobs abortam loop atual
// 6. PG close (último; tudo já parou de consumi-lo)
```

Implementado via `errgroup` ou `sync.WaitGroup` com ordem explícita; não usar `context.WithCancel` global que cancele tudo em paralelo (perde garantias de drain).

**Convenção de `outbox.Event.Type`** (aplicável a todos os eventos novos): formato `<domain>.<eventName>` em lowercase, dot-separated. Exemplos no MVP: `auth.principal_established`, `auth.failed`, `auth.unknown_user`, `user.deleted`. `reason` (no caso de `auth.failed`) viaja no payload JSON, **não** no `Type` (mantém cardinalidade de `Type` baixa e estável).

**Payload outbox para `auth.principal_established` (JSON):**

```json
{
  "event_id": "<uuid>",
  "user_id": "<uuid|null>",
  "kind": "principal_established|failed|unknown_user",
  "source": "whatsapp",
  "reason": "invalid_signature|unknown_wa_id|invalid_country|rate_limited|db_unavailable|null",
  "occurred_at": "<rfc3339>"
}
```

`outbox.Event`:
- `ID = event_id` (uuid)
- `Type = "auth.principal_established" | "auth.failed" | "auth.unknown_user"`
- `AggregateType = "auth_event"`
- `AggregateID = event_id` (não há agregado externo — auth event é seu próprio agregado para fins de outbox)
- `Payload = JSON acima`
- `OccurredAt = time.Now().UTC()`

### Endpoints de API

| Método + Path | Descrição |
|---|---|
| `POST /api/v1/whatsapp/inbound` | Webhook Meta. Middleware: `RawBodyBuffer` → `HMACMiddleware` → handler. Status: 401 (assinatura inválida), 200 (qualquer outro caso decidido), 503 (PG/outbox indisponível — Meta gera retry). |
| `GET /api/v1/whatsapp/verify` | Handshake Meta. Lê `hub.mode`, `hub.verify_token`, `hub.challenge`; retorna `hub.challenge` se token bate. 403 caso contrário. |

Endpoints de domínio (`/api/v1/cards`, `/api/v1/categories`, `/api/v1/budgets`) usarão `RequireUser` quando seus PRDs forem implementados — fora do escopo desta techspec, governado pelos PRDs respectivos (RF-21).

## Pontos de Integração

### Meta WhatsApp Cloud API (inbound)

- Autenticação: HMAC SHA-256 sobre o **raw body** com `META_APP_SECRET`; header `X-Hub-Signature-256: sha256=<hex>`.
- Rotação: env vars `META_APP_SECRET` + `META_APP_SECRET_NEXT` lidos via `internal/platform/config`. Runbook `docs/runbooks/auth-meta-secret-rotation.md`.
- Tratamento de erros: assinatura inválida → 401 + `auth.failed{invalid_signature}`. Mensagem inválida (país, formato) → 200 + `auth.failed{invalid_country}` (evita retry storm). PG/outbox indisponível → 503 (Meta retenta).
- Sem chamadas outbound de auth — quem responde ao usuário é o handler de onboarding ou agent (fora do escopo da auth).

### Outbox (`internal/platform/outbox`)

- `Publisher.Publish(ctx, evt)` valida `ID`, `Type`, `AggregateType`, `AggregateID`, `Payload`, `OccurredAt`. Falha de `Publish` dentro da UoW força rollback.
- Consumer registrado via `module.go` (padrão `consumers.SubscriptionEventProjector`).
- Idempotência por `event_id` — `Insert` em `auth_events` usa `ON CONFLICT (id) DO NOTHING`.

### Identity `UserRepository`

- Novo método `TryFindActiveByWhatsApp(ctx, wa WhatsAppNumber) (entities.User, bool, error)`. Implementação Postgres faz `SELECT ... WHERE whatsapp_number = $1 AND deleted_at IS NULL LIMIT 1` retornando `(user, true, nil)` quando hit, `(zero, false, nil)` quando miss, `(zero, false, err)` em erro real. Evita uso de sentinel error no caminho quente.

## Abordagem de Testes

### Testes Unitários

**Pacote `auth`:**
- `Principal.IsZero()` cobre nil UUID.
- `WithPrincipal`/`FromContext` round-trip: insere e recupera valor.
- `FromContext` em ctx vazio retorna `(zero, false)`.
- Microbenchmark: `BenchmarkWithPrincipal` e `BenchmarkFromContext` — alvo < 50 ns/op.

**Middleware `RequireUser`:**
- Tabela-driven com: (a) ctx sem Principal → 401 + body genérico; (b) ctx com Principal → next chamado, sem alteração no response.
- Microbenchmark `BenchmarkRequireUser_NoPrincipal` e `BenchmarkRequireUser_WithPrincipal` — alvo < 1 µs/op overhead.

**`EstablishPrincipal` usecase:**
- Mock `uow.UnitOfWork`, `factory.UserRepository`, `outbox.Publisher` (via mockery).
- Cenários: (a) usuário ativo → Principal retornado + `principal_established` publicado; (b) usuário inexistente → `ErrUnknownUser` + `unknown_user` publicado; (c) PG falha em `TryFindActiveByWhatsApp` → erro propagado com `%w`; (d) outbox falha → rollback do UoW (publisher mock retorna erro; assert que retorna erro e nenhum evento foi commitado).

**`Limiter`:**
- Tabela-driven com: (a) primeiro Allow para user novo → true; (b) excede capacidade → false; (c) após refill simulado (manipulando relógio interno) → true; (d) bucket inativo > TTL → removido por cleanup.
- Race detector obrigatório (`-race`). Teste de concorrência: 1000 goroutines × 100 Allow paralelos em N users → soma de Allows = soma esperada (sem perda nem duplicação por race).
- Microbenchmark `BenchmarkLimiter_Allow` com pool de 5000 buckets — alvo < 200 ns/op.
- Test `TestLimiter_Shutdown` confirma que `Shutdown(ctx)` retorna < timeout e a goroutine de cleanup encerra.

**`Dispatcher`:**
- Mock de dedup, parser, establish, limiter, publisher, onboardingRoute, agentRoute (funções).
- Cenários: ATIVAR → onboarding; mensagem normal + Principal → agent; Principal não estabelecido → fallback; rate-limit excedido → ratelimited + outbox `auth.failed{rate_limited}`; WAMID duplicado → outcome `duplicate` + nenhuma chamada subsequente.

**HMAC signature (após migração para `internal/platform/whatsapp/signature`):**
- Suite de testes table-driven já existente em `meta_signature_test.go` é movida com o código. Inclui: valid current, valid next (rotated), invalid, missing header, prefix errado.
- Property-based test `quick.Config{MaxCount: 1000}`: random body + random secret → assinatura calculada externamente bate.

**Linter `.golangci.yml`:**
- Smoke test em CI: introduzir arquivo `internal/identity/infrastructure/http/server/handlers/_lint_smoke_test/handler.go` que tenta ler `X-User-ID` (não build tag, file `.go.bad` referenciado em script). Job de CI roda golangci-lint sobre cópia, espera failure. Apaga o arquivo após.

### Testes de Integração

> Critério atendido: 2+ "sim" → integration tests recomendados.
> - [x] Fronteiras de IO críticas: Postgres + outbox.
> - [x] Histórico do projeto mostra preferência por integração com PG real (`migrations_integration_test.go`).
> - [x] Custo aceitável (PG já provisionado em `dockertest`/CI existente).

Adotar testcontainers via `_integration_test.go` com build tag `//go:build integration`.

**`auth_events_repository_integration_test.go`:**
- `Insert` → linha persistida com colunas corretas.
- `Insert` duplicado por `id` → `ON CONFLICT DO NOTHING` mantém 1 linha.
- `AnonymizeByUserID(uid)` → todas as linhas daquele user viram `user_id = NULL`; outras intactas; segunda chamada é no-op.
- `DeleteOlderThan(cutoff, batchSize=10000)` → linhas com `occurred_at < cutoff` apagadas; batchSize respeitado; idempotente.
- CHECK constraints disparam em `Insert` com `kind`/`source`/`reason` inválidos.

**`establish_principal_integration_test.go`:**
- UoW real + Postgres real + outbox real:
  - Usuário ativo: Execute → Principal retornado + linha em `platform_outbox_events` com `type='auth.principal_established'` + após dispatcher do outbox, linha em `auth_events`.
  - Usuário deletado/inexistente: Execute → `ErrUnknownUser` + linha em outbox com `type='auth.unknown_user'`.
  - Outbox simulado em erro (config inválida) → rollback observável: nenhum lock residual, `auth_events` vazia.

**`auth_events_consumer_integration_test.go`:**
- Consumir `auth.principal_established` → linha em `auth_events`.
- Consumir mesmo `event_id` duas vezes → 1 linha (idempotência).
- Consumir `user.deleted` para `user_id=X` → todas linhas de X viram `user_id = NULL`.

**`auth_events_housekeeping_job_integration_test.go`:**
- Insere 25k linhas com timestamps variados; roda job; confirma que linhas > 180d foram apagadas em 3 lotes de ≤10k; idempotente em segunda execução.

**`whatsapp_dispatcher_integration_test.go`:**
- POST com HMAC válido + payload Meta válido + usuário ativo → linha em `auth_events` + roteamento para agent stub.
- POST com HMAC inválido → 401 + linha `auth.failed{invalid_signature}` em `auth_events`.
- POST com WAMID duplicado → 200 + métrica de duplicação + sem evento adicional.

### Testes E2E (Load + Smoke)

**`scripts/load-test/auth-webhook.k6.js`:**

```js
import http from 'k6/http';
import crypto from 'k6/crypto';
import { check } from 'k6';

export const options = {
  scenarios: {
    sustained: {
      executor: 'constant-arrival-rate',
      rate: 500, timeUnit: '1m', duration: '10m',
      preAllocatedVUs: 50, maxVUs: 200,
    },
  },
  thresholds: {
    http_req_duration: ['p(99)<300'],
    http_req_failed:   ['rate<0.001'],
  },
};

const secret = __ENV.META_APP_SECRET;

export default function () {
  const body = JSON.stringify(samplePayload());
  const sig  = 'sha256=' + crypto.hmac('sha256', secret, body, 'hex');
  const res  = http.post(__ENV.WEBHOOK_URL, body, {
    headers: { 'Content-Type': 'application/json', 'X-Hub-Signature-256': sig },
  });
  check(res, { 'status 200': r => r.status === 200 });
}
```

Acceptance: 500 msg/min × 10 min, p99 < 300 ms, error rate < 0.1%. Relatório anexado ao último PR do épico (RF-29).

**`task auth:smoke` em `Taskfile.yml`:**

```yaml
auth:smoke:
  desc: "Smoke test do webhook auth em staging"
  cmds:
    - go run ./scripts/smoke/auth_webhook --url $WEBHOOK_URL --secret $META_APP_SECRET --user-wa $SMOKE_WA
    - psql $DB_URL -c "SELECT count(*) FROM auth_events WHERE occurred_at > now() - interval '10 seconds';" | grep -E '^\s+[1-9]'
```

CI invoca no merge para `main`; pipeline de deploy invoca após cada deploy staging; falha aborta deploy em prod (RF-30).

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Fundação Principal** (sem deps): `auth.Principal` + helpers + `RequireUser` + microbench + ADR-001.
2. **Migration + repo + entidade `AuthEvent`**: migration `0014` + repository + tipos. (Bloqueia consumer e usecase.)
3. **`EstablishPrincipal` usecase** + `TryFindActiveByWhatsApp` no UserRepository + `auth.unknown_user`/`auth.principal_established` payloads. (Depende do passo 2.)
4. **`AuthEventsConsumer`** + projeção + handler de `user.deleted`. (Depende do passo 2.)
5. **`AuthEventsHousekeepingJob`**. (Independente após passo 2.)
6. **Strangler Fig atômico (PR único, RF-28)**: cria `internal/platform/whatsapp/{signature,payload,dedup}`, migra `internal/onboarding/...` para consumir, **deleta** arquivos antigos, atualiza `prd-onboarding-magic-token` (RF-31), publica ADR-002. (Bloqueia 7-8.)
7. **`whatsapp.ratelimit.Limiter`** + `Start/Shutdown` + race tests. (Independente após R0–R7 alinhados.)
8. **`whatsapp.dispatcher.Dispatcher`** + integra `EstablishPrincipal` + `Limiter` + roteamento. (Depende de 3, 6, 7.)
9. **Wiring** em `internal/identity/module.go` e `cmd/api/main.go` (lifecycle hook). (Depende de 7-8.)
10. **Linter `.golangci.yml`** (depguard + forbidigo). (Independente; entra cedo para validar handlers nascentes.)
11. **Métricas + spans** instrumentados em cada componente. (Cross-cutting; entra junto com cada componente.)
12. **Runbooks + Dashboard Grafana**. (Após sistema estável em staging.)
13. **Load test k6 + smoke task**. (Gate final do épico.)

### Dependências Técnicas

- Postgres rodando (já existe em dev/staging).
- `internal/platform/outbox` em estado atual (já estável).
- `internal/identity` operacional.
- `internal/onboarding` operacional (será refatorado em-place pelo passo 6, sem janela de regressão).
- `golangci-lint` configurado (já em CI).
- `k6` instalado em CI staging.

### Tarefas pre-build obrigatórias (antes da Ordem de Build)

Antes de qualquer task de implementação codificar, as 3 tarefas de descoberta abaixo MUST rodar e seus resultados serem documentados na techspec ou em comentários do PR:

- **PRE-01: Detectar framework de integration tests.** `cat migrations/migrations_integration_test.go | head -30` para confirmar se o projeto usa `dockertest` ou `testcontainers-go`. Documentar e seguir o mesmo padrão em todos os `_integration_test.go` de auth.
- **PRE-02: Auditar headers atualmente lidos por handlers.** `grep -rE 'r\.Header\.(Get|Values)' internal/*/infrastructure/http/server/handlers/` para descobrir headers legítimos não inclusos na allowlist do `depguard`. Expandir allowlist com justificativa por header. Documentar no PR.
- **PRE-03: Identificar pacote canônico de config.** `ls internal/platform/config configs/` + `grep -rl "package config\|package configs" --include="*.go" .` para confirmar qual pacote é o canônico (provavelmente `configs/`). A regra `forbidigo` aponta para o canônico real, não para um valor presumido.

## Monitoramento e Observabilidade

### Métricas (OTel, Prometheus-compatible)

| Nome | Tipo | Labels |
|---|---|---|
| `auth_principal_established_total` | counter | `source` |
| `auth_failed_total` | counter | `reason ∈ {invalid_signature, unknown_wa_id, invalid_country, rate_limited, db_unavailable}` |
| `auth_unknown_wa_id_total` | counter | — |
| `auth_rate_limit_hits_total` | counter | — |
| `auth_resolve_wa_duration_seconds` | histograma | buckets [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0] |
| `whatsapp_dispatcher_route_total` | counter | `outcome ∈ {onboarding, agent, fallback, rate_limited, duplicate, invalid}` |
| `whatsapp_ratelimit_buckets_count` | gauge | — |
| `whatsapp_ratelimit_cleanup_duration_seconds` | histograma | — |
| `meta_signature_status_total` | counter | `status ∈ {valid, invalid, rotated}` (reusada do onboarding) |
| `auth_events_housekeeping_deleted_total` | counter | — |
| `auth_events_housekeeping_duration_seconds` | histograma | — |

**Cardinalidade controlada**: nenhuma métrica leva `user_id` ou `wa_id` como label (RF-17).

### Logs

- `slog` JSON com handler OTel-aware (do devkit) que injeta `trace_id`/`span_id` automaticamente.
- Campos típicos: `level`, `msg`, `trace_id`, `span_id`, `user_id` (somente quando Principal estabelecido), `source`, `wa_id_masked`.
- Níveis: INFO em transições normais (`principal_established`, `routed`); WARN em falhas operacionais toleráveis (`rate_limited`, `dedup_skip`); ERROR em falhas que exigem atenção (`db_unavailable`, `outbox_publish_failed`).
- Nunca logar: `wa_id` cru, `META_APP_SECRET`, `Authorization`, payload integral em INFO.

### Traces

- `auth.resolve_principal` (atributos `source`, `outcome`, `user_id` em sucesso).
- `auth.require_user` (atributo `result`).
- `whatsapp.dispatcher.route` (atributos `outcome`, `is_activation`, `is_dedup`).
- `whatsapp.ratelimit.cleanup` (atributo `removed_count`).

### Alertas (Grafana)

- `auth_failed_total{reason="invalid_signature"} > 0 in 5min` → ataque/rotação errada.
- `auth_failed_total{reason="db_unavailable"} > 1 in 1min` → saturação PG.
- `whatsapp_ratelimit_buckets_count > 10000` → leak.
- `histogram_quantile(0.99, auth_resolve_wa_duration_seconds) > 0.1 sustained 3d` → gatilho de cache.
- `outbox_publish_failed_total{kind=~"auth\\..*"} > 0` → audit perdido (crítico).

Dashboard "Auth Module" agrega todos.

## Considerações Técnicas

### Decisões Chave

**ADR-001** (`adr-001-principal-contract-and-future-http-boundary.md`):
- Decisão: `auth.Principal` é um tipo concreto Go imutável transportado por `context.Context`; nenhum dado de transporte (header, JWT cru, cookie) entra no domínio.
- Cobre: estrutura do `Principal`, regra de invariância, esboço de boundary HTTP futura (JWT Ed25519 + JWKs + refresh + claims `sub`/`sid`/`aud`/`exp`/`iat`/`kid`), constantes Go reservadas (`SourceJWT`, `SourceSystem`) que ainda não entram no código.
- Alternativas rejeitadas: interface Go (viola "tipos concretos por padrão"); JWT já no MVP (gestão de chaves em VPS sem KMS é vetor de erro humano); sessão opaca em PG (acoplamento + over-engineering para LLM in-process).

**ADR-002** (`adr-002-strangler-fig-onboarding-whatsapp.md`):
- Decisão: migração de `meta_signature.go`, `raw_body_buffer.go` e parser Meta de `internal/onboarding/...` para `internal/platform/whatsapp/{signature,payload}` em **PR único atômico** que reescreve o onboarding para consumir o novo pacote e **deleta** os arquivos antigos.
- Cobre: passos do PR, política de testes de regressão (suite existente movida), impacto em `prd-onboarding-magic-token` (spec-version bump), critérios de aceitação.
- Alternativas rejeitadas: 3 PRs sequenciais (janela de duplicação); manter ambos por 1 sprint (cripto duplicada em produção).

### Riscos Conhecidos

| Risco | Impacto | Mitigação |
|---|---|---|
| Strangler Fig regressão silenciosa em onboarding | Alto (ativação quebra em produção) | Suíte existente `meta_signature_test.go` movida intacta; integration test do webhook em pre-merge; smoke task `task onboarding:smoke` (criar se ausente) cobre fluxo de ativação. |
| Bucket creation race no `Limiter` | Médio (over-allowance momentâneo) | `sync.Map.LoadOrStore` é atômico; CAS loop em `tryConsume` evita perda de token; race detector obrigatório. |
| Cleanup goroutine fica órfã em testes | Baixo (leak de goroutine em CI) | `t.Cleanup` chama `Limiter.Shutdown(ctx)` em todo teste que instancia. |
| Outbox `Publish` falha após `Find` bem-sucedido | Médio (audit perdido se sem rollback) | UoW única envolve ambos; falha de publish dispara rollback automático; teste de integração simula. |
| Linter regra ampla bloqueia header legítimo no onboarding | Baixo (CI vermelho em PR não-relacionado) | Allowlist explícita (`X-Request-ID`, `Content-Type`, `Idempotency-Key`); revisão da regra incluída no PR Strangler Fig. |
| Migration `0014` colide com PR concorrente | Baixo (conflito de numeração) | RF-24: CI valida unicidade; resolução é renomear o conflito antes de merge. |
| LLM in-process futuro pode demandar chunked/streaming via SSE | Médio | Fora do MVP — RF-23 trava in-process; mudança exige novo PRD. |

### Conformidade com Padrões

- `.claude/rules/governance.md` (R-GOV-001) — hard rules respeitadas.
- `AGENTS.md` Padrão Obrigatório de Módulo — `internal/identity` mantém estrutura `domain/application/infrastructure`; novos pacotes seguem o mesmo layout.
- `CLAUDE.md` Outbox — `auth_events` projetado via outbox, idempotência por `event_id`.
- Skill `go-implementation` Regras Estritas R0–R7 (todas `[HARD]`):
  - R0: nenhum `init()` nos novos pacotes.
  - R5.12: nenhum `panic` em produção (apenas em testes ou bootstrap fail-fast claramente delimitado).
  - R6: `context.Context` em toda fronteira de IO; interface no consumidor (`DedupRepository` definida em `dispatcher/`, implementada por adapter externo).
  - R7.6: `errors.Join` para agregar; `fmt.Errorf("ctx: %w", err)` em todos os pontos de wrap.
- ADR-008 identity (repository-factory-per-module) reusado.
- ADR-007 identity (partial unique indexes) inspirou os índices parciais de `auth_events`.

### Arquivos Relevantes e Dependentes

**Criados:**
- `internal/agent/agent.go` (interface `AgentHandler`)
- `internal/agent/stub.go` + `_test.go` (RF-35 — deletado quando PRD `internal/agent` real entrar)
- `migrations/0015_seed_smoke_user_staging.up.sql` + `.down.sql` (RF-36)
- `internal/identity/application/auth/principal.go` + `_test.go`
- `internal/identity/application/usecases/establish_principal.go` + `_test.go`
- `internal/identity/application/usecases/establish_principal_integration_test.go`
- `internal/identity/application/dtos/input/establish_principal.go`
- `internal/identity/application/errors.go` (adicionar `ErrUnknownUser` se ainda não existir)
- `internal/identity/infrastructure/http/server/middleware/require_user.go` + `_test.go`
- `internal/identity/infrastructure/repositories/postgres/auth_events_repository.go` + `_test.go` + `_integration_test.go`
- `internal/identity/infrastructure/messaging/database/consumers/auth_events_consumer.go` + `_test.go` + `_integration_test.go`
- `internal/identity/infrastructure/jobs/handlers/auth_events_housekeeping_job.go` + `_test.go` + `_integration_test.go`
- `internal/platform/whatsapp/signature/hmac.go` + `_test.go` (movido)
- `internal/platform/whatsapp/signature/raw_body_buffer.go` + `_test.go` (movido)
- `internal/platform/whatsapp/payload/parser.go` + `_test.go` (movido)
- `internal/platform/whatsapp/dedup/repository.go` (porta)
- `internal/platform/whatsapp/dedup/postgres/repository.go` (adapter)
- `internal/platform/whatsapp/dispatcher/dispatcher.go` + `_test.go` + `_integration_test.go`
- `internal/platform/whatsapp/ratelimit/limiter.go` + `_test.go` (race + bench)
- `migrations/0014_create_identity_auth_events.up.sql` + `.down.sql`
- `.golangci.yml` (criar ou alterar regras `depguard` + `forbidigo`)
- `scripts/load-test/auth-webhook.k6.js`
- `scripts/smoke/auth_webhook/main.go`
- `docs/runbooks/auth-meta-secret-rotation.md`
- `docs/runbooks/auth-incident-response.md`
- `.specs/prd-auth-foundation/adr-001-principal-contract-and-future-http-boundary.md`
- `.specs/prd-auth-foundation/adr-002-strangler-fig-onboarding-whatsapp.md`

**Modificados:**
- `internal/identity/module.go` (registra novos componentes e EventHandlerRegistration)
- `internal/identity/application/interfaces/user_repository.go` (adiciona `TryFindActiveByWhatsApp`)
- `internal/identity/infrastructure/repositories/postgres/user_repository.go` (implementa)
- `internal/identity/application/usecases/mark_user_deleted.go` (confirma/ajusta publicação de `user.deleted`)
- `internal/onboarding/infrastructure/http/server/handlers/whatsapp_inbound_handler.go` (consome novo dispatcher)
- `internal/onboarding/infrastructure/http/server/router.go` (apontar para novos pacotes)
- `Taskfile.yml` (adiciona `auth:smoke`)
- `cmd/api/main.go` (lifecycle hooks de `Limiter` + housekeeping job + consumer)
- `.specs/prd-onboarding-magic-token/prd.md` (spec-version bump, RF-31)

**Deletados (no PR Strangler Fig):**
- `internal/onboarding/infrastructure/http/server/middleware/meta_signature.go`
- `internal/onboarding/infrastructure/http/server/middleware/meta_signature_test.go` (move para `internal/platform/whatsapp/signature/`)
- `internal/onboarding/infrastructure/http/server/middleware/raw_body_buffer.go`
- `internal/onboarding/infrastructure/http/server/handlers/meta_models.go`

<!-- spec-hash-prd: eb1398f20d26c6ec2753d4c31c5ebada6a0eca1922972b3e23b1195da50d841b -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Módulo `internal/card` (MVP CRUD + InvoiceFor)

## Resumo Executivo

Introduz o bounded context `internal/card` aderente ao Padrão Obrigatório de Módulo (`AGENTS.md`) e isomórfico a `internal/identity`/`internal/billing`. Entrega: (1) CRUD HTTP sob `/api/v1/cards` consumindo o `RequireUser` canônico de `internal/identity/.../middleware`; (2) função pura `InvoiceFor(purchase, cycle, tz) Invoice` em `domain/services/billing_cycle.go`, reutilizada por um endpoint público (`GET /cards/{id}/invoices`) e por uma porta interna `CardLookup` para o futuro módulo de transações; (3) pacote genérico `internal/platform/idempotency/` com `Storage`, `PostgresStorage` (pgx puro sobre `database.DBTX`) e middleware chi de replay; (4) middleware adicional `InjectPrincipalFromHeader` em `internal/identity/.../middleware` que materializa `auth.Principal` a partir de `X-User-ID` enquanto JWT/OIDC não existe (substitui o `RequireUser` transitório do PRD original, é additive e preserva o contrato canônico).

Persistência em pgx puro, schema canônico `mecontrola.` (drift D-01 frente ao PRD), migrations 6 dígitos `000004_` e `000005_` com `down` por rename (drift D-02). Observabilidade via `github.com/JailtonJunior94/devkit-go/pkg/observability` (drift D-03: `internal/platform/observability` citado pelo PRD não existe). Tempo SP carregado uma única vez via `sync.Once` em variável de pacote; falha de load encerra o processo no startup, nunca via `panic` em runtime (R0/R5.12). Zero comentários em código Go (`R-ADAPTER-001.1` HARD). Adapters finos (`R-ADAPTER-001.2` HARD): handlers e producers nunca executam SQL, branching de domínio ou regra de negócio.

**Atomicidade idempotência (ADR-006)**: gravação 2xx no `idempotency_keys` participa da MESMA `uow.UnitOfWork` da escrita de negócio (exactly-once real). Middleware vira pre-check/replay/conflict; use case grava `Storage.Put` dentro do UoW. Caching de respostas 4xx é best-effort feito pelo middleware fora do UoW (retentativa de validation error é determinística — perda do cache não compromete correção). 5xx nunca cacheado.

Toda implementação Go derivada DEVE carregar `.agents/skills/go-implementation/SKILL.md`, carregar exemplos sob demanda, verificar `go.mod` (Go 1.26.4) antes de usar recursos da linguagem, e partir de `cmd/server/server.go` — nunca de `internal/platform/runtime`.

## Arquitetura do Sistema

### Visão Geral dos Componentes

Novos pacotes/arquivos (todos novos salvo notas explícitas):

**Plataforma — idempotência genérica**

- `internal/platform/idempotency/storage.go` — interface `Storage` (`Get`, `Put`), tipo `Record`, sentinels `ErrNotFound`, `ErrHashMismatch`.
- `internal/platform/idempotency/postgres_storage.go` — `PostgresStorage` baseada em `database.DBTX`, queries `INSERT … ON CONFLICT DO NOTHING RETURNING` e `SELECT`. Mapeia `pgerrcode.UniqueViolation` para `ErrHashMismatch` quando hash diverge na linha existente.
- `internal/platform/idempotency/context.go` — tipo `IdempotencyContext{Scope, Key, UserID, RequestHash, ExpiresAt}` + helpers `WithContext(ctx, ic) context.Context` e `FromContext(ctx) (IdempotencyContext, bool)`. É a ponte material entre middleware (pré-check) e use case (Put dentro do UoW). Ver ADR-006.
- `internal/platform/idempotency/middleware.go` — `Middleware(scope string, storage Storage, ttl time.Duration, o11y observability.Observability, opts ...Option) func(http.Handler) http.Handler`. Responsabilidades: (a) ler header `Idempotency-Key` (1–128 ASCII); (b) calcular `request_hash` via SHA-256 do body; (c) `Storage.Get` → hit+match: replay; hit+mismatch: 409; miss: injeta `IdempotencyContext` no ctx e chama `next`; (d) após `next` retornar, se status ∈ [400,499] e o use case NÃO gravou (verifica via `Storage.Get` rápido), middleware grava best-effort em tx separada com response body capturado por `responseRecorder`. 2xx NUNCA é gravado pelo middleware (responsabilidade do use case dentro do UoW). 5xx descartado.
- `internal/platform/idempotency/recorder.go` — `responseRecorder` interno com buffer limitado a 64 KB (`ErrResponseTooLarge` se exceder; promove 500 ao cliente — ver §Riscos R7).
- `internal/platform/idempotency/mocks/` — gerado por mockery a partir de `mockery.yml`.

**Identity — middleware adicional**

- `internal/identity/infrastructure/http/server/middleware/inject_principal_from_header.go` — extrai `X-User-ID` (UUID v4), constrói `auth.Principal{UserID, Source: SourceHeader}` e injeta via `auth.WithPrincipal`. Modificação additive em `internal/identity/application/auth/principal.go`: novo constante `SourceHeader PrincipalSource = "header"`. Modificado: `principal.go` (adiciona constante).

**Card — bounded context novo**

- `internal/card/module.go` — `NewCardModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) CardModule`. Retorno sem `error` (sem IO complexo no constructor).
- `internal/card/domain/valueobjects/`
  - `card_name.go` — VO `CardName` 1–64 chars.
  - `nickname.go` — VO `Nickname` 1–32 chars.
  - `billing_cycle.go` — VO `BillingCycle{ClosingDay, DueDay}` com `ClosingDay/DueDay ∈ [1,31]`.
- `internal/card/domain/entities/card.go` — agregado `Card` com construtores `New` (cria) e `Hydrate` (rehydrata persistência).
- `internal/card/domain/services/billing_cycle.go` — função pura `BillingCycle.InvoiceFor(purchase time.Time, cycle valueobjects.BillingCycle, tz *time.Location) Invoice` + tipo `Invoice{ClosingDate, DueDate civil.Date}` (datas em SP, sem hora).
- `internal/card/domain/services/timezone.go` — `func SaoPauloLocation() *time.Location` via `sync.Once`; helper `MustLoadSaoPauloOrExit()` chamado em `module.go`. Falha → `slog.Error` + `os.Exit(1)` (sem `init()`, sem `panic`).
- `internal/card/domain/errors.go` — sentinels `ErrCardNotFound`, `ErrNicknameConflict`, `ErrInvalidClosingDay`, `ErrInvalidDueDay`, `ErrInvalidCardName`, `ErrInvalidNickname`, `ErrInvalidPurchaseDate`.
- `internal/card/application/dtos/input/` — `CreateCard`, `UpdateCard`, `GetCard`, `ListCards`, `SoftDeleteCard`, `InvoiceFor`.
- `internal/card/application/dtos/output/` — `Card`, `CardList`, `Invoice`.
- `internal/card/application/interfaces/`
  - `repository.go` — `CardRepository` (`Insert`, `GetByIDForUser`, `ListByUser`, `UpdateByIDForUser`, `SoftDeleteByIDForUser`), `RepositoryFactory.CardRepository(database.DBTX) CardRepository`.
- `internal/card/application/usecases/`
  - `create_card.go`, `get_card.go`, `list_cards.go`, `update_card.go`, `soft_delete_card.go`, `invoice_for.go`. Cada um expõe `Execute(ctx, in) (out, error)`.
- `internal/card/application/usecases/mocks/` — gerado por mockery.
- `internal/card/infrastructure/repositories/factory.go` + `internal/card/infrastructure/repositories/postgres/card_repository.go` — pgx puro, queries inline, mapping `pgerrcode.UniqueViolation` → `ErrNicknameConflict`.
- `internal/card/infrastructure/http/server/router.go` — `CardRouter` (chi `Register(r chi.Router)`), encadeia `inject_principal_from_header.InjectPrincipalFromHeader` → `identity middleware.RequireUserWithO11y` → `idempotency.Middleware("card", storage, 24h, o11y)` (apenas em POST/PUT/DELETE).
- `internal/card/infrastructure/http/server/handlers/` — `create.go`, `list.go`, `get.go`, `update.go`, `delete.go`, `invoice_for.go`. Cada handler chama exclusivamente o use case correspondente; nenhum SQL, sem branching de domínio (`R-ADAPTER-001.2` HARD).
- `internal/card/infrastructure/http/server/testdata/` — golden files OpenAPI + responses canônicas.
- `internal/card/infrastructure/http/server/openapi.yaml` — contrato OpenAPI 3.1 publicado como artifact de CI.
- `internal/card/infrastructure/observability/redact.go` — `redactCardLogFields(card)` (helper interno consumido por handlers e use cases para padronizar atributos de log sem `name`/`nickname`).

**Migrations (`migrations/`)**

- `000004_create_platform_idempotency_keys.up.sql` + `.down.sql` (drift D-02 vs. PRD `0010`).
- `000005_create_card_cards.up.sql` + `.down.sql` (drift D-02 vs. PRD `0011`).

**Wiring (`cmd/server/server.go`)**

- Após `onboardingModule`, instanciar `cardModule := card.NewCardModule(cfg, o11y, dbManager)` e, se `cardModule.CardRouter != nil`, `srv.RegisterRouters(cardModule.CardRouter)`.

**Runbook**

- `docs/runbooks/card-rollback.md` (novo, ver §"Sequenciamento de Desenvolvimento").

### Fluxo de Dados

```
HTTP request
  └─► chi router
        └─► InjectPrincipalFromHeader  (lê X-User-ID, popula auth.Principal)
              └─► RequireUserWithO11y  (canônico identity; 401 se ctx sem Principal)
                    └─► idempotency.Middleware (POST|PUT|DELETE)
                          ├── hit + hash igual    → replay resposta armazenada
                          ├── hit + hash diverge  → 409 Conflict (envelope)
                          └── miss
                                └─► handler.<op> (thin: decode → usecase → encode)
                                      └─► usecase.<op>.Execute(ctx, in)
                                            ├── domain.services.BillingCycle (puro)
                                            └── repositoryFactory.CardRepository(mgr.DBTX(ctx))
                                                  └─► pgx → mecontrola.cards
                                └── (após resposta 2xx) → idempotency.Storage.Put
```

`InvoiceFor` (porta interna): consumidores Go importam `card.CardLookup` exposto pela struct `CardModule`. O use case `usecases.InvoiceFor` resolve `Card` por ID/usuário e delega para `domain/services.BillingCycle.InvoiceFor(purchase, cycle, SaoPauloLocation())`.

## Design de Implementação

### Interfaces Chave

```go
// internal/platform/idempotency/storage.go
type Record struct {
    Scope, Key, UserID, RequestHash string
    ResponseStatus                  int
    ResponseBody                    []byte
    ExpiresAt, CreatedAt            time.Time
}

type Storage interface {
    Get(ctx context.Context, scope, key, userID string) (Record, error)
    Put(ctx context.Context, rec Record) error
}

// internal/platform/idempotency/context.go
type IdempotencyContext struct {
    Scope, Key, UserID, RequestHash string
    ExpiresAt                       time.Time
}

func WithContext(ctx context.Context, ic IdempotencyContext) context.Context
func FromContext(ctx context.Context) (IdempotencyContext, bool)

// internal/card/domain/services/billing_cycle.go
type Invoice struct{ ClosingDate, DueDate time.Time }

type BillingCycle struct{}

func (BillingCycle) InvoiceFor(
    purchase time.Time,
    cycle valueobjects.BillingCycle,
    tz *time.Location,
) Invoice

// internal/card/application/interfaces/repository.go
type CardRepository interface {
    Insert(ctx context.Context, c entities.Card) error
    GetByIDForUser(ctx context.Context, cardID, userID string) (entities.Card, error)
    ListByUser(ctx context.Context, userID, cursor string, limit int) ([]entities.Card, string, error)
    UpdateByIDForUser(ctx context.Context, c entities.Card) (entities.Card, error)
    SoftDeleteByIDForUser(ctx context.Context, cardID, userID string, now time.Time) error
}

type RepositoryFactory interface {
    CardRepository(db database.DBTX) CardRepository
}

// internal/card/module.go (resumo)
type CardModule struct {
    RepositoryFactory interfaces.RepositoryFactory
    CardRouter        *server.CardRouter
    CardLookup        *usecases.InvoiceFor
}

func NewCardModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) CardModule
```

**Use cases mutadores (Create/Update/SoftDelete)** recebem `uow.UnitOfWork[T]` + `idempotency.Storage` e seguem o pattern:

```go
func (u *CreateCard) Execute(ctx context.Context, in input.CreateCard) (output.Card, error) {
    ic, _ := idempotency.FromContext(ctx)
    return u.uow.Execute(ctx, func(ctx context.Context) (entities.Card, error) {
        repo := u.factory.CardRepository(u.mgr.DBTX(ctx))
        card := entities.NewCard(in)
        if err := repo.Insert(ctx, card); err != nil { return entities.Card{}, err }
        if ic.Key != "" {
            body, _ := json.Marshal(toOutput(card))
            rec := idempotency.Record{
                Scope: ic.Scope, Key: ic.Key, UserID: ic.UserID,
                RequestHash: ic.RequestHash,
                ResponseStatus: http.StatusCreated, ResponseBody: body,
                ExpiresAt: ic.ExpiresAt,
            }
            if err := u.idem.Put(ctx, rec); err != nil { return entities.Card{}, err }
        }
        return card, nil
    }).Then(toOutput)
}
```

A serialização JSON dentro do use case usa o mesmo `encoding/json` que o handler chamará para escrever no `ResponseWriter`, garantindo replay byte-idêntico. Decisão e trade-offs em ADR-006.

**Cursor de paginação** (decidido nesta techspec): formato `base64.URLEncoding.EncodeToString(json.Marshal(cursorPayload{CreatedAt: t, ID: id}))`. Decodificação valida ambos os campos; cursor malformado → 400 `invalid_cursor`. Listagem usa `WHERE (created_at, id) < ($cursor_t, $cursor_id) ORDER BY created_at DESC, id DESC LIMIT $limit+1` (tupla composta para keyset estável). Próximo cursor existe se retornou `limit+1` linhas.

### Modelos de Dados

**`mecontrola.cards` (`000005_create_card_cards.up.sql`)**

```sql
CREATE TABLE IF NOT EXISTS mecontrola.cards (
    id          UUID        NOT NULL,
    user_id     UUID        NOT NULL,
    name        TEXT        NOT NULL,
    nickname    TEXT        NOT NULL,
    closing_day SMALLINT    NOT NULL,
    due_day     SMALLINT    NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ NULL,
    CONSTRAINT cards_pkey PRIMARY KEY (id),
    CONSTRAINT cards_user_fk FOREIGN KEY (user_id)
        REFERENCES mecontrola.users(id) ON DELETE RESTRICT,
    CONSTRAINT cards_closing_day_chk CHECK (closing_day BETWEEN 1 AND 31),
    CONSTRAINT cards_due_day_chk     CHECK (due_day     BETWEEN 1 AND 31),
    CONSTRAINT cards_name_len_chk     CHECK (char_length(name)     BETWEEN 1 AND 64),
    CONSTRAINT cards_nickname_len_chk CHECK (char_length(nickname) BETWEEN 1 AND 32)
);

CREATE UNIQUE INDEX IF NOT EXISTS cards_user_nickname_active_uniq_idx
    ON mecontrola.cards (user_id, nickname)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS cards_user_pagination_idx
    ON mecontrola.cards (user_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
```

`down`: `ALTER TABLE mecontrola.cards RENAME TO cards_archived_<timestamp>;` + `DROP INDEX IF EXISTS …`. Sem `DROP TABLE` (RF-18). Drift D-02: numeração e schema diferem do PRD original.

**`mecontrola.idempotency_keys` (`000004_create_platform_idempotency_keys.up.sql`)**

```sql
CREATE TABLE IF NOT EXISTS mecontrola.idempotency_keys (
    scope            TEXT        NOT NULL,
    key              TEXT        NOT NULL,
    user_id          UUID        NOT NULL,
    request_hash     TEXT        NOT NULL,
    response_status  INT         NOT NULL,
    response_body    BYTEA       NOT NULL,
    expires_at       TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT idempotency_keys_pkey PRIMARY KEY (scope, key, user_id),
    CONSTRAINT idempotency_keys_key_len_chk          CHECK (char_length(key) BETWEEN 1 AND 128),
    CONSTRAINT idempotency_keys_request_hash_len_chk CHECK (char_length(request_hash) = 64),
    CONSTRAINT idempotency_keys_status_chk           CHECK (response_status BETWEEN 200 AND 599),
    CONSTRAINT idempotency_keys_body_size_chk        CHECK (octet_length(response_body) <= 65536)
);

CREATE INDEX IF NOT EXISTS idempotency_keys_expires_idx
    ON mecontrola.idempotency_keys (expires_at);
```

`down`: rename equivalente. Sem job de limpeza no MVP (PRD S-05); índice por `expires_at` deixa o job futuro barato.

### Endpoints de API

Todos sob `/api/v1/cards`. `Content-Type: application/json`. Envelope de erro padrão `{"message": "...", "details": {...}}` via `responses.Error`/`responses.ErrorWithDetails`.

| Método | Path | Status sucesso | Headers exigidos | Notas |
|-------|------|----------------|------------------|-------|
| POST   | `/`                       | 201 + `Location` | `X-User-ID`, `Idempotency-Key` | body `{name, nickname, closing_day, due_day}` |
| GET    | `/`                       | 200             | `X-User-ID`                    | `?cursor=<base64>&limit=<1..100>` |
| GET    | `/{id}`                   | 200             | `X-User-ID`                    | 404 se inexistente/soft-deleted |
| PUT    | `/{id}`                   | 200             | `X-User-ID`, `Idempotency-Key` | sparse JSON |
| DELETE | `/{id}`                   | 204             | `X-User-ID`, `Idempotency-Key` | soft-delete |
| GET    | `/{id}/invoices?for=YYYY-MM-DD` | 200       | `X-User-ID`                    | resposta `{closing_date, due_date}` em SP |

Mapping de erros → HTTP:

- `ErrInvalidPayload` (decode) → 400 `invalid_payload`.
- `ErrInvalidCardName|Nickname|ClosingDay|DueDay|PurchaseDate` → 400 com `code` semântico.
- `ausência/invalid X-User-ID` → 401 `unauthorized` (canônico identity).
- `ausência Idempotency-Key` → 400 `missing_idempotency_key`.
- `ErrHashMismatch` → 409 `idempotency_conflict`.
- `ErrNicknameConflict` → 409 `nickname_in_use`.
- `ErrCardNotFound` → 404 `card_not_found`.
- default → 500 `internal_error`.

`responses.ErrorWithDetails` (devkit-go) emite o envelope; mensagens em pt-BR.

Algoritmo `InvoiceFor` (referência inline; detalhe em ADR-002):

```text
1. purchase = purchase.In(tz).Truncate(24h)        // dia local SP
2. cYear, cMonth := purchase.Year(), purchase.Month()
3. clamp := func(d, year, month) → min(d, daysIn(year, month))
4. close := Date(cYear, cMonth, clamp(cycle.ClosingDay, cYear, cMonth))
   if cycle.ClosingDay > cycle.DueDay → due in (cMonth+1)
   if cycle.ClosingDay < cycle.DueDay → due in (cMonth)
   if cycle.ClosingDay == cycle.DueDay → close := close - 1 day; due in (cMonth ou cMonth+1 conforme)
5. due := Date(due.Year, due.Month, clamp(cycle.DueDay, due.Year, due.Month))
6. if purchase.After(close.EndOfDay) → advance both close/due by one cycle (mês ou conforme convenção); reaplica clamp
7. return Invoice{ClosingDate: close, DueDate: due}
```

Determinístico, sem IO, sem `time.Now`. Não usa abstração de relógio (R6.7).

## Pontos de Integração

Nenhuma integração externa. Consumidores do módulo:

- `internal/transaction` (futuro): importa `cardModule.CardLookup` (porta Go).
- Front/WhatsApp/agentes IA: consomem REST `/api/v1/cards`.

Dependências internas:

- `internal/platform/idempotency` (novo pacote desta techspec).
- `internal/identity/application/auth` (`Principal`, `WithPrincipal`, `FromContext`) e `internal/identity/.../middleware` (`RequireUserWithO11y`).
- `mecontrola.users(id)` via FK física (decisão sobre S-01 em ADR-004).
- `manager.Manager` e `database.DBTX` (devkit-go).

## Abordagem de Testes

### Testes Unitários

- `domain/services/billing_cycle_test.go`
  - Suite `BillingCycleSuite` (testify/suite) com ≥ 50 fixtures table-driven (RF-44): fev/28, fev/29 (bissexto: 2024/2028), abr/jun/set/nov (30 dias), virada dez→jan, `due == closing`, `due > closing`, `due < closing`, `closing=31`, `due=31`, DST histórico BR (2018-10-21 e 2018-11-04 — Brasil tinha DST), compras em horário-padrão.
  - Property-based via `testing/quick` (RF-45) com `quick.Config{MaxCount: 10000}` validando invariantes (a–d).
- `domain/valueobjects/*_test.go` — limites de tamanho e faixa.
- `application/usecases/*_test.go` — mocks gerados via mockery; cobertura ≥ 90% caminho feliz + erros sentinels.
- `infrastructure/http/server/handlers/*_test.go` — uso de `httptest.NewRecorder` + mocks de use case; valida codes/envelopes/headers.
- `infrastructure/observability/redact_test.go` — regressão de PII (M-07): inspeciona `slog.Handler` capturado e garante ausência de `name`/`nickname`.
- `internal/platform/idempotency/middleware_test.go` — cobre hit (replay), miss (persiste), `ErrHashMismatch` (409), TTL expirado (tratado como miss), body grande (limite de buffer).

### Testes de Integração

Critérios: ✔ fronteira de IO crítica (Postgres + soft-delete + unique parcial); ✔ histórico de incidente potencial em race conditions de `Idempotency-Key`; ✔ custo de testcontainers já amortizado pelo repo (`internal/billing` integrationtest path). → adotar `//go:build integration` com `testcontainers-go` Postgres 16.

- `internal/card/infrastructure/repositories/postgres/card_repository_integration_test.go` (RF-46): insert + read; soft-delete + read (404); unicidade parcial concorrente (`ErrNicknameConflict`); listagem paginada estável (cursor opaco round-trip ≥ 250 cartões).
- `internal/platform/idempotency/postgres_storage_integration_test.go`: race condition (10 goroutines, mesma key, expectativa: 1 inserção persistida e 9 replays); TTL expirado é descartado.
- `migrations/migrations_integration_test.go` (estender): roda `up`/`down`/`up` para `000004` e `000005`, valida que `down` cria tabela `cards_archived_*` e `idempotency_keys_archived_*`.

### Testes E2E

Contract tests via golden files (RF-29/47) em `internal/card/infrastructure/http/server/testdata/`:

- `openapi_responses_test.go` aplica migration, sobe `chi` com `CardRouter`, executa requests canônicos (POST/GET/PUT/DELETE/invoices) e diffa contra goldens. Spec OpenAPI 3.1 validada por `kin-openapi` (já presente em deps transitivas devkit-go, validar; se não, fallback é YAML schema check manual).
- Cenário replay end-to-end: POST `Idempotency-Key=k1` duas vezes — segunda resposta byte-a-byte idêntica à primeira.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Migrations & schema** — `000004_create_platform_idempotency_keys.{up,down}.sql` + `000005_create_card_cards.{up,down}.sql` + estender `migrations_integration_test.go`. Sem código Go ainda.
2. **`internal/platform/idempotency/`** — interface `Storage`, `PostgresStorage`, middleware chi, mocks. Testes unitários + integração isoladamente verdes.
3. **Domain** — VOs, agregado `Card`, sentinels, `domain/services/billing_cycle.go` + `domain/services/timezone.go` com `sync.Once`. Testes table-driven + property-based ≥ 95% line coverage.
4. **Application** — DTOs, interfaces, use cases, mocks. Testes unitários.
5. **Infrastructure** — repository pgx, `redact.go`, handlers, router, OpenAPI + testdata, integração.
6. **Identity middleware additive** — `inject_principal_from_header.go` + extensão `SourceHeader` em `auth/principal.go` + testes.
7. **`module.go` + wiring** — `NewCardModule` em `internal/card`, registro em `cmd/server/server.go`, log "card module wired". Contract tests rodando contra binário.
8. **Runbook** — `docs/runbooks/card-rollback.md` documentando rename + revert do registro.
9. **CI hardening** — regra `golangci-lint` para bloquear identificadores `pan|cvv|cvc|track|pin` em `internal/card/**` (custom `forbidigo` / pré-commit hook; resolução final em ADR-005 ou techspec follow-up; PRD S-04 marcado como suposição).

### Dependências Técnicas

- Postgres 16 (já em uso).
- `mecontrola.users` (já existente em `000001_initial_baseline`).
- `manager.Manager` e `devkit-go/pkg/{database,observability,responses,http_server}` (já em `go.mod`).
- `github.com/jackc/pgx/v5`, `pgerrcode`, `google/uuid`, `testify`, `mockery v2`, `testcontainers-go` (já em `go.mod`).

Nenhuma dependência nova prevista.

## Monitoramento e Observabilidade

- **Spans OTel** (RF-33): nomes `card.handler.*`, `card.middleware.*`, `card.usecase.*`, `card.repository.pg.*`, `card.domain.invoice_for`. Cada span recebe atributos `card_id`, `user_id`, `outcome` (`success|conflict|not_found|invalid|internal_error`). Sem `name`/`nickname` em atributos.
- **Logs estruturados** via `o11y.Logger()` (`log/slog` wrapper devkit). Helper `redactCardLogFields(card)` retorna `[]observability.Attribute` apenas com `card_id`, `user_id`, `closing_day`, `due_day`. Eventos canônicos: `card.create.started|completed|failed`, `card.list.served`, `card.update.completed`, `card.delete.completed`, `card.invoice_for.computed`, `card.idempotency.replay`, `card.auth.rejected`.
- **Métricas Prometheus dedicadas**: não no MVP (RF — espelho do PRD; fora de escopo F2). Latência/erro derivados do exporter de spans + métricas HTTP do `httpserver.WithOTelMetrics()` já em produção.
- **Dashboard "Card Module" (Grafana)**: criado como parte do runbook; usa métricas existentes `http_server_*` filtradas por `route=/api/v1/cards*`.
- **Alertas (futuro / fora MVP)**: `error_rate{route=~"/api/v1/cards.*"} > 1% por 5min`, `p99_latency_seconds{route=~"/api/v1/cards.*"} > 0.3` por 10min. Documentar no runbook.

## Considerações Técnicas

### Decisões Chave

Cada decisão material possui ADR dedicada neste diretório:

- **ADR-001** — Pacote genérico `internal/platform/idempotency/` com PK `(scope, key, user_id)`. Trade-off: simplicidade vs. eventual job de cleanup; janela de TTL 24h + índice por `expires_at` reduz overhead até fase 2. Alternativa rejeitada: idempotência local por módulo (duplicação + drift de schema).
- **ADR-002** — Algoritmo `InvoiceFor`: auto-detect convenção pela relação `closing_day` vs. `due_day`, clamp por `daysInMonth`, regra `closing == due` → fechamento dia anterior. Alternativa rejeitada: forçar usuário a escolher convenção explícita (UX pior, mais campos no contrato).
- **ADR-003** — Middleware adicional `InjectPrincipalFromHeader` em `internal/identity/.../middleware`, additive ao canônico `RequireUser`. Decisão: localizar em `identity` (dono do `auth.Principal`), introduzir constante `SourceHeader`. Alternativa rejeitada: definir middleware dentro do `internal/card` (vazaria responsabilidade de auth para outro bounded context).
- **ADR-004** — Persistência `mecontrola.cards` com FK física para `mecontrola.users(id)` (`ON DELETE RESTRICT`), soft-delete por `deleted_at` e índice parcial. Resolve PRD S-01. Alternativa rejeitada: FK lógica (perde integridade referencial sem ganho operacional, pois schema é compartilhado).
- **ADR-005** — Numeração de migrations padronizada em 6 dígitos vigentes (`000004_`, `000005_`) e `down` por rename. Resolve drift D-02. Alternativa rejeitada: criar trilha paralela com numeração 4 dígitos (quebra o `migrations/embed.go` + ordering do golang-migrate).
- **ADR-006** — Idempotência atômica via `uow.UnitOfWork` do devkit: use case grava `Storage.Put` dentro do mesmo UoW da escrita de negócio para 2xx (exactly-once real). Middleware faz pre-check/replay/conflict + cache best-effort de 4xx em tx separada. Alternativa rejeitada: middleware grava sempre via `responseRecorder` (janela de inconsistência se servidor cai entre commit do INSERT e UPDATE do response_body).

**Decisões tácticas resolvidas (inline, sem ADR):**

- **Cursor de paginação**: `base64.URLEncoding(json{created_at, id})`, keyset com tupla composta. Mais simples evoluir do que pipe-separated; HMAC rejeitado (defesa em profundidade desnecessária já que repo filtra por `user_id` do principal).
- **Cache idempotency**: 2xx (atômico, via UoW) + 4xx (best-effort, pós-handler). 5xx nunca cacheado. Replay determinístico mesmo em validation error → UX consistente.
- **Limite `response_body` 64 KB**: `CHECK octet_length <= 65536` no schema + `responseRecorder` com cap. Exceder → log + 500 ao cliente (sinal de bug, payload de cartão fica sub-1KB).
- **Validação OpenAPI 3.1**: dep nova `github.com/getkin/kin-openapi` (Apache 2.0); contract tests carregam `openapi.yaml` e validam cada response canônico contra schema.
- **Guarda anti-PCI**: regra `forbidigo` em `.golangci.yml` com padrão `\b(pan|cvv|cvc|track|pin)\b` aplicada a `internal/card/...`. Custom message: "PAN/CVV/CVC/track/PIN são proibidos no escopo card — aplicação é não-PCI (PRD RF-16)".
- **Load test**: k6 (script em `loadtest/card/`, container oficial, integra com Grafana Cloud para evidência de SLO M-02/M-03/M-04).

### Riscos Conhecidos

- **R1 — DST histórico Brasil (2008–2019)**: cálculo em `America/Sao_Paulo` para datas antigas pode ressurgir caso `tzdata` divirja entre containers. **Mitigação**: testes fixados em datas com transição (2018-10-21 e 2018-11-04) + `time.LoadLocation` via `sync.Once` na imagem com `tzdata` instalado (validar no Dockerfile).
- **R2 — Race de `Idempotency-Key` concorrente**: duas requisições paralelas com mesma `(scope, key, user_id)` podem disparar dupla execução. **Mitigação**: `INSERT … ON CONFLICT DO NOTHING RETURNING` no `Storage.Put`; perdedor relê via `SELECT` e replica resposta. Teste de integração com 10 goroutines.
- **R3 — Crescimento de `idempotency_keys` sem cleanup**: PRD S-05. **Mitigação MVP**: TTL 24h + índice `expires_at`. Volume estacionário ~10k linhas. Fase 2 introduz job ou `pg_cron`.
- **R4 — Vazamento de PII em logs**: regressão se desenvolvedor logar `card` direto. **Mitigação**: helper obrigatório `redactCardLogFields` + teste de regressão M-07 + revisão (`R-ADAPTER-001` HARD veta comentários, mas não loga PII; teste é a salvaguarda real).
- **R5 — `RequireUser` canônico só pass por chamadas HTTP que tenham middleware injetor**: chamadas que escaparem do chain (rota mal-registrada) virão 401. **Mitigação**: `CardRouter.Register` sempre encadeia ambos os middlewares; teste de router cobre 401 quando `X-User-ID` ausente.
- **R6 — Mudança de fuso pelo Postgres**: cálculo é em Go, persistência em UTC. Postgres não influencia. Sem ação.
- **R7 — Resposta > 64 KB no idempotency**: `responseRecorder` retorna `ErrResponseTooLarge`. Middleware/use case promove 500 ao cliente e log `card.idempotency.body_overflow`. Endpoints CRUD do MVP têm payloads sub-1KB; estouro indica bug. CHECK no schema é segunda camada.
- **R8 — UoW.Execute do devkit rola back em qualquer erro** (incluindo erro do `Storage.Put`): garante que falha de gravação da idempotência não deixe cartão "órfão" do registro idempotente. Validar em teste de integração injetando `idempotency.Storage` mockado que retorna erro após `repo.Insert`.

### Conformidade com Padrões

- `.claude/rules/governance.md` — precedência respeitada; segurança/correção priorizadas.
- `.claude/rules/go-adapters.md`:
  - **R-ADAPTER-001.1 (HARD)** — zero comentários em `.go` produção (gate `grep` configurado no checklist de validação).
  - **R-ADAPTER-001.2 (HARD)** — handlers/consumers/jobs/producers do `card` permanecem finos: nenhum SQL direto, nenhum branching de domínio, apenas decode → usecase → encode/erro.
  - **R-ADAPTER-001.3 (HARD)** — implementação Go deve carregar apenas: `architecture.md` + `api.md` (handlers); `architecture.md` + `messaging.md` apenas se producers forem introduzidos (não há no MVP); `examples-infrastructure.md` para lifecycle do `module.go`; `observability.md` para os spans. Nunca carregar `patterns-structural.md`.
- `AGENTS.md` — Padrão Obrigatório de Módulo, R0–R7, fronteiras `infrastructure → application → domain`, `domain` puro, sem `clock.Clock`, sem `init()`, sem `panic`, sem `var _ Interface = (*Type)(nil)`.

### Drifts Registrados

- **D-01 — Schema `mecontrola.`**: PRD não cita schema; migrations baseline criam tudo em `mecontrola.`. Techspec adota `mecontrola.cards` e `mecontrola.idempotency_keys` por consistência. Risco baixo.
- **D-02 — Numeração de migrations**: PRD cita `0010_*` e `0011_*`; repositório usa 6 dígitos (`000001_`–`000003_`). Techspec adota `000004_` e `000005_`. Coberto por ADR-005.
- **D-03 — `internal/platform/observability` inexistente**: PRD F-07 e Restrições mencionam o pacote; na realidade observabilidade vem de `github.com/JailtonJunior94/devkit-go/pkg/observability`. Techspec usa o devkit (sem novo pacote local).
- **D-04 — Substituição do middleware transitório**: PRD v2 já antecipa substituição pelo `RequireUser` canônico, porém não existe middleware HTTP que injete `auth.Principal` na cadeia HTTP (`EstablishPrincipal` hoje só é consumido pelo dispatcher WhatsApp). Techspec introduz `InjectPrincipalFromHeader` em `internal/identity` (ADR-003). Não altera contrato HTTP do `card`.

### Riscos Abertos / Próximos Passos

- _(nenhum)_ — todas as suposições do PRD foram resolvidas:
  - **S-01 → ADR-004** (FK física `ON DELETE RESTRICT` para `mecontrola.users(id)`).
  - **S-02** retenção de soft-deleted: confirmado fora do MVP; fase 2.
  - **S-03 → k6** (decisão tática inline).
  - **S-04 → `golangci-lint forbidigo`** (decisão tática inline).
  - **S-05** job de cleanup de `idempotency_keys`: fora do MVP; TTL 24h + índice `expires_at` cobrem volume estacionário.
  - **S-06 → ADR-002** (`closing == due` aceito com convenção `fechamento = due − 1`).
  - **S-07** pré-condição operacional: `X-User-ID` confiável vem do gateway; exposição interna até gateway pronto (ADR-003).
- **Revisitar `ON DELETE RESTRICT` na fase 2** quando exclusão LGPD do usuário entrar — política precisa cascatear via aplicação (anonimização) ou trocar para `ON DELETE SET NULL`+`deleted_at`. Tratado fora do MVP.

### Arquivos Relevantes e Dependentes

Novos:
- `migrations/000004_create_platform_idempotency_keys.{up,down}.sql`
- `migrations/000005_create_card_cards.{up,down}.sql`
- `internal/platform/idempotency/{storage,postgres_storage,middleware,recorder}.go` + tests
- `internal/identity/infrastructure/http/server/middleware/inject_principal_from_header.go` + test
- `internal/card/**` (todo o módulo)
- `docs/runbooks/card-rollback.md`
- `loadtest/card/` (scripts k6)
- `.specs/prd-card-crud-mvp/adr-001..006-*.md`

Modificados:
- `internal/identity/application/auth/principal.go` (adiciona constante `SourceHeader`)
- `cmd/server/server.go` (registra `cardModule`)
- `migrations/migrations_integration_test.go` (cobre 000004/000005)
- `mockery.yml` (adiciona alvos do `card` e `idempotency`)
- `.golangci.yml` (regra `forbidigo` anti-PCI escopada em `internal/card/...`)
- `go.mod`/`go.sum` (adiciona `github.com/getkin/kin-openapi`)

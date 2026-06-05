<!-- spec-hash-prd: 7abc7ae26aff1d53146918cd563fa6c4b21d0a28f73cbd71e7018b20555c5e97 -->
<!-- spec-hash-techspec: bd62ade7a89222b68bf24f43c03d8e65aa5f65d40b105c0cf47b7de21020fee0 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — E1 `identity-foundation`

> **PRD consumido:** [`prd.md`](./prd.md)
> **Épico de origem:** `docs/epics/epic-01-identity-foundation.md`
> **Posição no roadmap:** raiz do MVP — bloqueia E2 (`billing-pipeline`) e E3 (`onboarding-magic-token`).
> **Próxima skill:** `create-tasks` (após aprovação desta techspec).
> **Esta techspec NÃO contém implementação de código Go.** Apenas o desenho de implementação que deve ser executado por `execute-task` carregando obrigatoriamente `.agents/skills/go-implementation/SKILL.md` (ver §15).

## Resumo Executivo

Esta especificação define como materializar o módulo `internal/identity` no working tree atual (scaffold vazio) seguindo o **Padrão Obrigatório de Módulo** declarado em `AGENTS.md` e o runbook canônico [`docs/runbooks/handler-usecase-uow-repository.md`](../../docs/runbooks/handler-usecase-uow-repository.md). A entrega cobre: o agregado `User` com identidade UUID estável gerada pelo próprio domínio (`entities.NewID()`, sem `IDGenerator` injetado); os Value Objects `WhatsAppNumber` (E.164 BR) e `Email`; o atributo opcional `display_name` com política `first-write-wins` aplicada no agregado (`User.SetDisplayNameIfEmpty`); soft delete com reanimação dentro de janela de 30 dias; histórico de números em tabela auxiliar; o port `UserRepository` em `application/interfaces` com implementação Postgres; **`RepositoryFactory` por módulo** (ADR-008) para amarrar repos a uma `database.DBTX`; **`uow.UnitOfWork[T]` da devkit-go v0.4.0 direto** (`github.com/JailtonJunior94/devkit-go/pkg/database/uow`) — proibido reimplementar localmente; helper compartilhado `internal/platform/sqlnull` para conversão zero-value→NULL; handlers HTTP via `github.com/JailtonJunior94/devkit-go/pkg/responses`; a função pura `IsEntitled(sub, now) (bool, Reason)` consumida por E2 sem cross-module; mascaramento de PII em logs por método nos VOs; índices parciais únicos para coexistir com soft delete; regras `depguard` para enforçar fronteiras hexagonais.

Decisões materiais foram extraídas para 8 ADRs (§14). O wiring expõe `IdentityModule{RepositoryFactory, UserRouter}` com `UserRouter == nil` no MVP — `cmd/server/server.go` registra o router só quando `!= nil` (item 4 do Padrão). `NewIdentityModule(cfg, o11y, mgr)` recebe `manager.Manager` (não `UoW`, não `IDGenerator`) e cria 1 `uow.New[T](mgr, uow.WithObservability(o11y))` tipado por use case. `cmd/worker/worker.go` recebe o refactor do outbox (§17) para adotar o mesmo padrão UoW + Factory. Toda observabilidade é propagada via `o11y observability.Observability` (devkit-go v0.4.0): tracer abre span por operação relevante e o UoW emite `db.{driver}.tx` automaticamente; logger estrutura sem PII (helpers `Masked()` e `pii.MaskDisplayName`). Persistência via `database.DBTX` com `ExecContext`/`QueryContext`/`QueryRowContext` diretos (sem `PrepareContext` — não existe na interface da devkit), `span.RecordError` em IO, log estruturado em erro, `sqlnull.Str`/`sqlnull.Time` em colunas anuláveis e `pgerrcode.UniqueViolation` → sentinel em `application/errors.go`.

## Arquitetura do Sistema

### Visão Geral dos Componentes

Estrutura alvo (toda nova; árvore vazia hoje em `internal/identity/`):

```text
internal/identity/
├── module.go                                    [novo] IdentityModule + NewIdentityModule(cfg, o11y, mgr) (ADR-005)
├── doc.go                                       [novo] documentação canônica do módulo (sem RBAC/JWT — F-12)
├── application/
│   ├── errors.go                                [novo] ErrUserNotFound, ErrWhatsAppNumberInUse, ErrEmailInUse (ADR-004)
│   ├── interfaces/
│   │   ├── user_repository.go                   [novo] port UserRepository (RF-10)
│   │   └── repository_factory.go                [novo] port RepositoryFactory (ADR-008)
│   ├── usecases/upsert_user_by_whatsapp.go      [novo] uow.Do + factory.UserRepository(tx) (RF-08-ter)
│   ├── usecases/find_user_by_id.go              [novo] lookup via factory.UserRepository(pool)
│   ├── usecases/find_user_by_whatsapp.go        [novo] lookup via factory.UserRepository(pool)
│   ├── usecases/mark_user_deleted.go            [novo] soft delete (RF-06) com uow.NewVoid
│   └── dtos/
│       ├── input/                               [novo] DTOs de entrada por use case + doc.go
│       └── output/                              [novo] DTOs de saída por use case + doc.go
├── domain/
│   ├── entities/id.go                           [novo] entities.NewID() = uuid.NewString() (sem DI — ADR-008)
│   ├── entities/user.go                         [novo] agregado User + SetDisplayNameIfEmpty/MarkDeleted/Reanimate/CanReanimate
│   ├── entities/whatsapp_history_entry.go       [novo] NewWhatsAppHistoryEntry (F-05) — autossuficiente em ID
│   ├── valueobjects/whatsapp_number.go          [novo] VO + Masked() (ADR-003)
│   ├── valueobjects/email.go                    [novo] VO + Masked() (ADR-003)
│   ├── entitlement.go                           [novo] IsEntitled, Reason, Subscription contract (ADR-001, ADR-002)
│   ├── policies.go                              [novo] ReanimationWindow (ADR-006)
│   ├── pii/mask.go                              [novo] MaskDisplayName (ADR-003)
│   └── services/doc.go                          [novo] placeholder vazio neste MVP
└── infrastructure/
    ├── http/
    │   ├── server/
    │   │   ├── router.go                        [novo] UserRouter placeholder (Register(chi.Router) vazio) — item 4 do Padrão
    │   │   └── handlers/
    │   │       ├── upsert_user_by_whatsapp_handler.go [novo] usa devkit-go/pkg/responses (sem writeError local)
    │   │       └── doc.go                       [novo se necessário] placeholder até E3
    │   └── client/doc.go                        [novo] placeholder vazio neste MVP
    ├── jobs/handlers/doc.go                     [novo] placeholder vazio neste MVP
    ├── messaging/database/consumers/doc.go      [novo] placeholder vazio neste MVP
    ├── messaging/database/producers/doc.go      [novo] placeholder vazio neste MVP
    └── repositories/
        ├── factory.go                           [novo] NewRepositoryFactory(o11y) (ADR-008)
        └── postgres/user_repository.go          [novo] implementação do port (ADR-007), consome internal/platform/sqlnull
```

Migrations:

```text
migrations/
├── 000002_identity_users.up.sql                 [novo] users + CHECK + 2 índices parciais + 1 índice auxiliar (ADR-007)
├── 000002_identity_users.down.sql               [novo]
├── 000003_identity_user_whatsapp_history.up.sql [novo] user_whatsapp_history + FK CASCADE + 2 índices
└── 000003_identity_user_whatsapp_history.down.sql [novo]
```

Lint:

```text
.golangci.yml                                    [editar] adicionar regras depguard e forbidigo (RF-15, ADR-003)
```

Relacionamentos:

- `application/usecases/*` consome `uow.UnitOfWork[T]` (devkit) + port `application/interfaces.RepositoryFactory` (ADR-008).
- `RepositoryFactory.UserRepository(db database.DBTX)` devolve `interfaces.UserRepository` amarrada à `db` (pool ou tx do callback).
- `infrastructure/repositories/postgres.userRepository` satisfaz o port; consome `internal/platform/sqlnull` para colunas anuláveis.
- `infrastructure/http/server/handlers/*` consome `usecases.*` e responde via `devkit-go/pkg/responses.JSON/Error/ErrorWithDetails`.
- `cmd/server/server.go` constrói `IdentityModule` via `NewIdentityModule(cfg, o11y, dbManager)` e (futuro) registra `UserRouter`.
- `cmd/worker/worker.go` recebe o refactor do outbox (§17) — sem jobs de identity no MVP.
- E2 (futuro) declara sua própria interface mínima e a satisfaz com `identityModule.RepositoryFactory.UserRepository(tx)`.
- E2 implementa `domain.Subscription` (interface) e passa para `domain.IsEntitled`.

### Leitura do estado atual

| Caminho | Estado | Observação |
|---|---|---|
| `internal/identity/module.go` | apenas `package identity` | Todo wiring é novo. |
| `internal/identity/{application,domain,infrastructure}/...` | árvore vazia (apenas pastas) | Toda implementação é nova. |
| `internal/billing/module.go` | apenas `package billing` | Padrão do `InvoiceModule` vem de `AGENTS.md`, não de exemplo real no working tree. |
| `cmd/server/server.go` | bootstrap real (cobra `server`); inicializa `o11y` (`otel.NewProvider`), `dbManager` (`manager.New(...)`), `httpserver.New(...)` com `chi_server` | **Não registra nenhum módulo de negócio hoje.** Ponto de extensão para identity ainda inexistente; a chamada `srv.RegisterRouters(...)` (item 4 do Padrão) será introduzida quando o primeiro router real chegar. |
| `cmd/worker/worker.go` | bootstrap real; monta `[]worker.Job` com outbox dispatcher/reaper/housekeeping e `worker.NewManager(cfg, jobs, nil, logger)` | Parâmetro `consumers` está `nil`. Identity não adiciona jobs/consumers no MVP. |
| `internal/platform/outbox/storage_postgres.go` | referência canônica de repository transacional | Padrão `database.FromContext(ctx)`, `BeginTx`, `errors.Join` em rollback, `defer rows.Close()` — replicado em §10. |
| `internal/platform/worker/{job,consumer}` | `Job`, `Consumer`, `Manager`, `job.NewAdapter`, `consumer.NewAdapter` | Reuso obrigatório se identity vier a precisar (não no MVP). |
| `internal/platform/id` | `id.Generator` + `UUIDGenerator` | **Preservado por compat; não importar em código novo.** Identity gera UUID dentro do domínio via `entities.NewID()` (ADR-008). |
| `internal/platform/sqlnull` | `Str(s string) any`, `Time(t time.Time) any` | Reuso obrigatório em repositórios — proibido helper local `nullableString` (R6.11). |
| `migrations/` | contém `000001_outbox_events.{up,down}.sql` + `embed.go` | Próximo número livre: `000002_`. |
| `.golangci.yml` | regras `depguard` já existem para várias fronteiras (linhas 37–158) | Adicionar regras específicas de `internal/identity/domain` e `application` (RF-15). |
| `go.mod` | `go 1.26.2`, `toolchain go1.26.4`, `devkit-go v0.4.0` | Versão de linguagem usável; nenhuma feature pós-1.26 disparada por esta spec. |
| `configs/config.go` | `*configs.Config` com `AppConfig`, `HTTPConfig`, `DBConfig`, `O11yConfig`, `OutboxConfig`, `KiwifyConfig`, `BillingConfig` | Identity **não adiciona seção própria de config neste MVP**. |

**Drift documental — status atualizado:**

- **D-01 (RESOLVIDO):** `o11y.Tracer()` validado contra devkit-go v0.4.0 (`pkg/observability/tracer.go`). API confirmada:
  - `Tracer.Start(ctx, spanName string, opts ...SpanOption) (context.Context, Span)`
  - `Span.RecordError(err error, fields ...Field)` + `Span.End()`
  - Helpers `observability.String(key, value)`, `observability.Error(err)`.
  Esta techspec adota `o11y.Tracer().Start(ctx, "<layer>.<operation>")` como shape obrigatório.
- **D-02 (RESOLVIDO):** `chi_server.Server.RegisterRouters(routers ...Router)` validado em `pkg/http_server/chi_server/server.go`, recebendo qualquer valor que implemente `Router{ Register(chi.Router) }`. O MVP de E1 entrega `UserRouter` placeholder (struct com `Register(chi.Router)` vazio) para satisfazer o item 4 do Padrão sem registrar rotas reais.
- **D-03 (RESOLVIDO):** `internal/identity/doc.go` será criado do zero — F-12 satisfeita por inexistência ativa (nenhuma menção a RBAC/JWT/sessions/`is_admin`).
- **D-04 (NOVO):** `devkit-go/pkg/database/uow` (validado no cache local v0.4.0) oferece `UnitOfWork[T any]` genérica + `New[T]`/`NewVoid` + `WithObservability`/`WithIsolation`/`WithReadOnly`. Esta spec adota como dependência direta — **proibido** reimplementar localmente em `internal/platform/uow/` (ADR-008).
- **D-05 (NOVO):** `devkit-go/pkg/responses` (validado em v0.4.0) oferece `JSON(w, status, data)`, `Error(w, status, message)`, `ErrorWithDetails(w, status, message, details)`. Esta spec adota como dependência direta para handlers HTTP — **proibido** helpers locais `writeJSON`/`writeError` por handler.

## Escopo Incluído / Fora de Escopo

### Incluído (replica e fixa o PRD)

- Agregado `User`, VOs (`WhatsAppNumber`, `Email`), `display_name` opcional com `first-write-wins` aplicado no agregado (`User.SetDisplayNameIfEmpty`).
- Geração de UUID dentro do domínio (`entities.NewID()`) — sem `IDGenerator` injetado em qualquer camada.
- Soft delete + reanimação dentro de janela; histórico de números (schema + método de repo).
- `IsEntitled` puro com `Subscription` mínima como interface (ADR-002).
- Helpers de mascaramento de PII (ADR-003).
- Migrations DDL + índices (ADR-007).
- Regras `depguard` e forbidigo no `.golangci.yml`.
- `IdentityModule{RepositoryFactory, UserRouter}` com `NewIdentityModule(cfg, o11y, mgr)` (ADR-005).
- `RepositoryFactory` por módulo + `uow.UnitOfWork[T]` da devkit direto (ADR-008).
- Handlers HTTP via `devkit-go/pkg/responses`; helpers SQL NULL via `internal/platform/sqlnull` (ver runbook §7.1, §8).
- **Refactor outbox** (sub-épico de E1 — ver §17) adotando o mesmo padrão UoW + Factory.

### Fora de escopo (replicado do PRD; **não** será desenhado nesta techspec)

- `EntitlementService` (cache Redis + invalidação) — pertence a E2.
- Handlers HTTP, jobs, consumers e producers de identity no MVP (slots ficam vazios).
- Comando administrativo "trocar número de WhatsApp" como UC/handler.
- Anonimização LGPD após 30d (E4).
- RBAC/JWT/sessions/`is_admin` — proibidos por RF-02 (ver F-12).
- Multi-país do `WhatsAppNumber`.
- Métricas/alertas/dashboards Prometheus/Grafana (E4).
- Publicação de evento `user_created` (S-04: nenhum consumidor no MVP).

## Arquitetura e Fronteiras

### Fluxo permitido (replica `AGENTS.md`)

`infrastructure → application → domain` (runtime); domain puro.

### Contratos cross-module

- **`identity.RepositoryFactory`** (port em `application/interfaces`, ADR-008): exposto como campo de `IdentityModule`. E2/E3 declaram a interface mínima que precisam e a satisfazem chamando `identityModule.RepositoryFactory.UserRepository(tx)` dentro do callback do seu próprio `uow.UnitOfWork[T]` (R6 — interface no consumidor de identity).
- **`identity.UserRepository`** (port em `application/interfaces`): devolvido pela factory; nunca instanciado direto por consumidor externo.
- **`identity/domain.Subscription`** (interface mínima — ADR-002): consumido por `IsEntitled`; E2 implementa.
- **`identity/domain.Reason`** (`type string` — ADR-001): consumido por E2 (`Decision.Reason`) e E3 (copy de bloqueio).
- **`identity/domain.ReanimationWindow`** (constante — ADR-006): consumida por E4 (job de anonimização) para coerência.

### Sem comunicação cross-module assíncrona no MVP

S-04 do PRD confirma: nenhum evento `user_created` no outbox. Quando E2/E3 precisarem, identity ganha producer (slot `infrastructure/messaging/database/producers/` está pronto), `IdentityModule` ganha campo `Producers []consumer.Producer` ou equivalente, e o wiring no worker passa a injetar — mudança aditiva sobre ADR-005.

### Regras Go obrigatórias (replica das R0–R7 aplicáveis nesta superfície)

- **R0:** sem `init()` em nenhum arquivo de identity.
- **R1:** funções de domínio/aplicação/infraestrutura são métodos de struct (`User.MarkDeleted(now)`, `WhatsAppNumber.Masked()`, etc.). Exceções permitidas: `main` (não aplicável aqui), construtores (`NewIdentityModule`, `NewWhatsAppNumber`, `NewRepositoryFactory`, `entities.New`, `entities.NewID`, …), helpers de testes. **`IsEntitled` é função, não método** — justificativa: é pura, não pertence a um receiver natural (`sub` é parâmetro e pode ser `nil`); adicionar receiver implicaria `Subscription.IsEntitled(now)` que viola o contrato "interface no consumidor de identity". Documentar exceção em `doc.go`.
- **R5.8:** `SubscriptionStatus` é `string`, não `iota` — aceito conscientemente em ADR-002 (interop JSON).
- **R5.10:** erros via `errors.New` (sentinels — ADR-004), `fmt.Errorf("ctx: %w", err)` para wrapping.
- **R5.12:** sem `panic` em produção.
- **R5.26:** globais não exportados em camelCase, sem prefixo `_` (regra revogada em 2026-06-04, conforme memória de governança).
- **R6:** `context.Context` em toda fronteira de IO (usecase, repository, futuros handler/job/consumer); DI via construtores explícitos; interface no consumidor (`UserRepository` e `RepositoryFactory` declarados em `application/interfaces`, satisfeitos em `infrastructure/repositories`).
- **R6.4:** sem `var _ Interface = (*Type)(nil)` em código de produção. Apenas em testes, se necessário (ADR-002).
- **R6.7 — reforço:** sem `clock.Clock` injetado e **proibido** capturar `now := time.Now().UTC()` em variável intermediária. Chamar `time.Now().UTC()` **inline no call-site** (parâmetro do método, campo do construtor de entidade). Aplica a UC, repo, handler.
- **R6.8 (novo — ADR-008):** **proibido injetar `IDGenerator`** (`entities.IDGenerator`, `id.Generator`, ou similar) em qualquer camada. Domínio gera ID via `entities.NewID()` chamada dentro do construtor da entidade (`entities.New`, `entities.NewWhatsAppHistoryEntry`).
- **R6.9 (novo — ADR-008):** **proibido reimplementar `UnitOfWork`** em `internal/platform/uow/` ou similar. Consumir sempre `github.com/JailtonJunior94/devkit-go/pkg/database/uow` (`uow.UnitOfWork[T]`, `New[T]`, `NewVoid`).
- **R6.10 (novo):** **proibido reimplementar helpers de resposta HTTP** (`writeJSON`/`writeError`) em handlers. Consumir sempre `github.com/JailtonJunior94/devkit-go/pkg/responses` (`JSON`, `Error`, `ErrorWithDetails`). Códigos semânticos viajam em `details` (`map[string]string{"code": "..."}`).
- **R6.11 (novo):** **proibido reimplementar helpers de conversão zero-value → SQL NULL** (`nullableString`, `nullableTime`) em repositórios. Consumir sempre `internal/platform/sqlnull` (`sqlnull.Str`, `sqlnull.Time`).
- **R7.1:** `any`, não `interface{}`.
- **R7.2:** logging estruturado via `o11y.Logger()` (devkit-go encapsula `log/slog`).
- **R7.6:** `errors.Join` para agregar erros (uso pontual; o UoW da devkit já faz o trabalho transacional).

## Design por Superfície

### Domínio (`internal/identity/domain/...`)

**`valueobjects/whatsapp_number.go`**

```go
package valueobjects

import (
    "errors"
    "fmt"
    "regexp"
    "strings"
)

type WhatsAppNumber struct {
    e164 string // imutável; sempre "+55DDD9NNNNNNNN"
}

var (
    ErrWhatsAppNumberEmpty   = errors.New("identity: whatsapp number is empty")
    ErrWhatsAppNumberInvalid = errors.New("identity: whatsapp number invalid for BR E.164")
)

// padrão BR-only conforme S-03: DDD (2) + 9 (celular) + 8 dígitos.
var brCellPattern = regexp.MustCompile(`^\+55\d{2}9\d{8}$`)

func NewWhatsAppNumber(raw string) (WhatsAppNumber, error) {
    // normalização: remove espaços, parênteses, traços; força prefixo +55; rejeita números fixos.
    // (algoritmo detalhado em go-implementation; este shape é mandatório)
    cleaned := normalizeRaw(raw)
    if cleaned == "" {
        return WhatsAppNumber{}, ErrWhatsAppNumberEmpty
    }
    if !brCellPattern.MatchString(cleaned) {
        return WhatsAppNumber{}, fmt.Errorf("identity: %q: %w", raw, ErrWhatsAppNumberInvalid)
    }
    return WhatsAppNumber{e164: cleaned}, nil
}

func (w WhatsAppNumber) String() string { return w.e164 } // canônico
func (w WhatsAppNumber) Equal(o WhatsAppNumber) bool { return w.e164 == o.e164 }

// Masked retorna "+55 DD 9****-NNNN" (4 dígitos finais visíveis) — ADR-003.
func (w WhatsAppNumber) Masked() string { /* "+55 11 9****-7777" */ }

func normalizeRaw(raw string) string { /* strip, prefixo +55, validações iniciais */ }
```

**`valueobjects/email.go`**

```go
package valueobjects

import (
    "errors"
    "fmt"
    "net/mail"
    "strings"
)

type Email struct {
    addr string // imutável; lowercase
}

var ErrEmailInvalid = errors.New("identity: email invalid")

func NewEmail(raw string) (Email, error) {
    trimmed := strings.TrimSpace(strings.ToLower(raw))
    if trimmed == "" {
        return Email{}, fmt.Errorf("identity: %w", ErrEmailInvalid)
    }
    if _, err := mail.ParseAddress(trimmed); err != nil {
        return Email{}, fmt.Errorf("identity: %q: %w", raw, ErrEmailInvalid)
    }
    return Email{addr: trimmed}, nil
}

func (e Email) String() string { return e.addr }
func (e Email) Equal(o Email) bool { return e.addr == o.addr }

// Masked retorna "<primeira-letra>***@<domínio>" — ADR-003.
func (e Email) Masked() string { /* "j***@example.com" */ }
```

**`pii/mask.go`** (helper externo para `display_name`):

```go
package pii

import "unicode/utf8"

// MaskDisplayName mascara display_name conforme ADR-003.
// - "" → ""
// - 1 rune → "*"
// - >=2 runes → primeira rune + 4 asteriscos fixos
func MaskDisplayName(name string) string {
    if name == "" {
        return ""
    }
    r, size := utf8.DecodeRuneInString(name)
    if r == utf8.RuneError {
        return "****"
    }
    if size == len(name) {
        return "*"
    }
    return string(r) + "****"
}
```

**`entities/user.go`** (esqueleto):

```go
package entities

import (
    "time"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type Status string

const (
    StatusActive  Status = "ACTIVE"
    StatusDeleted Status = "DELETED"
)

type User struct {
    id          string                       // UUID v4 (RF-01)
    whatsapp    valueobjects.WhatsAppNumber  // imutável durante vida do agregado
    email       valueobjects.Email           // opcional (zero-value ok); first-write-wins lógica vive no UC
    displayName string                       // opcional; first-write-wins (RF-08-bis)
    status      Status                       // ACTIVE | DELETED
    createdAt   time.Time
    updatedAt   time.Time
    deletedAt   time.Time                    // zero quando ACTIVE
}

// New constrói um User novo (status=ACTIVE). ID é gerado por entities.NewID()
// (ver entities/id.go) — sem IDGenerator injetado, sem DI (ADR-008/R6.8).
// created_at e updated_at são resolvidos inline com time.Now().UTC() (R6.7).
func New(whatsapp valueobjects.WhatsAppNumber, opts ...Option) User { /* ... */ }

type Option func(*User)

func WithEmail(e valueobjects.Email) Option           { /* ... */ }
func WithDisplayName(name string) Option              { /* ... */ }

// ID, WhatsApp, Email, DisplayName, Status, CreatedAt, UpdatedAt, DeletedAt — getters (R1: métodos).

// MarkDeleted seta status=DELETED e deleted_at=now (RF-06; CHECK constraint defende invariante).
func (u *User) MarkDeleted(now time.Time) { /* ... */ }

// Reanimate reseta deleted_at, status=ACTIVE, zera email/display_name (RF-08-ter).
// Caller deve fornecer os novos valores via subsequent setters/UC.
func (u *User) Reanimate(now time.Time) { /* ... */ }

// CanReanimate decide se a janela ReanimationWindow permite reanimar (ADR-006).
func (u User) CanReanimate(now time.Time) bool { /* ... */ }
```

**`policies.go`** (ADR-006):

```go
package domain

import "time"

const ReanimationWindow = 30 * 24 * time.Hour
```

**`entitlement.go`** (ADR-001, ADR-002):

```go
package domain

import "time"

type SubscriptionStatus string

const (
    SubscriptionTrialing        SubscriptionStatus = "TRIALING"
    SubscriptionActive          SubscriptionStatus = "ACTIVE"
    SubscriptionPastDue         SubscriptionStatus = "PAST_DUE"
    SubscriptionCanceledPending SubscriptionStatus = "CANCELED_PENDING"
    SubscriptionExpired         SubscriptionStatus = "EXPIRED"
    SubscriptionRefunded        SubscriptionStatus = "REFUNDED"
)

type Subscription interface {
    Status() SubscriptionStatus
    PeriodEnd() time.Time
    GracePeriodEnd() time.Time // zero time = sem grace period
}

type Reason string

const (
    ReasonNoSubscription  Reason = "no_subscription"
    ReasonActive          Reason = "active"
    ReasonTrialing        Reason = "trialing"
    ReasonCanceledPending Reason = "canceled_pending"
    ReasonPastDueGrace    Reason = "past_due_grace"
    ReasonExpired         Reason = "expired"
    ReasonRefunded        Reason = "refunded"
    ReasonPastDueNoGrace  Reason = "past_due_no_grace"
)

// IsEntitled é função pura: sem I/O, sem cache, sem efeito colateral (RF-12).
// Cobre as 11 transições documentadas em RF-12 + sub == nil.
func IsEntitled(sub Subscription, now time.Time) (bool, Reason) {
    if sub == nil {
        return false, ReasonNoSubscription
    }
    switch sub.Status() {
    case SubscriptionActive:
        if sub.PeriodEnd().After(now) {
            return true, ReasonActive
        }
        return false, ReasonExpired
    case SubscriptionTrialing:
        if sub.PeriodEnd().After(now) {
            return true, ReasonTrialing
        }
        return false, ReasonExpired
    case SubscriptionPastDue:
        grace := sub.GracePeriodEnd()
        if !grace.IsZero() && grace.After(now) {
            return true, ReasonPastDueGrace
        }
        return false, ReasonPastDueNoGrace
    case SubscriptionCanceledPending:
        if sub.PeriodEnd().After(now) {
            return true, ReasonCanceledPending
        }
        return false, ReasonExpired
    case SubscriptionExpired:
        return false, ReasonExpired
    case SubscriptionRefunded:
        return false, ReasonRefunded
    default:
        // status desconhecido: comportamento conservador.
        return false, ReasonExpired
    }
}
```

### Application (`internal/identity/application/...`)

**`errors.go`** (ADR-004) — já detalhado na ADR.

**`interfaces/user_repository.go`** (RF-10):

```go
package interfaces

import (
    "context"
    "time"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type WhatsAppHistoryEntry struct {
    ID         string    // gerado pelo construtor da entidade no domain (entities.NewID)
    Number     string
    Active     bool
    LinkedAt   time.Time
    UnlinkedAt time.Time // zero quando ainda ativo
    Reason     string    // opcional
}

// UserRepository não recebe tx em assinaturas — a instância concreta já carrega
// a database.DBTX correta (pool ou tx), amarrada pelo RepositoryFactory.
type UserRepository interface {
    UpsertByWhatsAppNumber(ctx context.Context, u entities.User, now time.Time) (entities.User, error)
    FindByID(ctx context.Context, id string) (entities.User, error)
    FindByWhatsAppNumber(ctx context.Context, number valueobjects.WhatsAppNumber) (entities.User, error)
    MarkDeleted(ctx context.Context, id string, now time.Time) error
    AppendWhatsAppHistory(ctx context.Context, userID string, entry WhatsAppHistoryEntry) error
}
```

**`interfaces/repository_factory.go`** (ADR-008):

```go
package interfaces

import "github.com/JailtonJunior94/devkit-go/pkg/database"

// RepositoryFactory devolve instâncias de repositório amarradas a uma database.DBTX
// (pool ou tx recebida no callback de uow.UnitOfWork[T].Do). Permite orquestrar
// 2+ repos diferentes na mesma transação dentro de um único use case.
type RepositoryFactory interface {
    UserRepository(db database.DBTX) UserRepository
    // WhatsAppHistoryRepository(db database.DBTX) WhatsAppHistoryRepository  // futuro
}
```

Erros tipados (`application/errors.go`):
- `application.ErrUserNotFound` em `FindByID`/`FindByWhatsAppNumber` quando inexistente ou soft-deletado.
- `application.ErrWhatsAppNumberInUse` em violação rara de unicidade (race com índice parcial).
- `application.ErrEmailInUse` idem para `email`.

**`usecases/upsert_user_by_whatsapp.go`** (esqueleto — segue runbook §5):

```go
package usecases

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/JailtonJunior94/devkit-go/pkg/database"
    "github.com/JailtonJunior94/devkit-go/pkg/database/uow"
    "github.com/JailtonJunior94/devkit-go/pkg/observability"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/output"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

const prefixUpsertUser = "identity.usecase.upsert_user_by_whatsapp:"

type UpsertUserByWhatsApp struct {
    uow     uow.UnitOfWork[entities.User]
    factory interfaces.RepositoryFactory
    o11y    observability.Observability
}

func NewUpsertUserByWhatsApp(
    u uow.UnitOfWork[entities.User],
    factory interfaces.RepositoryFactory,
    o11y observability.Observability,
) *UpsertUserByWhatsApp {
    return &UpsertUserByWhatsApp{uow: u, factory: factory, o11y: o11y}
}

func (u *UpsertUserByWhatsApp) Execute(ctx context.Context, in input.UpsertUserByWhatsApp) (output.UpsertUserByWhatsApp, error) {
    ctx, span := u.o11y.Tracer().Start(ctx, "identity.usecase.upsert_user_by_whatsapp")
    defer span.End()

    result, err := u.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.User, error) {
        userRepo := u.factory.UserRepository(tx)

        existing, findErr := userRepo.FindByWhatsAppNumber(ctx, in.WhatsApp)
        switch {
        case errors.Is(findErr, application.ErrUserNotFound):
            // entities.New gera UUID via entities.NewID() e resolve created_at/updated_at
            // com time.Now().UTC() inline no construtor — sem IDGenerator injetado.
            candidate := entities.New(in.WhatsApp,
                entities.WithEmail(in.Email),
                entities.WithDisplayName(in.DisplayName),
            )
            persisted, err := userRepo.UpsertByWhatsAppNumber(ctx, candidate, time.Now().UTC())
            if err != nil {
                return entities.User{}, fmt.Errorf("%s upsert insert: %w", prefixUpsertUser, err)
            }
            return persisted, nil

        case findErr != nil:
            return entities.User{}, fmt.Errorf("%s find by whatsapp: %w", prefixUpsertUser, findErr)
        }

        existing.SetDisplayNameIfEmpty(in.DisplayName) // first-write-wins no agregado
        existing.SetEmailIfEmpty(in.Email)
        persisted, err := userRepo.UpsertByWhatsAppNumber(ctx, existing, time.Now().UTC())
        if err != nil {
            return entities.User{}, fmt.Errorf("%s upsert update: %w", prefixUpsertUser, err)
        }
        return persisted, nil
    })

    if err != nil {
        u.o11y.Logger().Error(ctx, "identity.usecase.upsert_failed",
            observability.String("layer", "usecase"),
            observability.String("operation", "upsert_user_by_whatsapp"),
            observability.String("whatsapp", in.WhatsApp.Masked()),
            observability.Error(err),
        )
        return output.UpsertUserByWhatsApp{}, err
    }
    return toOutput(result), nil
}
```

> Demais use cases (`FindUserByID`, `FindUserByWhatsApp`, `MarkUserDeleted`) seguem o mesmo shape: `uow.UnitOfWork[T]` tipado (T = `entities.User` para reads; `struct{}` via `uow.NewVoid` para `MarkUserDeleted`); factory.UserRepository(tx) dentro do callback; `time.Now().UTC()` **inline no call-site**; wrap de erros com prefixo da operação (R5.10).

### Infrastructure

**`repositories/factory.go`** (ADR-008):

```go
package repositories

import (
    "github.com/JailtonJunior94/devkit-go/pkg/database"
    "github.com/JailtonJunior94/devkit-go/pkg/observability"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories/postgres"
)

type repositoryFactory struct {
    o11y observability.Observability
}

func NewRepositoryFactory(o11y observability.Observability) interfaces.RepositoryFactory {
    return &repositoryFactory{o11y: o11y}
}

func (r *repositoryFactory) UserRepository(db database.DBTX) interfaces.UserRepository {
    return postgres.NewUserRepository(r.o11y, db)
}
```

**`repositories/postgres/user_repository.go`** — shape mandatório em §10 (consome `internal/platform/sqlnull` em colunas anuláveis, sem `PrepareContext`).

### HTTP, Jobs, Consumers, Producers

**`infrastructure/http/server/router.go`** — placeholder concreto (item 4 do Padrão):

```go
package server

import "github.com/go-chi/chi/v5"

type UserRouter struct {
    upsertHandler *UpsertUserByWhatsAppHandler // só preenchido quando E3 trouxer handler real
}

func NewUserRouter(upsert *UpsertUserByWhatsAppHandler) *UserRouter {
    return &UserRouter{upsertHandler: upsert}
}

func (rt *UserRouter) Register(r chi.Router) {
    // No MVP de E1: nenhum endpoint registrado. Quando E3 plugar, mover handlers para Route(...).
}
```

**`infrastructure/http/server/handlers/*.go`** — handlers HTTP consomem `github.com/JailtonJunior94/devkit-go/pkg/responses` (ver runbook §8). **Proibido** helpers locais `writeError`/`writeJSON`. Códigos semânticos viajam em `details`.

**Slots vazios com `doc.go` placeholder** (cada um declarando o pacote e a reserva de uso para E2/E3/E4):

- `application/dtos/input/doc.go`, `application/dtos/output/doc.go`
- `domain/services/doc.go`
- `infrastructure/http/client/doc.go`
- `infrastructure/jobs/handlers/doc.go`
- `infrastructure/messaging/database/{consumers,producers}/doc.go`

## Design de Implementação

### Interfaces Chave

```go
// application/interfaces — port consumido por use cases.
type UserRepository interface { /* ver §Application */ }

// domain — interface mínima consumida por IsEntitled (ADR-002).
type Subscription interface {
    Status() SubscriptionStatus
    PeriodEnd() time.Time
    GracePeriodEnd() time.Time
}

// domain/entities — geração de ID dentro do domínio (ADR-008/R6.8).
// Função package-level chamada pelos construtores das entidades; sem DI, sem interface.
//   func NewID() string { return uuid.NewString() }
```

### Modelos de Dados

**Entidade `User`** (campos com semântica):

| Campo | Tipo Go | Coluna SQL | Notas |
|---|---|---|---|
| `id` | `string` (UUID v4) | `id UUID PRIMARY KEY` | RF-01; estável; nunca deriva de número. |
| `whatsapp` | `valueobjects.WhatsAppNumber` | `whatsapp_number TEXT NOT NULL` | RF-03/04; E.164 BR. |
| `email` | `valueobjects.Email` | `email TEXT NULL` | RF-05; opcional. |
| `displayName` | `string` | `display_name TEXT NULL` | RF-08-bis; `first-write-wins`. |
| `status` | `entities.Status` (`string`) | `status TEXT NOT NULL DEFAULT 'ACTIVE'` | RF-06; `ACTIVE` ou `DELETED`. |
| `createdAt` | `time.Time` | `created_at TIMESTAMPTZ NOT NULL DEFAULT now()` | — |
| `updatedAt` | `time.Time` | `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()` | "touch garantido" em todo upsert (RF-10). |
| `deletedAt` | `time.Time` (zero = NULL) | `deleted_at TIMESTAMPTZ NULL` | RF-06; invariante CHECK. |

**Entidade `WhatsAppHistoryEntry`** (apenas projeção; sem comportamento próprio no MVP):

| Campo Go | Coluna SQL | Notas |
|---|---|---|
| `id` (gerado no repo) | `id UUID PRIMARY KEY` | UUID v4. |
| `userID` | `user_id UUID NOT NULL` | FK ON DELETE CASCADE. |
| `number` | `number TEXT NOT NULL` | string E.164 (cópia para histórico). |
| `active` | `active BOOLEAN NOT NULL` | true para vínculo atual. |
| `linkedAt` | `linked_at TIMESTAMPTZ NOT NULL` | — |
| `unlinkedAt` | `unlinked_at TIMESTAMPTZ NULL` | populado quando desativa. |
| `reason` | `reason TEXT NULL` | livre. |

**Schemas:** definições completas em ADR-007 + nas migrations `000002` e `000003`.

### Endpoints de API

**Nenhum endpoint no MVP de E1.** Slot `infrastructure/http/server/` existe para uso por E3.

## Pontos de Integração

**Nenhuma integração externa nesta etapa.** Identity é fundação interna. Integrações HTTP outbound (`httpclient`) e brokers ficam para E2 (Kiwify) e E3 (WhatsApp Business webhook).

## Abordagem de Testes

### Testes Unitários

Cobertura mínima:

| Pacote | Casos críticos | Cobertura alvo |
|---|---|---|
| `domain/valueobjects` | `NewWhatsAppNumber`: vazios, sem `+55`, com formatação (`(11) 9...`, `11 9...`), sem `9` (rejeita), com 7 dígitos finais (rejeita), com 9 dígitos finais (rejeita), e o caminho feliz. `Masked()` para entrada canônica. | 100% (CA-01) |
| `domain/valueobjects` | `NewEmail`: vazio, sem `@`, com espaços, com uppercase, válido. `Masked()` para entrada canônica. | 100% (CA-01) |
| `domain` | `IsEntitled`: caso `sub == nil` + as 11 transições enumeradas em RF-12 (parametrizado). | 100% (CA-01) |
| `domain/entities` | `User.MarkDeleted`/`Reanimate`/`CanReanimate` (borda `==`, `+1ns`, `>`). | >=95% |
| `domain/pii` | `MaskDisplayName`: vazio, 1 rune, multi-byte (acentos), 2+ runes. | 100% |
| `application/usecases` | Erros propagados; tracer/logger chamados; `time.Now()` inline. Mocks do `UserRepository` via `mockery` (já em `go.mod`). | >=85% |

**Mocks:** geração via `mockery` configurada por package (config em `.mockery.yaml` é responsabilidade do `go-implementation` na execução; esta spec apenas exige que mocks de `UserRepository` existam para testar use cases).

### Testes de Integração

> **Decisão:** este projeto precisa de integration tests?
> - [x] Tem fronteiras de IO críticas (Postgres) onde mocks não garantem correção.
> - [x] O CHECK constraint e os índices parciais só validam contra Postgres real.
> - [x] Custo de testcontainers é baixo (já há precedente: `internal/platform/outbox` exige Postgres real).
>
> **Sim, são obrigatórios.** Build tag `//go:build integration`.

Stack:

- [`testcontainers-go`](https://golang.testcontainers.org/) com imagem `postgres:16` (ou alinhar com produção).
- `golang-migrate` aplica migrations no container antes do teste.
- Pacote alvo: `internal/identity/infrastructure/repositories/postgres/user_repository_integration_test.go`.

Cenários (cobrem CA-04 a–h):

1. **(a)** primeiro `UpsertByWhatsAppNumber` cria User com UUID novo; segundo (mesmo número, mesmos campos) atualiza `updated_at`.
2. **(b)** `MarkDeleted` + `FindByID` → `ErrUserNotFound` (filtro `deleted_at IS NULL`).
3. **(c)** mudança de número via método de repo: novo registro em `user_whatsapp_history` ativo, anterior desativado (este cenário **não** é exercitado por UC no MVP; teste exercita apenas `AppendWhatsAppHistory` + leitura).
4. **(d)** soft delete + upsert dentro da janela (`now - deletedAt = 29d`) → mesmo UUID, `email`/`display_name` zerados antes do input.
5. **(e)** soft delete + upsert fora da janela (`now - deletedAt = 31d`) → novo UUID; linha antiga permanece soft-deletada.
6. **(f)** `display_name` first-write-wins: segundo upsert com nome diferente preserva o primeiro.
7. **(g)** upsert sem mudanças → `updated_at` muda (touch garantido).
8. **(h)** SQL direto tentando setar `status='DELETED' AND deleted_at IS NULL` é rejeitado pelo CHECK.

### Testes E2E

Para o MVP de E1, **smoke E2E é o próprio teste de integração descrito acima** — não há endpoint HTTP nem worker para exercitar end-to-end. A pipeline CI executa:

```
go test -race -count=1 -tags=integration ./internal/identity/...
```

Risco operacional de Docker no CI: já mitigado por R-02 do PRD. Se a esteira não suportar testcontainers, fallback é job manual local até resolver (decisão do Time de Plataforma, fora do escopo desta techspec).

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Migrations** (`000002_identity_users`, `000003_identity_user_whatsapp_history`) — primeiro, porque toda implementação depende do schema.
2. **Domínio puro** — `entities.NewID()` (`domain/entities/id.go`), VOs (`WhatsAppNumber`, `Email`), helper `pii.MaskDisplayName`, `entitlement.go` (`Subscription`, `Reason`, `IsEntitled`), `policies.ReanimationWindow`, agregados `User` + `WhatsAppHistoryEntry` (com `entities.NewID()` inline nos construtores).
3. **Testes unitários do domínio** — antes de qualquer infra; alvo de cobertura 100% nos VOs e `IsEntitled`. ID validado por invariante (UUID v4 não vazio), não por valor determinístico.
4. **Erros tipados** em `application/errors.go`.
5. **Ports `UserRepository` + `RepositoryFactory`** em `application/interfaces/`.
6. **Use cases** em `application/usecases/` (ordem: `UpsertUserByWhatsApp` → `FindUserByID` → `FindUserByWhatsApp` → `MarkUserDeleted`). Cada UC recebe `uow.UnitOfWork[T]` (T tipado por retorno) + `RepositoryFactory` no construtor. Testes unitários com mocks dos dois.
7. **`internal/platform/sqlnull`** (se ainda não estiver presente) — helpers `Str`/`Time` + testes.
8. **Implementação Postgres** do `UserRepository` em `infrastructure/repositories/postgres/` consumindo `sqlnull` + impl da `RepositoryFactory` em `infrastructure/repositories/factory.go`. Testes de integração (build tag `integration`).
9. **Handlers HTTP** em `infrastructure/http/server/handlers/` consumindo `devkit-go/pkg/responses` + `infrastructure/http/server/router.go` (placeholder).
10. **`module.go`** com `NewIdentityModule(cfg, o11y, mgr)`. Testes de compilação (`go build ./...`).
11. **Wiring em `cmd/server/server.go`** instanciando `IdentityModule`. **Refactor outbox** (§17) atualizando `cmd/worker/worker.go` em PR paralelo.
12. **`.golangci.yml`** com `depguard` + `forbidigo`. Validar `golangci-lint run`.
13. **`doc.go`** documentando contratos exportados e referências às ADRs.
14. **Slots `doc.go`** placeholder em cada subpasta vazia (`application/dtos/{input,output}`, `domain/services`, `infrastructure/http/client`, `infrastructure/jobs/handlers`, `infrastructure/messaging/database/{consumers,producers}`).
15. **F-12 (limpeza documental):** garantir ausência de `JWT|RBAC|role|is_admin` em `internal/identity/**/*.go` (criação do zero já satisfaz).

### Dependências Técnicas

- Postgres reachable para testes de integração (testcontainers ou docker-compose local).
- `golang-migrate` runner já presente em `cmd/migrate` — reutilizado.
- `github.com/google/uuid v1.6.0` — dependência pura (sem IO) já em `go.mod`; consumida diretamente por `internal/identity/domain/entities/id.go` em `entities.NewID()`.
- `internal/platform/sqlnull` (já presente no working tree) — helpers `Str`/`Time` consumidos pelo repo Postgres em colunas anuláveis.
- `devkit-go v0.4.0` — versão já fixada em `go.mod`; pacotes consumidos: `pkg/database`, `pkg/database/manager`, `pkg/database/uow`, `pkg/observability`, `pkg/http_server/chi_server`, `pkg/responses`. Nenhum upgrade necessário.

## Monitoramento e Observabilidade

### Tracing (mandatório)

Todo método com I/O ou orquestração relevante abre span próprio:

- Use cases: `identity.usecase.<nome>` (e.g., `identity.usecase.upsert_user_by_whatsapp`).
- Repositórios: `identity.repository.<entidade>.<operação>` (e.g., `identity.repository.user.upsert_by_whatsapp_number`).
- Quando handler HTTP existir: `identity.handler.<operação>`.

Pattern obrigatório (R6 + ADR-005):

```go
ctx, span := uc.o11y.Tracer().Start(ctx, "identity.usecase.upsert_user_by_whatsapp")
defer span.End()
// ...
span.RecordError(err) // em todo erro de I/O
```

`go-implementation` valida no momento da implementação que `o11y.Tracer()` retorna a forma esperada (D-01).

### Logging estruturado

Logger via `o11y.Logger()` (devkit-go encapsula `log/slog`). Campos obrigatórios em logs de identity:

| Campo | Quando | Forma |
|---|---|---|
| `layer` | sempre | `"usecase"`, `"repository"`, `"handler"` |
| `operation` | sempre | nome curto (`"upsert_user_by_whatsapp"`) |
| `entity` | quando aplicável | `"user"`, `"user_whatsapp_history"` |
| `user_id` | quando ID conhecido | `user.ID()` — não é PII |
| `whatsapp` | sempre que loga PII | `WhatsAppNumber.Masked()` (ADR-003) |
| `email` | sempre que loga PII | `Email.Masked()` |
| `display_name` | sempre que loga PII | `pii.MaskDisplayName(...)` |
| `error` | em erro | `observability.Error(err)` |

Eventos esperados (lista mínima — `go-implementation` pode adicionar quando necessário):

- `identity.usecase.upsert_started` (Debug)
- `identity.usecase.upsert_succeeded` (Info)
- `identity.usecase.upsert_failed` (Error)
- `identity.repository.user.upsert.prepare_failed` (Error)
- `identity.repository.user.upsert.stmt_close_failed` (Error)
- `identity.repository.user.find_by_id.not_found` (Debug — opcional)

### Métricas

**Sem métricas Prometheus no MVP** (Restrição Operacional do PRD). Quando E4 introduzir o pilar de métricas, identity ganha contadores derivados de logs/traces — fora do escopo desta techspec.

## Considerações Técnicas

### Decisões Chave (ADRs)

Cada decisão material vive em uma ADR separada (cumpre Etapa 6 da skill):

| ADR | Fecha | Resumo |
|---|---|---|
| [ADR-001](./adr-001-reason-string-type.md) | Q-06 | `Reason` como `type Reason string` com constantes nomeadas. |
| [ADR-002](./adr-002-subscription-contract-interface.md) | Q-02 | `Subscription` mínima como interface em `identity/domain`. |
| [ADR-003](./adr-003-pii-masking-vo-methods.md) | Q-01 | Mascaramento de PII via `Masked()` nos VOs + `pii.MaskDisplayName`. |
| [ADR-004](./adr-004-typed-errors-application-package.md) | Q-04 | Erros tipados em `internal/identity/application/errors.go` (sentinels). |
| [ADR-005](./adr-005-identity-module-shape-mvp.md) | Q-05 | `IdentityModule{RepositoryFactory, UserRouter}` com `NewIdentityModule(cfg, o11y, mgr)` — 3 params, sem `IDGenerator`. |
| [ADR-006](./adr-006-reanimation-window-constant.md) | R-06 | `ReanimationWindow = 30 * 24 * time.Hour` como constante de domínio. |
| [ADR-007](./adr-007-postgres-partial-unique-indexes.md) | Q-03 | Índices parciais únicos para coexistir com soft delete + invariante CHECK. |
| [ADR-008](./adr-008-repository-factory-per-module.md) | Iter 1–4 | `RepositoryFactory` por módulo + `uow.UnitOfWork[T]` da devkit direto; proibido reimplementar UoW localmente; proibido injetar `IDGenerator`. |

### Riscos Conhecidos

Replicados do PRD com mitigação técnica desta techspec:

| Risco | Mitigação técnica |
|---|---|
| **R-01:** ferramenta de migration | Manter `golang-migrate`; nenhum câmbio. |
| **R-02:** testcontainers no CI | Build tag `//go:build integration`; fallback é job manual local. |
| **R-03:** drift do épico | Após techspec aprovada, atualizar `docs/epics/epic-01-identity-foundation.md` para refletir realidade. |
| **R-04:** `is_admin` na discovery | Override em 2026-06-05 prevalece; nenhum atributo de autorização no agregado (RF-02). |
| **R-05:** ambiguidade BR no `WhatsAppNumber` | Regex `^\+55\d{2}9\d{8}$`; testes parametrizados cobrem casos limítrofes. |
| **R-06:** janela de reanimação ↔ E4 | `ReanimationWindow` como constante (ADR-006); E4 importa. |
| **R-07:** `display_name` ocioso até E3 | Aceito; slot persistido + helper de mascaramento. |
| **R-08:** invariante `status ⇔ deleted_at` | CHECK constraint (ADR-007) + `User.MarkDeleted(now)`/`User.Reanimate(now)` setam ambos juntos. |
| **R-09:** "touch garantido" mascara mudanças | Documentar em `doc.go`; consumidores de dirty-tracking comparam antes ou aguardam eventos. |
| **D-01:** `o11y.Tracer()` não exercitado | `go-implementation` valida a assinatura na primeira execução; spec adota como padrão. |
| **D-02:** `srv.RegisterRouters` não chamado | MVP entrega `UserRouter == nil`; nenhuma chamada feita até E3. |

### Conformidade com Padrões

Regras aplicáveis (de `.claude/rules/governance.md` e `AGENTS.md`):

- **R-GOV-001:** precedência respeitada; `go-implementation` prevalece sobre `object-calisthenics-go` quando houver conflito.
- **AGENTS.md "Padrão Obrigatório de Módulo"** (itens 1–7): satisfeito por ADR-005 + ADR-008.
- **AGENTS.md "Layout Obrigatório por Módulo"**: estrutura completa de pastas mantida; slots vazios materializados com `doc.go` placeholder.
- **AGENTS.md R0–R7 + R6.7–R6.11 (extensões desta techspec)** + memórias de governança: aplicados conforme §Arquitetura e §Handoff.
- **AGENTS.md "Plataforma Compartilhada"**: identity consome `internal/platform/sqlnull` (criado para esse fim) e **não** importa `internal/platform/id` (domain autossuficiente — ADR-008). Capacidade transversal de UoW vem da devkit; proibido reimplementar.
- **AGENTS.md "Worker, HTTP Outbound e Outbox"**: identity não publica eventos no MVP; o outbox sofre o refactor descrito em §17 dentro do escopo de E1.
- **`.claude/rules/governance.md` Política de Evidência**: cada decisão tem ADR; cada requisito tem mapeamento (§Mapeamento abaixo); runbook é fonte de verdade do shape de código.

### Mapeamento Requisito → Decisão → Teste

| Requisito | Onde implementa | Teste |
|---|---|---|
| **RF-01** UUID v4 estável | `entities.User` + `entities.NewID()` em `domain/entities/id.go` (ADR-008/R6.8 — sem DI) | Unit em `entities/user_test.go` (valida invariante UUID v4, não valor determinístico). |
| **RF-02** Sem atributo de autorização | Ausência ativa em `User` e schema; `forbidigo` opcional. | `grep -RInE "JWT|RBAC|\brole\b|is_admin" internal/identity/` vazio (CA-03). |
| **RF-03** `WhatsAppNumber` VO E.164 BR | `valueobjects/whatsapp_number.go` | Unit parametrizado 100% (CA-01). |
| **RF-04** APIs só com VO, nunca string | Assinatura do port e dos use cases | Compile-time enforcement. |
| **RF-05** `Email` VO opcional | `valueobjects/email.go` | Unit 100% (CA-01). |
| **RF-06** Soft delete + CHECK invariante | `User.MarkDeleted`, schema, ADR-007 | Integração CA-04(b) e (h). |
| **RF-07** Filtro `deleted_at IS NULL` em leituras | `user_repository.go` SQL | Integração CA-04(b). |
| **RF-08** UNIQUE parcial em `whatsapp_number`/`email` | ADR-007 | Integração CA-04(a). |
| **RF-08-bis** `display_name` first-write-wins | UC `UpsertUserByWhatsApp` lê estado antes; `pii.MaskDisplayName` em logs | Integração CA-04(f). |
| **RF-08-ter** Reanimação ≤30d / criação >30d | `User.CanReanimate` + UC; ADR-006 | Integração CA-04(d) e (e). |
| **RF-09** Schema `user_whatsapp_history` | Migration `000003` | Integração CA-04(c). |
| **RF-10** Port com semântica "touch garantido" + `RepositoryFactory` (ADR-008) | `application/interfaces/{user_repository,repository_factory}.go` | Integração CA-04(g) + multi-repo. |
| **RF-11** Postgres amarrado via `database.DBTX` recebida do `uow.Do` callback | `infrastructure/repositories/factory.go` + `postgres/user_repository.go` | Integração com `uow.New[T]`. |
| **RF-12** `IsEntitled` puro 11 transições + nil | `domain/entitlement.go` | Unit 100% (CA-01). |
| **RF-13** `Subscription` mínima em `domain` | ADR-002 | Compile-time; E2 satisfaz. |
| **RF-14** Helper de PII com `Masked()` + `MaskDisplayName` | ADR-003 | Unit em VOs + `pii`. |
| **RF-15** `depguard` enforça fronteiras | `.golangci.yml` editado | `golangci-lint run` verde (CA-02). |
| **RF-16** Migration via `golang-migrate` | `migrations/000002`/`000003` | Aplicação local. |
| **RF-17** Docs sem RBAC/JWT | `doc.go` novo + ausência | `grep` vazio (CA-03). |
| **RF-18** `NewIdentityModule(...)` no Padrão | `module.go` (ADR-005) | `go build ./...`. |

### Arquivos Relevantes e Dependentes

**Criar:**

- `internal/identity/module.go`
- `internal/identity/doc.go`
- `internal/identity/application/errors.go`
- `internal/identity/application/interfaces/user_repository.go`
- `internal/identity/application/interfaces/repository_factory.go` (ADR-008)
- `internal/identity/application/usecases/upsert_user_by_whatsapp.go`
- `internal/identity/application/usecases/find_user_by_id.go`
- `internal/identity/application/usecases/find_user_by_whatsapp.go`
- `internal/identity/application/usecases/mark_user_deleted.go`
- `internal/identity/application/dtos/{input,output}/...` (DTOs + `doc.go` por subpacote)
- `internal/identity/domain/entitlement.go`
- `internal/identity/domain/policies.go`
- `internal/identity/domain/entities/id.go` (`entities.NewID()` — ADR-008)
- `internal/identity/domain/entities/user.go`
- `internal/identity/domain/entities/whatsapp_history_entry.go`
- `internal/identity/domain/valueobjects/whatsapp_number.go`
- `internal/identity/domain/valueobjects/email.go`
- `internal/identity/domain/pii/mask.go`
- `internal/identity/domain/services/doc.go` (placeholder)
- `internal/identity/infrastructure/repositories/factory.go` (ADR-008)
- `internal/identity/infrastructure/repositories/postgres/user_repository.go`
- `internal/identity/infrastructure/http/server/router.go` (UserRouter placeholder)
- `internal/identity/infrastructure/http/server/handlers/upsert_user_by_whatsapp_handler.go` (usa `devkit-go/pkg/responses`)
- `internal/identity/infrastructure/http/client/doc.go` (placeholder)
- `internal/identity/infrastructure/jobs/handlers/doc.go` (placeholder)
- `internal/identity/infrastructure/messaging/database/{consumers,producers}/doc.go` (placeholder)
- Testes correspondentes (`*_test.go`) + `*_integration_test.go`
- `migrations/000002_identity_users.up.sql`/`.down.sql`
- `migrations/000003_identity_user_whatsapp_history.up.sql`/`.down.sql`

**Editar:**

- `cmd/server/server.go` — instanciar `IdentityModule(cfg, o11y, dbManager)` (sem `RegisterRouters` no MVP).
- `cmd/worker/worker.go` — receber refactor outbox (§17): adotar `uow.New[[]outbox.Row]` + `OutboxRepositoryFactory` para os jobs.
- `internal/platform/outbox/storage_postgres.go` — refactor para receber `database.DBTX` no construtor (§17).
- `internal/platform/outbox/dispatcher.go` — receber `uow.UnitOfWork[[]outbox.Row]` + factory (§17).
- `.golangci.yml` — adicionar regras `depguard` e `forbidigo` (RF-15 + ADR-003).

**Já existe no working tree (não editar):**

- `internal/platform/sqlnull/{sqlnull.go,sqlnull_test.go}` — helpers compartilhados, pacote testado.
- `internal/platform/id/id.go` — preservado por compatibilidade histórica, **não importado** por módulos novos.

**Não editar nesta etapa:**

- `configs/config.go` (nenhuma seção nova).
- Demais módulos.

---

## §10 — Persistência: Shape Obrigatório de Repository

Cada método público de `infrastructure/repositories/postgres/user_repository.go` segue o shape canônico (espelha runbook §7).

**Princípios inegociáveis:**

- Construtor recebe `database.DBTX` (pool **ou** tx) e a instância carrega para todos os métodos. **Sem** `tx` em assinatura de método (ADR-008).
- **Sem `PrepareContext`** — a interface `database.DBTX` da devkit não expõe `PrepareContext`; usar `ExecContext`/`QueryContext`/`QueryRowContext` direto (pgx cacheia prepared statements internamente).
- **Sem `database.FromContext(ctx)`** dentro do repo — o UoW da devkit propaga tx para o `tx database.DBTX` do callback, que vai para a factory.
- `sqlnull.Str` / `sqlnull.Time` em colunas anuláveis (proibido helper local — R6.11).
- `pgerrcode.UniqueViolation` + `ConstraintName` mapeia para sentinel de `application/errors.go`.
- `errors.Is(err, sql.ErrNoRows)` antes de outros caminhos.
- Tracer span `identity.repository.user.<op>` por método; `span.RecordError(err)` em IO.
- Log estruturado em erro com PII mascarada (`candidate.WhatsApp().Masked()`).

**Shape canônico:**

```go
package postgres

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "time"

    "github.com/jackc/pgerrcode"
    "github.com/jackc/pgx/v5/pgconn"

    "github.com/JailtonJunior94/devkit-go/pkg/database"
    "github.com/JailtonJunior94/devkit-go/pkg/observability"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/sqlnull"
)

const prefixUserRepository = "identity.repository.user:"

type userRepository struct {
    o11y observability.Observability
    db   database.DBTX
}

func NewUserRepository(o11y observability.Observability, db database.DBTX) interfaces.UserRepository {
    return &userRepository{o11y: o11y, db: db}
}

func (r *userRepository) UpsertByWhatsAppNumber(
    ctx context.Context,
    candidate entities.User,
    now time.Time,
) (entities.User, error) {
    ctx, span := r.o11y.Tracer().Start(ctx, "identity.repository.user.upsert_by_whatsapp_number")
    defer span.End()

    // Sequência atômica: o UoW da devkit já amarra esta operação (e operações em
    // outros repos chamadas no mesmo callback) na mesma transação. A lógica de
    // reanimação (RF-08-ter) é decidida pelo use case via Find + branch; aqui
    // o INSERT … ON CONFLICT (whatsapp_number) WHERE deleted_at IS NULL aplica
    // first-write-wins no display_name/email via COALESCE.
    const query = `
        INSERT INTO users (
            id, whatsapp_number, email, display_name, status,
            created_at, updated_at, deleted_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, NULL)
        ON CONFLICT (whatsapp_number) WHERE deleted_at IS NULL
        DO UPDATE SET
            display_name = COALESCE(users.display_name, EXCLUDED.display_name),
            email        = COALESCE(users.email,        EXCLUDED.email),
            updated_at   = EXCLUDED.updated_at
        RETURNING id, whatsapp_number, email, display_name, status, created_at, updated_at
    `

    row := r.db.QueryRowContext(ctx, query,
        candidate.ID(),
        candidate.WhatsApp().String(),
        sqlnull.Str(candidate.Email().String()),
        sqlnull.Str(candidate.DisplayName()),
        string(entities.StatusActive),
        candidate.CreatedAt(),
        now,
    )

    var (
        id, whatsapp, status string
        email, displayName   sql.NullString
        createdAt, updatedAt time.Time
    )
    if err := row.Scan(&id, &whatsapp, &email, &displayName, &status, &createdAt, &updatedAt); err != nil {
        span.RecordError(err)
        r.o11y.Logger().Error(ctx, "identity.repository.user.upsert.scan_failed",
            observability.String("layer", "repository"),
            observability.String("entity", "user"),
            observability.String("operation", "upsert_by_whatsapp_number"),
            observability.String("whatsapp", candidate.WhatsApp().Masked()),
            observability.Error(err),
        )

        var pgErr *pgconn.PgError
        if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
            switch pgErr.ConstraintName {
            case "users_whatsapp_number_active_uniq":
                return entities.User{}, fmt.Errorf("%s %w", prefixUserRepository, application.ErrWhatsAppNumberInUse)
            case "users_email_active_uniq":
                return entities.User{}, fmt.Errorf("%s %w", prefixUserRepository, application.ErrEmailInUse)
            }
        }
        return entities.User{}, fmt.Errorf("%s upsert scan: %w", prefixUserRepository, err)
    }

    return entities.Hydrate(id, whatsapp, email.String, displayName.String, status, createdAt, updatedAt, time.Time{}), nil
}
```

> `FindByID`, `FindByWhatsAppNumber`, `MarkDeleted`, `AppendWhatsAppHistory` seguem o mesmo shape — `Tracer.Start`/`span.RecordError`, `QueryRowContext`/`ExecContext` direto, `sql.ErrNoRows` → `application.ErrUserNotFound`, `pgerrcode.UniqueViolation` → sentinel correspondente, wrap com prefixo `prefixUserRepository`. **Reanimação RF-08-ter** é orquestrada no use case (Find por número → decisão de reanimar vs criar novo via `User.CanReanimate(time.Now().UTC())` → chamadas a `MarkDeleted` ou `Reanimate` no agregado seguidas de upsert) — todas dentro do mesmo `uow.Do`, garantindo atomicidade.

## §11 — Migrations

### `000002_identity_users.up.sql`

Ver SQL completo em [ADR-007](./adr-007-postgres-partial-unique-indexes.md). Resumo:

- `CREATE TABLE users (...)` com PK UUID, `status TEXT NOT NULL DEFAULT 'ACTIVE'`, `deleted_at TIMESTAMPTZ NULL`, `display_name TEXT NULL`.
- `CHECK (status IN ('ACTIVE','DELETED'))`.
- `CHECK ((status = 'DELETED') = (deleted_at IS NOT NULL))`.
- `CREATE UNIQUE INDEX users_whatsapp_number_active_uniq ON users (whatsapp_number) WHERE deleted_at IS NULL`.
- `CREATE INDEX users_whatsapp_number_deleted_idx ON users (whatsapp_number) WHERE deleted_at IS NOT NULL`.
- `CREATE UNIQUE INDEX users_email_active_uniq ON users (email) WHERE email IS NOT NULL AND deleted_at IS NULL`.

### `000002_identity_users.down.sql`

`DROP INDEX` na ordem inversa + `DROP TABLE users`.

### `000003_identity_user_whatsapp_history.up.sql`

- `CREATE TABLE user_whatsapp_history (...)` com FK `user_id REFERENCES users(id) ON DELETE CASCADE`.
- `CREATE INDEX user_whatsapp_history_user_active_idx ON user_whatsapp_history (user_id, active)`.
- `CREATE INDEX user_whatsapp_history_number_idx ON user_whatsapp_history (number)`.

### `000003_identity_user_whatsapp_history.down.sql`

`DROP INDEX` + `DROP TABLE`.

`migrations/embed.go` continua expondo `embed.FS` — nenhuma alteração necessária.

## §12 — `.golangci.yml` (regras a adicionar — RF-15 + ADR-003)

`go-implementation` adiciona as seguintes regras:

```yaml
linters-settings:
  depguard:
    rules:
      identity-domain-no-application:
        files:
          - "**/internal/identity/domain/**/*.go"
        deny:
          - pkg: github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application
            desc: "identity/domain não pode importar identity/application (fronteira hexagonal — F-11/RF-15)"
          - pkg: github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure
            desc: "identity/domain não pode importar identity/infrastructure (fronteira hexagonal — F-11/RF-15)"
      identity-application-no-infrastructure:
        files:
          - "**/internal/identity/application/**/*.go"
        deny:
          - pkg: github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure
            desc: "identity/application não pode importar identity/infrastructure (fronteira hexagonal — F-11/RF-15)"

  forbidigo:
    forbid:
      # ADR-003: proibir String() de VOs em chamadas de logger no módulo identity.
      # A regra é guardada por revisão de PR; analyzer pega chamadas explícitas.
      - p: 'WhatsAppNumber\)\.String\(\)'
        msg: "use WhatsAppNumber.Masked() em logs (ADR-003)"
      - p: 'Email\)\.String\(\)'
        msg: "use Email.Masked() em logs (ADR-003)"
```

> Regras existentes em `.golangci.yml` (linhas 37–158 do working tree atual) **não** são removidas. Apenas adicionadas.

## §13 — Bootstrap

### `cmd/server/server.go` (edit cirúrgico)

Inserir, após a inicialização do `httpserver`:

```go
identityModule := identity.NewIdentityModule(cfg, o11y, dbManager)
if identityModule.UserRouter != nil {
    srv.RegisterRouters(identityModule.UserRouter)
}
_ = identityModule.RepositoryFactory // exposto para E2/E3 quando consumirem identity
```

**Assinatura:** `NewIdentityModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) IdentityModule` — 3 parâmetros, sem `IDGenerator` (domínio se auto-serve via `entities.NewID()`), sem `UnitOfWork` (cada UC instancia seu `uow.New[T](mgr, …)` internamente — ADR-005).

### `cmd/worker/worker.go`

**Edit cirúrgico** — o refactor do outbox (§17) entra aqui:

- Construir `uow.New[[]outbox.Row](dbManager, uow.WithObservability(o11y))` para o `DispatcherJob`.
- Construir `OutboxRepositoryFactory` e passar para os jobs.
- Demais jobs (Reaper, Housekeeping) recebem `uow.NewVoid(dbManager, …)` + factory.

Identity não adiciona jobs/consumers próprios no MVP — o trabalho aqui é exclusivamente o refactor do outbox.

## §14 — Handoff para `go-implementation`

> **Mandatório e inegociável.** Qualquer execução posterior desta techspec por outro agente/time DEVE cumprir este handoff. Spec executada sem este carregamento é considerada inválida.

1. **Carregar a skill canônica:**
   - `.agents/skills/go-implementation/SKILL.md` antes de qualquer edição em `internal/identity/**` ou no refactor do outbox.
2. **Carregar o runbook canônico:**
   - [`docs/runbooks/handler-usecase-uow-repository.md`](../../docs/runbooks/handler-usecase-uow-repository.md) é a **fonte de verdade** do padrão Handler → UC → UoW → Factory → Repository. Toda dúvida de shape de código consulta o runbook **antes** desta techspec.
3. **Verificar `go.mod`:**
   - Versão atual: `go 1.26.2`, `toolchain go1.26.4`, `devkit-go v0.4.0`.
   - Pacotes devkit obrigatórios para E1: `pkg/database`, `pkg/database/manager`, `pkg/database/uow`, `pkg/observability`, `pkg/http_server/chi_server`, `pkg/responses`.
   - Pacote interno obrigatório: `internal/platform/sqlnull` (já criado).
   - Não introduzir dependência nova sem decisão registrada.
4. **Executar as Etapas 1–5 do `go-implementation/SKILL.md`:**
   - **Etapa 1:** carregar `references/architecture.md` e R0–R7 + R6.7–R6.11 (todas `[HARD]`).
   - **Etapa 2:** carregar sob demanda as referências aplicáveis — `examples-domain-flow.md` (UC end-to-end), `examples-testing.md` (parametrizado + integration), `examples-infrastructure.md` (repository).
   - **Etapa 3:** modelar antes de escrever — fronteiras hexagonais, interface no consumidor, sem clock global, **sem `IDGenerator` injetado** (domain autossuficiente).
   - **Etapa 4:** implementar adaptando exemplos do runbook; nunca replicar literalmente.
   - **Etapa 5:** validar com R0–R7 + R6.7–R6.11 e o gate de `references/build.md`.
5. **Regras Go obrigatórias** (sempre em memória, ver §Arquitetura desta techspec):
   - R0: sem `init()`.
   - R1: métodos de struct (exceções: `IsEntitled`, `entities.NewID`, construtores).
   - R5.10: sentinels via `errors.New`; wrapping com `fmt.Errorf("prefixo: %w", err)`; tratar erro uma única vez.
   - R5.12: sem `panic` em produção.
   - R6: `context.Context` em toda fronteira de IO; DI via construtores explícitos; interface no consumidor.
   - R6.4: `var _ Interface = (*Type)(nil)` proibido em produção.
   - R6.7: **`time.Now().UTC()` inline no call-site**; proibido capturar em variável intermediária.
   - R6.8: **proibido injetar `IDGenerator`** em qualquer camada.
   - R6.9: **proibido reimplementar `UnitOfWork`** localmente; consumir devkit `pkg/database/uow`.
   - R6.10: **handler usa `devkit-go/pkg/responses`**; proibido `writeJSON`/`writeError` local.
   - R6.11: **`sqlnull.Str`/`sqlnull.Time`** em colunas anuláveis; proibido `nullableString` local.
   - R7.1: `any` em vez de `interface{}`.
   - R7.2: logging estruturado via `o11y.Logger()`.
   - R7.6: `errors.Join` para agregar (uso pontual — UoW da devkit já trata rollback transacional).
6. **Validação obrigatória ao concluir:**
   - `gofmt -w` nos arquivos alterados.
   - `go build ./...`.
   - `go vet ./...`.
   - `go test -race -count=1 ./internal/identity/... ./internal/platform/sqlnull/... ./internal/platform/outbox/...`.
   - `go test -race -count=1 -tags=integration ./internal/identity/infrastructure/repositories/postgres/...` (smoke).
   - `golangci-lint run` no escopo alterado.
   - `grep -RInE "JWT|RBAC|\\brole\\b|is_admin" internal/identity/` retorna vazio (CA-03).
   - `grep -RInE "internal/platform/uow|func writeError|func writeJSON|func nullableString|entities\\.IDGenerator|idGen[^a-zA-Z_]" internal/identity/ internal/platform/outbox/` retorna vazio (gate de aderência ao runbook).
7. **Subagents:** refator amplo é orquestrado por categoria (feedback `feedback_subagents_orchestration`) — VOs, repository, factory, usecase, handler, módulo/migrations, refactor outbox podem ser paralelizados.

## §15 — Critérios de Aceite da Própria Spec

Esta techspec é considerada inválida se:

1. Não tiver `<!-- spec-hash-prd: ... -->` no topo com o hash atual de `prd.md`.
2. Desviar para implementação (criar arquivos `.go` em `internal/identity/`).
3. Omitir wiring de `module.go`, `o11y observability.Observability`, `database.DBTX`, bootstrap em `cmd/server`/`cmd/worker`.
4. Não declarar handoff obrigatório para `.agents/skills/go-implementation/SKILL.md`.
5. Não ancorar decisões no working tree (citar arquivos que não existem como se existissem).
6. Inventar APIs do devkit-go sem registrar como drift verificável.
7. Não cobrir todas as questões abertas Q-01..Q-06 + R-06 via ADR.

A spec é considerada válida quando o índice de ADRs (§14) cobre Q-01..Q-06 + R-06 + ADR-008, o handoff §14 aponta para o runbook, §17 define o escopo do refactor outbox, e o working tree (`git status`) mostra apenas Markdown novo em `.specs/prd-identity-foundation/` + `internal/platform/sqlnull/` (já criado) + edição opcional em `docs/runs/`.

## §16 — Próximos Passos

1. **Aprovar esta techspec** (revisão humana).
2. **Executar `create-tasks`** consumindo `prd.md` + `techspec.md` + ADRs + runbook para decompor em tarefas incrementais.
3. **`execute-task`** por tarefa, carregando obrigatoriamente `.agents/skills/go-implementation/SKILL.md` + o runbook.
4. Após implementação, **atualizar `docs/epics/epic-01-identity-foundation.md`** para refletir conclusão real (mitiga R-03).

## §17 — Refactor `internal/platform/outbox` (sub-épico in-scope de E1)

Inegociável: o outbox migra para o **mesmo padrão** que identity adota (ADR-008) — `database.DBTX` no construtor do storage, `uow.UnitOfWork[T]` da devkit, `OutboxRepositoryFactory`. Sem isso, o working tree fica com dois padrões coexistindo, contradizendo a regra "fonte de verdade única" do runbook.

### Estado atual do outbox

- `internal/platform/outbox/storage_postgres.go` (cf. `git log cf5dac7`) recebe `manager.Manager` no construtor e chama `s.db.BeginTx(ctx, ...)` direto dentro de `ClaimBatch`. Padrão pré-runbook.
- `internal/platform/outbox/dispatcher.go` consome `Storage` e orquestra batches.
- `cmd/worker/worker.go` instancia `outbox.NewPostgresStorage(dbManager)` e passa para `outbox.NewDispatcherJob(...)`.

### Alvo do refactor

**Ports** (`internal/platform/outbox/ports.go` — criar):

```go
package outbox

import "github.com/JailtonJunior94/devkit-go/pkg/database"

type OutboxRepository interface {
    ClaimBatch(ctx context.Context, lockedBy string, batchSize int) ([]Row, error)
    MarkProcessed(ctx context.Context, id string) error
    // … demais métodos preservando contrato atual
}

type OutboxRepositoryFactory interface {
    OutboxRepository(db database.DBTX) OutboxRepository
}
```

**Storage refatorado:**

```go
type postgresStorage struct {
    o11y observability.Observability
    db   database.DBTX
}

func NewPostgresStorage(o11y observability.Observability, db database.DBTX) OutboxRepository {
    return &postgresStorage{o11y: o11y, db: db}
}
```

- Métodos chamam `r.db.QueryContext`/`ExecContext` direto — **sem** `BeginTx` interno.
- Lifecycle de TX fica com o caller (DispatcherJob via `uow.Do`).

**Factory** (`internal/platform/outbox/factory.go` — criar):

```go
type repositoryFactory struct { o11y observability.Observability }

func NewRepositoryFactory(o11y observability.Observability) OutboxRepositoryFactory {
    return &repositoryFactory{o11y: o11y}
}

func (r *repositoryFactory) OutboxRepository(db database.DBTX) OutboxRepository {
    return NewPostgresStorage(r.o11y, db)
}
```

**DispatcherJob refatorado:**

```go
type DispatcherJob struct {
    uow        uow.UnitOfWork[[]Row]
    factory    OutboxRepositoryFactory
    dispatcher events.Dispatcher
    cfg        OutboxConfig
    logger     observability.Logger
    rng        *rand.Rand
}

func (j *DispatcherJob) Run(ctx context.Context) error {
    processed, err := j.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) ([]Row, error) {
        storage := j.factory.OutboxRepository(tx)
        rows, err := storage.ClaimBatch(ctx, j.lockedBy(), j.cfg.BatchSize)
        if err != nil { return nil, err }
        for _, row := range rows {
            // … dispatch + storage.MarkProcessed(ctx, row.ID) — tudo no mesmo tx
        }
        return rows, nil
    })
    // … logging, metrics
    return err
}
```

**Reaper/Housekeeping:** padrão idêntico com `uow.NewVoid(dbManager, uow.WithObservability(o11y))`.

**`cmd/worker/worker.go`:**

```go
outboxFactory := outbox.NewRepositoryFactory(o11y)
dispatcherUoW := uow.New[[]outbox.Row](dbManager, uow.WithObservability(o11y))
reaperUoW     := uow.NewVoid(dbManager, uow.WithObservability(o11y))
housekeepUoW  := uow.NewVoid(dbManager, uow.WithObservability(o11y))

jobs := []worker.Job{
    outbox.NewDispatcherJob(dispatcherUoW, outboxFactory, eventsDispatcher, cfg.OutboxConfig, o11y.Logger(), rng),
    outbox.NewReaperJob(reaperUoW, outboxFactory, cfg.OutboxConfig, o11y.Logger()),
    outbox.NewHousekeepingJob(housekeepUoW, outboxFactory, cfg.OutboxConfig, o11y.Logger()),
}
```

### Compatibilidade

- O **contrato externo** do outbox (`Insert`, `ClaimBatch`, semântica de retry/poll) é preservado. Apenas a **composição** (DI, lifecycle de TX) muda.
- Migrations `000001_outbox_events.{up,down}.sql` ficam intactas.
- Testes existentes (`dispatcher_test.go`) atualizam mocks para `OutboxRepositoryFactory` + `uow.UnitOfWork[[]Row]`. `events/dispatcher_test.go` não precisa mudar.

### Validação

- `go test -race -count=1 ./internal/platform/outbox/...` verde.
- Smoke local: `cmd/worker` sobe, processa um evento de teste inserido manualmente, marca como processado — observação via logs estruturados.
- Sinal de drift: `grep -RInE "manager\\.Manager" internal/platform/outbox/` deve retornar 0 (storage agora vê `database.DBTX`, o Manager fica em `cmd/worker` apenas para construir UoWs).

---

**Fim da especificação técnica.**

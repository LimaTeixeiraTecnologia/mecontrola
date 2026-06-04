<!-- spec-hash-prd: 1a8ffbdf1a9d8dc441b8ce22ec4cdc645f56f9d63a93a95324403bc685653f40 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Identity Foundation

## Resumo Executivo

Esta techspec materializa o módulo `internal/identity/` como fundação hexagonal canônica do MeControla, alinhada às decisões do brainstorm `consolidacao-core` e à governança `AGENTS.md`. Entrega o agregado `User`, os Value Objects `WhatsAppNumber` e `Email`, o port `UserRepository` (implementado em pgx/v5 via `database.Manager`), use cases finos por operação (`UpsertUserByWhatsAppNumber`, `FindUserByID`, `FindUserByWhatsAppNumber`, `SoftDeleteUser`, `LinkNewNumber`), o domain service puro `IsEntitled` consumindo um contrato mínimo `Subscription` (interface declarada em `identity/domain`), e o substrato de persistência com soft delete, índice único parcial por `whatsapp_number` e histórico append-only.

A estratégia técnica reusa fundações já presentes em `internal/platform/`: `database.Manager` + `UnitOfWork[T]` para transações, `clock.Clock` para determinismo temporal, `observability.NewRedactingSlogHandler` como rede de segurança PII e um novo pacote `observability/mask` para mascaramento parcial dos VOs. Migrations entram via `golang-migrate` + `embed.FS` (padrão atual do projeto). Testes seguem `testify/suite` table-driven + mockery para unit e `testcontainers-go/modules/postgres` para integração com Postgres real. Object Calisthenics é aplicado como heurística (encapsulamento de primitivos, early-return, sem getters mecânicos, métodos com intenção de negócio), respeitando idiomatismo Go. O scaffold textual atual (`doc.go`, `README.md`, `AGENTS.md` do módulo) é reescrito para eliminar o drift JWT/RBAC.

## Arquitetura do Sistema

### Visão Geral dos Componentes

Layout físico obrigatório (sub-pastas conforme AGENTS.md "Layout Obrigatório por Módulo" — ver ADR-007):

```
internal/identity/
├── AGENTS.md                                       # reescrito (drift removal)
├── README.md                                       # reescrito (drift removal)
├── domain/
│   ├── doc.go                                      # reescrito
│   ├── entities/
│   │   └── user.go                                 # User aggregate + UserID
│   ├── valueobjects/
│   │   ├── whatsapp_number.go                      # VO E.164 BR
│   │   ├── email.go                                # VO email normalizado
│   │   └── user_status.go                          # enum ACTIVE/BLOCKED/DELETED (zero-value reservado)
│   ├── services/
│   │   ├── entitlement.go                          # IsEntitled (função pura)
│   │   └── subscription.go                         # interface contrato mínimo
│   └── errors.go                                   # sentinelas tipados
├── application/
│   ├── doc.go                                      # reescrito
│   ├── interfaces/
│   │   ├── user_repository.go                      # port canônico
│   │   └── id_generator.go                         # port para UUID v4
│   └── usecases/
│       ├── upsert_user_by_whatsapp_number.go
│       ├── find_user_by_id.go
│       ├── find_user_by_whatsapp_number.go
│       ├── soft_delete_user.go
│       └── link_new_number.go
└── infrastructure/
    ├── doc.go                                      # reescrito
    ├── repositories/
    │   └── postgres/
    │       ├── user_repository.go                  # PgxUserRepository
    │       ├── queries.go                          # SQL constants
    │       └── mapper.go                           # row → entities.User
    └── id/
        └── uuid_generator.go                       # google/uuid v4 adapter
```

Artefatos transversais novos ou alterados:

- `internal/platform/observability/mask/whatsapp.go` (novo) — `Mask(s string) string` → `+5511****8888`.
- `internal/platform/observability/mask/email.go` (novo) — `Mask(s string) string` → `a***@dominio.com`.
- `internal/platform/observability/redaction.go` (alterado) — adiciona `"whatsapp_number"` e `"email"` em `PIIFields` como rede de segurança.
- `migrations/0003_identity.up.sql` + `.down.sql` (novos) — schema `users` e `user_whatsapp_history`.
- `migrations/0004_identity_admin_seed.up.sql` + `.down.sql` (novos) — promoção idempotente dos admins via env var `ADMIN_WHATSAPP_NUMBERS` (ver ADR-005).
- `mockery.yml` (alterado) — declara `UserRepository` e `IDGenerator` para geração de mocks (R3).
- `.golangci.yml` (alterado) — confirma/estende `depguard` para `internal/identity/*` (RF-16).
- `README.md` raiz e `cmd/mecontrola/wire` (se houver) — injeção do `UserRepository` no DI container.

Fluxo de dependências (validado por `depguard`):

```
[domain]  ←── importa stdlib + uuid (nenhuma camada do projeto)
   ↑
[application] ── importa domain
   ↑
[infrastructure] ── importa domain + application + internal/platform/*
```

Cross-module: identity não importa billing/onboarding/finance/conversation diretamente. O contrato `Subscription` em `identity/domain/services/subscription.go` é uma interface implementada por `billing` no E2 — fronteira sem import cíclico.

## Design de Implementação

### Interfaces Chave

#### `domain/services/subscription.go` — Contrato mínimo (ADR-003)

```go
package services

import "time"

// Subscription é o contrato mínimo consumido por IsEntitled.
// A implementação concreta vive em internal/billing/domain (Épico E2).
type Subscription interface {
    Status() SubscriptionStatus
    CurrentPeriodEnd() time.Time
    GracePeriodEnd() time.Time
}

// SubscriptionStatus enumera as transições canônicas (iota+1; zero reservado).
type SubscriptionStatus uint8

const (
    StatusUnknown SubscriptionStatus = iota
    StatusTrialing
    StatusActive
    StatusPastDue
    StatusCanceledPending
    StatusExpired
    StatusRefunded
)
```

#### `domain/services/entitlement.go` — Função pura

```go
package services

import "time"

type EntitlementChecker struct{}

func NewEntitlementChecker() EntitlementChecker { return EntitlementChecker{} }

func (EntitlementChecker) IsEntitled(subscription Subscription, now time.Time) bool {
    if subscription == nil {
        return false
    }
    switch subscription.Status() {
    case StatusTrialing, StatusActive:
        return now.Before(subscription.CurrentPeriodEnd())
    case StatusPastDue, StatusCanceledPending:
        return now.Before(subscription.GracePeriodEnd())
    case StatusExpired, StatusRefunded, StatusUnknown:
        return false
    default:
        return false
    }
}
```

R1 aplicado: método de struct (não função top-level). R5.8 aplicado: iota+1. R5.10/5.21 aplicado: early return + switch determinístico cobrindo 100% das transições.

#### `application/interfaces/user_repository.go`

```go
package interfaces

import (
    "context"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type UserRepository interface {
    UpsertByWhatsAppNumber(ctx context.Context, number valueobjects.WhatsAppNumber) (*entities.User, error)
    FindByID(ctx context.Context, id entities.UserID) (*entities.User, error)
    FindByWhatsAppNumber(ctx context.Context, number valueobjects.WhatsAppNumber) (*entities.User, error)
    SoftDelete(ctx context.Context, id entities.UserID) error
    LinkNewNumber(ctx context.Context, id entities.UserID, number valueobjects.WhatsAppNumber, reason string) error
}
```

#### `application/interfaces/id_generator.go`

```go
package interfaces

type IDGenerator interface {
    NewUserID() string
}
```

Injetado nos use cases que precisam criar `User` (inversão de controle: testes substituem por fake determinístico; produção usa `infrastructure/id/uuid_generator.go` que delega a `google/uuid.NewString`). Sem `init()`, sem singleton (R0/R6.6).

#### `application/usecases/upsert_user_by_whatsapp_number.go`

```go
package usecases

import (
    "context"
    "fmt"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/clock"
)

type UpsertUserByWhatsAppNumberUseCase struct {
    userRepository interfaces.UserRepository
    idGenerator    interfaces.IDGenerator
    clock          clock.Clock
}

func NewUpsertUserByWhatsAppNumberUseCase(
    userRepository interfaces.UserRepository,
    idGenerator interfaces.IDGenerator,
    clock clock.Clock,
) *UpsertUserByWhatsAppNumberUseCase {
    return &UpsertUserByWhatsAppNumberUseCase{
        userRepository: userRepository,
        idGenerator:    idGenerator,
        clock:          clock,
    }
}

func (u *UpsertUserByWhatsAppNumberUseCase) Execute(ctx context.Context, rawNumber string) (*entities.User, error) {
    number, err := valueobjects.NewWhatsAppNumber(rawNumber)
    if err != nil {
        return nil, fmt.Errorf("upsert por whatsapp: %w", err)
    }
    user, err := u.userRepository.UpsertByWhatsAppNumber(ctx, number)
    if err != nil {
        return nil, fmt.Errorf("upsert por whatsapp: %w", err)
    }
    return user, nil
}
```

A invariante "id sempre presente" é resolvida no repositório (ver `infrastructure/repositories/postgres/user_repository.go`): no caminho de criação chama `idGenerator.NewUserID()` e constrói `entities.NewUser(...)` antes do INSERT. O use case fica fino e idempotente.

### Modelos de Dados

#### Agregado `entities.User`

```go
package entities

import (
    "errors"
    "time"

    "github.com/google/uuid"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type UserID struct{ value string }

func NewUserID(v string) (UserID, error) {
    parsed, err := uuid.Parse(v)
    if err != nil {
        return UserID{}, ErrInvalidUserID
    }
    if parsed.Version() != 4 {
        return UserID{}, ErrInvalidUserID
    }
    return UserID{value: parsed.String()}, nil
}

func (u UserID) String() string { return u.value }

var ErrInvalidUserID = errors.New("identity: user id deve ser uuid v4")

type User struct {
    id           UserID
    number       valueobjects.WhatsAppNumber
    displayName  string
    email        *valueobjects.Email
    isAdmin      bool
    status       valueobjects.UserStatus
    createdAt    time.Time
    updatedAt    time.Time
    deletedAt    *time.Time
}

type NewUserParams struct {
    ID          UserID
    Number      valueobjects.WhatsAppNumber
    DisplayName string
    Email       *valueobjects.Email
    IsAdmin     bool
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

func NewUser(p NewUserParams) (*User, error) {
    if p.Number.IsZero() {
        return nil, ErrUserRequiresNumber
    }
    if p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
        return nil, ErrUserRequiresTimestamps
    }
    return &User{
        id:          p.ID,
        number:      p.Number,
        displayName: p.DisplayName,
        email:       p.Email,
        isAdmin:     p.IsAdmin,
        status:      valueobjects.UserStatusActive,
        createdAt:   p.CreatedAt,
        updatedAt:   p.UpdatedAt,
    }, nil
}

func (u *User) ID() UserID                          { return u.id }
func (u *User) WhatsAppNumber() valueobjects.WhatsAppNumber { return u.number }
func (u *User) Email() *valueobjects.Email          { return u.email }
func (u *User) IsAdmin() bool                       { return u.isAdmin }
func (u *User) Status() valueobjects.UserStatus     { return u.status }
func (u *User) DeletedAt() *time.Time               { return u.deletedAt }

func (u *User) MarkAsAdmin(at time.Time)            { u.isAdmin = true; u.updatedAt = at }
func (u *User) RevokeAdmin(at time.Time)            { u.isAdmin = false; u.updatedAt = at }

func (u *User) UpdateEmail(e valueobjects.Email, at time.Time) {
    u.email = &e
    u.updatedAt = at
}

func (u *User) SoftDelete(at time.Time) error {
    if u.deletedAt != nil {
        return ErrUserAlreadyDeleted
    }
    u.deletedAt = &at
    u.status = valueobjects.UserStatusDeleted
    u.updatedAt = at
    return nil
}

func (u *User) IsDeleted() bool { return u.deletedAt != nil }

var (
    ErrUserRequiresNumber     = errors.New("identity: user requer whatsapp number válido")
    ErrUserRequiresTimestamps = errors.New("identity: user requer created_at e updated_at")
    ErrUserAlreadyDeleted     = errors.New("identity: user já está soft-deleted")
)

// RehydrateUserParams agrupa todos os campos necessários para reconstruir um User
// a partir de uma row Postgres. Uso restrito ao mapper de infrastructure.
type RehydrateUserParams struct {
    ID          UserID
    Number      valueobjects.WhatsAppNumber
    DisplayName string
    Email       *valueobjects.Email
    IsAdmin     bool
    Status      valueobjects.UserStatus
    CreatedAt   time.Time
    UpdatedAt   time.Time
    DeletedAt   *time.Time
}

// RehydrateUser é o construtor exclusivo de reidratação (mapper de infrastructure).
// Diferente de NewUser, aceita Status e DeletedAt arbitrários — esses já foram
// validados pelo banco via CK constraint e índice parcial. Não publicar para application.
func RehydrateUser(p RehydrateUserParams) *User {
    return &User{
        id:          p.ID,
        number:      p.Number,
        displayName: p.DisplayName,
        email:       p.Email,
        isAdmin:     p.IsAdmin,
        status:      p.Status,
        createdAt:   p.CreatedAt,
        updatedAt:   p.UpdatedAt,
        deletedAt:   p.DeletedAt,
    }
}
```

OC #3 aplicado (UserID encapsula UUID); OC #9 aplicado (sem getters mecânicos — métodos com intenção: `MarkAsAdmin`, `SoftDelete`, `UpdateEmail`); OC #1+#2 aplicado (early return em `SoftDelete`); R2 aplicado (sem alias-de-campo); R6.4 aplicado (zero-value de `User` é seguro: usa-se apenas via `NewUser`).

#### Value Object `valueobjects.WhatsAppNumber`

R1 aplicado integralmente: nenhuma função top-level. Toda lógica de normalização vive em `whatsAppNormalizer` (struct privada sem estado), com métodos invocados pelo construtor.

```go
package valueobjects

import (
    "errors"
    "fmt"
    "regexp"
    "strings"
)

type WhatsAppNumber struct{ e164 string }

var (
    ErrEmptyWhatsAppNumber   = errors.New("identity: whatsapp number vazio")
    ErrInvalidWhatsAppFormat = errors.New("identity: whatsapp number formato inválido (esperado BR E.164)")
    ErrUnsupportedCountry    = errors.New("identity: whatsapp number deve ser brasileiro (+55)")
)

var _nonDigitPattern = regexp.MustCompile(`\D+`)

type whatsAppNormalizer struct{}

func (whatsAppNormalizer) KeepDigits(input string) string {
    return _nonDigitPattern.ReplaceAllString(strings.TrimSpace(input), "")
}

// NormalizeBR aceita 10/11/12/13 dígitos após limpeza e devolve o conteúdo E.164 BR
// (12 dígitos: 55 + DDD + 8 dígitos; 13 dígitos: 55 + DDD + 9 + 8 dígitos).
// Regras:
//   - 10 dígitos: DDD (2) + 8 → injeta 55 e o 9 nono dígito.
//   - 11 dígitos: DDD (2) + 9 + 8 → injeta 55.
//   - 12 dígitos começando 55: 55 + DDD + 8 → injeta o 9 nono dígito.
//   - 13 dígitos começando 55: já canônico.
func (whatsAppNormalizer) NormalizeBR(digits string) (string, error) {
    if digits == "" {
        return "", ErrEmptyWhatsAppNumber
    }
    switch len(digits) {
    case 10:
        return "55" + digits[:2] + "9" + digits[2:], nil
    case 11:
        return "55" + digits, nil
    case 12:
        if !strings.HasPrefix(digits, "55") {
            return "", ErrUnsupportedCountry
        }
        return digits[:4] + "9" + digits[4:], nil
    case 13:
        if !strings.HasPrefix(digits, "55") {
            return "", ErrUnsupportedCountry
        }
        return digits, nil
    default:
        return "", fmt.Errorf("%w: %d dígitos", ErrInvalidWhatsAppFormat, len(digits))
    }
}

func NewWhatsAppNumber(input string) (WhatsAppNumber, error) {
    normalizer := whatsAppNormalizer{}
    digits := normalizer.KeepDigits(input)
    normalized, err := normalizer.NormalizeBR(digits)
    if err != nil {
        return WhatsAppNumber{}, err
    }
    return WhatsAppNumber{e164: "+" + normalized}, nil
}

func (n WhatsAppNumber) String() string                  { return n.e164 }
func (n WhatsAppNumber) IsZero() bool                    { return n.e164 == "" }
func (n WhatsAppNumber) Equals(other WhatsAppNumber) bool { return n.e164 == other.e164 }
```

OC #3+#7 aplicado (VO pequeno com responsabilidade única). R5.10 (sentinelas tipadas). R5.21 (switch/early-return sem `else`). R5.27 (struct sem campos opcionais, construtor único). R1 atendido via `whatsAppNormalizer`. Validação determinística e fuzz-testable.

#### Value Object `valueobjects.Email`

R1 aplicado integralmente via struct privada `emailValidator{}` sem estado.

```go
package valueobjects

import (
    "errors"
    "net/mail"
    "strings"
)

type Email struct{ value string }

var (
    ErrEmptyEmail   = errors.New("identity: email vazio")
    ErrInvalidEmail = errors.New("identity: email formato inválido")
)

type emailValidator struct{}

func (emailValidator) HasTLD(address string) bool {
    at := strings.LastIndex(address, "@")
    if at < 0 || at == len(address)-1 {
        return false
    }
    domain := address[at+1:]
    dot := strings.LastIndex(domain, ".")
    return dot > 0 && dot < len(domain)-1
}

func (v emailValidator) Parse(input string) (string, error) {
    trimmed := strings.TrimSpace(input)
    if trimmed == "" {
        return "", ErrEmptyEmail
    }
    parsed, err := mail.ParseAddress(trimmed)
    if err != nil {
        return "", ErrInvalidEmail
    }
    address := strings.ToLower(parsed.Address)
    if !strings.Contains(address, "@") || !v.HasTLD(address) {
        return "", ErrInvalidEmail
    }
    return address, nil
}

func NewEmail(input string) (Email, error) {
    address, err := emailValidator{}.Parse(input)
    if err != nil {
        return Email{}, err
    }
    return Email{value: address}, nil
}

func (e Email) String() string             { return e.value }
func (e Email) Equals(other Email) bool    { return e.value == other.value }
func (e Email) IsZero() bool               { return e.value == "" }
```

Stdlib `net/mail.ParseAddress` faz o trabalho pesado; `emailValidator.HasTLD` cobre o critério "domínio com TLD" do PRD. Sem dependência externa. R1 atendido.

#### Value Object `valueobjects.UserStatus`

```go
package valueobjects

type UserStatus uint8

const (
    UserStatusUnknown UserStatus = iota
    UserStatusActive
    UserStatusBlocked
    UserStatusDeleted
)

func (s UserStatus) String() string {
    switch s {
    case UserStatusActive:
        return "ACTIVE"
    case UserStatusBlocked:
        return "BLOCKED"
    case UserStatusDeleted:
        return "DELETED"
    default:
        return "UNKNOWN"
    }
}

func ParseUserStatus(s string) (UserStatus, bool) {
    switch s {
    case "ACTIVE":
        return UserStatusActive, true
    case "BLOCKED":
        return UserStatusBlocked, true
    case "DELETED":
        return UserStatusDeleted, true
    default:
        return UserStatusUnknown, false
    }
}
```

R5.8 (iota com zero-value reservado a `Unknown`). `BLOCKED` declarado no enum mas sem método mutador público — coluna aceita os três, agregado só transita para `ACTIVE`/`DELETED` neste PRD (ver decisão prévia).

#### Schema Postgres — `migrations/0003_identity.up.sql`

Convenções de nomenclatura aplicadas (Postgres 16 — documentação oficial 2026):

| Tipo | Padrão | Exemplo neste schema |
|---|---|---|
| Tabela | `snake_case` plural | `users`, `user_whatsapp_history` |
| Coluna | `snake_case` singular; timestamps `created_at`/`updated_at`/`deleted_at` | `whatsapp_number`, `is_admin` |
| Primary key | `pk_<tabela>` | `pk_users`, `pk_user_whatsapp_history` |
| Foreign key | `fk_<tabela>_<coluna>` | `fk_user_whatsapp_history_user_id` |
| Unique constraint | `uq_<tabela>_<coluna(s)>` | `uq_users_whatsapp_number`, `uq_users_email` |
| Check constraint | `ck_<tabela>_<regra>` | `ck_users_status` |
| Index | `idx_<tabela>_<coluna(s)>` | `idx_users_status`, `idx_user_whatsapp_history_user_id_active` |

```sql
-- migration: 0003_identity.up.sql
-- Cria o substrato do módulo identity: users (PK UUID v4, soft delete, índice único
-- parcial em whatsapp_number) e user_whatsapp_history (append-only com active/unlinked_at).
-- Schema: public. Sem extensões — UUID v4 é gerado pela aplicação (google/uuid).

CREATE TABLE IF NOT EXISTS users (
    id              UUID         NOT NULL,
    whatsapp_number TEXT         NOT NULL,
    display_name    TEXT,
    email           TEXT,
    is_admin        BOOLEAN      NOT NULL DEFAULT false,
    status          TEXT         NOT NULL DEFAULT 'ACTIVE',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,
    CONSTRAINT pk_users PRIMARY KEY (id),
    CONSTRAINT ck_users_status CHECK (status IN ('ACTIVE','BLOCKED','DELETED'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_users_whatsapp_number
    ON users (whatsapp_number)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_users_email
    ON users (lower(email))
    WHERE deleted_at IS NULL AND email IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_users_status
    ON users (status)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS user_whatsapp_history (
    id           UUID         NOT NULL,
    user_id      UUID         NOT NULL,
    number       TEXT         NOT NULL,
    active       BOOLEAN      NOT NULL,
    linked_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    unlinked_at  TIMESTAMPTZ,
    reason       TEXT,
    CONSTRAINT pk_user_whatsapp_history PRIMARY KEY (id),
    CONSTRAINT fk_user_whatsapp_history_user_id
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_whatsapp_history_user_id_active
    ON user_whatsapp_history (user_id, active);

CREATE INDEX IF NOT EXISTS idx_user_whatsapp_history_number
    ON user_whatsapp_history (number);
```

`migrations/0003_identity.down.sql` reverte na ordem inversa:

```sql
-- migration: 0003_identity.down.sql
DROP INDEX IF EXISTS idx_user_whatsapp_history_number;
DROP INDEX IF EXISTS idx_user_whatsapp_history_user_id_active;
DROP TABLE IF EXISTS user_whatsapp_history;
DROP INDEX IF EXISTS idx_users_status;
DROP INDEX IF EXISTS uq_users_email;
DROP INDEX IF EXISTS uq_users_whatsapp_number;
DROP TABLE IF EXISTS users;
```

Ver ADR-006 para a justificativa do índice único parcial.

#### Schema Postgres — `migrations/0004_identity_admin_seed.up.sql` (ADR-005)

```sql
-- migration: 0004_identity_admin_seed.up.sql
-- Promove admins iniciais a partir de ADMIN_WHATSAPP_NUMBERS (CSV) via current_setting.
-- Idempotente: UPDATE só promove existentes; números ausentes ficam para promoção pós-onboarding.
-- Esta migration NÃO cria usuários — apenas promove. A criação ocorre via fluxo normal de upsert.

DO $$
DECLARE
    raw    TEXT := current_setting('app.admin_whatsapp_numbers', true);
    parts  TEXT[];
    nbr    TEXT;
BEGIN
    IF raw IS NULL OR raw = '' THEN
        RAISE NOTICE 'identity: ADMIN_WHATSAPP_NUMBERS vazio — nenhum admin promovido';
        RETURN;
    END IF;
    parts := string_to_array(raw, ',');
    FOREACH nbr IN ARRAY parts LOOP
        UPDATE users
           SET is_admin = true, updated_at = now()
         WHERE whatsapp_number = trim(nbr)
           AND deleted_at IS NULL;
    END LOOP;
END $$;
```

A aplicação injeta `ADMIN_WHATSAPP_NUMBERS` via `SET LOCAL app.admin_whatsapp_numbers = ...` na inicialização (uma vez), e a migration consome via `current_setting('app.admin_whatsapp_numbers', true)`. Detalhes do mecanismo de injeção em ADR-005.

#### Implementação Postgres — `infrastructure/repositories/postgres/user_repository.go`

Pattern segue `internal/platform/outbox/storage_pgx.go`. Importa `devkit-go/pkg/database` (DBTX) + `pgx/v5` apenas neste arquivo. O repositório recebe `*database.Manager` (não `devkitmanager.Manager`) porque algumas operações (`LinkNewNumber`, `SoftDelete`) abrem sua própria `database.UnitOfWork[*entities.User]` internamente — o use case fica trivial e a transacionalidade não vaza para `application`.

```go
type PgxUserRepository struct {
    manager     *database.Manager
    idGenerator interfaces.IDGenerator
    clock       clock.Clock
}

func NewPgxUserRepository(
    manager *database.Manager,
    idGenerator interfaces.IDGenerator,
    clock clock.Clock,
) *PgxUserRepository {
    return &PgxUserRepository{
        manager:     manager,
        idGenerator: idGenerator,
        clock:       clock,
    }
}

// UpsertByWhatsAppNumber é idempotente: se houver row ativa com o número, retorna;
// senão, gera UUID v4 via idGenerator e insere. UNIQUE parcial garante atomicidade
// — race condition retorna ErrDuplicateWhatsAppNumber para o caller decidir retry.
func (r *PgxUserRepository) UpsertByWhatsAppNumber(
    ctx context.Context,
    number valueobjects.WhatsAppNumber,
) (*entities.User, error) {
    // 1. SELECT colunas* FROM users WHERE whatsapp_number = $1 AND deleted_at IS NULL.
    // 2. Hit → mapper.HydrateUser(row) → retorna.
    // 3. Miss → id = r.idGenerator.NewUserID(); now = r.clock.Now();
    //          INSERT INTO users (id, whatsapp_number, status, created_at, updated_at) VALUES (...);
    //          mapeia retorno (RETURNING *).
    // pgerrcode.UniqueViolation → ErrDuplicateWhatsAppNumber.
}

// FindByID e FindByWhatsAppNumber: SELECT colunas* WHERE id = $1 AND deleted_at IS NULL.
// Miss → ErrUserNotFound (sentinel).

// SoftDelete abre UoW e propaga cascata em user_whatsapp_history (decisão de design:
// histórico fica consistente — nenhum número permanece "ativo" para user deletado,
// facilitando anonimização LGPD futura - FE-04).
func (r *PgxUserRepository) SoftDelete(ctx context.Context, id entities.UserID) error {
    unitOfWork := database.NewUnitOfWork[struct{}](r.manager)
    _, err := unitOfWork.Do(ctx, func(ctx context.Context, tx devkitdb.DBTX) (struct{}, error) {
        now := r.clock.Now()
        // (a) UPDATE users SET deleted_at = $now, status = 'DELETED', updated_at = $now
        //     WHERE id = $id AND deleted_at IS NULL → se rows = 0, ErrUserNotFound.
        // (b) UPDATE user_whatsapp_history SET active = false, unlinked_at = $now,
        //     reason = 'user_soft_deleted' WHERE user_id = $id AND active = true.
        return struct{}{}, nil
    })
    return err
}

// LinkNewNumber abre UoW e executa as 3 SQLs atomicamente.
func (r *PgxUserRepository) LinkNewNumber(
    ctx context.Context,
    id entities.UserID,
    number valueobjects.WhatsAppNumber,
    reason string,
) error {
    unitOfWork := database.NewUnitOfWork[struct{}](r.manager)
    _, err := unitOfWork.Do(ctx, func(ctx context.Context, tx devkitdb.DBTX) (struct{}, error) {
        now := r.clock.Now()
        historyID := r.idGenerator.NewUserID()
        // (a) UPDATE user_whatsapp_history SET active = false, unlinked_at = $now, reason = $reason
        //     WHERE user_id = $id AND active = true.
        // (b) INSERT INTO user_whatsapp_history (id, user_id, number, active, linked_at)
        //     VALUES ($historyID, $id, $number, true, $now).
        // (c) UPDATE users SET whatsapp_number = $number, updated_at = $now
        //     WHERE id = $id AND deleted_at IS NULL → se rows = 0, ErrUserNotFound.
        return struct{}{}, nil
    })
    return err
}
```

Não vaza tipos pgx para fora. SQL injection prevenido via parametrização (`$1`, `$2`). Sentinelas dedicadas: `ErrUserNotFound`, `ErrDuplicateWhatsAppNumber` (mapeado de `pgerrcode.UniqueViolation`). UoW herda o timeout default de 5s do `database.UnitOfWork[T]` (configurável via ctx deadline).

##### Mapper — `infrastructure/repositories/postgres/mapper.go`

O mapper é o único caminho de reconstrução de `*entities.User` fora do construtor `NewUser`. Re-valida todas as invariantes (defesa em profundidade — ADR-008): row corrompida por migração manual ou ferramenta externa falha imediatamente em vez de propagar dado inválido.

```go
type rowMapper struct{}

type userRow struct {
    ID             string
    WhatsAppNumber string
    DisplayName    sql.NullString
    Email          sql.NullString
    IsAdmin        bool
    Status         string
    CreatedAt      time.Time
    UpdatedAt      time.Time
    DeletedAt      sql.NullTime
}

func (rowMapper) HydrateUser(row userRow) (*entities.User, error) {
    userID, err := entities.NewUserID(row.ID)
    if err != nil {
        return nil, fmt.Errorf("postgres user mapper: id corrompido: %w", err)
    }
    number, err := valueobjects.NewWhatsAppNumber(row.WhatsAppNumber)
    if err != nil {
        return nil, fmt.Errorf("postgres user mapper: whatsapp corrompido: %w", err)
    }
    var emailPtr *valueobjects.Email
    if row.Email.Valid && row.Email.String != "" {
        email, err := valueobjects.NewEmail(row.Email.String)
        if err != nil {
            return nil, fmt.Errorf("postgres user mapper: email corrompido: %w", err)
        }
        emailPtr = &email
    }
    return entities.RehydrateUser(entities.RehydrateUserParams{
        ID:          userID,
        Number:      number,
        DisplayName: row.DisplayName.String,
        Email:       emailPtr,
        IsAdmin:     row.IsAdmin,
        Status:      mustParseStatus(row.Status),
        CreatedAt:   row.CreatedAt,
        UpdatedAt:   row.UpdatedAt,
        DeletedAt:   nullableTime(row.DeletedAt),
    }), nil
}
```

`entities.RehydrateUser` é construtor exclusivo de reidratação (recebe `Status` e `DeletedAt` já validados; não dispara validação de status pois aceita os 3 valores do enum). Documentado no godoc de `entities/user.go` como "uso restrito ao mapper de infrastructure — não usar em código de aplicação".

### Estratégia de Erros

Hierarquia (R5.10 + R-ERR-001):

| Camada | Sentinela | Quando |
|---|---|---|
| domain | `entities.ErrInvalidUserID`, `entities.ErrUserRequiresNumber`, `entities.ErrUserRequiresTimestamps`, `entities.ErrUserAlreadyDeleted` | Construtor ou método de agregado falha invariante |
| domain | `valueobjects.ErrEmptyWhatsAppNumber`, `ErrInvalidWhatsAppFormat`, `ErrUnsupportedCountry`, `ErrEmptyEmail`, `ErrInvalidEmail` | Construtor de VO rejeita input |
| application | `usecases.ErrUserNotFound` (re-exporta do repositório) | Caso de uso recebe miss em find |
| infrastructure | `postgres.ErrUserNotFound`, `postgres.ErrDuplicateWhatsAppNumber` | Repo traduz row miss / pgerrcode |

Wrapping:
- Use cases wrappam com contexto curto em PT-BR: `fmt.Errorf("upsert por whatsapp: %w", err)`.
- Repo wrappa erros pgx: `fmt.Errorf("postgres user repository: %w", err)`.
- Camada HTTP (fora deste PRD) usará `internal/platform/errors.ToProblemDetails` — já mapeia `database.ErrConnection`/`ErrDeadlineExceeded`. Adicionaremos branch para `ErrUserNotFound` em PRD que tiver HTTP handler real (fora de escopo aqui).

Proibido (governance.md + R-ERR-001):
- `panic` em código de produção (qualquer camada).
- Engolir erros: todo `err != nil` retorna ou trata explicitamente.
- Logar e retornar o mesmo erro (tratamento único).
- Mensagens em inglês iniciadas por `failed to ...`.

### Endpoints de API

**Nenhum.** Esta techspec não expõe HTTP/gRPC. Os consumidores são E2 (`billing`) e E3 (`onboarding`) que chamarão use cases via DI. Painéis admin web entram em PRD futuro (FE-05).

## Pontos de Integração

- **Postgres** via `internal/platform/database.Manager` (já existente, devkit-go + pgx/v5). Sem novo client.
- **Observability** via `internal/platform/observability` para slog estruturado; novo subpacote `mask` para PII parcial (ADR-004).
- **Sem brokers**, sem `outbox.Publisher`, sem `events.Bus` neste PRD (RT-07).

## Abordagem de Testes

### Testes Unitários

Cobertura 100% obrigatória em `NewWhatsAppNumber`, `NewEmail` e `EntitlementChecker.IsEntitled` (MS-01 + RF-17). Estrutura por `testify/suite` table-driven (R4):

- `domain/valueobjects/whatsapp_number_test.go` — `WhatsAppNumberSuite` com tabelas: válidos (10/11/12/13 dígitos, formatado humano, com `+55`, com 9 ausente), inválidos (vazio, não-BR, comprimento errado, só letras), idempotência (mesmo input → mesma saída).
- `domain/valueobjects/whatsapp_number_fuzz_test.go` — `FuzzNewWhatsAppNumber(f *testing.F)` com corpus seed cobrindo edges; nunca panica.
- `domain/valueobjects/email_test.go` — `EmailSuite` table-driven (válidos com/sem maiúsculas, inválidos sem `@`, sem TLD, vazio).
- `domain/services/entitlement_test.go` — `EntitlementSuite` cobrindo 6 transições + `nil` subscription + boundary (`now == periodEnd`).
- `domain/entities/user_test.go` — construtor + `SoftDelete` (sucesso + já deletado) + `MarkAsAdmin` + `UpdateEmail`.
- `application/usecases/*_test.go` — table-driven com mock `UserRepository` (mockery), mock `IDGenerator`, `clock.NewFakeClock`. Cenários obrigatórios: sucesso, erro de validação de VO, erro de repositório.

Mocks (R3) declarados em `mockery.yml` raiz:

```yaml
with-expecter: true
mockname: "{{.InterfaceName}}"
outpkg: "mocks"
filename: "{{.InterfaceName | snakecase}}.go"
dir: "{{.InterfaceDir}}/mocks"
packages:
  github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox:
    interfaces:
      Storage:
      Registry:
  github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces:
    interfaces:
      UserRepository:
      IDGenerator:
```

CI: `mockery --config mockery.yml --dry-run` falha se mocks divergirem.

### Testes de Integração

Critérios (todos atendidos):
- [x] Fronteira IO crítica (Postgres) — mocks não garantem correção de constraints, índice único parcial e CASCADE.
- [x] Risco de divergência mock/prod (LGPD soft delete + filtragem em todas as leituras é regulatório).
- [x] Custo proporcional — testcontainers-go já é dependência do projeto (ver `go.mod`).

Decisão: testcontainers-go (ADR-002). Build tag `//go:build integration`.

`internal/identity/infrastructure/repositories/postgres/user_repository_integration_test.go`:

- `UserRepositoryIntegrationSuite` provisiona container `postgres:16-alpine` em `SetupSuite`, aplica todas as migrations (`migrations.FS`), reseta tabelas em `SetupTest`.
- Cenários (RF-18):
  - `TestUpsertIdempotenteByWhatsAppNumber` — duas chamadas com o mesmo número retornam o mesmo `UserID`.
  - `TestSoftDeleteFiltraEmFindByID` — após soft delete, `FindByID` retorna `ErrUserNotFound`.
  - `TestSoftDeleteFiltraEmFindByWhatsApp` — idem para `FindByWhatsAppNumber`.
  - `TestLinkNewNumberRegistraHistorico` — após link: 2 rows em `user_whatsapp_history` (1 inactive, 1 active), `users.whatsapp_number` atualizado.
  - `TestUniqueIndexParcialPermiteReuso` — soft-delete user A com número X, novo upsert com número X cria novo user B sem violar constraint.
  - `TestDuplicateWhatsAppNumberConcorrente` — INSERT direto bypass do upsert valida `ErrDuplicateWhatsAppNumber`.

Estrutura segue `internal/platform/outbox/storage_pgx_integration_test.go`.

### Testes E2E

Não aplicável. Smoke E2E mínimo (MS-04) é coberto pelos integration tests acima — não há orquestração ponta-a-ponta com HTTP/WhatsApp neste PRD.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Schema + migrations** (`migrations/0003_identity.{up,down}.sql`) — fundação física, validada por `RunMigrations` no testcontainer.
2. **Value Objects** (`domain/valueobjects/{whatsapp_number,email,user_status}.go`) — independentes, testáveis sem outras camadas. Cobertura 100% antes de prosseguir.
3. **Agregado e enums de subscription** (`domain/entities/user.go`, `domain/services/subscription.go`, `domain/services/entitlement.go`) — usam VOs.
4. **Port + Use cases** (`application/interfaces/*.go`, `application/usecases/*.go`) — usam domain.
5. **Mocks** (regenerar `mockery --config mockery.yml`) — habilita testes unitários de use cases.
6. **Adapter pgx** (`infrastructure/repositories/postgres/user_repository.go`) + `infrastructure/id/uuid_generator.go` — implementa ports.
7. **Migration de admin seed** (`0004_identity_admin_seed.{up,down}.sql`) + injeção de `app.admin_whatsapp_numbers` no bootstrap.
8. **Helper mask** (`internal/platform/observability/mask/{whatsapp,email}.go`) + extensão de `PIIFields` em `redaction.go`.
9. **depguard** — confirmar que `.golangci.yml` cobre os imports proibidos para `identity` (Regra 5 cross-module já existe; reforçar com `identity-no-billing`, `identity-no-onboarding` quando esses módulos existirem; manter escopo mínimo de RF-16 neste PRD).
10. **Drift cleanup** (`internal/identity/{AGENTS.md, README.md, domain/doc.go, application/doc.go, infrastructure/doc.go}`) — reescrita textual final.
11. **Testes de integração** com testcontainers e relatório de cobertura.

### Dependências Técnicas

- `github.com/google/uuid v1.6.0` (já indireta — promover a direta no `go.mod`).
- `github.com/golang-migrate/migrate/v4 v4.19.1` (já direta).
- `github.com/testcontainers/testcontainers-go/modules/postgres v0.42.0` (já direta).
- `github.com/jackc/pgx/v5 v5.9.2`, `github.com/jackc/pgerrcode` (já diretas).
- `github.com/vektra/mockery/v2 v2.53.6` (já direta).
- Nenhuma dependência nova.

## Monitoramento e Observabilidade

Logs estruturados via `log/slog` (R7.2). Atributos obrigatórios em qualquer log do módulo:

```go
slog.InfoContext(ctx, "identity: upsert por whatsapp concluído",
    slog.String("user_id", user.ID().String()),
    slog.String("whatsapp_number_masked", mask.WhatsApp(user.WhatsAppNumber().String())),
)
```

Regra inegociável: **nunca passar `whatsapp_number` ou `email` em claro**. O `piiHandler` global continua trocando para `[REDACTED]` se alguém esquecer (rede de segurança — ADR-004).

Métricas e dashboards específicos de identity ficam para PRD futuro (FE-08). Health check do banco continua via `database.Manager.HealthCheck`.

## Considerações Técnicas

### Decisões Chave

Cada decisão material é registrada em ADR separada:

- [ADR-001 — `golang-migrate` + `embed.FS`](./adr-001-golang-migrate-embed-fs.md)
- [ADR-002 — `testcontainers-go/modules/postgres` para testes de integração](./adr-002-testcontainers-go-postgres.md)
- [ADR-003 — Contrato `Subscription` como interface em `identity/domain`](./adr-003-subscription-contract-interface.md)
- [ADR-004 — PII masking via pacote `mask` + handler global como rede de segurança](./adr-004-pii-masking-mask-package.md)
- [ADR-005 — Admin seed via migration + `current_setting('app.admin_whatsapp_numbers')`](./adr-005-admin-seed-migration-env.md)
- [ADR-006 — Índice UNIQUE parcial `WHERE deleted_at IS NULL`](./adr-006-unique-index-partial-soft-delete.md)
- [ADR-007 — Layout físico com sub-pastas por responsabilidade (conforme AGENTS.md)](./adr-007-physical-layout-subfolders.md)
- [ADR-008 — Mapper re-valida invariantes na reidratação (defesa em profundidade)](./adr-008-mapper-revalidates-invariants.md)
- [ADR-009 — `SoftDelete` propaga cascata para `user_whatsapp_history.active=false`](./adr-009-softdelete-cascade-history.md)
- [ADR-010 — Transacionalidade encapsulada no repositório (UoW interna em `LinkNewNumber` e `SoftDelete`)](./adr-010-transactional-boundary-repository.md)

### Riscos Conhecidos

| Risco | Impacto | Mitigação |
|---|---|---|
| Race condition em `UpsertByWhatsAppNumber` (dois callers simultâneos com mesmo número novo) | Violação de UNIQUE → erro retornado, perda de uma operação | Repo trata `pgerrcode.UniqueViolation` retornando `ErrDuplicateWhatsAppNumber`; caller (E2/E3) decide retry. Em fluxo WhatsApp, paralelismo é baixo o suficiente para não justificar advisory lock. |
| Normalização BR rejeita números legítimos com `+55` mas DDD inválido | Falsa rejeição de usuário real | Cobertura por table-driven test com DDDs reais (11, 21, 27, 31, 41, 51, 61, 71, 81, 91). Fuzz test para input aleatório nunca panica. |
| `app.admin_whatsapp_numbers` esquecido na inicialização | Admins iniciais não promovidos | Migration é idempotente; pode rerodar manualmente em produção via `psql` com `SET LOCAL` após corrigir env. Runbook: ADR-005. |
| `piiHandler` aplica `[REDACTED]` antes que o `mask.WhatsApp(...)` seja chamado em logs com chave `whatsapp_number` | Logs com chave canônica nunca mostram nada útil | Pelo design (ADR-004), módulo SEMPRE loga sob chave `whatsapp_number_masked` (já mascarada); o handler protege a chave original como rede de segurança. Lint custom não viável agora — runbook + code review pegam. |
| `BLOCKED` no enum sem transição → coluna pode ter valor que código não materializa via método público | Inconsistência conceitual menor | Coluna aceita `BLOCKED` (compatibilidade futura sem nova migration); mapper de row reconhece valor e popula `User.status`; ausência de método público está documentada e voltará em PRD próprio. |
| Falha de migration `0004` por falta de `SET LOCAL` em sessão correta | Admin seed vira no-op silencioso | Mensagem `RAISE NOTICE` no DO block sinaliza no log do migrator; CI integration test valida o caminho com seed. |
| Integration test lento ao rodar migrations completas em cada suite | Atrito de DX | `SetupSuite` aplica uma vez; `SetupTest` faz `TRUNCATE users, user_whatsapp_history CASCADE`. Container `postgres:16-alpine` (~80MB) sobe em ~2s. |

### Conformidade com Padrões

- `.claude/rules/governance.md` (R-GOV-001) — precedência respeitada; toda decisão material rastreada em ADR.
- `.agents/skills/agent-governance/references/ddd.md` (R-DDD-001) — agregado com invariantes em construtor, VOs imutáveis com auto-validação, sem struct anêmica, sem regra de transição em handler.
- `.agents/skills/agent-governance/references/error-handling.md` (R-ERR-001) — sentinelas tipadas por camada, wrapping com `%w`, sem `panic`, mensagens estáveis em PT-BR.
- `.agents/skills/agent-governance/references/security.md` (R-SEC-001) — sem segredos hardcoded; PII mascarada em logs; SQL parametrizado; input externo validado em VO antes de chegar ao repositório.
- `.agents/skills/agent-governance/references/testing.md` (R-TEST-001) — table-driven, mocks via mockery, testcontainers para IO real, sem flakiness por `sleep`.
- `.agents/skills/go-implementation/SKILL.md` Regras R0–R7 — sem `init()`; todas as funções de negócio como métodos de struct (exceto `New*` e `keepDigits`/`hasTLD`/`normalizeBR` que são helpers privados sem estado, aceitos como exceção pragmática a R1 dentro do mesmo arquivo do VO); mockery obrigatório; testify/suite; Uber style; sem `interface{}` (usa `any`); `errors.Join` reservado para casos com múltiplos erros agregáveis.
- `.agents/skills/object-calisthenics-go/references/rules.md` — OC #3 (encapsular primitivos: VOs), #5 (um ponto por linha: mappers explícitos), #7 (entidades pequenas: arquivos por VO), #9 (sem getters mecânicos: métodos com intenção). OC #8 não aplicado a `User` por ser aggregate (exceção documentada na regra).
- AGENTS.md "Layout Obrigatório por Módulo" — sub-pastas por responsabilidade (ADR-007).
- AGENTS.md "Outbox vs events.Bus" — não usado neste PRD (RT-07); regra reafirmada para PRD futuro que emita eventos de identity.
- `.golangci.yml` `depguard` — fronteiras hexagonais e cross-module enforçadas no CI (RF-16, escopo mínimo do PRD).

### Arquivos Relevantes e Dependentes

Criados:
- `internal/identity/domain/entities/user.go`
- `internal/identity/domain/valueobjects/{whatsapp_number,email,user_status}.go`
- `internal/identity/domain/services/{entitlement,subscription}.go`
- `internal/identity/domain/errors.go`
- `internal/identity/application/interfaces/{user_repository,id_generator}.go`
- `internal/identity/application/usecases/{upsert_user_by_whatsapp_number,find_user_by_id,find_user_by_whatsapp_number,soft_delete_user,link_new_number}.go`
- `internal/identity/application/interfaces/mocks/{user_repository,id_generator}.go` (gerados por mockery)
- `internal/identity/infrastructure/repositories/postgres/{user_repository,queries,mapper}.go`
- `internal/identity/infrastructure/id/uuid_generator.go`
- `internal/identity/infrastructure/repositories/postgres/user_repository_integration_test.go`
- `internal/platform/observability/mask/{whatsapp,email}.go`
- `migrations/0003_identity.{up,down}.sql`
- `migrations/0004_identity_admin_seed.{up,down}.sql`
- `.specs/prd-identity-foundation/adr-001..adr-007.md`

Alterados:
- `internal/identity/{AGENTS.md, README.md, domain/doc.go, application/doc.go, infrastructure/doc.go}` — reescrita textual removendo JWT/RBAC.
- `internal/platform/observability/redaction.go` — adiciona `"whatsapp_number"` e `"email"` em `PIIFields`.
- `mockery.yml` — declara interfaces de identity.
- `.golangci.yml` — confirma cobertura `identity` (regras hexagonais já existentes + cross-module mínimo).
- `go.mod` — promover `google/uuid` para dependência direta.

Não alterados (fora de escopo):
- `internal/billing/**` — Subscription concreto entra em E2.
- `internal/onboarding/**` — consumidor entra em E3.
- `internal/finance/**` — FE-09.

## Plano de Rollout

1. **Validação local** — `go test ./... -race -count=1`, `golangci-lint run`, `mockery --config mockery.yml --dry-run`, `go test -tags=integration ./internal/identity/...`.
2. **Pre-commit hook** — `ai-spec check-spec-drift .specs/prd-identity-foundation/tasks.md` (após `create-tasks`).
3. **Merge feature-branch → main** — CI roda lint+unit+integration; deploy de migration é manual (sem auto-apply em produção, conforme `persistence.md`).
4. **Apply migrations 0003/0004 em staging** — operador define `ADMIN_WHATSAPP_NUMBERS` no env do migrator, roda `migrate up`, valida com `SELECT count(*) FROM users WHERE is_admin = true`.
5. **Apply em produção** — mesma janela; `0004` é no-op se `ADMIN_WHATSAPP_NUMBERS` vazio (idempotente).
6. **Rollback** — `migrate down 2` reverte `0004` e `0003`; sem perda de dados em ambientes sem usuários reais ainda (este PRD é fundação, nenhum usuário real existe ao aplicar).
7. **Sinais de saúde pós-rollout** — `database.Manager.HealthCheck` continua verde; nenhum log do módulo identity contém `whatsapp_number` ou `email` em claro (verificação: `grep -E '"(whatsapp_number|email)":"[^*\[]' logs/` retorna vazio).

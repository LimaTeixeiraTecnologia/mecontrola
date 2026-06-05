# Runbook — Fluxo canônico: Handler → Use Case → UoW → Repository Factory → Repository

> **Escopo:** padrão obrigatório para todo módulo (`internal/<modulo>/`) que execute IO em Postgres.
> **Aderência:** `AGENTS.md` (Padrão Obrigatório de Módulo, R0–R7), `CLAUDE.md`, `.claude/rules/governance.md`.
> **Stack:** devkit-go v0.4.0 (`database`, `database/manager`, `observability`, `http_server/chi_server`).
> **Não cobre:** consumers de mensageria, jobs do worker, producers do outbox — esses seguem variantes do mesmo padrão UoW e ficam em runbooks dedicados.

---

## 1. Visão geral do fluxo

```
HTTP request
   │
   ▼
infrastructure/http/server/handlers/*.go        ← decodifica DTO, valida HTTP, chama UC
   │
   ▼
application/usecases/*.go                       ← orquestra regras, abre TX via uow.Do
   │     │
   │     ▼
   │  devkitUoW.UnitOfWork[T].Do(ctx, fn, …)    ← BeginTx + Commit/Rollback (devkit-go)
   │     │
   │     ▼
   │  factory.<Repo>Repository(tx)              ← devolve repo amarrado ao tx
   │     │
   │     ▼
   │  repo.Method(ctx, args...)                 ← Exec/Query contra o tx
   │     │
   ▼     ▼
   resposta HTTP (Output DTO → JSON)
```

> **UoW é da devkit-go v0.4.0** (`github.com/JailtonJunior94/devkit-go/pkg/database/uow`).
> **Proibido reimplementar** localmente em `internal/platform/uow/` ou similar.

**Por que esse formato:**

- **UoW** abstrai `BeginTx`/`Commit`/`Rollback`. O UC não conhece o `manager.Manager`; só pede "execute essa unidade de trabalho atomicamente". A devkit já abre span (`db.{driver}.tx`), emite métricas (`database.tx.duration_ms`, `committed`/`rolledback`), faz rollback em panic, bloqueia reentrada por goroutine ID.
- **RepositoryFactory** entrega instâncias de repositório amarradas ao `tx` recebido pelo callback. Permite **orquestrar múltiplos repositórios na mesma transação** sem cada um abrir TX por conta própria.
- **Repository** recebe `database.DBTX` (pool ou tx) no construtor. Métodos não recebem `tx` explícito — a instância já carrega a `DBTX` correta.

---

## 2. Estrutura de pastas alvo (módulo `identity` como exemplo)

```
internal/identity/
├── module.go
├── doc.go
├── application/
│   ├── errors.go
│   ├── interfaces/
│   │   ├── repository_factory.go              ← port da factory
│   │   └── user_repository.go                 ← port do repositório
│   ├── usecases/
│   │   └── upsert_user_by_whatsapp.go         ← orquestra com uow.Do
│   └── dtos/
│       ├── input/upsert_user_by_whatsapp.go
│       └── output/upsert_user_by_whatsapp.go
├── domain/
│   ├── entities/user.go
│   └── valueobjects/{whatsapp_number,email}.go
└── infrastructure/
    ├── http/
    │   └── server/
    │       ├── router.go                      ← UserRouter (Register chi.Router)
    │       └── handlers/
    │           └── upsert_user_by_whatsapp_handler.go
    └── repositories/
        ├── factory.go                          ← impl RepositoryFactory
        └── postgres/
            └── user_repository.go              ← impl Postgres do port
```

---

## 3. Dependência: `devkit-go/pkg/database/uow`

**Pacote:** `github.com/JailtonJunior94/devkit-go/pkg/database/uow` (v0.4.0).
**Local em nosso working tree:** **não existe** — é dependência externa via `go.mod`. Não criar `internal/platform/uow/` nem similar.

### Assinatura pública (do pacote)

```go
package uow // devkit-go

type UnitOfWork[T any] interface {
    Do(
        ctx context.Context,
        fn func(ctx context.Context, tx database.DBTX) (T, error),
        opts ...Option,
    ) (T, error)
}

func New[T any](mgr manager.Manager, opts ...Option) UnitOfWork[T]
func NewVoid(mgr manager.Manager, opts ...Option) UnitOfWork[struct{}]

// Options (passáveis no construtor New/NewVoid ou por chamada de Do):
//   WithObservability(observability.Observability) — habilita tracer + métricas + logger
//   WithIsolation(sql.IsolationLevel)              — default: LevelDefault
//   WithReadOnly(bool)                             — default: false
```

### Garantias do contrato (já implementadas pela devkit)

- **Span automático** `db.{driver}.tx` aberto em `Do`; encerra com tag de outcome (`committed`, `rolled_back`, `panic`, `error`).
- **Métricas** internas: histograma `database.tx.duration_ms` + contadores `database.tx.committed` / `database.tx.rolledback`.
- **Propagação dupla** do `tx`: o callback recebe `tx database.DBTX` direto **e** o ctx é enriquecido com `database.WithTx(ctx, tx)` — repositórios que ainda leem via `database.FromContext(ctx)` continuam funcionando.
- **Panic-safe**: `recover` interno, rollback com contexto fresh (timeout 5s), re-panic do erro original.
- **Rollback automático** em erro do callback; falha de `Commit` é wrappeada como `fmt.Errorf("uow: commit: %w", err)`.
- **Reentrada bloqueada** na mesma goroutine via `petermattis/goid` — chamadas aninhadas de `Do` na mesma goroutine falham cedo.

### Política

- **Proibido reimplementar UoW** em qualquer pacote do working tree (`internal/platform/uow/`, `internal/<m>/infrastructure/uow/`, etc.). Consumir sempre a devkit.
- **Proibido criar wrapper "para adicionar tracer/log"** — a devkit já faz, e duplicar abre risco de span/métrica duplicada.
- **Use cases tipam o `T`** pelo retorno natural da operação (e.g., `entities.User` para upsert, `[]outbox.Row` para claim batch). UCs sem retorno usam `uow.UnitOfWork[struct{}]` injetado via `uow.NewVoid(mgr, …)`.
- Sem `panic` em produção (R5.12), sem `init()` (R0), sem `Clock` abstrato (R6.7) — `time.Now().UTC()` resolvido dentro do callback ou do UC.

---

## 4. Aplicação — Ports

### 4.1. `RepositoryFactory` (port)

**Local:** `internal/identity/application/interfaces/repository_factory.go`

```go
package interfaces

import "github.com/JailtonJunior94/devkit-go/pkg/database"

// RepositoryFactory devolve instâncias de repositórios amarradas a uma DBTX
// (pool fora de TX, ou tx vindo do callback de UnitOfWork.Do).
// Permite orquestrar 2+ repos na mesma transação dentro de um único use case.
type RepositoryFactory interface {
	UserRepository(db database.DBTX) UserRepository
}
```

### 4.2. `UserRepository` (port)

**Local:** `internal/identity/application/interfaces/user_repository.go`

```go
package interfaces

import (
	"context"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type WhatsAppHistoryEntry struct {
	ID         string
	Number     string
	Active     bool
	LinkedAt   time.Time
	UnlinkedAt time.Time
	Reason     string
}

type UserRepository interface {
	UpsertByWhatsAppNumber(ctx context.Context, u entities.User, now time.Time) (entities.User, error)
	FindByID(ctx context.Context, id string) (entities.User, error)
	FindByWhatsAppNumber(ctx context.Context, number valueobjects.WhatsAppNumber) (entities.User, error)
	MarkDeleted(ctx context.Context, id string, now time.Time) error
	AppendWhatsAppHistory(ctx context.Context, userID string, entry WhatsAppHistoryEntry) error
}
```

**Observe:** nenhum método recebe `tx database.DBTX` na assinatura — a instância concreta já carrega a `DBTX` correta via construtor.

### 4.3. Erros tipados

**Local:** `internal/identity/application/errors.go`

```go
package application

import "errors"

var (
	ErrUserNotFound         = errors.New("identity: user not found")
	ErrWhatsAppNumberInUse  = errors.New("identity: whatsapp number already in use")
	ErrEmailInUse           = errors.New("identity: email already in use")
)
```

### 4.4. DTOs do use case

**Local:** `internal/identity/application/dtos/input/upsert_user_by_whatsapp.go`

```go
package input

import "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"

type UpsertUserByWhatsApp struct {
	WhatsApp    valueobjects.WhatsAppNumber
	Email       valueobjects.Email
	DisplayName string
}
```

**Local:** `internal/identity/application/dtos/output/upsert_user_by_whatsapp.go`

```go
package output

import "time"

type UpsertUserByWhatsApp struct {
	ID          string
	WhatsApp    string
	Email       string
	DisplayName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
```

---

## 5. Aplicação — Use Case

**Local:** `internal/identity/application/usecases/upsert_user_by_whatsapp.go`

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
	uow     uow.UnitOfWork[entities.User]   // tipado por T = retorno do callback
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
			// entities.New é autossuficiente: gera o UUID via entities.NewID()
			// e resolve created_at/updated_at com time.Now().UTC() inline no construtor.
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

		existing.SetDisplayNameIfEmpty(in.DisplayName) // first-write-wins no agregado (DDD)
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

	return output.UpsertUserByWhatsApp{
		ID:          result.ID(),
		WhatsApp:    result.WhatsApp().String(),
		Email:       result.Email().String(),
		DisplayName: result.DisplayName(),
		CreatedAt:   result.CreatedAt(),
		UpdatedAt:   result.UpdatedAt(),
	}, nil
}
```

**Anatomia:**

1. `u.o11y.Tracer().Start(ctx, ...)` abre span da operação (o span `db.{driver}.tx` do UoW aninha por baixo automaticamente).
2. **`time.Now().UTC()` chamado inline no call-site** (parâmetro do método) — **proibido** capturar em variável intermediária (`now := ...`). Cada uso resolve o seu instante; sem clock abstrato (R6.7).
3. **Domínio autossuficiente** para geração de ID: `entities.New(in.WhatsApp, opts...)` chama internamente `entities.NewID()` (ver §5.1). **Proibido injetar `IDGenerator`** em qualquer camada — sem campo `idGen` no UC, sem `id.Generator` no module, sem `entities.IDGenerator` como interface.
4. `u.uow.Do(ctx, func(ctx, tx) (entities.User, error) { ... })` — **único ponto** que conhece "transação". O callback retorna `(T, error)` — sem closure sobre `var result`.
5. Dentro do callback, `u.factory.UserRepository(tx)` devolve instância amarrada ao `tx`. Se este UC precisasse orquestrar `UserRepository` + `WhatsAppHistoryRepository` + `OutboxRepository` na mesma TX, bastaria chamar `factory.<Outro>(tx)` mais vezes — todos compartilham o mesmo `tx`.
6. Regras de domínio (`SetDisplayNameIfEmpty`) vivem no agregado (`entities.User`), não no repo nem no SQL.
7. Erro é wrappeado com prefixo da operação (R5.10) e logado **uma vez** no nível externo do UC.

> **UCs sem retorno** (e.g., `MarkUserDeleted`) declaram `uow uow.UnitOfWork[struct{}]` e o módulo injeta via `uow.NewVoid(mgr, uow.WithObservability(o11y))`. O callback retorna `(struct{}{}, err)`.
>
> **UCs read-only** podem passar `uow.WithReadOnly(true)` na chamada de `Do(...)` (hint para réplicas/snapshot). UCs com isolation level específico passam `uow.WithIsolation(sql.LevelSerializable)` por chamada. Defaults bastam para o MVP.

### 5.1. Geração de ID no domínio (padrão inegociável)

**Local:** `internal/identity/domain/entities/id.go`

```go
package entities

import "github.com/google/uuid"

// NewID gera um identificador UUID v4 para entidades do agregado de identidade.
// Função de domínio — sem parâmetros, sem fonte injetável, sem DI.
// Construtores das entidades (entities.New, entities.NewWhatsAppHistoryEntry, …)
// chamam NewID() internamente. Nenhuma camada externa (UC, repo, factory, module)
// recebe ou conhece geradores de ID.
//
// Trade-off explícito: testes não verificam o valor exato do ID; apenas as
// invariantes (UUID v4 válido, não vazio, único entre entidades).
func NewID() string { return uuid.NewString() }
```

**Local:** `internal/identity/domain/entities/user.go` (extrato do construtor)

```go
func New(whatsapp valueobjects.WhatsAppNumber, opts ...Option) User {
    u := User{
        id:        NewID(),                       // domínio se auto-serve
        whatsapp:  whatsapp,
        status:    StatusActive,
        createdAt: time.Now().UTC(),              // inline no construtor
        updatedAt: time.Now().UTC(),
    }
    for _, opt := range opts {
        opt(&u)
    }
    return u
}
```

> `internal/platform/id.UUIDGenerator` permanece no working tree por compatibilidade histórica, mas **não é importado por nenhum módulo novo**. Identity, outbox refactor e demais módulos seguem o padrão "domínio autossuficiente".

---

## 6. Infrastructure — Repository Factory

**Local:** `internal/identity/infrastructure/repositories/factory.go`

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

**Quando crescer:** ao adicionar `WhatsAppHistoryRepository`, basta:

1. Declarar `WhatsAppHistoryRepository(db database.DBTX) WhatsAppHistoryRepository` no port.
2. Implementar o método no `repositoryFactory` chamando `postgresHist.NewWhatsAppHistoryRepository(r.o11y, db)`.
3. UCs ganham acesso via `factory.WhatsAppHistoryRepository(tx)`.

---

## 7. Infrastructure — Repository Postgres

**Local:** `internal/identity/infrastructure/repositories/postgres/user_repository.go`

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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
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
		id, whatsapp, status            string
		email, displayName              sql.NullString
		createdAt, updatedAt            time.Time
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

func (r *userRepository) FindByID(ctx context.Context, id string) (entities.User, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "identity.repository.user.find_by_id")
	defer span.End()

	const query = `
        SELECT id, whatsapp_number, email, display_name, status, created_at, updated_at
          FROM users
         WHERE id = $1 AND deleted_at IS NULL
    `

	var (
		whatsapp, status     string
		email, displayName   sql.NullString
		createdAt, updatedAt time.Time
		foundID              string
	)
	err := r.db.QueryRowContext(ctx, query, id).
		Scan(&foundID, &whatsapp, &email, &displayName, &status, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return entities.User{}, fmt.Errorf("%s %w", prefixUserRepository, application.ErrUserNotFound)
	}
	if err != nil {
		span.RecordError(err)
		return entities.User{}, fmt.Errorf("%s find by id: %w", prefixUserRepository, err)
	}

	return entities.Hydrate(foundID, whatsapp, email.String, displayName.String, status, createdAt, updatedAt, time.Time{}), nil
}

// FindByWhatsAppNumber, MarkDeleted, AppendWhatsAppHistory: mesmo shape — Tracer.Start,
// QueryRowContext/ExecContext, span.RecordError em IO, sql.ErrNoRows → ErrUserNotFound,
// pgerrcode.UniqueViolation → sentinel correspondente, wrap com prefixo.
```

**Pontos críticos:**

- Campo `db database.DBTX` é setado uma única vez pelo factory; a mesma instância serve tanto para pool quanto para tx (transparente).
- **Sem `PrepareContext`** — a interface `database.DBTX` da devkit-go não expõe Prepare; usar `ExecContext`/`QueryContext`/`QueryRowContext` direto (pgx faz cache de prepared statements internamente).
- `errors.Is(err, sql.ErrNoRows)` antes de outros caminhos.
- Detecção de constraint violation por `pgerrcode.UniqueViolation` + `ConstraintName` mapeia para sentinel do `application/errors.go`.
- Log estruturado com PII mascarada (`candidate.WhatsApp().Masked()`).
- **Conversão de zero-value Go → SQL NULL via `internal/platform/sqlnull`** (ver §7.1) — **proibido reimplementar** helpers `nullableString`/`nullableTime` locais por módulo.

### 7.1. Helpers compartilhados — `internal/platform/sqlnull`

**Local:** `internal/platform/sqlnull/sqlnull.go`
**Responsabilidade:** converter valores zero do Go em SQL NULL quando passados como parâmetros para drivers que aceitam `any` (`database/sql`, `pgx`, `database.DBTX`).

```go
package sqlnull

import "time"

// Str retorna nil quando s == "" e o próprio s caso contrário.
func Str(s string) any {
    if s == "" { return nil }
    return s
}

// Time retorna nil quando t.IsZero() e o próprio t caso contrário.
func Time(t time.Time) any {
    if t.IsZero() { return nil }
    return t
}
```

**Uso obrigatório em todo módulo** que escreva colunas anuláveis:

```go
row := r.db.QueryRowContext(ctx, query,
    user.ID(),
    user.WhatsApp().String(),
    sqlnull.Str(user.Email().String()),     // "" → NULL
    sqlnull.Str(user.DisplayName()),         // "" → NULL
    sqlnull.Time(user.DeletedAt()),          // zero → NULL
    user.CreatedAt(),
    user.UpdatedAt(),
)
```

**Regras inegociáveis:**

- **Proibido reimplementar** `nullableString`/`nullableEmail`/`nullableTime` em `infrastructure/repositories/...` de qualquer módulo. Consumir sempre `sqlnull.Str` / `sqlnull.Time`.
- **Sem helpers para inteiros/bools** intencional: `0` e `false` podem ser valores semanticamente válidos; usar `*int64`/`*bool` ou `sql.NullInt64`/`sql.NullBool` explicitamente no call-site quando colunas inteiras/booleanas forem anuláveis. Foot-gun "0 vira NULL silenciosamente" não cabe em código de produção.
- `sqlnull.Str` **não trima** espaços — `" "` (espaço puro) é considerado valor válido. Normalização (trim, lowercase) é responsabilidade do Value Object ou do call-site, não desse helper.
- Pacote sem dependências externas; testes verdes em `go test -race -count=1 ./internal/platform/sqlnull/...`.

---

## 8. Infrastructure — Router + Handler HTTP

### 8.1. Router

**Local:** `internal/identity/infrastructure/http/server/router.go`

```go
package server

import "github.com/go-chi/chi/v5"

// UserRouter implementa chi_server.Router (Register(chi.Router)).
// Composto pelos handlers do módulo. Quando rotas reais existirem,
// IdentityModule popula o campo UserRouter e o bootstrap registra
// no httpserver via srv.RegisterRouters(m.UserRouter).
type UserRouter struct {
	upsertHandler *UpsertUserByWhatsAppHandler
}

func NewUserRouter(upsert *UpsertUserByWhatsAppHandler) *UserRouter {
	return &UserRouter{upsertHandler: upsert}
}

func (rt *UserRouter) Register(r chi.Router) {
	r.Route("/api/v1/identity/users", func(sub chi.Router) {
		sub.Post("/", rt.upsertHandler.Handle)
	})
}
```

### 8.2. Handler

**Local:** `internal/identity/infrastructure/http/server/handlers/upsert_user_by_whatsapp_handler.go`

```go
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type UpsertUserByWhatsAppHandler struct {
	usecase *usecases.UpsertUserByWhatsApp
	o11y    observability.Observability
}

func NewUpsertUserByWhatsAppHandler(
	uc *usecases.UpsertUserByWhatsApp,
	o11y observability.Observability,
) *UpsertUserByWhatsAppHandler {
	return &UpsertUserByWhatsAppHandler{usecase: uc, o11y: o11y}
}

type upsertUserRequest struct {
	WhatsApp    string `json:"whatsapp"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type upsertUserResponse struct {
	ID          string `json:"id"`
	WhatsApp    string `json:"whatsapp"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
}

func (h *UpsertUserByWhatsAppHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "identity.handler.upsert_user_by_whatsapp")
	defer span.End()

	var req upsertUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responses.Error(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	whatsapp, err := valueobjects.NewWhatsAppNumber(req.WhatsApp)
	if err != nil {
		responses.ErrorWithDetails(w, http.StatusBadRequest, "whatsapp inválido",
			map[string]string{"code": "invalid_whatsapp", "field": "whatsapp"})
		return
	}

	var email valueobjects.Email
	if req.Email != "" {
		email, err = valueobjects.NewEmail(req.Email)
		if err != nil {
			responses.ErrorWithDetails(w, http.StatusBadRequest, "email inválido",
				map[string]string{"code": "invalid_email", "field": "email"})
			return
		}
	}

	out, err := h.usecase.Execute(ctx, input.UpsertUserByWhatsApp{
		WhatsApp:    whatsapp,
		Email:       email,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, application.ErrWhatsAppNumberInUse):
			responses.ErrorWithDetails(w, http.StatusConflict, "número já vinculado a outra conta",
				map[string]string{"code": "whatsapp_in_use"})
		case errors.Is(err, application.ErrEmailInUse):
			responses.ErrorWithDetails(w, http.StatusConflict, "email já vinculado a outra conta",
				map[string]string{"code": "email_in_use"})
		default:
			h.o11y.Logger().Error(ctx, "identity.handler.upsert_failed",
				observability.String("layer", "handler"),
				observability.String("operation", "upsert_user_by_whatsapp"),
				observability.Error(err),
			)
			responses.Error(w, http.StatusInternalServerError, "erro inesperado")
		}
		return
	}

	responses.JSON(w, http.StatusOK, upsertUserResponse{
		ID:          out.ID,
		WhatsApp:    out.WhatsApp,
		Email:       out.Email,
		DisplayName: out.DisplayName,
	})
}
```

**Princípios:**

- Handler **não** chama repositório direto. Caminho único: `Handler → UseCase → UoW → Factory → Repository`.
- Handler converte JSON → VOs (`NewWhatsAppNumber`, `NewEmail`) antes de chamar o UC. Domínio nunca recebe `string` cru (RF-04).
- Mapeamento de erros tipados para HTTP status acontece no handler, não no UC.
- **Resposta via `devkit-go/pkg/responses`** (`JSON`, `Error`, `ErrorWithDetails`). **Proibido reimplementar** `writeJSON`/`writeError` localmente — o pacote já seta `Content-Type: application/json` e serializa via `encoding/json` de forma uniforme entre handlers.
- **Códigos semânticos** (`whatsapp_in_use`, `invalid_email`, `email_in_use`, …) viajam em `details` (`map[string]string{"code": "..."}`); mensagem humana em PT-BR; código em snake_case para consumo por máquinas. Contrato estável mesmo com pacote minimalista.
- Log estruturado com `layer` + `operation` para correlacionar com tracer.

---

## 9. Wiring — `module.go`

**Local:** `internal/identity/module.go`

```go
package identity

import (
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
)

type IdentityModule struct {
	RepositoryFactory interfaces.RepositoryFactory
	UserRouter        *server.UserRouter
}

func NewIdentityModule(
	cfg *configs.Config,
	o11y observability.Observability,
	mgr manager.Manager,                  // ← cada UC ganha seu UoW[T]; sem IDGenerator
) IdentityModule {
	factory := repositories.NewRepositoryFactory(o11y)

	// 1 UoW tipado por UC (T = retorno do callback).
	upsertUoW := uow.New[entities.User](mgr, uow.WithObservability(o11y))
	upsertUC := usecases.NewUpsertUserByWhatsApp(upsertUoW, factory, o11y)

	// UCs sem retorno: NewVoid produz uow.UnitOfWork[struct{}].
	markDeletedUoW := uow.NewVoid(mgr, uow.WithObservability(o11y))
	markDeletedUC := usecases.NewMarkUserDeleted(markDeletedUoW, factory, o11y)

	upsertHandler := handlers.NewUpsertUserByWhatsAppHandler(upsertUC, o11y)
	// … demais handlers consomem markDeletedUC etc.

	return IdentityModule{
		RepositoryFactory: factory,
		UserRouter:        server.NewUserRouter(upsertHandler),
	}
}
```

**Convenções:**

- Construtor **único** `NewIdentityModule(...) IdentityModule` (sem `opts ...Option`, sem `With...`).
- Recebe `manager.Manager` (não um `UnitOfWork` solto): cada UC tem `T` próprio, então o module instancia `uow.New[T](mgr, …)` por UC.
- **Sem `IDGenerator`** na assinatura: domínio é autossuficiente (ver §5.1). UCs não recebem gerador.
- `uow.WithObservability(o11y)` é passada em **toda** construção — habilita tracer/métricas/logs da devkit.
- Ordem dos campos espelha a ordem de construção: `factory → uow[T] → usecase → handler → router`.
- `UserRouter` é `nil` se o módulo ainda não tiver rotas reais; bootstrap só registra quando `!= nil`.

---

## 10. Bootstrap — `cmd/server/server.go` (delta)

```go
// ...após dbManager (manager.Manager) e o11y já inicializados...

identityModule := identity.NewIdentityModule(cfg, o11y, dbManager)

srv, err := httpserver.New(o11y, /* ...opts... */)
if err != nil { return fmt.Errorf("run: failed to create http server: %w", err) }

if identityModule.UserRouter != nil {
	srv.RegisterRouters(identityModule.UserRouter)
}
```

**Sem UoW único no bootstrap.** Cada Module cria seus `uow.New[T](dbManager, …)` tipados conforme os UCs que orquestra. O bootstrap só repassa `dbManager` (Manager) + `o11y` + `cfg` — **sem `IDGenerator`** (domínio se auto-serve).

**`cmd/worker/worker.go`** segue o mesmo padrão: instancia módulos que tenham consumers/jobs (cada um cria seus UoWs tipados internamente) e os passa para `worker.NewManager(...)`.

---

## 11. Orquestrando múltiplos repos na mesma TX

Exemplo: UC que atualiza `User` e grava entrada em `user_whatsapp_history` **atomicamente**.

```go
// Campo do UC: uow uow.UnitOfWork[entities.User]
updated, err := u.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.User, error) {
	userRepo := u.factory.UserRepository(tx)
	historyRepo := u.factory.WhatsAppHistoryRepository(tx)

	if err := userRepo.UpdateWhatsAppNumber(ctx, userID, newNumber, time.Now().UTC()); err != nil {
		return entities.User{}, fmt.Errorf("%s update number: %w", prefix, err)
	}

	// entities.NewWhatsAppHistoryEntry é autossuficiente: gera o UUID via entities.NewID()
	// e resolve linked_at com time.Now().UTC() inline no construtor — sem IDGenerator injetado.
	entry := entities.NewWhatsAppHistoryEntry(userID, oldNumber, "port_in")
	if err := historyRepo.Append(ctx, userID, entry); err != nil {
		return entities.User{}, fmt.Errorf("%s append history: %w", prefix, err)
	}

	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		return entities.User{}, fmt.Errorf("%s fetch updated: %w", prefix, err)
	}
	return user, nil
})
```

Se `historyRepo.Append` falhar, **ambas** as operações sofrem rollback (a do `userRepo` também), porque compartilham o mesmo `tx`. O retorno tipado (`entities.User`) elimina a closure sobre `var result` que o padrão antigo exigia. `time.Now().UTC()` é resolvido **inline** em cada call-site; sem variável intermediária. Construtores das entidades carregam a própria geração de ID via `entities.NewID()` — nenhuma camada externa injeta gerador.

---

## 12. Testes

### 12.1. Unitário de UC (sem banco)

Mocka `uow.UnitOfWork[T]`, `interfaces.RepositoryFactory`, `interfaces.UserRepository`. O mock de `UnitOfWork[T].Do` executa o callback inline com um `tx` "dummy" (`nil` ou stub) — basta que o factory mock devolva o repo mock independentemente do `tx`.

```go
// Mockery v2.30+ suporta interfaces genéricas. Mock tipado por T = entities.User.
mockUoW := mocks.NewMockUnitOfWork[entities.User](t)
mockUoW.EXPECT().
	Do(
		mock.Anything,
		mock.AnythingOfType("func(context.Context, database.DBTX) (entities.User, error)"),
	).
	RunAndReturn(func(
		ctx context.Context,
		fn func(context.Context, database.DBTX) (entities.User, error),
		_ ...uow.Option,
	) (entities.User, error) {
		return fn(ctx, nil) // executa o callback inline; factory mock devolve repo mock independente do tx
	})

mockFactory := mocks.NewMockRepositoryFactory(t)
mockUserRepo := mocks.NewMockUserRepository(t)
mockFactory.EXPECT().UserRepository(mock.Anything).Return(mockUserRepo)

mockUserRepo.EXPECT().
	FindByWhatsAppNumber(mock.Anything, mock.Anything).
	Return(entities.User{}, application.ErrUserNotFound)

mockUserRepo.EXPECT().
	UpsertByWhatsAppNumber(mock.Anything, mock.Anything, mock.Anything).
	Return(expectedUser, nil)

// Construtor recebe apenas 3 parâmetros — sem IDGenerator.
uc := usecases.NewUpsertUserByWhatsApp(mockUoW, mockFactory, noopO11y)
out, err := uc.Execute(ctx, input.UpsertUserByWhatsApp{WhatsApp: vo, /*...*/ })

require.NoError(t, err)
// Asserção valida invariante (UUID v4 não vazio), não valor determinístico.
require.NotEmpty(t, out.ID)
require.Equal(t, expectedUser.WhatsApp().String(), out.WhatsApp)
```

### 12.2. Integração de repository (Postgres real, build tag `integration`)

```go
//go:build integration

func TestUserRepository_UpsertAndFindWithinTx(t *testing.T) {
	mgr := setupManager(t) // testcontainers postgres + migrate
	defer cleanup(t, mgr)

	o11y := noop.NewProvider()
	u := uow.New[entities.User](mgr, uow.WithObservability(o11y))
	factory := repositories.NewRepositoryFactory(o11y)

	ctx := context.Background()
	whatsapp, _ := valueobjects.NewWhatsAppNumber("+5511988887777")

	fetched, err := u.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.User, error) {
		repo := factory.UserRepository(tx)

		// entities.New gera ID via entities.NewID(); time.Now().UTC() inline no construtor.
		candidate := entities.New(whatsapp, entities.WithDisplayName("Alice"))
		if _, err := repo.UpsertByWhatsAppNumber(ctx, candidate, time.Now().UTC()); err != nil {
			return entities.User{}, err
		}
		return repo.FindByWhatsAppNumber(ctx, whatsapp)
	})
	require.NoError(t, err)
	require.Equal(t, "Alice", fetched.DisplayName())
	require.NotEmpty(t, fetched.ID()) // invariante: UUID v4 gerado pelo domínio
}
```

### 12.3. Integração end-to-end (Handler → UC → UoW → Repo real)

Use `httptest.NewServer` montando `srv.RegisterRouters(identityModule.UserRouter)` com Postgres real e exercite o endpoint HTTP. Cobre todos os layers em um único teste — útil para CA-04 de E1.

---

## 13. Checklist de aderência

Antes de abrir PR de qualquer módulo novo, valide:

- [ ] Use case **nunca** chama `manager.Manager.BeginTx` direto — sempre `devkitUoW.UnitOfWork[T].Do`. UCs sem retorno usam `uow.UnitOfWork[struct{}]` injetado via `uow.NewVoid`.
- [ ] **Proibido reimplementar UoW** localmente em `internal/platform/uow/` ou similar. Consumir sempre `github.com/JailtonJunior94/devkit-go/pkg/database/uow`.
- [ ] **Proibido injetar `IDGenerator`** em qualquer camada (UC, repository, factory, module, handler). Domínio gera ID via `entities.NewID()` chamada dentro do construtor da entidade — sem DI, sem interface `entities.IDGenerator`, sem `id.Generator` no module.
- [ ] **Handler usa `devkit-go/pkg/responses`** (`JSON`, `Error`, `ErrorWithDetails`) — proibido reimplementar `writeJSON`/`writeError` locais. Códigos semânticos viajam em `details` (`map[string]string{"code": "..."}`); mensagem humana em PT-BR; código em snake_case.
- [ ] **`time.Now().UTC()` chamado inline no call-site** (parâmetro do método ou campo do construtor) — **proibido** capturar em variável intermediária (`now := time.Now().UTC()`). Reforça R6.7 (sem clock abstrato).
- [ ] **Conversão zero-value → SQL NULL via `internal/platform/sqlnull`** (`sqlnull.Str`, `sqlnull.Time`) — **proibido** reimplementar `nullableString`/`nullableEmail`/`nullableTime` em repositórios. Para colunas inteiras/booleanas anuláveis usar `*int64`/`*bool` ou `sql.Null*` explicitamente.
- [ ] `uow.WithObservability(o11y)` passada na construção de **todo** UoW — habilita tracer + métricas internas.
- [ ] Repositórios **nunca** recebem `tx` como argumento de método — somente via construtor injetado pelo factory.
- [ ] Factory devolve **interface** do port (`interfaces.UserRepository`), nunca o struct concreto (R6 — interface no consumidor).
- [ ] Handler **nunca** chama repositório direto; segue Handler → UC.
- [ ] Sem `init()` (R0); sem `panic` em produção (R5.12); sem `Clock` abstrato (R6.7); sem `var _ Interface = (*Type)(nil)` (R6.4).
- [ ] Todo erro de IO é envelopado com `fmt.Errorf("<prefixo> ctx: %w", err)` (R5.10) e logado **uma única vez** no ponto adequado.
- [ ] Sem `PrepareContext` direto na interface `database.DBTX` (não existe na devkit v0.4.0).
- [ ] PII sempre mascarada em logs (`WhatsAppNumber.Masked()`, `Email.Masked()`, `pii.MaskDisplayName`).
- [ ] Tracer span por camada (`handler.*`, `usecase.*`, `repository.*`) — `db.{driver}.tx` é emitido pela devkit-uow automaticamente; não duplicar.
- [ ] Testes: UC com mocks de `UnitOfWork[T]` + `RepositoryFactory` + repo; repo com integration test contra Postgres real (build tag `integration`). Asserções de ID validam invariantes (UUID v4, não vazio), não valor determinístico.

---

## 14. Referências

- `AGENTS.md` — Padrão Obrigatório de Módulo (itens 1–7).
- `.claude/rules/governance.md` — precedência de regras e política de evidência.
- `.agents/skills/go-implementation/SKILL.md` — etapas 1–5 obrigatórias antes de codar.
- `.agents/skills/go-implementation/references/architecture.md` — R0–R7 detalhadas.
- **`devkit-go/pkg/database/uow`** — leitura obrigatória antes de implementar qualquer UC transacional. Fonte canônica do contrato `UnitOfWork[T]`.
- **`internal/platform/sqlnull`** — helpers `Str`/`Time` para conversão zero-value Go → SQL NULL. Único ponto compartilhado entre módulos para esse padrão.
- `internal/platform/outbox/storage_postgres.go` (após refactor de E1) — referência viva do mesmo padrão aplicado a producer/consumer.
- devkit-go v0.4.0:
  - `pkg/database` — `DBTX`, `Tx`, `WithTx`, `FromContext`, `TxOptions`.
  - `pkg/database/manager` — `Manager.BeginTx`, `Manager.DBTX(ctx)`.
  - `pkg/database/uow` — `UnitOfWork[T]`, `New[T]`, `NewVoid`, opções (`WithObservability`, `WithIsolation`, `WithReadOnly`).
  - `pkg/observability` — `Tracer.Start`, `Span.RecordError`, `Logger`.
  - `pkg/http_server/chi_server` — `Server.RegisterRouters`, `Router{Register(chi.Router)}`.
  - `pkg/responses` — `JSON(w, status, data)`, `Error(w, status, message)`, `ErrorWithDetails(w, status, message, details)`. **Proibido reimplementar** helpers locais de resposta.

---

**Fim do runbook.**

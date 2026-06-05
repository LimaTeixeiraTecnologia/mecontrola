# Tarefa 6.0: Impl Postgres — RepositoryFactory + user_repository com sqlnull

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar a implementação Postgres do `UserRepository` e a `RepositoryFactory` que produz instâncias amarradas a uma `database.DBTX` recebida. O repo consome `internal/platform/sqlnull` (já criado) para conversão zero-value→NULL e mapeia `pgerrcode.UniqueViolation` por `ConstraintName` para os sentinels declarados em 4.0. **Proibido** `PrepareContext` (não existe na interface `database.DBTX` da devkit) — usar `ExecContext`/`QueryContext`/`QueryRowContext` direto. **Proibido** `database.FromContext` no repo — o `tx` chega via factory. Testes de integração com testcontainers cobrem CA-04 a–h.

<requirements>
- RF-07: SQL filtra `WHERE deleted_at IS NULL` em todas as leituras.
- RF-08: violação de UNIQUE parcial mapeia para sentinel correspondente via `pgerrcode.UniqueViolation` + `ConstraintName` (`users_whatsapp_number_active_uniq` → `ErrWhatsAppNumberInUse`, `users_email_active_uniq` → `ErrEmailInUse`).
- RF-11: implementação em `internal/identity/infrastructure/repositories/postgres/` recebendo `database.DBTX` no construtor.
- ADR-008: `RepositoryFactory.UserRepository(db database.DBTX)` devolve repo amarrado.
- R6.11: consumir `sqlnull.Str` / `sqlnull.Time` em colunas anuláveis — proibido `nullableString` local.
- Sem `PrepareContext` (não existe em `database.DBTX`); sem `database.FromContext` interno.
- Tracer span por método: `identity.repository.user.<operação>`. `span.RecordError(err)` em todo erro de IO.
- Log estruturado com PII mascarada (`candidate.WhatsApp().Masked()`).
- Reanimação (RF-08-ter) é decidida no UC (5.0), não no SQL — o repo só executa upsert atômico com `COALESCE(users.display_name, EXCLUDED.display_name)` para FWW.
</requirements>

## Subtarefas

- [ ] 6.1 `internal/identity/infrastructure/repositories/postgres/user_repository.go` com struct `userRepository{o11y, db database.DBTX}` + `NewUserRepository(o11y, db) interfaces.UserRepository`.
- [ ] 6.2 Métodos: `UpsertByWhatsAppNumber` (INSERT … ON CONFLICT … DO UPDATE … RETURNING + COALESCE para FWW), `FindByID`, `FindByWhatsAppNumber`, `MarkDeleted`, `AppendWhatsAppHistory` — cada um com Tracer.Start, QueryRow/Exec, `sql.ErrNoRows`→`ErrUserNotFound`, `pgerrcode.UniqueViolation`→sentinel, wrap com prefixo `identity.repository.user:`.
- [ ] 6.3 `internal/identity/infrastructure/repositories/factory.go` com `repositoryFactory{o11y}` + `NewRepositoryFactory(o11y) interfaces.RepositoryFactory` + método `UserRepository(db database.DBTX)`.
- [ ] 6.4 Setup de testcontainers reutilizável em `internal/identity/infrastructure/repositories/postgres/testutil_test.go` (build tag `integration`): sobe `postgres:16`, aplica `golang-migrate` com FS embutido de `migrations/`.
- [ ] 6.5 Testes de integração `user_repository_integration_test.go` (build tag `integration`) cobrindo CA-04 a–h.

## Detalhes de Implementação

Referenciar:
- [`techspec.md` §10](./techspec.md) — shape canônico do repository (sem PrepareContext, com sqlnull).
- [Runbook §6 + §7](../../docs/runbooks/handler-usecase-uow-repository.md) — factory + repo Postgres.
- [ADR-007](./adr-007-postgres-partial-unique-indexes.md) — SQL completo (consistência com 1.0).
- [ADR-008](./adr-008-repository-factory-per-module.md) — padrão factory.

**SQL canônico do upsert (FWW via COALESCE):**

```sql
INSERT INTO users (id, whatsapp_number, email, display_name, status, created_at, updated_at, deleted_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NULL)
ON CONFLICT (whatsapp_number) WHERE deleted_at IS NULL
DO UPDATE SET
    display_name = COALESCE(users.display_name, EXCLUDED.display_name),
    email        = COALESCE(users.email,        EXCLUDED.email),
    updated_at   = EXCLUDED.updated_at
RETURNING id, whatsapp_number, email, display_name, status, created_at, updated_at
```

**Mapping de constraint → sentinel:**

```go
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
    switch pgErr.ConstraintName {
    case "users_whatsapp_number_active_uniq":
        return entities.User{}, fmt.Errorf("%s %w", prefixUserRepository, application.ErrWhatsAppNumberInUse)
    case "users_email_active_uniq":
        return entities.User{}, fmt.Errorf("%s %w", prefixUserRepository, application.ErrEmailInUse)
    }
}
```

**Cenários CA-04 (integration tests, build tag `integration`):**

a) Primeiro upsert insere; segundo upsert (mesmo número, mesmos campos) atualiza `updated_at`.
b) `MarkDeleted(id, now)` + `FindByID(id)` → `ErrUserNotFound` (filtro deleted_at IS NULL).
c) `AppendWhatsAppHistory` cria entrada; leitura SQL direta confirma persistência.
d) Soft delete + upsert dentro da janela (`now - deletedAt = 29d`) com mesmo número e novo input → reanimação (mesmo UUID, email/display_name zerados antes do input — orquestrado no caller via `User.Reanimate`, o repo só persiste).
e) Soft delete + upsert fora da janela (`now - deletedAt = 31d`) → novo UUID (caller decide via `User.CanReanimate`).
f) `display_name` first-write-wins: segundo upsert com nome diferente preserva o primeiro (validado por COALESCE no SQL).
g) Upsert sem mudanças → `updated_at` muda (touch garantido).
h) SQL direto tentando `status='DELETED' AND deleted_at IS NULL` é rejeitado pelo CHECK constraint (validação cruzada com 1.0).

## Critérios de Sucesso

- `go test -race -count=1 ./internal/identity/infrastructure/repositories/...` verde.
- `go test -race -count=1 -tags=integration ./internal/identity/infrastructure/repositories/postgres/...` verde (CA-04 a–h cobertos).
- `grep -n "PrepareContext\|database.FromContext\|nullableString\|nullableEmail" internal/identity/infrastructure/repositories/` retorna 0 (R6.11 + ADR-008).
- Mapping de constraint testado: integration test força violação de cada UNIQUE parcial e valida sentinel.
- Sem `BeginTx` interno no repo (lifecycle de TX é do UoW chamado pelo UC em 5.0).
- `go build ./...` verde.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff). -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] `user_repository_test.go` (sem build tag) — testes que não exigem banco: scanning, helper interno, type assertion de erros.
- [ ] `user_repository_integration_test.go` (build tag `integration`) — CA-04 a–h.
- [ ] `factory_test.go` — `NewRepositoryFactory(o11y).UserRepository(pool)` devolve instância não-nil que satisfaz `interfaces.UserRepository`.
- [ ] Integration test multi-repo (futuro): `uow.New[entities.User](mgr, ...)` callback usa `factory.UserRepository(tx)` e `factory.WhatsAppHistoryRepository(tx)` (quando este último for adicionado em E1+).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/identity/infrastructure/repositories/factory.go` (criar)
- `internal/identity/infrastructure/repositories/factory_test.go` (criar)
- `internal/identity/infrastructure/repositories/postgres/user_repository.go` (criar)
- `internal/identity/infrastructure/repositories/postgres/user_repository_test.go` (criar)
- `internal/identity/infrastructure/repositories/postgres/user_repository_integration_test.go` (criar — build tag `integration`)
- `internal/identity/infrastructure/repositories/postgres/testutil_test.go` (criar — build tag `integration`)
- Dependências: `internal/platform/sqlnull` (já criado), `internal/identity/application/{errors.go,interfaces/*}` (4.0), `internal/identity/domain/entities/*` (3.0), `migrations/000002`/`000003` (1.0).
- Pacotes externos: `github.com/jackc/pgerrcode`, `github.com/jackc/pgx/v5/pgconn`, `github.com/testcontainers/testcontainers-go` (testes integração).

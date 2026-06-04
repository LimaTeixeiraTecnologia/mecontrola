# Tarefa 7.0: Adapter Postgres `PgxUserRepository` + `rowMapper` + UoW interna + `uuid_generator`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o adapter concreto `PgxUserRepository` em `internal/identity/infrastructure/repositories/postgres/` usando pgx/v5 via `internal/platform/database.Manager` + `UnitOfWork[T]`. Cobre as 5 operações com soft delete filtrado em leitura (RF-12), upsert idempotente, UoW interna em `SoftDelete` (cascata em `user_whatsapp_history`) e `LinkNewNumber` (3 statements atômicos). `rowMapper.HydrateUser` reconstrói `*entities.User` re-validando invariantes via construtores de VOs (ADR-008). Implementar também `infrastructure/id/uuid_generator.go` (adapter para `google/uuid`).

<requirements>
- RF-12: todas as queries de leitura filtram `WHERE deleted_at IS NULL`.
- RF-07 (cascata): `SoftDelete` propaga `active=false` em `user_whatsapp_history` (ADR-009).
- ADR-010: `SoftDelete` e `LinkNewNumber` abrem `database.UnitOfWork[struct{}]` interna; assinatura do port não recebe `tx`.
- ADR-008: `rowMapper.HydrateUser` chama `NewUserID`, `NewWhatsAppNumber`, `NewEmail` — falha tipada em row corrompida.
- `UpsertByWhatsAppNumber` traduz `pgerrcode.UniqueViolation` para `ErrDuplicateWhatsAppNumber`.
- Sentinelas: `ErrUserNotFound`, `ErrDuplicateWhatsAppNumber` em `infrastructure/repositories/postgres/errors.go`.
- Receiver single-letter `r` para `*PgxUserRepository`; fields com nomes completos (`manager`, `idGenerator`, `clock`).
- `infrastructure/id/uuid_generator.go` implementa `interfaces.IDGenerator` chamando `uuid.NewString()`.
- Apenas `user_repository.go` importa pgx/pgerrcode (R5.10 + isolamento).
</requirements>

## Subtarefas

- [ ] 7.1 Criar `internal/identity/infrastructure/repositories/postgres/errors.go` com `ErrUserNotFound`, `ErrDuplicateWhatsAppNumber`.
- [ ] 7.2 Criar `internal/identity/infrastructure/repositories/postgres/queries.go` com constantes SQL (SELECT, INSERT, UPDATE soft delete, UPDATE link, INSERT history, UPDATE history). Toda query parametrizada (`$1`, `$2`).
- [ ] 7.3 Criar `internal/identity/infrastructure/repositories/postgres/mapper.go` com `type rowMapper struct{}`, `type userRow struct{...}` e `(rowMapper) HydrateUser(row userRow) (*entities.User, error)` re-validando via construtores.
- [ ] 7.4 Criar `internal/identity/infrastructure/repositories/postgres/user_repository.go` com `type PgxUserRepository struct{ manager *database.Manager; idGenerator interfaces.IDGenerator; clock clock.Clock }`, `NewPgxUserRepository`, e as 5 operações.
- [ ] 7.5 Implementar `UpsertByWhatsAppNumber`: SELECT por number; hit → mapper; miss → INSERT com id de `r.idGenerator.NewUserID()` + `now = r.clock.Now()`; capturar `pgerrcode.UniqueViolation` em race.
- [ ] 7.6 Implementar `FindByID` e `FindByWhatsAppNumber` com filtro `deleted_at IS NULL`.
- [ ] 7.7 Implementar `SoftDelete` abrindo `database.NewUnitOfWork[struct{}](r.manager).Do(...)` com (a) UPDATE users (retorna `ErrUserNotFound` se rows=0) + (b) UPDATE user_whatsapp_history `SET active=false, unlinked_at=now, reason='user_soft_deleted'`.
- [ ] 7.8 Implementar `LinkNewNumber` abrindo UoW com (a) UPDATE history active→false, (b) INSERT new history active=true, (c) UPDATE users.whatsapp_number.
- [ ] 7.9 Criar `internal/identity/infrastructure/id/uuid_generator.go` com `type UUIDGenerator struct{}`, `NewUUIDGenerator()`, e `(UUIDGenerator) NewUserID() string` retornando `uuid.NewString()`.
- [ ] 7.10 Unit test puro para `rowMapper.HydrateUser` em `mapper_test.go` cobrindo: hidratação válida; row com id corrompido → erro; row com WhatsApp corrompido → erro; row com email vazio (NULL) → User com email nil.

## Detalhes de Implementação

Ver techspec §"Implementação Postgres" subseção `user_repository.go` (pseudo-código completo) e §"Mapper" subseção `mapper.go`. ADR-008 (re-validação), ADR-009 (cascata SoftDelete), ADR-010 (UoW interna).

## Critérios de Sucesso

- `PgxUserRepository` implementa `interfaces.UserRepository` (verificação em compile-time: `var _ interfaces.UserRepository = (*PgxUserRepository)(nil)` em `user_repository.go`).
- `UUIDGenerator` implementa `interfaces.IDGenerator`.
- Soft delete em user inexistente retorna `ErrUserNotFound`.
- `LinkNewNumber` em user inexistente retorna `ErrUserNotFound`.
- Race condition de upsert simultâneo gera `ErrDuplicateWhatsAppNumber` (covered in 9.0).
- Mapper rejeita row com id não-UUID-v4 com erro wrappado `"postgres user mapper: id corrompido: %w"`.
- Apenas `user_repository.go` importa `pgx` e `pgerrcode`.

## Definition of Done (DoD)

- [ ] `go build ./internal/identity/...` passa.
- [ ] `go test -race -count=1 ./internal/identity/infrastructure/repositories/postgres/mapper_test.go ./internal/identity/infrastructure/repositories/postgres/mapper.go` passa.
- [ ] `golangci-lint run ./internal/identity/infrastructure/...` passa.
- [ ] `grep -rn 'pgx\|pgerrcode' internal/identity/infrastructure/repositories/postgres/ | grep -v user_repository.go` retorna vazio (isolamento).
- [ ] `grep -rn 'database/sql' internal/identity/infrastructure/repositories/postgres/` permitido apenas em `mapper.go` para `sql.NullString`/`sql.NullTime`.
- [ ] `var _ interfaces.UserRepository = (*PgxUserRepository)(nil)` está presente.
- [ ] Receivers single-letter `r` (`func (r *PgxUserRepository)`).
- [ ] `infrastructure/id/uuid_generator_test.go` valida que `NewUserID()` retorna UUID v4 parseável.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit puro para `rowMapper.HydrateUser` (hidratação válida + 3 cenários de corrupção).
- [ ] Unit puro para `UUIDGenerator.NewUserID` (formato UUID v4).
- [ ] Cobertura real do adapter contra Postgres fica na task 9.0 (integration).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/identity/infrastructure/repositories/postgres/errors.go` (novo)
- `internal/identity/infrastructure/repositories/postgres/queries.go` (novo)
- `internal/identity/infrastructure/repositories/postgres/mapper.go` (novo)
- `internal/identity/infrastructure/repositories/postgres/mapper_test.go` (novo)
- `internal/identity/infrastructure/repositories/postgres/user_repository.go` (novo)
- `internal/identity/infrastructure/id/uuid_generator.go` (novo)
- `internal/identity/infrastructure/id/uuid_generator_test.go` (novo)

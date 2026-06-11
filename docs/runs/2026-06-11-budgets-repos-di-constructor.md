# Refatorar repositórios budgets — DI via construtor

**Data:** 2026-06-11
**Escopo:** `internal/budgets/`

## Problema

Todos os 5 repositórios do módulo budgets passavam `db database.DBTX` como parâmetro de método em vez de injetá-lo via construtor. Violava o padrão de DI adotado nos demais módulos (card, identity, billing).

## Mudanças

### Interfaces (`application/interfaces/`)
- Removido `db database.DBTX` de todos os métodos em 5 interfaces: `ExpenseRepository`, `AlertRepository`, `BudgetRepository`, `PendingEventRepository`, `ThresholdStateRepository`
- Criado `repository_factory.go` com `RepositoryFactory` interface

### Repositórios (`infrastructure/repositories/postgres/`)
- Adicionado campo `db database.DBTX` nos 5 structs
- Construtores atualizados: `New*Repository(o11y, db)`
- Todos os métodos removeram o parâmetro `db`, passaram a usar `r.db`

### Factory (`infrastructure/repositories/factory.go`)
- Criado `repositoryFactory` que implementa `RepositoryFactory`
- `NewRepositoryFactory(o11y)` retorna factory pronta para injeção

### Use cases (`application/usecases/`)
- 15 use cases convertidos: campos de repo individuais → `factory interfaces.RepositoryFactory`
- Closures UoW criam repos via `uc.factory.XxxRepository(tx)`
- `apply_pending_event.go`: `Execute(ctx, db, evt)` ainda recebe `db` para criar repo via factory
- `create_or_auto_draft_for_expense.go`: `EnsureExists(ctx, tx, ...)` ainda recebe `tx` para criar repo via factory

### Module (`module.go`)
- `buildRepositories()` substituído por factory via `repositories.NewRepositoryFactory(b.o11y)`

### Mocks
- Regenerados com `mockery --config .mockery.yml`
- `RepositoryFactory` mock adicionado

## Validação
- `go build ./...` — OK
- `go vet ./internal/budgets/...` — OK
- `go test ./internal/budgets/application/usecases/...` — OK (all pass)
- Zero comentários em `.go` de produção — OK
- Sem `db` como parâmetro em interfaces de repositório — OK
- Sem `init()` — OK

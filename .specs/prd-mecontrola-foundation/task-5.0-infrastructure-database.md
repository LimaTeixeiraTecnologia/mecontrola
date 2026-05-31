# Tarefa 5.0: `internal/infrastructure/database` — Manager + UnitOfWork[T] + migrations `//go:embed` + sentinels

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Compor o `devkit-go/pkg/database` em `internal/infrastructure/database` provendo o **`manager.Manager` central** (D-08, ADR-002), **`UnitOfWork[T]` genérico tipado por agregado**, **migrations embarcadas via `//go:embed migrations/*.sql`** (D-07, ADR-007) com `golang-migrate`, e **sentinels de erro** (`ErrConnection`, `ErrMigration`, `ErrDeadlineExceeded`) traduzíveis pelo mapper RFC 7807 (consumido em 6.0). Cobre **RF-12** (migration de exemplo aplicável/revertível) e o lado de **RF-03** que executa migrations.

<requirements>
- Pool de 30 conexões (limite documentado no discovery); pool único compartilhado entre módulos (D-08).
- Factory `NewManager(cfg *configs.Config) (*Manager, error)` composando `manager.Manager` do devkit-go.
- `Manager.Pool() *pgxpool.Pool` + `Manager.HealthCheck(ctx) error` (SELECT do `health_probe`).
- Generic `UnitOfWork[T any]` re-exportado/tipado do devkit-go; consumidores (PRDs futuros) chamam `UnitOfWork[*Aggregate]`.
- Migrations embarcadas: `migrations/0001_init.up.sql` cria tabela `health_probe` (1 linha estática); `migrations/0001_init.down.sql` reverte.
- `RunMigrations(ctx context.Context, m *Manager) error` aplica via `golang-migrate/source/iofs` com `//go:embed migrations/*.sql`.
- Sentinels: `ErrConnection`, `ErrMigration`, `ErrDeadlineExceeded`.
- Default timeout para `UnitOfWork.Do`: 5s via `context.WithTimeout` obrigatório (caller pode override).
- Integration test com testcontainers `postgres:16-alpine` (D-20) cobrindo: pool startup, HealthCheck, migrate up/down, UoW commit/rollback.
</requirements>

## Subtarefas

- [ ] 5.1 Criar `internal/infrastructure/database/manager.go` com `NewManager(cfg) (*Manager, error)`; `Manager.Pool()`; `Manager.HealthCheck(ctx)`.
- [ ] 5.2 Criar `internal/infrastructure/database/uow.go` com `UnitOfWork[T any]` (factory + `Do(ctx, fn func(Tx) (T, error)) (T, error)` com timeout default 5s).
- [ ] 5.3 Criar diretório `migrations/` na raiz; criar `migrations/0001_init.up.sql` (CREATE TABLE health_probe + INSERT) + `migrations/0001_init.down.sql` (DROP TABLE).
- [ ] 5.4 Criar `internal/infrastructure/database/migrations.go` com `//go:embed migrations/*.sql` + `RunMigrations(ctx, m) error` usando `golang-migrate/source/iofs`.
- [ ] 5.5 Criar `internal/infrastructure/database/errors.go` com sentinels `ErrConnection`, `ErrMigration`, `ErrDeadlineExceeded`.
- [ ] 5.6 Criar `internal/infrastructure/database/manager_test.go` (unit, com mock minimal).
- [ ] 5.7 Criar `internal/infrastructure/database/database_integration_test.go` com tag `//go:build integration`: sobe `postgres:16-alpine` via testcontainers, executa `RunMigrations` up/down, valida UoW commit/rollback, valida HealthCheck.
- [ ] 5.8 Adicionar dependência `github.com/testcontainers/testcontainers-go` + `github.com/testcontainers/testcontainers-go/modules/postgres`.
- [ ] 5.9 Atualizar `taskfiles/test.yml` ou `taskfiles/migrations.yml` com `task migrate:up` e `task migrate:down` chamando o subcomando `mecontrola migrate` ou helper interno.
- [ ] 5.10 Implementar a interface placeholder usada por `cmd/migrate` em 3.0 (concretizar `database.NewManager` e `database.RunMigrations`).

## Detalhes de Implementação

Ver techspec §"Modelos de Dados" + §"Estratégia de Erros" + ADR-002 + ADR-007.

## Critérios de Sucesso

- `go build ./internal/infrastructure/database/...` compila.
- `go test -tags=integration ./internal/infrastructure/database/...` verde (testcontainers `postgres:16-alpine`).
- `task migrate:up` aplica `0001_init` + cria tabela `health_probe`; `task migrate:down` reverte sem erro.
- `Manager.HealthCheck(ctx)` retorna nil quando DB up; retorna `ErrConnection` (ou erro wrapping) quando DB down.
- UoW commit/rollback valido em test: tx que retorna erro reverte; tx ok commita.
- Cobre RF-03 (parcial — execução de migrations) e RF-12 (integralmente).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: `manager_test.go` com mock de pgx (assert pool init + HealthCheck happy/failure paths); `uow_test.go` com mock validando rollback em erro + timeout default.
- [ ] Testes de integração: `database_integration_test.go` com testcontainers — pool real, migrations up/down, UoW commit/rollback contra Postgres real.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/infrastructure/database/manager.go`
- `internal/infrastructure/database/uow.go`
- `internal/infrastructure/database/migrations.go`
- `internal/infrastructure/database/errors.go`
- `internal/infrastructure/database/manager_test.go`
- `internal/infrastructure/database/uow_test.go`
- `internal/infrastructure/database/database_integration_test.go`
- `migrations/0001_init.up.sql`
- `migrations/0001_init.down.sql`
- `taskfiles/migrations.yml` (ou seção em build.yml)
- `cmd/migrate/cmd.go` (binding concreto)
- `go.mod`, `go.sum`

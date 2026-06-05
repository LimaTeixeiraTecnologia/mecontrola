# Tarefa 10.0: Refactor internal/platform/outbox para UoW + RepositoryFactory (§17 techspec)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Migrar o outbox para o mesmo padrão canônico que identity adota (ADR-008): construtor do storage recebe `database.DBTX` (sem `manager.Manager`), `OutboxRepositoryFactory` produz instâncias amarradas ao `tx`, dispatcher job consome `uow.UnitOfWork[[]outbox.Row]` da devkit. Sem isso, o working tree fica com dois padrões coexistindo, contradizendo o gate do runbook. **Contrato externo do outbox é preservado** — apenas a composição (DI, lifecycle de TX) muda. Testes existentes são adaptados.

<requirements>
- Storage Postgres deixa de receber `manager.Manager` e passa a receber `database.DBTX` no construtor.
- `OutboxRepository` (port) e `OutboxRepositoryFactory` (port) declarados em `internal/platform/outbox/ports.go`.
- `DispatcherJob` recebe `uow.UnitOfWork[[]Row]` + `OutboxRepositoryFactory`.
- `ReaperJob` e `HousekeepingJob` recebem `uow.UnitOfWork[struct{}]` (via `uow.NewVoid`) + factory.
- `cmd/worker/worker.go` instancia os UoWs do outbox e os passa para os jobs (junto com a factory).
- **Sem `BeginTx` interno no storage** — lifecycle de TX é do caller via `uow.Do`.
- Contrato externo de inserção de evento (`Insert(ctx, evt, maxAttempts)`) e poll/claim preservado.
- Migrations `000001_outbox_events.{up,down}.sql` intactas.
- Testes existentes em `internal/platform/outbox/` (`dispatcher_test.go`, `events/dispatcher_test.go`) adaptados ao novo shape.
- `grep -RInE "manager\\.Manager" internal/platform/outbox/` retorna 0 após o refactor.
</requirements>

## Subtarefas

- [ ] 10.1 `internal/platform/outbox/ports.go` (criar): declarar `OutboxRepository` interface (com `Insert`, `ClaimBatch`, `MarkProcessed`, `MarkFailed`, `Reap`, `Housekeep`, conforme contrato atual exposto) + `OutboxRepositoryFactory` interface (`OutboxRepository(db database.DBTX) OutboxRepository`).
- [ ] 10.2 `internal/platform/outbox/factory.go` (criar): `repositoryFactory{o11y observability.Observability}` + `NewRepositoryFactory(o11y) OutboxRepositoryFactory` + método `OutboxRepository(db database.DBTX)`.
- [ ] 10.3 Refatorar `internal/platform/outbox/storage_postgres.go`:
  - Renomear `postgresStorage` para receber `db database.DBTX` em vez de `manager.Manager`.
  - Remover `s.db.BeginTx(ctx, ...)` interno do `ClaimBatch` e métodos correlatos — agora as operações usam `s.db.ExecContext`/`QueryContext` direto.
  - Construtor passa a ser `NewPostgresStorage(o11y observability.Observability, db database.DBTX) OutboxRepository`.
  - `database.FromContext(ctx)` (se houver) removido — tx chega via factory.
- [ ] 10.4 Refatorar `internal/platform/outbox/dispatcher.go`:
  - `DispatcherJob` ganha campos `uow uow.UnitOfWork[[]Row]` + `factory OutboxRepositoryFactory`.
  - `Run(ctx)` faz `uow.Do(ctx, func(ctx, tx) ([]Row, error) { storage := factory.OutboxRepository(tx); rows, err := storage.ClaimBatch(...); ... return rows, nil })`.
  - Dispatch dos eventos para `events.Dispatcher` continua igual; só o lifecycle de TX muda.
- [ ] 10.5 Refatorar `reaper.go` e `housekeeping.go`:
  - Ambos passam a usar `uow.UnitOfWork[struct{}]` (`uow.NewVoid`).
- [ ] 10.6 Editar `cmd/worker/worker.go`:
  - Substituir `storage := outbox.NewPostgresStorage(dbManager)` por construção do factory + UoWs.
  - `outboxFactory := outbox.NewRepositoryFactory(o11y)`.
  - `dispatcherUoW := uow.New[[]outbox.Row](dbManager, uow.WithObservability(o11y))`.
  - `reaperUoW := uow.NewVoid(dbManager, uow.WithObservability(o11y))`.
  - `housekeepUoW := uow.NewVoid(dbManager, uow.WithObservability(o11y))`.
  - Passar `(uow, factory, …)` para cada job.
- [ ] 10.7 Adaptar testes existentes:
  - `internal/platform/outbox/dispatcher_test.go` — mocks de `UnitOfWork[[]Row]` e `OutboxRepositoryFactory` (gerados via mockery genéricos).
  - Testes que dependiam de `manager.Manager` mock passam a usar callback inline.
  - `events/dispatcher_test.go` permanece inalterado (não toca storage).
- [ ] 10.8 Smoke local: subir `cmd worker` com Postgres, inserir um evento de teste manualmente, observar log de dispatch + marcação como processado.

## Detalhes de Implementação

Referenciar:
- [`techspec.md` §17](./techspec.md) — escopo completo do refactor (target, compatibilidade, validação).
- [ADR-008](./adr-008-repository-factory-per-module.md) — padrão canônico aplicado ao outbox.
- [Runbook §6 + §7 + §11](../../docs/runbooks/handler-usecase-uow-repository.md) — factory + repo + multi-repo no mesmo TX.

**Shape canônico do dispatcher:**

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
        if err != nil {
            return nil, err
        }
        for _, row := range rows {
            // dispatch + storage.MarkProcessed(ctx, row.ID) no mesmo tx
        }
        return rows, nil
    })
    // … logging
    return err
}
```

**Removido do storage:** todas as ocorrências de `s.db.BeginTx(...)`, `tx.Commit`, `tx.Rollback`, `errors.Join(opErr, tx.Rollback)` — UoW da devkit faz o trabalho.

## Critérios de Sucesso

- `go build ./...` verde.
- `go test -race -count=1 ./internal/platform/outbox/...` verde após adaptação dos testes.
- `grep -RInE "manager\\.Manager|s\\.db\\.BeginTx" internal/platform/outbox/` retorna 0 (storage agora vê `database.DBTX`).
- `grep -RInE "internal/platform/uow" internal/platform/outbox/` retorna 0 (consome devkit direto).
- Smoke local do `cmd worker` processa evento de teste sem erro.
- `cmd/worker/worker.go` mostra factory + UoWs instanciados; `dbManager` (manager.Manager) só fica no escopo do bootstrap.
- Migrações `000001_outbox_events.*` permanecem intactas no diff.
- Contrato externo (`Insert(ctx, evt, maxAttempts)`) preservado — chamadas existentes do método não quebram.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff). -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] `dispatcher_test.go` (adaptado) — mocks `UnitOfWork[[]Row]` + `OutboxRepositoryFactory`; callback executado inline; valida que `ClaimBatch + MarkProcessed` ocorrem no mesmo `tx`.
- [ ] `factory_test.go` (criar) — `NewRepositoryFactory(o11y).OutboxRepository(pool)` retorna não-nil.
- [ ] `storage_postgres_integration_test.go` (criar/adaptar — build tag `integration`): inserir evento + `ClaimBatch` + `MarkProcessed` via UoW real contra Postgres.
- [ ] Smoke local manual: `cmd worker` processa evento.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/platform/outbox/ports.go` (criar)
- `internal/platform/outbox/factory.go` (criar)
- `internal/platform/outbox/factory_test.go` (criar)
- `internal/platform/outbox/storage_postgres.go` (editar — refactor de assinatura e remoção de BeginTx interno)
- `internal/platform/outbox/dispatcher.go` (editar — campo `uow uow.UnitOfWork[[]Row]` + factory)
- `internal/platform/outbox/reaper.go` (editar)
- `internal/platform/outbox/housekeeping.go` (editar)
- `internal/platform/outbox/dispatcher_test.go` (adaptar)
- `internal/platform/outbox/storage_postgres_integration_test.go` (criar/adaptar — build tag)
- `cmd/worker/worker.go` (editar — instanciar UoWs + factory; passar para jobs)
- Dependências: `internal/platform/sqlnull` (já criado), devkit `pkg/database/uow`, `pkg/database/manager`.

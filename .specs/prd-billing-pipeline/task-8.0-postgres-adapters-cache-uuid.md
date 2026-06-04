# Tarefa 8.0: Repositórios Postgres + cache LRU + UUID generator + mapper

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar adapters de infrastructure em `internal/billing/infrastructure/`: `PgxSubscriptionRepository` com `SELECT ... FOR UPDATE` no caminho de mutação (ADR-012); `PgxWebhookEventRepository` com `InsertIfNew` idempotente e `RecordApplication` via `ON CONFLICT DO NOTHING` (ADR-009); mapper que revalida invariantes; `EntitlementLRU` wrappando `hashicorp/golang-lru/v2/expirable` (ADR-004); `UUIDGenerator` adapter `google/uuid`. Adicionar `hashicorp/golang-lru/v2` ao `go.mod`.

<requirements>
- `PgxSubscriptionRepository` implementa `SubscriptionRepository` (task 5.0); usa `database.Manager` + `pgx/v5`
- `FindActiveByUserIDForUpdate` usa `SELECT ... FOR UPDATE` (sem SKIP LOCKED — serializa) (ADR-012)
- `ListByStatusInBatch(statuses, cursorCreatedAt, cursorID, limit)` paginação estável por `(created_at, id)`
- `Upsert` trata `pgerrcode.UniqueViolation` → `ErrDuplicateActiveSubscription` (transitório, Dispatcher retenta)
- `PgxWebhookEventRepository.InsertIfNew` retorna `(false, nil)` em conflict (não é erro)
- `PgxWebhookEventRepository.RecordApplication` mesmo padrão
- Mapper `HydrateSubscription(row)` revalida via VOs (não usa `RehydrateSubscription` direto — passa por `NewBillingPeriodFor` etc.)
- `EntitlementLRU` usa `expirable.NewLRU[string, EntitlementDecision]` com TTL fixo (passado no construtor)
- `UUIDGenerator` retorna UUID v4 string via `uuid.NewString()`
- Sentinelas: `ErrSubscriptionNotFound`, `ErrDuplicateActiveSubscription`, `ErrWebhookEventNotFound`
- Sem vazar tipos pgx para fora; queries em `queries.go` como string constants parametrizadas (`$1`, `$2`)
- Cobertura ≥ 90% por arquivo (CA-04 — integration em task 10.0)
</requirements>

## Subtarefas

- [ ] 8.1 `go get github.com/hashicorp/golang-lru/v2@latest`; atualizar `go.mod`/`go.sum`.
- [ ] 8.2 `infrastructure/repositories/postgres/queries.go` com SQL constants nomeadas: `findActiveByUserIDForUpdate`, `findActiveByUserID`, `findByExternalID`, `listByStatusInBatch`, `upsertSubscription`, `insertIfNewWebhookEvent`, `findRawPayload`, `markProcessed`, `recordApplication`, `listPendingAnonymization`, `anonymize`.
- [ ] 8.3 `infrastructure/repositories/postgres/mapper.go` com `rowMapper struct{}` + `HydrateSubscription(row)` e `HydrateWebhookEvent(row)` revalidando VOs.
- [ ] 8.4 `infrastructure/repositories/postgres/subscription_repository.go` com `PgxSubscriptionRepository` + construtor recebendo `*database.Manager`, `clock.Clock`. Implementa interface da task 5.0. Trata `pgerrcode.UniqueViolation` em `Upsert`.
- [ ] 8.5 `infrastructure/repositories/postgres/webhook_event_repository.go` com `PgxWebhookEventRepository` (mesmo padrão; `InsertIfNew` e `RecordApplication` via `ON CONFLICT DO NOTHING`).
- [ ] 8.6 `infrastructure/repositories/postgres/errors.go` com sentinelas.
- [ ] 8.7 `infrastructure/cache/entitlement_lru.go` com `EntitlementLRU` wrapper de `expirable.LRU[string, output.EntitlementDecision]`. Construtor `NewEntitlementLRU(capacity int, defaultTTL time.Duration)`. Métodos `Get`, `Set`, `Invalidate`.
- [ ] 8.8 `infrastructure/id/uuid_generator.go` com `UUIDGenerator struct{}` + `NewID() string` retornando `uuid.NewString()` (v4).
- [ ] 8.9 Suites unit tests cobrindo: mapper revalida e rejeita row inválida; cache LRU hit/miss/eviction/invalidate; UUIDGenerator gera ID válido (UUID v4 parse).

## Detalhes de Implementação

Ver techspec §Cache LRU, §Modelos de Dados, ADR-001 (storage_pgx pattern), ADR-004 (LRU expirable), ADR-009 (idempotência), ADR-012 (pessimist lock SQL exato).

## Critérios de Sucesso

- `go test ./internal/billing/infrastructure/repositories/postgres/... -cover` retorna ≥ 90% (caminhos unit testáveis via mock de pgx fora do scope — main coverage vem do integration test task 10.0).
- `go test ./internal/billing/infrastructure/cache/... -cover` retorna 100% para `EntitlementLRU`.
- Test `TestEntitlementLRU_EvictsByCapacity` passa: capacity=2, insere 3 entries, primeiro é evicted.
- Test `TestEntitlementLRU_ExpiresByTTL` passa: TTL=50ms, Get após 100ms retorna `(_, false)`.
- Test `TestUUIDGenerator_ReturnsValidV4` passa: `uuid.Parse(id)` sucesso + `Version() == 4`.
- Test mapper rejeita row com `status = "INVALID"` (rejeita antes de chegar a `RehydrateSubscription` se mapper revalida).
- `golangci-lint run ./internal/billing/infrastructure/...` verde — em especial `depguard` confirma que `infrastructure/repositories/postgres/` é o único permitido a importar `pgx`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Suite `EntitlementLRUSuite`: hit, miss, set sobrescreve, invalidate remove, capacity eviction, TTL expira.
- [ ] Suite `UUIDGeneratorSuite`: gera v4 válido, 100 chamadas produzem 100 IDs distintos.
- [ ] Suite `RowMapperSuite`: row válida → `*Subscription` sem perda; row com plan_code inválido → erro; row com status inválido → erro.
- [ ] Testes de integração dos repos ficam em task 10.0 com testcontainers (escopo de IO).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/billing/infrastructure/repositories/postgres/queries.go` (novo)
- `internal/billing/infrastructure/repositories/postgres/mapper.go` (novo)
- `internal/billing/infrastructure/repositories/postgres/subscription_repository.go` (novo)
- `internal/billing/infrastructure/repositories/postgres/webhook_event_repository.go` (novo)
- `internal/billing/infrastructure/repositories/postgres/errors.go` (novo)
- `internal/billing/infrastructure/repositories/postgres/mapper_test.go` (novo)
- `internal/billing/infrastructure/cache/entitlement_lru.go` (novo)
- `internal/billing/infrastructure/cache/entitlement_lru_test.go` (novo)
- `internal/billing/infrastructure/id/uuid_generator.go` (novo)
- `internal/billing/infrastructure/id/uuid_generator_test.go` (novo)
- `go.mod`, `go.sum` (alterados — adicionar `hashicorp/golang-lru/v2`)
- Depende de: task 1.0 (schema real), task 5.0 (interfaces), task 2.0 (VOs), task 4.0 (entities)
- Referência: `internal/platform/outbox/storage_pgx.go` (padrão de query com `FOR UPDATE SKIP LOCKED`)

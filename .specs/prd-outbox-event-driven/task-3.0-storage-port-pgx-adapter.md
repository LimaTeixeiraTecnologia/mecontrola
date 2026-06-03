# Tarefa 3.0: Porta Storage + adapter PgxStorage + integration tests + mocks

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar a única camada SQL do pacote `outbox`: porta `Storage` (interface no mesmo pacote — D-14) e implementação `PgxStorage` que consome `database.DBTX` exposto pelo `Manager`/`UnitOfWork`. Todas as queries operacionais (claim com `FOR UPDATE SKIP LOCKED`, mark processed/failed/DLQ, reaper, housekeeping, stats) ficam concentradas aqui. Mocks da interface gerados via `mockery` para reuso nas tasks 5.0/6.0.

<requirements>
- RF-04: idempotência do publish via constraint UNIQUE `(event_id, subscription_name)`.
- RF-10: claim usa `SELECT ... FOR UPDATE SKIP LOCKED` em lote configurável (default 50), respeita `next_retry_at <= now()` e `ORDER BY id`.
- RF-14: múltiplas réplicas nunca processam a mesma delivery 2× — comprovar em integration.
- RF-15: cada delivery registra `status`, `attempts`, `next_retry_at`, `last_error`, `processed_at`, `dead_letter_at`, `claimed_at`, `claimed_by`.
- D-14: `Storage` interface e `PgxStorage` no mesmo pacote `outbox`.
- D-17: query do reaper usa `SELECT ... FOR UPDATE SKIP LOCKED` para evitar race com Dispatcher.
</requirements>

## Subtarefas

- [ ] 3.1 Criar `storage.go` com interface `Storage` exatamente conforme techspec (Insert/Claim/Mark/Release/Purge/Stats).
- [ ] 3.2 Criar `storage_pgx.go` implementando `PgxStorage` com construtor `NewPgxStorage(inner *databaseimpl.Manager)` ou equivalente que aceite `database.DBTX` por chamada (zero estado mutável fora da conexão).
- [ ] 3.3 Implementar `InsertEvent` e `InsertDeliveries` recebendo a `tx` do caller (sem abrir nova transação).
- [ ] 3.4 Implementar `ClaimReady(ctx, batchSize, instanceID)` com a query SQL `UPDATE ... WHERE id IN (SELECT ... FOR UPDATE SKIP LOCKED) RETURNING ...` exatamente como techspec; hidratar `Event` via `SELECT outbox_events`.
- [ ] 3.5 Implementar `MarkProcessed`, `MarkFailed`, `MarkDLQ` com transações curtas (`pgx.Tx` interno) atualizando `outbox_deliveries` e respeitando `DeliveryStatus.CanTransitionTo`.
- [ ] 3.6 Implementar `ReleaseStuck(ctx, olderThan)` com a query SQL D-17.
- [ ] 3.7 Implementar `PurgeOlderThan(ctx, olderThan)` com 2 DELETEs (deliveries finalizadas + eventos órfãos) em transação curta.
- [ ] 3.8 Implementar `Stats(ctx)` retornando `Stats{Pending: map[SubscriptionName]int64, DeadLetter: map[...], OldestPendingAt}` via `SELECT subscription_name, status, COUNT(*) GROUP BY 1,2`.
- [ ] 3.9 Gerar mocks via `task mocks` em `internal/infrastructure/outbox/mocks/storage.go` para uso futuro.
- [ ] 3.10 Criar `storage_pgx_integration_test.go` com build tag `//go:build integration`, usando `testcontainers-go/modules/postgres` e aplicando `migrations/0002_outbox.up.sql` na subida.

## Detalhes de Implementação

Ver techspec.md seções **Design de Implementação → Interfaces Chave** (assinatura completa de `Storage`), **Modelos de Dados → Query de claim**, **→ Query do reaper (D-17)**, **→ Query do housekeeping** e **Abordagem de Testes → Testes de Integração** (cenários obrigatórios).

## Critérios de Sucesso

- `go test -tags=integration ./internal/infrastructure/outbox/...` verde, < 60s na suíte.
- Cenário 1: `InsertEvent` + `InsertDeliveries` com 3 subscriptions cria 1 event + 3 deliveries; segundo insert do mesmo `(event_id, subscription_name)` retorna erro de UNIQUE.
- Cenário 2: `ClaimReady(batch=10)` com 100 deliveries `pending` e `next_retry_at <= now()` retorna 10 ordenadas por `id`, todas com `status='claimed'`, `attempts+=1`, `claimed_by=instanceID`.
- Cenário 3: 2 goroutines paralelas chamando `ClaimReady(batch=50, instanceID="a")` e `ClaimReady(batch=50, instanceID="b")` em uma massa de 100 nunca retornam o mesmo `claim.ID` (RF-14 micro-teste).
- Cenário 4: `MarkProcessed` em delivery `claimed` move para `processed`; tentar marcar `processed` em delivery já `processed` falha (transição inválida).
- Cenário 5: `ReleaseStuck(now - 10min)` em deliveries `claimed` com `claimed_at = now() - 15min` volta para `pending`.
- Cenário 6: `PurgeOlderThan(now - 90d)` apaga deliveries finalizadas e os eventos cujas deliveries todas sumiram.
- Mocks gerados: `mocks.Storage` com `NewStorage(t)` callable em testes de tasks 5.0/6.0.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: não aplicável de forma significativa para o adapter (lógica reside no SQL); foco em integration. Pode haver testes de mapeamento de erros pgx → sentinels (`uq_*` → `ErrDuplicateSubscription` quando aplicável; manter conservador).
- [ ] Testes de integração: `storage_pgx_integration_test.go` cobrindo os 6 cenários listados em "Critérios de Sucesso" com `testcontainers-go/modules/postgres` e suite `testify/suite`.

**Definition of Done**:
- [ ] Build tag `//go:build integration` aplicada corretamente — `go test ./...` (sem tag) ignora o arquivo.
- [ ] Suíte de integração roda < 60s em CI (tempo alvo da techspec).
- [ ] Concorrência interna no claim provada com 2 goroutines + assert de disjuntividade de IDs.
- [ ] Mocks gerados sem warning; `mocks.NewStorage(t)` instanciável.
- [ ] Nenhuma dependência de `events.Bus` no pacote — apenas `events.EventID` / `events.EventName`.
- [ ] `gofmt -w .` aplicado; `golangci-lint run` verde.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/infrastructure/outbox/storage.go` (novo)
- `internal/infrastructure/outbox/storage_pgx.go` (novo)
- `internal/infrastructure/outbox/storage_pgx_integration_test.go` (novo)
- `internal/infrastructure/outbox/mocks/storage.go` (gerado por mockery)
- `migrations/0002_outbox.up.sql` (consumido — criado em 1.0)
- `internal/infrastructure/database/` (dependência — `Manager`, `DBTX`)

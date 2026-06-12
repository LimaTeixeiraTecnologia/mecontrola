# Tarefa 5.0: Use cases Transaction + repo postgres + producer + integration tests

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementa os 5 use cases de `Transaction` (`Create`, `Update`, `Delete`, `Get`, `List`), o `transaction_repository.go` (com `db` como campo da struct — gate `alert_repository.go`), e o `transaction_event_publisher.go` fino. `Create` e `Update` consomem `TransactionWorkflow.Decide*`; `Delete/Get/List` são orquestradores diretos (ADR-006 §"Práticas proibidas — Decide* em CRUD trivial"). Cobre idempotência, optimistic locking, soft-delete, cursor base64.

<requirements>
- Use cases em `application/usecases/`: `create_transaction.go`, `update_transaction.go`, `delete_transaction.go`, `get_transaction.go`, `list_transactions.go`. `Create`/`Update` chamam `commands.NewXxx` → `workflow.Decide*` → `uow.Execute` → `publisher.Publish` na mesma TX.
- `Update` quando `occurred_at` muda de mês: `RefMonthsAffected = {old.RefMonth, new.RefMonth}` deduped, calculado no `Decide*` (RF-25).
- Optimistic locking via `version` (RF-10): `409 conflict` + `code=transaction_version_conflict`.
- Cursor base64 para `List` (RF-07): `base64(created_at,id)` descendente, limit padrão 50, máx 200.
- Soft-delete em `Delete` (RF-08): `UPDATE ... SET deleted_at = now WHERE id = ? AND user_id = ? AND deleted_at IS NULL`.
- `transaction_repository.go` com `db database.DBTX` como **campo da struct** (gate inegociável). Métodos: `Create`, `UpdateWithVersion`, `SoftDelete`, `GetByID`, `ListByMonth`, `SumByMonth`.
- `transaction_event_publisher.go` fino: `Publish(ctx, db, evt entities.TransactionCreated|Updated|Deleted) error` → `json.Marshal(evt)` + `outbox.NewPostgresPublisher(...).Publish(ctx, envelope)`. Nada mais.
- `category_name_snapshot`/`subcategory_name_snapshot` gravados no insert/update (RF-04).
- Filtro `user_id` obrigatório em toda query (RF-42 — multi-tenant lógico).
- Integration tests com `internal/platform/testcontainer.Postgres` (build tag `integration`).
</requirements>

## Subtarefas

- [ ] 5.1 `create_transaction.go` + unit test com mocks (factory, publisher, idempotency).
- [ ] 5.2 `update_transaction.go` + unit test cobrindo mudança de `ref_month` → 2 competências em `RefMonthsAffected`.
- [ ] 5.3 `delete_transaction.go` + unit test (soft-delete + evento).
- [ ] 5.4 `get_transaction.go` + `list_transactions.go` (cursor base64) + unit tests.
- [ ] 5.5 `infrastructure/repositories/postgres/transaction_repository.go` (db como campo) + integration test cobrindo CRUD + cursor + version conflict + soft-delete.
- [ ] 5.6 `infrastructure/messaging/database/producers/transaction_event_publisher.go` + integration test verificando que evento é gravado na MESMA TX (rollback descarta evento).

## Detalhes de Implementação

Referência: techspec "Endpoints de API" (rotas `/api/v1/transactions/*`), "Padrão de Use Case com `Decide*` puro", "Contrato Inegociável de Repositórios". RF-01 a RF-10, RF-35, RF-38, RF-41, RF-42, RF-43, RF-44, RF-45.

## Critérios de Sucesso

- `go test -race -count=1 ./internal/transactions/application/usecases/...` passa com cobertura ≥ 85% nos 5 use cases.
- `go test -race -count=1 -tags=integration ./internal/transactions/infrastructure/repositories/postgres/...` passa.
- Integration test `producer_integration_test.go`: rollback da TX agregada descarta linha do outbox.
- Cursor base64 round-trip preservando ordenação descendente.
- 409 retornado em version conflict; 404 em recurso de outro usuário.
- Nenhum método de repositório com `db` na assinatura.
- Zero comentários em `.go` de produção.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff). -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit tests dos 5 use cases com mocks.
- [ ] Integration test do `transaction_repository.go` (CRUD + cursor + version + soft-delete).
- [ ] Integration test do `transaction_event_publisher.go` (mesma TX + rollback).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/transactions/application/usecases/{create,update,delete,get,list}_transaction.go` (novos)
- `internal/transactions/application/usecases/*_test.go` (novos)
- `internal/transactions/infrastructure/repositories/postgres/transaction_repository.go` (novo)
- `internal/transactions/infrastructure/repositories/postgres/transaction_repository_integration_test.go` (novo)
- `internal/transactions/infrastructure/messaging/database/producers/transaction_event_publisher.go` (novo)
- `internal/transactions/infrastructure/messaging/database/producers/transaction_event_publisher_integration_test.go` (novo)
- `internal/transactions/application/usecases/errors.go` (sentinel errors)

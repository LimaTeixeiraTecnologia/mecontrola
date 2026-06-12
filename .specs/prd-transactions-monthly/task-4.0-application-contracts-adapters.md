# Tarefa 4.0: Application contracts — interfaces, DTOs, `CategoriesCache`, `CardLookup` adapter

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Declara todas as portas consumidas pelos use cases (12 interfaces — audit fix #5), DTOs de input/output, o adapter outbound para `internal/card` (delega a `GetCardForUser`), e o `CategoriesCache` local espelhando o padrão do `internal/budgets`. Sem implementação de use case ainda; foco em contratos finos.

<requirements>
- `application/interfaces/` com **12 interfaces** declaradas no consumidor (R6.3): `RepositoryFactory`, `TransactionRepository`, `CardPurchaseRepository`, `CardInvoiceRepository`, `RecurringTemplateRepository`, `MonthlySummaryRepository`, `RecurringMaterializationRepository`, `CardLookup`, `CategoryValidator`, `TransactionEventPublisher`, `CardPurchaseEventPublisher`, `RecurringTemplateEventPublisher`.
- `CardLookup.GetForUser(ctx, cardID, userID) (valueobjects.CardBillingSnapshot, error)` (audit fix #2 — nome único).
- Publishers recebem domain event tipado (não `EventEnvelope` genérico): `Publish(ctx context.Context, db database.DBTX, evt entities.CardPurchaseCreated) error` (e similares para os 8 outros eventos).
- DTOs de input em `application/dtos/input/`: `RawCreateTransaction`, `RawUpdateTransaction`, `RawCreateCardPurchase`, `RawUpdateCardPurchase`, `RawCreateRecurringTemplate`, `RawUpdateRecurringTemplate`. Raw = decode bruto do HTTP, sem tipos de domínio.
- DTOs de output em `application/dtos/output/`: `Transaction`, `CardPurchase` (inclui `RefMonthsAffected []string` em update/delete), `CardInvoice`, `RecurringTemplate`, `MonthlySummary`, `MonthlyEntriesPage`.
- `infrastructure/config/categories_cache.go` espelhando `internal/budgets/infrastructure/config/categories_cache.go`: `Boot(ctx)` carrega raízes oficiais; cache TTL 60s para subcategorias; implementa `CategoryValidator`.
- `infrastructure/repositories/postgres/categories_reader_adapter.go` implementando `CategoriesReader` que o cache consome (espelha o do `budgets`).
- `infrastructure/http/client/card_lookup_adapter.go` que delega a `internal/card.GetCardForUser`; converte erros para `404 card_not_found` ou `502 card_lookup_failed`.
- Atualizar `mockery.yml` com inventário nominal de 12 mocks (audit fix #5).
- Todos os repositórios obedecem ao padrão `db` como campo da struct (gate `alert_repository.go:20-27`): `Repo.Method(ctx, args)` — nunca `Repo.Method(ctx, db, args)`.
</requirements>

## Subtarefas

- [ ] 4.1 12 arquivos em `application/interfaces/*.go` com interfaces enxutas (≤ 6 métodos cada).
- [ ] 4.2 6 arquivos em `application/dtos/input/*.go` (structs Raw com tags `json:`).
- [ ] 4.3 6 arquivos em `application/dtos/output/*.go` + função `From(decision) Output`.
- [ ] 4.4 `infrastructure/config/categories_cache.go` + `Boot(ctx)` + testes unitários com mock de `CategoriesReader`.
- [ ] 4.5 `infrastructure/repositories/postgres/categories_reader_adapter.go` + integration test.
- [ ] 4.6 `infrastructure/http/client/card_lookup_adapter.go` + unit test com mock de `card.GetCardForUser` (via interface declarada localmente em `application/interfaces/`).
- [ ] 4.7 Atualizar `mockery.yml`; rodar `task mocks` para gerar mocks em `application/usecases/mocks/`.

## Detalhes de Implementação

Referência: techspec "Interfaces Chave" + "Padrão de Use Case com `Decide*` puro" + "Contrato Inegociável de Repositórios" (`db` como campo). ADR-001 (CardLookup via `GetCardForUser`); audit fix #2 e #5.

## Critérios de Sucesso

- `go build ./internal/transactions/...` compila (interfaces + DTOs prontos para uso na Task 5.0).
- `go test -race -count=1 ./internal/transactions/infrastructure/config/...` passa.
- Integration test do `categories_reader_adapter` passa com testcontainer Postgres.
- `task mocks` gera 12 mocks em `internal/transactions/application/usecases/mocks/` sem erro.
- Nenhuma interface de repositório com `db database.DBTX` em método (gate de revisão).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff). -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit test do `categories_cache.go` (Boot + cache TTL + invalidation).
- [ ] Integration test do `categories_reader_adapter.go` com testcontainer.
- [ ] Unit test do `card_lookup_adapter.go` cobrindo happy path + not found + erro de IO.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/transactions/application/interfaces/*.go` (12 arquivos novos)
- `internal/transactions/application/dtos/input/*.go` (6 arquivos novos)
- `internal/transactions/application/dtos/output/*.go` (6 arquivos novos)
- `internal/transactions/infrastructure/config/categories_cache.go` (novo)
- `internal/transactions/infrastructure/repositories/postgres/categories_reader_adapter.go` (novo)
- `internal/transactions/infrastructure/http/client/card_lookup_adapter.go` (novo)
- `mockery.yml` (modificado)

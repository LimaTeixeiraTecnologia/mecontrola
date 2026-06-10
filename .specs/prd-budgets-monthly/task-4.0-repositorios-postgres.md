# Tarefa 4.0: Repositórios Postgres + integration tests com testcontainers

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar os 5 repositórios Postgres do módulo (`BudgetRepository`, `ExpenseRepository`, `AlertRepository`, `ThresholdStateRepository`, `PendingEventRepository`) consumindo `database.DBTX` direto e cobertura integration test com `internal/platform/testcontainer`. O `ThresholdStateRepository.UpsertIfTransition` é o núcleo idempotente do avaliador (RF-60e/f). O `ExpenseRepository` consolida soft-delete + tombstone + agregação por raiz com `EXPLAIN` validando o índice parcial RT-29.

<requirements>
- Cada repositório consome `database.DBTX` em todos os métodos (R-ADAPTER-001.2).
- Sem regra de negócio em SQL além de filtros estritos; cálculos pertencem aos use cases.
- `UpsertIfTransition` em SQL único `INSERT ... ON CONFLICT ... DO UPDATE` retornando `transitioned` (boolean) via `RETURNING`/CTE. Não atualiza nada se `committed_at <= last_evaluated_committed_at` (out-of-order safe).
- `SumByRoot` agrega só raízes com despesas; use case completa com zero para raízes ausentes.
- `SoftDelete` é idempotente: chamada com versão esperada divergente retorna conflito tipado; chamada com mesma versão que já produziu tombstone retorna sucesso (RF-29e).
- Identidade canônica `(user_id, source, external_transaction_id)` UNIQUE cobre tombstone (RF-45).
- Integration tests build tag `//go:build integration`.
- Zero comentários em `.go` de produção.
</requirements>

## Subtarefas

- [ ] 4.1 `repositories/postgres/budget_repository.go` — CRUD básico + `ListFutureNotActivated` para recorrência.
- [ ] 4.2 `repositories/postgres/expense_repository.go` — `GetByIdentity` retornando expense ou tombstone; `Insert/Update/SoftDelete/SumByRoot` com `RETURNING version`.
- [ ] 4.3 `repositories/postgres/alert_repository.go` — `Insert`, `CountDelivered`, `ListForUser` com cursor.
- [ ] 4.4 `repositories/postgres/threshold_state_repository.go` — `UpsertIfTransition` em SQL único.
- [ ] 4.5 `repositories/postgres/pending_event_repository.go` — `Insert`, `ListReady(limit)` com `FOR UPDATE SKIP LOCKED`, `Transition(id, to, reason)`.
- [ ] 4.6 Integration tests por repositório com `internal/platform/testcontainer`, incluindo `EXPLAIN ANALYZE` no `SumByRoot` confirmando `Index Scan` no índice parcial.
- [ ] 4.7 `testutil_test.go` para fixtures comuns.

## Detalhes de Implementação

Ver seções **Modelos de Dados** e **Interfaces Chave** da `techspec.md`. ADRs vinculadas: [`adr-003`](./adr-003-threshold-state-tabela-dedicada.md) (estado de cruzamento), [`adr-004`](./adr-004-summary-on-demand-indice-parcial.md) (soft-delete e índice parcial).

Padrão de implementação: espelhar `internal/billing/infrastructure/repositories/postgres/*.go` (mesmo estilo de DTX, mesma estratégia de tradução de erro Postgres para erro tipado do `application/interfaces`).

## Critérios de Sucesso

- Todos os testes de integração passam com `task test:integration` (`go test -tags=integration ./internal/budgets/infrastructure/repositories/postgres/...`).
- `EXPLAIN ANALYZE SELECT root_slug, SUM(amount_cents) FROM budgets_expenses WHERE user_id=$1 AND competence=$2 AND deleted_at IS NULL GROUP BY root_slug;` usa `Index Scan` no índice composto parcial.
- `UpsertIfTransition` cobre 3 cenários: false→true, true→false, idempotente (sem transição).
- Pending events: `ListReady` retorna ordenados por `received_at` ASC com `SKIP LOCKED`.
- Tombstone bloqueia reuso de identidade canônica enquanto não houver expurgo.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/budgets/application/interfaces/{budget,expense,alert,threshold_state,pending_event}_repository.go` (novo)
- `internal/budgets/infrastructure/repositories/postgres/*.go` (novo)
- `internal/budgets/infrastructure/repositories/postgres/testutil_test.go` (novo)
- Referência: `internal/billing/infrastructure/repositories/postgres/`, `internal/categories/infrastructure/repositories/postgres/`

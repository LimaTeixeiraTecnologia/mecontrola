# Tarefa 1.0: Migration baseline + schema do módulo budgets

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar a migration baseline `000009_create_budgets_baseline` (up + down) com as 6 tabelas do módulo (`budgets`, `budgets_allocations`, `budgets_expenses`, `budgets_alerts`, `budgets_threshold_states`, `budgets_expense_events_pending`), todos os índices exigidos pela techspec — incluindo o índice composto parcial RT-29 (`(user_id, competence, subcategory_id) WHERE deleted_at IS NULL`) e o índice parcial de pendentes `WHERE state = 1` — e cobrir a aplicação/rollback no integration test agregado `migrations/migrations_integration_test.go`.

<requirements>
- Schema `mecontrola` (igual aos outros módulos).
- Constraints: `UNIQUE (user_id, competence)` em `budgets`; `UNIQUE (user_id, source, external_transaction_id)` em `budgets_expenses` cobrindo linhas com `deleted_at IS NOT NULL` (tombstone bloqueia reuso — RF-45).
- `root_slug` em `budgets_allocations` e `budgets_expenses` validado por `CHECK` listando os 5 slugs oficiais.
- `state` modelado como `SMALLINT` com convenção iota+1 (1=draft, 2=active em budgets; 1=pending/2=applied/3=failed/4=expired em pending; máquina de 5 estados em alerts).
- Down migration limpa tudo na ordem inversa de FKs.
- Sem comentários em arquivos `.go` (test de integração inclusive).
</requirements>

## Subtarefas

- [ ] 1.1 Criar `migrations/000009_create_budgets_baseline.up.sql` com as 6 tabelas + índices + checks.
- [ ] 1.2 Criar `migrations/000009_create_budgets_baseline.down.sql` em ordem inversa.
- [ ] 1.3 Adicionar caso de teste em `migrations/migrations_integration_test.go` validando apply + rollback do par.
- [ ] 1.4 Validar com `task migrate:up` local + `EXPLAIN` no índice parcial.

## Detalhes de Implementação

Ver seção **Modelos de Dados** da `techspec.md` para o DDL completo de cada tabela (linhas com `CREATE TABLE`/`CREATE INDEX`). Não duplicar aqui — o DDL canônico está na techspec.

## Critérios de Sucesso

- `task migrate:up` aplica a migration sem erros.
- `task migrate:down` reverte sem deixar objeto remanescente.
- Integration test `migrations_integration_test.go` passa.
- `EXPLAIN SELECT root_slug, SUM(amount_cents) FROM budgets_expenses WHERE user_id=$1 AND competence=$2 AND deleted_at IS NULL GROUP BY root_slug` usa `Index Scan` no índice composto parcial.

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

- `migrations/000009_create_budgets_baseline.up.sql` (novo)
- `migrations/000009_create_budgets_baseline.down.sql` (novo)
- `migrations/migrations_integration_test.go` (atualizar caso agregado)
- Referência: `migrations/000004_categories_baseline.up.sql`, `migrations/000008_create_card_cards.up.sql`

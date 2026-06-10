# Tarefa 6.0: Use cases de despesa + outbox publisher + soft-delete

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o caminho síncrono de ingestão financeira: `UpsertExpense` (criação/edição idempotente com versão monotônica) e `DeleteExpense` (soft-delete + tombstone_version), ambos transacionais via `uow.UnitOfWork[entities.Expense]` e publicando `budgets.expense.committed.v1` no outbox na **mesma transação** do commit financeiro (ADR-002, RT-24). O publisher é adapter fino sobre `internal/platform/outbox.Publisher` (R-ADAPTER-001.2).

<requirements>
- Identidade canônica `(user_id, source, external_transaction_id)` é a chave de idempotência (RF-42).
- Criação rejeita payload com `version` explícito; impõe `version=1` (RF-29c/d).
- Edição/exclusão exigem `expected_version`; conflito retorna erro tipado convertido em 409 pelo handler (RF-29a/b).
- Retry de criação por mesma identidade retorna sucesso idempotente sem nova mutação (RF-43).
- Retry com identidade em tombstone NÃO recria (RF-45); após expurgo de 24m a identidade é livre (RF-47b).
- `subcategory_id` validado via `CategoriesReader.ValidateExpenseSubcategory` antes do commit; `root_slug` desnormalizado é resolvido no commit e atualizado em edição (RF-30).
- `cutoff_competence_br` calculada no instante do commit (RT-21), serializada no payload do outbox.
- Source fixado pelo servidor: `"api"` para handler HTTP, `producer_source` validado contra allowlist para consumer externo.
- Publisher consome `internal/platform/outbox.Publisher` na mesma `tx` recebida da UoW (sem nova conexão).
- Auto-draft (`CreateOrAutoDraftForExpense` da tarefa 5.0) é invocado **dentro da mesma tx** somente quando a despesa for a primeira válida da competência (RF-12a).
- Zero comentários em `.go` de produção.
</requirements>

## Subtarefas

- [ ] 6.1 `application/dtos/input/{upsert_expense,delete_expense}_input.go`.
- [ ] 6.2 `application/dtos/output/expense_output.go`.
- [ ] 6.3 `application/interfaces/outbox_publisher.go` declarando `ExpenseCommittedPublisher`.
- [ ] 6.4 `application/usecases/upsert_expense.go` (cobre create e update) + unit tests cobrindo: criação fresca, retry idempotente, conflito de versão, edição alterando `subcategory_id` (recalcula `root_slug`), payload com `version` rejeitado, tombstone bloqueia recriação.
- [ ] 6.5 `application/usecases/delete_expense.go` + unit tests: soft-delete + tombstone_version, retry de exclusão idempotente, conflito de versão.
- [ ] 6.6 `infrastructure/messaging/database/producers/expense_committed_publisher.go` (adapter fino).
- [ ] 6.7 Integration test do publisher: rollback simultâneo da despesa + outbox.
- [ ] 6.8 Atualizar `mockery.yml`.

## Detalhes de Implementação

Ver **Interfaces Chave**, **Fluxo de Dados** (caminho síncrono) e ADR-002 (envelope canônico do evento) na `techspec.md`.

`ExpenseCommittedEnvelope` (campos obrigatórios no payload):
```
user_id, competence, subcategory_id, root_slug, mutation_kind, committed_at, cutoff_competence_br
```

O `event_id` é UUID v4 gerado por `internal/platform/id` (consistente com billing). `occurred_at` do envelope `outbox.Event` = `committed_at`.

## Critérios de Sucesso

- Cobertura unitária ≥ 85% nos use cases.
- Integration test confirma: (a) rollback da despesa rola atrás a linha do outbox; (b) commit da despesa torna a linha do outbox visível imediatamente.
- Test de retry concorrente (table-driven simulando 5 requisições paralelas com mesma identidade): apenas 1 despesa criada, demais retornam idempotente.
- Linter limpo; sem comentários.

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

- `internal/budgets/application/dtos/input/{upsert_expense,delete_expense}_input.go` (novo)
- `internal/budgets/application/dtos/output/expense_output.go` (novo)
- `internal/budgets/application/interfaces/outbox_publisher.go` (novo)
- `internal/budgets/application/usecases/{upsert_expense,delete_expense}.go` (novo)
- `internal/budgets/infrastructure/messaging/database/producers/expense_committed_publisher.go` (novo)
- `internal/budgets/infrastructure/messaging/database/producers/expense_committed_publisher_integration_test.go` (novo)
- Referência: `internal/billing/infrastructure/messaging/database/producers/subscription_event_publisher.go`

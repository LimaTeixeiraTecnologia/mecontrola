# Tarefa 3.0: Budgets — usecase `ReconcileTransactionUpdate` + Input DTO `Validate()` + mocks + testes

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar o usecase de reconciliação do orçamento a partir da edição de transação: remove a representação anterior e aplica a atual, numa UoW. Encapsula a regra de delete-antigos + upsert-atuais (ADR-001), mantendo o consumidor fino.

<requirements>
- RF-28: atualizar `budgets_expenses` pelo caminho de update (`ExpectedVersion`/`MutationKindUpdate`) reusando `UpsertExpense`/`DeleteExpense`.
- RF-29: troca de categoria move consumo; troca de valor/data ajusta consumo/competência; resumo e alertas refletem o novo estado.
- RF-30: editar competência passada respeita o cutoff existente do orçamento.
- Não-cartão: update in-place por identidade estável (`source=transactions`, `external=aggregateID`), lendo a version atual.
- Cartão: delete das `PreviousItemIDs` + upsert das parcelas atuais (`source=transactions_card`).
- Migração pix↔crédito tratada (remover representação anterior, aplicar a atual).
- Input DTO com `Validate()` (`errors.Join`, mensagens com campo, receiver pointer, puro) — R-DTO-VALIDATE-001.
- Idempotente sob reprocessamento (delete-por-identidade + upsert-por-identidade).
</requirements>

## Subtarefas

- [ ] 3.1 `ReconcileTransactionUpdateInput` + `Validate()` em `internal/budgets/application/dtos/input/`.
- [ ] 3.2 `ReconcileTransactionUpdate` usecase (`application/usecases/reconcile_transaction_update.go`) orquestrando `DeleteExpense`/`UpsertExpense` numa UoW; não-cartão in-place (ler version → update), cartão delete+upsert; migração de forma de pagamento.
- [ ] 3.3 Registrar interfaces novas no `.mockery.yml` e rodar `task mocks`.
- [ ] 3.4 Testes testify/suite (whitebox, `fake.NewProvider`, IIFE por mock) cobrindo não-cartão, cartão 3x→2x, migração pix↔crédito, competência passada (cutoff), redelivery idempotente, tombstone/conflict.

## Detalhes de Implementação

Ver `techspec.md` (Interfaces Chave — `ReconcileTransactionUpdate`; Abordagem de Testes) e `adr-001`. Reusa `UpsertExpense` (`upsert_expense.go:125-219`) e `DeleteExpense.ExecuteByExternalID` (`delete_expense.go:88-125`). Lembrar: `UpsertExpense` com existente + `ExpectedVersion==nil` é no-op — para update in-place, ler a version atual.

## Critérios de Sucesso

- Reconciliação correta e idempotente em todos os cenários; sem linha fantasma.
- DTO com `Validate()` conforme R-DTO-VALIDATE-001.
- Padrão canônico de teste (R-TESTING-001) respeitado.
- `go build`, `go vet`, `go test -race`, lint do módulo verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — modelar reconciliação com estados/erros de domínio fechados e DTO validado na fronteira.
- `design-patterns-mandatory` — gate de desenho do novo usecase (selector = reject; reusar Facade/Adapter inline).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/budgets/application/usecases/reconcile_transaction_update.go`
- `internal/budgets/application/dtos/input/reconcile_transaction_update_input.go`
- `internal/budgets/application/usecases/upsert_expense.go`
- `internal/budgets/application/usecases/delete_expense.go`
- `.mockery.yml`

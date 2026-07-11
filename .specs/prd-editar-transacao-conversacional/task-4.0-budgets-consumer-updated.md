# Tarefa 4.0: Budgets — `TransactionUpdatedConsumer` fino + registro em `module.go` + testes

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar o consumidor de `transactions.transaction.updated.v1` (adapter fino) que decodifica o envelope, filtra despesas e delega ao usecase `ReconcileTransactionUpdate`; registrar no wiring de budgets.

<requirements>
- RF-28: consumir `transactions.transaction.updated.v1` e refletir a edição no orçamento via `ReconcileTransactionUpdate`.
- RF-32: consumidor observável (spans + contadores) com cardinalidade controlada (sem `user_id`/`category_id` como label).
- Adapter fino: sem SQL, sem branching de domínio, zero comentários (R-ADAPTER-001).
- Filtro `direction=outcome`; skip com `reason` label.
</requirements>

## Subtarefas

- [ ] 4.1 `TransactionUpdatedConsumer` (`internal/budgets/infrastructure/messaging/database/consumers/transaction_updated_consumer.go`), molde `transaction_created_consumer.go`: decode `outbox.Envelope` → `transactionUpdatedPayload` → filtro → `reconcile.Execute`.
- [ ] 4.2 Contadores `budgets_transaction_updated_consumer_decode_failed_total` e `..._skipped_total{reason}`.
- [ ] 4.3 Registrar em `internal/budgets/module.go` (construção + linha `{EventType: "transactions.transaction.updated.v1", Handler: transactionUpdatedConsumer}`) e expor no retorno do módulo.
- [ ] 4.4 Testes testify/suite (whitebox, `fake.NewProvider`, IIFE por mock) para decode, filtro, delegação e contadores.

## Detalhes de Implementação

Ver `techspec.md` (Visão Geral dos Componentes; Interfaces Chave — consumidor) e `adr-001`. Reusar o molde e o padrão de registro dos consumidores `created`/`deleted` (`module.go:132-146`).

## Critérios de Sucesso

- Consumidor registrado e delegando ao usecase; nenhuma regra/SQL no consumidor.
- Gates R-ADAPTER-001.1/.2 verdes; labels sem alta cardinalidade.
- `go build`, `go vet`, `go test -race`, lint do módulo verdes.

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
- `internal/budgets/infrastructure/messaging/database/consumers/transaction_updated_consumer.go`
- `internal/budgets/module.go`
- `internal/budgets/infrastructure/messaging/database/consumers/transaction_created_consumer.go`

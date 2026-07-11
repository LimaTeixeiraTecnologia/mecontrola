# Tarefa 5.0: Integração transactions→budgets (testcontainers)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Teste de integração da cadeia real (Postgres via testcontainers): editar transação publica `updated.v1`, o consumidor reconcilia o orçamento e o read model fica coerente — inclusive parcelas e migração de forma de pagamento.

<requirements>
- RF-27: evento enriquecido percorre outbox → consumidor.
- RF-28/RF-29: `budgets_expenses`, `GetMonthlySummary` e thresholds refletem a edição.
- RF-30: edição de competência passada respeita cutoff.
- RF-24: compra parcelada editada recompõe parcelas sem consumo fantasma.
- Idempotência de redelivery do evento.
</requirements>

## Subtarefas

- [ ] 5.1 Cenário: criar transação (created) → editar valor/categoria/data (updated) → asserir `budgets_expenses`/`GetMonthlySummary` sem linha fantasma.
- [ ] 5.2 Cenário: editar compra parcelada 3x→2x → asserir remoção da parcela extinta e recomputo por competência.
- [ ] 5.3 Cenário: migração pix→crédito e crédito→pix → asserir representação correta no orçamento.
- [ ] 5.4 Cenário: redelivery do evento `updated.v1` mantém o read model idêntico.

## Detalhes de Implementação

Ver `techspec.md` (Abordagem de Testes — Integração) e `adr-001`. Usar `//go:build integration` + testcontainers-go; reusar fixtures dos testes de integração existentes de budgets/transactions.

## Critérios de Sucesso

- Todos os cenários verdes com Postgres real.
- Nenhum consumo fantasma após edição; thresholds coerentes.
- Suíte de integração executa em CI (`task test:integration` equivalente).

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
- `internal/budgets/integration/transaction_to_budget_chain_integration_test.go`
- `internal/budgets/infrastructure/messaging/database/consumers/transaction_updated_consumer.go`
- `internal/transactions/infrastructure/messaging/database/producers/transaction_event_publisher.go`

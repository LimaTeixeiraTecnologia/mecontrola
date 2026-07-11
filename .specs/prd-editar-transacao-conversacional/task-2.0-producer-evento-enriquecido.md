# Tarefa 2.0: Producer — serializar `TransactionUpdated` enriquecido (só mapeia) + teste

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Garantir que `PublishUpdated` serialize o evento `TransactionUpdated` enriquecido (campos novos da tarefa 1.0) para o envelope do outbox, sem introduzir cálculo de domínio (R-TXN-003).

<requirements>
- RF-27: o evento publicado em `transactions.transaction.updated.v1` carrega os campos novos.
- Producer apenas mapeia domain event → envelope (`EventID`/`AggregateID`/`occurred_at`/`RefMonthsAffected` vêm pré-computados do `DecideUpdate`).
- Zero comentários; sem branching de domínio; adapter fino (R-ADAPTER-001).
</requirements>

## Subtarefas

- [ ] 2.1 Confirmar que `PublishUpdated` serializa o struct enriquecido via `json.Marshal(evt)` sem lógica nova (`transaction_event_publisher.go:69-94`).
- [ ] 2.2 Teste do producer garantindo que o payload do envelope contém `category_id`, `subcategory_id`, `installments`, `previous_item_ids`.

## Detalhes de Implementação

Ver `techspec.md` (Pontos de Integração — Outbox) e `adr-001`. O struct enriquecido já serializa via tags; a tarefa é assegurar o contrato e cobri-lo com teste, sem cálculo no producer.

## Critérios de Sucesso

- Payload do envelope reflete os campos novos.
- Gate R-TXN-003 (producer sem cálculo de domínio) verde.
- `go build`, `go vet`, `go test` do pacote verdes.

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
- `internal/transactions/infrastructure/messaging/database/producers/transaction_event_publisher.go`
- `internal/transactions/infrastructure/messaging/database/producers/transaction_event_publisher_test.go`

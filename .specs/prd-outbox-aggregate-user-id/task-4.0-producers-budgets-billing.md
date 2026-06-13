# Tarefa 4.0: Atualizar producers de budgets + billing

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Atualiza `expense_committed_publisher.go` (budgets) e `subscription_event_publisher.go` (billing) para popular `AggregateUserID` em `outbox.EventInput`.

<requirements>
- RF-14 (parcial): 2 sites populam AggregateUserID
- Sem mudança de assinatura pública
- Sem comentário em `.go`
- Sem nova dep
</requirements>

## Subtarefas

- [ ] 4.1 Atualizar `internal/budgets/.../producers/expense_committed_publisher.go`: `AggregateUserID: evt.UserID.String()`.
- [ ] 4.2 Atualizar `internal/billing/.../producers/subscription_event_publisher.go`: idem. Para billing, identificar o campo `user_id` na entity de subscription (pode ser `evt.UserID` direto).
- [ ] 4.3 Confirmar que o teste existente do producer assertce `AggregateUserID == expectedUserID.String()`.

## Detalhes de Implementação

Ver techspec. Padrão idêntico aos producers de transactions.

## Critérios de Sucesso

- `go test -count=1 ./internal/budgets/infrastructure/messaging/database/producers/...` PASS.
- `go test -count=1 ./internal/billing/infrastructure/messaging/database/producers/...` PASS.
- Asserções de teste validam o campo correto.
- `task lint && task test && task vulncheck` PASS.

## Skills Necessárias

<!-- MANDATÓRIO -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Asserção `AggregateUserID` em cada producer
- [ ] Sem regressão

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/budgets/infrastructure/messaging/database/producers/expense_committed_publisher.go`
- `internal/billing/infrastructure/messaging/database/producers/subscription_event_publisher.go`
- Testes correspondentes

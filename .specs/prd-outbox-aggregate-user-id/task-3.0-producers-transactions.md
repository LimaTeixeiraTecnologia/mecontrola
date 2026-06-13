# Tarefa 3.0: Atualizar 3 producers de transactions

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Atualiza os 3 producers em `internal/transactions/infrastructure/messaging/database/producers/` para popular `AggregateUserID` em `outbox.EventInput`, lendo `UserID` direto da entity/event do domínio.

<requirements>
- RF-14 (parcial): 3 sites de transactions populam AggregateUserID
- Reuso de `evt.UserID()` ou `evt.UserID` (campo da entity) — sem reparseio do payload JSON
- Sem mudança de assinatura pública dos producers
- Sem comentário em `.go`
- Sem nova dep
</requirements>

## Subtarefas

- [ ] 3.1 Atualizar `transaction_event_publisher.go`: nos 3 métodos (Created/Updated/Deleted ou equivalente), adicionar `AggregateUserID: evt.UserID.String()` no `outbox.EventInput`.
- [ ] 3.2 Atualizar `card_purchase_event_publisher.go`: idem.
- [ ] 3.3 Atualizar `recurring_template_event_publisher.go`: idem.
- [ ] 3.4 Confirmar que os events tipados (`entities.TransactionCreated`, `entities.CardPurchaseCreated`, etc.) carregam `UserID uuid.UUID` — já validado na baseline (`internal/transactions/domain/entities/events.go:14`).
- [ ] 3.5 Testes existentes dos publishers continuam verdes; adicionar 1 asserção em cada teste que valide `AggregateUserID == evt.UserID.String()`.

## Detalhes de Implementação

Ver techspec seção "Fluxo de Dados". Publishers são adapters finos (R-ADAPTER-001.2): apenas serializam + delegam ao outbox.Publisher. Não reabrir publisher para fazer cálculo de domínio.

## Critérios de Sucesso

- `go test -count=1 ./internal/transactions/infrastructure/messaging/database/producers/...` PASS.
- Testes verificam `EventInput.AggregateUserID == expectedUserID.String()`.
- `task lint:user-isolation` PASS.
- Sem regressão em outros pacotes.

## Skills Necessárias

<!-- MANDATÓRIO -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Asserção `AggregateUserID` correto em cada um dos 3 publishers
- [ ] Sem regressão em testes existentes

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/transactions/infrastructure/messaging/database/producers/transaction_event_publisher.go`
- `internal/transactions/infrastructure/messaging/database/producers/card_purchase_event_publisher.go`
- `internal/transactions/infrastructure/messaging/database/producers/recurring_template_event_publisher.go`
- Testes correspondentes

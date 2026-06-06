# Tarefa 6.0: Outbox producer billing — publisher + structs de evento

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar a camada de emissão de eventos de domínio de billing via `internal/platform/outbox.Publisher`. Inclui os 5 structs concretos (`SubscriptionActivated`, `SubscriptionRenewed`, `SubscriptionPastDue`, `SubscriptionCanceled`, `SubscriptionRefunded`) que implementam o contrato de `events.Event` e o publisher concreto que serializa, valida UUID/payload/type e chama `outbox.Publisher.Publish` dentro da mesma transação aberta pelo use case.

<requirements>
- Implementação em `internal/billing/infrastructure/messaging/database/producers/`.
- Cada evento implementa `events.Event` (`GetEventType()`, `GetPayload() any`) e expõe `aggregate` correto (`Subscription`).
- `event.ID` é UUID gerado via `internal/platform/id` (sem `crypto/rand` local).
- `event.OccurredAt` é `time.Now().UTC()` inline (R6.7) ou o `occurred_at` do payload original quando relevante.
- Publish ocorre **dentro** da UoW do use case (não abre transação própria).
- Payload JSON estável: serialização via `json.Marshal`; campos conforme techspec §6.6.
- Sem panic; erros são wrappados com `fmt.Errorf("billing/producer: %w", err)`.
</requirements>

## Subtarefas

- [ ] 6.1 `producers/events.go`: 5 structs (`SubscriptionActivated`, `SubscriptionRenewed`, `SubscriptionPastDue`, `SubscriptionCanceled`, `SubscriptionRefunded`) com campos do payload conforme tabela §6.6.
- [ ] 6.2 `producers/subscription_event_publisher.go`: implementa `application/interfaces.SubscriptionEventPublisher` chamando `outbox.Publisher.Publish` para cada método; serializa payload para `json.RawMessage`.
- [ ] 6.3 Validação dos campos exigidos pelo `outbox.Publisher` (UUID, type não vazio, aggregate, payload válido) antes do call.
- [ ] 6.4 Unit tests cobrindo: cada `PublishX` chama `outbox.Publisher.Publish` exatamente 1× com `type` esperado e payload válido; erro do publisher é propagado wrappado.
- [ ] 6.5 Integration test (testcontainers Postgres) garantindo que, após `ProcessSaleApproved` (use case da Tarefa 5.0) executar dentro de UoW, há exatamente 1 linha em `platform_outbox_events` com `event_type='billing.subscription.activated'`.

## Detalhes de Implementação

- Contrato e regras do `outbox.Publisher` em `internal/platform/outbox/publisher.go` (já existente).
- Type names estáveis: `billing.subscription.activated|renewed|past_due|canceled|refunded` — não mudar sem ADR nova.
- `aggregate = "Subscription"` em todos; `aggregate_id = subscription.ID`.
- `occurred_at` no payload é o `occurred_at` do trigger original (do webhook ou do reconcile), separado de `event.OccurredAt` (carimbo de emissão local).
- `subscription_id`, `funnel_token`, `plan_code`, `period_start`, `period_end`, `grace_end` conforme tabela §6.6 — não acrescentar campos não previstos.

## Critérios de Sucesso

- `go build ./internal/billing/infrastructure/messaging/database/producers/...` verde.
- `go test -race -count=1 ./internal/billing/infrastructure/messaging/database/producers/...` verde.
- Integ test: linha outbox criada na mesma transação do use case (RF-10, RF-11).
- Payload JSON desserializável sem erro num decoder com schema esperado (smoke test).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit tests por método de publisher.
- [ ] Integ test confirmando row outbox criada transacionalmente após use case.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/billing/infrastructure/messaging/database/producers/{events,subscription_event_publisher}.go` + `_test.go`
- Referência: `internal/platform/outbox/publisher.go`, `internal/platform/events/events.go`.
- Referência: techspec §6.6 (tabela de eventos), §6.7 (campos).

# Tarefa 4.0: Agregados Subscription + WebhookEvent + satisfação de identity.Subscription

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar agregados em `internal/billing/domain/entities/`: `Subscription` com VOs encapsulados, transições via métodos de intenção (`Activate`/`Renew`/`MarkPastDue`/`Cancel`/`Expire`/`Refund`), construtor validador `NewSubscription`, construtor exclusivo de reidratação `RehydrateSubscription`, satisfação estrutural da interface `identity.domain.services.Subscription` (ADR-005). `WebhookEvent` entity como projeção da row `webhook_events`. Sentinelas tipados em `errors.go`.

<requirements>
- `Subscription` aggregate com campos privados; nunca exportar setters
- Métodos de intenção (verbos de domínio): `Activate`, `Renew`, `MarkPastDue`, `Cancel`, `Expire`, `Refund` — cada um delega `StateMachine.AssertLegal` antes de mutar (ADR-010)
- Construtor `NewSubscription(NewSubscriptionParams) (*Subscription, error)` valida invariantes (status inicial `Active`/`Trialing`, período válido, plan_code definido)
- Construtor `RehydrateSubscription(RehydrateSubscriptionParams) *Subscription` exclusivo para mapper (recebe status arbitrário, sem validação de transição)
- Implementa `identity.domain.services.Subscription` (`Status()`, `CurrentPeriodEnd()`, `GracePeriodEnd()`) por satisfação estrutural (sem `var _ Interface = (*Type)(nil)` — R6.4)
- `RF-17a` chargeback: método `Refund(at, amount, reason)` aceita transição direta de qualquer estado não-terminal para `Refunded`, armazena `refund_amount_cents`
- `WebhookEvent` entity com `NewWebhookEvent(NewWebhookEventParams) WebhookEvent` (value type ok, não aggregate)
- Sentinelas tipados em PT-BR (R5.10)
- Object Calisthenics #9 aplicado (sem getters mecânicos — métodos de intenção)
- Sem `init()`, sem funções top-level além de `New*` e `RehydrateSubscription` (R0/R1)
</requirements>

## Subtarefas

- [ ] 4.1 `entities/subscription_id.go` com `SubscriptionID struct{ value string }` e construtor `NewSubscriptionID(v string) (SubscriptionID, error)` rejeitando vazio.
- [ ] 4.2 `entities/subscription.go` com `Subscription struct{...}` (campos não exportados), `NewSubscriptionParams`, `NewSubscription`. Construtor valida: ID não-vazio, provider não-vazio, status `IsCreatable`, period_start < period_end.
- [ ] 4.3 Método privado `applyTransition(target, reason, at, period)` que invoca `services.NewStateMachine().AssertLegal(s.status, target)` e atualiza estado, `lastEventAt`, `updatedAt`, e `periodStart`/`periodEnd` se `period.AdvancesPeriod()`. Se `target == PastDue`, set `gracePeriodEnd = at + DefaultGracePeriod`.
- [ ] 4.4 Métodos públicos: `Activate`, `Renew`, `MarkPastDue`, `Cancel`, `Expire`, `Refund` (último aceita `MoneyBRL` e `TransitionReason`).
- [ ] 4.5 Getters de intenção: `ID()`, `UserID()`, `Status()`, `CurrentPeriodEnd()` (alias semântico de `PeriodEnd()`), `GracePeriodEnd()`, `PlanCode()`, `ExternalSubscriptionID()`, `LastEventAt()`, `SnapshotForNotification()`.
- [ ] 4.6 `entities/rehydrate.go` com `RehydrateSubscriptionParams` e `RehydrateSubscription(p) *Subscription` (sem validação de transição; uso restrito a mapper — documentar no godoc).
- [ ] 4.7 `entities/webhook_event.go` com `WebhookEvent struct{...}` value type + `NewWebhookEvent(NewWebhookEventParams) WebhookEvent` validando ID/provider/external_event_id não-vazios, payload não-vazio.
- [ ] 4.8 `domain/errors.go` consolidado: `ErrInvalidSubscriptionID`, `ErrSubscriptionRequiresID`, `ErrSubscriptionRequiresProvider`, `ErrSubscriptionInitialStatusInvalid`, `ErrSubscriptionRequiresPeriod`, `ErrWebhookEventRequiresPayload`.
- [ ] 4.9 Validar implementação implícita de `identity.domain.services.Subscription` via teste: `services.NewEntitlementChecker().IsEntitled(billingSub, now)` compila e roda (sem `var _`).

## Detalhes de Implementação

Ver techspec §Agregado `entities.Subscription` e ADR-005 (satisfação estrutural cross-module), ADR-010 (delegação ao state machine). Status method retorna tipo de `identity/domain/services` para satisfazer interface — billing importa apenas `identity/domain` (permitido por depguard).

## Critérios de Sucesso

- `go test ./internal/billing/domain/entities/... -cover` retorna `coverage: 100%`.
- `NewSubscription` rejeita status inicial `Expired`/`Refunded`/`PastDue` com `ErrSubscriptionInitialStatusInvalid`.
- `sub.Activate(at, period)` em estado `Trialing` muda status para `Active`, atualiza `lastEventAt`, atualiza `periodEnd`.
- `sub.MarkPastDue(at)` em estado `Active` muda status, preserva `periodEnd`, set `gracePeriodEnd = at + 7d`.
- `sub.Refund(at, money, ChargebackReceived)` em estado `PastDue` muda status para `Refunded`, armazena amount.
- `sub.Activate(at, period)` em estado `Refunded` retorna `ErrIllegalTransition`.
- Teste `TestSubscriptionImplementsIdentityContract`: `identity.NewEntitlementChecker().IsEntitled(sub, now)` compila e retorna bool sem panic.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Suite `SubscriptionSuite` cobrindo construtor (válido + cada falha de invariante) + cada método de transição (sucesso + transição ilegal).
- [ ] Suite `RefundSuite` table-driven cobrindo chargeback de cada estado não-terminal → `Refunded` com amount armazenado (RF-17a).
- [ ] Suite `WebhookEventSuite` cobrindo construtor válido + payload vazio + external_event_id vazio.
- [ ] Suite `IdentitySatisfactionSuite` valida que `*Subscription` é aceito por `identity.Subscription` em chamada real.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/billing/domain/entities/subscription_id.go` (novo)
- `internal/billing/domain/entities/subscription.go` (novo)
- `internal/billing/domain/entities/rehydrate.go` (novo)
- `internal/billing/domain/entities/webhook_event.go` (novo)
- `internal/billing/domain/errors.go` (novo)
- `internal/billing/domain/entities/*_test.go` (novos)
- Depende de: `internal/billing/domain/valueobjects/` (task 2.0), `internal/billing/domain/services/` (task 3.0)
- Importa: `internal/identity/domain/services` (interface Subscription) e `internal/identity/domain/entities` (UserID) — entregues por E1

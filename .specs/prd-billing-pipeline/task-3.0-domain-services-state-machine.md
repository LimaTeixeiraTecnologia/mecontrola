# Tarefa 3.0: Domain services — StateMachine + CanonicalEvent + CanonicalSubscription

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar serviços de domínio stateless em `internal/billing/domain/services/`: `StateMachine` com tabela exaustiva 6×6 de transições legais; `CanonicalEvent` e `CanonicalSubscription` como estruturas canônicas (independentes de provider); `PeriodChange` como VO de transição de período. Encapsular regra de transição centralizada e reusável pelo agregado e pelo state machine.

<requirements>
- `StateMachine` stateless (`struct{}`) com método `AssertLegal(from, to SubscriptionStatus) error`
- Switch exaustivo cobrindo 36 combinações (6 estados × 6 estados); transições legais conforme RF-17 + RF-17a
- `ErrIllegalTransition` sentinel em PT-BR
- `CanonicalEvent` carrega: `Type`, `ExternalEventID`, `ExternalSubscriptionID`, `PlanCode`, `OccurredAt`, `PeriodStart`, `PeriodEnd`, `SignupToken`, `Customer` (WhatsApp + Email), `RefundAmountCents`
- `CanonicalSubscription` carrega: `ExternalID`, `Status`, `PlanCode`, `PeriodStart`, `PeriodEnd` (formato de retorno de `FetchSubscription`)
- `PeriodChange{NewStart, NewEnd time.Time}` com método `AdvancesPeriod() bool` e factory `NoPeriodChange()`
- `DefaultGracePeriod = 7 * 24 * time.Hour` exportado como constante
- Cobertura 100% em StateMachine (CA-04)
</requirements>

## Subtarefas

- [ ] 3.1 `state_machine.go` com `StateMachine struct{}`, construtor `NewStateMachine()`, método `AssertLegal(from, to valueobjects.SubscriptionStatus) error`. Função privada `isLegalTransition(from, to)` com switch exaustivo. Constante `DefaultGracePeriod = 7 * 24 * time.Hour`.
- [ ] 3.2 Definir transições legais conforme RF-17 + RF-17a:
  - `Trialing → {Active, Expired}`
  - `Active → {PastDue, CanceledPending, Refunded}`
  - `PastDue → {Active, Expired, Refunded}`
  - `CanceledPending → {Expired, Active, Refunded}`
  - `Expired, Refunded, Unknown → ∅` (terminal)
- [ ] 3.3 `period_change.go` com `PeriodChange struct{ NewStart, NewEnd time.Time }`, método `AdvancesPeriod() bool` (true se ambos não-zero), factory `NoPeriodChange() PeriodChange`.
- [ ] 3.4 `canonical_event.go` com `CanonicalEvent struct{...}` agrupando campos obrigatórios e `CanonicalCustomer struct{ WhatsApp identityvo.WhatsAppNumber; Email string }`. Sem construtor — montado pelo `payload_mapper` (task 7.0).
- [ ] 3.5 `canonical_subscription.go` com `CanonicalSubscription struct{...}` para retorno de `FetchSubscription` (reconciliation).
- [ ] 3.6 `errors.go` com `ErrIllegalTransition = errors.New("billing: transição de estado ilegal")`.

## Detalhes de Implementação

Ver techspec §Domain Service `services.StateMachine` e ADR-010 (state machine canônica). A função `isLegalTransition` é o único helper privado sem estado aceito (exceção pragmática a R1 documentada em ADR-007 de identity). Sem `init()`.

## Critérios de Sucesso

- `go test ./internal/billing/domain/services/... -cover` retorna `coverage: 100%`.
- Tabela exaustiva 7×7 (incluindo `Unknown`) confirmada por test table-driven.
- `NewStateMachine().AssertLegal(Active, PastDue)` retorna `nil`.
- `NewStateMachine().AssertLegal(Expired, Active)` retorna `ErrIllegalTransition` (terminal).
- `NewStateMachine().AssertLegal(Refunded, qualquer)` retorna `ErrIllegalTransition`.
- `PeriodChange{NewStart: t1, NewEnd: t2}.AdvancesPeriod()` retorna `true`; `NoPeriodChange().AdvancesPeriod()` retorna `false`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Suite `StateMachineSuite` table-driven 7×7 (Unknown + 6 estados) cobrindo cada combinação (`from → to`) com resultado esperado (legal/illegal).
- [ ] Suite `PeriodChangeSuite` cobrindo `AdvancesPeriod` true/false e `NoPeriodChange` factory.
- [ ] Suite `CanonicalEventSuite` validando zero-value seguro (struct vazio não panica em uso).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/billing/domain/services/state_machine.go` (novo)
- `internal/billing/domain/services/period_change.go` (novo)
- `internal/billing/domain/services/canonical_event.go` (novo)
- `internal/billing/domain/services/canonical_subscription.go` (novo)
- `internal/billing/domain/services/errors.go` (novo)
- `internal/billing/domain/services/*_test.go` (novos)
- Depende de: `internal/billing/domain/valueobjects/` (task 2.0)
- Importa também: `internal/identity/domain/valueobjects` (`WhatsAppNumber`) — entregue por E1

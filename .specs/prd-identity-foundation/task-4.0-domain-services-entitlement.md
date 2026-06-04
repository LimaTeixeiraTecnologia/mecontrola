# Tarefa 4.0: Domain services — interface `Subscription` + `EntitlementChecker.IsEntitled`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o contrato mínimo `Subscription` como **interface** em `internal/identity/domain/services/subscription.go` (ADR-003) e o domain service `EntitlementChecker` com método puro `IsEntitled(subscription Subscription, now time.Time) bool` cobrindo as 6 transições canônicas + `nil`. Cobertura 100% obrigatória (RF-17). Sem I/O, sem cache, sem mock externo — testes usam fake local que implementa a interface.

<requirements>
- RF-13: `IsEntitled` cobre `TRIALING`, `ACTIVE`, `PAST_DUE`, `CANCELED_PENDING`, `EXPIRED`, `REFUNDED` + `subscription == nil` (retorna `false`).
- RF-14: contrato `Subscription` é interface mínima com `Status() SubscriptionStatus`, `CurrentPeriodEnd() time.Time`, `GracePeriodEnd() time.Time` — declarada em `identity/domain/services`.
- `SubscriptionStatus` enum `iota+1` com `StatusUnknown` zero-value reservado.
- RF-17 (parcial): cobertura 100% em `IsEntitled`.
- R1: `IsEntitled` é método de `EntitlementChecker` (struct vazia), não função top-level.
- Função pura: nenhuma dependência injetada (sem clock, sem repo, sem logger).
- Boundary: `now == CurrentPeriodEnd` ou `now == GracePeriodEnd` retornam `false` (uso de `now.Before(...)`).
</requirements>

## Subtarefas

- [ ] 4.1 Criar `internal/identity/domain/services/subscription.go` com `type Subscription interface { ... }` e `type SubscriptionStatus uint8` + constantes `StatusUnknown`, `StatusTrialing`, `StatusActive`, `StatusPastDue`, `StatusCanceledPending`, `StatusExpired`, `StatusRefunded`.
- [ ] 4.2 Adicionar método `String()` em `SubscriptionStatus` retornando os tokens canônicos.
- [ ] 4.3 Criar `internal/identity/domain/services/entitlement.go` com `type EntitlementChecker struct{}`, `NewEntitlementChecker()` e `(EntitlementChecker) IsEntitled(subscription Subscription, now time.Time) bool`.
- [ ] 4.4 Criar `internal/identity/domain/services/entitlement_test.go` com `EntitlementSuite` (testify/suite) table-driven cobrindo: `subscription == nil` → false; `TRIALING` com `now < CurrentPeriodEnd` → true; `TRIALING` com `now >= CurrentPeriodEnd` → false; `ACTIVE` análogo; `PAST_DUE` com `now < GracePeriodEnd` → true; `PAST_DUE` com `now >= GracePeriodEnd` → false; `CANCELED_PENDING` análogo; `EXPIRED` → false; `REFUNDED` → false; `StatusUnknown` → false; cenário de boundary exato (`now == CurrentPeriodEnd`).
- [ ] 4.5 Definir fake `fakeSubscription` no próprio `_test.go` implementando a interface (campos exportáveis para tabela).

## Detalhes de Implementação

Ver techspec §"Interfaces Chave" subseções `domain/services/subscription.go` e `domain/services/entitlement.go`. ADR-003 justifica o uso de interface (e não struct concreta) para evitar import cíclico cross-module com billing/E2.

## Critérios de Sucesso

- `go test -cover ./internal/identity/domain/services/...` reporta 100% em `IsEntitled`.
- Todos os 6 status + `nil` + boundary têm caso de teste explícito.
- `domain/services` não importa `application`, `infrastructure`, `platform` ou `configs` (depguard).
- Fake `fakeSubscription` implementa a interface compilando — validação em compile-time via `var _ services.Subscription = (*fakeSubscription)(nil)` no test file.
- `SubscriptionStatus.String()` retorna strings que casam com os tokens canônicos do PRD (`"TRIALING"`, `"ACTIVE"`, etc.).

## Definition of Done (DoD)

- [ ] `go test -cover ./internal/identity/domain/services/...` ≥ 100% em `IsEntitled` (verificável via `go tool cover -func=cover.out | grep IsEntitled`).
- [ ] Tabela de testes inclui ≥ 11 cenários (6 status × 2 boundaries + nil).
- [ ] `golangci-lint run ./internal/identity/domain/services/...` passa.
- [ ] `grep -rn '^func ' internal/identity/domain/services/*.go | grep -v '_test.go'` retorna apenas `New*` e métodos com receiver.
- [ ] `go vet ./...` passa.
- [ ] Nenhum import de `application`, `infrastructure`, `platform` em `domain/services/`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit suite table-driven cobrindo 6 status + nil + boundary.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/identity/domain/services/subscription.go` (novo)
- `internal/identity/domain/services/entitlement.go` (novo)
- `internal/identity/domain/services/entitlement_test.go` (novo)

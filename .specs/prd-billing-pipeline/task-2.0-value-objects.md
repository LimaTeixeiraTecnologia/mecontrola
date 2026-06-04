# Tarefa 2.0: Value Objects do domínio billing

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar Value Objects imutáveis em `internal/billing/domain/valueobjects/` encapsulando primitivos do domínio: `PlanCode`, `BillingPeriod`, `SubscriptionStatus`, `CanonicalEventType`, `TransitionReason`, `ExternalEventID` (cascata com hash SHA-256 fallback), `ExternalSubscriptionID`, `MoneyBRL`, `WebhookEventID`. Aplicar object calisthenics #3 (encapsular primitivos) e #9 (sem getters mecânicos); enums com `iota+1` (R5.8) e zero-value reservado a `Unknown`.

<requirements>
- VOs imutáveis (campos não exportados, sem setters)
- Construtores `New*` validam invariantes; ausência/inválido retorna sentinela
- Enums (`PlanCode`, `SubscriptionStatus`, `CanonicalEventType`, `TransitionReason`) começam em `iota+1` com `Unknown` reservado
- `BillingPeriod.NewBillingPeriodFor(PlanCode)` cobre 3 planos (30/90/365 dias)
- `ExternalEventID.NewExternalEventIDCascade(raw []byte)` extrai `id` → `order.id` → `sha256(raw)` em cascata
- `MoneyBRL` rejeita valor negativo
- Sentinelas em PT-BR conforme R5.10
- Sem `init()`, sem funções top-level que não sejam `New*` ou helpers privados sem estado (R0/R1)
- Cobertura 100% em todos os VOs (CA-04)
</requirements>

## Subtarefas

- [ ] 2.1 `plan_code.go` com `PlanCode uint8`, constantes `PlanCodeUnknown/Monthly/Quarterly/Annual`, `String()` switch exaustivo, `ParsePlanCode(s string) (PlanCode, error)` retornando `ErrUnknownPlanCode`.
- [ ] 2.2 `billing_period.go` com `BillingPeriod struct{ length time.Duration }`, `NewBillingPeriodFor(PlanCode)`, métodos `Advance(time.Time) time.Time` e `Length() time.Duration`.
- [ ] 2.3 `subscription_status.go` com enum 7 estados (Unknown + 6 canônicos), `String()`, `IsCreatable() bool` retornando true apenas para `Active`/`Trialing`.
- [ ] 2.4 `canonical_event_type.go` com enum 7 valores (Unknown + 6 canônicos: PurchaseApproved, Renewed, Late, Canceled, Refunded, Chargeback), `String()`.
- [ ] 2.5 `transition_reason.go` com VO/enum dos 7 motivos (purchase_approved, renewed, late, canceled, refunded, chargeback_received, reconciliation_sync), `String()`.
- [ ] 2.6 `external_event_id.go` com `ExternalEventID struct{ value string }`, `NewExternalEventIDCascade(raw []byte) (ExternalEventID, error)` implementando cascata `id` → `order.id` → `sha256:<hex>`, `String()`.
- [ ] 2.7 `external_subscription_id.go` com VO opaque string + validação de não-vazio.
- [ ] 2.8 `money_brl.go` com `MoneyBRL struct{ cents int64 }`, `NewMoneyBRL(cents int64)` rejeitando negativo, `Cents()`, `IsZero()`.
- [ ] 2.9 `webhook_event_id.go` com VO `WebhookEventID` (UUID v4 string), `String()`.
- [ ] 2.10 `errors.go` consolidando sentinelas `ErrUnknownPlanCode`, `ErrNegativeAmount`, `ErrEmptyPayload`, `ErrEmptyExternalSubscriptionID`, `ErrInvalidWebhookEventID` em PT-BR.

## Detalhes de Implementação

Ver techspec §Value Objects e ADR-010 (state machine). Convenção segue VOs de identity (`whatsapp_number.go`, `email.go`, `user_status.go`) — mesma estrutura `struct{ value T }` + construtor + métodos de intenção + `IsZero()`.

## Critérios de Sucesso

- `go test ./internal/billing/domain/valueobjects/... -cover` retorna `coverage: 100%`.
- `ParsePlanCode("XYZ")` retorna `(PlanCodeUnknown, ErrUnknownPlanCode)`.
- `NewBillingPeriodFor(PlanCodeAnnual).Length()` retorna `365*24*time.Hour`.
- `NewExternalEventIDCascade([]byte('{"id":"abc"}'))` retorna `ExternalEventID{value:"abc"}`.
- `NewExternalEventIDCascade([]byte('{"order":{"id":"xyz"}}'))` retorna `ExternalEventID{value:"xyz"}`.
- `NewExternalEventIDCascade([]byte('{}'))` retorna `ExternalEventID{value:"sha256:..."}` com hex de 64 chars.
- `NewMoneyBRL(-1)` retorna erro `ErrNegativeAmount`.
- Zero-value de cada enum (`PlanCode(0)`, `SubscriptionStatus(0)`, etc.) imprime `"UNKNOWN"`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Suite `PlanCodeSuite` table-driven com nomes canônicos válidos/inválidos.
- [ ] Suite `BillingPeriodSuite` cobrindo 3 planos + erro em plano desconhecido + `Advance(t).Sub(t) == Length()`.
- [ ] Suite `SubscriptionStatusSuite` cobrindo `IsCreatable` true só para Active/Trialing.
- [ ] Suite `CanonicalEventTypeSuite` + `TransitionReasonSuite` cobrindo enum value/string.
- [ ] Suite `ExternalEventIDSuite` table-driven cobrindo cascata: id presente / order.id presente / fallback hash / payload inválido (empty) → erro.
- [ ] Suite `MoneyBRLSuite` cobrindo zero, positivo, negativo.
- [ ] Fuzz test `FuzzNewExternalEventIDCascade` com corpus seed cobrindo JSONs malformados, vazios, com `id` em vários níveis; **nunca panica**.
- [ ] Fuzz test `FuzzNewMoneyBRL` com inteiros aleatórios; nunca panica.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/billing/domain/valueobjects/plan_code.go` (novo)
- `internal/billing/domain/valueobjects/billing_period.go` (novo)
- `internal/billing/domain/valueobjects/subscription_status.go` (novo)
- `internal/billing/domain/valueobjects/canonical_event_type.go` (novo)
- `internal/billing/domain/valueobjects/transition_reason.go` (novo)
- `internal/billing/domain/valueobjects/external_event_id.go` (novo)
- `internal/billing/domain/valueobjects/external_subscription_id.go` (novo)
- `internal/billing/domain/valueobjects/money_brl.go` (novo)
- `internal/billing/domain/valueobjects/webhook_event_id.go` (novo)
- `internal/billing/domain/valueobjects/errors.go` (novo)
- `internal/billing/domain/valueobjects/*_test.go` (novos)
- `internal/billing/domain/valueobjects/external_event_id_fuzz_test.go` (novo)
- `internal/billing/domain/valueobjects/money_brl_fuzz_test.go` (novo)
- Referência: `internal/identity/domain/valueobjects/whatsapp_number.go` (padrão VO)

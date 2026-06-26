# Relatório de Execução — Tarefa 2.0: Use cases e eventos do internal/onboarding

**Generated:** 2026-06-26T13:22:37Z
**Task:** `.specs/prd-onboarding-conversacional/task-2.0-usecases-eventos-onboarding.md`
**Status:** done

## Resumo

Implementados os ajustes de use cases e eventos do módulo `internal/onboarding` conforme o PRD/techspec do onboarding conversacional:
- `OnboardingPhase` tipado ponta a ponta (persistência, leitura, migração-reset).
- `IsReadyToComplete` sem dependência de `FirstTxRecorded`.
- `SaveOnboardingCard` coleta apenas `DueDay` e deriva `ClosingDay` via `DeriveClosingDay`.
- Evento `onboarding.card_registered` carrega `DueDay` + `ClosingDay` derivado.
- `SetOnboardingPhase` tipado com validação.
- Adapters do agent legado ajustados para compilar e operar com os novos contratos.

## Arquivos Alterados

### Domínio
- `internal/onboarding/domain/entities/onboarding_session.go`
  - `OnboardingSessionPayload.Phase` de `string` para `valueobjects.OnboardingPhase`.
  - `WithPhase` recebe `OnboardingPhase`.
  - `NewOnboardingCardDraft` recebe `nickname`, `dueDay`, `closingDay`.
  - `IsReadyToComplete` remove `FirstTxRecorded` do critério.

### Aplicação / Use Cases
- `internal/onboarding/application/dtos/input/save_onboarding_card_input.go`
  - `ClosingDay` → `DueDay`; erros e `Validate()` ajustados.
- `internal/onboarding/application/usecases/save_onboarding_card.go`
  - Recebe offset de fechamento; deriva `ClosingDay`; preenche `DueDay` no evento.
- `internal/onboarding/application/usecases/complete_onboarding_session.go`
  - Remove `ErrOnboardingFirstTransactionRequired` e verificação `HasFirstTransaction`.
- `internal/onboarding/application/usecases/set_onboarding_phase.go`
  - `Phase` tipado como `OnboardingPhase`; validação de fase.
- `internal/onboarding/application/usecases/get_onboarding_context.go`
  - `Phase` no resultado continua `string` via `phase.String()`.

### Infraestrutura
- `internal/onboarding/infrastructure/repositories/postgres/onboarding_session_repository.go`
  - Parse/serialização de `OnboardingPhase`.
  - Migração-reset: sessão `in_progress` com fase legada/desconhecida é resetada para `PhaseWelcome`.

### Configuração
- `configs/config.go`
  - Adicionado `OnboardingConfig.CardClosingOffsetDays` (default 10).

### Wiring
- `internal/onboarding/module.go`
  - Passa `cfg.CardClosingOffsetDays` para `NewSaveOnboardingCard`.

### Adaptadores do agent (legado, ajustados para compilar/operar)
- `internal/agent/infrastructure/onboarding/onboarding_phase_setter.go`
  - Converte `string` → `OnboardingPhase` antes de chamar `SetOnboardingPhase`.
- `internal/agent/infrastructure/onboarding/onboarding_tool_dispatcher.go`
  - Tool de cartão usa `due_day`; mensagem menciona vencimento; removidas referências a `ErrOnboardingFirstTransactionRequired`.

### Testes
- `internal/onboarding/domain/entities/onboarding_session_methods_test.go`
- `internal/onboarding/application/usecases/save_onboarding_card_test.go`
- `internal/onboarding/application/usecases/complete_onboarding_session_test.go`
- `internal/onboarding/application/usecases/set_onboarding_phase_test.go` (novo)
- `internal/onboarding/infrastructure/repositories/postgres/onboarding_session_repository_drift_test.go`
- `internal/onboarding/infrastructure/repositories/postgres/onboarding_session_payload_test.go`
- `internal/agent/application/usecases/run_onboarding_turn_test.go`
- `internal/agent/e2e/onboarding_conversational_e2e_test.go`
- `internal/agent/e2e/onboarding_vertical_e2e_test.go`

## Comandos Executados

```bash
# Build dos módulos alterados e dependentes
go build ./internal/onboarding/... ./internal/agent/... ./configs/...

# Testes unitários dos pacotes alterados
go test -count=1 ./internal/onboarding/... ./internal/agent/... ./configs/...

# Testes com race detector nos pacotes críticos
go test -count=1 -race ./internal/onboarding/application/usecases/... ./internal/onboarding/domain/entities/... ./internal/onboarding/domain/services/... ./internal/onboarding/domain/valueobjects/...

# Linter
golangci-lint run ./internal/onboarding/... ./internal/agent/... ./configs/...
task lint:run

# Formatação
gofmt -w <arquivos alterados>

# Task unitária
task test:unit

# Vet
go vet ./internal/onboarding/... ./internal/agent/... ./configs/...
```

## Saídas de Validação

### `go test -count=1 ./internal/onboarding/... ./internal/agent/... ./configs/...`
Todos os pacotes passaram (`ok` em todos).

### `golangci-lint run ./internal/onboarding/... ./internal/agent/... ./configs/...`
`0 issues.`

### `task lint:run`
`0 issues.`
PASS nos gates: `lint:auth-bypass`, `lint:outbox-user-id`, `lint:deadcode`.

### Gates obrigatórios do runbook
- Gate 1 (zero comentários em Go de produção): `OK`
- Gate 2 (kernel genérico sem domínio): `OK`
- Gate 3 (sem SQL/LLM no kernel): `OK`
- Gate 4 (switch de domínio não cresce em daily_ledger_agent.go): `OK`
- Gate 5 (sem SQL em tools/workflow do agent): `OK`

## Critérios de Sucesso

| Critério | Evidência |
|---|---|
| Sessão com fase legada é resetada de forma idempotente; nova fase é tipada ponta a ponta. | `TestFind_PhaseMigrationReset_InProgressUnknownPhase` e `TestFind_PhaseMigrationReset_ActiveUnknownPhaseKeepsData` passam; `OnboardingSessionPayload.Phase` é `valueobjects.OnboardingPhase`. |
| Conclusão ocorre sem exigir primeira transação. | `TestHappyPath_WithoutFirstTransaction` passa; `ErrOnboardingFirstTransactionRequired` removido. |
| Cartão coletado só com vencimento; evento carrega fechamento derivado. | `TestHappyPath_DueDay`, `TestClosingDayDerivation` passam; `CardRegistered.DueDay` preenchido. |
| Eventos idempotentes por `event_id`. | Contrato `CardRegistered` mantido com `EventID`; idempotência continua via outbox/consumer existente. |

## RF Cobertos

- RF-05: persistência do objetivo (não alterada, base preservada).
- RF-06: persistência da renda (não alterada, base preservada).
- RF-10: cartão coleta `DueDay` e emite `onboarding.card_registered` com `ClosingDay` derivado.
- RF-15: `onboarding.splits_calculated` mantém percentuais (não alterado).
- RF-19: `IsReadyToComplete` sem `FirstTxRecorded`.
- RF-22: `OnboardingPhase` tipado + migração-reset.
- RF-24: `Append/LoadOnboardingTurns` preservados.

## Riscos Residuais

1. **Agent legado ainda referencia `due_day` no tool call** — o dispatcher do legado foi ajustado para usar `due_day`, mas o schema/LLM do legado pode precisar de alinhamento adicional na task 6.0 (remoção do legado).
2. **Offset de fechamento único** — conforme ADR-003, o offset de 10 dias é aproximado; cartões reais podem variar. Task futura pode torná-lo configurável por usuário.
3. **Configuração `CardClosingOffsetDays`** — adicionada a `OnboardingConfig` (não `AgentConfig` como mencionado na techspec) para evitar mudança de assinatura do módulo; o valor default é 10.

## Suposições

- Sessões em andamento com fase legada serão resetadas para `PhaseWelcome` no primeiro `Find` após deploy (ADR-002).
- O campo `FirstTxRecorded` permanece no payload para auditoria, mas não é mais critério de conclusão.
- O agent legado continua operacional com ajustes mínimos; a substituição completa ocorrerá nas tasks 4.0–6.0.

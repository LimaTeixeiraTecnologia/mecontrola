# Tarefa 6.0: [onboarding] Lifecycle — turnos + MarkWelcomeSent + CompleteOnboardingSession

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar/ajustar, em `internal/onboarding`, os usecases de ciclo de vida que operam sobre
`onboarding_sessions`: `AppendOnboardingTurn`, `LoadOnboardingTurns`, `MarkWelcomeSent` (idempotente)
e o hardening de `CompleteOnboardingSession` (gravar `completed_at` + limpar `recent_turns` no mesmo
`uow.Do` + publicar `onboarding.completed`). Expor via binding.

<requirements>
- RF-20: histórico bounded (3 pares) isolado em `onboarding_sessions`.
- RF-23/24/25: conclusão com pré-requisitos, write transacional, evento após persistência.
- RF-29: `MarkWelcomeSent` idempotente (`alreadySent`).
- RF-35: ao concluir, `recent_turns` limpo.
- ADR-001, ADR-002.
</requirements>

## Subtarefas

- [ ] 6.1 `AppendOnboardingTurn` (bound 3 pares via `WithAppendedTurn`) e `LoadOnboardingTurns`.
- [ ] 6.2 `MarkWelcomeSent` idempotente (retorna `alreadySent` quando `welcome_sent_at` já setado).
- [ ] 6.3 `CompleteOnboardingSession`: `WithCompletion(now)` (state=active + completed_at + limpa turns) + publish no mesmo `uow.Do`.
- [ ] 6.4 Expor os usecases via binding; testes unitários (suite testify) de cada um.

## Detalhes de Implementação

Ver techspec.md → "Interfaces Chave" (usecases de histórico/marcos) e "Conclusão determinística e
drift". `CompleteOnboardingSession` mantém `AlreadyActive` (idempotente) e
`ErrFirstTransactionRequired`.

## Critérios de Sucesso

- Append nunca excede 3 pares; `MarkWelcomeSent` é idempotente.
- Conclusão é atômica (persistência + evento no mesmo `uow.Do`) e limpa `recent_turns`.
- **Contrato honrado**: `BudgetAllocation` exige exatamente 5 categorias, soma == income (exata),
  sem duplicata (techspec "Contratos de Validação"); a conclusão só ocorre com 1ª transação (`ErrOnboardingFirstTransactionRequired`).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem). go-implementation (linguagem, auto) aplica-se.

## Testes da Tarefa

- [ ] Testes unitários (suite testify: append bound; welcome idempotente; conclusão sucesso/AlreadyActive/ErrFirstTransactionRequired/falha publish)
- [ ] Testes de integração (T12 — isolamento, retomada, conclusão determinística)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Definition of Done (DoD)

- [ ] 4 usecases implementados e expostos via binding.
- [ ] Conclusão grava `completed_at`, limpa turns e publica evento na mesma transação.
- [ ] Zero comentários no `.go` de produção; sem acesso a `agent_sessions`.
- [ ] `go build ./internal/onboarding/...` e `go test ./internal/onboarding/application/usecases/... -count=1` passam.

## Critérios de Aceite (validações executáveis)

```bash
go build ./internal/onboarding/... && \
go test ./internal/onboarding/application/usecases/... -count=1
grep -rn "noop.NewProvider" internal/onboarding/application/usecases/*_test.go && echo FAIL || echo OK
```

## Arquivos Relevantes
- `internal/onboarding/application/usecases/append_onboarding_turn.go`, `load_onboarding_turns.go`, `mark_welcome_sent.go` (novos)
- `internal/onboarding/application/usecases/complete_onboarding_session.go` (modificado)
- `internal/onboarding/application/binding/` (expor usecases)

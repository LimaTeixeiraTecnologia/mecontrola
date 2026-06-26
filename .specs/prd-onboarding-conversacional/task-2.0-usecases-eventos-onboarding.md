# Tarefa 2.0: Use cases e eventos do internal/onboarding

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adequar o módulo `internal/onboarding` ao oficial: fase tipada com migração-reset, conclusão sem primeira transação, coleta de vencimento e eventos correspondentes.

<requirements>
- `onboarding_sessions.payload.Phase` passa a persistir/ler `OnboardingPhase` (via `String()`/`Parse`); migração-reset ao carregar sessão `in_progress` com fase legada/desconhecida → reinicia em PhaseWelcome (ADR-002, RF-22).
- `IsReadyToComplete()` sem `FirstTxRecorded`: `Objective != "" && IncomeCents > 0 && len(CustomSplit) == 5` (RF-19).
- `complete_onboarding_session` sem `ErrOnboardingFirstTransactionRequired` no caminho; remover `MarkFirstTransactionRecorded` do gate de conclusão.
- `SaveOnboardingCard` coleta `Nickname` + `DueDay` (vencimento, 1..31); `ClosingDay` derivado via `DeriveClosingDay` (RF-08, RF-10).
- Evento `onboarding.card_registered` carrega `DueDay` + `ClosingDay` derivado; `onboarding.splits_calculated` mantém percentuais do domínio (RF-15).
- `SetOnboardingPhase` tipado; `Append/LoadOnboardingTurns` preservados (RF-24).
- Inputs com `Validate()` (R-DTO-VALIDATE-001); VOs com smart constructors.
</requirements>

## Subtarefas

- [ ] 2.1 Trocar `Phase string` por `OnboardingPhase` no payload + migração-reset na carga.
- [ ] 2.2 `IsReadyToComplete` sem `FirstTxRecorded`; ajustar `complete_onboarding_session`.
- [ ] 2.3 `SaveOnboardingCard` coletar vencimento; derivar fechamento; ajustar DTO/Validate.
- [ ] 2.4 Ajustar evento `card_registered` (DueDay + ClosingDay derivado).
- [ ] 2.5 `SetOnboardingPhase` tipado; remover caminho de 1ª transação.

## Detalhes de Implementação

Ver `techspec.md` → "Modelos de Dados", "Pontos de Integração" e ADR-002/ADR-003. Manter `Decide*`/derivação na 1.0; aqui só orquestração `parse→validate→decide→persist→publish`.

## Critérios de Sucesso

- Sessão com fase legada é resetada de forma idempotente; nova fase é tipada ponta a ponta.
- Conclusão ocorre sem exigir primeira transação.
- Cartão coletado só com vencimento; evento carrega fechamento derivado.
- Eventos idempotentes por `event_id`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários (testify/suite, whitebox, `fake.NewProvider()`, mocks por IIFE — R-TESTING-001): `SaveOnboardingCard` (vencimento+derivação), `complete_onboarding_session` (sem 1ª tx), parse/serialização de fase, migração-reset.
- [ ] Testes de integração — cobertos na 9.0 (persistência/eventos com Postgres).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/onboarding/domain/entities/onboarding_session.go` (Phase, IsReadyToComplete)
- `internal/onboarding/application/usecases/save_onboarding_card.go`, `complete_onboarding_session.go`, `set_onboarding_phase.go`
- `internal/onboarding/application/dtos/input/save_onboarding_card_input.go`
- `internal/onboarding/domain/entities/onboarding_session_events.go`

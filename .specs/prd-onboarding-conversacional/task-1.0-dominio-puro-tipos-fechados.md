# Tarefa 1.0: Domínio puro e tipos fechados (DMMF state-as-type)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar a fundação determinística do onboarding: tipos fechados de estado e as funções `Decide*` puras, sem IO. É a base de todas as demais tarefas.

<requirements>
- `OnboardingPhase` (tipo fechado, 8 constantes: PhaseWelcome…PhaseConclusion) com `String()`, `ParseOnboardingPhase()`, `IsValid()` — substitui `string` livre (RF-22).
- `OnboardingAwaiting` (AwaitingNone|AwaitingText|AwaitingConfirm) e `CorrectionTarget` (none|objective|budget|cards|values) como tipos fechados.
- `DeriveClosingDay(dueDay, offsetDays) int` — puro, wrap 1..31 (RF-08).
- `Decide*` puros: objetivo, orçamento, cartões, valores, resumo; classificação resposta-da-etapa × comando-diário × dúvida; validação soma(valores)==renda (RF-13, RF-14); semântica confirmar/cancelar/corrigir/reprompt (RF-17); clarify por etapa (RF-07, RF-26); desvio diário → OutcomeDeferred (RF-25).
- Sem IO, sem `context.Context`, sem `time.Now()` interno (receber `now`/`ids` quando preciso). Testável sem mock.
</requirements>

## Subtarefas

- [ ] 1.1 `OnboardingPhase` em `internal/onboarding/domain/valueobjects/onboarding_phase.go` (smart constructor/parse).
- [ ] 1.2 `OnboardingAwaiting` e `CorrectionTarget` em `internal/agent/application/workflow/onboarding_state.go`.
- [ ] 1.3 `DeriveClosingDay` em `internal/onboarding/domain/services/card_closing.go`.
- [ ] 1.4 `Decide*` em `internal/agent/application/workflow/onboarding_decide.go` (todas as etapas + classificação + validação soma + semântica de confirmação/correção).
- [ ] 1.5 Outcomes fechados (`OutcomeAdvance|OutcomeClarify|OutcomeDeferred|...`) tipados.

## Detalhes de Implementação

Ver `techspec.md` → "Interfaces Chave" (OnboardingPhase/OnboardingState/DeriveClosingDay) e ADR-002/ADR-003/ADR-004. Seguir DMMF (state-as-type, `Decide*` puro) e R-AGENT-WF-001.3 (tipos fechados, nunca string livre).

## Critérios de Sucesso

- Todos os tipos de estado são fechados, com `String()`/`Parse`/`IsValid`; nenhuma `string` livre em assinatura pública.
- `Decide*` puros e determinísticos; cobertura de bordas (soma≠renda, entrada ambígua, comando diário, confirm/cancel/correção/reprompt, wrap de dia 1..31).
- Sem IO/`context.Context`/`time.Now()` interno.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários (domínio puro, sem mock): parse/serialização de `OnboardingPhase`; `DeriveClosingDay` (offsets, bordas, wrap); cada `Decide*` (advance/clarify/deferred/confirm/correct/reprompt; soma==renda e mismatch).
- [ ] Testes de integração — N/A (domínio puro).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/onboarding/domain/valueobjects/onboarding_phase.go` (novo)
- `internal/onboarding/domain/services/card_closing.go` (novo)
- `internal/agent/application/workflow/onboarding_state.go` (novo)
- `internal/agent/application/workflow/onboarding_decide.go` (novo)

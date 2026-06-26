# Tarefa 5.0: ETAPA 7 (Resumo + gate HITL) e ETAPA 8 (Conclusão)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar a jornada: resumo consolidado com gate de confirmação durável e correção guiada por LLM, conclusão sem primeira transação, e desvio de comandos diários durante o onboarding.

<requirements>
- ETAPA 7: resumo consolidado exibindo valor + percentual por categoria + "Está tudo certo?"; gate HITL durável com `AwaitingConfirm` (tipo fechado) reusando primitivos do kernel (ADR-004, RF-16, RF-18).
- Correção guiada por LLM: identifica campo (objetivo/orçamento/cartões/valores) e novo valor, atualiza via use case e re-exibe; ambíguo → pergunta qual campo; reprompt único (RF-17).
- ETAPA 8: conclui após confirmação, sem exigir primeira transação; mensagem de conclusão com exemplos de uso diário; emite `onboarding.completed` (RF-19, RF-20).
- Desvio de comando diário durante o onboarding → `OutcomeDeferred` (redireciona gentilmente, não registra) (RF-25).
- Resume aplica o texto via merge-patch sobre o snapshot (fonte única de verdade); sem side-store.
</requirements>

## Subtarefas

- [ ] 5.1 `newSummaryStep`: render do resumo (valor+%) + suspensão `AwaitingConfirm`.
- [ ] 5.2 Decisão de confirmação/correção/reprompt (usa `Decide*` da 1.0) + correção via use cases.
- [ ] 5.3 `newConclusionStep`: conclusão sem 1ª tx + exemplos + `completed`.
- [ ] 5.4 Desvio de comando diário (`OutcomeDeferred`) integrado aos steps.

## Detalhes de Implementação

Ver `techspec.md` → "Interfaces Chave" (resume), ADR-004 (gate HITL + desvio) e ADR-001 (conclusão sem 1ª tx). Reusar suspend/resume do kernel; nada de LLM no kernel.

## Critérios de Sucesso

- Resumo mostra valor + percentual; gate só avança com confirmação explícita.
- Correção por fala natural atualiza o campo correto e re-exibe o resumo.
- Conclusão ocorre sem primeira transação e emite `onboarding.completed`.
- Comando diário no meio do fluxo não registra nada e redireciona.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — gate HITL (pending step/AwaitingApproval) e ciclo de confirmação no `internal/agent` reusando primitivos do kernel.

## Testes da Tarefa

- [ ] Testes unitários (testify/suite, mocks por IIFE): summary suspende com `AwaitingConfirm`; confirmar conclui; corrigir atualiza campo; ambíguo re-pergunta; reprompt único; conclusão sem 1ª tx; desvio diário não registra.
- [ ] Testes de integração — fluxo de resume coberto na 9.0.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/application/workflow/onboarding_steps_summary.go`, `onboarding_steps_conclusion.go` (novos)
- `internal/agent/application/workflow/onboarding_decide.go` (confirmação/correção/desvio — da 1.0)
- `internal/onboarding/application/usecases/complete_onboarding_session.go`

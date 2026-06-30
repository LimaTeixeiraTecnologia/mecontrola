# Tarefa 6.0: Onboarding workflow durável de 8 etapas (fases fechadas, suspend/resume)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o onboarding obrigatório de 8 etapas como `workflow.Definition[OnboardingState]` durável, com `OnboardingPhase` como tipo fechado, suspend/resume por merge-patch, mensagens via `agent.Stream` e parse das respostas por `StructuredContract`. Conclui com objetivo na working memory + orçamento ativo.

<requirements>
- ADR-002 (workflow durável), ADR-007 (I/O LLM sancionado), ADR-008 (timezone/competência).
- Distribuição das 5 categorias em **mensagem única** fechando 100% (RF-14); cartão coleta **só apelido+vencimento** com defaults Name=apelido/Limit=0/ClosingDay=DueDay (RF-15/RF-15.2); recorrência 12m (RF-16.1); reuso de estado pré-existente (RF-15.1).
- Cobre: RF-10, RF-11, RF-11.1, RF-12, RF-13, RF-13.1, RF-14, RF-15, RF-15.1, RF-15.2, RF-16, RF-16.1, RF-17, RF-18, RF-19, RF-19.1, RF-28, RF-30.1.
</requirements>

## Subtarefas

- [ ] 6.1 `OnboardingPhase` (8 constantes fechadas) e `OnboardingState` (objetivo, renda, cartões, alocações, recorrência, ResumeText) — smart constructors.
- [ ] 6.2 `Sequence` dos 8 steps; `Durable:true`; suspend após cada pergunta; resume por merge-patch **antes do parse**.
- [ ] 6.3 Step `goal` grava objetivo na working memory (RF-28, um objetivo principal); step `income` coleta renda (RF-13.1).
- [ ] 6.4 Step `methodology`+`distribution`: apresenta as 5 categorias e coleta a distribuição em **mensagem única**; `Decide*` puro valida 100%; se falhar, reapresenta tudo (RF-14).
- [ ] 6.5 Step `cards`: coleta só apelido+vencimento; defaults para obrigatórios; reuso via `ListCards` (RF-15/15.1/15.2).
- [ ] 6.6 Step `summary`+`conclusion`: resumo, confirmação ativa o orçamento; pergunta recorrência (12m); exemplos de uso diário.
- [ ] 6.7 Mensagens via `agent.Stream`; parse das respostas via `llm.StructuredContract[T]` (`Strict:true`).
- [ ] 6.8 `ResolveOnboardingOrAgent`: deriva "onboardado" (orçamento ativo + objetivo na WM) e roteia onboarding × operação (RF-30.1); dúvida intermediária responde e retoma a fase exata (RF-18).

## Detalhes de Implementação

Ver techspec.md → "OnboardingState/OnboardingPhase" e ADR-002/007/008. Efeitos via bindings (2.0); regra de domínio em `Decide*` puro; sem IO no step de decisão.

## Critérios de Sucesso

- `OnboardingPhase` fechado (DMMF state-as-type); `Decide*` puro e determinístico (sem IO/context/time).
- Retomada na fase exata após dúvida/dias (suspend/resume durável); sem reinício (RF-18/RF-19/RF-19.1).
- Distribuição em mensagem única fecha 100% (`budget.go:132-150`); cartão com defaults aplicados.
- LLM só nas call-sites sancionadas (Stream/parse); sem LLM no kernel (R-WF-KERNEL-001/R-AGENT-WF-001.4).
- Conclusão: objetivo na WM + orçamento ativo (RF-17); reuso de estado pré-existente (RF-15.1).
- Zero comentários; build/gofmt/governança verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — Workflow/Step durável, Thread→Run, WorkingMemory e Structured Output do substrato do agente.

## Testes da Tarefa

- [ ] Testes unitários: cada `Decide*` puro; transições de fase; distribuição 100% (sucesso/falha→reapresenta); recorrência 12m; reuso de estado.
- [ ] Testes de integração (testcontainers): suspend/resume persistindo `Snapshot`; merge-patch antes do parse; conclusão grava WM + ativa orçamento.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go` (substitui workflow weather)
- Depende de bindings (2.0) e tools/idempotência (4.0)
- techspec.md (OnboardingState), ADR-002/007/008

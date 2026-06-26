# Tarefa 4.0: Steps ETAPAS 1–6 e OnboardingWorkflow no kernel

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Construir o workflow durável das primeiras seis etapas oficiais sobre o kernel `internal/platform/workflow`, com suspend/resume por etapa e condução por LLM no tom oficial.

<requirements>
- `OnboardingWorkflow` monta `Definition[OnboardingState]` com `Sequence` das etapas e `Engine[OnboardingState].Start/Resume` (ADR-001).
- ETAPA 1: boas-vindas + handshake "Vamos começar?" (suspende `AwaitingText`, aguarda "Sim"); sem pedir objetivo nem apresentar categorias (RF-04).
- ETAPA 4: cartões com self-loop ("outro cartão?") e caminho "não uso" (RF-09).
- ETAPA 5: apresentação das 5 categorias + "Faz sentido?" (RF-11, RF-12).
- ETAPA 6: coleta valor por categoria, uma a uma, sem auto-sugestão (RF-13).
- Interpreter de onboarding: LLM faz parse da entrada e gera a mensagem no tom oficial (exceção sancionada R-AGENT-WF-001.4); LLM nunca no kernel (R-WF-KERNEL-001.5).
- Steps finos `adapter → binding → usecase`; clarify por etapa no tom oficial (RF-26).
- Suspensão/resume via primitivos do kernel; estado em `OnboardingState` (snapshot = fonte do resume).
</requirements>

## Subtarefas

- [ ] 4.1 `OnboardingWorkflow`/`BuildOnboardingDefinition` + montagem da `Sequence`.
- [ ] 4.2 Interpreter de onboarding (parse + render no tom oficial) reusando a cadeia LLM existente.
- [ ] 4.3 Steps: welcome(+handshake), objetivo, orçamento.
- [ ] 4.4 Step cartões com self-loop e "não uso" (emite `card_registered`).
- [ ] 4.5 Steps categorias (apresentação) e valores (um a um, validação soma==renda → `splits_calculated`).

## Detalhes de Implementação

Ver `techspec.md` → "Interfaces Chave" (Step/Workflow), "Design de Implementação" e ADR-001/ADR-004. Reusar combinadores `Sequence`/`Branch` do kernel; `Decide*` da 1.0; use cases da 2.0.

## Critérios de Sucesso

- As 8 etapas são distintas e na ordem oficial (1–6 aqui); sem "Etapa X/4".
- Cada etapa suspende com a pergunta e retoma com a resposta (resume durável).
- Cartões em laço e "não uso" funcionam; valores coletados um a um; sem auto-sugestão.
- Mensagens no tom oficial geradas via interpreter; nenhuma chamada LLM no kernel.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — adiciona Workflow/Tool/Step e ciclo de execução no `internal/agent` sobre o kernel; mapeia conceitos Mastra ao código real.

## Testes da Tarefa

- [ ] Testes unitários (testify/suite, whitebox, `fake.NewProvider()`, mocks por IIFE): cada step (primeira entrada suspende; entrada válida persiste via mock e avança; inválida re-suspende; laço de cartões; "não uso"); interpreter mockado.
- [ ] Testes de integração — resume durável coberto na 9.0.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/application/workflow/onboarding_workflow.go` (novo)
- `internal/agent/application/workflow/onboarding_steps_*.go` (novos)
- `internal/agent/infrastructure/onboarding/` (interpreter/binding)
- `internal/platform/workflow/` (consumo de Engine/Sequence — sem alterar o kernel)

# Tarefa 7.0: Working memory e limpeza de turns

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Consolidar a working memory na conclusão do onboarding e disciplinar o histórico de turns.

<requirements>
- Consumer de `onboarding.completed` consolida a working memory do usuário (markdown: objetivo, renda, cartões, distribuição) e disponibiliza no system prompt da operação diária; ausência de WM não é erro (RF-21).
- `recent_turns` são limpos na conclusão do onboarding (RF-24).
- Idempotência por `event_id` no consumer (RF-28 herdado).
- WorkingMemory permanece exclusiva do `internal/agent` (R-AGENT-WF-001.8/.8-A).
</requirements>

## Subtarefas

- [ ] 7.1 Consumer `onboarding.completed` → consolida WM (markdown) via use case dedicado.
- [ ] 7.2 Limpeza de `recent_turns` na conclusão.
- [ ] 7.3 Garantir idempotência e ausência-tolerante de WM.

## Detalhes de Implementação

Ver `techspec.md` → "Decisões internas (QT-07)" e "Monitoramento". A WM é construída no agent; os dados vêm de `GetOnboardingContext`.

## Critérios de Sucesso

- Após `onboarding.completed`, a WM reflete objetivo/renda/cartões/distribuição.
- `recent_turns` zerados na conclusão.
- Reprocessamento do evento não duplica WM.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — WorkingMemory (escopo resource) e injeção no system prompt no `internal/agent`.

## Testes da Tarefa

- [ ] Testes unitários (testify/suite, mocks por IIFE): consumer consolida WM; limpeza de turns; idempotência; WM ausente não é erro.
- [ ] Testes de integração — consolidação pós-`completed` coberta na 9.0.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/infrastructure/messaging/database/consumers/onboarding_completed_consumer.go`
- `internal/agent/application/prompting/` (WM no system prompt)
- `internal/onboarding/application/usecases/` (limpeza de turns na conclusão)

# Tarefa 7.0: Agente + system prompt + scorers + memória/histórico/roteamento

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Montar o `MeControlaAgent` (`BuildMeControlaAgent`): system prompt com identidade/tom/emojis/regras de comunicação, tools de operação diária (4.0), gate destrutivo (5.0) e onboarding (6.0). Definir os scorers mínimos do MVP e garantir injeção de working memory + histórico e o roteamento onboarding × operação.

<requirements>
- ADR-006 (scorers), ADR-001 (identidade do produto, molde weather).
- Identidade/tom/emojis/regras (RF-06..RF-09), promessa/valor (RF-05), substituição (RF-01).
- Working memory do objetivo no system prompt (RF-28), janela de histórico ~20 (RF-29), persistência robusta (RF-30), Run auditável (RF-37), roteamento derivado (RF-30.1).
- Cobre: RF-01, RF-05, RF-06, RF-07, RF-08, RF-09, RF-28, RF-29, RF-30, RF-30.1, RF-37, RF-39.
</requirements>

## Subtarefas

- [ ] 7.1 `BuildMeControlaAgent(provider, tools, hooks, o11y)`: system prompt em pt-br com identidade de parceiro financeiro, tom (simples/direto/amigável/motivacional), emojis oficiais, regras de comunicação (uma pergunta por vez, perguntar só o que falta).
- [ ] 7.2 Registrar o agente único no `AgentRegistry` (sem `switch intent.Kind`).
- [ ] 7.3 `BuildMeControlaScorers(provider)`: tool-call-accuracy (code), completeness (code), categorization (LLM-judged), amostragem configurável; `ScoringHooks`.
- [ ] 7.4 Garantir injeção de working memory (objetivo) + histórico (~20) no system prompt (reuso de `runtime.buildMessages`); confirmar escopo `resourceID=user_id`.
- [ ] 7.5 `ResolveOnboardingOrAgent` integrado ao `HandleInbound`: ordem `confirmação destrutiva → onboarding → operação`; Run auditável por interação (RF-37).

## Detalhes de Implementação

Ver techspec.md → "Visão Geral dos Componentes", ADR-006/ADR-001. Reusa o molde `BuildWeatherAgent`/`BuildWeatherScorers`; LLM só nas call-sites sancionadas.

## Critérios de Sucesso

- System prompt reflete identidade/tom/emojis/regras (RF-06..RF-09) sem linguagem bancária/jurídica/fria.
- Roteamento por registry (sem `switch intent.Kind`); Thread-first; Run auditável com `thread_id`/`run_id`/`status`/`duration_ms` (RF-37).
- Scorers persistem resultados; assíncronos, fora do caminho principal (RF-39).
- Working memory (objetivo) e histórico injetados (RF-28/29/30).
- Zero comentários; go-implementation R0–R7; DMMF; build/gofmt/governança verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — construção do Agent, Scorers, WorkingMemory e ciclo Thread→Run sobre `internal/platform`.

## Testes da Tarefa

- [ ] Testes unitários: scorers code-based; montagem do agente; roteamento onboarding×operação.
- [ ] Testes de integração / `RUN_REAL_LLM`: scorer LLM-judged (categorization) e tool-calling no chain real.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/agents/mecontrola_agent.go` (substitui agent.go weather)
- `internal/agents/application/scorers/` (novos scorers)
- `internal/agents/application/usecases/` (`ResolveOnboardingOrAgent`, `HandleInbound`)
- techspec.md (Componentes), ADR-006/ADR-001

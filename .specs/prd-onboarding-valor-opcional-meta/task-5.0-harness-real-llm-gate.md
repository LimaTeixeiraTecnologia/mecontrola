# Tarefa 5.0: Harness real-LLM (gate de merge ≥ 0.90 em gpt-4o-mini)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar o harness de casos rotulados que executa o `BuildGoalStep` com LLM real e valida a extração combinada meta+valor, computando um ratio de acerto e exigindo ≥ 0.90 no modelo de produção. Este é o gate de merge da funcionalidade.

<requirements>
- RF-14: harness real-LLM valida a extração combinada nos três cenários (valor junto / ausente / inválido) e nos formatos de RF-09, com acerto ≥ 0.90 como gate de merge; medição em `openai/gpt-4o-mini`.
- RF-09: os 5 formatos monetários exercitados como casos rotulados.
- ADR-003: modelo fixado em `gpt-4o-mini`; modelos mais fortes não satisfazem o gate; assert estrito (anti-falso-positivo).
</requirements>

## Subtarefas

- [ ] 5.1 Criar `internal/agents/application/agents/onboarding_goal_value_realllm_test.go` (`//go:build integration`, `package agents`), gate `RUN_REAL_LLM=1` + `OPENROUTER_API_KEY` via `buildRealLLMProvider(t)` (reuso de `mecontrola_agent_realllm_test.go:26`); modelo default `openai/gpt-4o-mini` (override `AGENT_HARNESS_MODEL`).
- [ ] 5.2 Tabela de casos rotulados (mín. os 8 da ADR-003): 3 cenários + 5 formatos, com `wantGoalPresent`/`wantValueCents`.
- [ ] 5.3 Chamar `workflows.BuildGoalStep(a)` diretamente (padrão de `onboarding_methodology_realllm_test.go:83`); computar `hits/total` (acerto conjunto meta E valor) e `require.GreaterOrEqual(ratio, 0.90)`.
- [ ] 5.4 Log por caso (goal, valueCents, esperado, ok) e log do modelo (`AGENT_HARNESS_MODEL`) para diagnóstico.

## Detalhes de Implementação

Ver `techspec.md` seção "Testes de Integração" (esqueleto do harness) e `adr-003-gate-real-llm-gpt4o-mini.md` (tabela de casos e contrato do gate). Depende do `BuildGoalStep` da 3.0 e dos schemas/prompts da 2.0. Se o ratio ficar <0.90 por instabilidade do `hasAmount`, reforçar instruction-by-example no prompt da 2.0 (ADR-001/R3) — nunca subir o modelo para passar.

## Critérios de Sucesso

- `RUN_REAL_LLM=1 OPENROUTER_API_KEY=... go test -tags integration ./internal/agents/application/agents/ -run GoalValue` atinge ratio ≥ 0.90 reproduzível em `gpt-4o-mini`.
- Assert estrito por caso (ex.: "1,5 milhão" → 150000000), sem tolerância que mascare falha.
- Sem o env gate, o teste é `Skip` (não quebra o build padrão).
- Zero comentários no arquivo de teste (exceto `//go:build`).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — harness real-LLM sobre `agent.Agent`/`BuildGoalStep`, provider OpenRouter e gate de acerto de extração (eval).
- `go-testing` — estrutura de teste tabular, gate de env e assert de ratio.

## Testes da Tarefa

- [ ] Testes unitários (não aplicável — esta tarefa É o teste)
- [ ] Testes de integração (harness real-LLM ≥ 0.90 — gate de merge RF-14)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/agents/onboarding_goal_value_realllm_test.go` — arquivo novo (harness).
- `internal/agents/application/agents/mecontrola_agent_realllm_test.go` — `buildRealLLMProvider` (reuso).
- `internal/agents/application/agents/onboarding_methodology_realllm_test.go` — padrão de chamada de step (referência).
- `internal/agents/application/workflows/onboarding_workflow.go` — `BuildGoalStep` (sob teste).

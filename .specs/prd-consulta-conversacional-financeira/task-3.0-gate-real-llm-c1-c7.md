# Tarefa 3.0: Testes de regressão real-LLM C1–C7 (gate M-04 ≥ 0.90)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar os cenários C1–C7 ao harness real-LLM de seleção de ferramenta e validar o gate de
produção `M-04 ≥ 0.90` com zero alucinação. Cobre a verificação comportamental do roteamento e das
cadeias C4 (`resolve_card`→`query_card_invoice`) e C5 (`query_month`→`get_transaction`). É o gate de
aceite da feature (métrica de sucesso do PRD).

<requirements>
- RF-01..RF-09: cada cenário C1–C7 seleciona a(s) ferramenta(s) correta(s); C1 é multi-tool.
- RF-06a: C5 renderiza subcategoria quando presente; degrada sem erro quando ausente.
- RF-10: nenhuma resposta contém valor ausente do retorno das tools (anti-alucinação).
- RF-32a: cadeia C4 usa `cardId` vindo de `resolve_card`, nunca de texto do usuário.
- Métrica de sucesso do PRD: `M-04 ≥ 0.90` nos cenários; não-regressão dos 22 cenários existentes.
</requirements>

## Subtarefas

- [ ] 3.1 Adicionar cenários C1–C7 ao slice `scenarios []harnessScenario` em `internal/agents/application/scorers/mecontrola_tools_realllm_test.go:225`, com inputs verbatim das personas C1–C7 e `expectedTool` por cenário.
- [ ] 3.2 Implementar asserter de conjunto de tools (`expectedTools []string`, helper de teste) para C1 (`query_month` + `query_plan`), sem inflar o denominador de M-04.
- [ ] 3.3 Estender `mecontrola_agent_chain_realllm_test.go` para asseverar as cadeias C4 (`resolve_card`→`query_card_invoice`) e C5 (`query_month`→`get_transaction`) e a ausência de alucinação na resposta final.
- [ ] 3.4 Rodar com `RUN_REAL_LLM=1` + credenciais `OPENROUTER_*` do `.env`; confirmar `M-04 ≥ 0.90` e não-regressão dos cenários pré-existentes.

## Detalhes de Implementação

Ver techspec.md, seção "Testes E2E / Real-LLM (gate de aceite)". Guard vigente: `//go:build integration`
+ `RUN_REAL_LLM=1` + `OPENROUTER_API_KEY` (`mecontrola_agent_realllm_test.go:1-31`). Gate:
`require.GreaterOrEqual(t, m04, 0.90)` (`mecontrola_tools_realllm_test.go:264-268`).

## Critérios de Sucesso

- Cenários C1–C7 presentes no harness; C1 valida presença de `query_month` e `query_plan`.
- `M-04 ≥ 0.90` no log do harness; nenhum cenário pré-existente regride.
- Cadeias C4/C5 asseveradas; resposta final sem valor fora do retorno das tools.
- `go test -tags integration -race -count=1 ./internal/agents/application/scorers/... ./internal/agents/application/agents/...` verde com `RUN_REAL_LLM=1`.
- Zero comentários nos arquivos de teste tocados que violem R-ADAPTER-001.1 (exceções de teste conforme regra).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — cenários de scorer/evals e cadeias sobre o substrato de agent (`scorer.RunSample`, `ExpectedToolScorer`, harness real-LLM do consumidor).

## Testes da Tarefa

- [ ] Testes unitários (asserter de conjunto de tools — helper determinístico)
- [ ] Testes de integração (harness real-LLM `//go:build integration` + `RUN_REAL_LLM=1`; gate M-04 ≥ 0.90)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/scorers/mecontrola_tools_realllm_test.go` (cenários C1–C7 + asserter — modificado)
- `internal/agents/application/agents/mecontrola_agent_chain_realllm_test.go` (cadeias C4/C5 — modificado)
- Dependências: Tarefa 1.0 (campo subcategoria) e Tarefa 2.0 (instruções C1–C7)

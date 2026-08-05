# Tarefa 8.0: Golden set áudio/texto e suites reais por flag

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar validação de regressão zero com golden set pareado texto/áudio, negativos obrigatórios, modo simulado de transcrição e suite real STT por flag. Nenhum RF dependente de provider real pode ser marcado como atendido apenas por mock.

<requirements>
- Cobrir RF-30, RF-31, RF-32, RF-34, RF-35, RF-36 e RF-44.
- Incluir ao menos 3 casos positivos por grupo: despesas, receitas, consultas, edições e confirmações existentes.
- Incluir negativos: ruído, fala cortada, idioma não PT-BR, duplicado, formato inválido e timeout STT.
- Exigir score por grupo `>= 0.90`.
- Exigir falso-sucesso mutacional `0`.
- Exigir 0 tool call e 0 `HandleInbound` em `TranscriptionUncertain`.
</requirements>

## Subtarefas

- [x] 8.1 Ler harness golden real existente e padrões de scorer/evals.
- [x] 8.2 Adicionar modo de transcrição simulada para testar fluxo sem OpenRouter real.
- [x] 8.3 Criar golden pareado texto/audio por categoria de capacidade textual.
- [x] 8.4 Criar negativos obrigatórios de áudio.
- [x] 8.5 Criar suite real STT por `RUN_REAL_STT=1` e credencial OpenRouter.
- [x] 8.6 Criar assert explícito de falso-sucesso `0`, tool call `0` e `HandleInbound` `0` em incerteza.

## Detalhes de Implementação

Referenciar `techspec.md` nas seções `Abordagem de Testes`, `Golden Set Audio` e `Gates de Validacao para Merge`.

Evidências de codebase a respeitar:
- `internal/agents/application/golden/harness_realllm_test.go:25`
- `internal/agents/application/golden/harness_realllm_test.go:31`
- `internal/agents/application/scorers/`
- `.specs/prd-agente-audio-openrouter/stt-lote-30-results.json`

## Critérios de Sucesso

- Golden textual existente continua verde.
- Golden áudio/texto cobre todas as categorias exigidas.
- Suite real STT fica skipada sem flag e falha explicitamente com flag sem credencial.
- Nenhum mock substitui RF que exige comportamento real com OpenRouter.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — A tarefa valida o consumidor agentivo, tools, workflows, scorers e runtime.
- `domain-modeling-production` — Casos positivos/negativos precisam refletir estados e invariantes fechados.
- `design-patterns-mandatory` — A validação deve evitar harness excessivamente abstrato sem ganho real.

## Testes da Tarefa

- [x] `go test -race -count=1 ./internal/agents/application/golden/...`
- [x] `RUN_REAL_LLM=1 RUN_REAL_STT=1 STT_REAL_AUDIO_FIXTURE=<audio-ptbr-real> go test -tags=integration -count=1 ./internal/agents/application/golden/...`
- [x] Teste de modo simulado sem chamada real ao OpenRouter.
- [x] Teste de falso-sucesso mutacional `0`.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/golden/`
- `internal/agents/application/scorers/`
- `internal/agents/application/tools/`

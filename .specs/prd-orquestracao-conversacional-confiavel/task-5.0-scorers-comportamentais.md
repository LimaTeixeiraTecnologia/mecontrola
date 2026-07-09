# Tarefa 5.0: Scorers comportamentais + captura de args

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Medir comportamento verificável (não presença de palavras) via scorers code-based, mantendo os 3 atuais
por continuidade de baseline. Corrige o descarte de args em `ScoringHooks.AfterTool`, pré-requisito dos
scorers que inspecionam argumentos.

<requirements>
- RF-29: manter os 3 scorers atuais (`tool-call-accuracy`, `completeness`, `categorization`).
- RF-30: adicionar scorers comportamentais code-based: `expected_tool` (golden), `required_args`,
  `no_hallucination`, `verbatim_required`, `whatsapp_format`, `no_internal_terms`, `no_empty_answer`,
  `no_duplicate_write`, `month_reference_correctness`.
- RF-31: promoção/rollback usa ambos os conjuntos.
- RF-32: observabilidade sem conteúdo sensível (run_id, scorer_id, score, etc.).
- RF-34: resultados rastreáveis por `run_id` sem expor mensagem em métrica.
- RF-19/RF-20/RF-21: `month_reference_correctness` verifica `monthRefKind` presente/consistente
  (nomeado sem ano ⇒ `named_without_year`) e mês por extenso; preserva a resolução existente.
</requirements>

## Subtarefas

- [ ] 5.1 `ScoringHooks.AfterTool`: preencher `ToolCallRecord.Args` a partir do `argsJSON` hoje
  descartado (`_ []byte`).
- [ ] 5.2 Scorers intrínsecos (prod, `AlwaysSample`): `no_empty_answer`, `whatsapp_format`,
  `no_internal_terms`, `verbatim_required`, `no_duplicate_write`, `no_hallucination`, `required_args`,
  `month_reference_correctness`.
- [ ] 5.3 Scorer oracle-dependente `expected_tool` (usa `RunSample.Metadata["expected_tool"]`), usado só
  pelo harness golden (não registrado em prod).
- [ ] 5.4 Registrar os intrínsecos em `BuildMeControlaScorers` mantendo os 3 atuais.

## Detalhes de Implementação

Ver `adr-004-scorers-comportamentais.md` e `techspec.md` → "Scorers comportamentais — intrínsecos vs
oracle". Interfaces: `Scorer{ID, Kind, Score(ctx, RunSample)}` (`scorer.go:59`); `RunSample{Input,
Output, ExpectedOutput, ToolCalls[]{ID,Name,Args}, Metadata}`; `ScoreResult{Score, Reason, Metadata}`.
`AfterTool(ctx, _, toolID string, argsJSON []byte, err error)` hoje descarta `argsJSON`. Runner é async,
persiste em `platform_scorer_results` (sem mudança de schema).

## Critérios de Sucesso

- 3 scorers atuais mantidos (baseline preservada); intrínsecos rodando em prod, persistidos.
- `ToolCallRecord.Args` populado; scorers de argumento funcionam.
- `expected_tool` avaliável no harness golden.
- Cardinalidade fechada nas métricas; `go build/vet/test -race` verdes; zero comentários.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — implementa scorers/evals e hooks no stack agentivo mecontrola (`internal/agents`).

## Testes da Tarefa

- [ ] Testes unitários: cada scorer intrínseco (RunSample fixo → ScoreResult); `AfterTool` captura args;
  `expected_tool` com metadata; não-regressão dos 3 scorers atuais.
- [ ] Testes de integração: persistência de `ScorerResult` (coberto/compartilhado com 7.0/8.0).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/agents/scoring_hooks.go`
- `internal/agents/application/scorers/mecontrola_scorers.go`
- `internal/agents/module.go` (registro dos scorers)

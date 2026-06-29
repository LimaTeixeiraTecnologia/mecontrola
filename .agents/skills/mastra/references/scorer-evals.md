# Scorer / Evals — `internal/platform/scorer`

Equivalente a `@mastra/core/evals`. Avalia execuções fora do caminho crítico, com amostragem e persistência
em `platform_scorer_results`. Dois modos: **code-based** (determinístico) e **LLM-judged** (via OpenRouter).

## Contrato (`scorer.go`)

```go
type Scorer interface {
    ID() string
    Kind() ScorerKind
    Score(ctx context.Context, s RunSample) (ScoreResult, error)
}

type RunSample struct { Input, Output, ExpectedOutput string; ToolCalls []ToolCallRecord; Metadata map[string]any }
type ScoreResult struct { Score float64; Reason string; Metadata map[string]any }
```

`ScorerKind` fechado: `ScorerKindCodeBased` | `ScorerKindLLMJudged` (`types.go`, com `String()`/`IsValid()`/`Parse*`).

## Scorers code-based (`code_based.go`)

- `NewToolCallAccuracyScorer(id, expectedTools)` — fração de tools esperadas que foram chamadas.
- `NewCompletenessScorer(id, requiredFields)` — fração de campos obrigatórios presentes no `Output` (JSON).

Puros, sem IO; retornam `Score` em [0,1] + `Reason` + `Metadata`.

## Scorer LLM-judged (`llm_judged.go`)

`NewLLMJudgedScorer(id, provider, instructions)` — chama `provider.Complete` com `judgeContract`
(`llm.Schema` strict: `{score: number 0..1, reason: string}`). O output é **structured output validado**
(`judgeContract.Decode` rejeita score fora de [0,1] com `ErrJudgeContractNotMet`). É a única call-site de LLM
no scorer.

## Runner assíncrono (`runner.go`)

```go
runner := scorer.NewScorerRunner(entries, resultStore, o11y, scorer.WithWorkers(n))
runner.Observe(ctx, runID, scorer.RunSample{...})   // não-bloqueante; descarta se a fila encher
runner.Shutdown(ctx)                                 // drena workers
```

- `Observe` enfileira; pool de workers (`defaultWorkers = 8`) processa fora do caminho de request.
- Sampling por scorer: `AlwaysSample()`, `NeverSample()`, `RatioSample(r)`.
- Persiste `ScorerResult{RunID, ScorerID, Kind, Score, Reason, Metadata, Sampled}` via `ResultStore.Insert`
  (`scorerpostgres.NewResultStore`).
- Métricas: `scorer_runs_total` (labels `scorer_id`, `kind`, `outcome`), `scorer_duration_seconds`.
  **Proibido** `user_id`/`run_id` como label.

## Ligando ao agente via hooks (`internal/agents/application/agents/scoring_hooks.go`)

`ScoringHooks` implementa `agent.Hooks`:

- `BeforeExecute` guarda o `input` (última mensagem do usuário) no contexto.
- `AfterTool` acumula `ToolCallRecord`.
- `AfterExecute` (se sem erro) lê `RunIDFromContext` e chama `runner.Observe(ctx, runID, RunSample{...})`.

Não altera o caminho principal: scoring é best-effort e assíncrono.

## Construir os scorers do consumidor (`application/scorers/scorers.go`)

```go
func BuildWeatherScorers(provider llm.Provider) []scorer.ScorerEntry {
    return []scorer.ScorerEntry{
        scorer.NewScorerEntry(scorer.NewToolCallAccuracyScorer("tool-call-accuracy", []string{"get-weather"}), scorer.AlwaysSample()),
        scorer.NewScorerEntry(scorer.NewCompletenessScorer("completeness", []string{"temperature", ...}), scorer.AlwaysSample()),
        scorer.NewScorerEntry(scorer.NewLLMJudgedScorer("translation", provider, instructions), scorer.AlwaysSample()),
    }
}
```

No `module.go`: `scorerRunner := scorer.NewScorerRunner(BuildWeatherScorers(provider), resultStore, o11y)` →
`scoringHooks := NewScoringHooks(scorerRunner)` → passado a `BuildXAgent(..., scoringHooks, ...)`. O
`Module.Shutdown` chama `scorerRunner.Shutdown(ctx)`.

Ver `build-new-agent.md` para o wiring completo e `llm-structured-output.md` para o contrato do LLM-judged.

# Core concepts — primitivos do substrato e o layering

Situa cada primitivo Mastra no código Go real. Duas camadas: **substrato** `internal/platform/*`
(genérico, reutilizável) e **consumidor** `internal/agents` (port weather, molde a copiar).

## Layering unidirecional (ADR-002)

```
consumidor (internal/agents, test/conformance/weather)
        │
        ▼
internal/platform/agent ──► internal/platform/tool
        │      │      └────► internal/platform/memory ──► internal/platform/llm (Embed)
        │      └───────────► internal/platform/scorer ──► internal/platform/llm (Complete)
        │
        ├──► internal/platform/llm        (Complete | Stream | Embed via OpenRouter)
        └──► internal/platform/workflow   (KERNEL — Engine[S], Step[S], Store, Codec)
```

Regra dura: o **kernel** `internal/platform/workflow` não importa nenhuma camada superior nem domínio
(R-WF-KERNEL-001.1). Camadas superiores consomem o kernel; nunca o contrário.

## Agent — `internal/platform/agent`

- `Agent` (`ports.go`): `ID()`, `Instructions()`, `Execute(ctx, Request) (Result, error)`,
  `Stream(ctx, Request) (ResultStream, error)`.
- `NewAgent(id, instructions, provider, o11y, opts...)` (`agent.go`) com `WithTools(...)` e `WithHooks(...)`.
  `Execute` roda o loop de tool-calling (`completeWithTools`, máx. `maxToolRounds = 5`); se houver
  `Decoder`, valida `resp.RawJSON` contra o contrato (`ErrContractNotMet`).
- `AgentRuntime` (`runtime.go`): fronteira de inbound. `Execute(ctx, InboundRequest) (Outcome, error)`
  resolve Thread, abre Run, monta mensagens (system+working memory+recent+user), chama `Agent.Execute`,
  persiste turnos e fecha o Run. Único ponto que orquestra Thread→Run.
- `AgentRegistry` (`registry.go`): `Register`/`Resolve(id)`.
- `WorkflowRegistry[S]`/`MutableWorkflowRegistry[S]` (`registry.go`): resolve `workflow.Definition[S]` por agentId.

## Tool — `internal/platform/tool`

- `ToolHandle` (`tool.go`): `ID()`, `Description()`, `Parameters()`, `Invoke(ctx, argsJSON) ([]byte, error)`.
- `NewTool[I, O any](id, desc string, in, out llm.Schema, exec func(ctx, I) (O, error)) ToolHandle` —
  valida o input contra o JSON Schema (lazy, `sync.Once`), faz unmarshal tipado, chama `exec`, marshal do output.
- `Registry` (`registry.go`): `Register`/`Resolve(id)`, erro `ErrToolNotFound`.
- A Tool é **adapter fino**: o `exec` delega a um client/usecase; sem regra/SQL/branching de domínio.

## Workflow / Step (kernel) — `internal/platform/workflow`

- `Step[S]` (`step.go`): `ID()`, `Execute(ctx, state S) (StepOutput[S], error)`. `StepOutput[S]` carrega
  `State`, `Status` (`StepStatus`) e `*Suspension`. Helper `NewStepFunc[S](id, fn)`.
- `Definition[S]` (`engine.go`): `ID`, `Root Step[S]`, `Durable bool`, `MaxAttempts int`.
- `Engine[S]` (`engine.go`): `Start(ctx, def, key, initial) (RunResult[S], error)` e
  `Resume(ctx, def, key, resume []byte)`. `NewEngine[S](store, o11y)`. Durável → `Snapshot`+`StepRecord`
  no `Store`; CAS por `Version` (`ErrVersionConflict`/`ErrRunConflict`); `ErrRunAlreadyExists` se já há run ativo.
- Combinators (`combinators.go`): `Sequence`, `Branch`, `Parallel(merge,...)`, `Retry(step, RetryPolicy)`.
- `Codec[S]` (`codec.go`): `Encode`/`Decode` JSON e `MergePatch(base, patch)` (RFC 7386 — `null` remove chave,
  arrays substituídos). O resume aplica o patch sobre `Snapshot.State` — nunca substitui o estado inteiro.

## Memory — `internal/platform/memory`

- `ThreadGateway.GetOrCreate(ctx, resourceID, threadID) (Thread, error)` — identidade opaca `(resourceId, threadId)`.
- `MessageStore.Append`/`Recent(ctx, threadPK, limit)` — turnos por thread; janela configurável (runtime usa 20).
- `WorkingMemory.Get`/`Upsert(ctx, resourceID, content)` — markdown por `resourceId`, injetado no system prompt.
- `SemanticRecall.Index`/`Recall(ctx, resourceID, query, embedding, k)` — RAG via pgvector.
- `MessageRole` (`types.go`): `RoleUser`/`RoleAssistant`/`RoleTool`/`RoleSystem` (tipo fechado).
- Indexação assíncrona: `NewPublishingMessageStore` publica `IndexMessagePayload` (evento
  `EventTypeEmbeddingIndex`); `infrastructure/indexer.EmbeddingIndexHandler` consome, gera embedding via
  `llm.Embed` e chama `SemanticRecall.Index`. Idempotência por `event_id` do outbox.

## Scorer / Evals — `internal/platform/scorer`

- `Scorer` (`scorer.go`): `ID()`, `Kind() ScorerKind`, `Score(ctx, RunSample) (ScoreResult, error)`.
- Code-based (`code_based.go`): `NewToolCallAccuracyScorer(id, expectedTools)`, `NewCompletenessScorer(id, fields)`.
- LLM-judged (`llm_judged.go`): `NewLLMJudgedScorer(id, provider, instructions)` — usa `judgeContract`
  com `llm.Schema` strict (score 0..1 + reason).
- `ScorerRunner` (`runner.go`): `Observe(ctx, runID, RunSample)` assíncrono (pool de workers), `Shutdown`.
  Persiste `ScorerResult` via `ResultStore`. Sampling: `AlwaysSample`/`NeverSample`/`RatioSample`.

## LLM Provider — `internal/platform/llm`

- `Provider` (`provider.go`): `Slug()`, `Complete(ctx, Request) (Response, error)`,
  `Stream(ctx, Request) (TokenStream, error)`, `Embed(ctx, texts) ([][]float32, error)`.
- OpenRouter é o **único** provider oficial (`NewOpenRouterProvider`, `openrouter.go`). Não há mais
  FallbackChain nem CircuitBreaker.
- `Request`/`Response`/`Schema`/`StructuredContract[T]`/`ToolSpec`/`ToolCall` em `types.go`.

## Run auditável — `internal/platform/agent`

- `Run` (`ports.go`): `ID`, `ThreadPK`, `ResourceID`, `ThreadID`, `AgentID`, `Status` (`RunStatus`),
  `Outcome` (`ToolOutcome`), `Error`, `StartedAt`, `EndedAt`, `DurationMs`.
- `RunStore`: `Insert`/`Update`/`Load`. Persistência em `agent/infrastructure/postgres/run_store.go`
  (tabela `platform_runs`).

## Structured output — `internal/platform/agent` + `internal/platform/llm`

- `StructuredDecoder` (`decoder.go`): `Schema()` + `Validate(raw)`. `NewDecoder[T](StructuredContract[T])`.
- Sync: `Agent.Execute` valida no fim (`ErrContractNotMet`). Stream: validação na **conclusão** do stream
  (ADR-003), nunca durante.

Ver as referências específicas por tarefa no `INDEX.yaml`.

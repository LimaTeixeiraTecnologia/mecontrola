# Thread → Run — ciclo de execução auditável (AgentRuntime)

Toda execução de inbound resolve um `Thread(resourceId, threadId)` e abre/fecha um `Run` auditável.
Implementação em `internal/platform/agent/runtime.go` (`agentRuntime.Execute`). Thread-first
(R-AGENT-WF-001.6); Run auditável (R-AGENT-WF-001.5).

## Identidade

- Chaves **opacas** `(resourceId, threadId)` — sem semântica de domínio. O consumidor decide o mapeamento
  (no port WhatsApp: `resourceId = user_id`, `threadId` = peer/canal).
- `memory.ThreadGateway.GetOrCreate(ctx, resourceID, threadID) (memory.Thread, error)`.
- `agent.RunStore.Insert`/`Update`/`Load` (tabela `platform_runs`).

## Ciclo em `AgentRuntime.Execute`

```
span "agent.runtime.execute" (attr agent_id)
  in.Validate()                                   // ErrEmptyAgentID/ResourceID/ThreadID/Message
  thread = threads.GetOrCreate(resourceID, threadID)
  run    = Run{ID: uuid.New(), ThreadPK: thread.ID, ResourceID, ThreadID, AgentID, Status: Running, StartedAt}
  runs.Insert(run)
  ctx = WithRunID(workflow.WithRuntime(ctx, in), runID)   // runtime context (DI), não persistido
  a = agents.Resolve(agentId)                     // erro → closeRun(Failed, MissingResolver)
  msgs = buildMessages(a, thread.ID, in)          // system(+working memory) + Recent(20) + user
  result = a.Execute(ctx, Request{...msgs})        // hooks BeforeExecute/AfterExecute em volta
  messages.Append(user) ; messages.Append(assistant)
  closeRun(Succeeded, Routed)                      // ou Failed/UsecaseError em erro
  return Outcome{RunID, Content, Status, Mode: Sync}
```

- `agent.RunStatus` fechado (`running|succeeded|failed`). `closeRun` grava `EndedAt`/`DurationMs` e emite métricas.
- `buildMessages` injeta WorkingMemory no system prompt quando presente (ausência não é erro).
- Hooks (`agent.Hooks`) permitem scoring assíncrono sem alterar o caminho (ver `scorer-evals.md`).

## Runtime context (DI — ADR-007)

`workflow.WithRuntime(ctx, in)` e `agent.WithRunID(ctx, runID)` injetam dependências efêmeras tipadas via
`context.Context`. Recuperáveis por `workflow.RuntimeFrom` e `agent.RunIDFromContext`. **Não** são persistidos
no estado durável.

## Métricas (cardinalidade controlada)

- `agent_runs_total{agent_id, status}`
- `agent_run_duration_seconds{agent_id}`
- `agent_tool_invocations_total{agent_id, tool}` (em `agent.go`)
- `agent_stream_total{agent_id}` (em `agent.go`)

**Proibido** `user_id`/`resource_id`/`thread_id`/`category_id` como label. Labels são enums/ids estáveis.

## Ao estender

- Não iniciar Run sem `thread_id` válido — `InboundRequest.Validate` rejeita campos vazios.
- Thread/Run/WorkingMemory são **primitivos de plataforma** (`internal/platform/agent` + `memory`); o
  consumidor os consome, não os reimplementa.
- Para execução durável multi-step, use o kernel `workflow.Engine[S]` (ver `workflow-engine.md`); o Run do
  kernel é mecanismo, distinto do Run semântico do agent (addendum R-AGENT-WF-001.6-A).

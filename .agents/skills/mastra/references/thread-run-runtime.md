# Thread → Run — ciclo de execução auditável (AgentRuntime)

R-AGENT-WF-001.5 / 001.6: toda execução resolve um `Thread(user_id, channel)` e abre/fecha um `Run`
auditável. Implementação em `application/services/agent_runtime.go`.

## Identidade Mastra

- `resourceId = user_id`, `threadId = channel`.
- `ThreadGateway.GetOrCreate(ctx, userID, channel) (entities.Thread, error)`.
- `RunGateway.Insert(ctx, run)` + `RunGateway.Finish(ctx, run)`.

## Ciclo em `AgentRuntime.Execute`

```
span "agent.runtime.execute"
  startRun:
    thread = threads.GetOrCreate(userID, channel)   // falha → degrada sem Run, segue roteando
    run    = entities.StartRun(StartRunParams{ThreadID, UserID, Channel, MessageID, AgentID})
    runs.Insert(run)                                 // status = running
  result = router.route(...)                         // resolve workflow/tool, executa
  run = run.Resolve(RunResolution{Workflow, ToolName, IntentKind})
  run = run.Finish(outcome, ok, errText)             // status = succeeded | failed
  runs.Finish(run)
  recordMetrics(...)
```

- `RunStatus` é fechado (`running|succeeded|failed`). `Finish(outcome, ok, errText)` decide o status.
- `outcomeSucceeded(outcome)` define `ok` — note que `routed, replay, fallback, clarify,
  authz_denied, policy_blocked, empty_text` contam como sucesso operacional do Run.
- Falha ao resolver Thread/inserir Run **não** aborta o roteamento: degrada para execução sem Run
  persistido (`hasRun=false`), ainda emitindo métricas.

## Atributos mínimos de auditoria

`thread_id`, `run_id`, `workflow`, `tool`, `status`, `duration_ms`, `outcome`, `kind`, `error`
(quando houver). Escritas referenciam `decision_id`.

## Métricas (cardinalidade controlada)

Emitidas em `recordMetrics`:
- `agent_runs_total{agent_id, channel, workflow, status}`
- `agent_run_duration_seconds{agent_id, channel, workflow}`
- `agent_tool_invocations_total{tool, outcome}`

**Proibido** `user_id`/`category_id` como label. Labels são enums fechados.

## Ao estender

- Novo workflow → mapear o kind em `workflowFor()`/`toolFor()` para que Run e métricas recebam o
  `workflow`/`tool` corretos.
- Nunca iniciar Run sem `thread_id` válido (`StartRun` rejeita `uuid.Nil`).
- `ThreadGateway`/`RunGateway` são exclusivos do `internal/agent` — não replicar em outro módulo.

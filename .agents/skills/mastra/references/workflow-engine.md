# Workflow engine (kernel) — Step, Engine, combinators, suspend/resume

O kernel `internal/platform/workflow` é o mecanismo genérico de orquestração durável. **Puro**: sem
import de domínio nem de camada superior, sem regra/SQL/branching/LLM (R-WF-KERNEL-001). Consumidores
montam `Step[S]` sobre seu próprio estado `S`.

## Step e StepOutput (`step.go`)

```go
type Step[S any] interface {
    ID() string
    Execute(ctx context.Context, state S) (StepOutput[S], error)
}

type StepOutput[S any] struct {
    State   S
    Status  StepStatus      // StepStatusCompleted | Suspended | Failed | Skipped
    Suspend *Suspension     // {Reason SuspendReason, Prompt string} quando Suspended
}
```

Helper: `NewStepFunc[S](id, func(ctx, S) (StepOutput[S], error))`. Um step que retorna `error` é tratado
como `StepStatusFailed` pelo engine; retornar `Status` zero equivale a `Completed`.

## Definition e Engine (`engine.go`)

```go
type Definition[S any] struct { ID string; Root Step[S]; Durable bool; MaxAttempts int }

type Engine[S any] interface {
    Start(ctx context.Context, def Definition[S], key string, initial S) (RunResult[S], error)
    Resume(ctx context.Context, def Definition[S], key string, resume []byte) (RunResult[S], error)
}
```

`NewEngine[S](store, o11y)`. `RunResult[S]` carrega `RunID`, `Status` (`RunStatus`), `State`, `*Suspend`.

- **Durable=true**: persiste `Snapshot` + `StepRecord` no `Store`. Antes de iniciar, `Load` verifica run
  ativo; se houver `Running`/`Suspended`, retorna `ErrRunAlreadyExists` (idempotência). Save usa CAS por
  `Version` → `ErrVersionConflict`/`ErrRunConflict` em corrida.
- **Durable=false**: execução em memória, sem store; `Resume` é no-op (`no_run_found`).

## Combinators (`combinators.go`)

- `Sequence(id, steps...)` — encadeia; para no primeiro `Suspended`/`Failed`. É o `Root` típico e o engine
  o trata com cursor (retoma do passo suspenso).
- `Branch(id, decide func(S) string, routes map[string]Step[S])` — roteia por chave derivada do estado.
- `Parallel(id, merge func(base S, results []S) S, steps...)` — concorrência com cancelamento cooperativo;
  primeiro `Suspended`/`Failed` cancela os demais; sucesso aplica `merge`.
- `Retry(step, RetryPolicy{MaxAttempts, BaseBackoff, MaxBackoff})` — backoff exponencial com jitter,
  cancelável por `ctx`.

## Suspend / Resume + MergePatch (`codec.go`)

Um step pede pausa retornando `Status: StepStatusSuspended` e `Suspend: &Suspension{Reason: SuspendAwaitingInput, Prompt: "..."}`.
O engine salva o `Snapshot.State` e o `Cursor`. Para retomar:

```go
res, err := engine.Resume(ctx, def, key, []byte(`{"ResumeText":"sim"}`))
```

`Resume` aplica o payload como **JSON merge-patch (RFC 7386)** sobre `Snapshot.State` — nunca substitui o
estado inteiro. `Codec.MergePatch`: chave com `null` remove; arrays são substituídos; objetos são mesclados
recursivamente. Resume vazio (`len(resume)==0`) é no-op. O `Snapshot.State` é a **fonte única de verdade**.

## Estados fechados (state-as-type)

`RunStatus` (`running|suspended|succeeded|failed`), `StepStatus` (`completed|suspended|failed|skipped`),
`SuspendReason` (`awaiting_input`) — todos tipos fechados com `String()`/`IsValid()`/`Parse*`. Nunca string
solta em assinatura pública. Ver `state-as-type.md`.

## Métricas (cardinalidade controlada — R-WF-KERNEL-001.4)

Labels permitidos: `workflow`, `step`, `status`, `outcome`. **Proibido** `user_id`, `correlation_key`,
`category_id`. Contadores/histogramas: `workflow_runs_total`, `workflow_run_duration_seconds`,
`workflow_steps_total`, `workflow_step_duration_seconds`, `workflow_suspend_total`, `workflow_resume_total`,
`workflow_version_conflict_total`.

## Proibições no kernel
- Importar `internal/{transactions,billing,identity}` ou `internal/platform/{agent,memory}`.
- Regra/branching de domínio, SQL fora de `infrastructure/postgres/`, LLM, comentários.
- Substituir `Snapshot.State` inteiro no resume (use `MergePatch`).

## Uso pelo consumidor
Ver `BuildWeatherWorkflow` em `internal/agents/application/workflows/workflow.go` e o exemplo em
`build-new-agent.md`. O estado `S` é a struct do consumidor (ex.: `WeatherState`), serializada por `Codec[S]`.

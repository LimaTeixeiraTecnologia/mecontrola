<!-- spec-hash-prd: 61bccc0e72f8f494c8210df6ddd040df9c6cd71878e4cb904d0d4ea9221146a9 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Workflow Kernel reutilizável

## Resumo Executivo

Esta techspec materializa o PRD `prd-workflow-kernel` (spec-version 2): um **kernel de workflow
genérico, durável e reutilizável** em `internal/platform/workflow`, inspirado no Mastra (Step,
control-flow, suspend/resume, run auditável) porém **sem qualquer dependência de domínio**. O kernel
expõe `Step[S]` genérico sobre um estado tipado `S` e os combinadores Core — `Sequence`, `Branch`,
`Parallel`, `Retry` — mais um `Engine[S]` que executa, persiste snapshot (apenas para runs de
escrita/suspensíveis), aplica lock otimista por versão, retry por passo com falha terminal
determinística e observabilidade por passo. A persistência reusa `uow`, `database.DBTX` e o padrão de
migrations/repos do `internal/platform`; o housekeeping reusa `internal/platform/worker` no molde do
reaper de outbox.

O `internal/agent` passa a ser o primeiro consumidor: o atual `workflow.Workflow`/`composite`
(dispatch plano `kind→tool`) é renomeado para **`IntentWorkflow`** e o write de transactions é
migrado para um `Definition` do kernel, em que `record-expense` vira um workflow multi-step real
(WriteGuard como passos 1:1 → `ResolveCategory` com branch e suspend/resume → `Persist` → `Format`).
O `pendingexpense.Draft` deixa de morar em `agent_sessions.pending_action` e passa a ser o **estado
serializado do run suspenso** do kernel — provando o reuso do suspend/resume genérico. O corte é
protegido por **feature flag** com fallback ao caminho atual (rollback instantâneo, zero regressão).
A separação "kernel genérico vs workflow de intent" é codificada em governança como **gate**
(`R-WF-KERNEL-001` + addendum `R-AGENT-WF-001.6/.8`) **antes** de qualquer código do kernel.

Decisões materiais estão registradas em ADRs: [ADR-001](adr-001-workflow-kernel-platform-coexist.md),
[ADR-002](adr-002-durable-state-optimistic-concurrency.md),
[ADR-003](adr-003-suspend-resume-generalization.md),
[ADR-004](adr-004-governance-gate.md),
[ADR-005](adr-005-feature-flag-cutover.md).

## Arquitetura do Sistema

### Visão Geral dos Componentes

**Novos — `internal/platform/workflow` (kernel genérico, sem deps de domínio):**

- `step.go` — `Step[S]`, `StepOutput[S]`, `StepFunc[S]`, tipos fechados `RunStatus`/`StepStatus`/
  `SuspendReason` e `Suspension`.
- `combinators.go` — `Sequence[S]`, `Branch[S]`, `Parallel[S]`, `Retry[S]` (cada um devolve `Step[S]`).
- `engine.go` — `Engine[S]`, `Definition[S]`, `Start`, `Resume`, `RunResult[S]`; cursor de retomada,
  laço de retry, emissão de métricas/spans por passo.
- `store.go` — porta `Store` (interface no consumidor/kernel), `Snapshot`, `StepRecord`, `RetryPolicy`.
- `codec.go` — serialização do estado `S` para `[]byte` (JSON) e de volta.
- `infrastructure/postgres/store.go` — adapter `Store` sobre `database.DBTX` (INSERT/Load/Save com
  CAS/AppendStep/DeleteCompleted), `nullable.go` espelhando o padrão do agent.
- `housekeeping.go` — `HousekeepingJob` (implementa `worker.Job`) para retenção configurável.
- `factory.go` — `NewStoreFactory(o11y)` no padrão de `RepositoryFactory` do agent.

**Modificados — `internal/agent` (consumidor; mantém semântica própria):**

- `application/workflow/{workflow,composite,registry}.go` → renomeados para `IntentWorkflow`/
  `intentComposite`/`IntentRegistry` (RF-19). Contrato e comportamento preservados para leitura/cards/
  budget/conversational.
- `application/workflow/transactions_write.go` (novo) — monta o `Definition[ExpenseState]` do kernel
  para o write de transactions, reusando bindings/usecases existentes.
- `application/workflow/steps/` (novo) — passos finos do agent: `authorize`, `replay`, `policy`,
  `audit_begin`, `resolve_category` (branch+suspend), `persist`, `format`. São adapters: zero regra de
  domínio, zero SQL, sem LLM.
- `application/services/daily_ledger_agent.go` — `dispatchWrite` e `continuePendingExpenseConfirmation`
  passam a, sob feature flag, delegar ao `Engine.Start`/`Engine.Resume`; caminho atual permanece como
  fallback.
- `module.go` — wiring do `Engine`, `Store` (sobre `sessionDB`) e do `HousekeepingJob` (DI manual).
- `domain/pendingexpense/draft.go` — passa a ser o estado serializado do run do kernel (sem mudança de
  contrato; `AwaitingKind`/`TransactionKind` continuam fechados).

**Governança (gate, antes do código — ADR-004):**

- `.claude/rules/workflow-kernel.md` (novo, `R-WF-KERNEL-001`).
- `.claude/rules/agent-workflows-tools.md` — addendum em `R-AGENT-WF-001.6/.8`.

### Fluxo de Dados

```
inbound (telegram/whatsapp consumer)
  -> AgentRuntime.Execute (Thread→Run audit do agent, INALTERADO)
     -> DailyLedgerAgent.route
        -> [resume-before-parse]
           if flag on: Engine.Resume(workflow="transactions_write", key=user:channel, resume=text)
           else:       continuePendingExpenseConfirmation (caminho atual)
        -> ParseInbound (LLM — única fronteira LLM)
        -> dispatchWrite (kind.IsWrite())
           if flag on: Engine.Start(Definition[ExpenseState], key=user:channel, initial)
              -> Sequence: Authorize -> Replay -> Policy -> AuditBegin
                          -> ResolveCategory (Branch: auto | ambiguous→Suspend | confirm→Suspend)
                          -> Persist (binding→usecase transactions, uow próprio do módulo)
                          -> Format
              -> snapshot durável em workflow_runs/workflow_steps (uow do sessionDB)
           else: IntentRegistry.Resolve(kind) + WriteGuard (caminho atual)
        -> RouteResult{Reply, Outcome, Kind}  (contrato inalterado)
```

O kernel **não** conhece `intent`, `user_id` ou `channel`: recebe um `correlationKey string` opaco e
um estado `S` serializável. A semântica (Thread/Run/WorkingMemory/PendingStep) permanece exclusiva do
agent (ADR-001).

## Design de Implementação

### Interfaces Chave

Primitivos do kernel (genéricos, `internal/platform/workflow`):

```go
type RunStatus int
const (
    RunStatusRunning RunStatus = iota + 1
    RunStatusSuspended
    RunStatusSucceeded
    RunStatusFailed
)

type StepStatus int
const (
    StepStatusCompleted StepStatus = iota + 1
    StepStatusSuspended
    StepStatusFailed
    StepStatusSkipped
)

type SuspendReason int
const (
    SuspendAwaitingInput SuspendReason = iota + 1
)

type Suspension struct {
    Reason SuspendReason
    Prompt string
}

type StepOutput[S any] struct {
    State    S
    Status   StepStatus
    Suspend  *Suspension
}

type Step[S any] interface {
    ID() string
    Execute(ctx context.Context, state S) (StepOutput[S], error)
}
```

Combinadores (cada um devolve `Step[S]`):

```go
func StepFunc[S any](id string, fn func(context.Context, S) (StepOutput[S], error)) Step[S]
func Sequence[S any](id string, steps ...Step[S]) Step[S]
func Branch[S any](id string, decide func(S) string, routes map[string]Step[S]) Step[S]
func Parallel[S any](id string, merge func(base S, results []S) S, steps ...Step[S]) Step[S]
func Retry[S any](step Step[S], policy RetryPolicy) Step[S]
```

Engine e porta de persistência:

```go
type Definition[S any] struct {
    ID          string
    Root        Step[S]
    Durable     bool
    MaxAttempts int
}

type Engine[S any] interface {
    Start(ctx context.Context, def Definition[S], key string, initial S) (RunResult[S], error)
    Resume(ctx context.Context, def Definition[S], key string, resume []byte) (RunResult[S], error)
}

type RunResult[S any] struct {
    RunID   uuid.UUID
    Status  RunStatus
    State   S
    Suspend *Suspension
}

type Store interface {
    Insert(ctx context.Context, snap Snapshot) error
    Load(ctx context.Context, workflow, key string) (Snapshot, bool, error)
    Save(ctx context.Context, snap Snapshot, expectedVersion int64) error
    AppendStep(ctx context.Context, rec StepRecord) error
    DeleteCompleted(ctx context.Context, retention time.Duration, limit int) (int64, error)
}
```

A porta `Store` segue R6.3 (interface no consumidor); o adapter postgres a implementa sobre
`database.DBTX`, resolvendo a tx via `database.FromContext` exatamente como `idempotency.postgresStorage.conn`.

### Modelos de Dados

`Snapshot` / `StepRecord` (kernel):

```go
type Snapshot struct {
    RunID         uuid.UUID
    Workflow      string
    CorrelationKey string
    Status        RunStatus
    SuspendReason SuspendReason
    Cursor        int
    State         []byte
    Attempts      int
    MaxAttempts   int
    Version       int64
    LastError     string
    CreatedAt     time.Time
    UpdatedAt     time.Time
    EndedAt       *time.Time
}

type StepRecord struct {
    ID         uuid.UUID
    RunID      uuid.UUID
    StepID     string
    Seq        int
    Status     StepStatus
    Attempt    int
    DurationMs int64
    Error      string
    StartedAt  time.Time
    EndedAt    *time.Time
}
```

Migração `migrations/000019_create_workflow_runtime.up.sql` (schema `mecontrola`, cabeçalho
`SET LOCAL lock_timeout/statement_timeout` e `fillfactor=70` no padrão de outbox):

```sql
CREATE TABLE IF NOT EXISTS mecontrola.workflow_runs (
    id              UUID        NOT NULL,
    workflow        TEXT        NOT NULL,
    correlation_key TEXT        NOT NULL,
    status          TEXT        NOT NULL,
    suspend_reason  TEXT        NOT NULL DEFAULT '',
    cursor          INT         NOT NULL DEFAULT 0,
    state           JSONB       NOT NULL,
    attempts        INT         NOT NULL DEFAULT 0,
    max_attempts    INT         NOT NULL,
    version         BIGINT      NOT NULL DEFAULT 1,
    last_error      TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at        TIMESTAMPTZ,
    CONSTRAINT workflow_runs_pkey PRIMARY KEY (id),
    CONSTRAINT workflow_runs_status_check
        CHECK (status IN ('running','suspended','succeeded','failed')),
    CONSTRAINT workflow_runs_attempts_check     CHECK (attempts >= 0),
    CONSTRAINT workflow_runs_max_attempts_check CHECK (max_attempts > 0)
) WITH (fillfactor = 70);

CREATE UNIQUE INDEX IF NOT EXISTS workflow_runs_active_key_uidx
    ON mecontrola.workflow_runs (workflow, correlation_key)
    WHERE status IN ('running','suspended');

CREATE INDEX IF NOT EXISTS workflow_runs_status_updated_idx
    ON mecontrola.workflow_runs (status, updated_at);

CREATE TABLE IF NOT EXISTS mecontrola.workflow_steps (
    id          UUID        NOT NULL,
    run_id      UUID        NOT NULL,
    step_id     TEXT        NOT NULL,
    seq         INT         NOT NULL,
    status      TEXT        NOT NULL,
    attempt     INT         NOT NULL DEFAULT 1,
    duration_ms BIGINT      NOT NULL DEFAULT 0,
    error       TEXT        NOT NULL DEFAULT '',
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at    TIMESTAMPTZ,
    CONSTRAINT workflow_steps_pkey PRIMARY KEY (id),
    CONSTRAINT workflow_steps_run_fkey
        FOREIGN KEY (run_id) REFERENCES mecontrola.workflow_runs (id) ON DELETE CASCADE,
    CONSTRAINT workflow_steps_status_check
        CHECK (status IN ('completed','suspended','failed','skipped'))
);

CREATE INDEX IF NOT EXISTS workflow_steps_run_seq_idx
    ON mecontrola.workflow_steps (run_id, seq);
```

- **Lock otimista (RF-10)**: `Save` executa `UPDATE ... SET ..., version = version + 1
  WHERE id = $ AND version = $expected`; `RowsAffected()==0` ⇒ `ErrVersionConflict` (run sendo
  processado por outra entrega/instância) — espelha `card_repository.UpdateLimitByIDForUser`.
- **Run ativo único (RF-09/RF-10)**: índice parcial único `(workflow, correlation_key)` em
  `status IN ('running','suspended')` impede dois runs ativos para a mesma chave.
- **Snapshot só p/ escrita/suspensível (RF-08)**: `Definition.Durable=false` ⇒ engine executa
  in-process e **não** toca `workflow_runs`.

### Estado do consumidor (agent)

O estado serializado `S` do workflow de transactions write **é** o `pendingexpense.Draft` (RF-20),
acrescido dos campos de execução necessários ao replay determinístico do passo suspenso. Nada de
`context`, closures ou handles não serializáveis no estado: o `settle` da auditoria vive **somente
in-process no turno inicial** (ver Resume abaixo). `correlationKey = user_id + ":" + channel`
(string opaca para o kernel).

### Decomposição do fluxo de prova (`record-expense`) — preservação 1:1

`Definition[ExpenseState]{ ID: "transactions_write", Durable: true, Root: Sequence(...) }`:

1. `authorize` — `StepFunc`; chama `authorizeWrite`; falha ⇒ `StepOutput{Status: completed}` com
   estado marcando `OutcomeAuthzDenied` e short-circuit (engine encerra com `succeeded` operacional,
   espelhando `outcomeSucceeded`).
2. `replay` — consulta `replayDecision`; hit ⇒ short-circuit `OutcomeReplay`.
3. `policy` — `policy.Evaluate(kind, confidence)`; clarify ⇒ short-circuit `OutcomePolicyBlocked`.
4. `audit_begin` — `beginDecisionAudit`; conflito ⇒ `OutcomeReplay`; falha ⇒ `OutcomeUsecaseError`;
   sucesso ⇒ guarda o `settle` **in-process** (mapa `runID→settle` no driver do agent, não no estado).
5. `resolve_category` — `Branch(decide)`:
   - `auto` ⇒ resolve categoria (score ≥ auto) e segue;
   - `ambiguous` ⇒ grava candidates no estado, `StepOutput{Status: suspended, Suspend{AwaitingInput}}`
     com `AwaitingKind=category_choice`;
   - `confirm` ⇒ idem com `AwaitingKind=category_confirm`.
6. `persist` — binding→usecase `log_transaction_from_agent` (uow do **módulo transactions**, nunca
   compartilhado — ver Riscos); sucesso ⇒ `OutcomeRouted`.
7. `format` — formata reply final (sem regra de domínio).

**Short-circuit**: passos de guarda que terminam o fluxo retornam estado com `Outcome` setado e o
engine encerra a sequência (cursor aponta para o passo; status terminal `succeeded`/`failed` conforme
`outcomeSucceeded`). Isso reproduz exatamente a semântica de `WriteGuard.Apply` + `GuardShortCircuit`.

**Settle (auditoria)**: ao suspender em `resolve_category`, o driver chama `settle(executed=false)`
no turno inicial — idêntico ao comportamento atual (decisão fica REJEITADA ao clarificar). Ao
concluir `persist` no turno inicial (caminho auto), chama `settle(executed=true)`. Preservação 1:1.

### Resume (suspend → resume), preservando "resume antes do parse"

- Em `Suspend`, o engine grava `Snapshot{Status: suspended, Cursor: idx(resolve_category),
  State: draft}` (uow do sessionDB). O `agent_sessions.pending_action` deixa de ser a fonte do draft
  (ADR-003); migração de leitura descrita em Riscos.
- `continuePendingExpenseConfirmation` (sob flag) chama `Engine.Resume(def, key, resumeBytes)`:
  - `Store.Load` busca run `suspended` por `(workflow, key)`; ausência ⇒ segue para o parse (igual hoje).
  - O passo suspenso (`resolve_category`) re-executa com o estado mesclado ao input do usuário
    (escolha/confirmação interpretada **sem LLM**, lógica migrada de `resolvePendingCategoryChoice/
    Confirm`). Resolve determinístico (`ForceCategory`), `persist` e `format` seguem.
  - Cancelamento ("não/cancela") ⇒ `Status: succeeded` com reply de cancelamento; run encerrado.
- Idempotência de resume (RF-09): `Save` com CAS por `version` + status terminal garante que duas
  entregas concorrentes do mesmo run não dupliquem efeito; o índice parcial único reforça run ativo
  único. Garantia **estritamente mais forte** que a atual (que não auditava o write de resume).

### Retry e falha terminal (RF-11/RF-12)

`Retry(step, RetryPolicy{MaxAttempts, BaseBackoff, MaxBackoff})` reexecuta o passo em erro
**não-terminal** (erros de domínio mapeados a outcome não retentam). Backoff exponencial com jitter no
molde de `outbox` (`reaper`/`dispatcher`). Esgotado `MaxAttempts` no nível do run, o engine grava
`Snapshot{Status: failed, LastError}` e emite `workflow_runs_total{status="failed"}` — sem retry
infinito; sem dead-letter no MVP (Fora de Escopo do PRD).

### Configuração (env)

```
WORKFLOW_KERNEL_TRANSACTIONS_WRITE_ENABLED   bool    (feature flag de cutover; default false)
WORKFLOW_KERNEL_MAX_ATTEMPTS                 int     (default 3)
WORKFLOW_KERNEL_RETRY_BASE_BACKOFF           duration(default 200ms)
WORKFLOW_KERNEL_RETRY_MAX_BACKOFF            duration(default 5s)
WORKFLOW_KERNEL_HOUSEKEEPING_RETENTION_DAYS  int     (default 30)
WORKFLOW_KERNEL_HOUSEKEEPING_SCHEDULE        string  (cron robfig; default "@daily")
```

Validação no padrão de `configs/config.go` (mensagens "X inválido"), com testes em `configs/config_test.go`.

## Pontos de Integração

- **Banco (Postgres)**: novas tabelas `workflow_runs`/`workflow_steps` no schema `mecontrola`,
  gravadas pela `uow` sobre o `sessionDB` do agent. Sem tx cross-módulo.
- **worker/jobs**: `HousekeepingJob` registrado no `worker.Manager` via `module.go` (lista de jobs),
  schedule cron, `Timeout()` no molde de `outbox.HousekeepingJob`.
- **Observabilidade (otel-lgtm)**: spans/métricas via `observability.Observability` injetada.
- **Sem novas integrações externas**; LLM permanece restrito ao `ParseInbound` (R-AGENT-WF-001.4).

## Abordagem de Testes

### Testes Unitários

- **Kernel control-flow (RF-31, puro, sem mocks de infra)**: tabela de cenários para `Sequence`
  (ordem, threading de estado, short-circuit), `Branch` (rota por decisão pura), `Parallel`
  (agregação determinística + cancelamento via `context` cancelado), `Retry` (tentativas, backoff,
  falha terminal). `Engine` com `Store` fake in-memory para Start/Resume/Suspend/cursor.
- **Store postgres**: unit com mock de `database.DBTX` para CAS (`RowsAffected==0 ⇒ ErrVersionConflict`),
  Insert/Load/AppendStep/DeleteCompleted.
- **Passos do agent (RF-26)**: cada passo testado isolado no padrão testify/suite (R-TESTING-001),
  whitebox, `fake.NewProvider()`, mocks por IIFE.
- **Não regressão (RF-24)**: suíte de paridade dirigida por tabela compara, para os mesmos inputs,
  `Reply`/`Outcome`/`Kind` do caminho atual vs caminho kernel (flag on/off) — incluindo auto-log,
  ambiguous→choice→resume, needs_confirm→confirm/cancel→resume, authz_denied, replay, policy_blocked,
  usecase_error, missing_resolver.

### Testes de Integração

Critérios atendidos (fronteira de IO crítica em banco; mocks não garantem correção do CAS/índice
parcial/resume durável) ⇒ **integration tests recomendados e adotados**, com `testcontainers-go` e
build tag `//go:build integration` (padrão de `migrations/migrations_integration_test.go`):

- Durabilidade/resume (RF-09): `Start`→`Suspend`→(reabrir Engine/Store, simulando restart)→`Resume`
  conclui sem duplicar `persist`.
- Concorrência (RF-10): duas goroutines `Resume` no mesmo run ⇒ exatamente uma vence (CAS), zero
  efeito duplicado.
- Housekeeping (RF-17): runs concluídos além da retenção são purgados; suspensos/ativos preservados.
- Migração `000019` up/down idempotente.

### Testes E2E

Fluxo inbound→reply do `record-expense` via consumer fake com flag on, cobrindo auto-log e o ciclo
ambiguous→escolha→persistência, validando reply final idêntico ao caminho atual.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Gate de governança (ADR-004)** — `R-WF-KERNEL-001` + addendum `R-AGENT-WF-001.6/.8`. Pré-condição
   bloqueante (RF-29), antes de qualquer `.go` do kernel.
2. **Kernel puro** — `step.go`, `combinators.go` (Sequence/Branch/Parallel/Retry), `codec.go` e testes
   unitários puros (RF-02..RF-06, RF-16, RF-31). Sem persistência ainda.
3. **Engine + Store (porta) + fake** — `engine.go` (cursor, retry, falha terminal, métricas), `store.go`,
   fake in-memory; testes de Start/Resume/Suspend (RF-08/09/11/12/13/14/15).
4. **Adapter postgres + migração `000019`** — `infrastructure/postgres/store.go`, CAS, índice parcial;
   integration tests (RF-07/10) + housekeeping job (RF-17).
5. **Rename para IntentWorkflow (RF-19)** — refactor mecânico de `workflow.Workflow`/`composite`/
   `registry` no agent, sem mudança de comportamento; suíte verde.
6. **Passos do agent + Definition transactions_write** — `application/workflow/steps/*` e
   `transactions_write.go`, reusando bindings/usecases (RF-21/22/23).
7. **Integração no driver sob flag** — `dispatchWrite`/`continuePendingExpenseConfirmation` delegam ao
   Engine quando `WORKFLOW_KERNEL_TRANSACTIONS_WRITE_ENABLED`; fallback ao caminho atual (ADR-005).
   Wiring em `module.go`.
8. **Paridade + DX (RF-20/24/25)** — suíte de não regressão; demo de extensão "novo fluxo só
   registrando steps, zero `case intent.Kind`".
9. **Validação final** — gates `R-ADAPTER-001`/`R-AGENT-WF-001`/`R-TESTING-001`/`R-WF-KERNEL-001` +
   checklist R0–R7 (RF-32).

### Dependências Técnicas

- Postgres (migração `000019`). `worker.Manager` já existente. `uow`/`database.DBTX` já existentes.
- Gate de governança concluído (item 1) é bloqueante para itens 2+.

## Monitoramento e Observabilidade

Métricas (cardinalidade controlada — RF-14, herda R-TXN-004/R-AGENT-WF-001.5; **proibido** `user_id`/
`correlation_key`/`category_id` como label):

- `workflow_runs_total{workflow, status}`
- `workflow_run_duration_seconds{workflow}`
- `workflow_steps_total{workflow, step, status}`
- `workflow_step_duration_seconds{workflow, step}`
- `workflow_suspend_total{workflow, reason}`
- `workflow_resume_total{workflow, result}`
- `workflow_version_conflict_total{workflow}`

Spans: `workflow.engine.start`, `workflow.engine.resume`, `workflow.step.execute` (atributos
`workflow`, `step`, `attempt`, `status`, `duration_ms`). Logs em pontos de suspensão, conflito de
versão e falha terminal. Métricas do `AgentRuntime` (`agent_runs_total` etc.) permanecem inalteradas
(ADR-001). Dashboards Grafana no padrão `otel-grafana-dashboards` (fast-follow).

## Considerações Técnicas

### Decisões Chave

- **ADR-001** — Kernel genérico em `internal/platform/workflow`, coexistindo com `agent_runs`/
  `RunGateway` (sem unificar): separação de concerns, MVP de menor risco.
- **ADR-002** — Estado durável em `workflow_runs`/`workflow_steps` (relacional) + lock otimista por
  versão + idempotência por run + falha terminal determinística; snapshot só para escrita/suspensível.
- **ADR-003** — Suspend/resume genérico; `pendingexpense.Draft` vira estado do run; "resume antes do
  parse" preservado; lógica de interpretação de resposta migrada sem LLM.
- **ADR-004** — Gate de governança (`R-WF-KERNEL-001` + addendum `R-AGENT-WF-001.6/.8`) antes do código.
- **ADR-005** — Cutover por feature flag com fallback ao caminho atual (rollback instantâneo).

Control-flow **Core** (Sequence/Branch/Parallel/Retry + suspend/resume) — sem loops/map/nested
(Fora de Escopo do PRD). `parallel()` incluído no MVP por decisão do PRD, com teste dedicado mesmo
sem consumidor imediato.

### Riscos Conhecidos

- **Tx cross-módulo proibida**: `persist` chama o módulo transactions (uow próprio) e o snapshot usa o
  uow do sessionDB — **não atômicos** entre si. Mitigação: idempotência por run (CAS + status
  terminal) + auditoria de decisão existente garantem "no duplicate effect" mesmo sem 2PC; a ordem é
  `persist` → marcar run `succeeded`; uma falha entre os dois resulta em retry/resume idempotente, não
  em duplicação (a re-execução de `persist` é protegida pela idempotência do módulo transactions e
  pelo CAS do run). Documentar no runbook.
- **Resume single-level (cursor de topo)**: MVP suporta suspensão no nível da `Sequence` raiz (1
  ponto de suspensão por run — suficiente para `record-expense`). Suspensão aninhada dentro de
  `Branch`/`Parallel` é **fast-follow** documentado; o modelo de cursor já comporta extensão.
- **Migração do draft de `agent_sessions` → `workflow_runs`**: drafts pendentes gravados antes do
  cutover não existem em `workflow_runs`. Mitigação: na ativação da flag, `continuePendingExpense...`
  consulta primeiro o `Store`; se vazio, faz fallback de leitura ao `agent_sessions.pending_action`
  legado (lido, executado e limpo pelo caminho atual) por uma janela de drenagem; sem perda de estado.
- **Renomear Workflow→IntentWorkflow**: churn amplo. Mitigação: refactor mecânico isolado (item 5),
  sem mudança de comportamento, com suíte verde antes de prosseguir.
- **Generics + serialização**: `S` deve ser JSON-serializável; campos não exportados não persistem.
  Mitigação: `ExpenseState` espelha `pendingexpense.Draft` (campos exportados) + teste de round-trip
  `Encode/Decode`.

### Conformidade com Padrões

- `.claude/rules/go-adapters.md` (R-ADAPTER-001): zero comentários em `.go`; passos do agent e tools
  finos sem SQL/regra/branching de domínio.
- `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001): roteamento via registry; sem novo `case
  intent.Kind`; `ToolOutcome`/`RunStatus` fechados; LLM só no parse; Run auditável; pending step salvo
  em clarificação. Addendum .6/.8 (ADR-004).
- `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001, novo): kernel sem regra/SQL de domínio, estados
  fechados, cardinalidade controlada.
- `.claude/rules/go-testing.md` (R-TESTING-001): testify/suite whitebox, `fake.NewProvider()`, IIFE.
- `.claude/rules/governance.md`: DMMF state-as-type prevalece; anti-padrões (`Result[T,E]` monádico,
  currying, DSL, monads) proibidos — `StepOutput[S]` é valor de saída, **não** mônada de erro (erros
  via `error`).
- go-implementation R0 (sem `init()`), R5.12 (sem `panic`), R6 (`context.Context` nas fronteiras de IO;
  interface `Store` no consumidor), R7 (`errors.Join`/`fmt.Errorf %w`). Sem abstração de tempo
  (`time.Now().UTC()` inline). `defer func(){ _ = rows.Close() }()`.

### Arquivos Relevantes e Dependentes

Novos: `internal/platform/workflow/{step,combinators,engine,store,codec,housekeeping,factory}.go`,
`internal/platform/workflow/infrastructure/postgres/{store,nullable}.go`,
`migrations/000019_create_workflow_runtime.{up,down}.sql`,
`internal/agent/application/workflow/transactions_write.go`,
`internal/agent/application/workflow/steps/*.go`,
`.claude/rules/workflow-kernel.md`,
`.specs/prd-workflow-kernel/adr-00{1..5}-*.md`.

Modificados: `internal/agent/application/workflow/{workflow,composite,registry}.go` (→ IntentWorkflow),
`internal/agent/application/services/{daily_ledger_agent,agent_workflows}.go`,
`internal/agent/module.go`, `internal/agent/domain/pendingexpense/draft.go` (uso),
`.claude/rules/agent-workflows-tools.md` (addendum), `configs/config.go` + `configs/config_test.go`.

Dependentes (somente leitura/validação): `internal/agent/application/services/agent_runtime.go`
(inalterado), `internal/agent/application/services/decision_audit.go`,
`internal/agent/application/tools/{transactions_tools,clarification}.go`,
`internal/agent/infrastructure/binding/{transaction_log,expense_confirmation,category_error}.go`,
`internal/platform/{database/uow,worker}/*`.

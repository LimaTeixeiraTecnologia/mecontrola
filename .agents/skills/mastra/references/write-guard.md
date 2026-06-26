# Write seam — guarda de escrita via kernel steps

R-AGENT-WF-001.2: a lógica de pré-escrita (authz + replay + policy + decision audit) **não** é
duplicada por tool. Ela vive como uma sequência de **kernel steps** durável, executada pelo kernel
genérico (`internal/platform/workflow`) a partir da definição de escrita do agent — não no workflow
roteável (`composite`).

## Arquivos

- `application/workflow/transactions_write.go` — `NewTransactionsWriteDefinition(TransactionsWriteDeps)`
  monta a `platform.Definition[steps.ExpenseState]` durável com os steps na ordem correta.
- `application/workflow/steps/` — os steps concretos: `authorize.go`, `replay.go`, `policy.go`,
  `audit_begin.go`, `resolve_category.go`, `persist.go`, `format.go`.
- `application/services/agent_workflows.go` — `buildKernelDefinition(*KernelDeps)` injeta as funções
  concretas (`Authorize`/`Replay`/`Policy`/`AuditBegin`/`Persist`/...) na definição.
- `application/services/daily_ledger_agent.go` — `dispatchWrite` roteia por `kind.IsKernelWrite()`
  para `dispatchWriteKernel`, que executa via `kernelEngine.Start(...)`/`Resume(...)`.

## Quando a guarda roda

A escrita durável passa pelo kernel **quando `kind.IsKernelWrite()`** (`domain/intent/intent.go`).
Leitura e fallback resolvem direto pelo `composite` (sem guarda). Operação destrutiva/sensível tem
seam próprio (confirmation) — ver `add-workflow-tool.md` §5.

```go
func (a *DailyLedgerAgent) dispatchWrite(...) RouteResult {
    kind := parsed.Intent.Kind()
    if a.isDestructiveKind(kind) {
        return a.dispatchWriteDestructive(...)   // confirmation seam (HITL)
    }
    if kind.IsKernelWrite() {
        return a.dispatchWriteKernel(...)        // kernel write seam (steps abaixo)
    }
    wf, ok := a.registry.Resolve(kind)           // leitura/composite: sem guarda
    ...
}
```

## A sequência de steps (`NewTransactionsWriteDefinition`)

`platform.Sequence("transactions_write_seq", ...)` — cada `platform.Step[ExpenseState]` pode
short-circuitar (bloquear antes do persist):

1. **`steps.NewAuthorize`** — `principal.UserID == in.UserID`; senão bloqueia com `DenyReply`.
2. **`steps.NewReplay`** — idempotência: se a mensagem já foi processada, devolve a resposta anterior.
3. **`steps.NewPolicy`** — `policy.Evaluate(kind, confidence)`; abaixo do mínimo configurado bloqueia
   (confidence insuficiente).
4. **`steps.NewAuditBegin`** (com `platform.Retry`) — registra a decisão e a `OnSettle` que fecha o
   decision audit; conflito idempotente devolve a resposta de replay.
5. **`steps.NewResolveCategory`** (com `platform.Retry`) — resolve a categoria.
6. **`steps.NewPersist`** (com `platform.Retry`) — efetiva a escrita via binding → usecase.
7. **`steps.NewFormat`** — formata a resposta final.

A definição é `Durable: true`: suspende/resume via `Snapshot` do kernel (merge-patch no resume,
R-WF-KERNEL-001.7). Toda escrita referencia o `decision_id` do audit trail (R-AGENT-WF-001.5).

## Como adicionar um novo write tool durável

1. Garantir `kind.IsKernelWrite() == true` (`domain/intent/intent.go`).
2. Encaixar o comportamento na sequência de `NewTransactionsWriteDefinition` (ou criar definição
   análoga no kernel write seam) — **não** reimplementar authz/replay/policy/audit dentro da tool.
3. Registrar a `CapabilitySpec` correspondente (`Mode: ModeWrite`) em
   `internal/agent/application/capability/build.go`.
4. Toda escrita referencia o `decision_id` do audit trail (R-AGENT-WF-001.5).

## Métricas

Labels permitidos são enums fechados (`agent_id`, `channel`, `workflow`, `status`, `tool`,
`outcome`). **Proibido** `user_id`, `correlation_key` ou `category_id` como label
(R-AGENT-WF-001.5 / R-TXN-004).

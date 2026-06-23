# WriteGuard — guarda de escrita compartilhada

R-AGENT-WF-001.2: a lógica de pré-escrita (authz + replay + policy + decision audit) **não** é
duplicada por tool. Vive num step de guarda reutilizável aplicado pelos workflows de escrita.

## Arquivos

- `application/workflow/write_guard.go` — `WriteGuard`, `GuardSteps`, `Apply`, `SettleFunc`.
- `application/services/agent_workflows.go` — `newWriteGuard()` injeta os 4 steps concretos.
- `application/workflow/composite.go` — `Execute` decide quando aplicar a guarda.

## Quando a guarda roda

```go
if !kind.IsWrite() || c.guard == nil {
    return tool.Execute(ctx, in)   // leitura/fallback: sem guarda
}
decision, blocked, settle := c.guard.Apply(ctx, in)
if decision == GuardShortCircuit {
    return blocked, nil            // authz negado / replay / policy / audit conflict
}
result, err := tool.Execute(ctx, in)
if settle != nil && err == nil {
    settle(ctx, result.Outcome == tools.OutcomeRouted)  // fecha o decision audit
}
return result, err
```

**Guard `nil` em workflow de leitura é correto e intencional** (ex.: `conversational`). Não é
regressão. `IsWrite()` (`domain/intent/intent.go`) é a fonte de verdade de quais kinds escrevem.

## Os 4 steps (`GuardSteps`)

Cada step retorna `(ToolResult, bool)` onde `bool=true` significa short-circuit (bloqueia antes da tool):

1. **Authorize** — `principal.UserID == in.UserID`; senão `OutcomeAuthzDenied`.
2. **Replay** — `replayDecision(...)`; se já processado, devolve a resposta anterior (idempotência).
3. **Policy** — `policy.Evaluate(kind, confidence)`; se `PolicyDecisionClarify`, bloqueia com
   `OutcomePolicyBlocked` (confidence abaixo do mínimo configurado).
4. **Audit** — `beginDecisionAudit(...)`: registra a decisão e devolve uma `SettleFunc`. Conflito
   idempotente → `OutcomeReplay`; falha de escrita do audit → `OutcomeUsecaseError`.

## Como adicionar um novo write tool

1. Garantir `kind.IsWrite() == true` (passo 1 de `add-workflow-tool.md`).
2. Vincular a tool a um workflow que receba o `guard` compartilhado (`a.newWriteGuard()`), **não**
   `nil`.
3. Não reimplementar authz/replay/policy/audit dentro da tool — a guarda já cobre.
4. Toda escrita referencia o `decision_id` do audit trail (R-AGENT-WF-001.5).

## Métricas

Labels permitidos são enums fechados (`kind`, `channel`, `outcome`, ...). **Proibido** `user_id` ou
`category_id` como label (R-AGENT-WF-001.5 / R-TXN-004).
